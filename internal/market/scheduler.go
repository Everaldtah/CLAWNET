package market

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/Everaldtah/CLAWNET/internal/config"
	"github.com/Everaldtah/CLAWNET/internal/protocol"
	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
)

// TaskScheduler manages task scheduling and execution
type TaskScheduler struct {
	mu sync.RWMutex

	cfg            *config.Config
	auctionManager *AuctionManager
	escrowManager  *EscrowManager
	reputation     *ReputationManager
	logger         *logrus.Entry

	// Task tracking
	tasks          map[string]*ScheduledTask
	pendingQueue   chan *ScheduledTask
	executingTasks map[string]*ScheduledTask
	completedTasks map[string]*ScheduledTask

	// Execution handlers
	handlers       map[protocol.TaskType]TaskHandler

	// Context
	ctx            context.Context
	cancel         context.CancelFunc
}

// ScheduledTask represents a scheduled task
type ScheduledTask struct {
	mu sync.RWMutex

	ID              string
	OriginalTask    *protocol.MarketTaskAnnouncePayload
	AuctionID       string
	Status          protocol.TaskStatus
	AssignedPeers   []string
	Results         map[string]*protocol.TaskResultPayload
	CreatedAt       time.Time
	StartedAt       time.Time
	CompletedAt     time.Time
	Deadline        time.Time
	RetryCount      int
	MaxRetries      int
	Timeout         time.Duration
}

// TaskHandler is a function that executes a task
type TaskHandler func(ctx context.Context, task *ScheduledTask) (*protocol.TaskResultPayload, error)

// NewTaskScheduler creates a new task scheduler
func NewTaskScheduler(cfg *config.Config, auctionManager *AuctionManager, 
	escrowManager *EscrowManager, reputation *ReputationManager, logger *logrus.Logger) *TaskScheduler {
	ctx, cancel := context.WithCancel(context.Background())

	ts := &TaskScheduler{
		cfg:            cfg,
		auctionManager: auctionManager,
		escrowManager:  escrowManager,
		reputation:     reputation,
		logger:         logger.WithField("component", "scheduler"),
		tasks:          make(map[string]*ScheduledTask),
		pendingQueue:   make(chan *ScheduledTask, 1000),
		executingTasks: make(map[string]*ScheduledTask),
		completedTasks: make(map[string]*ScheduledTask),
		handlers:       make(map[protocol.TaskType]TaskHandler),
		ctx:            ctx,
		cancel:         cancel,
	}

	// Start scheduler goroutines
	go ts.processPendingQueue()
	go ts.monitorTimeouts()
	go ts.processAuctionResults()

	return ts
}

// RegisterHandler registers a task handler
func (ts *TaskScheduler) RegisterHandler(taskType protocol.TaskType, handler TaskHandler) {
	ts.mu.Lock()
	defer ts.mu.Unlock()
	ts.handlers[taskType] = handler
}

// SubmitTask submits a task to the scheduler
func (ts *TaskScheduler) SubmitTask(task *protocol.MarketTaskAnnouncePayload) (*ScheduledTask, error) {
	ts.mu.Lock()
	defer ts.mu.Unlock()

	scheduledTask := &ScheduledTask{
		ID:            uuid.New().String(),
		OriginalTask:  task,
		Status:        protocol.TaskStatusPending,
		Results:       make(map[string]*protocol.TaskResultPayload),
		CreatedAt:     time.Now(),
		Deadline:      time.Unix(0, task.Deadline),
		MaxRetries:    3,
		Timeout:       ts.cfg.Market.TaskTimeout,
	}

	ts.tasks[scheduledTask.ID] = scheduledTask

	// If market is enabled, create auction
	if ts.cfg.Market.Enabled {
		scheduledTask.Status = protocol.TaskStatusBidding
		_, err := ts.auctionManager.CreateAuction(task)
		if err != nil {
			return nil, fmt.Errorf("failed to create auction: %w", err)
		}
	} else {
		// Direct assignment - add to pending queue
		select {
		case ts.pendingQueue <- scheduledTask:
		default:
			return nil, fmt.Errorf("pending queue full")
		}
	}

	ts.logger.WithFields(logrus.Fields{
		"task_id": scheduledTask.ID,
		"type":    task.Type,
		"status":  scheduledTask.Status,
	}).Info("Task submitted")

	return scheduledTask, nil
}

