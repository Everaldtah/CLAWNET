package network

import (
	"context"
	"fmt"
	"time"

	"github.com/Everaldtah/CLAWNET/internal/config"
	"github.com/ipfs/go-datastore"
	dssync "github.com/ipfs/go-datastore/sync"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/protocol"
	dht "github.com/libp2p/go-libp2p-kad-dht"
	"github.com/libp2p/go-libp2p-kad-dht/providers"
	"github.com/sirupsen/logrus"
)

const (
	ClawnetDHTProtocol = protocol.ID("/clawnet/dht/1.0.0")
	DiscoveryNamespace = "clawnet"
)

// Discovery wraps the DHT for peer discovery
type Discovery struct {
	host   host.Host
	dht    *dht.IpfsDHT
	cfg    *config.Config
	logger *logrus.Entry
	ctx    context.Context
	cancel context.CancelFunc
}

// NewDiscovery creates a new DHT discovery service
func NewDiscovery(ctx context.Context, h host.Host, cfg *config.Config, logger *logrus.Logger) (*Discovery, error) {
	dctx, cancel := context.WithCancel(ctx)

	d := &Discovery{
		host:   h,
		cfg:    cfg,
		logger: logger.WithField("component", "discovery"),
		ctx:    dctx,
		cancel: cancel,
	}

	// Create DHT
	ds := dssync.MutexWrap(datastore.NewMapDatastore())

	var mode dht.ModeOpt
	switch cfg.Network.DHTMode {
	case "server":
		mode = dht.ModeServer
	case "client":
		mode = dht.ModeClient
	default:
		mode = dht.ModeAuto
	}

	kadDHT, err := dht.New(dctx, h,
		dht.Datastore(ds),
		dht.Mode(mode),
		dht.ProtocolPrefix(ClawnetDHTProtocol),
		dht.QueryFilter(dht.PublicQueryFilter),
		dht.RoutingTableFilter(dht.PublicRoutingTableFilter),
		dht.DisableProviders(),
		dht.DisableValues(),
	)
	if err != nil {
		cancel()
		return nil, fmt.Errorf("failed to create DHT: %w", err)
	}

	d.dht = kadDHT
	return d, nil
}

// Bootstrap bootstraps the DHT
func (d *Discovery) Bootstrap(ctx context.Context) error {
	if err := d.dht.Bootstrap(ctx); err != nil {
		return fmt.Errorf("DHT bootstrap failed: %w", err)
	}

	// Wait for DHT to be ready
	time.Sleep(2 * time.Second)

	d.logger.Info("DHT bootstrapped")
	return nil
}

// Close closes the discovery service
func (d *Discovery) Close() error {
	d.cancel()
	return d.dht.Close()
}

// FindPeers finds peers with the given capability
func (d *Discovery) FindPeers(ctx context.Context, capability string, limit int) ([]peer.AddrInfo, error) {
	// In a real implementation, this would use DHT records or pubsub
	// For now, return connected peers from routing table
	peers := d.dht.RoutingTable().ListPeers()
	
	var result []peer.AddrInfo
	for _, p := range peers {
		if len(result) >= limit {
			break
		}

		// Skip self
		if p == d.host.ID() {
			continue
		}

		// Get peer info from host
		addrs := d.host.Network().Peerstore().Addrs(p)
		if len(addrs) > 0 {
			result = append(result, peer.AddrInfo{
				ID:    p,
				Addrs: addrs,
			})
		}
	}

	return result, nil
}

// Advertise advertises a capability
func (d *Discovery) Advertise(ctx context.Context, capability string, ttl time.Duration) error {
	// In a real implementation, this would publish to DHT or use pubsub
	d.logger.WithField("capability", capability).Debug("Advertising capability")
	return nil
}

// GetRoutingTableSize returns the routing table size
func (d *Discovery) GetRoutingTableSize() int {
	return d.dht.RoutingTable().Size()
}

// GetConnectedPeers returns connected bootstrap peers
func (d *Discovery) GetConnectedPeers() []peer.ID {
	return d.dht.RoutingTable().ListPeers()
}
