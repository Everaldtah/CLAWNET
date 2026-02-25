package market

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/Everaldtah/CLAWNET/internal/config"
	"github.com/mitchellh/go-homedir"
	"go.etcd.io/bbolt"
)

const (
	walletDBName     = "wallet.db"
	walletBucketName = "wallets"
)

// Wallet manages node credits and transactions
type Wallet struct {
	mu sync.RWMutex

	nodeID          string
	balance         float64
	lockedBalance   float64
	transactions    []Transaction
	db              *bbolt.DB
	cfg             *config.Config
}

// Transaction represents a wallet transaction
type Transaction struct {
	ID          string    `json:"id"`
	Type        TxType    `json:"type"`
	Amount      float64   `json:"amount"`
	From        string    `json:"from"`
	To          string    `json:"to"`
	Timestamp   time.Time `json:"timestamp"`
	Description string    `json:"description"`
	TaskID      string    `json:"task_id,omitempty"`
	EscrowID    string    `json:"escrow_id,omitempty"`
}

// TxType represents transaction type
type TxType string

const (
	TxTypeCredit    TxType = "credit"
	TxTypeDebit     TxType = "debit"
	TxTypeLock      TxType = "lock"
	TxTypeUnlock    TxType = "unlock"
	TxTypeTransfer  TxType = "transfer"
	TxTypeEscrow    TxType = "escrow"
	TxTypeRelease   TxType = "release"
	TxTypeRefund    TxType = "refund"
	TxTypeReward    TxType = "reward"
	TxTypePenalty   TxType = "penalty"
)

// NewWallet creates or loads a wallet
func NewWallet(nodeID string, cfg *config.Config) (*Wallet, error) {
	home, err := homedir.Dir()
	if err != nil {
		return nil, fmt.Errorf("failed to get home directory: %w", err)
	}

	walletDir := filepath.Join(home, cfg.Node.DataDir)
	if err := os.MkdirAll(walletDir, 0700); err != nil {
		return nil, fmt.Errorf("failed to create wallet directory: %w", err)
	}

	dbPath := filepath.Join(walletDir, walletDBName)
	db, err := bbolt.Open(dbPath, 0600, &bbolt.Options{Timeout: 1 * time.Second})
	if err != nil {
		return nil, fmt.Errorf("failed to open wallet database: %w", err)
	}

	// Create bucket if not exists
	if err := db.Update(func(tx *bbolt.Tx) error {
		_, err := tx.CreateBucketIfNotExists([]byte(walletBucketName))
		return err
	}); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to create wallet bucket: %w", err)
	}

	w := &Wallet{
		nodeID:       nodeID,
		transactions: make([]Transaction, 0),
		db:           db,
		cfg:          cfg,
	}

	// Load or initialize balance
	if err := w.loadBalance(); err != nil {
		// Initialize with starting balance
		w.balance = cfg.Market.InitialWalletBalance
		if err := w.saveBalance(); err != nil {
			db.Close()
			return nil, fmt.Errorf("failed to save initial balance: %w", err)
		}
	}

	return w, nil
}

// GetBalance returns the available balance
func (w *Wallet) GetBalance(nodeID string) float64 {
	if nodeID == w.nodeID {
		w.mu.RLock()
		defer w.mu.RUnlock()
		return w.balance - w.lockedBalance
	}
	// For other nodes, would query network
	return 0
}

// GetTotalBalance returns total balance including locked
func (w *Wallet) GetTotalBalance() float64 {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.balance
}

// GetLockedBalance returns locked balance
func (w *Wallet) GetLockedBalance() float64 {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.lockedBalance
}

// Deposit adds credits to the wallet
func (w *Wallet) Deposit(nodeID string, amount float64) error {
	if nodeID != w.nodeID {
		return fmt.Errorf("cannot deposit to other node's wallet directly")
	}

	if amount <= 0 {
		return fmt.Errorf("amount must be positive")
	}

	w.mu.Lock()
	defer w.mu.Unlock()

	w.balance += amount

	// Record transaction
	tx := Transaction{
		ID:        generateTxID(),
		Type:      TxTypeCredit,
		Amount:    amount,
		To:        nodeID,
		Timestamp: time.Now(),
	}
	w.transactions = append(w.transactions, tx)

	return w.saveBalance()
}