// CancelTask cancels a task
func (ts *TaskScheduler) CancelTask(taskID string) error {
	ts.mu.Lock()
	defer ts.mu.Unlock()

	task, exists := ts.tasks[taskID]
	if !exists {
		return fmt.Errorf("task not found")
	}

	task.mu.Lock()
	defer task.mu.Unlock()

	if task.Status == protocol.TaskStatusCompleted || task.Status == protocol.TaskStatusFailed {
		return fmt.Errorf("task already finished")
	}

	task.Status = protocol.TaskStatusCancelled

	// Cancel associated auction
	if task.AuctionID != "" {
		ts.auctionManager.CancelAuction(task.AuctionID)
	}

	// Refund escrow if exists
	escrows := ts.escrowManager.GetEscrowsByTask(taskID)
	for _, escrow := range escrows {
		ts.escrowManager.RefundEscrow(escrow.ID, 0)
	}

	ts.logger.WithField("task_id", taskID).Info("Task cancelled")
	return nil
}

// GetTask gets a task by ID
func (ts *TaskScheduler) GetTask(taskID string) (*ScheduledTask, error) {
	ts.mu.RLock()
	defer ts.mu.RUnlock()

	task, exists := ts.tasks[taskID]
	if !exists {
		return nil, fmt.Errorf("task not found")
	}

	return task, nil
}

// GetTasksByStatus gets tasks by status
func (ts *TaskScheduler) GetTasksByStatus(status protocol.TaskStatus) []*ScheduledTask {
	ts.mu.RLock()
	defer ts.mu.RUnlock()

	var result []*ScheduledTask
	for _, task := range ts.tasks {
		task.mu.RLock()
		taskStatus := task.Status
		task.mu.RUnlock()

		if taskStatus == status {
			result = append(result, task)
		}
	}
	return result
}

// processPendingQueue processes tasks from the pending queue
func (ts *TaskScheduler) processPendingQueue() {
	for {
		select {
		case task := <-ts.pendingQueue:
			ts.executeTask(task)
		case <-ts.ctx.Done():
			return
		}
	}
}

// executeTask executes a task
func (ts *TaskScheduler) executeTask(task *ScheduledTask) {
	ts.mu.Lock()
	handler, exists := ts.handlers[task.OriginalTask.Type]
	ts.mu.Unlock()

	if !exists {
		ts.logger.WithField("task_type", task.OriginalTask.Type).Error("No handler for task type")
		ts.failTask(task, "no handler for task type")
		return
	}

	task.mu.Lock()
	task.Status = protocol.TaskStatusExecuting
	task.StartedAt = time.Now()
	assignedPeers := task.AssignedPeers
	task.mu.Unlock()

	ts.mu.Lock()
	ts.executingTasks[task.ID] = task
	ts.mu.Unlock()

	// Create execution context with timeout
	ctx, cancel := context.WithTimeout(ts.ctx, task.Timeout)
	defer cancel()

	ts.logger.WithFields(logrus.Fields{
		"task_id": task.ID,
		"peers":   assignedPeers,
	}).Info("Executing task")

	// Execute the task
	result, err := handler(ctx, task)
	if err != nil {
		ts.logger.WithError(err).WithField("task_id", task.ID).Error("Task execution failed")
		ts.failTask(task, err.Error())
		return
	}

	// Store result
	task.mu.Lock()
	task.Results[result.ExecutorID] = result
	task.Status = protocol.TaskStatusCompleted
	task.CompletedAt = time.Now()
	task.mu.Unlock()

	ts.mu.Lock()
	delete(ts.executingTasks, task.ID)
	ts.completedTasks[task.ID] = task
	ts.mu.Unlock()

	// Release escrow
	ts.releaseEscrow(task, result)

	// Update reputation
	for _, peerID := range assignedPeers {
		onTime := task.CompletedAt.Before(task.Deadline)
		ts.reputation.RecordTaskCompletion(peerID, true, onTime)
	}

	ts.logger.WithFields(logrus.Fields{
		"task_id":  task.ID,
		"executor": result.ExecutorID,
	}).Info("Task completed")
}

