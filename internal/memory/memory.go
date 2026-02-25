package memory

import (
	"bytes"
	"compress/gzip"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"sync"
	"time"

	"github.com/Everaldtah/CLAWNET/internal/config"
	"github.com/Everaldtah/CLAWNET/internal/identity"
	"github.com/Everaldtah/CLAWNET/internal/protocol"
	"github.com/klauspost/compress/zstd"
	"go.etcd.io/bbolt"
)

const (
	memoryDBName     = "memory.db"
	memoryBucketName = "memory"
)

// MemoryManager manages distributed memory
type MemoryManager struct {
	mu sync.RWMutex

	nodeID    string
	db        *bbolt.DB
	cfg       *config.Config
	identity  *identity.Identity

	// Memory cache
	cache     map[string]*MemoryEntry
	cacheMu   sync.RWMutex

	// Sync tracking
	syncPeers map[string]time.Time
	lastSync  time.Time

	// Channels
	syncReqChan chan *SyncRequest
	updateChan  chan *MemoryEntry

	ctx       context.Context
	cancel    context.CancelFunc
}

// MemoryEntry represents a memory entry
type MemoryEntry struct {
	Key       string          `json:"key"`
	Value     json.RawMessage `json:"value"`
	Timestamp time.Time       `json:"timestamp"`
	TTL       time.Duration   `json:"ttl"`
	Version   uint64          `json:"version"`
	Owner     string          `json:"owner"`
	Signature string          `json:"signature"`
	Encrypted bool            `json:"encrypted"`
}

// SyncRequest represents a sync request
type SyncRequest struct {
	RequesterID  string
	LastSyncTime time.Time
	Keys         []string
	ResponseChan chan *SyncResponse
}

// SyncResponse represents a sync response
type SyncResponse struct {
	Entries     []*MemoryEntry
	DeletedKeys []string
	SyncTime    time.Time
}

// NewMemoryManager creates a new memory manager
func NewMemoryManager(nodeID string, cfg *config.Config, identity *identity.Identity) (*MemoryManager, error) {
	ctx, cancel := context.WithCancel(context.Background())

	mm := &MemoryManager{
		nodeID:      nodeID,
		cfg:         cfg,
		identity:    identity,
		cache:       make(map[string]*MemoryEntry),
		syncPeers:   make(map[string]time.Time),
		syncReqChan: make(chan *SyncRequest, 100),
		updateChan:  make(chan *MemoryEntry, 1000),
		lastSync:    time.Now(),
		ctx:         ctx,
		cancel:      cancel,
	}

	// Open database
	dbPath := fmt.Sprintf("%s/%s", cfg.GetConfigDir(), memoryDBName)
	db, err := bbolt.Open(dbPath, 0600, &bbolt.Options{Timeout: 1 * time.Second})
	if err != nil {
		cancel()
		return nil, fmt.Errorf("failed to open memory database: %w", err)
	}

	// Create bucket
	if err := db.Update(func(tx *bbolt.Tx) error {
		_, err := tx.CreateBucketIfNotExists([]byte(memoryBucketName))
		return err
	}); err != nil {
		db.Close()
		cancel()
		return nil, fmt.Errorf("failed to create memory bucket: %w", err)
	}

	mm.db = db

	// Load cache
	if err := mm.loadCache(); err != nil {
		// Continue with empty cache
	}

	// Start background processes
	go mm.syncProcessor()
	go mm.updateProcessor()
	go mm.gcProcessor()

	return mm, nil
}

// Get retrieves a memory entry
func (mm *MemoryManager) Get(key string) (*MemoryEntry, error) {
	// Check cache first
	mm.cacheMu.RLock()
	if entry, exists := mm.cache[key]; exists {
		mm.cacheMu.RUnlock()
		if entry.isExpired() {
			return nil, fmt.Errorf("key expired")
		}
		return entry, nil
	}
	mm.cacheMu.RUnlock()

	// Load from database
	var entry MemoryEntry
	err := mm.db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte(memoryBucketName))
		if b == nil {
			return fmt.Errorf("memory bucket not found")
		}

		data := b.Get([]byte(key))
		if data == nil {
			return fmt.Errorf("key not found")
		}

		// Decompress if needed
		decompressed, err := mm.decompress(data)
		if err != nil {
			return err
		}

		return json.Unmarshal(decompressed, &entry)
	})

	if err != nil {
		return nil, err
	}

	if entry.isExpired() {
		return nil, fmt.Errorf("key expired")
	}

	// Decrypt if needed
	if entry.Encrypted && mm.cfg.Memory.EncryptionEnabled {
		decrypted, err := mm.decrypt(entry.Value)
		if err != nil {
			return nil, fmt.Errorf("failed to decrypt: %w", err)
		}
		entry.Value = decrypted
	}

	// Add to cache
	mm.cacheMu.Lock()
	mm.cache[key] = &entry
	mm.cacheMu.Unlock()

	return &entry, nil
}

