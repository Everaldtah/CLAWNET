package market

import (
	"context"
	"fmt"
	"sync"

	"github.com/Everaldtah/CLAWNET/internal/config"
	"github.com/Everaldtah/CLAWNET/internal/identity"
	"github.com/Everaldtah/CLAWNET/internal/protocol"
	"github.com/sirupsen/logrus"
)

// MarketManager manages the entire market system
type MarketManager struct {
	mu sync.RWMutex

	cfg              *config.Config
	identity         *identity.Identity
	logger           *logrus.Entry

	// Sub-managers
	AuctionManager   *AuctionManager
	BidManager       *BidManager
	EscrowManager    *EscrowManager
	PricingEngine    *PricingEngine
	ReputationManager *ReputationManager
	Scheduler        *TaskScheduler
	SettlementManager *SettlementManager
	Wallet           *Wallet

	// Market state
	enabled          bool
	activeTasks      map[string]*protocol.MarketTaskAnnouncePayload
	activeBids       map[string]*protocol.MarketBidPayload

	ctx              context.Context
	cancel           context.CancelFunc
}

// NewMarketManager creates a new market manager
func NewMarketManager(cfg *config.Config, identity *identity.Identity, logger *logrus.Logger) (*MarketManager, error) {
	if !cfg.Market.Enabled {
		return &MarketManager{enabled: false}, nil
	}

	ctx, cancel := context.WithCancel(context.Background())

	// Create wallet
	wallet, err := NewWallet(identity.GetPeerIDString(), cfg)
	if err != nil {
		cancel()
		return nil, fmt.Errorf("failed to create wallet: %w", err)
	}

	// Create reputation manager
	reputation, err := NewReputationManager(identity.GetPeerIDString(), cfg)
	if err != nil {
		wallet.Close()
		cancel()
		return nil, fmt.Errorf("failed to create reputation manager: %w", err)
	}

	// Create pricing engine
	pricing := NewPricingEngine(cfg)

	// Create escrow manager
	escrow := NewEscrowManager(cfg, wallet, logger)

	// Create auction manager
	auction := NewAuctionManager(cfg, reputation, escrow, logger)

	// Create bid manager
	bid := NewBidManager(cfg, reputation, logger)

	// Create scheduler
	scheduler := NewTaskScheduler(cfg, auction, escrow, reputation, logger)

	// Create settlement manager
	settlement := NewSettlementManager(cfg, wallet, escrow, reputation, logger)

	mm := &MarketManager{
		cfg:               cfg,
		identity:          identity,
		logger:            logger.WithField("component", "market"),
		AuctionManager:    auction,
		BidManager:        bid,
		EscrowManager:     escrow,
		PricingEngine:     pricing,
		ReputationManager: reputation,
		Scheduler:         scheduler,
		SettlementManager: settlement,
		Wallet:            wallet,
		enabled:           true,
		activeTasks:       make(map[string]*protocol.MarketTaskAnnouncePayload),
		activeBids:        make(map[string]*protocol.MarketBidPayload),
		ctx:               ctx,
		cancel:            cancel,
	}

	return mm, nil
}

// SubmitTask submits a task to the market
func (mm *MarketManager) SubmitTask(task *protocol.MarketTaskAnnouncePayload) (*ScheduledTask, error) {
	if !mm.enabled {
		return nil, fmt.Errorf("market not enabled")
	}

	mm.mu.Lock()
	defer mm.mu.Unlock()

	// Validate task
	if task.MaxBudget <= 0 {
		return nil, fmt.Errorf("max budget must be positive")
	}

	if task.Deadline <= 0 {
		return nil, fmt.Errorf("deadline must be set")
	}

	// Check if we have sufficient balance if we're the requester
	if task.RequesterID == mm.identity.GetPeerIDString() {
		if mm.Wallet.GetBalance(mm.identity.GetPeerIDString()) < task.MaxBudget {
			return nil, fmt.Errorf("insufficient balance")
		}
	}

	// Store active task
	mm.activeTasks[task.TaskID] = task

	// Submit to scheduler
	scheduledTask, err := mm.Scheduler.SubmitTask(task)
	if err != nil {
		return nil, fmt.Errorf("failed to submit task to scheduler: %w", err)
	}

	mm.logger.WithFields(logrus.Fields{
		"task_id": task.TaskID,
		"type":    task.Type,
		"budget":  task.MaxBudget,
	}).Info("Task submitted to market")

	return scheduledTask, nil
}

