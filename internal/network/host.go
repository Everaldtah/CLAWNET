package network

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/Everaldtah/CLAWNET/internal/config"
	"github.com/Everaldtah/CLAWNET/internal/identity"
	"github.com/Everaldtah/CLAWNET/internal/protocol"
	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	libp2pprotocol "github.com/libp2p/go-libp2p/core/protocol"
	"github.com/libp2p/go-libp2p/p2p/discovery/mdns"
	"github.com/libp2p/go-libp2p/p2p/host/autorelay"
	"github.com/libp2p/go-libp2p/p2p/net/connmgr"
	"github.com/libp2p/go-libp2p/p2p/security/noise"
	quictransport "github.com/libp2p/go-libp2p/p2p/transport/quic"
	"github.com/libp2p/go-libp2p/p2p/transport/tcp"
	"github.com/multiformats/go-multiaddr"
	"github.com/sirupsen/logrus"
)

const (
	ClawnetProtocolID = "/clawnet/1.0.0"
	MaxMessageSize    = 10 * 1024 * 1024 // 10MB
)

// Host wraps libp2p host with CLAWNET functionality
type Host struct {
	mu sync.RWMutex

	host       host.Host
	cfg        *config.Config
	identity   *identity.Identity
	logger     *logrus.Logger

	// Protocol handlers
	handlers   map[libp2pprotocol.ID]network.StreamHandler
	messageHandlers map[protocol.MessageType]MessageHandler

	// Peer management
	peers      map[peer.ID]*PeerInfo
	peerChan   chan peer.ID

	// Discovery
	mdnsService mdns.Service
	dhtDiscovery *Discovery

	// Context
	ctx    context.Context
	cancel context.CancelFunc
}

// PeerInfo contains information about a peer
type PeerInfo struct {
	ID            peer.ID
	Addrs         []multiaddr.Multiaddr
	Capabilities  []string
	Version       string
	Platform      string
	ConnectedAt   time.Time
	LastSeen      time.Time
	Reputation    float64
	Latency       time.Duration
	Load          float64
	mu            sync.RWMutex
}

// MessageHandler is a function that handles messages
type MessageHandler func(ctx context.Context, msg *protocol.Message, from peer.ID) error

// NewHost creates a new CLAWNET host
func NewHost(cfg *config.Config, id *identity.Identity, logger *logrus.Logger) (*Host, error) {
	ctx, cancel := context.WithCancel(context.Background())

	h := &Host{
		cfg:             cfg,
		identity:        id,
		logger:          logger,
		handlers:        make(map[libp2pprotocol.ID]network.StreamHandler),
		messageHandlers: make(map[protocol.MessageType]MessageHandler),
		peers:           make(map[peer.ID]*PeerInfo),
		peerChan:        make(chan peer.ID, 100),
		ctx:             ctx,
		cancel:          cancel,
	}

	// Build libp2p options
	opts := h.buildHostOptions()

	// Create libp2p host
	libp2pHost, err := libp2p.New(opts...)
	if err != nil {
		cancel()
		return nil, fmt.Errorf("failed to create libp2p host: %w", err)
	}

	h.host = libp2pHost

	// Set up stream handler
	h.host.SetStreamHandler(libp2pprotocol.ID(ClawnetProtocolID), h.handleStream)

	// Set up connection notifications
	h.host.Network().Notify(&network.NotifyBundle{
		ConnectedF:    h.onPeerConnected,
		DisconnectedF: h.onPeerDisconnected,
	})

	logger.WithFields(logrus.Fields{
		"peer_id": h.host.ID().String(),
		"addrs":   h.host.Addrs(),
	}).Info("Host initialized")

	return h, nil
}

