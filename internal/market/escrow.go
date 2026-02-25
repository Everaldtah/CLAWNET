package market

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/Everaldtah/CLAWNET/internal/config"
	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
)

// EscrowStatus represents the status of an escrow
type EscrowStatus string

const (
	EscrowStatusPending    EscrowStatus = "pending"
	EscrowStatusLocked     EscrowStatus = "locked"
	EscrowStatusReleased   EscrowStatus = "released"
	EscrowStatusRefunded   EscrowStatus = "refunded"
	EscrowStatusDisputed   EscrowStatus = "disputed"
	EscrowStatusExpired    EscrowStatus = "expired"
)

// Escrow represents an escrow transaction
type Escrow struct {
	mu sync.RWMutex

	ID           string
	TaskID       string
	AuctionID    string
	RequesterID  string
	ExecutorID   string
	Amount       float64
	Status       EscrowStatus
	CreatedAt    time.Time
	LockedAt     time.Time
	ReleasedAt   time.Time
	ExpiryTime   time.Time
	ReleaseConditions EscrowConditions
}

// EscrowConditions defines conditions for escrow release
type EscrowConditions struct {
	CompletionRequired bool
	VerificationRequired bool
	Timeout            time.Duration
	ConsensusRequired  bool
	ConsensusThreshold int
}

// EscrowManager manages escrow transactions
type EscrowManager struct {
	mu sync.RWMutex

	cfg      *config.Config
	wallet   *Wallet
	escrows  map[string]*Escrow
	logger   *logrus.Entry

	// Channels
	lockChan     chan *Escrow
	releaseChan  chan *Escrow
	refundChan   chan *Escrow

	ctx    context.Context
	cancel context.CancelFunc
}

// NewEscrowManager creates a new escrow manager
func NewEscrowManager(cfg *config.Config, wallet *Wallet, logger *logrus.Logger) *EscrowManager {
	ctx, cancel := context.WithCancel(context.Background())

	em := &EscrowManager{
		cfg:         cfg,
		wallet:      wallet,
		escrows:     make(map[string]*Escrow),
		logger:      logger.WithField("component", "escrow"),
		lockChan:    make(chan *Escrow, 100),
		releaseChan: make(chan *Escrow, 100),
		refundChan:  make(chan *Escrow, 100),
		ctx:         ctx,
		cancel:      cancel,
	}

	// Start escrow processor
	go em.processEscrows()
	go em.monitorExpirations()

	return em
}

// CreateEscrow creates a new escrow
func (em *EscrowManager) CreateEscrow(taskID, requesterID string, amount float64, conditions EscrowConditions) (*Escrow, error) {
	em.mu.Lock()
	defer em.mu.Unlock()

	// Verify requester has sufficient balance
	requesterBalance := em.wallet.GetBalance(requesterID)
	if requesterBalance < amount {
		return nil, fmt.Errorf("insufficient balance: have %.2f, need %.2f", requesterBalance, amount)
	}

	// Lock the funds
	if err := em.wallet.LockFunds(requesterID, amount); err != nil {
		return nil, fmt.Errorf("failed to lock funds: %w", err)
	}

	escrow := &Escrow{
		ID:          uuid.New().String(),
		TaskID:      taskID,
		RequesterID: requesterID,
		Amount:      amount,
		Status:      EscrowStatusPending,
		CreatedAt:   time.Now(),
		ExpiryTime:  time.Now().Add(conditions.Timeout),
		ReleaseConditions: conditions,
	}

	em.escrows[escrow.ID] = escrow

	em.logger.WithFields(logrus.Fields{
		"escrow_id":    escrow.ID,
		"task_id":      taskID,
		"amount":       amount,
		"requester":    requesterID,
	}).Info("Escrow created")

	return escrow, nil
}

// LockEscrow locks an escrow for execution
func (em *EscrowManager) LockEscrow(escrowID, executorID string) error {
	em.mu.Lock()
	defer em.mu.Unlock()

	escrow, exists := em.escrows[escrowID]
	if !exists {
		return fmt.Errorf("escrow not found")
	}

	escrow.mu.Lock()
	defer escrow.mu.Unlock()

	if escrow.Status != EscrowStatusPending {
		return fmt.Errorf("escrow not in pending state")
	}

	escrow.ExecutorID = executorID
	escrow.Status = EscrowStatusLocked
	escrow.LockedAt = time.Now()

	em.logger.WithFields(logrus.Fields{
		"escrow_id": escrowID,
		"executor":  executorID,
	}).Info("Escrow locked")

	return nil
}