// Set stores a memory entry
func (mm *MemoryManager) Set(key string, value interface{}, ttl time.Duration) error {
	data, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("failed to marshal value: %w", err)
	}

	// Encrypt if enabled
	encrypted := false
	if mm.cfg.Memory.EncryptionEnabled {
		encryptedData, err := mm.encrypt(data)
		if err != nil {
			return fmt.Errorf("failed to encrypt: %w", err)
		}
		data = encryptedData
		encrypted = true
	}

	entry := &MemoryEntry{
		Key:       key,
		Value:     data,
		Timestamp: time.Now(),
		TTL:       ttl,
		Version:   1,
		Owner:     mm.nodeID,
		Encrypted: encrypted,
	}

	// Sign entry
	sig := mm.identity.Sign([]byte(key + string(data)))
	entry.Signature = identity.EncodeToString(sig)

	// Store in database
	if err := mm.storeEntry(entry); err != nil {
		return err
	}

	// Update cache
	mm.cacheMu.Lock()
	mm.cache[key] = entry
	mm.cacheMu.Unlock()

	// Broadcast update
	select {
	case mm.updateChan <- entry:
	default:
	}

	return nil
}

// Delete deletes a memory entry
func (mm *MemoryManager) Delete(key string) error {
	// Remove from cache
	mm.cacheMu.Lock()
	delete(mm.cache, key)
	mm.cacheMu.Unlock()

	// Remove from database
	return mm.db.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte(memoryBucketName))
		if b == nil {
			return fmt.Errorf("memory bucket not found")
		}
		return b.Delete([]byte(key))
	})
}

// List lists all memory keys
func (mm *MemoryManager) List(prefix string) ([]string, error) {
	var keys []string

	err := mm.db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte(memoryBucketName))
		if b == nil {
			return fmt.Errorf("memory bucket not found")
		}

		return b.ForEach(func(k, v []byte) error {
			key := string(k)
			if prefix == "" || len(key) >= len(prefix) && key[:len(prefix)] == prefix {
				keys = append(keys, key)
			}
			return nil
		})
	})

	return keys, err
}

// SyncRequest handles a sync request from a peer
func (mm *MemoryManager) SyncRequest(req *SyncRequest) (*SyncResponse, error) {
	var entries []*MemoryEntry
	var deletedKeys []string

	// Get entries modified since last sync
	err := mm.db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte(memoryBucketName))
		if b == nil {
			return fmt.Errorf("memory bucket not found")
		}

		return b.ForEach(func(k, v []byte) error {
			var entry MemoryEntry
			decompressed, err := mm.decompress(v)
			if err != nil {
				return nil // Skip corrupted entries
			}

			if err := json.Unmarshal(decompressed, &entry); err != nil {
				return nil // Skip invalid entries
			}

			// Check if modified since last sync
			if entry.Timestamp.After(req.LastSyncTime) {
				// Check if key was requested
				if len(req.Keys) == 0 || contains(req.Keys, entry.Key) {
					entries = append(entries, &entry)
				}
			}

			return nil
		})
	})

	if err != nil {
		return nil, err
	}

	return &SyncResponse{
		Entries:     entries,
		DeletedKeys: deletedKeys,
		SyncTime:    time.Now(),
	}, nil
}

// SyncWithPeer syncs memory with a peer
func (mm *MemoryManager) SyncWithPeer(peerID string, lastSync time.Time) error {
	req := &SyncRequest{
		RequesterID:  mm.nodeID,
		LastSyncTime: lastSync,
		ResponseChan: make(chan *SyncResponse, 1),
	}

	// This would send the request to the peer and wait for response
	// For now, just update sync tracking
	mm.mu.Lock()
	mm.syncPeers[peerID] = time.Now()
	mm.mu.Unlock()

	return nil
}