// buildHostOptions builds libp2p host options
func (h *Host) buildHostOptions() []libp2p.Option {
	opts := []libp2p.Option{
		libp2p.Identity(h.identity.GetLibp2pPrivKey()),
		libp2p.ListenAddrStrings(h.cfg.Node.ListenAddrs...),
		libp2p.Security(noise.ID, noise.New),
		libp2p.DefaultTransports,
		libp2p.NATPortMap(),
	}

	// Connection manager
	cm, err := connmgr.NewConnManager(
		h.cfg.Node.MinPeers,
		h.cfg.Node.MaxPeers,
		connmgr.WithGracePeriod(time.Minute),
	)
	if err != nil {
		cancel()
		return nil, fmt.Errorf("failed to create connection manager: %w", err)
	}
	opts = append(opts, libp2p.ConnectionManager(cm))

	// Enable relay if configured
	if h.cfg.Network.EnableRelay {
		opts = append(opts, libp2p.EnableRelay())
		opts = append(opts, libp2p.EnableNATService())
		opts = append(opts, libp2p.EnableAutoRelayWithStaticRelays(
			[]peer.AddrInfo{},
			autorelay.WithNumRelays(2),
		))
	}

	// Enable specific transports
	if h.cfg.Network.EnableQUIC {
		opts = append(opts, libp2p.Transport(quictransport.NewTransport))
	}
	if h.cfg.Network.EnableTCP {
		opts = append(opts, libp2p.Transport(tcp.NewTCPTransport))
	}

	return opts
}

// Start starts the host and discovery services
func (h *Host) Start() error {
	h.mu.Lock()
	defer h.mu.Unlock()

	// Start mDNS discovery
	if h.cfg.Network.EnableMDNS {
		mdnsService := mdns.NewMdnsService(h.host, "clawnet", h)
		if err := mdnsService.Start(); err != nil {
			return fmt.Errorf("failed to start mDNS: %w", err)
		}
		h.mdnsService = mdnsService
		h.logger.Info("mDNS discovery started")
	}

	// Start DHT discovery
	if h.cfg.Network.EnableDHT {
		dht, err := NewDiscovery(h.ctx, h.host, h.cfg, h.logger)
		if err != nil {
			return fmt.Errorf("failed to create DHT discovery: %w", err)
		}
		h.dhtDiscovery = dht
		if err := dht.Bootstrap(h.ctx); err != nil {
			h.logger.WithError(err).Warn("DHT bootstrap failed")
		}
		h.logger.Info("DHT discovery started")
	}

	// Connect to bootstrap peers
	go h.connectToBootstrapPeers()

	// Start peer maintenance
	go h.peerMaintenance()

	h.logger.Info("Host started")
	return nil
}

// Stop stops the host
func (h *Host) Stop() error {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.cancel()

	if h.mdnsService != nil {
		h.mdnsService.Close()
	}

	if h.dhtDiscovery != nil {
		h.dhtDiscovery.Close()
	}

	return h.host.Close()
}

// ID returns the host's peer ID
func (h *Host) ID() peer.ID {
	return h.host.ID()
}

// Addrs returns the host's addresses
func (h *Host) Addrs() []multiaddr.Multiaddr {
	return h.host.Addrs()
}

// Connect connects to a peer
func (h *Host) Connect(ctx context.Context, pi peer.AddrInfo) error {
	if err := h.host.Connect(ctx, pi); err != nil {
		return err
	}

	h.mu.Lock()
	h.peers[pi.ID] = &PeerInfo{
		ID:          pi.ID,
		Addrs:       pi.Addrs,
		ConnectedAt: time.Now(),
		LastSeen:    time.Now(),
	}
	h.mu.Unlock()

	return nil
}

// Disconnect disconnects from a peer
func (h *Host) Disconnect(p peer.ID) error {
	return h.host.Network().ClosePeer(p)
}

// SendMessage sends a message to a peer
func (h *Host) SendMessage(ctx context.Context, p peer.ID, msg *protocol.Message) error {
	stream, err := h.host.NewStream(ctx, p, libp2pprotocol.ID(ClawnetProtocolID))
	if err != nil {
		return fmt.Errorf("failed to open stream: %w", err)
	}
	defer stream.Close()

	data, err := msg.Encode()
	if err != nil {
		return fmt.Errorf("failed to encode message: %w", err)
	}

	if len(data) > MaxMessageSize {
		return fmt.Errorf("message too large: %d bytes", len(data))
	}

	// Write length prefix
	lengthBuf := make([]byte, 4)
	lengthBuf[0] = byte(len(data) >> 24)
	lengthBuf[1] = byte(len(data) >> 16)
	lengthBuf[2] = byte(len(data) >> 8)
	lengthBuf[3] = byte(len(data))

	if _, err := stream.Write(lengthBuf); err != nil {
		return fmt.Errorf("failed to write length: %w", err)
	}

	if _, err := stream.Write(data); err != nil {
		return fmt.Errorf("failed to write message: %w", err)
	}

	return nil
}

