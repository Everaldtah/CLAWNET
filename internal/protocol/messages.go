package protocol

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/libp2p/go-libp2p/core/peer"
)

// MessageType represents the type of protocol message
type MessageType uint8

const (
	// Core protocol messages
	MSG_TYPE_PING MessageType = iota
	MSG_TYPE_PONG
	MSG_TYPE_PEER_INFO
	MSG_TYPE_DISCONNECT

	// Task delegation messages
	MSG_TYPE_TASK_SUBMIT
	MSG_TYPE_TASK_ACCEPT
	MSG_TYPE_TASK_REJECT
	MSG_TYPE_TASK_RESULT
	MSG_TYPE_TASK_CANCEL

	// Memory synchronization messages
	MSG_TYPE_MEM_SYNC_REQ
	MSG_TYPE_MEM_SYNC_RESP
	MSG_TYPE_MEM_UPDATE
	MSG_TYPE_MEM_DELETE

	// Market messages
	MSG_TYPE_MARKET_TASK_ANNOUNCE
	MSG_TYPE_MARKET_BID
	MSG_TYPE_MARKET_WINNER
	MSG_TYPE_MARKET_ESCROW_LOCK
	MSG_TYPE_MARKET_SETTLEMENT
	MSG_TYPE_MARKET_DISPUTE
	MSG_TYPE_MARKET_REPUTATION_UPDATE

	// Swarm messages
	MSG_TYPE_SWARM_FORM
	MSG_TYPE_SWARM_JOIN
	MSG_TYPE_SWARM_LEAVE
	MSG_TYPE_SWARM_COORDINATE
	MSG_TYPE_SWARM_RESULT

	// OpenClaw integration messages
	MSG_TYPE_OPENCLAW_PROMPT
	MSG_TYPE_OPENCLAW_RESPONSE
	MSG_TYPE_OPENCLAW_STREAM

	// Social media messages (CLAWSocial)
	MSG_TYPE_SOCIAL_POST_CREATE
	MSG_TYPE_SOCIAL_POST_UPDATE
	MSG_TYPE_SOCIAL_POST_DELETE
	MSG_TYPE_SOCIAL_VOTE
	MSG_TYPE_SOCIAL_COMMENT
	MSG_TYPE_SOCIAL_FOLLOW
	MSG_TYPE_SOCIAL_UNFOLLOW
	MSG_TYPE_SOCIAL_PROFILE_UPDATE
	MSG_TYPE_SOCIAL_DM
	MSG_TYPE_SOCIAL_SUBSCRIBE
	MSG_TYPE_SOCIAL_FEED_REQUEST
	MSG_TYPE_SOCIAL_FEED_RESPONSE
	MSG_TYPE_SOCIAL_TRENDING
)

