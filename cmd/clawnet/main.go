package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/Everaldtah/CLAWNET/internal/config"
	"github.com/Everaldtah/CLAWNET/internal/identity"
	"github.com/Everaldtah/CLAWNET/internal/market"
	"github.com/Everaldtah/CLAWNET/internal/memory"
	"github.com/Everaldtah/CLAWNET/internal/network"
	"github.com/Everaldtah/CLAWNET/internal/protocol"
	"github.com/Everaldtah/CLAWNET/internal/social"
	"github.com/Everaldtah/CLAWNET/internal/task"
	"github.com/Everaldtah/CLAWNET/internal/tui"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var (
	cfgFile string
	logger  *logrus.Logger
)

// rootCmd represents the base command
var rootCmd = &cobra.Command{
	Use:   "clawnet",
	Short: "CLAWNET - Distributed AI Mesh Network",
	Long: `CLAWNET is a secure distributed AI mesh network with autonomous task market.
	
Features:
  - P2P networking with libp2p
  - Encrypted communication via Noise protocol
  - Autonomous task market with auctions and bidding
  - Memory synchronization
  - OpenClaw AI integration
  - Swarm intelligence`,
	Run: runNode,
}

// versionCmd prints version
var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print version information",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("CLAWNET v1.0.0")
		fmt.Println("Protocol: /clawnet/1.0.0")
		fmt.Println("Build: production")
	},
}

// initCmd initializes configuration
var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize CLAWNET configuration",
	Run: func(cmd *cobra.Command, args []string) {
		cfg := config.DefaultConfig()
		if err := cfg.Save(""); err != nil {
			logger.WithError(err).Fatal("Failed to save configuration")
		}
		fmt.Println("Configuration initialized at ~/.clawnet/config.yaml")
	},
}

// Node represents the CLAWNET node
type Node struct {
	config     *config.Config
	identity   *identity.Identity
	host       *network.Host
	market     *market.MarketManager
	memory     *memory.MemoryManager
	social     *social.SocialManager
	executor   *task.Executor
	logger     *logrus.Logger
	ctx        context.Context
	cancel     context.CancelFunc
}

func init() {
	cobra.OnInitialize(initConfig)

	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.clawnet/config.yaml)")
	rootCmd.PersistentFlags().String("log-level", "info", "log level (debug, info, warn, error)")

	rootCmd.AddCommand(versionCmd)
	rootCmd.AddCommand(initCmd)

	// Initialize logger
	logger = logrus.New()
	logger.SetFormatter(&logrus.JSONFormatter{})
}