// Withdraw removes credits from the wallet
func (w *Wallet) Withdraw(nodeID string, amount float64) error {
	if nodeID != w.nodeID {
		return fmt.Errorf("cannot withdraw from other node's wallet")
	}

	if amount <= 0 {
		return fmt.Errorf("amount must be positive")
	}

	w.mu.Lock()
	defer w.mu.Unlock()

	available := w.balance - w.lockedBalance
	if available < amount {
		return fmt.Errorf("insufficient balance: have %.2f, need %.2f", available, amount)
	}

	w.balance -= amount

	// Record transaction
	tx := Transaction{
		ID:        generateTxID(),
		Type:      TxTypeDebit,
		Amount:    amount,
		From:      nodeID,
		Timestamp: time.Now(),
	}
	w.transactions = append(w.transactions, tx)

	return w.saveBalance()
}

// LockFunds locks funds for escrow
func (w *Wallet) LockFunds(nodeID string, amount float64) error {
	if nodeID != w.nodeID {
		return fmt.Errorf("cannot lock other node's funds")
	}

	if amount <= 0 {
		return fmt.Errorf("amount must be positive")
	}

	w.mu.Lock()
	defer w.mu.Unlock()

	available := w.balance - w.lockedBalance
	if available < amount {
		return fmt.Errorf("insufficient balance: have %.2f, need %.2f", available, amount)
	}

	w.lockedBalance += amount

	// Record transaction
	tx := Transaction{
		ID:        generateTxID(),
		Type:      TxTypeLock,
		Amount:    amount,
		From:      nodeID,
		Timestamp: time.Now(),
	}
	w.transactions = append(w.transactions, tx)

	return w.saveBalance()
}

// UnlockFunds unlocks previously locked funds
func (w *Wallet) UnlockFunds(nodeID string, amount float64) error {
	if nodeID != w.nodeID {
		return fmt.Errorf("cannot unlock other node's funds")
	}

	if amount <= 0 {
		return fmt.Errorf("amount must be positive")
	}

	w.mu.Lock()
	defer w.mu.Unlock()

	if w.lockedBalance < amount {
		return fmt.Errorf("cannot unlock more than locked: locked %.2f, requested %.2f", w.lockedBalance, amount)
	}

	w.lockedBalance -= amount

	// Record transaction
	tx := Transaction{
		ID:        generateTxID(),
		Type:      TxTypeUnlock,
		Amount:    amount,
		To:        nodeID,
		Timestamp: time.Now(),
	}
	w.transactions = append(w.transactions, tx)

	return w.saveBalance()
}

// Transfer transfers funds between nodes
func (w *Wallet) Transfer(from, to string, amount float64) error {
	if amount <= 0 {
		return fmt.Errorf("amount must be positive")
	}

	// If we're the sender
	if from == w.nodeID {
		w.mu.Lock()
		defer w.mu.Unlock()

		available := w.balance - w.lockedBalance
		if available < amount {
			return fmt.Errorf("insufficient balance: have %.2f, need %.2f", available, amount)
		}

		w.balance -= amount

		// Record transaction
		tx := Transaction{
			ID:        generateTxID(),
			Type:      TxTypeTransfer,
			Amount:    amount,
			From:      from,
			To:        to,
			Timestamp: time.Now(),
		}
		w.transactions = append(w.transactions, tx)

		return w.saveBalance()
	}

	// If we're the receiver
	if to == w.nodeID {
		w.mu.Lock()
		defer w.mu.Unlock()

		w.balance += amount

		// Record transaction
		tx := Transaction{
			ID:        generateTxID(),
			Type:      TxTypeTransfer,
			Amount:    amount,
			From:      from,
			To:        to,
			Timestamp: time.Now(),
		}
		w.transactions = append(w.transactions, tx)

		return w.saveBalance()
	}

	return fmt.Errorf("node not involved in transfer")
}

// AddReward adds a reward for task completion
func (w *Wallet) AddReward(taskID string, amount float64, description string) error {
	if amount <= 0 {
		return fmt.Errorf("amount must be positive")
	}

	w.mu.Lock()
	defer w.mu.Unlock()

	w.balance += amount

	// Record transaction
	tx := Transaction{
		ID:          generateTxID(),
		Type:        TxTypeReward,
		Amount:      amount,
		To:          w.nodeID,
		Timestamp:   time.Now(),
		Description: description,
		TaskID:      taskID,
	}
	w.transactions = append(w.transactions, tx)

	return w.saveBalance()
}

