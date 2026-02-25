# CLAWNET Installation Guide

## System Requirements

### Minimum Requirements

- CPU: 2 cores
- RAM: 512 MB
- Storage: 100 MB
- Network: Internet connection

### Recommended Requirements

- CPU: 4+ cores
- RAM: 2 GB
- Storage: 1 GB
- Network: Broadband connection

## Platform-Specific Installation

### Linux

#### Debian/Ubuntu

```bash
# Install dependencies
sudo apt-get update
sudo apt-get install -y golang git make

# Build from source
git clone https://github.com/clawnet/clawnet
cd clawnet
make build
sudo make install

# Start service
sudo systemctl enable --now clawnet
```

#### RHEL/CentOS/Fedora

```bash
# Install dependencies
sudo dnf install -y golang git make

# Build from source
git clone https://github.com/clawnet/clawnet
cd clawnet
make build
sudo make install

# Start service
sudo systemctl enable --now clawnet
```

#### Arch Linux

```bash
# Install dependencies
sudo pacman -S go git make

# Build from source
git clone https://github.com/clawnet/clawnet
cd clawnet
make build
sudo make install

# Start service
sudo systemctl enable --now clawnet
```

### macOS

```bash
# Install Homebrew if not present
/bin/bash -c "$(curl -fsSL https://raw.githubusercontent.com/Homebrew/install/HEAD/install.sh)"

# Install dependencies
brew install go git make

# Build from source
git clone https://github.com/clawnet/clawnet
cd clawnet
make build

# Install (manual)
sudo cp build/clawnet /usr/local/bin/
mkdir -p ~/.clawnet
cp configs/config.example.yaml ~/.clawnet/config.yaml

# Run
clawnet
```

### Windows

#### Using WSL2 (Recommended)

```bash
# In WSL2 Ubuntu
sudo apt-get update
sudo apt-get install -y golang git make

git clone https://github.com/clawnet/clawnet
cd clawnet
make build

# Run
./build/clawnet
```

#### Native Windows

```powershell
# Install Go from https://golang.org/dl/
# Install Git from https://git-scm.com/download/win
# Install Make (via Chocolatey)
choco install make

# Clone and build
git clone https://github.com/clawnet/clawnet
cd clawnet
make build

# Run
.\build\clawnet.exe
```

### Android (Termux)

```bash
# Install Termux from F-Droid (not Play Store)

# In Termux
pkg update
pkg install -y golang git make

git clone https://github.com/clawnet/clawnet
cd clawnet

# Build
export CGO_ENABLED=1
export GOOS=android
export GOARCH=arm64
go build -o clawnet cmd/clawnet/main.go

# Run with Android script
chmod +x scripts/android-termux.sh
./scripts/android-termux.sh
```

### VPS/Cloud

#### AWS EC2

```bash
# Ubuntu 22.04 AMI
sudo apt-get update
sudo apt-get install -y golang git make

git clone https://github.com/clawnet/clawnet
cd clawnet
make build
sudo make install

# Configure for public IP
sudo nano /etc/clawnet/config.yaml
# Set listen_addrs to use public IP

sudo systemctl enable --now clawnet
```

#### DigitalOcean Droplet

```bash
# Ubuntu 22.04
curl -fsSL https://raw.githubusercontent.com/clawnet/clawnet/main/scripts/install.sh | bash

# Or manual
git clone https://github.com/clawnet/clawnet
cd clawnet
make build
sudo make install
sudo systemctl enable --now clawnet
```

#### Google Cloud Platform

```bash
# Create instance with firewall rules for ports 4001/udp and 4002/tcp

git clone https://github.com/clawnet/clawnet
cd clawnet
make build
sudo make install

# Configure
sudo nano /etc/clawnet/config.yaml

sudo systemctl enable --now clawnet
```

## Docker Installation

### Using Docker

```bash
# Pull image
docker pull clawnet/clawnet:latest

# Run
docker run -d \
  --name clawnet \
  -p 4001:4001/udp \
  -p 4002:4002/tcp \
  -v $(pwd)/data:/data \
  clawnet/clawnet:latest
```

### Using Docker Compose

```bash
# Clone repository
git clone https://github.com/clawnet/clawnet
cd clawnet

# Create environment file
cat > .env << EOF
NODE_NAME=clawnet-docker
LOG_LEVEL=info
MARKET_ENABLED=true
ENABLE_DHT=true
ENABLE_MDNS=false
EOF

# Start
docker-compose up -d

# View logs
docker-compose logs -f clawnet
```

## Configuration

### Initial Setup

```bash
# Initialize configuration
clawnet init

# Or manually
mkdir -p ~/.clawnet
cp configs/config.example.yaml ~/.clawnet/config.yaml
```

### Key Configuration Options

```yaml
node:
  name: "my-node"  # Node identifier
  capabilities:
    - "compute"
    - "ai-inference"
    - "storage"

network:
  # For public nodes
  listen_addrs:
    - "/ip4/0.0.0.0/udp/4001/quic-v1"
    - "/ip4/0.0.0.0/tcp/4002"
  
  # Bootstrap to existing network
  bootstrap_peers:
    - "/ip4/x.x.x.x/udp/4001/quic-v1/p2p/QmPeerID"

market:
  enabled: true
  initial_wallet_balance: 1000.0
```

## Verification

### Check Installation

```bash
# Version
clawnet version

# Configuration
clawnet --config ~/.clawnet/config.yaml

# Check logs
tail -f ~/.clawnet/logs/clawnet.log
```

### Test Connectivity

```bash
# In TUI, check peer count
/peer list

# Check market status
/market status
```

## Troubleshooting

### Port Binding Issues

```bash
# Check if ports are in use
sudo lsof -i :4001
sudo lsof -i :4002

# Use different ports in config
node:
  listen_addrs:
    - "/ip4/0.0.0.0/udp/5001/quic-v1"
    - "/ip4/0.0.0.0/tcp/5002"
```

### Permission Denied

```bash
# Fix permissions
sudo chown -R $USER:$USER ~/.clawnet
chmod 700 ~/.clawnet
```

### Connection Issues

```bash
# Check firewall
sudo ufw status
sudo ufw allow 4001/udp
sudo ufw allow 4002/tcp

# Test connectivity
nc -vzu <peer-ip> 4001
```

### High Resource Usage

```yaml
# Reduce resource usage in config
node:
  max_peers: 20

market:
  max_concurrent_auctions: 5

memory:
  max_memory_size: 268435456  # 256MB
```

## Uninstallation

### Linux/macOS

```bash
# Stop service
sudo systemctl stop clawnet

# Uninstall
sudo make uninstall

# Remove data
rm -rf ~/.clawnet
```

### Docker

```bash
docker-compose down -v
docker rmi clawnet/clawnet
```

### Android

```bash
# In Termux
rm -rf ~/clawnet
rm -rf ~/.clawnet
```

## Next Steps

1. Read the [Protocol Specification](PROTOCOL.md)
2. Explore [Example Workflows](EXAMPLES.md)
3. Join the network by connecting to bootstrap peers
4. Start submitting and executing tasks
