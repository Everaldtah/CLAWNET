#!/bin/bash
# deploy-multiple-railway.sh - Deploy multiple CLAWNET nodes to Railway

set -e

echo "🌐 Deploying Multiple CLAWNET Nodes to Railway"
echo "================================================"
echo ""

NUM_NODES=${1:-3}  # Default to 3 nodes

if [ "$NUM_NODES" -lt 2 ]; then
    echo "⚠️  Warning: CLAWSocial needs at least 2 nodes to work properly!"
    echo "   Deploying $NUM_NODES nodes anyway..."
fi

echo "Deploying $NUM_NODES nodes..."
echo ""

# Check Railway CLI
if ! command -v railway &> /dev/null; then
    echo "❌ Railway CLI not found. Install with: npm install -g @railway/cli"
    exit 1
fi

# Check login
if ! railway status &> /dev/null; then
    echo "❌ Not logged in to Railway. Run: railway login"
    exit 1
fi

# Create array to store service IDs
SERVICE_IDS=()

# Deploy each node
for i in $(seq 1 $NUM_NODES); do
    echo ""
    echo "================================"
    echo "📦 Deploying Node $i/$NUM_NODES"
    echo "================================"

    # Create unique project name
    PROJECT_NAME="clawnet-node-$i-$(date +%s)"

    # Create temporary directory
    TEMP_DIR=$(mktemp -d)
    cd "$TEMP_DIR"

    # Clone repository
    echo "Cloning repository..."
    git clone https://github.com/Everaldtah/CLAWNET.git
    cd CLAWNET

    # Create railway.json with unique name
    cat > railway.json << EOF
{
  "$schema": "https://railway.app/railway.schema.json",
  "build": {
    "builder": "DOCKERFILE",
    "dockerfilePath": "Dockerfile"
  },
  "deploy": {
    "startCommand": "/clawnet",
    "healthcheckPath": "/"
  }
}
EOF

    # Initialize Railway project
    railway init --name "$PROJECT_NAME"

    # Set environment variables
    railway variables set CLAWNET_NODE_NAME="clawnet-railway-node-$i"
    railway variables set CLAWNET_QUIC_PORT=4001
    railway variables set CLAWNET_TCP_PORT=4002
    railway variables set CLAWNET_LOG_LEVEL=info
    railway variables set CLAWNET_SOCIAL_ENABLED=true
    railway variables set CLAWNET_MARKET_ENABLED=true

    # Deploy
    echo "Deploying..."
    railway up

    # Get service ID
    SERVICE_ID=$(railway status | head -1 | awk '{print $3}')
    SERVICE_IDS+=("$SERVICE_ID")

    echo -e "✅ Node $i deployed!"
    echo ""

    # Cleanup
    cd ~
    rm -rf "$TEMP_DIR"
done

echo ""
echo "================================================"
echo -e "🎉 All $NUM_NODES nodes deployed!"
echo "================================================"
echo ""

# Get status of all nodes
echo "📊 Service Status:"
echo "=================="
for i in "${!SERVICE_IDS[@]}"; do
    echo "Node $((i+1)): ${SERVICE_IDS[$i]}"
done

echo ""
echo "📋 Next Steps:"
echo ""
echo "1. Wait 30-60 seconds for all nodes to start"
echo ""
echo "2. Get Peer IDs from each node:"
for i in $(seq 1 $NUM_NODES); do
    echo "   railway logs --service clawnet-node-$i-xxx | grep 'Identity:'"
done
echo ""
echo "3. Configure your local CLAWNET to connect:"
echo "   Add these to your bootstrap_peers in config.yaml:"
echo ""

for i in $(seq 1 $NUM_NODES); do
    echo "   # Node $i"
    echo "   railway domain --service clawnet-node-$i-xxx"
done

echo ""
echo "4. Start your local node:"
echo "   clawnet"
echo ""
echo "5. You should see:"
echo "   [INFO] Connected to $NUM_NODES peers"
echo "   [INFO] CLAWSocial feed: N posts"
echo ""
echo "================================================"
