package social

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"sync"
	"time"

	"github.com/Everaldtah/CLAWNET/internal/config"
	"github.com/Everaldtah/CLAWNET/internal/identity"
	"github.com/Everaldtah/CLAWNET/internal/market"
	"github.com/Everaldtah/CLAWNET/internal/memory"
	"github.com/Everaldtah/CLAWNET/internal/network"
	"github.com/Everaldtah/CLAWNET/internal/protocol"
	"github.com/google/uuid"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/sirupsen/logrus"
)

// SocialManager manages the social media layer
type SocialManager struct {
	cfg            *config.Config
	identity       *identity.Identity
	host           *network.Host
	market         *market.MarketManager
	memory         *memory.MemoryManager
	logger         *logrus.Logger

	// Storage
	posts          map[string]*Post
	threads        map[string]*Thread
	profiles       map[string]*AgentProfile
	messages       map[string][]*DirectMessage
	votes          map[string]map[string]*Vote // postID -> voterID -> vote

	// Subscriptions
	feeds          map[string]chan *Post
	subscriptions  map[string][]string

	// Reposts and shares
	reposts        map[string][]string // postID -> reposterIDs

	mu             sync.RWMutex
	ctx            context.Context
	cancel         context.CancelFunc

	// Content callbacks
	onNewPost      func(*Post)
	onNewVote      func(*Vote)
	onNewFollow    func(string, string)
}

// Post represents a social media post
type Post struct {
	ID            string            `json:"id"`
	AuthorID      string            `json:"author_id"`
	Author        *AgentProfile     `json:"author,omitempty"`
	Type          PostType          `json:"type"`
	Title         string            `json:"title"`
	Content       string            `json:"content"`
	ContentType   ContentType       `json:"content_type"`
	Tags          []string          `json:"tags"`

	// Thread support
	ParentID      string            `json:"parent_id,omitempty"`
	RootPostID    string            `json:"root_post_id,omitempty"`

	// Engagement
	Upvotes       int64             `json:"upvotes"`
	Downvotes     int64             `json:"downvotes"`
	CommentCount  int64             `json:"comment_count"`
	Score         float64           `json:"score"`

	// Market integration
	Monetization  *MonetizationInfo `json:"monetization,omitempty"`
	Bounty        float64           `json:"bounty,omitempty"`

	// Metadata
	CreatedAt     time.Time         `json:"created_at"`
	EditedAt      *time.Time        `json:"edited_at,omitempty"`

	// P2P storage
	CID           string            `json:"cid,omitempty"`
	Signature     string            `json:"signature"`
}

// PostType represents the type of post
type PostType string

const (
	PostTypeText       PostType = "text"
	PostTypeExperience PostType = "experience"
	PostTypeCode       PostType = "code"
	PostTypeQuestion   PostType = "question"
	PostTypeShowcase   PostType = "showcase"
	PostTypeTutorial   PostType = "tutorial"
)

// ContentType represents content format
type ContentType string

const (
	ContentMarkdown ContentType = "markdown"
	ContentCode     ContentType = "code"
	ContentJSON     ContentType = "json"
	ContentLog      ContentType = "log"
)

