#!/bin/bash
# deploy-railway.sh - Deploy CLAWNET to Railway

set -e

echo "🚀 CLAWNET Railway Deployment Script"
echo "======================================"
echo ""

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Check if Railway CLI is installed
if ! command -v railway &> /dev/null; then
    echo -e "${YELLOW}Railway CLI not found. Installing...${NC}"
    npm install -g @railway/cli
fi

echo -e "${GREEN}✓${NC} Railway CLI installed"

# Check if user is logged in
echo ""
echo "📝 Checking Railway login status..."
if railway status &> /dev/null; then
    echo -e "${GREEN}✓${NC} Already logged in to Railway"
else
    echo "Please login to Railway:"
    railway login
fi

# Initialize Railway project
echo ""
echo "📦 Initializing Railway project..."
if [ ! -f ".railway/config.json" ]; then
    railway init
    echo -e "${GREEN}✓${NC} Railway project initialized"
else
    echo -e "${YELLOW}⚠${NC} Railway project already exists"
fi

# Link to GitHub repository
echo ""
echo "🔗 Linking to GitHub repository..."
read -p "Enter your GitHub repo (https://github.com/Everaldtah/CLAWNET): " repo_url

if [ -n "$repo_url" ]; then
    railway link "$repo_url"
    echo -e "${GREEN}✓${NC} Repository linked"
fi

# Set environment variables
echo ""
echo "⚙️  Setting environment variables..."

railway variables set CLAWNET_NODE_NAME="clawnet-railway-$(whoami)"
railway variables set CLAWNET_QUIC_PORT=4001
railway variables set CLAWNET_TCP_PORT=4002
railway variables set CLAWNET_LOG_LEVEL=info
railway variables set CLAWNET_SOCIAL_ENABLED=true
railway variables set CLAWNET_MARKET_ENABLED=true
railway variables set CLAWNET_NETWORK_ENABLE_DHT=true
railway variables set CLAWNET_NETWORK_ENABLE_MDNS=false

echo -e "${GREEN}✓${NC} Environment variables configured"

# Deploy
echo ""
echo "🚀 Deploying to Railway..."
railway up

echo ""
echo -e "${GREEN}✓${NC} Deployment started!"
echo ""
echo "📊 Monitoring deployment..."
railway status

echo ""
echo "⏳ Waiting for service to be ready..."
sleep 10

echo ""
echo "📋 Viewing logs to get your Peer ID..."
echo ""
railway logs | head -50

echo ""
echo "================================================"
echo -e "${GREEN}🎉 Deployment Complete!${NC}"
echo "================================================"
echo ""
echo "Next steps:"
echo ""
echo "1. Get your Peer ID:"
echo "   railway logs | grep 'Identity:'"
echo ""
echo "2. View live logs:"
echo "   railway logs --tail"
echo ""
echo "3. Check service status:"
echo "   railway status"
echo ""
echo "4. Get your public URL:"
echo "   railway domain"
echo ""
echo "5. Connect to your node:"
echo "   railway domain"
echo "   # Add to your bootstrap_peers in config.yaml"
echo ""
echo "================================================"
