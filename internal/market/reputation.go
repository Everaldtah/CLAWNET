package market

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/Everaldtah/CLAWNET/internal/config"
	"github.com/mitchellh/go-homedir"
	"go.etcd.io/bbolt"
)

const (
	reputationDBName     = "reputation.db"
	reputationBucketName = "reputation"
	ownReputationKey     = "self"
)

// ReputationManager manages reputation scores
type ReputationManager struct {
	mu sync.RWMutex

	nodeID       string
	ownRep       *Reputation
	peers        map[string]*Reputation
	db           *bbolt.DB
	cfg          *config.Config
	decayRate    float64
	lastDecay    time.Time

	// Feedback tracking
	feedbackChan chan *Feedback
}

// Reputation represents a node's reputation
type Reputation struct {
	NodeID            string    `json:"node_id"`
	Score             float64   `json:"score"`
	CompletionRate    float64   `json:"completion_rate"`
	OnTimeRate        float64   `json:"on_time_rate"`
	DisputeRatio      float64   `json:"dispute_ratio"`
	ResponseAccuracy  float64   `json:"response_accuracy"`
	SwarmContribution float64   `json:"swarm_contribution"`
	TaskCount         int       `json:"task_count"`
	CompletedTasks    int       `json:"completed_tasks"`
	FailedTasks       int       `json:"failed_tasks"`
	DisputedTasks     int       `json:"disputed_tasks"`
	TotalEarned       float64   `json:"total_earned"`
	LastUpdated       time.Time `json:"last_updated"`
	Version           uint64    `json:"version"`
	Signature         string    `json:"signature"`
}

// Feedback represents peer feedback
type Feedback struct {
	FromPeer   string
	ToPeer     string
	TaskID     string
	Rating     float64
	Comment    string
	Timestamp  time.Time
}

// NewReputationManager creates a new reputation manager
func NewReputationManager(nodeID string, cfg *config.Config) (*ReputationManager, error) {
	home, err := homedir.Dir()
	if err != nil {
		return nil, fmt.Errorf("failed to get home directory: %w", err)
	}

	repDir := filepath.Join(home, cfg.Node.DataDir)
	if err := os.MkdirAll(repDir, 0700); err != nil {
		return nil, fmt.Errorf("failed to create reputation directory: %w", err)
	}

	dbPath := filepath.Join(repDir, reputationDBName)
	db, err := bbolt.Open(dbPath, 0600, &bbolt.Options{Timeout: 1 * time.Second})
	if err != nil {
		return nil, fmt.Errorf("failed to open reputation database: %w", err)
	}

	// Create bucket if not exists
	if err := db.Update(func(tx *bbolt.Tx) error {
		_, err := tx.CreateBucketIfNotExists([]byte(reputationBucketName))
		return err
	}); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to create reputation bucket: %w", err)
	}

	rm := &ReputationManager{
		nodeID:       nodeID,
		peers:        make(map[string]*Reputation),
		db:           db,
		cfg:          cfg,
		decayRate:    cfg.Market.ReputationDecayRate,
		lastDecay:    time.Now(),
		feedbackChan: make(chan *Feedback, 1000),
	}

	// Load own reputation
	if err := rm.loadOwnReputation(); err != nil {
		// Initialize with default reputation
		rm.ownRep = &Reputation{
			NodeID:      nodeID,
			Score:       0.5,
			CompletionRate: 1.0,
			OnTimeRate:     1.0,
			LastUpdated: time.Now(),
			Version:     1,
		}
		if err := rm.saveOwnReputation(); err != nil {
			db.Close()
			return nil, fmt.Errorf("failed to save initial reputation: %w", err)
		}
	}

	// Start decay processor
	go rm.decayProcessor()
	go rm.feedbackProcessor()

	return rm, nil
}

// GetOwnReputation returns our own reputation
func (rm *ReputationManager) GetOwnReputation() *Reputation {
	rm.mu.RLock()
	defer rm.mu.RUnlock()
	return rm.ownRep
}

// GetReputation gets a peer's reputation
func (rm *ReputationManager) GetReputation(peerID string) (*Reputation, error) {
	rm.mu.RLock()
	defer rm.mu.RUnlock()

	if peerID == rm.nodeID {
		return rm.ownRep, nil
	}

	rep, exists := rm.peers[peerID]
	if !exists {
		// Try to load from database
		rm.mu.RUnlock()
		rep, err := rm.loadPeerReputation(peerID)
		rm.mu.RLock()
		if err != nil {
			// Return default reputation
			return &Reputation{
				NodeID: peerID,
				Score:  0.5,
			}, nil
		}
		rm.peers[peerID] = rep
		return rep, nil
	}

	return rep, nil
}

