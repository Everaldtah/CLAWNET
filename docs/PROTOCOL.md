# CLAWNET Protocol Specification

## Overview

CLAWNET uses a custom protocol built on libp2p for peer-to-peer communication. All messages are encoded in JSON and signed using Ed25519 signatures.

## Transport

- **Primary**: QUIC (UDP-based, multiplexed streams)
- **Secondary**: TCP (fallback compatibility)
- **Security**: Noise protocol for encryption
- **Identity**: Ed25519 key pairs

## Message Format

All messages follow a standard envelope format:

```json
{
  "header": {
    "id": "uuid-v4",
    "type": 0,
    "timestamp": 1700000000000000000,
    "sender": "12D3KooW...",
    "nonce": "random-string",
    "signature": "base64-signature"
  },
  "payload": { ... }
}
```

### Header Fields

| Field | Type | Description |
|-------|------|-------------|
| id | string | UUID v4 message identifier |
| type | uint8 | Message type identifier |
| timestamp | int64 | Nanoseconds since Unix epoch |
| sender | string | Peer ID of sender |
| nonce | string | Random nonce for replay protection |
| signature | string | Ed25519 signature of message hash |

### Signature Algorithm

1. Serialize header fields (excluding signature) + payload
2. Compute SHA-256 hash
3. Sign with sender's Ed25519 private key
4. Encode signature as base64

## Message Types

### Core Protocol (0-9)

| Type | Name | Description |
|------|------|-------------|
| 0 | PING | Keepalive probe |
| 1 | PONG | Keepalive response |
| 2 | PEER_INFO | Peer capability announcement |
| 3 | DISCONNECT | Graceful disconnect notification |

### Task Delegation (10-19)

| Type | Name | Description |
|------|------|-------------|
| 10 | TASK_SUBMIT | Submit task for execution |
| 11 | TASK_ACCEPT | Accept task for execution |
| 12 | TASK_REJECT | Reject task submission |
| 13 | TASK_RESULT | Return task result |
| 14 | TASK_CANCEL | Cancel pending task |

### Memory Sync (20-29)

| Type | Name | Description |
|------|------|-------------|
| 20 | MEM_SYNC_REQ | Request memory synchronization |
| 21 | MEM_SYNC_RESP | Memory sync response |
| 22 | MEM_UPDATE | Memory entry update |
| 23 | MEM_DELETE | Memory entry deletion |

### Market (30-39)

| Type | Name | Description |
|------|------|-------------|
| 30 | MARKET_TASK_ANNOUNCE | Announce task to market |
| 31 | MARKET_BID | Submit bid for task |
| 32 | MARKET_WINNER | Announce auction winner |
| 33 | MARKET_ESCROW_LOCK | Lock escrow funds |
| 34 | MARKET_SETTLEMENT | Settle completed task |
| 35 | MARKET_DISPUTE | Raise dispute |
| 36 | MARKET_REPUTATION_UPDATE | Reputation score update |

### Swarm (40-49)

| Type | Name | Description |
|------|------|-------------|
| 40 | SWARM_FORM | Form new swarm |
| 41 | SWARM_JOIN | Join existing swarm |
| 42 | SWARM_LEAVE | Leave swarm |
| 43 | SWARM_COORDINATE | Coordinate swarm task |
| 44 | SWARM_RESULT | Swarm task result |

### OpenClaw (50-59)

| Type | Name | Description |
|------|------|-------------|
| 50 | OPENCLAW_PROMPT | AI prompt request |
| 51 | OPENCLAW_RESPONSE | AI response |
| 52 | OPENCLAW_STREAM | Streaming response chunk |

## Market Protocol Flow

### Task Announcement

```
Requester -> Broadcast: MARKET_TASK_ANNOUNCE
{
  "task_id": "uuid",
  "description": "Task description",
  "type": "openclaw_prompt",
  "max_budget": 10.0,
  "deadline": 1700000000000000000,
  "required_capabilities": ["ai-inference"],
  "minimum_reputation": 0.5,
  "requester_id": "peer-id",
  "bid_timeout": 5000000000,
  "escrow_required": true,
  "consensus_mode": false,
  "consensus_threshold": 2
}
```