func (mt MessageType) String() string {
	switch mt {
	case MSG_TYPE_PING:
		return "PING"
	case MSG_TYPE_PONG:
		return "PONG"
	case MSG_TYPE_PEER_INFO:
		return "PEER_INFO"
	case MSG_TYPE_DISCONNECT:
		return "DISCONNECT"
	case MSG_TYPE_TASK_SUBMIT:
		return "TASK_SUBMIT"
	case MSG_TYPE_TASK_ACCEPT:
		return "TASK_ACCEPT"
	case MSG_TYPE_TASK_REJECT:
		return "TASK_REJECT"
	case MSG_TYPE_TASK_RESULT:
		return "TASK_RESULT"
	case MSG_TYPE_TASK_CANCEL:
		return "TASK_CANCEL"
	case MSG_TYPE_MEM_SYNC_REQ:
		return "MEM_SYNC_REQ"
	case MSG_TYPE_MEM_SYNC_RESP:
		return "MEM_SYNC_RESP"
	case MSG_TYPE_MEM_UPDATE:
		return "MEM_UPDATE"
	case MSG_TYPE_MEM_DELETE:
		return "MEM_DELETE"
	case MSG_TYPE_MARKET_TASK_ANNOUNCE:
		return "MARKET_TASK_ANNOUNCE"
	case MSG_TYPE_MARKET_BID:
		return "MARKET_BID"
	case MSG_TYPE_MARKET_WINNER:
		return "MARKET_WINNER"
	case MSG_TYPE_MARKET_ESCROW_LOCK:
		return "MARKET_ESCROW_LOCK"
	case MSG_TYPE_MARKET_SETTLEMENT:
		return "MARKET_SETTLEMENT"
	case MSG_TYPE_MARKET_DISPUTE:
		return "MARKET_DISPUTE"
	case MSG_TYPE_MARKET_REPUTATION_UPDATE:
		return "MARKET_REPUTATION_UPDATE"
	case MSG_TYPE_SWARM_FORM:
		return "SWARM_FORM"
	case MSG_TYPE_SWARM_JOIN:
		return "SWARM_JOIN"
	case MSG_TYPE_SWARM_LEAVE:
		return "SWARM_LEAVE"
	case MSG_TYPE_SWARM_COORDINATE:
		return "SWARM_COORDINATE"
	case MSG_TYPE_SWARM_RESULT:
		return "SWARM_RESULT"
	case MSG_TYPE_OPENCLAW_PROMPT:
		return "OPENCLAW_PROMPT"
	case MSG_TYPE_OPENCLAW_RESPONSE:
		return "OPENCLAW_RESPONSE"
	case MSG_TYPE_OPENCLAW_STREAM:
		return "OPENCLAW_STREAM"
	case MSG_TYPE_SOCIAL_POST_CREATE:
		return "SOCIAL_POST_CREATE"
	case MSG_TYPE_SOCIAL_POST_UPDATE:
		return "SOCIAL_POST_UPDATE"
	case MSG_TYPE_SOCIAL_POST_DELETE:
		return "SOCIAL_POST_DELETE"
	case MSG_TYPE_SOCIAL_VOTE:
		return "SOCIAL_VOTE"
	case MSG_TYPE_SOCIAL_COMMENT:
		return "SOCIAL_COMMENT"
	case MSG_TYPE_SOCIAL_FOLLOW:
		return "SOCIAL_FOLLOW"
	case MSG_TYPE_SOCIAL_UNFOLLOW:
		return "SOCIAL_UNFOLLOW"
	case MSG_TYPE_SOCIAL_PROFILE_UPDATE:
		return "SOCIAL_PROFILE_UPDATE"
	case MSG_TYPE_SOCIAL_DM:
		return "SOCIAL_DM"
	case MSG_TYPE_SOCIAL_SUBSCRIBE:
		return "SOCIAL_SUBSCRIBE"
	case MSG_TYPE_SOCIAL_FEED_REQUEST:
		return "SOCIAL_FEED_REQUEST"
	case MSG_TYPE_SOCIAL_FEED_RESPONSE:
		return "SOCIAL_FEED_RESPONSE"
	case MSG_TYPE_SOCIAL_TRENDING:
		return "SOCIAL_TRENDING"
	default:
		return "UNKNOWN"
	}
}

// TaskType represents the type of task
type TaskType string

const (
	TaskTypeOpenClawPrompt TaskType = "openclaw_prompt"
	TaskTypeShellTask      TaskType = "shell_task"
	TaskTypeSwarmAI        TaskType = "swarm_ai"
	TaskTypeCompute        TaskType = "compute"
	TaskTypeStorage        TaskType = "storage"
)

// TaskStatus represents the status of a task
type TaskStatus string

const (
	TaskStatusPending    TaskStatus = "pending"
	TaskStatusBidding    TaskStatus = "bidding"
	TaskStatusAssigned   TaskStatus = "assigned"
	TaskStatusExecuting  TaskStatus = "executing"
	TaskStatusCompleted  TaskStatus = "completed"
	TaskStatusFailed     TaskStatus = "failed"
	TaskStatusCancelled  TaskStatus = "cancelled"
	TaskStatusDisputed   TaskStatus = "disputed"
	TaskStatusTimeout    TaskStatus = "timeout"
)

