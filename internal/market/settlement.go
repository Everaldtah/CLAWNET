package market

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/Everaldtah/CLAWNET/internal/config"
	"github.com/Everaldtah/CLAWNET/internal/protocol"
	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
)

// SettlementManager manages task settlement
type SettlementManager struct {
	mu sync.RWMutex

	cfg           *config.Config
	wallet        *Wallet
	escrowManager *EscrowManager
	reputation    *ReputationManager
	logger        *logrus.Entry

	// Settlement tracking
	settlements   map[string]*Settlement
	pendingChan   chan *Settlement

	// Fraud detection
	fraudDetector *FraudDetector

	ctx           context.Context
	cancel        context.CancelFunc
}

// Settlement represents a task settlement
type Settlement struct {
	mu sync.RWMutex

	ID               string
	TaskID           string
	EscrowID         string
	ExecutorID       string
	RequesterID      string
	Amount           float64
	Status           protocol.SettlementStatus
	VerificationData json.RawMessage
	SettlementTime   time.Time
	RefundAmount     float64
	ConsensusVotes   map[string]bool
	FraudScore       float64
}

// NewSettlementManager creates a new settlement manager
func NewSettlementManager(cfg *config.Config, wallet *Wallet, escrowManager *EscrowManager, 
	reputation *ReputationManager, logger *logrus.Logger) *SettlementManager {
	ctx, cancel := context.WithCancel(context.Background())

	sm := &SettlementManager{
		cfg:           cfg,
		wallet:        wallet,
		escrowManager: escrowManager,
		reputation:    reputation,
		logger:        logger.WithField("component", "settlement"),
		settlements:   make(map[string]*Settlement),
		pendingChan:   make(chan *Settlement, 1000),
		fraudDetector: NewFraudDetector(cfg),
		ctx:           ctx,
		cancel:        cancel,
	}

	// Start settlement processor
	go sm.processSettlements()

	return sm
}

// CreateSettlement creates a new settlement
func (sm *SettlementManager) CreateSettlement(taskID, escrowID, executorID, requesterID string, 
	amount float64, verificationData json.RawMessage) (*Settlement, error) {
	
	sm.mu.Lock()
	defer sm.mu.Unlock()

	// Verify escrow exists and is locked
	escrow, err := sm.escrowManager.GetEscrow(escrowID)
	if err != nil {
		return nil, fmt.Errorf("escrow not found: %w", err)
	}

	escrow.mu.RLock()
	escrowStatus := escrow.Status
	escrow.mu.RUnlock()

	if escrowStatus != EscrowStatusLocked {
		return nil, fmt.Errorf("escrow not in locked state")
	}

	settlement := &Settlement{
		ID:               uuid.New().String(),
		TaskID:           taskID,
		EscrowID:         escrowID,
		ExecutorID:       executorID,
		RequesterID:      requesterID,
		Amount:           amount,
		Status:           protocol.SettlementStatusPending,
		VerificationData: verificationData,
		ConsensusVotes:   make(map[string]bool),
	}

	// Run fraud detection
	if sm.cfg.Market.FraudDetection {
		settlement.FraudScore = sm.fraudDetector.Analyze(taskID, executorID, verificationData)
		if settlement.FraudScore > 0.8 {
			sm.logger.WithFields(logrus.Fields{
				"settlement_id": settlement.ID,
				"fraud_score":   settlement.FraudScore,
			}).Warn("High fraud score detected")
		}
	}

	sm.settlements[settlement.ID] = settlement

	sm.logger.WithFields(logrus.Fields{
		"settlement_id": settlement.ID,
		"task_id":       taskID,
		"executor":      executorID,
		"amount":        amount,
	}).Info("Settlement created")

	// Add to processing queue
	select {
	case sm.pendingChan <- settlement:
	default:
	}

	return settlement, nil
}