### Bid Submission

```
Bidder -> Requester: MARKET_BID
{
  "task_id": "uuid",
  "bidder_id": "peer-id",
  "bid_price": 5.5,
  "estimated_duration_ms": 5000,
  "confidence_score": 0.9,
  "capability_hash": "hash-of-capabilities",
  "reputation_score": 0.8,
  "current_load": 0.3,
  "energy_cost": 0.1,
  "nonce": "random-nonce",
  "signature": "signature"
}
```

### Winner Selection

Score calculation:
```
score = (price_weight * (1/bid_price)) +
        (reputation_weight * reputation_score) +
        (latency_weight * (1/estimated_duration)) +
        (confidence_weight * confidence_score)
```

```
Requester -> Winner: MARKET_WINNER
{
  "task_id": "uuid",
  "winner_id": "peer-id",
  "winning_bid": 5.5,
  "bid_count": 5,
  "selection_reason": "highest_score",
  "timestamp": 1700000000000000000,
  "consensus_winners": ["peer-1", "peer-2"]
}
```

### Escrow Lock

```
Requester -> Network: MARKET_ESCROW_LOCK
{
  "task_id": "uuid",
  "requester_id": "peer-id",
  "amount": 5.5,
  "lock_time": 1700000000000000000,
  "expiry_time": 1700000060000000000,
  "escrow_id": "escrow-uuid"
}
```

### Settlement

```
Executor -> Requester: MARKET_SETTLEMENT
{
  "task_id": "uuid",
  "escrow_id": "escrow-uuid",
  "executor_id": "peer-id",
  "amount": 5.5,
  "status": "completed",
  "settlement_time": 1700000030000000000,
  "verification_data": { ... },
  "refund_amount": 0.0
}
```

## Memory Synchronization

### Sync Request

```
Peer A -> Peer B: MEM_SYNC_REQ
{
  "requester_id": "peer-a",
  "last_sync_time": 1700000000000000000,
  "keys": ["key1", "key2"]
}
```

### Sync Response

```
Peer B -> Peer A: MEM_SYNC_RESP
{
  "responder_id": "peer-b",
  "entries": [
    {
      "key": "key1",
      "value": "...",
      "timestamp": 1700000010000000000,
      "ttl": 3600000000000,
      "version": 5,
      "signature": "..."
    }
  ],
  "deleted_keys": ["key3"],
  "sync_time": 1700000020000000000
}
```

## Security Considerations

### Replay Protection

- All messages include a nonce
- Signatures include timestamp
- Reject messages older than 5 minutes
- Track seen nonces per peer

### Message Authentication

- All messages must be signed
- Verify signature before processing
- Reject unsigned messages

### Rate Limiting

- Limit messages per peer per second
- Exponential backoff for violations
- Temporary ban for abuse

### Encryption

- Transport-level encryption via Noise
- Optional payload encryption for sensitive data
- Memory entries can be encrypted with AES-GCM

## Error Handling

### Error Codes

| Code | Description |
|------|-------------|
| 100 | Invalid message format |
| 101 | Invalid signature |
| 102 | Message too old |
| 103 | Rate limit exceeded |
| 200 | Task not found |
| 201 | Insufficient funds |
| 202 | Bid too low |
| 203 | Auction closed |
| 300 | Escrow not found |
| 301 | Escrow expired |
| 302 | Verification failed |

### Error Response

```json
{
  "header": { ... },
  "payload": {
    "error_code": 101,
    "error_message": "Invalid signature",
    "original_message_id": "uuid"
  }
}
```

## Version Compatibility

- Protocol version in handshake
- Reject connections from incompatible versions
- Backward compatibility for one minor version

## References

- libp2p: https://libp2p.io/
- Noise Protocol: https://noiseprotocol.org/
- Ed25519: https://ed25519.cr.yp.to/