func initConfig() {
	logLevel, _ := rootCmd.Flags().GetString("log-level")
	level, err := logrus.ParseLevel(logLevel)
	if err != nil {
		level = logrus.InfoLevel
	}
	logger.SetLevel(level)
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func runNode(cmd *cobra.Command, args []string) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Load configuration
	cfg, err := config.Load(cfgFile)
	if err != nil {
		logger.WithError(err).Fatal("Failed to load configuration")
	}

	logger.Info("Starting CLAWNET node...")

	// Create identity
	id, err := identity.NewIdentity(cfg)
	if err != nil {
		logger.WithError(err).Fatal("Failed to create identity")
	}

	logger.WithField("peer_id", id.GetPeerIDString()).Info("Identity loaded")

	// Create network host
	host, err := network.NewHost(cfg, id, logger)
	if err != nil {
		logger.WithError(err).Fatal("Failed to create host")
	}

	// Start host
	if err := host.Start(); err != nil {
		logger.WithError(err).Fatal("Failed to start host")
	}

	logger.WithField("addrs", host.Addrs()).Info("Host started")

	// Create memory manager
	mem, err := memory.NewMemoryManager(id.GetPeerIDString(), cfg, id)
	if err != nil {
		logger.WithError(err).Warn("Failed to create memory manager - continuing without memory features")
		mem = nil
	}

	// Create market manager
	mkt, err := market.NewMarketManager(cfg, id, logger)
	if err != nil {
		logger.WithError(err).Warn("Failed to create market manager - continuing without market features")
		mkt = nil
	}

	// Create social manager
	var soc *social.SocialManager
	if cfg.Social.Enabled {
		soc, err = social.NewSocialManager(cfg, id, host, mkt, mem, logger)
		if err != nil {
			logger.WithError(err).Warn("Failed to create social manager")
		} else {
			logger.Info("CLAWSocial initialized")
		}
	}

	// Create task executor
	exec, err := task.NewExecutor(cfg, id, logger)
	if err != nil {
		logger.WithError(err).Fatal("Failed to create executor")
	}

	// Create node
	node := &Node{
		config:   cfg,
		identity: id,
		host:     host,
		market:   mkt,
		memory:   mem,
		social:   soc,
		executor: exec,
		logger:   logger,
		ctx:      ctx,
		cancel:   cancel,
	}

	// Register message handlers
	node.registerHandlers()

	// Register task handlers
	node.registerTaskHandlers()

	// Setup graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Start TUI if enabled
	if cfg.TUI.Enabled {
		go func() {
			if err := tui.Run(host, mkt, soc, logger); err != nil {
				logger.WithError(err).Error("TUI error")
			}
			cancel()
		}()
	}

	logger.Info("CLAWNET node running. Press Ctrl+C to stop.")

	// Wait for shutdown signal
	select {
	case <-sigChan:
		logger.Info("Shutdown signal received")
	case <-ctx.Done():
		logger.Info("Context cancelled")
	}

	// Shutdown
	node.shutdown()
}

func (n *Node) registerHandlers() {
	// Register ping handler
	n.host.RegisterMessageHandler(protocol.MSG_TYPE_PING, func(ctx context.Context, msg *protocol.Message, from peer.ID) error {
		// Send pong
		pong, _ := protocol.NewMessage(protocol.MSG_TYPE_PONG, n.host.ID(), struct{}{})
		return n.host.SendMessage(ctx, from, pong)
	})

	// Register pong handler
	n.host.RegisterMessageHandler(protocol.MSG_TYPE_PONG, func(ctx context.Context, msg *protocol.Message, from peer.ID) error {
		n.logger.WithField("peer", from.String()).Debug("Received pong")
		return nil
	})

	// Register peer info handler
	n.host.RegisterMessageHandler(protocol.MSG_TYPE_PEER_INFO, func(ctx context.Context, msg *protocol.Message, from peer.ID) error {
		var info protocol.PeerInfoPayload
		if err := json.Unmarshal(msg.Payload, &info); err != nil {
			return err
		}
		n.logger.WithField("peer", info.PeerID).Debug("Received peer info")
		return nil
	})

	// Register market task announce handler
	n.host.RegisterMessageHandler(protocol.MSG_TYPE_MARKET_TASK_ANNOUNCE, func(ctx context.Context, msg *protocol.Message, from peer.ID) error {
		if !n.market.IsEnabled() {
			return nil
		}

		var task protocol.MarketTaskAnnouncePayload
		if err := json.Unmarshal(msg.Payload, &task); err != nil {
			return err
		}

		n.logger.WithFields(logrus.Fields{
			"task_id": task.TaskID,
			"from":    from.String(),
		}).Debug("Received task announcement")

		// Evaluate and potentially bid
		bid, err := n.market.EvaluateAndBid(&task)
		if err != nil {
			n.logger.WithError(err).Debug("Did not bid on task")
			return nil
		}

		// Send bid
		bidMsg, _ := protocol.NewMessage(protocol.MSG_TYPE_MARKET_BID, n.host.ID(), bid)
		return n.host.SendMessage(ctx, from, bidMsg)
	})

	// Register market bid handler
	n.host.RegisterMessageHandler(protocol.MSG_TYPE_MARKET_BID, func(ctx context.Context, msg *protocol.Message, from peer.ID) error {
		if !n.market.IsEnabled() {
			return nil
		}

		var bid protocol.MarketBidPayload
		if err := json.Unmarshal(msg.Payload, &bid); err != nil {
			return err
		}

		n.logger.WithFields(logrus.Fields{
			"task_id": bid.TaskID,
			"bidder":  bid.BidderID,
			"price":   bid.BidPrice,
		}).Debug("Received bid")

		// Find auction and submit bid
		auctions := n.market.AuctionManager.GetAuctionsByTask(bid.TaskID)
		for _, auction := range auctions {
			n.market.AuctionManager.SubmitBid(auction.ID, &bid)
		}

		return nil
	})

	// Register memory sync handlers
	n.host.RegisterMessageHandler(protocol.MSG_TYPE_MEM_SYNC_REQ, func(ctx context.Context, msg *protocol.Message, from peer.ID) error {
		var req protocol.MemorySyncRequestPayload
		if err := json.Unmarshal(msg.Payload, &req); err != nil {
			return err
		}

		// Handle sync request
		syncReq := &memory.SyncRequest{
			RequesterID:  req.RequesterID,
			LastSyncTime: time.Unix(0, req.LastSyncTime),
			Keys:         req.Keys,
		}

		resp, err := n.memory.SyncRequest(syncReq)
		if err != nil {
			return err
		}

		// Send response
		respPayload := protocol.MemorySyncResponsePayload{
			ResponderID: n.host.ID().String(),
			SyncTime:    resp.SyncTime.UnixNano(),
		}

		for _, entry := range resp.Entries {
			respPayload.Entries = append(respPayload.Entries, protocol.MemoryEntry{
				Key:       entry.Key,
				Value:     entry.Value,
				Timestamp: entry.Timestamp.UnixNano(),
				TTL:       int64(entry.TTL),
				Version:   entry.Version,
				Signature: entry.Signature,
			})
		}

		respMsg, _ := protocol.NewMessage(protocol.MSG_TYPE_MEM_SYNC_RESP, n.host.ID(), respPayload)
		return n.host.SendMessage(ctx, from, respMsg)
	})
}

