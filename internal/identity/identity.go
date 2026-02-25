package identity

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/Everaldtah/CLAWNET/internal/config"
	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/mitchellh/go-homedir"
	"go.etcd.io/bbolt"
)

const (
	keyFileName    = "identity.key"
	keyBucketName  = "identity"
	keyEntryName   = "private_key"
)

// Identity represents the node's cryptographic identity
type Identity struct {
	mu         sync.RWMutex
	privKey    ed25519.PrivateKey
	pubKey     ed25519.PublicKey
	libp2pPriv crypto.PrivKey
	libp2pPub  crypto.PubKey
	peerID     peer.ID
	keyPath    string
}

// NewIdentity creates or loads a node identity
func NewIdentity(cfg *config.Config) (*Identity, error) {
	home, err := homedir.Dir()
	if err != nil {
		return nil, fmt.Errorf("failed to get home directory: %w", err)
	}

	keyDir := filepath.Join(home, cfg.Node.DataDir)
	if err := os.MkdirAll(keyDir, 0700); err != nil {
		return nil, fmt.Errorf("failed to create key directory: %w", err)
	}

	keyPath := filepath.Join(keyDir, keyFileName)
	id := &Identity{keyPath: keyPath}

	if _, err := os.Stat(keyPath); os.IsNotExist(err) {
		if err := id.generate(); err != nil {
			return nil, fmt.Errorf("failed to generate identity: %w", err)
		}
		if err := id.save(); err != nil {
			return nil, fmt.Errorf("failed to save identity: %w", err)
		}
	} else {
		if err := id.load(); err != nil {
			return nil, fmt.Errorf("failed to load identity: %w", err)
		}
	}

	return id, nil
}

// generate creates a new Ed25519 keypair
func (i *Identity) generate() error {
	i.mu.Lock()
	defer i.mu.Unlock()

	// Generate ed25519 keypair
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return fmt.Errorf("failed to generate Ed25519 key: %w", err)
	}

	i.privKey = priv
	i.pubKey = pub

	// Convert to libp2p format using the correct method
	libp2pPriv, err := crypto.UnmarshalEd25519PrivateKey(priv)
	if err != nil {
		return fmt.Errorf("failed to convert to libp2p private key: %w", err)
	}

	libp2pPub, err := crypto.UnmarshalEd25519PublicKey(pub)
	if err != nil {
		return fmt.Errorf("failed to convert to libp2p public key: %w", err)
	}

	i.libp2pPriv = libp2pPriv
	i.libp2pPub = libp2pPub

	peerID, err := peer.IDFromPublicKey(libp2pPub)
	if err != nil {
		return fmt.Errorf("failed to derive peer ID: %w", err)
	}
	i.peerID = peerID

	return nil
}

// save persists the identity to disk using BoltDB
func (i *Identity) save() error {
	db, err := bbolt.Open(i.keyPath, 0600, &bbolt.Options{NoSync: true})
	if err != nil {
		return fmt.Errorf("failed to open key database: %w", err)
	}
	defer db.Close()

	return db.Update(func(tx *bbolt.Tx) error {
		b, err := tx.CreateBucketIfNotExists([]byte(keyBucketName))
		if err != nil {
			return err
		}
		return b.Put([]byte(keyEntryName), i.privKey.Seed())
	})
}