// failTask marks a task as failed
func (ts *TaskScheduler) failTask(task *ScheduledTask, reason string) {
	task.mu.Lock()
	task.Status = protocol.TaskStatusFailed
	task.mu.Unlock()

	ts.mu.Lock()
	delete(ts.executingTasks, task.ID)
	ts.completedTasks[task.ID] = task
	ts.mu.Unlock()

	// Refund escrow
	escrows := ts.escrowManager.GetEscrowsByTask(task.ID)
	for _, escrow := range escrows {
		ts.escrowManager.RefundEscrow(escrow.ID, 0)
	}

	// Update reputation for assigned peers
	for _, peerID := range task.AssignedPeers {
		ts.reputation.RecordTaskCompletion(peerID, false, false)
	}
}

// releaseEscrow releases escrow for a completed task
func (ts *TaskScheduler) releaseEscrow(task *ScheduledTask, result *protocol.TaskResultPayload) {
	escrows := ts.escrowManager.GetEscrowsByTask(task.ID)
	for _, escrow := range escrows {
		verificationData, _ := result.Result.MarshalJSON()
		if err := ts.escrowManager.ReleaseEscrow(escrow.ID, verificationData); err != nil {
			ts.logger.WithError(err).WithField("escrow_id", escrow.ID).Error("Failed to release escrow")
		}
	}
}

// monitorTimeouts monitors for task timeouts
func (ts *TaskScheduler) monitorTimeouts() {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			ts.checkTimeouts()
		case <-ts.ctx.Done():
			return
		}
	}
}

// checkTimeouts checks for timed out tasks
func (ts *TaskScheduler) checkTimeouts() {
	ts.mu.RLock()
	tasks := make([]*ScheduledTask, 0, len(ts.executingTasks))
	for _, task := range ts.executingTasks {
		tasks = append(tasks, task)
	}
	ts.mu.RUnlock()

	now := time.Now()
	for _, task := range tasks {
		task.mu.RLock()
		deadline := task.Deadline
		started := task.StartedAt
		timeout := task.Timeout
		task.mu.RUnlock()

		// Check deadline timeout
		if now.After(deadline) {
			ts.logger.WithField("task_id", task.ID).Warn("Task deadline exceeded")
			ts.failTask(task, "deadline exceeded")
			continue
		}

		// Check execution timeout
		if !started.IsZero() && now.After(started.Add(timeout)) {
			ts.logger.WithField("task_id", task.ID).Warn("Task execution timeout")
			ts.failTask(task, "execution timeout")
			continue
		}
	}
}

// processAuctionResults processes auction results
func (ts *TaskScheduler) processAuctionResults() {
	resultChan := ts.auctionManager.GetResultChan()

	for {
		select {
		case result := <-resultChan:
			ts.handleAuctionResult(result)
		case <-ts.ctx.Done():
			return
		}
	}
}