// storeEntry stores an entry in the database
func (mm *MemoryManager) storeEntry(entry *MemoryEntry) error {
	data, err := json.Marshal(entry)
	if err != nil {
		return err
	}

	// Compress
	compressed, err := mm.compress(data)
	if err != nil {
		return err
	}

	return mm.db.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte(memoryBucketName))
		if b == nil {
			return fmt.Errorf("memory bucket not found")
		}
		return b.Put([]byte(entry.Key), compressed)
	})
}

// compress compresses data
func (mm *MemoryManager) compress(data []byte) ([]byte, error) {
	var buf bytes.Buffer
	
	level := mm.cfg.Memory.CompressionLevel
	if level < 1 || level > 9 {
		level = 6
	}

	writer, err := gzip.NewWriterLevel(&buf, level)
	if err != nil {
		return nil, err
	}

	if _, err := writer.Write(data); err != nil {
		return nil, err
	}

	if err := writer.Close(); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

// decompress decompresses data
func (mm *MemoryManager) decompress(data []byte) ([]byte, error) {
	reader, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	defer reader.Close()

	return io.ReadAll(reader)
}

// encrypt encrypts data using AES-GCM
func (mm *MemoryManager) encrypt(data []byte) ([]byte, error) {
	// Derive key from identity
	key := sha256.Sum256(mm.identity.GetPublicKeyBytes())

	block, err := aes.NewCipher(key[:])
	if err != nil {
		return nil, err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, err
	}

	return gcm.Seal(nonce, nonce, data, nil), nil
}

// decrypt decrypts data using AES-GCM
func (mm *MemoryManager) decrypt(data []byte) ([]byte, error) {
	key := sha256.Sum256(mm.identity.GetPublicKeyBytes())

	block, err := aes.NewCipher(key[:])
	if err != nil {
		return nil, err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	nonceSize := gcm.NonceSize()
	if len(data) < nonceSize {
		return nil, fmt.Errorf("ciphertext too short")
	}

	nonce, ciphertext := data[:nonceSize], data[nonceSize:]
	return gcm.Open(nil, nonce, ciphertext, nil)
}

// loadCache loads entries into cache
func (mm *MemoryManager) loadCache() error {
	return mm.db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte(memoryBucketName))
		if b == nil {
			return nil
		}

		return b.ForEach(func(k, v []byte) error {
			var entry MemoryEntry
			decompressed, err := mm.decompress(v)
			if err != nil {
				return nil
			}

			if err := json.Unmarshal(decompressed, &entry); err != nil {
				return nil
			}

			if !entry.isExpired() {
				mm.cache[string(k)] = &entry
			}

			return nil
		})
	})
}

// syncProcessor processes sync requests
func (mm *MemoryManager) syncProcessor() {
	ticker := time.NewTicker(mm.cfg.Memory.SyncInterval)
	defer ticker.Stop()

	for {
		select {
		case req := <-mm.syncReqChan:
			// Process sync request
			_ = req
		case <-ticker.C:
			// Periodic sync
			mm.lastSync = time.Now()
		case <-mm.ctx.Done():
			return
		}
	}
}

// updateProcessor processes updates
func (mm *MemoryManager) updateProcessor() {
	for {
		select {
		case entry := <-mm.updateChan:
			// Process update - would broadcast to peers
			_ = entry
		case <-mm.ctx.Done():
			return
		}
	}
}

// gcProcessor runs garbage collection
func (mm *MemoryManager) gcProcessor() {
	ticker := time.NewTicker(mm.cfg.Memory.GCInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			mm.gc()
		case <-mm.ctx.Done():
			return
		}
	}
}

// gc removes expired entries
func (mm *MemoryManager) gc() {
	mm.cacheMu.Lock()
	for key, entry := range mm.cache {
		if entry.isExpired() {
			delete(mm.cache, key)
			mm.Delete(key)
		}
	}
	mm.cacheMu.Unlock()
}

// isExpired checks if entry is expired
func (e *MemoryEntry) isExpired() bool {
	if e.TTL <= 0 {
		return false
	}
	return time.Since(e.Timestamp) > e.TTL
}

// contains checks if slice contains string
func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

// Close closes the memory manager
func (mm *MemoryManager) Close() error {
	mm.cancel()
	return mm.db.Close()
}
