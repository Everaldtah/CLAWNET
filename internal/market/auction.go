package market

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/Everaldtah/CLAWNET/internal/config"
	"github.com/Everaldtah/CLAWNET/internal/protocol"
	"github.com/google/uuid"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/sirupsen/logrus"
)

// Auction represents a task auction
type Auction struct {
	mu sync.RWMutex

	ID              string
	TaskID          string
	Description     string
	TaskType        protocol.TaskType
	MaxBudget       float64
	Deadline        time.Time
	RequiredCaps    []string
	MinReputation   float64
	RequesterID     string
	Status          AuctionStatus
	Bids            map[string]*Bid
	WinnerID        string
	WinningBid      float64
	ConsensusWinners []string
	CreatedAt       time.Time
	BidDeadline     time.Time
	EscrowID        string
	ConsensusMode   bool
	ConsensusThreshold int
}

// AuctionStatus represents auction status
type AuctionStatus string

const (
	AuctionStatusOpen       AuctionStatus = "open"
	AuctionStatusBidding    AuctionStatus = "bidding"
	AuctionStatusEvaluating AuctionStatus = "evaluating"
	AuctionStatusClosed     AuctionStatus = "closed"
	AuctionStatusCancelled  AuctionStatus = "cancelled"
	AuctionStatusTimeout    AuctionStatus = "timeout"
)

// Bid represents a bid in an auction
type Bid struct {
	ID                string
	AuctionID         string
	BidderID          string
	BidPrice          float64
	EstimatedDuration time.Duration
	ConfidenceScore   float64
	CapabilityHash    string
	ReputationScore   float64
	CurrentLoad       float64
	EnergyCost        float64
	Timestamp         time.Time
	Status            protocol.BidStatus
	Score             float64
}

// AuctionManager manages all auctions
type AuctionManager struct {
	mu       sync.RWMutex
	auctions  map[string]*Auction
	cfg      *config.Config
	reputation *ReputationManager
	scheduler  *Scheduler
	escrow     *EscrowManager
	logger     *logrus.Entry

	// Channels
	auctionChan chan *Auction
	bidChan     chan *Bid
	resultChan  chan *AuctionResult

	ctx    context.Context
	cancel context.CancelFunc
}

// AuctionResult represents the result of an auction
type AuctionResult struct {
	AuctionID        string
	TaskID           string
	WinnerID         string
	WinningBid       float64
	Success          bool
	Reason           string
	ConsensusWinners []string
}

// NewAuctionManager creates a new auction manager
func NewAuctionManager(cfg *config.Config, reputation *ReputationManager, escrow *EscrowManager, logger *logrus.Logger) *AuctionManager {
	ctx, cancel := context.WithCancel(context.Background())

	am := &AuctionManager{
		auctions:    make(map[string]*Auction),
		cfg:         cfg,
		reputation:  reputation,
		escrow:      escrow,
		logger:      logger.WithField("component", "auction"),
		auctionChan: make(chan *Auction, 100),
		bidChan:     make(chan *Bid, 1000),
		resultChan:  make(chan *AuctionResult, 100),
		ctx:         ctx,
		cancel:      cancel,
	}

	// Start auction processor
	go am.processAuctions()
	go am.processBids()

	return am
}

// CreateAuction creates a new auction
func (am *AuctionManager) CreateAuction(task *protocol.MarketTaskAnnouncePayload) (*Auction, error) {
	am.mu.Lock()
	defer am.mu.Unlock()

	// Check max concurrent auctions
	if len(am.auctions) >= am.cfg.Market.MaxConcurrentAuctions {
		return nil, fmt.Errorf("max concurrent auctions reached")
	}

	auction := &Auction{
		ID:                 uuid.New().String(),
		TaskID:             task.TaskID,
		Description:        task.Description,
		TaskType:           task.Type,
		MaxBudget:          task.MaxBudget,
		Deadline:           time.Unix(0, task.Deadline),
		RequiredCaps:       task.RequiredCapabilities,
		MinReputation:      task.MinimumReputation,
		RequesterID:        task.RequesterID,
		Status:             AuctionStatusOpen,
		Bids:               make(map[string]*Bid),
		CreatedAt:          time.Now(),
		BidDeadline:        time.Now().Add(time.Duration(task.BidTimeout)),
		ConsensusMode:      task.ConsensusMode,
		ConsensusThreshold: task.ConsensusThreshold,
	}

	am.auctions[auction.ID] = auction

	am.logger.WithFields(logrus.Fields{
		"auction_id": auction.ID,
		"task_id":    task.TaskID,
		"budget":     task.MaxBudget,
	}).Info("Auction created")

	// Start bid collection timer
	go am.collectBids(auction)

	return auction, nil
}

