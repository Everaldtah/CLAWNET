package task

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/Everaldtah/CLAWNET/internal/config"
	"github.com/Everaldtah/CLAWNET/internal/identity"
	"github.com/Everaldtah/CLAWNET/internal/market"
	"github.com/Everaldtah/CLAWNET/internal/openclaw"
	"github.com/Everaldtah/CLAWNET/internal/protocol"
	"github.com/sirupsen/logrus"
)

// Executor executes tasks
type Executor struct {
	mu sync.RWMutex

	cfg        *config.Config
	identity   *identity.Identity
	openclaw   *openclaw.Client
	logger     *logrus.Entry

	// Execution tracking
	executing  map[string]*ExecutingTask
	handlers   map[protocol.TaskType]TaskHandler
}

// ExecutingTask represents a task being executed
type ExecutingTask struct {
	ID           string
	Type         protocol.TaskType
	Data         json.RawMessage
	StartTime    time.Time
	CancelFunc   context.CancelFunc
}

// TaskHandler executes a specific task type
type TaskHandler func(ctx context.Context, data json.RawMessage) (json.RawMessage, error)

// NewExecutor creates a new task executor
func NewExecutor(cfg *config.Config, identity *identity.Identity, logger *logrus.Logger) (*Executor, error) {
	var ocClient *openclaw.Client
	var err error

	if cfg.OpenClaw.Enabled {
		ocClient, err = openclaw.NewClient(cfg.OpenClaw)
		if err != nil {
			return nil, fmt.Errorf("failed to create OpenClaw client: %w", err)
		}
	}

	e := &Executor{
		cfg:       cfg,
		identity:  identity,
		openclaw:  ocClient,
		logger:    logger.WithField("component", "executor"),
		executing: make(map[string]*ExecutingTask),
		handlers:  make(map[protocol.TaskType]TaskHandler),
	}

	// Register default handlers
	e.registerDefaultHandlers()

	return e, nil
}

// registerDefaultHandlers registers default task handlers
func (e *Executor) registerDefaultHandlers() {
	e.handlers[protocol.TaskTypeOpenClawPrompt] = e.handleOpenClawPrompt
	e.handlers[protocol.TaskTypeShellTask] = e.handleShellTask
	e.handlers[protocol.TaskTypeCompute] = e.handleComputeTask
	e.handlers[protocol.TaskTypeStorage] = e.handleStorageTask
}

// Execute executes a task
func (e *Executor) Execute(ctx context.Context, task *market.ScheduledTask) (*protocol.TaskResultPayload, error) {
	e.mu.Lock()
	handler, exists := e.handlers[task.OriginalTask.Type]
	if !exists {
		e.mu.Unlock()
		return nil, fmt.Errorf("no handler for task type: %s", task.OriginalTask.Type)
	}

	execTask := &ExecutingTask{
		ID:        task.ID,
		Type:      task.OriginalTask.Type,
		Data:      json.RawMessage("{}"),
		StartTime: time.Now(),
	}

	execCtx, cancel := context.WithCancel(ctx)
	execTask.CancelFunc = cancel
	e.executing[task.ID] = execTask
	e.mu.Unlock()

	defer func() {
		e.mu.Lock()
		delete(e.executing, task.ID)
		e.mu.Unlock()
	}()

	e.logger.WithFields(logrus.Fields{
		"task_id": task.ID,
		"type":    task.OriginalTask.Type,
	}).Info("Executing task")

	// Execute handler
	result, err := handler(execCtx, json.RawMessage("{}"))
	if err != nil {
		return nil, err
	}

	// Generate verification hash
	hash := sha256.Sum256(result)
	verificationHash := hex.EncodeToString(hash[:])

	// Create result
	resultPayload := &protocol.TaskResultPayload{
		TaskID:           task.ID,
		ExecutorID:       e.identity.GetPeerIDString(),
		Status:           protocol.TaskStatusCompleted,
		Result:           result,
		CompletionTime:   time.Now().UnixNano(),
		ExecutionCost:    e.calculateCost(task),
		VerificationHash: verificationHash,
	}

	// Sign result
	sig := e.identity.Sign([]byte(task.ID + verificationHash))
	resultPayload.Signature = identity.EncodeToString(sig)

	e.logger.WithFields(logrus.Fields{
		"task_id": task.ID,
		"duration": time.Since(execTask.StartTime),
	}).Info("Task executed")

	return resultPayload, nil
}

