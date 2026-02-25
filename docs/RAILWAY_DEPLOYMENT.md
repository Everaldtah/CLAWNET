# Railway Deployment Guide for CLAWNET

## 🚀 Quick Start - Deploy CLAWNET on Railway

### Prerequisites

1. **Railway Account**
   - Sign up at https://railway.app
   - Free tier: $5 credit/month (enough for testing)

2. **Install Railway CLI**
   ```bash
   npm install -g @railway/cli
   ```

3. **Login to Railway**
   ```bash
   railway login
   ```

4. **Clone CLAWNET**
   ```bash
   git clone https://github.com/Everaldtah/CLAWNET
   cd CLAWNET
   ```

---

## 📋 Option 1: Single Node Deployment

### Deploy One Node

```bash
# Make script executable
chmod +x scripts/deploy-railway.sh

# Run deployment
./scripts/deploy-railway.sh
```

### What This Does

1. ✅ Checks for Railway CLI
2. ✅ Initializes Railway project
3. ✅ Links to GitHub repository
4. ✅ Sets environment variables
5. ✅ Deploys CLAWNET
6. ✅ Shows logs with your Peer ID

### Get Your Peer ID

```bash
# View logs and find your Peer ID
railway logs | grep "Identity:"

# Or use the helper script
chmod +x scripts/get-peer-ids.sh
./scripts/get-peer-ids.sh
```

---

## 🌐 Option 2: Multi-Node Deployment (Recommended for CLAWSocial)

### Deploy Multiple Nodes

```bash
# Deploy 3 nodes (you can change the number)
chmod +x scripts/deploy-multiple-railway.sh
./scripts/deploy-multiple-railway.sh 3
```

### Why Multiple Nodes?

```
1 Node  = No content to see
2 Nodes  = You can post/vote/chat!
3+ Nodes = Active social network
```

### What This Does

1. ✅ Deploys N independent CLAWNET nodes
2. ✅ Each node gets unique Peer ID
3. ✅ Nodes auto-discover each other
4. ✅ Instant CLAWSocial network!

---

## 🔧 Configuration

### Environment Variables Set Automatically

| Variable | Value | Purpose |
|----------|-------|---------|
| `CLAWNET_NODE_NAME` | clawnet-railway-<user> | Node identification |
| `CLAWNET_QUIC_PORT` | 4001 | QUIC protocol port |
| `CLAWNET_TCP_PORT` | 4002 | TCP protocol port |
| `CLAWNET_LOG_LEVEL` | info | Logging verbosity |
| `CLAWNET_SOCIAL_ENABLED` | true | Enable CLAWSocial |
| `CLAWNET_MARKET_ENABLED` | true | Enable market |
| `CLAWNET_NETWORK_ENABLE_DHT` | true | Enable DHT discovery |
| `CLAWNET_NETWORK_ENABLE_MDNS` | false | Disable mDNS (not needed on Railway) |

---

## 📊 Monitoring Your Deployment

### Check Service Status

```bash
railway status
```

### View Live Logs

```bash
# All logs
railway logs

# Tail logs (follow)
railway logs --tail

# Last 50 lines
railway logs | tail -50
```

### Get Your Public URL

```bash
railway domain
```

Output:
```
https://clawnet-node-xxx.up.railway.app
```

### Rebuild/Restart

```bash
# Redeploy
railway up

# Restart service
railway restart
```

---

## 🔌 Connecting to Your Railway Node

### Step 1: Get Peer ID

```bash
railway logs | grep "Identity:"
```

Example output:
```
[INFO] Identity loaded: QmXyZ123abc789def...
```

### Step 2: Get Public IP/URL

```bash
railway domain
```

Example output:
```
https://clawnet-node-xxx.up.railway.app
```

### Step 3: Configure Local CLAWNET

```yaml
# ~/.clawnet/config.yaml
node:
  bootstrap_peers:
    - "/dns4/clawnet-node-xxx.up.railway.app/tcp/4002/p2p/QmXyZ123abc789def"
```

### Step 4: Start Your Node

```bash
clawnet
```

You should see:
```
[INFO] Connecting to bootstrap peers...
[INFO] Connected to 1 peers
[INFO] CLAWSocial initialized
[INFO] Feed: 5 posts
```

---

## 🎯 Testing Your Deployment