// SubmitBid submits a bid to an auction
func (am *AuctionManager) SubmitBid(auctionID string, bidPayload *protocol.MarketBidPayload) (*Bid, error) {
	am.mu.RLock()
	auction, exists := am.auctions[auctionID]
	am.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("auction not found")
	}

	auction.mu.Lock()
	defer auction.mu.Unlock()

	// Validate auction status
	if auction.Status != AuctionStatusOpen && auction.Status != AuctionStatusBidding {
		return nil, fmt.Errorf("auction not accepting bids")
	}

	// Check bid deadline
	if time.Now().After(auction.BidDeadline) {
		return nil, fmt.Errorf("bidding period ended")
	}

	// Validate minimum reputation
	if bidPayload.ReputationScore < auction.MinReputation {
		return nil, fmt.Errorf("bidder reputation below minimum")
	}

	// Check for duplicate bids
	if _, exists := auction.Bids[bidPayload.BidderID]; exists {
		return nil, fmt.Errorf("bidder already submitted bid")
	}

	bid := &Bid{
		ID:                uuid.New().String(),
		AuctionID:         auctionID,
		BidderID:          bidPayload.BidderID,
		BidPrice:          bidPayload.BidPrice,
		EstimatedDuration: time.Duration(bidPayload.EstimatedDuration) * time.Millisecond,
		ConfidenceScore:   bidPayload.ConfidenceScore,
		CapabilityHash:    bidPayload.CapabilityHash,
		ReputationScore:   bidPayload.ReputationScore,
		CurrentLoad:       bidPayload.CurrentLoad,
		EnergyCost:        bidPayload.EnergyCost,
		Timestamp:         time.Now(),
		Status:            protocol.BidStatusPending,
	}

	// Calculate bid score
	weights := protocol.MarketWeights{
		Price:      am.cfg.Market.Weights.Price,
		Reputation: am.cfg.Market.Weights.Reputation,
		Latency:    am.cfg.Market.Weights.Latency,
		Confidence: am.cfg.Market.Weights.Confidence,
	}
	bid.Score = bidPayload.CalculateScore(weights)

	auction.Bids[bidPayload.BidderID] = bid

	am.logger.WithFields(logrus.Fields{
		"auction_id": auctionID,
		"bidder":     bid.BidderID,
		"price":      bid.BidPrice,
		"score":      bid.Score,
	}).Debug("Bid submitted")

	return bid, nil
}

// collectBids collects bids for the specified duration
func (am *AuctionManager) collectBids(auction *Auction) {
	timer := time.NewTimer(time.Until(auction.BidDeadline))
	defer timer.Stop()

	select {
	case <-timer.C:
		am.closeAuction(auction.ID)
	case <-am.ctx.Done():
		return
	}
}

// closeAuction closes an auction and selects winner(s)
func (am *AuctionManager) closeAuction(auctionID string) {
	am.mu.Lock()
	auction, exists := am.auctions[auctionID]
	if !exists {
		am.mu.Unlock()
		return
	}
	am.mu.Unlock()

	auction.mu.Lock()
	defer auction.mu.Unlock()

	if auction.Status == AuctionStatusCancelled {
		return
	}

	auction.Status = AuctionStatusEvaluating

	// Select winner(s)
	if auction.ConsensusMode {
		am.selectConsensusWinners(auction)
	} else {
		am.selectSingleWinner(auction)
	}
}

// selectSingleWinner selects a single winner
func (am *AuctionManager) selectSingleWinner(auction *Auction) {
	if len(auction.Bids) == 0 {
		auction.Status = AuctionStatusClosed
		am.resultChan <- &AuctionResult{
			AuctionID: auction.ID,
			TaskID:    auction.TaskID,
			Success:   false,
			Reason:    "no bids received",
		}
		return
	}

	// Sort bids by score (highest first)
	bids := make([]*Bid, 0, len(auction.Bids))
	for _, bid := range auction.Bids {
		bids = append(bids, bid)
	}
	sort.Slice(bids, func(i, j int) bool {
		return bids[i].Score > bids[j].Score
	})

	winner := bids[0]
	winner.Status = protocol.BidStatusAccepted
	auction.WinnerID = winner.BidderID
	auction.WinningBid = winner.BidPrice
	auction.Status = AuctionStatusClosed

	// Reject other bids
	for _, bid := range bids[1:] {
		bid.Status = protocol.BidStatusRejected
	}

	am.logger.WithFields(logrus.Fields{
		"auction_id": auction.ID,
		"winner":     winner.BidderID,
		"price":      winner.BidPrice,
		"score":      winner.Score,
	}).Info("Auction winner selected")

	am.resultChan <- &AuctionResult{
		AuctionID:  auction.ID,
		TaskID:     auction.TaskID,
		WinnerID:   winner.BidderID,
		WinningBid: winner.BidPrice,
		Success:    true,
		Reason:     "winner selected",
	}
}