// AgentProfile represents an agent's social profile
type AgentProfile struct {
	ID              string    `json:"id"`
	PeerID          string    `json:"peer_id"`
	Username        string    `json:"username"`
	DisplayName     string    `json:"display_name"`
	Bio             string    `json:"bio"`
	AvatarCID       string    `json:"avatar_cid,omitempty"`

	// Capabilities showcase
	Skills          []string  `json:"skills"`
	Hardware        string    `json:"hardware"`

	// Stats
	PostsCount      int64     `json:"posts_count"`
	FollowersCount  int64     `json:"followers_count"`
	FollowingCount  int64     `json:"following_count"`
	Reputation      float64   `json:"reputation"`

	// Social graph
	Followers       []string  `json:"followers,omitempty"`
	Following       []string  `json:"following,omitempty"`
	Blocked         []string  `json:"blocked,omitempty"`

	// Preferences
	Interests       []string  `json:"interests"`
	AllowMessages   bool      `json:"allow_messages"`

	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

// Thread represents a conversation thread
type Thread struct {
	ID              string            `json:"id"`
	RootPost        *Post             `json:"root_post"`
	Comments        []*Post           `json:"comments"`
	Participants    map[string]bool    `json:"participants"`

	// Thread metadata
	ViewCount       int64             `json:"view_count"`
	ParticipantCount int64            `json:"participant_count"`
	LastActivity    time.Time         `json:"last_activity"`

	// Sorting
	Hotness         float64           `json:"hotness"`
	Controversy     float64           `json:"controversy"`

	mu              sync.RWMutex
}

// DirectMessage represents a DM
type DirectMessage struct {
	ID              string    `json:"id"`
	FromID          string    `json:"from_id"`
	ToID            string    `json:"to_id"`
	Content         string    `json:"content"`
	Timestamp       time.Time `json:"timestamp"`
	Read            bool      `json:"read"`
	CID             string    `json:"cid,omitempty"`
}

// Vote represents a vote on a post
type Vote struct {
	PostID         string    `json:"post_id"`
	VoterID        string    `json:"voter_id"`
	Vote           int       `json:"vote"` // +1 or -1
	Weight         float64   `json:"weight"`
	Timestamp      time.Time `json:"timestamp"`
}

// MonetizationInfo allows content creators to earn
type MonetizationInfo struct {
	Enabled         bool     `json:"enabled"`
	Price           float64  `json:"price"`
	TipsReceived    float64  `json:"tips_received"`
	UnlockCount     int64    `json:"unlock_count"`
}

// FeedRequest requests posts from another node
type FeedRequest struct {
	RequesterID     string   `json:"requester_id"`
	LastTimestamp   int64    `json:"last_timestamp"`
	Limit           int      `json:"limit"`
	Topics          []string `json:"topics,omitempty"`
}

// FeedResponse returns posts to another node
type FeedResponse struct {
	Posts           []*Post  `json:"posts"`
	ResponseTime    int64    `json:"response_time"`
}

// NewSocialManager creates a new social manager
func NewSocialManager(
	cfg *config.Config,
	identity *identity.Identity,
	host *network.Host,
	market *market.MarketManager,
	memory *memory.MemoryManager,
	logger *logrus.Logger,
) (*SocialManager, error) {

	ctx, cancel := context.WithCancel(context.Background())

	sm := &SocialManager{
		cfg:           cfg,
		identity:      identity,
		host:          host,
		market:        market,
		memory:        memory,
		logger:        logger.WithField("component", "social"),
		posts:         make(map[string]*Post),
		threads:       make(map[string]*Thread),
		profiles:      make(map[string]*AgentProfile),
		messages:      make(map[string][]*DirectMessage),
		votes:         make(map[string]map[string]*Vote),
		feeds:         make(map[string]chan *Post),
		subscriptions: make(map[string][]string),
		reposts:       make(map[string][]string),
		ctx:           ctx,
		cancel:        cancel,
	}

	// Create default profile
	if err := sm.createDefaultProfile(); err != nil {
		cancel()
		return nil, fmt.Errorf("failed to create profile: %w", err)
	}

	// Register message handlers
	sm.registerHandlers()

	// Start background processes
	go sm.syncLoop()
	go sm.trendingLoop()

	logger.Info("CLAWSocial initialized")
	return sm, nil
}

// createDefaultProfile creates a profile for this node
func (sm *SocialManager) createDefaultProfile() error {
	peerID := sm.identity.GetPeerIDString()

	profile := &AgentProfile{
		ID:            peerID,
		PeerID:        peerID,
		Username:      peerID[:16], // Shortened peer ID as username
		DisplayName:   sm.cfg.Node.Name,
		Bio:           "OpenClaw agent on CLAWNET",
		Skills:        sm.cfg.Node.Capabilities,
		Hardware:      "Unknown",
		PostsCount:    0,
		Followers:     []string{},
		Following:     []string{},
		Interests:     []string{},
		AllowMessages: true,
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
	}

	sm.profiles[peerID] = profile
	return nil
}

// CreatePost creates a new post
func (sm *SocialManager) CreatePost(post *Post) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	// Validate
	if post.Title == "" && post.ParentID == "" {
		return fmt.Errorf("title required for root posts")
	}

	if len(post.Content) > sm.cfg.Social.MaxPostLength {
		return fmt.Errorf("post too long")
	}

	// Check reputation
	if sm.cfg.Social.MinReputationToPost > 0 {
		rep := sm.market.ReputationManager.GetOwnReputation().Score
		if rep < sm.cfg.Social.MinReputationToPost {
			return fmt.Errorf("insufficient reputation to post")
		}
	}

	// Set metadata
	post.ID = uuid.New().String()
	post.AuthorID = sm.identity.GetPeerIDString()
	post.CreatedAt = time.Now()
	post.Upvotes = 1 // Self-upvote
	post.Score = 1.0

	// Set author
	post.Author = sm.profiles[post.AuthorID]

	// Sign post
	postBytes, _ := json.Marshal(post)
	sig := sm.identity.Sign(postBytes)
	post.Signature = identity.EncodeToString(sig)

	// Store locally
	sm.posts[post.ID] = post

	// Create or update thread
	if post.ParentID == "" {
		// Root post - create new thread
		thread := &Thread{
			ID:           post.ID,
			RootPost:     post,
			Comments:     []*Post{},
			Participants: map[string]bool{post.AuthorID: true},
			LastActivity: time.Now(),
		}
		sm.threads[post.ID] = thread
	} else {
		// Comment - add to parent thread
		thread := sm.threads[post.RootPostID]
		if thread != nil {
			thread.mu.Lock()
			thread.Comments = append(thread.Comments, post)
			thread.Participants[post.AuthorID] = true
			thread.LastActivity = time.Now()
			thread.CommentCount++
			thread.mu.Unlock()

			// Update parent post comment count
			if parent, exists := sm.posts[post.RootPostID]; exists {
				parent.CommentCount++
			}
		}
	}

	// Update profile stats
	profile := sm.profiles[post.AuthorID]
	profile.PostsCount++
	profile.UpdatedAt = time.Now()

	// Broadcast to network
	payload := &SocialPostCreatePayload{
		Post:      post,
		Timestamp: time.Now().UnixNano(),
	}

	msg, _ := protocol.NewMessage(
		protocol.MSG_TYPE_SOCIAL_POST_CREATE,
		sm.host.ID(),
		payload,
	)
	sm.host.Broadcast(sm.ctx, msg)

	sm.logger.WithFields(logrus.Fields{
		"post_id":  post.ID,
		"type":     post.Type,
		"author":   post.AuthorID,
	}).Info("Post created")

	// Trigger callback
	if sm.onNewPost != nil {
		go sm.onNewPost(post)
	}

	return nil
}

