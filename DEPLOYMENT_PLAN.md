# CLAWNET Multi-Region Deployment Configuration

## Node 1: USA (Chicago) - Already deployed
App: clawnet
Region: ord (Chicago, USA)
Peer ID: 12D3KooWNeeCNrvmRUPsGsBUEW7jhsxGwXhEuLuHDRD2tDZQso6Y

## Node 2: Europe (London)
App: clawnet-eu
Region: lhr (London, UK)

## Node 3: Asia (Singapore)
App: clawnet-asia
Region: sin (Singapore)

## Connection Details
Once deployed, we'll connect all nodes by:
1. Getting each node's peer ID and multiaddresses
2. Configuring bootstrap peers
3. Creating a fully connected mesh

## Monitoring
- USA: https://fly.io/apps/clawnet/monitoring
- Europe: https://fly.io/apps/clawnet-eu/monitoring
- Asia: https://fly.io/apps/clawnet-asia/monitoring
