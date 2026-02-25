# CLAWNET Project Summary

## Overview

CLAWNET is a production-ready distributed AI mesh network built in Go, featuring:

- **Secure P2P Networking**: libp2p with QUIC/TCP, Noise encryption
- **Autonomous Task Market**: Auction-based delegation with escrow
- **Reputation System**: Decay-based scoring with fraud detection
- **Memory Synchronization**: Distributed encrypted storage
- **Cross-Platform**: Linux, macOS, Windows, Android (Termux)

## Project Structure

```
clawnet/
├── cmd/clawnet/           # Main entry point
│   └── main.go           # CLI and node orchestration
├── internal/             # Internal packages
│   ├── config/          # Configuration management
│   ├── identity/        # Ed25519 identity and crypto
│   ├── network/         # libp2p host and discovery
│   ├── protocol/        # Message schemas and protocols
│   ├── market/          # Market system
│   │   ├── auction.go   # Auction management
│   │   ├── bid.go       # Bid evaluation
│   │   ├── escrow.go    # Escrow system
│   │   ├── market.go    # Market manager
│   │   ├── pricing.go   # Wallet and pricing
│   │   ├── reputation.go # Reputation system
│   │   ├── scheduler.go # Task scheduling
│   │   └── settlement.go # Settlement system
│   ├── memory/          # Memory synchronization
│   ├── task/            # Task execution
│   ├── openclaw/        # OpenClaw AI client
│   └── tui/             # Bubble Tea terminal UI
├── configs/             # Configuration examples
├── deployments/         # Deployment files
├── scripts/             # Helper scripts
├── docs/                # Documentation
├── Dockerfile           # Docker image
├── docker-compose.yml   # Docker Compose setup
├── Makefile            # Build automation
├── go.mod              # Go module definition
└── README.md           # Project readme
```

## Core Components

### 1. Identity (internal/identity)

- Ed25519 key generation and management
- libp2p key conversion
- Message signing and verification
- Secure key storage with BoltDB

### 2. Network (internal/network)

- libp2p host with QUIC/TCP transports
- Noise protocol encryption
- mDNS LAN discovery
- Kademlia DHT WAN discovery
- Peer connection management

### 3. Protocol (internal/protocol)

- Message type definitions
- JSON serialization
- Signature verification
- Replay protection

### 4. Market (internal/market)

**Auction System:**
- Task announcement broadcasting
- Bid collection and scoring
- Winner selection algorithm
- Consensus mode support

**Bid Management:**
- Capability matching
- Price calculation
- Rate limiting
- Anti-collusion detection

**Escrow:**
- Fund locking
- Timeout handling
- Dispute resolution
- Refund processing

**Reputation:**
- Score calculation
- Decay mechanism
- Feedback processing
- Persistent storage

**Scheduler:**
- Task queue management
- Execution timeout
- Retry logic
- Status tracking

**Settlement:**
- Payment processing
- Consensus voting
- Fraud detection
- Dispute handling

### 5. Memory (internal/memory)

- Key-value storage
- Compression (gzip)
- Encryption (AES-GCM)
- Synchronization protocol
- Garbage collection

### 6. Task Execution (internal/task)

- OpenClaw AI prompts
- Shell command execution
- Compute tasks
- Storage operations
- Handler registration

### 7. TUI (internal/tui)

- Bubble Tea framework
- Multi-panel interface
- Real-time updates
- Command mode
- Keyboard shortcuts

## Key Features

### Security

- Ed25519 signatures on all messages
- Noise protocol transport encryption
- Optional payload encryption
- Replay attack protection
- Rate limiting
- Fraud detection

### Market Mechanism

```
Task Flow:
1. User submits task with budget
2. Task broadcast to network
3. Peers evaluate and bid
4. Winner selected by score
5. Escrow locked
6. Task executed
7. Result verified
8. Settlement completed
```

Scoring weights:
- Price: 40%
- Reputation: 30%
- Latency: 10%
- Confidence: 20%

### Reputation System

Components:
- Task completion rate (30%)
- On-time completion (20%)
- Response accuracy (20%)
- Dispute ratio (20%)
- Swarm contribution (10%)

Decay: 1% per day of inactivity

## Configuration

Key options in `config.yaml`:

```yaml
node:
  name: "node-name"
  capabilities: ["compute", "ai-inference"]
  max_peers: 100

network:
  enable_quic: true
  enable_dht: true
  enable_mdns: true

market:
  enabled: true
  initial_wallet_balance: 1000.0
  bid_timeout: "5s"
  task_timeout: "60s"
  escrow_required: true

memory:
  encryption_enabled: true
  compression_level: 6
```

## Build & Deploy

### Build

```bash
# Current platform
make build

# All platforms
make build-all

# Docker
make docker
```

### Install

```bash
# System install
sudo make install

# Quick install
curl -fsSL https://get.clawnet.io | sudo bash
```

### Deploy

```bash
# Docker Compose
docker-compose up -d

# Systemd
sudo systemctl enable --now clawnet

# Kubernetes
kubectl apply -f deployments/k8s/
```

## Usage Examples

### Start Node

```bash
clawnet --config ~/.clawnet/config.yaml
```

### Submit Task

```bash
# In TUI
/market submit "Summarize this text"
```

### Check Status

```bash
/market status
/peer list
/memory sync
```

## API Integration

### Go

```go
import "github.com/clawnet/clawnet/internal/market"

task := &protocol.MarketTaskAnnouncePayload{
    TaskID:    uuid.New().String(),
    Type:      protocol.TaskTypeOpenClawPrompt,
    MaxBudget: 10.0,
}

scheduled, err := market.SubmitTask(task)
```

### HTTP (Future)

```bash
curl -X POST http://localhost:8080/api/tasks \
  -H "Content-Type: application/json" \
  -d '{"type":"openclaw_prompt","budget":10.0}'
```

## Testing

```bash
# Unit tests
make test

# Coverage
make test-coverage

# Benchmarks
make benchmark

# Security scan
make security-scan
```

## Monitoring

### Metrics

- `clawnet_peers_connected`
- `clawnet_auctions_active`
- `clawnet_tasks_completed`
- `clawnet_wallet_balance`
- `clawnet_reputation_score`

### Logs

```bash
# Systemd
journalctl -u clawnet -f

# Docker
docker-compose logs -f clawnet

# File
tail -f ~/.clawnet/logs/clawnet.log
```

## Roadmap

### Phase 1 (Current)
- [x] Core P2P networking
- [x] Market mechanism
- [x] Task execution
- [x] Memory sync
- [x] TUI interface

### Phase 2 (Planned)
- [ ] Web dashboard
- [ ] Mobile app
- [ ] Plugin system
- [ ] Advanced analytics
- [ ] Cross-chain integration

### Phase 3 (Future)
- [ ] Federated learning
- [ ] Zero-knowledge proofs
- [ ] DAO governance
- [ ] Token economics

## Contributing

1. Fork repository
2. Create feature branch
3. Run tests: `make test`
4. Submit PR

## License

MIT License - See LICENSE file

## Contact

- GitHub: https://github.com/clawnet/clawnet
- Discord: https://discord.gg/clawnet
- Email: dev@clawnet.io

## Acknowledgments

- libp2p team
- OpenClaw project
- Bubble Tea framework
- Go community
