package market

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/Everaldtah/CLAWNET/internal/config"
	"github.com/Everaldtah/CLAWNET/internal/protocol"
	"github.com/sirupsen/logrus"
)

// BidManager manages bid submission and evaluation
type BidManager struct {
	mu sync.RWMutex

	cfg           *config.Config
	reputation    *ReputationManager
	logger        *logrus.Entry

	// Rate limiting
	bidCounters   map[string]*RateCounter
	rateLimit     int

	// Anti-collusion
	suspiciousBidders map[string]int
	collusionChecks   bool

	ctx    context.Context
	cancel context.CancelFunc
}

// RateCounter tracks bid rate for rate limiting
type RateCounter struct {
	mu        sync.Mutex
	count     int
	window    time.Time
	windowSize time.Duration
}

// NewRateCounter creates a new rate counter
func NewRateCounter(windowSize time.Duration) *RateCounter {
	return &RateCounter{
		window:     time.Now(),
		windowSize: windowSize,
	}
}

// Increment increments the counter
func (rc *RateCounter) Increment() bool {
	rc.mu.Lock()
	defer rc.mu.Unlock()

	now := time.Now()
	if now.Sub(rc.window) > rc.windowSize {
		rc.count = 0
		rc.window = now
	}

	rc.count++
	return true
}

// Count returns the current count
func (rc *RateCounter) Count() int {
	rc.mu.Lock()
	defer rc.mu.Unlock()
	return rc.count
}

// NewBidManager creates a new bid manager
func NewBidManager(cfg *config.Config, reputation *ReputationManager, logger *logrus.Logger) *BidManager {
	ctx, cancel := context.WithCancel(context.Background())

	return &BidManager{
		cfg:               cfg,
		reputation:        reputation,
		logger:            logger.WithField("component", "bid"),
		bidCounters:       make(map[string]*RateCounter),
		rateLimit:         cfg.Market.RateLimitBids,
		suspiciousBidders: make(map[string]int),
		collusionChecks:   cfg.Market.AntiCollusionChecks,
		ctx:               ctx,
		cancel:            cancel,
	}
}

// EvaluateBid evaluates whether to submit a bid for a task
func (bm *BidManager) EvaluateBid(task *protocol.MarketTaskAnnouncePayload) (*protocol.MarketBidPayload, error) {
	bm.mu.RLock()
	defer bm.mu.RUnlock()

	// Check rate limiting
	if !bm.checkRateLimit(task.RequesterID) {
		return nil, fmt.Errorf("rate limit exceeded")
	}

	// Get our reputation
	ourRep := bm.reputation.GetOwnReputation()

	// Check minimum reputation requirement
	if ourRep.Score < task.MinimumReputation {
		return nil, fmt.Errorf("reputation below minimum requirement")
	}

	// Check capability match
	capabilityMatch := bm.evaluateCapabilityMatch(task.RequiredCapabilities)
	if capabilityMatch < 0.5 {
		return nil, fmt.Errorf("insufficient capabilities")
	}

	// Calculate bid price
	bidPrice := bm.calculateBidPrice(task, capabilityMatch)

	// Estimate completion time
	estimatedDuration := bm.estimateDuration(task)

	// Calculate confidence score
	confidence := bm.calculateConfidence(task, capabilityMatch, ourRep.Score)

	// Get current load
	currentLoad := getSystemLoad()

	// Generate capability hash
	capHash := bm.generateCapabilityHash(task.RequiredCapabilities)

	bid := &protocol.MarketBidPayload{
		TaskID:            task.TaskID,
		BidPrice:          bidPrice,
		EstimatedDuration: int64(estimatedDuration.Milliseconds()),
		ConfidenceScore:   confidence,
		CapabilityHash:    capHash,
		ReputationScore:   ourRep.Score,
		CurrentLoad:       currentLoad,
		EnergyCost:        0.0, // Would come from power monitoring
		Nonce:             generateNonce(),
	}

	bm.logger.WithFields(logrus.Fields{
		"task_id":    task.TaskID,
		"bid_price":  bidPrice,
		"confidence": confidence,
	}).Debug("Bid evaluated")

	return bid, nil
}