// ReleaseEscrow releases escrow to executor
func (em *EscrowManager) ReleaseEscrow(escrowID string, verificationData []byte) error {
	em.mu.Lock()
	defer em.mu.Unlock()

	escrow, exists := em.escrows[escrowID]
	if !exists {
		return fmt.Errorf("escrow not found")
	}

	escrow.mu.Lock()
	defer escrow.mu.Unlock()

	if escrow.Status != EscrowStatusLocked {
		return fmt.Errorf("escrow not in locked state")
	}

	// Verify completion if required
	if escrow.ReleaseConditions.VerificationRequired {
		if !em.verifyCompletion(escrow, verificationData) {
			return fmt.Errorf("completion verification failed")
		}
	}

	// Transfer funds to executor
	if err := em.wallet.Transfer(escrow.RequesterID, escrow.ExecutorID, escrow.Amount); err != nil {
		return fmt.Errorf("failed to transfer funds: %w", err)
	}

	escrow.Status = EscrowStatusReleased
	escrow.ReleasedAt = time.Now()

	em.logger.WithFields(logrus.Fields{
		"escrow_id": escrowID,
		"executor":  escrow.ExecutorID,
		"amount":    escrow.Amount,
	}).Info("Escrow released")

	return nil
}

// RefundEscrow refunds escrow to requester
func (em *EscrowManager) RefundEscrow(escrowID string, partialAmount float64) error {
	em.mu.Lock()
	defer em.mu.Unlock()

	escrow, exists := em.escrows[escrowID]
	if !exists {
		return fmt.Errorf("escrow not found")
	}

	escrow.mu.Lock()
	defer escrow.mu.Unlock()

	if escrow.Status != EscrowStatusPending && escrow.Status != EscrowStatusLocked {
		return fmt.Errorf("escrow cannot be refunded")
	}

	refundAmount := escrow.Amount
	if partialAmount > 0 && partialAmount < escrow.Amount {
		refundAmount = partialAmount
	}

	// Unlock/return funds to requester
	if escrow.Status == EscrowStatusPending {
		if err := em.wallet.UnlockFunds(escrow.RequesterID, refundAmount); err != nil {
			return fmt.Errorf("failed to unlock funds: %w", err)
		}
	} else {
		// Return from locked state
		if err := em.wallet.Deposit(escrow.RequesterID, refundAmount); err != nil {
			return fmt.Errorf("failed to refund: %w", err)
		}
	}

	escrow.Status = EscrowStatusRefunded

	em.logger.WithFields(logrus.Fields{
		"escrow_id":     escrowID,
		"requester":     escrow.RequesterID,
		"refund_amount": refundAmount,
	}).Info("Escrow refunded")

	return nil
}

// DisputeEscrow marks an escrow as disputed
func (em *EscrowManager) DisputeEscrow(escrowID, reason string) error {
	em.mu.Lock()
	defer em.mu.Unlock()

	escrow, exists := em.escrows[escrowID]
	if !exists {
		return fmt.Errorf("escrow not found")
	}

	escrow.mu.Lock()
	defer escrow.mu.Unlock()

	if escrow.Status != EscrowStatusLocked {
		return fmt.Errorf("escrow not in locked state")
	}

	escrow.Status = EscrowStatusDisputed

	em.logger.WithFields(logrus.Fields{
		"escrow_id": escrowID,
		"reason":    reason,
	}).Warn("Escrow disputed")

	return nil
}