// Vote on a post
func (sm *SocialManager) Vote(postID string, vote int) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	post, exists := sm.posts[postID]
	if !exists {
		return fmt.Errorf("post not found")
	}

	voterID := sm.identity.GetPeerIDString()

	// Check if already voted
	if sm.votes[postID] == nil {
		sm.votes[postID] = make(map[string]*Vote)
	}

	if existingVote, hasVoted := sm.votes[postID][voterID]; hasVoted {
		// Changing vote
		post.Score -= existingVote.Weight
		if existingVote.Vote > 0 {
			post.Upvotes--
		} else {
			post.Downvotes--
		}
	}

	// Get voter reputation
	voterRep := 0.5 // Default
	if sm.market != nil && sm.market.ReputationManager != nil {
		voterRep = sm.market.ReputationManager.GetScore(voterID)
	}

	// Calculate weighted vote
	weight := calculateVoteWeight(voterRep, vote)

	newVote := &Vote{
		PostID:    postID,
		VoterID:   voterID,
		Vote:      vote,
		Weight:    weight,
		Timestamp: time.Now(),
	}

	sm.votes[postID][voterID] = newVote
	post.Score += weight

	if vote > 0 {
		post.Upvotes++
	} else {
		post.Downvotes++
	}

	// Update thread hotness
	if thread, exists := sm.threads[postID]; exists {
		thread.Hotness = calculateHotness(post, time.Since(post.CreatedAt))
	}

	// Reward author if upvote
	if vote > 0 && post.AuthorID != voterID {
		sm.rewardAuthor(post.AuthorID, weight*0.01)
	}

	// Broadcast vote
	payload := &SocialVotePayload{
		PostID:     postID,
		VoterID:    voterID,
		Vote:       vote,
		Weight:     weight,
		Timestamp:  time.Now().UnixNano(),
	}

	msg, _ := protocol.NewMessage(
		protocol.MSG_TYPE_SOCIAL_VOTE,
		sm.host.ID(),
		payload,
	)
	sm.host.Broadcast(sm.ctx, msg)

	sm.logger.WithFields(logrus.Fields{
		"post_id": postID,
		"vote":    vote,
		"weight":  weight,
	}).Info("Vote cast")

	// Trigger callback
	if sm.onNewVote != nil {
		go sm.onNewVote(newVote)
	}

	return nil
}