func (n *Node) registerTaskHandlers() {
	// Skip if market manager is not available
	if n.market == nil {
		n.logger.Warn("Market manager not available - skipping task handler registration")
		return
	}

	// Register OpenClaw prompt handler
	n.market.Scheduler.RegisterHandler(protocol.TaskTypeOpenClawPrompt, func(ctx context.Context, task *market.ScheduledTask) (*protocol.TaskResultPayload, error) {
		return n.executor.Execute(ctx, task)
	})

	// Register shell task handler
	n.market.Scheduler.RegisterHandler(protocol.TaskTypeShellTask, func(ctx context.Context, task *market.ScheduledTask) (*protocol.TaskResultPayload, error) {
		return n.executor.Execute(ctx, task)
	})

	// Register compute task handler
	n.market.Scheduler.RegisterHandler(protocol.TaskTypeCompute, func(ctx context.Context, task *market.ScheduledTask) (*protocol.TaskResultPayload, error) {
		return n.executor.Execute(ctx, task)
	})

	// Register storage task handler
	n.market.Scheduler.RegisterHandler(protocol.TaskTypeStorage, func(ctx context.Context, task *market.ScheduledTask) (*protocol.TaskResultPayload, error) {
		return n.executor.Execute(ctx, task)
	})
}

func (n *Node) shutdown() {
	n.logger.Info("Shutting down...")

	// Stop social
	if n.social != nil {
		n.social.Stop()
	}

	// Stop market
	if n.market != nil {
		n.market.Stop()
	}

	// Stop memory
	if n.memory != nil {
		n.memory.Close()
	}

	// Stop executor
	if n.executor != nil {
		n.executor.Close()
	}

	// Stop host
	if n.host != nil {
		n.host.Stop()
	}

	n.logger.Info("Shutdown complete")
}