// Cancel cancels a task
func (e *Executor) Cancel(taskID string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	task, exists := e.executing[taskID]
	if !exists {
		return fmt.Errorf("task not found")
	}

	if task.CancelFunc != nil {
		task.CancelFunc()
	}

	delete(e.executing, taskID)
	return nil
}

// handleOpenClawPrompt handles OpenClaw prompts
func (e *Executor) handleOpenClawPrompt(ctx context.Context, data json.RawMessage) (json.RawMessage, error) {
	if e.openclaw == nil {
		return nil, fmt.Errorf("OpenClaw not enabled")
	}

	var prompt struct {
		Prompt      string  `json:"prompt"`
		Context     string  `json:"context,omitempty"`
		MaxTokens   int     `json:"max_tokens"`
		Temperature float64 `json:"temperature"`
		Model       string  `json:"model"`
	}

	if err := json.Unmarshal(data, &prompt); err != nil {
		return nil, fmt.Errorf("failed to unmarshal prompt: %w", err)
	}

	// Use defaults if not specified
	if prompt.MaxTokens == 0 {
		prompt.MaxTokens = e.cfg.OpenClaw.MaxTokens
	}
	if prompt.Temperature == 0 {
		prompt.Temperature = e.cfg.OpenClaw.Temperature
	}
	if prompt.Model == "" {
		prompt.Model = e.cfg.OpenClaw.Model
	}

	req := &openclaw.Request{
		Prompt:      prompt.Prompt,
		Context:     prompt.Context,
		MaxTokens:   prompt.MaxTokens,
		Temperature: prompt.Temperature,
		Model:       prompt.Model,
	}

	resp, err := e.openclaw.Complete(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("OpenClaw completion failed: %w", err)
	}

	result := struct {
		Response     string `json:"response"`
		TokensUsed   int    `json:"tokens_used"`
		FinishReason string `json:"finish_reason"`
	}{
		Response:     resp.Text,
		TokensUsed:   resp.TokensUsed,
		FinishReason: resp.FinishReason,
	}

	return json.Marshal(result)
}

// handleShellTask handles shell command tasks
func (e *Executor) handleShellTask(ctx context.Context, data json.RawMessage) (json.RawMessage, error) {
	var cmd struct {
		Command string   `json:"command"`
		Args    []string `json:"args,omitempty"`
		Timeout int      `json:"timeout_seconds,omitempty"`
		WorkDir string   `json:"work_dir,omitempty"`
		Env     []string `json:"env,omitempty"`
	}

	if err := json.Unmarshal(data, &cmd); err != nil {
		return nil, fmt.Errorf("failed to unmarshal command: %w", err)
	}

	// Security: Validate command
	if !e.isCommandAllowed(cmd.Command) {
		return nil, fmt.Errorf("command not allowed: %s", cmd.Command)
	}

	// Set timeout
	timeout := 30 * time.Second
	if cmd.Timeout > 0 {
		timeout = time.Duration(cmd.Timeout) * time.Second
	}

	execCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// Execute command
	command := exec.CommandContext(execCtx, cmd.Command, cmd.Args...)
	if cmd.WorkDir != "" {
		command.Dir = cmd.WorkDir
	}
	if len(cmd.Env) > 0 {
		command.Env = cmd.Env
	}

	var stdout, stderr bytes.Buffer
	command.Stdout = &stdout
	command.Stderr = &stderr

	err := command.Run()

	result := struct {
		Stdout   string `json:"stdout"`
		Stderr   string `json:"stderr"`
		ExitCode int    `json:"exit_code"`
		Success  bool   `json:"success"`
	}{
		Stdout:  stdout.String(),
		Stderr:  stderr.String(),
		Success: err == nil,
	}

	if exitErr, ok := err.(*exec.ExitError); ok {
		result.ExitCode = exitErr.ExitCode()
	}

	return json.Marshal(result)
}