// BidStatus represents the status of a bid
type BidStatus string

const (
	BidStatusPending   BidStatus = "pending"
	BidStatusAccepted  BidStatus = "accepted"
	BidStatusRejected  BidStatus = "rejected"
	BidStatusWithdrawn BidStatus = "withdrawn"
)

// SettlementStatus represents the status of a settlement
type SettlementStatus string

const (
	SettlementStatusPending    SettlementStatus = "pending"
	SettlementStatusCompleted  SettlementStatus = "completed"
	SettlementStatusDisputed   SettlementStatus = "disputed"
	SettlementStatusRefunded   SettlementStatus = "refunded"
)

// MessageHeader contains common fields for all messages
type MessageHeader struct {
	ID        string    `json:"id"`
	Type      MessageType `json:"type"`
	Timestamp int64     `json:"timestamp"`
	Sender    string    `json:"sender"`
	Nonce     string    `json:"nonce"`
	Signature string    `json:"signature"`
}

// Message is the base message structure
type Message struct {
	Header  MessageHeader   `json:"header"`
	Payload json.RawMessage `json:"payload"`
}

// NewMessage creates a new message
func NewMessage(msgType MessageType, sender peer.ID, payload interface{}) (*Message, error) {
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal payload: %w", err)
	}

	return &Message{
		Header: MessageHeader{
			ID:        uuid.New().String(),
			Type:      msgType,
			Timestamp: time.Now().UnixNano(),
			Sender:    sender.String(),
			Nonce:     generateNonce(),
		},
		Payload: payloadBytes,
	}, nil
}

// GenerateHash generates a hash for signing
func (m *Message) GenerateHash() []byte {
	h := sha256.New()
	h.Write([]byte(m.Header.ID))
	h.Write([]byte(m.Header.Type.String()))
	h.Write([]byte(fmt.Sprintf("%d", m.Header.Timestamp)))
	h.Write([]byte(m.Header.Sender))
	h.Write([]byte(m.Header.Nonce))
	h.Write(m.Payload)
	return h.Sum(nil)
}

// VerifySignature verifies the message signature
func (m *Message) VerifySignature(verifyFunc func(data, sig []byte) bool) bool {
	hash := m.GenerateHash()
	sigBytes, err := DecodeString(m.Header.Signature)
	if err != nil {
		return false
	}
	return verifyFunc(hash, sigBytes)
}

// Sign signs the message
func (m *Message) Sign(signFunc func([]byte) []byte) {
	hash := m.GenerateHash()
	m.Header.Signature = base64.StdEncoding.EncodeToString(signFunc(hash))
}

// Encode serializes the message to bytes
func (m *Message) Encode() ([]byte, error) {
	return json.Marshal(m)
}

// Decode deserializes bytes to a message
func Decode(data []byte) (*Message, error) {
	var msg Message
	if err := json.Unmarshal(data, &msg); err != nil {
		return nil, fmt.Errorf("failed to decode message: %w", err)
	}
	return &msg, nil
}

// TaskSubmitPayload represents a task submission
type TaskSubmitPayload struct {
	TaskID               string            `json:"task_id"`
	Description          string            `json:"description"`
	Type                 TaskType          `json:"type"`
	Data                 json.RawMessage   `json:"data"`
	MaxBudget            float64           `json:"max_budget"`
	Deadline             int64             `json:"deadline"`
	RequiredCapabilities []string          `json:"required_capabilities"`
	MinimumReputation    float64           `json:"minimum_reputation"`
	RequesterID          string            `json:"requester_id"`
	EscrowAmount         float64           `json:"escrow_amount"`
	ConsensusRequired    bool              `json:"consensus_required"`
	ConsensusThreshold   int               `json:"consensus_threshold"`
}