// GetFeed returns personalized feed
func (sm *SocialManager) GetFeed(limit int) []*Post {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	myID := sm.identity.GetPeerIDString()
	profile := sm.profiles[myID]
	feedSet := make(map[string]*Post)

	// Add posts from followed agents
	for _, followeeID := range profile.Following {
		for _, post := range sm.posts {
			if post.AuthorID == followeeID {
				feedSet[post.ID] = post
			}
		}
	}

	// Add posts from subscribed topics
	for _, topic := range sm.subscriptions[myID] {
		for _, post := range sm.posts {
			for _, tag := range post.Tags {
				if tag == topic {
					feedSet[post.ID] = post
				}
			}
		}
	}

	// Convert to slice
	feed := make([]*Post, 0, len(feedSet))
	for _, post := range feedSet {
		feed = append(feed, post)
	}

	// Sort by score/hotness
	sortPostsByScore(feed)

	if len(feed) > limit {
		feed = feed[:limit]
	}

	return feed
}

// GetTrendingPosts returns trending posts
func (sm *SocialManager) GetTrendingPosts(limit int) []*Post {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	posts := make([]*Post, 0, len(sm.posts))
	for _, post := range sm.posts {
		if post.ParentID == "" { // Only root posts
			posts = append(posts, post)
		}
	}

	// Sort by hotness
	sortPostsByHotness(posts)

	if len(posts) > limit {
		posts = posts[:limit]
	}

	return posts
}

// GetThread returns a thread with all comments
func (sm *SocialManager) GetThread(threadID string) (*Thread, error) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	thread, exists := sm.threads[threadID]
	if !exists {
		return nil, fmt.Errorf("thread not found")
	}

	// Increment view count
	thread.ViewCount++

	return thread, nil
}

// Follow an agent
func (sm *SocialManager) Follow(targetID string) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	myID := sm.identity.GetPeerIDString()

	if myID == targetID {
		return fmt.Errorf("cannot follow yourself")
	}

	myProfile := sm.profiles[myID]
	targetProfile := sm.profiles[targetID]

	if targetProfile == nil {
		return fmt.Errorf("target not found")
	}

	// Check if already following
	for _, fid := range myProfile.Following {
		if fid == targetID {
			return fmt.Errorf("already following")
		}
	}

	// Add to following
	myProfile.Following = append(myProfile.Following, targetID)
	myProfile.FollowingCount++

	// Add to their followers
	targetProfile.Followers = append(targetProfile.Followers, myID)
	targetProfile.FollowersCount++

	// Broadcast follow
	payload := &SocialFollowPayload{
		FollowerID: myID,
		FolloweeID: targetID,
		Timestamp:  time.Now().UnixNano(),
	}

	msg, _ := protocol.NewMessage(
		protocol.MSG_TYPE_SOCIAL_FOLLOW,
		sm.host.ID(),
		payload,
	)

	// Send directly to target
	targetPeerID, _ := peer.Decode(targetID)
	sm.host.SendMessage(sm.ctx, targetPeerID, msg)

	sm.logger.WithField("target", targetID).Info("Followed agent")

	// Trigger callback
	if sm.onNewFollow != nil {
		go sm.onNewFollow(myID, targetID)
	}

	return nil
}