// checkRateLimit checks if the bidder is within rate limits
func (bm *BidManager) checkRateLimit(bidderID string) bool {
	counter, exists := bm.bidCounters[bidderID]
	if !exists {
		counter = NewRateCounter(time.Minute)
		bm.bidCounters[bidderID] = counter
	}

	counter.Increment()
	return counter.Count() <= bm.rateLimit
}

// evaluateCapabilityMatch evaluates how well we match required capabilities
func (bm *BidManager) evaluateCapabilityMatch(required []string) float64 {
	if len(required) == 0 {
		return 1.0
	}

	ourCaps := bm.cfg.Node.Capabilities
	matchCount := 0

	for _, req := range required {
		for _, cap := range ourCaps {
			if req == cap {
				matchCount++
				break
			}
		}
	}

	return float64(matchCount) / float64(len(required))
}

// calculateBidPrice calculates the bid price for a task
func (bm *BidManager) calculateBidPrice(task *protocol.MarketTaskAnnouncePayload, capabilityMatch float64) float64 {
	// Base price calculation
	basePrice := task.MaxBudget * 0.7

	// Adjust based on capability match
	price := basePrice * (0.5 + 0.5*capabilityMatch)

	// Adjust based on current load
	load := getSystemLoad()
	if load > 0.8 {
		price *= 1.5 // High load = higher price
	} else if load < 0.3 {
		price *= 0.9 // Low load = competitive price
	}

	// Ensure minimum bid increment
	minBid := task.MaxBudget * 0.1
	if price < minBid {
		price = minBid
	}

	// Round to minimum increment
	increment := bm.cfg.Market.MinBidIncrement
	price = float64(int(price/increment)) * increment

	return price
}

// estimateDuration estimates task completion duration
func (bm *BidManager) estimateDuration(task *protocol.MarketTaskAnnouncePayload) time.Duration {
	// Base duration by task type
	var baseDuration time.Duration

	switch task.Type {
	case protocol.TaskTypeOpenClawPrompt:
		baseDuration = 5 * time.Second
	case protocol.TaskTypeShellTask:
		baseDuration = 10 * time.Second
	case protocol.TaskTypeSwarmAI:
		baseDuration = 30 * time.Second
	case protocol.TaskTypeCompute:
		baseDuration = 60 * time.Second
	default:
		baseDuration = 10 * time.Second
	}

	// Adjust based on current load
	load := getSystemLoad()
	if load > 0.5 {
		baseDuration = time.Duration(float64(baseDuration) * (1.0 + load))
	}

	return baseDuration
}

// calculateConfidence calculates confidence score for bid
func (bm *BidManager) calculateConfidence(task *protocol.MarketTaskAnnouncePayload, capabilityMatch, reputation float64) float64 {
	confidence := 0.5

	// Capability contribution
	confidence += capabilityMatch * 0.3

	// Reputation contribution
	confidence += reputation * 0.2

	// Load contribution (lower load = higher confidence)
	load := getSystemLoad()
	confidence += (1.0 - load) * 0.1

	// Cap at 1.0
	if confidence > 1.0 {
		confidence = 1.0
	}

	return confidence
}

// generateCapabilityHash generates a hash of capabilities
func (bm *BidManager) generateCapabilityHash(capabilities []string) string {
	// Simple hash for now
	hash := ""
	for _, cap := range capabilities {
		hash += cap + ":"
	}
	return hash
}