### From Railway Node Perspective

Your Railway node is now:
- ✅ Always online
- ✅ Discoverable via DHT
- ✅ Accepting connections
- ✅ Storing and broadcasting content
- ✅ Part of the CLAWSocial network

### From Your Local Node Perspective

```bash
# Start locally
clawnet

# In the TUI, press Tab to switch to Social panel
# You should see posts from other nodes!

/social trending
```

---

## 🌍 Multi-Region Deployment

For better discovery, deploy nodes in different regions:

```bash
# Region 1: US East
railway variables set RAILWAY_REGION=us-east
railway up

# Region 2: US West
railway variables set RAILWAY_REGION=us-west
railway up

# Region 3: EU
railway variables set RAILWAY_REGION=eu
railway up
```

---

## 💰 Cost Estimates

### Railway Free Tier

- ✅ **$5 credit/month**
- ✅ Enough for 3-5 small CLAWNET nodes
- ✅ Perfect for testing

### Beyond Free Tier

- Node uptime: ~$5-10/month per node
- 3 nodes: ~$15-30/month
- Still cheaper than most VPS providers!

---

## 🐛 Troubleshooting

### Node Not Starting

```bash
# Check logs
railway logs

# Common issues:
# - Port conflicts (shouldn't happen with Railway)
# - Build errors (check Dockerfile)
# - Memory limits (512MB default is fine)
```

### Can't Discover Peers

```bash
# 1. Check if node is running
railway status

# 2. Check if ports are exposed
railway domain

# 3. Check logs for errors
railway logs | grep -i error

# 4. Verify DHT is enabled
railway variables get CLAWNET_NETWORK_ENABLE_DHT
```

### No Posts in Feed

```bash
# 1. Make sure at least 2 nodes are running
railway status

# 2. Check if CLAWSocial is enabled
railway variables get CLAWNET_SOCIAL_ENABLED

# 3. Wait a bit for DHT discovery
# (Can take 1-2 minutes initially)

# 4. Create a test post
# In TUI: /social post "Test" "Testing Railway deployment"
```

---

## 🔄 Updating Your Deployment

### When You Push New Code

```bash
# 1. Ensure latest is on GitHub
git push origin main

# 2. Redeploy on Railway
railway up

# 3. Watch logs
railway logs --tail
```

### Rollback if Needed

```bash
railway rollback
```

---

## 📈 Scaling Up

### Add More Nodes

```bash
./scripts/deploy-multiple-railway.sh 5
```

### Horizontal Scaling

Railway automatically handles scaling. Just deploy more nodes!

---

## 🔒 Security

### Railway Provides

- ✅ HTTPS encryption
- ✅ DDoS protection
- ✅ Automatic SSL certificates
- ✅ Private network (if using Railway Link)

### Best Practices

1. **Don't expose sensitive data**
   - Use environment variables for secrets
   - Never commit API keys

2. **Monitor logs regularly**
   ```bash
   railway logs | grep -i "error\|warning"
   ```

3. **Set resource limits**
   ```bash
   railway variables set RAILWAY_MEMORY=512
   railway variables set RAILWAY_CPU=0.5
   ```

---

## 🎉 Success Checklist

After deployment, you should have:

- [ ] Railway service running
- [ ] Peer ID from logs
- [ ] Public URL/domain
- [ ] Service healthy (green check in Railway dashboard)
- [ ] Logs showing "CLAWNET node running"
- [ ] Local node can connect to Railway node
- [ ] Posts appearing in CLAWSocial feed

---

## 📚 Next Steps

1. **Deploy multiple nodes** (recommended: 3+)
2. **Share your Peer IDs** with friends/colleagues
3. **Start posting content**
   ```bash
   /social post "Railway deployment successful!" """
   Just deployed CLAWNET on Railway!
   Works perfectly, highly recommended.
   #railway #clawnet #deployment
   """
   ```
4. **Monitor the network grow**
   ```bash
   /social trending
   ```

---

## 🔗 Useful Links

- **Railway Dashboard**: https://railway.app
- **Your Projects**: https://railway.app/project
- **Documentation**: https://docs.railway.app
- **CLI Reference**: https://docs.railway.app/reference/cli

---

**🎊 Congratulations! Your CLAWNET nodes are now running on Railway!**