// UpdateReputation updates a peer's reputation
func (rm *ReputationManager) UpdateReputation(peerID string, update *ReputationUpdate) error {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	var rep *Reputation
	if peerID == rm.nodeID {
		rep = rm.ownRep
	} else {
		rep = rm.peers[peerID]
		if rep == nil {
			rep = &Reputation{NodeID: peerID, Score: 0.5}
			rm.peers[peerID] = rep
		}
	}

	// Apply weighted updates
	alpha := 0.1 // Learning rate

	rep.CompletionRate = (1-alpha)*rep.CompletionRate + alpha*update.CompletionDelta
	rep.OnTimeRate = (1-alpha)*rep.OnTimeRate + alpha*update.TimelinessDelta
	rep.ResponseAccuracy = (1-alpha)*rep.ResponseAccuracy + alpha*update.QualityDelta

	// Recalculate overall score
	rep.Score = rm.calculateScore(rep)
	rep.LastUpdated = time.Now()
	rep.Version++

	// Save to database
	if peerID == rm.nodeID {
		return rm.saveOwnReputation()
	}
	return rm.savePeerReputation(peerID, rep)
}

// RecordTaskCompletion records task completion
func (rm *ReputationManager) RecordTaskCompletion(peerID string, success bool, onTime bool) error {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	var rep *Reputation
	if peerID == rm.nodeID {
		rep = rm.ownRep
	} else {
		rep = rm.peers[peerID]
		if rep == nil {
			rep = &Reputation{NodeID: peerID, Score: 0.5}
			rm.peers[peerID] = rep
		}
	}

	rep.TaskCount++
	if success {
		rep.CompletedTasks++
	} else {
		rep.FailedTasks++
	}

	// Update completion rate
	if rep.TaskCount > 0 {
		rep.CompletionRate = float64(rep.CompletedTasks) / float64(rep.TaskCount)
	}

	// Update on-time rate
	if onTime {
		// Weighted update
		rep.OnTimeRate = 0.9*rep.OnTimeRate + 0.1
	} else {
		rep.OnTimeRate = 0.9 * rep.OnTimeRate
	}

	// Recalculate score
	rep.Score = rm.calculateScore(rep)
	rep.LastUpdated = time.Now()
	rep.Version++

	if peerID == rm.nodeID {
		return rm.saveOwnReputation()
	}
	return rm.savePeerReputation(peerID, rep)
}

// RecordDispute records a dispute
func (rm *ReputationManager) RecordDispute(peerID string) error {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	var rep *Reputation
	if peerID == rm.nodeID {
		rep = rm.ownRep
	} else {
		rep = rm.peers[peerID]
		if rep == nil {
			rep = &Reputation{NodeID: peerID, Score: 0.5}
			rm.peers[peerID] = rep
		}
	}

	rep.DisputedTasks++
	if rep.TaskCount > 0 {
		rep.DisputeRatio = float64(rep.DisputedTasks) / float64(rep.TaskCount)
	}

	// Penalty for disputes
	rep.Score *= 0.95

	rep.LastUpdated = time.Now()
	rep.Version++

	if peerID == rm.nodeID {
		return rm.saveOwnReputation()
	}
	return rm.savePeerReputation(peerID, rep)
}

// AddFeedback adds peer feedback
func (rm *ReputationManager) AddFeedback(feedback *Feedback) {
	select {
	case rm.feedbackChan <- feedback:
	default:
	}
}

// calculateScore calculates overall reputation score
func (rm *ReputationManager) calculateScore(rep *Reputation) float64 {
	// Weighted combination of factors
	weights := struct {
		Completion   float64
		Timeliness   float64
		Accuracy     float64
		Dispute      float64
		Contribution float64
	}{
		Completion:   0.3,
		Timeliness:   0.2,
		Accuracy:     0.2,
		Dispute:      0.2,
		Contribution: 0.1,
	}

	// Dispute penalty (lower is better)
	disputeScore := 1.0 - rep.DisputeRatio

	score := weights.Completion*rep.CompletionRate +
		weights.Timeliness*rep.OnTimeRate +
		weights.Accuracy*rep.ResponseAccuracy +
		weights.Dispute*disputeScore +
		weights.Contribution*rep.SwarmContribution

	// Clamp to [0, 1]
	if score < 0 {
		score = 0
	}
	if score > 1 {
		score = 1
	}

	return score
}