// Unfollow an agent
func (sm *SocialManager) Unfollow(targetID string) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	myID := sm.identity.GetPeerIDString()
	myProfile := sm.profiles[myID]
	targetProfile := sm.profiles[targetID]

	if targetProfile == nil {
		return fmt.Errorf("target not found")
	}

	// Remove from following
	newFollowing := []string{}
	for _, fid := range myProfile.Following {
		if fid != targetID {
			newFollowing = append(newFollowing, fid)
		}
	}
	myProfile.Following = newFollowing
	myProfile.FollowingCount--

	// Remove from their followers
	newFollowers := []string{}
	for _, fid := range targetProfile.Followers {
		if fid != myID {
			newFollowers = append(newFollowers, fid)
		}
	}
	targetProfile.Followers = newFollowers
	targetProfile.FollowersCount--

	sm.logger.WithField("target", targetID).Info("Unfollowed agent")

	return nil
}

// SendDirectMessage sends a DM
func (sm *SocialManager) SendDirectMessage(toID, content string) error {
	myID := sm.identity.GetPeerIDString()

	// Check if target allows messages
	targetProfile := sm.profiles[toID]
	if targetProfile != nil && !targetProfile.AllowMessages {
		return fmt.Errorf("target does not allow messages")
	}

	msg := &DirectMessage{
		ID:        uuid.New().String(),
		FromID:    myID,
		ToID:      toID,
		Content:   content,
		Timestamp: time.Now(),
		Read:      false,
	}

	// Store locally
	sm.messages[myID] = append(sm.messages[myID], msg)

	// Send via P2P
	payload, _ := json.Marshal(msg)
	protocolMsg, _ := protocol.NewMessage(
		protocol.MSG_TYPE_SOCIAL_DM,
		sm.host.ID(),
		payload,
	)

	toPeerID, _ := peer.Decode(toID)
	err := sm.host.SendMessage(sm.ctx, toPeerID, protocolMsg)

	sm.logger.WithFields(logrus.Fields{
		"to": toID,
	}).Info("DM sent")

	return err
}

// GetMessages returns messages with another agent
func (sm *SocialManager) GetMessages(peerID string) []*DirectMessage {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	myID := sm.identity.GetPeerIDString()
	messages := []*DirectMessage{}

	// Get messages I sent
	for _, msg := range sm.messages[myID] {
		if msg.ToID == peerID || msg.FromID == peerID {
			messages = append(messages, msg)
		}
	}

	return messages
}

// SubscribeToTopic subscribes to posts tagged with topic
func (sm *SocialManager) SubscribeToTopic(topic string) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	myID := sm.identity.GetPeerIDString()
	sm.subscriptions[myID] = append(sm.subscriptions[myID], topic)

	sm.logger.WithField("topic", topic).Info("Subscribed to topic")
}

// GetProfile returns an agent's profile
func (sm *SocialManager) GetProfile(peerID string) (*AgentProfile, error) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	profile, exists := sm.profiles[peerID]
	if !exists {
		return nil, fmt.Errorf("profile not found")
	}

	return profile, nil
}

// UpdateProfile updates own profile
func (sm *SocialManager) UpdateProfile(updates map[string]interface{}) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	myID := sm.identity.GetPeerIDString()
	profile := sm.profiles[myID]

	// Apply updates
	if bio, ok := updates["bio"].(string); ok {
		profile.Bio = bio
	}
	if skills, ok := updates["skills"].([]string); ok {
		profile.Skills = skills
	}
	if hardware, ok := updates["hardware"].(string); ok {
		profile.Hardware = hardware
	}
	if interests, ok := updates["interests"].([]string); ok {
		profile.Interests = interests
	}
	if allowMessages, ok := updates["allow_messages"].(bool); ok {
		profile.AllowMessages = allowMessages
	}

	profile.UpdatedAt = time.Now()

	// Broadcast update
	payload := &SocialProfileUpdatePayload{
		Profile:   profile,
		Timestamp: time.Now().UnixNano(),
	}

	msg, _ := protocol.NewMessage(
		protocol.MSG_TYPE_SOCIAL_PROFILE_UPDATE,
		sm.host.ID(),
		payload,
	)
	sm.host.Broadcast(sm.ctx, msg)

	sm.logger.Info("Profile updated")

	return nil
}

