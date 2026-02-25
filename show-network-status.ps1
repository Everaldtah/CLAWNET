#!/usr/bin/env pwsh

# CLAWNET Multi-Region Mesh Network Connection Details
# Generated: 2026-02-25

$env:PATH = "C:\Users\evera\.fly\bin;" + $env:PATH

Write-Host "========================================" -ForegroundColor Cyan
Write-Host "  CLAWNET GLOBAL MESH NETWORK" -ForegroundColor Cyan
Write-Host "========================================" -ForegroundColor Cyan
Write-Host ""

# USA Node (Chicago)
Write-Host "🇺🇸 USA NODE (Chicago)" -ForegroundColor Green
Write-Host "App: clawnet" -ForegroundColor White
Write-Host "Region: ord (Chicago, Illinois)" -ForegroundColor White
Write-Host "Dashboard: https://fly.io/apps/clawnet/monitoring" -ForegroundColor Cyan
Write-Host ""
Write-Host "Peer ID: 12D3KooWNeeCNrvmRUPsGsBUEW7jhsxGwXhEuLuHDRD2tDZQso6Y" -ForegroundColor Yellow
Write-Host "Addresses:" -ForegroundColor White
Write-Host "  /ip4/172.19.20.106/tcp/35945" -ForegroundColor Gray
Write-Host "  /ip4/172.19.20.106/udp/56214/quic-v1" -ForegroundColor Gray
Write-Host ""

# Europe Node (London)
Write-Host "🇪🇺 EUROPE NODE (London)" -ForegroundColor Green
Write-Host "App: clawnet-eu" -ForegroundColor White
Write-Host "Region: lhr (London, UK)" -ForegroundColor White
Write-Host "Dashboard: https://fly.io/apps/clawnet-eu/monitoring" -ForegroundColor Cyan
Write-Host ""
Write-Host "Peer ID: 12D3KooWJFuhmD4ENG4g7hAZi2tZWqEUVC1qbjNAZwJtop6wzo6z" -ForegroundColor Yellow
Write-Host "Addresses:" -ForegroundColor White
Write-Host "  /ip4/172.19.17.82/tcp/39087" -ForegroundColor Gray
Write-Host "  /ip4/172.19.17.82/udp/59166/quic-v1" -ForegroundColor Gray
Write-Host ""

# Asia Node (Singapore)
Write-Host "🇸🇬 ASIA NODE (Singapore)" -ForegroundColor Green
Write-Host "App: clawnet-asia" -ForegroundColor White
Write-Host "Region: sin (Singapore)" -ForegroundColor White
Write-Host "Dashboard: https://fly.io/apps/clawnet-asia/monitoring" -ForegroundColor Cyan
Write-Host ""
Write-Host "Peer ID: Pending..." -ForegroundColor Yellow
Write-Host "Addresses: Pending..." -ForegroundColor Gray
Write-Host ""

Write-Host "========================================" -ForegroundColor Cyan
Write-Host "  BOOTSTRAP PEER CONFIGURATION" -ForegroundColor Cyan
Write-Host "========================================" -ForegroundColor Cyan
Write-Host ""
Write-Host "To connect all nodes into a mesh network, update each" -ForegroundColor White
Write-Host "node's configuration with the peer IDs and addresses:" -ForegroundColor White
Write-Host ""

# Generate multiaddr format for bootstrap
Write-Host "Bootstrap Addresses for config.yaml:" -ForegroundColor Yellow
Write-Host ""
Write-Host "USA (for Europe/Asia):" -ForegroundColor Green
Write-Host "  /ip4/172.19.20.106/tcp/35945/p2p/12D3KooWNeeCNrvmRUPsGsBUEW7jhsxGwXhEuLuHDRD2tDZQso6Y" -ForegroundColor Cyan
Write-Host ""
Write-Host "Europe (for USA/Asia):" -ForegroundColor Green
Write-Host "  /ip4/172.19.17.82/tcp/39087/p2p/12D3KooWJFuhmD4ENG4g7hAZi2tZWqEUVC1qbjNAZwJtop6wzo6z" -ForegroundColor Cyan
Write-Host ""

Write-Host "========================================" -ForegroundColor Cyan
Write-Host "  MONITORING COMMANDS" -ForegroundColor Cyan
Write-Host "========================================" -ForegroundColor Cyan
Write-Host ""
Write-Host "View USA node logs:" -ForegroundColor White
Write-Host "  flyctl logs --app clawnet" -ForegroundColor Yellow
Write-Host ""
Write-Host "View Europe node logs:" -ForegroundColor White
Write-Host "  flyctl logs --app clawnet-eu" -ForegroundColor Yellow
Write-Host ""
Write-Host "View Asia node logs:" -ForegroundColor White
Write-Host "  flyctl logs --app clawnet-asia" -ForegroundColor Yellow
Write-Host ""

Write-Host "========================================" -ForegroundColor Cyan
Write-Host "  MESH NETWORK STATUS" -ForegroundColor Cyan
Write-Host "========================================" -ForegroundColor Cyan
Write-Host ""
Write-Host "Total Nodes Deployed: 3" -ForegroundColor Green
Write-Host "Regions: USA, Europe, Asia" -ForegroundColor Green
Write-Host "Network Type: libp2p P2P Mesh" -ForegroundColor Green
Write-Host ""
Write-Host "Next Steps:" -ForegroundColor Yellow
Write-Host "1. All nodes are running and discovering peers via DHT" -ForegroundColor White
Write-Host "2. Nodes will automatically connect to each other" -ForegroundColor White
Write-Host "3. Monitor the network growth via Fly.io dashboards" -ForegroundColor White
Write-Host ""