// applyDecay applies reputation decay
func (rm *ReputationManager) applyDecay() {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	// Apply decay to own reputation
	if time.Since(rm.ownRep.LastUpdated) > 24*time.Hour {
		rm.ownRep.Score *= (1.0 - rm.decayRate)
		rm.ownRep.LastUpdated = time.Now()
		rm.saveOwnReputation()
	}

	// Apply decay to peers
	for peerID, rep := range rm.peers {
		if time.Since(rep.LastUpdated) > 24*time.Hour {
			rep.Score *= (1.0 - rm.decayRate)
			rep.LastUpdated = time.Now()
			rm.savePeerReputation(peerID, rep)
		}
	}
}

// decayProcessor periodically applies reputation decay
func (rm *ReputationManager) decayProcessor() {
	ticker := time.NewTicker(1 * time.Hour)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			rm.applyDecay()
		case <-rm.ctxDone():
			return
		}
	}
}

// feedbackProcessor processes feedback
func (rm *ReputationManager) feedbackProcessor() {
	for {
		select {
		case feedback := <-rm.feedbackChan:
			rm.processFeedback(feedback)
		case <-rm.ctxDone():
			return
		}
	}
}

// processFeedback processes a feedback entry
func (rm *ReputationManager) processFeedback(feedback *Feedback) {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	rep := rm.peers[feedback.ToPeer]
	if rep == nil {
		rep = &Reputation{NodeID: feedback.ToPeer, Score: 0.5}
		rm.peers[feedback.ToPeer] = rep
	}

	// Update response accuracy based on rating
	alpha := 0.1
	rep.ResponseAccuracy = (1-alpha)*rep.ResponseAccuracy + alpha*feedback.Rating
	rep.Score = rm.calculateScore(rep)
	rep.LastUpdated = time.Now()
	rep.Version++

	rm.savePeerReputation(feedback.ToPeer, rep)
}

// loadOwnReputation loads own reputation from database
func (rm *ReputationManager) loadOwnReputation() error {
	return rm.db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte(reputationBucketName))
		if b == nil {
			return fmt.Errorf("reputation bucket not found")
		}

		data := b.Get([]byte(ownReputationKey))
		if data == nil {
			return fmt.Errorf("own reputation not found")
		}

		return json.Unmarshal(data, rm.ownRep)
	})
}

// saveOwnReputation saves own reputation to database
func (rm *ReputationManager) saveOwnReputation() error {
	data, err := json.Marshal(rm.ownRep)
	if err != nil {
		return err
	}

	return rm.db.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte(reputationBucketName))
		if b == nil {
			return fmt.Errorf("reputation bucket not found")
		}
		return b.Put([]byte(ownReputationKey), data)
	})
}

// loadPeerReputation loads peer reputation from database
func (rm *ReputationManager) loadPeerReputation(peerID string) (*Reputation, error) {
	var rep Reputation
	err := rm.db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte(reputationBucketName))
		if b == nil {
			return fmt.Errorf("reputation bucket not found")
		}

		data := b.Get([]byte(peerID))
		if data == nil {
			return fmt.Errorf("peer reputation not found")
		}

		return json.Unmarshal(data, &rep)
	})

	if err != nil {
		return nil, err
	}

	return &rep, nil
}

// savePeerReputation saves peer reputation to database
func (rm *ReputationManager) savePeerReputation(peerID string, rep *Reputation) error {
	data, err := json.Marshal(rep)
	if err != nil {
		return err
	}

	return rm.db.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte(reputationBucketName))
		if b == nil {
			return fmt.Errorf("reputation bucket not found")
		}
		return b.Put([]byte(peerID), data)
	})
}

// GetTopPeers returns top peers by reputation
func (rm *ReputationManager) GetTopPeers(limit int) []*Reputation {
	rm.mu.RLock()
	defer rm.mu.RUnlock()

	// Collect all reputations
	allReps := make([]*Reputation, 0, len(rm.peers)+1)
	allReps = append(allReps, rm.ownRep)
	for _, rep := range rm.peers {
		allReps = append(allReps, rep)
	}

	// Sort by score
	for i := 0; i < len(allReps); i++ {
		for j := i + 1; j < len(allReps); j++ {
			if allReps[j].Score > allReps[i].Score {
				allReps[i], allReps[j] = allReps[j], allReps[i]
			}
		}
	}

	// Return top N
	if limit > len(allReps) {
		limit = len(allReps)
	}
	return allReps[:limit]
}

// Close closes the reputation manager
func (rm *ReputationManager) Close() error {
	return rm.db.Close()
}

// ctxDone returns a done channel for context cancellation
func (rm *ReputationManager) ctxDone() <-chan struct{} {
	// Simplified - in production would use proper context
	return make(chan struct{})
}

// ReputationUpdate represents a reputation update
type ReputationUpdate struct {
	CompletionDelta float64
	TimelinessDelta float64
	QualityDelta    float64
}