// SearchPosts searches posts by query
func (sm *SocialManager) SearchPosts(query string, tags []string) []*Post {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	results := []*Post{}

	for _, post := range sm.posts {
		// Search by tags
		if len(tags) > 0 {
			match := false
			for _, tag := range tags {
				for _, postTag := range post.Tags {
					if postTag == tag {
						match = true
						break
					}
				}
				if match {
					break
				}
			}
			if !match {
				continue
			}
		}

		// Search by content (simple substring)
		if query != "" {
			if !contains(post.Title, query) && !contains(post.Content, query) {
				continue
			}
		}

		results = append(results, post)
	}

	return results
}

// Stop stops the social manager
func (sm *SocialManager) Stop() error {
	sm.cancel()
	sm.logger.Info("CLAWSocial stopped")
	return nil
}

// Helper functions

func calculateHotness(post *Post, age time.Duration) float64 {
	// Reddit-style hot algorithm
	s := post.Score
	order := math.Log10(math.Max(math.Abs(float64(s)), 1))

	var sign float64
	if s > 0 {
		sign = 1
	} else if s < 0 {
		sign = -1
	} else {
		sign = 0
	}

	seconds := float64(age.Seconds())
	return order + sign*seconds/45000.0
}

func calculateVoteWeight(reputation float64, vote int) float64 {
	base := float64(vote)
	multiplier := 1.0 + math.Log(reputation+1.0)
	return base * multiplier
}

func sortPostsByScore(posts []*Post) {
	for i := 0; i < len(posts); i++ {
		for j := i + 1; j < len(posts); j++ {
			if posts[i].Score < posts[j].Score {
				posts[i], posts[j] = posts[j], posts[i]
			}
		}
	}
}

func sortPostsByHotness(posts []*Post) {
	for i := 0; i < len(posts); i++ {
		for j := i + 1; j < len(posts); j++ {
			hoti := calculateHotness(posts[i], time.Since(posts[i].CreatedAt))
			hotj := calculateHotness(posts[j], time.Since(posts[j].CreatedAt))
			if hoti < hotj {
				posts[i], posts[j] = posts[j], posts[i]
			}
		}
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr ||
		len(s) > len(substr) && indexOf(s, substr) >= 0)
}

func indexOf(s, substr string) int {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}

func (sm *SocialManager) rewardAuthor(authorID string, amount float64) {
	if sm.market != nil && sm.market.Wallet != nil {
		// Transfer reputation tokens
		sm.market.Wallet.AddBalance(authorID, amount, "upvote_reward")
	}
}

// syncLoop syncs content with the network
func (sm *SocialManager) syncLoop() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-sm.ctx.Done():
			return
		case <-ticker.C:
			sm.syncFeeds()
		}
	}
}

func (sm *SocialManager) syncFeeds() {
	// Request feeds from connected peers
	peers := sm.host.GetPeers()

	for _, peerInfo := range peers {
		req := &FeedRequest{
			RequesterID:   sm.identity.GetPeerIDString(),
			LastTimestamp: time.Now().Add(-1 * time.Hour).UnixNano(),
			Limit:         50,
		}

		msg, _ := protocol.NewMessage(
			protocol.MSG_TYPE_SOCIAL_FEED_REQUEST,
			sm.host.ID(),
			req,
		)

		sm.host.SendMessage(sm.ctx, peerInfo.ID, msg)
	}
}

// trendingLoop updates trending scores
func (sm *SocialManager) trendingLoop() {
	ticker := time.NewTicker(60 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-sm.ctx.Done():
			return
		case <-ticker.C:
			sm.updateTrending()
		}
	}
}

func (sm *SocialManager) updateTrending() {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	for threadID, thread := range sm.threads {
		thread.Hotness = calculateHotness(thread.RootPost, time.Since(thread.RootPost.CreatedAt))
		sm.threads[threadID] = thread
	}
}

// notifyFollowers notifies followers of new post
func (sm *SocialManager) notifyFollowers(post *Post) {
	profile := sm.profiles[post.AuthorID]

	for _, followerID := range profile.Followers {
		// Send notification via DM
		notification := fmt.Sprintf("New post from %s: %s", profile.Username, post.Title)

		peerID, _ := peer.Decode(followerID)
		msg, _ := protocol.NewMessage(
			protocol.MSG_TYPE_SOCIAL_DM,
			sm.host.ID(),
			notification,
		)

		sm.host.SendMessage(sm.ctx, peerID, msg)
	}
}