// Broadcast broadcasts a message to all connected peers
func (h *Host) Broadcast(ctx context.Context, msg *protocol.Message) error {
	h.mu.RLock()
	peers := make([]peer.ID, 0, len(h.peers))
	for p := range h.peers {
		peers = append(peers, p)
	}
	h.mu.RUnlock()

	var lastErr error
	for _, p := range peers {
		if err := h.SendMessage(ctx, p, msg); err != nil {
			h.logger.WithError(err).WithField("peer", p.String()).Warn("Failed to send message")
			lastErr = err
		}
	}

	return lastErr
}

// RegisterMessageHandler registers a handler for a message type
func (h *Host) RegisterMessageHandler(msgType protocol.MessageType, handler MessageHandler) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.messageHandlers[msgType] = handler
}

// handleStream handles incoming streams
func (h *Host) handleStream(stream network.Stream) {
	defer stream.Close()

	peerID := stream.Conn().RemotePeer()

	// Read length prefix
	lengthBuf := make([]byte, 4)
	if _, err := stream.Read(lengthBuf); err != nil {
		h.logger.WithError(err).Warn("Failed to read message length")
		return
	}

	length := int(lengthBuf[0])<<24 | int(lengthBuf[1])<<16 | int(lengthBuf[2])<<8 | int(lengthBuf[3])
	if length > MaxMessageSize {
		h.logger.WithField("size", length).Warn("Message too large")
		return
	}

	// Read message data
	data := make([]byte, length)
	if _, err := stream.Read(data); err != nil {
		h.logger.WithError(err).Warn("Failed to read message")
		return
	}

	// Decode message
	msg, err := protocol.Decode(data)
	if err != nil {
		h.logger.WithError(err).Warn("Failed to decode message")
		return
	}

	// Update peer info
	h.updatePeerLastSeen(peerID)

	// Handle message
	if handler, ok := h.messageHandlers[msg.Header.Type]; ok {
		if err := handler(h.ctx, msg, peerID); err != nil {
			h.logger.WithError(err).WithField("type", msg.Header.Type).Warn("Message handler failed")
		}
	} else {
		h.logger.WithField("type", msg.Header.Type).Debug("No handler for message type")
	}
}

// HandlePeerFound implements mdns.Notifee
func (h *Host) HandlePeerFound(pi peer.AddrInfo) {
	h.logger.WithField("peer", pi.ID.String()).Debug("Peer discovered via mDNS")

	if pi.ID == h.host.ID() {
		return
	}

	ctx, cancel := context.WithTimeout(h.ctx, h.cfg.Network.ConnectionTimeout)
	defer cancel()

	if err := h.Connect(ctx, pi); err != nil {
		h.logger.WithError(err).WithField("peer", pi.ID.String()).Debug("Failed to connect to discovered peer")
	} else {
		h.logger.WithField("peer", pi.ID.String()).Info("Connected to discovered peer")
	}
}

// onPeerConnected handles peer connection
func (h *Host) onPeerConnected(n network.Network, c network.Conn) {
	peerID := c.RemotePeer()
	h.logger.WithField("peer", peerID.String()).Debug("Peer connected")

	h.mu.Lock()
	if _, exists := h.peers[peerID]; !exists {
		h.peers[peerID] = &PeerInfo{
			ID:          peerID,
			Addrs:       []multiaddr.Multiaddr{c.RemoteMultiaddr()},
			ConnectedAt: time.Now(),
			LastSeen:    time.Now(),
		}
	}
	h.mu.Unlock()

	// Send peer info
	go h.sendPeerInfo(peerID)

	select {
	case h.peerChan <- peerID:
	default:
	}
}

// onPeerDisconnected handles peer disconnection
func (h *Host) onPeerDisconnected(n network.Network, c network.Conn) {
	peerID := c.RemotePeer()
	h.logger.WithField("peer", peerID.String()).Debug("Peer disconnected")

	h.mu.Lock()
	delete(h.peers, peerID)
	h.mu.Unlock()
}