// ProcessSettlement processes a settlement
func (sm *SettlementManager) ProcessSettlement(settlementID string) error {
	sm.mu.RLock()
	settlement, exists := sm.settlements[settlementID]
	sm.mu.RUnlock()

	if !exists {
		return fmt.Errorf("settlement not found")
	}

	settlement.mu.Lock()
	defer settlement.mu.Unlock()

	if settlement.Status != protocol.SettlementStatusPending {
		return fmt.Errorf("settlement not in pending state")
	}

	// Check consensus if required
	if sm.cfg.Market.ConsensusMode {
		if !sm.checkConsensus(settlement) {
			sm.logger.WithField("settlement_id", settlementID).Debug("Waiting for consensus")
			return nil
		}
	}

	// Check fraud score
	if settlement.FraudScore > 0.9 {
		settlement.Status = protocol.SettlementStatusDisputed
		sm.escrowManager.DisputeEscrow(settlement.EscrowID, "fraud detected")
		return fmt.Errorf("settlement flagged for fraud")
	}

	// Release escrow
	if err := sm.escrowManager.ReleaseEscrow(settlement.EscrowID, settlement.VerificationData); err != nil {
		return fmt.Errorf("failed to release escrow: %w", err)
	}

	// Transfer funds to executor
	if err := sm.wallet.Transfer(settlement.RequesterID, settlement.ExecutorID, settlement.Amount); err != nil {
		return fmt.Errorf("failed to transfer funds: %w", err)
	}

	settlement.Status = protocol.SettlementStatusCompleted
	settlement.SettlementTime = time.Now()

	// Update executor reputation
	sm.reputation.RecordTaskCompletion(settlement.ExecutorID, true, true)

	// Add reward
	sm.wallet.AddReward(settlement.TaskID, settlement.Amount*0.1, "task completion reward")

	sm.logger.WithFields(logrus.Fields{
		"settlement_id": settlementID,
		"executor":      settlement.ExecutorID,
		"amount":        settlement.Amount,
	}).Info("Settlement completed")

	return nil
}

// checkConsensus checks if consensus is reached
func (sm *SettlementManager) checkConsensus(settlement *Settlement) bool {
	if len(settlement.ConsensusVotes) < sm.cfg.Market.ConsensusThreshold {
		return false
	}

	positiveVotes := 0
	for _, vote := range settlement.ConsensusVotes {
		if vote {
			positiveVotes++
		}
	}

	// Require majority of positive votes
	return positiveVotes >= sm.cfg.Market.ConsensusThreshold/2+1
}

// VoteConsensus adds a consensus vote
func (sm *SettlementManager) VoteConsensus(settlementID, voterID string, approve bool) error {
	sm.mu.RLock()
	settlement, exists := sm.settlements[settlementID]
	sm.mu.RUnlock()

	if !exists {
		return fmt.Errorf("settlement not found")
	}

	settlement.mu.Lock()
	defer settlement.mu.Unlock()

	if settlement.Status != protocol.SettlementStatusPending {
		return fmt.Errorf("settlement not in pending state")
	}

	settlement.ConsensusVotes[voterID] = approve

	sm.logger.WithFields(logrus.Fields{
		"settlement_id": settlementID,
		"voter":         voterID,
		"approve":       approve,
		"vote_count":    len(settlement.ConsensusVotes),
	}).Debug("Consensus vote recorded")

	return nil
}

// DisputeSettlement disputes a settlement
func (sm *SettlementManager) DisputeSettlement(settlementID, reason string, evidence json.RawMessage) error {
	sm.mu.RLock()
	settlement, exists := sm.settlements[settlementID]
	sm.mu.RUnlock()

	if !exists {
		return fmt.Errorf("settlement not found")
	}

	settlement.mu.Lock()
	defer settlement.mu.Unlock()

	if settlement.Status != protocol.SettlementStatusPending && 
	   settlement.Status != protocol.SettlementStatusCompleted {
		return fmt.Errorf("settlement cannot be disputed")
	}

	settlement.Status = protocol.SettlementStatusDisputed

	// Dispute the escrow
	if err := sm.escrowManager.DisputeEscrow(settlement.EscrowID, reason); err != nil {
		return fmt.Errorf("failed to dispute escrow: %w", err)
	}

	// Penalize executor reputation
	sm.reputation.RecordDispute(settlement.ExecutorID)

	sm.logger.WithFields(logrus.Fields{
		"settlement_id": settlementID,
		"reason":        reason,
		"executor":      settlement.ExecutorID,
	}).Warn("Settlement disputed")

	return nil
}