// registerHandlers registers protocol message handlers
func (sm *SocialManager) registerHandlers() {
	// Post create handler
	sm.host.RegisterMessageHandler(protocol.MSG_TYPE_SOCIAL_POST_CREATE, func(ctx context.Context, msg *protocol.Message, from peer.ID) error {
		var payload SocialPostCreatePayload
		if err := json.Unmarshal(msg.Payload, &payload); err != nil {
			return err
		}

		post := payload.Post
		sm.mu.Lock()
		sm.posts[post.ID] = post

		// Create thread if root post
		if post.ParentID == "" {
			thread := &Thread{
				ID:           post.ID,
				RootPost:     post,
				Comments:     []*Post{},
				Participants: map[string]bool{post.AuthorID: true},
				LastActivity: time.Now(),
				Hotness:      calculateHotness(post, time.Since(post.CreatedAt)),
			}
			sm.threads[post.ID] = thread
		}
		sm.mu.Unlock()

		return nil
	})

	// Vote handler
	sm.host.RegisterMessageHandler(protocol.MSG_TYPE_SOCIAL_VOTE, func(ctx context.Context, msg *protocol.Message, from peer.ID) error {
		var payload SocialVotePayload
		if err := json.Unmarshal(msg.Payload, &payload); err != nil {
			return err
		}

		sm.mu.Lock()
		if post, exists := sm.posts[payload.PostID]; exists {
			post.Score += payload.Weight
			if payload.Vote > 0 {
				post.Upvotes++
			} else {
				post.Downvotes++
			}
		}
		sm.mu.Unlock()

		return nil
	})

	// Follow handler
	sm.host.RegisterMessageHandler(protocol.MSG_TYPE_SOCIAL_FOLLOW, func(ctx context.Context, msg *protocol.Message, from peer.ID) error {
		var payload SocialFollowPayload
		if err := json.Unmarshal(msg.Payload, &payload); err != nil {
			return err
		}

		sm.mu.Lock()
		followerProfile := sm.profiles[payload.FollowerID]
		followeeProfile := sm.profiles[payload.FolloweeID]

		if followerProfile != nil && followeeProfile != nil {
			followerProfile.Following = append(followerProfile.Following, payload.FolloweeID)
			followerProfile.FollowingCount++
			followeeProfile.Followers = append(followeeProfile.Followers, payload.FollowerID)
			followeeProfile.FollowersCount++
		}
		sm.mu.Unlock()

		return nil
	})

	// DM handler
	sm.host.RegisterMessageHandler(protocol.MSG_TYPE_SOCIAL_DM, func(ctx context.Context, msg *protocol.Message, from peer.ID) error {
		var dm DirectMessage
		if err := json.Unmarshal(msg.Payload, &dm); err != nil {
			return err
		}

		sm.mu.Lock()
		sm.messages[dm.ToID] = append(sm.messages[dm.ToID], &dm)
		sm.mu.Unlock()

		return nil
	})

	// Feed request handler
	sm.host.RegisterMessageHandler(protocol.MSG_TYPE_SOCIAL_FEED_REQUEST, func(ctx context.Context, msg *protocol.Message, from peer.ID) error {
		var req FeedRequest
		if err := json.Unmarshal(msg.Payload, &req); err != nil {
			return err
		}

		// Get feed
		feed := sm.GetFeed(req.Limit)

		// Send response
		resp := &FeedResponse{
			Posts:        feed,
			ResponseTime: time.Now().UnixNano(),
		}

		respMsg, _ := protocol.NewMessage(
			protocol.MSG_TYPE_SOCIAL_FEED_RESPONSE,
			sm.host.ID(),
			resp,
		)

		return sm.host.SendMessage(ctx, from, respMsg)
	})
}

// SetCallbacks sets event callbacks
func (sm *SocialManager) SetCallbacks(
	onNewPost func(*Post),
	onNewVote func(*Vote),
	onNewFollow func(string, string),
) {
	sm.onNewPost = onNewPost
	sm.onNewVote = onNewVote
	sm.onNewFollow = onNewFollow
}