// ResolveDispute resolves a disputed escrow
func (em *EscrowManager) ResolveDispute(escrowID string, releaseToExecutor bool, partialAmount float64) error {
	em.mu.Lock()
	defer em.mu.Unlock()

	escrow, exists := em.escrows[escrowID]
	if !exists {
		return fmt.Errorf("escrow not found")
	}

	escrow.mu.Lock()
	defer escrow.mu.Unlock()

	if escrow.Status != EscrowStatusDisputed {
		return fmt.Errorf("escrow not in disputed state")
	}

	if releaseToExecutor {
		amount := escrow.Amount
		if partialAmount > 0 && partialAmount < escrow.Amount {
			amount = partialAmount
		}

		if err := em.wallet.Transfer(escrow.RequesterID, escrow.ExecutorID, amount); err != nil {
			return fmt.Errorf("failed to transfer to executor: %w", err)
		}

		// Refund remainder to requester
		if amount < escrow.Amount {
			if err := em.wallet.Deposit(escrow.RequesterID, escrow.Amount-amount); err != nil {
				return fmt.Errorf("failed to refund remainder: %w", err)
			}
		}

		escrow.Status = EscrowStatusReleased
	} else {
		// Full refund to requester
		if err := em.wallet.Deposit(escrow.RequesterID, escrow.Amount); err != nil {
			return fmt.Errorf("failed to refund requester: %w", err)
		}
		escrow.Status = EscrowStatusRefunded
	}

	em.logger.WithFields(logrus.Fields{
		"escrow_id":         escrowID,
		"release_to_executor": releaseToExecutor,
		"partial_amount":    partialAmount,
	}).Info("Dispute resolved")

	return nil
}

// GetEscrow gets an escrow by ID
func (em *EscrowManager) GetEscrow(escrowID string) (*Escrow, error) {
	em.mu.RLock()
	defer em.mu.RUnlock()

	escrow, exists := em.escrows[escrowID]
	if !exists {
		return nil, fmt.Errorf("escrow not found")
	}

	return escrow, nil
}

// GetEscrowsByTask gets escrows by task ID
func (em *EscrowManager) GetEscrowsByTask(taskID string) []*Escrow {
	em.mu.RLock()
	defer em.mu.RUnlock()

	var result []*Escrow
	for _, escrow := range em.escrows {
		if escrow.TaskID == taskID {
			result = append(result, escrow)
		}
	}
	return result
}

// GetPendingEscrows returns all pending escrows
func (em *EscrowManager) GetPendingEscrows() []*Escrow {
	em.mu.RLock()
	defer em.mu.RUnlock()

	var result []*Escrow
	for _, escrow := range em.escrows {
		escrow.mu.RLock()
		status := escrow.Status
		escrow.mu.RUnlock()

		if status == EscrowStatusPending || status == EscrowStatusLocked {
			result = append(result, escrow)
		}
	}
	return result
}

// verifyCompletion verifies task completion
func (em *EscrowManager) verifyCompletion(escrow *Escrow, verificationData []byte) bool {
	// In a real implementation, this would verify:
	// - Task result signature
	// - Completion hash
	// - Consensus from multiple nodes (if required)
	// - Output schema validation

	if escrow.ReleaseConditions.ConsensusRequired {
		// Would check for consensus signatures
		return len(verificationData) > 0
	}

	return true
}

// monitorExpirations monitors for expired escrows
func (em *EscrowManager) monitorExpirations() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			em.checkExpirations()
		case <-em.ctx.Done():
			return
		}
	}
}

// checkExpirations checks for and handles expired escrows
func (em *EscrowManager) checkExpirations() {
	em.mu.RLock()
	escrows := make([]*Escrow, 0, len(em.escrows))
	for _, e := range em.escrows {
		escrows = append(escrows, e)
	}
	em.mu.RUnlock()

	now := time.Now()
	for _, escrow := range escrows {
		escrow.mu.RLock()
		status := escrow.Status
		expiry := escrow.ExpiryTime
		escrow.mu.RUnlock()

		if (status == EscrowStatusPending || status == EscrowStatusLocked) && now.After(expiry) {
			em.logger.WithFields(logrus.Fields{
				"escrow_id": escrow.ID,
				"task_id":   escrow.TaskID,
			}).Warn("Escrow expired")

			// Auto-refund expired escrows
			if err := em.RefundEscrow(escrow.ID, 0); err != nil {
				em.logger.WithError(err).Error("Failed to refund expired escrow")
			}
		}
	}
}

// processEscrows processes escrow events
func (em *EscrowManager) processEscrows() {
	for {
		select {
		case escrow := <-em.lockChan:
			em.logger.WithField("escrow_id", escrow.ID).Debug("Processing escrow lock")
		case escrow := <-em.releaseChan:
			em.logger.WithField("escrow_id", escrow.ID).Debug("Processing escrow release")
		case escrow := <-em.refundChan:
			em.logger.WithField("escrow_id", escrow.ID).Debug("Processing escrow refund")
		case <-em.ctx.Done():
			return
		}
	}
}

// Stop stops the escrow manager
func (em *EscrowManager) Stop() {
	em.cancel()
}