// TaskResultPayload represents a task result
type TaskResultPayload struct {
	TaskID          string          `json:"task_id"`
	ExecutorID      string          `json:"executor_id"`
	Status          TaskStatus      `json:"status"`
	Result          json.RawMessage `json:"result"`
	CompletionTime  int64           `json:"completion_time"`
	ExecutionCost   float64         `json:"execution_cost"`
	VerificationHash string         `json:"verification_hash"`
	Signature       string          `json:"signature"`
}

// MarketTaskAnnouncePayload announces a task to the market
type MarketTaskAnnouncePayload struct {
	TaskID               string          `json:"task_id"`
	Description          string          `json:"description"`
	Type                 TaskType        `json:"type"`
	MaxBudget            float64         `json:"max_budget"`
	Deadline             int64           `json:"deadline"`
	RequiredCapabilities []string        `json:"required_capabilities"`
	MinimumReputation    float64         `json:"minimum_reputation"`
	RequesterID          string          `json:"requester_id"`
	BidTimeout           int64           `json:"bid_timeout"`
	EscrowRequired       bool            `json:"escrow_required"`
	ConsensusMode        bool            `json:"consensus_mode"`
	ConsensusThreshold   int             `json:"consensus_threshold"`
}

// MarketBidPayload represents a bid on a task
type MarketBidPayload struct {
	TaskID             string  `json:"task_id"`
	BidderID           string  `json:"bidder_id"`
	BidPrice           float64 `json:"bid_price"`
	EstimatedDuration  int64   `json:"estimated_duration_ms"`
	ConfidenceScore    float64 `json:"confidence_score"`
	CapabilityHash     string  `json:"capability_hash"`
	ReputationScore    float64 `json:"reputation_score"`
	CurrentLoad        float64 `json:"current_load"`
	EnergyCost         float64 `json:"energy_cost"`
	Nonce              string  `json:"nonce"`
}

// CalculateScore calculates the bid score based on weights
func (b *MarketBidPayload) CalculateScore(weights MarketWeights) float64 {
	// Lower price is better, so we invert
	priceScore := 1.0 / (1.0 + b.BidPrice)
	
	// Higher reputation is better
	reputationScore := b.ReputationScore
	
	// Lower latency (estimated duration) is better
	latencyScore := 1.0 / (1.0 + float64(b.EstimatedDuration)/1000.0)
	
	// Higher confidence is better
	confidenceScore := b.ConfidenceScore

	return weights.Price*priceScore +
		weights.Reputation*reputationScore +
		weights.Latency*latencyScore +
		weights.Confidence*confidenceScore
}

// MarketWeights defines scoring weights
type MarketWeights struct {
	Price      float64 `json:"price"`
	Reputation float64 `json:"reputation"`
	Latency    float64 `json:"latency"`
	Confidence float64 `json:"confidence"`
}

// MarketWinnerPayload announces the auction winner
type MarketWinnerPayload struct {
	TaskID           string   `json:"task_id"`
	WinnerID         string   `json:"winner_id"`
	WinningBid       float64  `json:"winning_bid"`
	BidCount         int      `json:"bid_count"`
	SelectionReason  string   `json:"selection_reason"`
	Timestamp        int64    `json:"timestamp"`
	ConsensusWinners []string `json:"consensus_winners,omitempty"`
}

// MarketEscrowLockPayload represents an escrow lock
type MarketEscrowLockPayload struct {
	TaskID       string  `json:"task_id"`
	RequesterID  string  `json:"requester_id"`
	Amount       float64 `json:"amount"`
	LockTime     int64   `json:"lock_time"`
	ExpiryTime   int64   `json:"expiry_time"`
	EscrowID     string  `json:"escrow_id"`
}

// MarketSettlementPayload represents a settlement
type MarketSettlementPayload struct {
	TaskID           string           `json:"task_id"`
	EscrowID         string           `json:"escrow_id"`
	ExecutorID       string           `json:"executor_id"`
	Amount           float64          `json:"amount"`
	Status           SettlementStatus `json:"status"`
	SettlementTime   int64            `json:"settlement_time"`
	VerificationData json.RawMessage  `json:"verification_data"`
	RefundAmount     float64          `json:"refund_amount,omitempty"`
}