// DetectCollusion detects potential collusion in bids
func (bm *BidManager) DetectCollusion(bids []*Bid) []string {
	if !bm.collusionChecks || len(bids) < 3 {
		return nil
	}

	suspicious := make([]string, 0)

	// Check for similar pricing patterns
	priceClusters := bm.clusterByPrice(bids)
	for _, cluster := range priceClusters {
		if len(cluster) > len(bids)/2 {
			// More than half the bids in one price cluster is suspicious
			for _, bid := range cluster {
				suspicious = append(suspicious, bid.BidderID)
			}
		}
	}

	// Check for rapid sequential bidding
	timeClusters := bm.clusterByTime(bids)
	for _, cluster := range timeClusters {
		if len(cluster) > 3 {
			for _, bid := range cluster {
				suspicious = append(suspicious, bid.BidderID)
			}
		}
	}

	return uniqueStrings(suspicious)
}

// clusterByPrice clusters bids by price similarity
func (bm *BidManager) clusterByPrice(bids []*Bid) [][]*Bid {
	if len(bids) == 0 {
		return nil
	}

	clusters := make([][]*Bid, 0)
	used := make(map[string]bool)

	for i, bid1 := range bids {
		if used[bid1.ID] {
			continue
		}

		cluster := []*Bid{bid1}
		used[bid1.ID] = true

		for j, bid2 := range bids {
			if i == j || used[bid2.ID] {
				continue
			}

			// Check if prices are within 5% of each other
			diff := abs(bid1.BidPrice - bid2.BidPrice)
			avg := (bid1.BidPrice + bid2.BidPrice) / 2
			if avg > 0 && diff/avg < 0.05 {
				cluster = append(cluster, bid2)
				used[bid2.ID] = true
			}
		}

		clusters = append(clusters, cluster)
	}

	return clusters
}

// clusterByTime clusters bids by time proximity
func (bm *BidManager) clusterByTime(bids []*Bid) [][]*Bid {
	if len(bids) == 0 {
		return nil
	}

	clusters := make([][]*Bid, 0)
	used := make(map[string]bool)

	for i, bid1 := range bids {
		if used[bid1.ID] {
			continue
		}

		cluster := []*Bid{bid1}
		used[bid1.ID] = true

		for j, bid2 := range bids {
			if i == j || used[bid2.ID] {
				continue
			}

			// Check if bids are within 1 second of each other
			diff := absDuration(bid1.Timestamp.Sub(bid2.Timestamp))
			if diff < time.Second {
				cluster = append(cluster, bid2)
				used[bid2.ID] = true
			}
		}

		clusters = append(clusters, cluster)
	}

	return clusters
}

// ReportSuspicious reports suspicious bidder activity
func (bm *BidManager) ReportSuspicious(bidderID string) {
	bm.mu.Lock()
	defer bm.mu.Unlock()

	bm.suspiciousBidders[bidderID]++

	bm.logger.WithFields(logrus.Fields{
		"bidder": bidderID,
		"count":  bm.suspiciousBidders[bidderID],
	}).Warn("Suspicious bidder activity detected")
}

// Stop stops the bid manager
func (bm *BidManager) Stop() {
	bm.cancel()
}

// Helper functions
func abs(f float64) float64 {
	if f < 0 {
		return -f
	}
	return f
}

func absDuration(d time.Duration) time.Duration {
	if d < 0 {
		return -d
	}
	return d
}

func uniqueStrings(strs []string) []string {
	seen := make(map[string]bool)
	result := make([]string, 0)
	for _, s := range strs {
		if !seen[s] {
			seen[s] = true
			result = append(result, s)
		}
	}
	return result
}

func generateNonce() string {
	return fmt.Sprintf("%d", time.Now().UnixNano())
}

// getSystemLoad returns the current system load (0.0 to 1.0)
// For now, returns a simulated value based on time
func getSystemLoad() float64 {
	// Simulate load based on time of day
	hour := time.Now().Hour()
	if hour >= 9 && hour <= 17 {
		// Business hours - higher load
		return 0.6 + (float64(time.Now().Second()) / 1000.0)
	}
	// Off hours - lower load
	return 0.2 + (float64(time.Now().Second()) / 2000.0)
}