// ResolveDispute resolves a disputed settlement
func (sm *SettlementManager) ResolveDispute(settlementID string, releaseToExecutor bool, partialAmount float64) error {
	sm.mu.RLock()
	settlement, exists := sm.settlements[settlementID]
	sm.mu.RUnlock()

	if !exists {
		return fmt.Errorf("settlement not found")
	}

	settlement.mu.Lock()
	defer settlement.mu.Unlock()

	if settlement.Status != protocol.SettlementStatusDisputed {
		return fmt.Errorf("settlement not in disputed state")
	}

	// Resolve escrow
	if err := sm.escrowManager.ResolveDispute(settlement.EscrowID, releaseToExecutor, partialAmount); err != nil {
		return fmt.Errorf("failed to resolve escrow: %w", err)
	}

	if releaseToExecutor {
		amount := settlement.Amount
		if partialAmount > 0 && partialAmount < settlement.Amount {
			amount = partialAmount
		}

		// Transfer to executor
		if err := sm.wallet.Transfer(settlement.RequesterID, settlement.ExecutorID, amount); err != nil {
			return fmt.Errorf("failed to transfer to executor: %w", err)
		}

		settlement.RefundAmount = settlement.Amount - amount
		settlement.Status = protocol.SettlementStatusCompleted
	} else {
		// Full refund to requester
		settlement.RefundAmount = settlement.Amount
		settlement.Status = protocol.SettlementStatusRefunded
	}

	settlement.SettlementTime = time.Now()

	sm.logger.WithFields(logrus.Fields{
		"settlement_id":      settlementID,
		"release_to_executor": releaseToExecutor,
		"partial_amount":     partialAmount,
	}).Info("Dispute resolved")

	return nil
}

// GetSettlement gets a settlement by ID
func (sm *SettlementManager) GetSettlement(settlementID string) (*Settlement, error) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	settlement, exists := sm.settlements[settlementID]
	if !exists {
		return nil, fmt.Errorf("settlement not found")
	}

	return settlement, nil
}

// GetSettlementsByTask gets settlements by task ID
func (sm *SettlementManager) GetSettlementsByTask(taskID string) []*Settlement {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	var result []*Settlement
	for _, settlement := range sm.settlements {
		if settlement.TaskID == taskID {
			result = append(result, settlement)
		}
	}
	return result
}

// processSettlements processes pending settlements
func (sm *SettlementManager) processSettlements() {
	for {
		select {
		case settlement := <-sm.pendingChan:
			if err := sm.ProcessSettlement(settlement.ID); err != nil {
				sm.logger.WithError(err).WithField("settlement_id", settlement.ID).Error("Settlement processing failed")
			}
		case <-sm.ctx.Done():
			return
		}
	}
}

// Stop stops the settlement manager
func (sm *SettlementManager) Stop() {
	sm.cancel()
}

// FraudDetector detects fraudulent activity
type FraudDetector struct {
	cfg *config.Config

	// Historical data
	executionTimes map[string][]time.Duration
	resultPatterns map[string]map[string]int
}

// NewFraudDetector creates a new fraud detector
func NewFraudDetector(cfg *config.Config) *FraudDetector {
	return &FraudDetector{
		cfg:            cfg,
		executionTimes: make(map[string][]time.Duration),
		resultPatterns: make(map[string]map[string]int),
	}
}

// Analyze analyzes a result for fraud
func (fd *FraudDetector) Analyze(taskID, executorID string, verificationData json.RawMessage) float64 {
	fraudScore := 0.0

	// Check for duplicate results
	dataHash := hashData(verificationData)
	if patterns, exists := fd.resultPatterns[executorID]; exists {
		if count, exists := patterns[dataHash]; exists && count > 2 {
			fraudScore += 0.3 // Penalty for duplicate results
		}
	}

	// Record pattern
	if fd.resultPatterns[executorID] == nil {
		fd.resultPatterns[executorID] = make(map[string]int)
	}
	fd.resultPatterns[executorID][dataHash]++

	// Check execution time anomalies
	// Would compare against historical data

	return fraudScore
}

// hashData creates a simple hash of data
func hashData(data []byte) string {
	// Simple hash for demonstration
	if len(data) == 0 {
		return ""
	}
	sum := 0
	for _, b := range data {
		sum += int(b)
	}
	return fmt.Sprintf("%d", sum)
}