// MarketDisputePayload represents a dispute
type MarketDisputePayload struct {
	TaskID          string          `json:"task_id"`
	EscrowID        string          `json:"escrow_id"`
	DisputerID      string          `json:"disputer_id"`
	Reason          string          `json:"reason"`
	Evidence        json.RawMessage `json:"evidence"`
	DisputeTime     int64           `json:"dispute_time"`
	RequestedAction string          `json:"requested_action"`
}

// ReputationUpdatePayload represents a reputation update
type ReputationUpdatePayload struct {
	PeerID            string  `json:"peer_id"`
	NewScore          float64 `json:"new_score"`
	CompletionDelta   float64 `json:"completion_delta"`
	TimelinessDelta   float64 `json:"timeliness_delta"`
	QualityDelta      float64 `json:"quality_delta"`
	UpdateTime        int64   `json:"update_time"`
	Signature         string  `json:"signature"`
}

// MemorySyncRequestPayload represents a memory sync request
type MemorySyncRequestPayload struct {
	RequesterID   string   `json:"requester_id"`
	LastSyncTime  int64    `json:"last_sync_time"`
	Keys          []string `json:"keys,omitempty"`
}

// MemorySyncResponsePayload represents a memory sync response
type MemorySyncResponsePayload struct {
	ResponderID string                 `json:"responder_id"`
	Entries     []MemoryEntry          `json:"entries"`
	DeletedKeys []string               `json:"deleted_keys"`
	SyncTime    int64                  `json:"sync_time"`
}

// MemoryEntry represents a memory entry
type MemoryEntry struct {
	Key        string          `json:"key"`
	Value      json.RawMessage `json:"value"`
	Timestamp  int64           `json:"timestamp"`
	TTL        int64           `json:"ttl"`
	Version    uint64          `json:"version"`
	Signature  string          `json:"signature"`
}

// MemoryUpdatePayload represents a memory update
type MemoryUpdatePayload struct {
	Entries   []MemoryEntry `json:"entries"`
	SourceID  string        `json:"source_id"`
	UpdateTime int64        `json:"update_time"`
}

// SwarmFormPayload represents a swarm formation request
type SwarmFormPayload struct {
	SwarmID      string   `json:"swarm_id"`
	TaskID       string   `json:"task_id"`
	CoordinatorID string  `json:"coordinator_id"`
	RequiredPeers int     `json:"required_peers"`
	Capabilities []string `json:"capabilities"`
	Budget       float64  `json:"budget"`
}

// SwarmCoordinatePayload represents swarm coordination
type SwarmCoordinatePayload struct {
	SwarmID      string          `json:"swarm_id"`
	CoordinatorID string         `json:"coordinator_id"`
	SubTasks     []SubTask       `json:"sub_tasks"`
	AggregationStrategy string   `json:"aggregation_strategy"`
}

// SubTask represents a sub-task in a swarm
type SubTask struct {
	SubTaskID    string          `json:"sub_task_id"`
	AssignedPeer string          `json:"assigned_peer"`
	Data         json.RawMessage `json:"data"`
	Budget       float64         `json:"budget"`
	Deadline     int64           `json:"deadline"`
}

// OpenClawPromptPayload represents an OpenClaw prompt
type OpenClawPromptPayload struct {
	PromptID     string          `json:"prompt_id"`
	Prompt       string          `json:"prompt"`
	Context      json.RawMessage `json:"context,omitempty"`
	MaxTokens    int             `json:"max_tokens"`
	Temperature  float64         `json:"temperature"`
	Model        string          `json:"model"`
	Stream       bool            `json:"stream"`
}