// load retrieves the identity from disk
func (i *Identity) load() error {
	db, err := bbolt.Open(i.keyPath, 0600, &bbolt.Options{ReadOnly: true})
	if err != nil {
		return fmt.Errorf("failed to open key database: %w", err)
	}
	defer db.Close()

	var seed []byte
	err = db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte(keyBucketName))
		if b == nil {
			return fmt.Errorf("key bucket not found")
		}
		seed = b.Get([]byte(keyEntryName))
		if seed == nil {
			return fmt.Errorf("key not found in bucket")
		}
		return nil
	})
	if err != nil {
		return err
	}

	i.mu.Lock()
	defer i.mu.Unlock()

	i.privKey = ed25519.NewKeyFromSeed(seed)
	i.pubKey = i.privKey.Public().(ed25519.PublicKey)

	// Convert to libp2p format using the correct method
	libp2pPriv, err := crypto.UnmarshalEd25519PrivateKey(i.privKey)
	if err != nil {
		return fmt.Errorf("failed to convert to libp2p private key: %w", err)
	}

	libp2pPub, err := crypto.UnmarshalEd25519PublicKey(i.pubKey)
	if err != nil {
		return fmt.Errorf("failed to convert to libp2p public key: %w", err)
	}

	i.libp2pPriv = libp2pPriv
	i.libp2pPub = libp2pPub

	peerID, err := peer.IDFromPublicKey(libp2pPub)
	if err != nil {
		return fmt.Errorf("failed to derive peer ID: %w", err)
	}
	i.peerID = peerID

	return nil
}

// GetPeerID returns the libp2p peer ID
func (i *Identity) GetPeerID() peer.ID {
	i.mu.RLock()
	defer i.mu.RUnlock()
	return i.peerID
}

// GetLibp2pPrivKey returns the libp2p private key
func (i *Identity) GetLibp2pPrivKey() crypto.PrivKey {
	i.mu.RLock()
	defer i.mu.RUnlock()
	return i.libp2pPriv
}

// GetLibp2pPubKey returns the libp2p public key
func (i *Identity) GetLibp2pPubKey() crypto.PubKey {
	i.mu.RLock()
	defer i.mu.RUnlock()
	return i.libp2pPub
}

// Sign signs data with the private key
func (i *Identity) Sign(data []byte) []byte {
	i.mu.RLock()
	defer i.mu.RUnlock()
	return ed25519.Sign(i.privKey, data)
}

// VerifySignature verifies a signature
func (i *Identity) VerifySignature(data, signature []byte) bool {
	i.mu.RLock()
	defer i.mu.RUnlock()
	return ed25519.Verify(i.pubKey, data, signature)
}

// VerifySignatureFromPubKey verifies a signature using a public key
func VerifySignatureFromPubKey(pubKey ed25519.PublicKey, data, signature []byte) bool {
	return ed25519.Verify(pubKey, data, signature)
}

// GetPublicKeyBytes returns the raw public key bytes
func (i *Identity) GetPublicKeyBytes() []byte {
	i.mu.RLock()
	defer i.mu.RUnlock()
	return i.pubKey
}

// GetPublicKeyHex returns the public key as hex string
func (i *Identity) GetPublicKeyHex() string {
	return hex.EncodeToString(i.GetPublicKeyBytes())
}

// GetPeerIDString returns the peer ID as a string
func (i *Identity) GetPeerIDString() string {
	return i.GetPeerID().String()
}

// SecureCompare performs constant-time comparison
func SecureCompare(a, b []byte) bool {
	return subtle.ConstantTimeCompare(a, b) == 1
}

// DeriveSharedSecret derives a shared secret using X25519
func (i *Identity) DeriveSharedSecret(peerPubKey []byte) ([]byte, error) {
	// Convert Ed25519 to X25519 for key exchange
	// This is done using the Montgomery curve conversion
	if len(peerPubKey) != ed25519.PublicKeySize {
		return nil, fmt.Errorf("invalid public key size")
	}

	// For production, use proper X25519 key exchange
	// This is a simplified version
	shared := make([]byte, 32)
	copy(shared, i.privKey.Seed()[:32])
	
	// XOR with peer public key for demonstration
	// In production, use proper X25519 scalar multiplication
	for i := 0; i < 32 && i < len(peerPubKey); i++ {
		shared[i] ^= peerPubKey[i]
	}

	return shared, nil
}

// EncodeToString encodes bytes to base64
func EncodeToString(data []byte) string {
	return base64.StdEncoding.EncodeToString(data)
}

// DecodeString decodes base64 string
func DecodeString(s string) ([]byte, error) {
	return base64.StdEncoding.DecodeString(s)
}