// ApplyPenalty applies a penalty for misbehavior
func (w *Wallet) ApplyPenalty(reason string, amount float64) error {
	if amount <= 0 {
		return fmt.Errorf("amount must be positive")
	}

	w.mu.Lock()
	defer w.mu.Unlock()

	available := w.balance - w.lockedBalance
	if available < amount {
		amount = available // Apply maximum available
	}

	if amount > 0 {
		w.balance -= amount

		// Record transaction
		tx := Transaction{
			ID:          generateTxID(),
			Type:        TxTypePenalty,
			Amount:      amount,
			From:        w.nodeID,
			Timestamp:   time.Now(),
			Description: reason,
		}
		w.transactions = append(w.transactions, tx)
	}

	return w.saveBalance()
}

// GetTransactions returns transaction history
func (w *Wallet) GetTransactions(limit int) []Transaction {
	w.mu.RLock()
	defer w.mu.RUnlock()

	if limit <= 0 || limit > len(w.transactions) {
		limit = len(w.transactions)
	}

	// Return most recent transactions
	start := len(w.transactions) - limit
	if start < 0 {
		start = 0
	}

	result := make([]Transaction, limit)
	copy(result, w.transactions[start:])

	// Reverse to get newest first
	for i, j := 0, len(result)-1; i < j; i, j = i+1, j-1 {
		result[i], result[j] = result[j], result[i]
	}

	return result
}

// loadBalance loads balance from database
func (w *Wallet) loadBalance() error {
	return w.db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte(walletBucketName))
		if b == nil {
			return fmt.Errorf("wallet bucket not found")
		}

		data := b.Get([]byte(w.nodeID))
		if data == nil {
			return fmt.Errorf("wallet not found")
		}

		var state struct {
			Balance       float64       `json:"balance"`
			LockedBalance float64       `json:"locked_balance"`
			Transactions  []Transaction `json:"transactions"`
		}

		if err := json.Unmarshal(data, &state); err != nil {
			return err
		}

		w.balance = state.Balance
		w.lockedBalance = state.LockedBalance
		w.transactions = state.Transactions

		return nil
	})
}

// saveBalance saves balance to database
func (w *Wallet) saveBalance() error {
	state := struct {
		Balance       float64       `json:"balance"`
		LockedBalance float64       `json:"locked_balance"`
		Transactions  []Transaction `json:"transactions"`
	}{
		Balance:       w.balance,
		LockedBalance: w.lockedBalance,
		Transactions:  w.transactions,
	}

	data, err := json.Marshal(state)
	if err != nil {
		return err
	}

	return w.db.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte(walletBucketName))
		if b == nil {
			return fmt.Errorf("wallet bucket not found")
		}
		return b.Put([]byte(w.nodeID), data)
	})
}

// Close closes the wallet database
func (w *Wallet) Close() error {
	return w.db.Close()
}

// generateTxID generates a unique transaction ID
func generateTxID() string {
	return fmt.Sprintf("tx_%d", time.Now().UnixNano())
}

// PricingEngine calculates task pricing
type PricingEngine struct {
	cfg *config.Config
}

// NewPricingEngine creates a new pricing engine
func NewPricingEngine(cfg *config.Config) *PricingEngine {
	return &PricingEngine{cfg: cfg}
}

// CalculateTaskPrice calculates the price for a task
func (pe *PricingEngine) CalculateTaskPrice(taskType protocol.TaskType, complexity float64) float64 {
	// Base price by task type
	basePrice := 0.0
	switch taskType {
	case protocol.TaskTypeOpenClawPrompt:
		basePrice = 1.0
	case protocol.TaskTypeShellTask:
		basePrice = 0.5
	case protocol.TaskTypeSwarmAI:
		basePrice = 5.0
	case protocol.TaskTypeCompute:
		basePrice = 2.0
	case protocol.TaskTypeStorage:
		basePrice = 0.1
	default:
		basePrice = 1.0
	}

	// Adjust by complexity (0.0 to 1.0)
	price := basePrice * (1.0 + complexity*2.0)

	// Round to minimum increment
	increment := pe.cfg.Market.MinBidIncrement
	price = math.Round(price/increment) * increment

	return price
}

// EstimateComplexity estimates task complexity
func (pe *PricingEngine) EstimateComplexity(taskType protocol.TaskType, dataSize int) float64 {
	// Base complexity
	complexity := 0.5

	// Adjust by data size
	if dataSize > 1024*1024 { // > 1MB
		complexity += 0.3
	} else if dataSize > 100*1024 { // > 100KB
		complexity += 0.2
	} else if dataSize > 10*1024 { // > 10KB
		complexity += 0.1
	}

	// Task type adjustments
	switch taskType {
	case protocol.TaskTypeSwarmAI:
		complexity += 0.3
	case protocol.TaskTypeCompute:
		complexity += 0.2
	}

	if complexity > 1.0 {
		complexity = 1.0
	}

	return complexity
}