// OpenClawResponsePayload represents an OpenClaw response
type OpenClawResponsePayload struct {
	PromptID     string          `json:"prompt_id"`
	Response     string          `json:"response"`
	TokensUsed   int             `json:"tokens_used"`
	FinishReason string          `json:"finish_reason"`
	Metadata     json.RawMessage `json:"metadata,omitempty"`
}

// PeerInfoPayload represents peer information
type PeerInfoPayload struct {
	PeerID       string   `json:"peer_id"`
	Capabilities []string `json:"capabilities"`
	Version      string   `json:"version"`
	Platform     string   `json:"platform"`
	Uptime       int64    `json:"uptime"`
	Load         float64  `json:"load"`
	Reputation   float64  `json:"reputation"`
	WalletBalance float64 `json:"wallet_balance"`
}

// SocialPostCreatePayload represents a new social post
type SocialPostCreatePayload struct {
	Post      *SocialPost `json:"post"`
	CID       string      `json:"cid,omitempty"`
	Timestamp int64       `json:"timestamp"`
}

// SocialPost represents a social media post (simplified for protocol)
type SocialPost struct {
	ID          string        `json:"id"`
	AuthorID    string        `json:"author_id"`
	Type        string        `json:"type"`
	Title       string        `json:"title"`
	Content     string        `json:"content"`
	Tags        []string      `json:"tags"`
	ParentID    string        `json:"parent_id,omitempty"`
	RootPostID  string        `json:"root_post_id,omitempty"`
	Score       float64       `json:"score"`
	CreatedAt   int64         `json:"created_at"`
	CID         string        `json:"cid,omitempty"`
	Signature   string        `json:"signature"`
}

// SocialVotePayload represents a vote on a post
type SocialVotePayload struct {
	PostID     string  `json:"post_id"`
	VoterID    string  `json:"voter_id"`
	Vote       int     `json:"vote"`
	Weight     float64 `json:"weight"`
	Timestamp  int64   `json:"timestamp"`
}

// SocialFollowPayload represents following an agent
type SocialFollowPayload struct {
	FollowerID  string `json:"follower_id"`
	FolloweeID  string `json:"followee_id"`
	Timestamp   int64  `json:"timestamp"`
}

// SocialProfileUpdatePayload represents profile updates
type SocialProfileUpdatePayload struct {
	Profile   *SocialProfile `json:"profile"`
	Timestamp int64          `json:"timestamp"`
}

// SocialProfile represents an agent profile (simplified)
type SocialProfile struct {
	ID            string   `json:"id"`
	PeerID        string   `json:"peer_id"`
	Username      string   `json:"username"`
	DisplayName   string   `json:"display_name"`
	Bio           string   `json:"bio"`
	Skills        []string `json:"skills"`
	Hardware      string   `json:"hardware"`
	FollowersCount int64   `json:"followers_count"`
	FollowingCount int64   `json:"following_count"`
}

// SocialFeedRequest requests a feed from another node
type SocialFeedRequest struct {
	RequesterID   string   `json:"requester_id"`
	LastTimestamp int64    `json:"last_timestamp"`
	Limit         int      `json:"limit"`
	Topics        []string `json:"topics,omitempty"`
}

// SocialFeedResponse returns feed posts
type SocialFeedResponse struct {
	Posts        []*SocialPost `json:"posts"`
	ResponseTime int64         `json:"response_time"`
}

// generateNonce generates a random nonce
func generateNonce() string {
	return uuid.New().String()[:8]
}

// EncodeString encodes bytes to base64 string
func EncodeString(data []byte) string {
	return encodeBase64(data)
}

// DecodeString decodes base64 string to bytes
func DecodeString(s string) ([]byte, error) {
	return decodeBase64(s)
}

// Helper functions (defined in crypto.go)
var encodeBase64 func([]byte) string
var decodeBase64 func(string) ([]byte, error)

// SetEncodingFunctions sets the encoding functions (called from identity package)
func SetEncodingFunctions(enc func([]byte) string, dec func(string) ([]byte, error)) {
	encodeBase64 = enc
	decodeBase64 = dec
}