// SubmitBid submits a bid for a task
func (mm *MarketManager) SubmitBid(auctionID string, bid *protocol.MarketBidPayload) error {
	if !mm.enabled {
		return fmt.Errorf("market not enabled")
	}

	mm.mu.Lock()
	defer mm.mu.Unlock()

	_, err := mm.AuctionManager.SubmitBid(auctionID, bid)
	if err != nil {
		return err
	}

	mm.activeBids[bid.TaskID] = bid

	return nil
}

// EvaluateAndBid evaluates a task and submits a bid if appropriate
func (mm *MarketManager) EvaluateAndBid(task *protocol.MarketTaskAnnouncePayload) (*protocol.MarketBidPayload, error) {
	if !mm.enabled {
		return nil, fmt.Errorf("market not enabled")
	}

	// Evaluate bid
	bid, err := mm.BidManager.EvaluateBid(task)
	if err != nil {
		return nil, err
	}

	// Sign bid
	sig := mm.identity.Sign([]byte(bid.TaskID + bid.BidderID + fmt.Sprintf("%f", bid.BidPrice)))
	bid.Nonce = identity.EncodeToString(sig)

	return bid, nil
}

// GetWallet returns the wallet
func (mm *MarketManager) GetWallet() *Wallet {
	mm.mu.RLock()
	defer mm.mu.RUnlock()
	return mm.Wallet
}

// GetReputation returns own reputation
func (mm *MarketManager) GetReputation() *Reputation {
	mm.mu.RLock()
	defer mm.mu.RUnlock()
	if mm.ReputationManager == nil {
		return nil
	}
	return mm.ReputationManager.GetOwnReputation()
}

// GetActiveAuctions returns active auctions
func (mm *MarketManager) GetActiveAuctions() []*Auction {
	if !mm.enabled {
		return nil
	}
	return mm.AuctionManager.GetActiveAuctions()
}

// GetActiveTasks returns active tasks
func (mm *MarketManager) GetActiveTasks() []*ScheduledTask {
	if !mm.enabled {
		return nil
	}
	return mm.Scheduler.GetTasksByStatus(protocol.TaskStatusExecuting)
}

// GetTask gets a task by ID
func (mm *MarketManager) GetTask(taskID string) (*ScheduledTask, error) {
	if !mm.enabled {
		return nil, fmt.Errorf("market not enabled")
	}
	return mm.Scheduler.GetTask(taskID)
}

// CancelTask cancels a task
func (mm *MarketManager) CancelTask(taskID string) error {
	if !mm.enabled {
		return fmt.Errorf("market not enabled")
	}
	return mm.Scheduler.CancelTask(taskID)
}

// GetStats returns market statistics
func (mm *MarketManager) GetStats() map[string]interface{} {
	if !mm.enabled {
		return map[string]interface{}{"enabled": false}
	}

	mm.mu.RLock()
	defer mm.mu.RUnlock()

	return map[string]interface{}{
		"enabled":         true,
		"active_tasks":    len(mm.activeTasks),
		"active_bids":     len(mm.activeBids),
		"wallet_balance":  mm.Wallet.GetTotalBalance(),
		"reputation":      mm.ReputationManager.GetOwnReputation().Score,
		"scheduler_stats": mm.Scheduler.GetStats(),
	}
}

// Stop stops the market manager
func (mm *MarketManager) Stop() error {
	if !mm.enabled {
		return nil
	}

	mm.cancel()

	mm.AuctionManager.Stop()
	mm.BidManager.Stop()
	mm.Scheduler.Stop()
	mm.SettlementManager.Stop()
	mm.EscrowManager.Stop()

	mm.Wallet.Close()
	mm.ReputationManager.Close()

	return nil
}

// IsEnabled returns whether market is enabled
func (mm *MarketManager) IsEnabled() bool {
	return mm.enabled
}