// selectConsensusWinners selects multiple winners for consensus mode
func (am *AuctionManager) selectConsensusWinners(auction *Auction) {
	if len(auction.Bids) == 0 {
		auction.Status = AuctionStatusClosed
		am.resultChan <- &AuctionResult{
			AuctionID: auction.ID,
			TaskID:    auction.TaskID,
			Success:   false,
			Reason:    "no bids received",
		}
		return
	}

	// Sort bids by score
	bids := make([]*Bid, 0, len(auction.Bids))
	for _, bid := range auction.Bids {
		bids = append(bids, bid)
	}
	sort.Slice(bids, func(i, j int) bool {
		return bids[i].Score > bids[j].Score
	})

	// Select top N winners
	threshold := auction.ConsensusThreshold
	if threshold > len(bids) {
		threshold = len(bids)
	}

	winners := make([]string, threshold)
	for i := 0; i < threshold; i++ {
		bids[i].Status = protocol.BidStatusAccepted
		winners[i] = bids[i].BidderID
	}

	auction.ConsensusWinners = winners
	auction.Status = AuctionStatusClosed

	am.logger.WithFields(logrus.Fields{
		"auction_id": auction.ID,
		"winners":    winners,
		"threshold":  threshold,
	}).Info("Consensus winners selected")

	am.resultChan <- &AuctionResult{
		AuctionID:        auction.ID,
		TaskID:           auction.TaskID,
		Success:          true,
		Reason:           "consensus winners selected",
		ConsensusWinners: winners,
	}
}

// CancelAuction cancels an auction
func (am *AuctionManager) CancelAuction(auctionID string) error {
	am.mu.Lock()
	defer am.mu.Unlock()

	auction, exists := am.auctions[auctionID]
	if !exists {
		return fmt.Errorf("auction not found")
	}

	auction.mu.Lock()
	defer auction.mu.Unlock()

	if auction.Status == AuctionStatusClosed {
		return fmt.Errorf("auction already closed")
	}

	auction.Status = AuctionStatusCancelled

	am.logger.WithField("auction_id", auctionID).Info("Auction cancelled")
	return nil
}

// GetAuction gets an auction by ID
func (am *AuctionManager) GetAuction(auctionID string) (*Auction, error) {
	am.mu.RLock()
	defer am.mu.RUnlock()

	auction, exists := am.auctions[auctionID]
	if !exists {
		return nil, fmt.Errorf("auction not found")
	}

	return auction, nil
}

// GetAuctionsByTask gets auctions by task ID
func (am *AuctionManager) GetAuctionsByTask(taskID string) []*Auction {
	am.mu.RLock()
	defer am.mu.RUnlock()

	var result []*Auction
	for _, auction := range am.auctions {
		if auction.TaskID == taskID {
			result = append(result, auction)
		}
	}
	return result
}

// GetActiveAuctions returns all active auctions
func (am *AuctionManager) GetActiveAuctions() []*Auction {
	am.mu.RLock()
	defer am.mu.RUnlock()

	var result []*Auction
	for _, auction := range am.auctions {
		auction.mu.RLock()
		status := auction.Status
		auction.mu.RUnlock()

		if status == AuctionStatusOpen || status == AuctionStatusBidding {
			result = append(result, auction)
		}
	}
	return result
}

// GetResultChan returns the auction result channel
func (am *AuctionManager) GetResultChan() <-chan *AuctionResult {
	return am.resultChan
}

// processAuctions processes auction events
func (am *AuctionManager) processAuctions() {
	for {
		select {
		case auction := <-am.auctionChan:
			am.logger.WithField("auction_id", auction.ID).Debug("Processing auction")
		case <-am.ctx.Done():
			return
		}
	}
}

// processBids processes bid events
func (am *AuctionManager) processBids() {
	for {
		select {
		case bid := <-am.bidChan:
			am.logger.WithFields(logrus.Fields{
				"auction_id": bid.AuctionID,
				"bidder":     bid.BidderID,
			}).Debug("Processing bid")
		case <-am.ctx.Done():
			return
		}
	}
}

// Stop stops the auction manager
func (am *AuctionManager) Stop() {
	am.cancel()
}

// Helper function to convert peer.ID to string
func peerIDToString(id peer.ID) string {
	return id.String()
}