// handleComputeTask handles compute tasks
func (e *Executor) handleComputeTask(ctx context.Context, data json.RawMessage) (json.RawMessage, error) {
	var compute struct {
		Operation string          `json:"operation"`
		Input     json.RawMessage `json:"input"`
	}

	if err := json.Unmarshal(data, &compute); err != nil {
		return nil, fmt.Errorf("failed to unmarshal compute task: %w", err)
	}

	var result interface{}

	switch compute.Operation {
	case "hash":
		var input struct {
			Data string `json:"data"`
			Algo string `json:"algo"`
		}
		if err := json.Unmarshal(compute.Input, &input); err != nil {
			return nil, err
		}
		hash := sha256.Sum256([]byte(input.Data))
		result = hex.EncodeToString(hash[:])

	case "sort":
		var input struct {
			Data []string `json:"data"`
		}
		if err := json.Unmarshal(compute.Input, &input); err != nil {
			return nil, err
		}
		// Simple bubble sort for demonstration
		for i := 0; i < len(input.Data); i++ {
			for j := i + 1; j < len(input.Data); j++ {
				if input.Data[i] > input.Data[j] {
					input.Data[i], input.Data[j] = input.Data[j], input.Data[i]
				}
			}
		}
		result = input.Data

	default:
		return nil, fmt.Errorf("unknown compute operation: %s", compute.Operation)
	}

	return json.Marshal(result)
}

// handleStorageTask handles storage tasks
func (e *Executor) handleStorageTask(ctx context.Context, data json.RawMessage) (json.RawMessage, error) {
	var storage struct {
		Operation string `json:"operation"`
		Key       string `json:"key"`
		Value     string `json:"value,omitempty"`
	}

	if err := json.Unmarshal(data, &storage); err != nil {
		return nil, fmt.Errorf("failed to unmarshal storage task: %w", err)
	}

	// This would integrate with the memory module
	result := struct {
		Success bool   `json:"success"`
		Key     string `json:"key"`
		Value   string `json:"value,omitempty"`
	}{
		Success: true,
		Key:     storage.Key,
	}

	return json.Marshal(result)
}

// isCommandAllowed checks if a command is allowed
func (e *Executor) isCommandAllowed(cmd string) bool {
	// Whitelist of allowed commands
	allowed := []string{
		"echo", "cat", "grep", "awk", "sed", "wc", "sort", "uniq",
		"head", "tail", "cut", "tr", "find", "ls", "pwd", "whoami",
		"date", "hostname", "uname", "ps", "top", "df", "du",
	}

	cmd = strings.TrimSpace(cmd)
	for _, allowedCmd := range allowed {
		if cmd == allowedCmd || strings.HasPrefix(cmd, allowedCmd+" ") {
			return true
		}
	}

	return false
}

// calculateCost calculates execution cost
func (e *Executor) calculateCost(task *market.ScheduledTask) float64 {
	// Base cost by type
	baseCost := 0.0
	switch task.OriginalTask.Type {
	case protocol.TaskTypeOpenClawPrompt:
		baseCost = 1.0
	case protocol.TaskTypeShellTask:
		baseCost = 0.5
	case protocol.TaskTypeCompute:
		baseCost = 0.2
	case protocol.TaskTypeStorage:
		baseCost = 0.1
	default:
		baseCost = 0.5
	}

	// Adjust by execution time
	elapsed := time.Since(task.StartedAt)
	if elapsed > time.Minute {
		baseCost *= 1.5
	}

	return baseCost
}

// GetExecutingTasks returns currently executing tasks
func (e *Executor) GetExecutingTasks() []*ExecutingTask {
	e.mu.RLock()
	defer e.mu.RUnlock()

	tasks := make([]*ExecutingTask, 0, len(e.executing))
	for _, task := range e.executing {
		tasks = append(tasks, task)
	}
	return tasks
}

// Close closes the executor
func (e *Executor) Close() error {
	// Cancel all executing tasks
	e.mu.Lock()
	for _, task := range e.executing {
		if task.CancelFunc != nil {
			task.CancelFunc()
		}
	}
	e.mu.Unlock()

	return nil
}
