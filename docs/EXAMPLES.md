# CLAWNET Example Workflows

## Table of Contents

1. [Basic Node Setup](#basic-node-setup)
2. [Task Market Workflow](#task-market-workflow)
3. [Swarm AI Execution](#swarm-ai-execution)
4. [Memory Synchronization](#memory-synchronization)
5. [Cross-Platform Deployment](#cross-platform-deployment)
6. [Advanced Market Strategies](#advanced-market-strategies)

---

## Basic Node Setup

### Single Node (Laptop)

```bash
# 1. Install and initialize
make install
clawnet init

# 2. Edit configuration
nano ~/.clawnet/config.yaml
```

```yaml
node:
  name: "laptop-node"
  capabilities:
    - "compute"
    - "ai-inference"
    - "shell-execution"

network:
  enable_mdns: true  # For LAN discovery
  enable_dht: true
```

```bash
# 3. Start node
clawnet

# 4. In TUI, check status
/peer list
```

### Two-Node Setup (Laptop + VPS)

**VPS Node:**
```yaml
node:
  name: "vps-node"
  listen_addrs:
    - "/ip4/0.0.0.0/udp/4001/quic-v1"
    - "/ip4/0.0.0.0/tcp/4002"
  capabilities:
    - "compute"
    - "storage"

network:
  enable_dht: true
  enable_mdns: false
```

```bash
# Start VPS node
clawnet --config vps-config.yaml

# Note the peer ID from logs
# [INFO] Host initialized peer_id=12D3KooW...
```

**Laptop Node:**
```yaml
node:
  name: "laptop-node"
  bootstrap_peers:
    - "/ip4/<vps-ip>/udp/4001/quic-v1/p2p/12D3KooW..."
```

```bash
# Start laptop node
clawnet --config laptop-config.yaml

# Should connect to VPS automatically
```

---

## Task Market Workflow

### Example 1: Simple AI Task

**Step 1: Submit Task (Requester)**

```bash
# In TUI
/market submit "Summarize the following text: [your text here]"
```

**What happens:**
1. Task broadcast to network
2. Peers evaluate capabilities
3. Bids submitted within 5 seconds
4. Winner selected by score
5. Escrow locked (10 credits)
6. Task executed by winner
7. Result returned and verified
8. Escrow released to winner

**Step 2: Monitor Auction**

```bash
/market status
```

Output:
```
Active Auctions: 1
Wallet: 990.00 credits
Reputation: 0.85

Auction: a1b2c3d4
  Task: Summarize text
  Budget: 10.00
  Bids: 3
  Status: selecting winner
```

**Step 3: View Result**

```
Task e5f6g7h8 completed!
Result: [AI-generated summary]
Cost: 7.50 credits
Executor: 12D3KooW... (reputation: 0.92)
```

### Example 2: Shell Task with Budget

```bash
/market submit --type shell_task --budget 5.0 "find /var/log -name '*.log' | head -20"
```

**Bid Evaluation:**
- Peer A: Bid 4.0, ETA 10s, Load 20%
- Peer B: Bid 5.0, ETA 5s, Load 60%
- Peer C: Bid 3.5, ETA 15s, Load 10%

**Winner Selection:**
```
Score(Peer A) = 0.4*(1/4.0) + 0.3*0.85 + 0.1*(1/10) + 0.2*0.9 = 0.4975
Score(Peer B) = 0.4*(1/5.0) + 0.3*0.90 + 0.1*(1/5) + 0.2*0.8 = 0.4700
Score(Peer C) = 0.4*(1/3.5) + 0.3*0.75 + 0.1*(1/15) + 0.2*0.85 = 0.5076

Winner: Peer C (highest score)
```

### Example 3: High-Priority Task

```bash
/market submit --budget 50.0 --deadline 30s --min-reputation 0.8 \
  "Analyze system logs for security anomalies"
```

---

## Swarm AI Execution

### Example: Distributed Document Analysis

**Task:** Analyze 1000 documents across multiple nodes

```bash
# Submit swarm task
/market submit --mode swarm-ai --budget 100.0 \
  "Analyze sentiment of attached documents"
```

**Swarm Formation:**
```
Swarm ID: swarm-abc123
Coordinator: laptop-node
Members:
  - laptop-node (local)
  - vps-node-1 (cloud)
  - vps-node-2 (cloud)
  - android-node (mobile)
```

**Task Distribution:**
```
Total documents: 1000
Documents per node: 250

laptop-node:   docs 0-249   (local processing)
vps-node-1:    docs 250-499 (cloud compute)
vps-node-2:    docs 500-749 (cloud compute)
android-node:  docs 750-999 (low-power, simple analysis)
```

**Aggregation:**
```python
# Coordinator aggregates results
results = collect_from_all_nodes()
final_result = aggregate_sentiment(results)
```

**Payment Distribution:**
```
Total budget: 100.0 credits
Based on work contribution:
  laptop-node:  25 credits (25% of docs)
  vps-node-1:   25 credits
  vps-node-2:   25 credits
  android-node: 25 credits
```

---

## Memory Synchronization

### Example 1: Shared Configuration

**Node A (Update):**
```go
memory.Set("config/theme", "dark", 24*time.Hour)
memory.Set("config/refresh_rate", 10, 24*time.Hour)
```

**Node B (Sync):**
```go
// Automatic sync every 30s
entry, _ := memory.Get("config/theme")
fmt.Println(entry.Value) // "dark"
```

### Example 2: Distributed Cache

```go
// Store computed result
result := expensiveComputation()
memory.Set("cache/result-hash", result, 1*time.Hour)

// Other nodes can retrieve
if cached, err := memory.Get("cache/result-hash"); err == nil {
    return cached.Value
}
```

### Example 3: Encrypted Secrets

```yaml
memory:
  encryption_enabled: true
```

```go
// Store encrypted
memory.Set("secrets/api-key", "sk-...", 0) // 0 = no expiry

// Automatically decrypted on retrieval
key, _ := memory.Get("secrets/api-key")
```

---

## Cross-Platform Deployment

### Scenario: Heterogeneous Network

```
Network Topology:

                    [Internet]
                        |
        +---------------+---------------+
        |                               |
   [VPS Node]                    [Home Server]
   (Cloud)                        (Linux)
        |                               |
        +---------------+---------------+
                        |
                [Router/Switch]
                        |
        +---------------+---------------+
        |                               |
   [Laptop]                      [Android Phone]
   (macOS)                        (Termux)
        |
   [Docker Container]
```

**VPS Configuration (Public):**
```yaml
node:
  name: "vps-bootstrap"
  listen_addrs:
    - "/ip4/0.0.0.0/udp/4001/quic-v1"
    - "/ip4/0.0.0.0/tcp/4002"
  capabilities:
    - "compute"
    - "storage"
    - "ai-inference"

network:
  enable_dht: true
  enable_mdns: false
```

**Home Server:**
```yaml
node:
  name: "home-server"
  bootstrap_peers:
    - "/ip4/<vps-ip>/udp/4001/quic-v1/p2p/<vps-peer-id>"
```

**Laptop:**
```yaml
node:
  name: "macbook"
  bootstrap_peers:
    - "/ip4/<vps-ip>/udp/4001/quic-v1/p2p/<vps-peer-id>"
  capabilities:
    - "ai-inference"
```

**Android (Termux):**
```yaml
node:
  name: "pixel-phone"
  bootstrap_peers:
    - "/ip4/<vps-ip>/udp/4001/quic-v1/p2p/<vps-peer-id>"
  capabilities:
    - "compute"  # Limited
  
network:
  enable_dht: false  # Save battery
  enable_mdns: true  # LAN only
```

**Docker:**
```yaml
services:
  clawnet:
    image: clawnet:latest
    network_mode: host
    environment:
      - CLAWNET_NODE_NAME=docker-node
```

---

## Advanced Market Strategies

### Strategy 1: Competitive Pricing

```go
// As a worker node, bid aggressively when load is low
func bidStrategy(task *Task) float64 {
    load := getSystemLoad()
    basePrice := task.MaxBudget * 0.6
    
    if load < 0.3 {
        // Low load = competitive price
        return basePrice * 0.8
    } else if load > 0.8 {
        // High load = premium price
        return basePrice * 1.3
    }
    
    return basePrice
}
```

### Strategy 2: Reputation Building

```yaml
# New node strategy
market:
  # Accept lower margins initially
  min_bid_margin: 0.1
  
  # Focus on quick wins
  preferred_task_types:
    - "shell_task"
    - "compute"
  
  # Always deliver on time
  max_concurrent_tasks: 2
```

### Strategy 3: Specialized Worker

```yaml
node:
  name: "ai-worker"
  capabilities:
    - "ai-inference"
  
  # Only bid on AI tasks
  task_filters:
    type: "openclaw_prompt"
    min_budget: 5.0

openclaw:
  enabled: true
  # Premium AI service
  max_tokens: 8192
  model: "gpt-4"
```

### Strategy 4: Arbitrage

```go
// Monitor market for underpriced tasks
func findArbitrage() {
    for _, auction := range market.GetActiveAuctions() {
        // Tasks with no bids and approaching deadline
        if len(auction.Bids) == 0 && 
           time.Until(auction.Deadline) < 10*time.Second {
            // Bid minimum price
            market.SubmitBid(auction.ID, auction.MaxBudget * 0.1)
        }
    }
}
```

---

## Troubleshooting Examples

### Debug Connection Issues

```bash
# Check peer connectivity
/peer list

# Detailed peer info
/peer info <peer-id>

# Network diagnostics
/network status
```

### Market Debug

```bash
# View auction details
/market auction <auction-id>

# View bid history
/market bids <task-id>

# Check escrow status
/market escrow <escrow-id>
```

### Performance Tuning

```yaml
# For high-throughput nodes
node:
  max_peers: 500

network:
  connection_timeout: "10s"
  ping_interval: "5s"

market:
  max_concurrent_auctions: 50
  bid_timeout: "3s"
```

---

## Integration Examples

### CI/CD Pipeline

```yaml
# .github/workflows/clawnet.yml
name: CLAWNET Task
on: [push]

jobs:
  analyze:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v2
      
      - name: Submit to CLAWNET
        run: |
          echo '{"type":"shell_task","data":{"command":"make test"}}' | \
          clawnet submit --wait
```

### Webhook Integration

```python
from flask import Flask, request
import requests

app = Flask(__name__)

@app.route('/webhook', methods=['POST'])
def webhook():
    data = request.json
    
    # Submit to CLAWNET
    task = {
        'type': 'openclaw_prompt',
        'description': f'Process: {data}',
        'max_budget': 10.0
    }
    
    requests.post('http://localhost:8080/api/tasks', json=task)
    return {'status': 'submitted'}
```

---

## More Examples

See the `examples/` directory for:
- Python client library
- JavaScript/Node.js integration
- Kubernetes deployment
- Monitoring dashboards
- Custom task types