// handleAuctionResult handles an auction result
func (ts *TaskScheduler) handleAuctionResult(result *AuctionResult) {
	ts.mu.RLock()
	task, exists := ts.tasks[result.TaskID]
	ts.mu.RUnlock()

	if !exists {
		ts.logger.WithField("task_id", result.TaskID).Warn("Task not found for auction result")
		return
	}

	if !result.Success {
		ts.logger.WithFields(logrus.Fields{
			"task_id": result.TaskID,
			"reason":  result.Reason,
		}).Warn("Auction failed")
		ts.failTask(task, "auction failed: "+result.Reason)
		return
	}

	task.mu.Lock()
	task.AuctionID = result.AuctionID

	if len(result.ConsensusWinners) > 0 {
		task.AssignedPeers = result.ConsensusWinners
	} else {
		task.AssignedPeers = []string{result.WinnerID}
	}
	task.mu.Unlock()

	// Create escrow
	if ts.cfg.Market.EscrowRequired {
		conditions := EscrowConditions{
			CompletionRequired:   true,
			VerificationRequired: true,
			Timeout:              ts.cfg.Market.TaskTimeout,
			ConsensusRequired:    task.OriginalTask.ConsensusMode,
			ConsensusThreshold:   task.OriginalTask.ConsensusThreshold,
		}

		escrow, err := ts.escrowManager.CreateEscrow(
			task.ID,
			task.OriginalTask.RequesterID,
			result.WinningBid,
			conditions,
		)
		if err != nil {
			ts.logger.WithError(err).Error("Failed to create escrow")
			ts.failTask(task, "escrow creation failed")
			return
		}

		// Lock escrow for each winner
		for _, peerID := range task.AssignedPeers {
			if err := ts.escrowManager.LockEscrow(escrow.ID, peerID); err != nil {
				ts.logger.WithError(err).Error("Failed to lock escrow")
			}
		}
	}

	// Add to pending queue for execution
	select {
	case ts.pendingQueue <- task:
	default:
		ts.logger.WithField("task_id", task.ID).Error("Pending queue full")
		ts.failTask(task, "queue full")
	}
}

// RetryTask retries a failed task
func (ts *TaskScheduler) RetryTask(taskID string) error {
	ts.mu.Lock()
	defer ts.mu.Unlock()

	task, exists := ts.tasks[taskID]
	if !exists {
		return fmt.Errorf("task not found")
	}

	task.mu.Lock()
	defer task.mu.Unlock()

	if task.Status != protocol.TaskStatusFailed {
		return fmt.Errorf("task not in failed state")
	}

	if task.RetryCount >= task.MaxRetries {
		return fmt.Errorf("max retries exceeded")
	}

	task.RetryCount++
	task.Status = protocol.TaskStatusPending

	// Re-submit to queue
	select {
	case ts.pendingQueue <- task:
	default:
		return fmt.Errorf("pending queue full")
	}

	ts.logger.WithFields(logrus.Fields{
		"task_id":     taskID,
		"retry_count": task.RetryCount,
	}).Info("Task retry scheduled")

	return nil
}

// GetStats returns scheduler statistics
func (ts *TaskScheduler) GetStats() map[string]interface{} {
	ts.mu.RLock()
	defer ts.mu.RUnlock()

	pendingCount := 0
	executingCount := 0
	completedCount := 0
	failedCount := 0

	for _, task := range ts.tasks {
		task.mu.RLock()
		status := task.Status
		task.mu.RUnlock()

		switch status {
		case protocol.TaskStatusPending, protocol.TaskStatusBidding:
			pendingCount++
		case protocol.TaskStatusExecuting:
			executingCount++
		case protocol.TaskStatusCompleted:
			completedCount++
		case protocol.TaskStatusFailed, protocol.TaskStatusTimeout:
			failedCount++
		}
	}

	return map[string]interface{}{
		"total_tasks":    len(ts.tasks),
		"pending":        pendingCount,
		"executing":      executingCount,
		"completed":      completedCount,
		"failed":         failedCount,
		"queue_length":   len(ts.pendingQueue),
	}
}

// Stop stops the scheduler
func (ts *TaskScheduler) Stop() {
	ts.cancel()
}
