#!/bin/bash
# get-peer-ids.sh - Get Peer IDs from Railway deployment

echo "🔍 Getting Peer IDs from Railway CLAWNET nodes..."
echo ""

# List all Railway services
echo "📋 Checking Railway services..."
railway status

echo ""
echo "────────────────────────────────────────"
echo ""

# Get logs from each service and extract Peer IDs
railway logs 2>&1 | grep "Identity:" | while read -r line; do
    # Extract Peer ID (format: [INFO] Identity: QmXyZ...)
    PEER_ID=$(echo "$line" | awk '{print $NF}')
    echo -e "✅ \033[1;32mPeer ID: $PEER_ID\033[0m"
done

echo ""
echo "────────────────────────────────────────"
echo ""
echo "📝 Add this to your ~/.clawnet/config.yaml:"
echo ""
echo "node:"
echo "  bootstrap_peers:"
echo "    - \"/ip4/<railway-ip>/tcp/4002/p2p/<PEER_ID>\""
echo ""
echo "To get the full multiaddr, check logs for 'Host initialized' line"
echo ""
