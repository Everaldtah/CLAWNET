# CLAWNET Security Model

## Overview

CLAWNET implements a defense-in-depth security model with multiple layers of protection for peer-to-peer communication, task execution, and economic transactions.

## Threat Model

### Threat Actors

| Actor | Capability | Motivation |
|-------|-----------|------------|
| Passive Observer | Network monitoring | Intelligence gathering |
| Active Attacker | Message injection | Disruption, fraud |
| Malicious Peer | Protocol participation | Free riding, sabotage |
| Colluding Group | Coordinated attack | Market manipulation |
| Compromised Node | Valid identity | Lateral movement |

### Assets

1. **Identity Keys** - Ed25519 private keys
2. **Wallet Funds** - Virtual credits
3. **Reputation Scores** - Trust metrics
4. **Task Data** - Sensitive computation inputs/outputs
5. **Memory Contents** - Distributed storage

## Security Layers

### Layer 1: Transport Security

**Noise Protocol**
- XX handshake pattern
- Perfect forward secrecy
- 0-RTT session resumption

**Implementation:**
```go
// libp2p with Noise
libp2p.Security(noise.ID, noise.New)
```

### Layer 2: Message Authentication

**Ed25519 Signatures**
- All messages signed
- Signature verification mandatory
- Replay protection via nonces

```go
// Sign message
hash := sha256(header + payload)
signature := ed25519.Sign(privateKey, hash)

// Verify
if !ed25519.Verify(publicKey, hash, signature) {
    return ErrInvalidSignature
}
```

### Layer 3: Identity Management

**Key Storage**
- Keys stored in encrypted BoltDB
- File permissions 0600
- Memory-only during runtime

**Key Rotation**
- Automatic rotation every 90 days
- Historical signatures remain valid
- Gradual trust transition

### Layer 4: Economic Security

**Escrow System**
```
1. Requester locks funds
2. Task assigned to winner
3. Execution with verification
4. Settlement or dispute
5. Funds released/refunded
```

**Anti-Fraud Measures**
- Rate limiting on bids
- Collusion detection
- Result verification
- Consensus requirements

### Layer 5: Access Control

**Capability-Based**
```yaml
node:
  capabilities:
    - "compute"
    - "ai-inference"
    - "shell-execution"
```

**Task Filtering**
- Reputation thresholds
- Capability matching
- Budget validation

## Attack Mitigations

### Sybil Attack

**Risk:** Single actor creates multiple identities

**Mitigations:**
- Reputation system with decay
- Economic costs for participation
- Bootstrap peer validation

### Eclipse Attack

**Risk:** Isolate node from honest peers

**Mitigations:**
- DHT with random peer selection
- Bootstrap peer diversity
- Connection monitoring

### Collusion Attack

**Risk:** Multiple peers coordinate to manipulate market

**Mitigations:**
- Price clustering detection
- Time-based bid analysis
- Reputation penalties

```go
// Detect similar pricing
func detectCollusion(bids []Bid) []string {
    clusters := clusterByPrice(bids)
    for _, cluster := range clusters {
        if len(cluster) > threshold {
            flagSuspicious(cluster)
        }
    }
}
```

### Free Riding

**Risk:** Peers consume resources without contributing

**Mitigations:**
- Reputation decay
- Minimum contribution requirements
- Rate limiting

### Data Leakage

**Risk:** Sensitive task data exposed

**Mitigations:**
- Optional payload encryption
- Memory encryption
- Secure deletion

```go
// Encrypt sensitive data
encrypted, err := aesGCMEncrypt(key, plaintext)
memory.Set("sensitive", encrypted, ttl)
```

## Cryptographic Primitives

### Key Exchange

| Algorithm | Use | Security Level |
|-----------|-----|----------------|
| X25519 | Key exchange | 128-bit |
| Ed25519 | Signatures | 128-bit |
| AES-256-GCM | Symmetric encryption | 256-bit |
| SHA-256 | Hashing | 256-bit |

### Protocol Constants

```go
const (
    // Key sizes
    PrivateKeySize = 64 // Ed25519
    PublicKeySize  = 32
    SignatureSize  = 64
    
    // Nonce size for AES-GCM
    NonceSize = 12
    
    // Max message size (10 MB)
    MaxMessageSize = 10 * 1024 * 1024
    
    // Message timeout
    MessageTimeout = 5 * time.Minute
)
```

## Secure Defaults

### Configuration

```yaml
network:
  # Encryption always enabled
  enable_encryption: true
  
  # Secure handshake only
  insecure_skip_verify: false

market:
  # Escrow required by default
  escrow_required: true
  
  # Fraud detection on
  fraud_detection: true
  
  # Anti-collusion checks
  anti_collusion_checks: true

memory:
  # Encryption enabled
  encryption_enabled: true
  
  # Compression before encryption
  compression_level: 6
```

## Audit Logging

### Security Events

```json
{
  "timestamp": "2024-01-01T00:00:00Z",
  "event": "security",
  "type": "invalid_signature",
  "peer": "12D3KooW...",
  "message_id": "uuid",
  "action": "dropped"
}
```

### Logged Events

- Invalid signatures
- Rate limit violations
- Authentication failures
- Escrow disputes
- Reputation penalties
- Configuration changes

## Incident Response

### Detection

```bash
# Monitor for suspicious activity
journalctl -u clawnet -f | grep "security"

# Check peer reputation
clawnet admin reputation <peer-id>

# View audit log
tail -f ~/.clawnet/security.log
```

### Response

1. **Isolate** - Disconnect from suspicious peers
2. **Analyze** - Review logs and evidence
3. **Report** - Document incident
4. **Recover** - Restore from backup if needed
5. **Improve** - Update defenses

```bash
# Ban malicious peer
clawnet admin ban <peer-id>

# Rotate keys
clawnet admin rotate-keys

# Reset reputation
clawnet admin reset-reputation <peer-id>
```

## Security Checklist

### Deployment

- [ ] Change default configuration
- [ ] Enable encryption
- [ ] Set strong firewall rules
- [ ] Configure log rotation
- [ ] Enable audit logging
- [ ] Set resource limits

### Operation

- [ ] Monitor peer connections
- [ ] Review reputation scores
- [ ] Check escrow balances
- [ ] Verify task results
- [ ] Update regularly
- [ ] Backup keys

## Vulnerability Disclosure

**Responsible Disclosure Process:**

1. Email security@clawnet.io
2. Include detailed description
3. Provide reproduction steps
4. Allow 90 days for fix
5. Coordinate public disclosure

**Bug Bounty:**
- Critical: $5000
- High: $2000
- Medium: $500
- Low: $100

## References

- [Noise Protocol Specification](https://noiseprotocol.org/)
- [Ed25519 Paper](https://ed25519.cr.yp.to/ed25519-20110926.pdf)
- [libp2p Security](https://docs.libp2p.io/concepts/security/)
- [OWASP Top 10](https://owasp.org/www-project-top-ten/)