// sendPeerInfo sends peer information to a newly connected peer
func (h *Host) sendPeerInfo(p peer.ID) {
	payload := protocol.PeerInfoPayload{
		PeerID:        h.host.ID().String(),
		Capabilities:  h.cfg.Node.Capabilities,
		Version:       "1.0.0",
		Platform:      getPlatform(),
		Uptime:        0,
		Load:          getSystemLoad(),
		Reputation:    0.5, // Default reputation
		WalletBalance: 0,
	}

	msg, err := protocol.NewMessage(protocol.MSG_TYPE_PEER_INFO, h.host.ID(), payload)
	if err != nil {
		h.logger.WithError(err).Warn("Failed to create peer info message")
		return
	}

	ctx, cancel := context.WithTimeout(h.ctx, 10*time.Second)
	defer cancel()

	if err := h.SendMessage(ctx, p, msg); err != nil {
		h.logger.WithError(err).Debug("Failed to send peer info")
	}
}

// updatePeerLastSeen updates the last seen time for a peer
func (h *Host) updatePeerLastSeen(p peer.ID) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if peer, ok := h.peers[p]; ok {
		peer.mu.Lock()
		peer.LastSeen = time.Now()
		peer.mu.Unlock()
	}
}

// connectToBootstrapPeers connects to configured bootstrap peers
func (h *Host) connectToBootstrapPeers() {
	for _, addr := range h.cfg.Node.BootstrapPeers {
		ma, err := multiaddr.NewMultiaddr(addr)
		if err != nil {
			h.logger.WithError(err).WithField("addr", addr).Warn("Invalid bootstrap address")
			continue
		}

		pi, err := peer.AddrInfoFromP2pAddr(ma)
		if err != nil {
			h.logger.WithError(err).WithField("addr", addr).Warn("Failed to parse bootstrap address")
			continue
		}

		ctx, cancel := context.WithTimeout(h.ctx, h.cfg.Network.ConnectionTimeout)
		err = h.Connect(ctx, *pi)
		cancel()

		if err != nil {
			h.logger.WithError(err).WithField("peer", pi.ID.String()).Warn("Failed to connect to bootstrap peer")
		} else {
			h.logger.WithField("peer", pi.ID.String()).Info("Connected to bootstrap peer")
		}
	}
}

// peerMaintenance maintains peer connections
func (h *Host) peerMaintenance() {
	ticker := time.NewTicker(h.cfg.Network.PingInterval)
	defer ticker.Stop()

	for {
		select {
		case <-h.ctx.Done():
			return
		case <-ticker.C:
			h.pingPeers()
		}
	}
}

// pingPeers sends ping messages to all peers
func (h *Host) pingPeers() {
	h.mu.RLock()
	peers := make([]peer.ID, 0, len(h.peers))
	for p := range h.peers {
		peers = append(peers, p)
	}
	h.mu.RUnlock()

	for _, p := range peers {
		msg, _ := protocol.NewMessage(protocol.MSG_TYPE_PING, h.host.ID(), struct{}{})
		
		ctx, cancel := context.WithTimeout(h.ctx, 5*time.Second)
		err := h.SendMessage(ctx, p, msg)
		cancel()

		if err != nil {
			h.logger.WithError(err).WithField("peer", p.String()).Debug("Ping failed")
		}
	}
}

// GetPeers returns connected peers
func (h *Host) GetPeers() []*PeerInfo {
	h.mu.RLock()
	defer h.mu.RUnlock()

	peers := make([]*PeerInfo, 0, len(h.peers))
	for _, p := range h.peers {
		peers = append(peers, p)
	}
	return peers
}

// GetPeerCount returns the number of connected peers
func (h *Host) GetPeerCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.peers)
}

// Host returns the underlying libp2p host
func (h *Host) Host() host.Host {
	return h.host
}

// Discovery returns the DHT discovery service
func (h *Host) Discovery() *Discovery {
	return h.dhtDiscovery
}

// Helper functions
func getPlatform() string {
	return "unknown"
}

func getSystemLoad() float64 {
	return 0.0
}
