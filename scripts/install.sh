#!/bin/bash
#
# CLAWNET Quick Install Script
# Supports: Linux (Debian/Ubuntu, RHEL/CentOS, Arch), macOS
#

set -e

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Configuration
REPO_URL="https://github.com/clawnet/clawnet"
INSTALL_DIR="/usr/local/bin"
CONFIG_DIR="/etc/clawnet"
DATA_DIR="/var/lib/clawnet"
SERVICE_DIR="/etc/systemd/system"

# Logging
log_info() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

log_warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

log_step() {
    echo -e "${BLUE}[STEP]${NC} $1"
}

# Detect OS
detect_os() {
    if [ -f /etc/os-release ]; then
        . /etc/os-release
        OS=$ID
        VERSION=$VERSION_ID
    elif [ -f /etc/redhat-release ]; then
        OS="rhel"
        VERSION=$(cat /etc/redhat-release | grep -oE '[0-9]+' | head -1)
    elif [ "$(uname)" == "Darwin" ]; then
        OS="macos"
        VERSION=$(sw_vers -productVersion)
    else
        OS=$(uname -s)
        VERSION=$(uname -r)
    fi
    
    log_info "Detected OS: $OS $VERSION"
}

# Check if running as root
check_root() {
    if [ "$EUID" -ne 0 ]; then
        log_error "Please run as root (use sudo)"
        exit 1
    fi
}

# Install dependencies
install_deps() {
    log_step "Installing dependencies..."
    
    case $OS in
        ubuntu|debian)
            apt-get update
            apt-get install -y golang git make curl
            ;;
        rhel|centos|fedora|rocky|almalinux)
            if command -v dnf &> /dev/null; then
                dnf install -y golang git make curl
            else
                yum install -y golang git make curl
            fi
            ;;
        arch|manjaro)
            pacman -Sy --noconfirm go git make curl
            ;;
        macos)
            if ! command -v brew &> /dev/null; then
                log_info "Installing Homebrew..."
                /bin/bash -c "$(curl -fsSL https://raw.githubusercontent.com/Homebrew/install/HEAD/install.sh)"
            fi
            brew install go git make
            ;;
        *)
            log_warn "Unknown OS, please install Go, Git, and Make manually"
            ;;
    esac
    
    log_info "Dependencies installed"
}

# Download and build
download_and_build() {
    log_step "Downloading CLAWNET..."
    
    BUILD_DIR=$(mktemp -d)
    cd "$BUILD_DIR"
    
    git clone --depth 1 "$REPO_URL" .
    
    log_step "Building CLAWNET..."
    make build
    
    log_info "Build complete"
}

# Install binary
install_binary() {
    log_step "Installing binary..."
    
    cp "$BUILD_DIR/build/clawnet" "$INSTALL_DIR/"
    chmod +x "$INSTALL_DIR/clawnet"
    
    log_info "Binary installed to $INSTALL_DIR/clawnet"
}

# Create directories
setup_directories() {
    log_step "Setting up directories..."
    
    mkdir -p "$CONFIG_DIR"
    mkdir -p "$DATA_DIR"
    mkdir -p "$DATA_DIR/logs"
    
    log_info "Directories created"
}

# Create configuration
create_config() {
    log_step "Creating configuration..."
    
    if [ ! -f "$CONFIG_DIR/config.yaml" ]; then
        cat > "$CONFIG_DIR/config.yaml" << 'EOF'
node:
  name: "clawnet-node"
  data_dir: "/var/lib/clawnet"
  listen_addrs:
    - "/ip4/0.0.0.0/udp/4001/quic-v1"
    - "/ip4/0.0.0.0/tcp/4002"
  bootstrap_peers: []
  capabilities:
    - "compute"
    - "storage"
    - "ai-inference"
  max_peers: 100
  min_peers: 5

network:
  enable_quic: true
  enable_tcp: true
  enable_mdns: true
  enable_dht: true
  dht_mode: "auto"
  connection_timeout: "30s"
  ping_interval: "15s"
  enable_relay: true
  enable_nat_portmap: true

market:
  enabled: true
  initial_wallet_balance: 1000.0
  min_bid_increment: 0.1
  max_concurrent_auctions: 10
  bid_timeout: "5s"
  task_timeout: "60s"
  weights:
    price: 0.4
    reputation: 0.3
    latency: 0.1
    confidence: 0.2
  escrow_required: true
  reputation_decay_rate: 0.01
  enable_swarm_mode: true
  anti_collusion_checks: true
  rate_limit_bids: 100
  fraud_detection: true
  consensus_mode: false
  consensus_threshold: 2

memory:
  enabled: true
  sync_interval: "30s"
  max_memory_size: 1073741824
  compression_level: 6
  encryption_enabled: true
  gc_interval: "5m"

openclaw:
  enabled: true
  api_endpoint: "http://localhost:8080"
  api_timeout: "30s"
  max_tokens: 4096
  temperature: 0.7
  model: "default"
  enable_streaming: true

tui:
  enabled: true
  theme: "dark"
  refresh_rate: 10
  show_peers_panel: true
  show_market_panel: true
  show_memory_panel: true
  show_log_panel: true

log:
  level: "info"
  format: "json"
  output: "stdout"
  max_size: 100
  max_backups: 3
  max_age: 7
EOF
        
        log_info "Configuration created at $CONFIG_DIR/config.yaml"
    else
        log_warn "Configuration already exists, skipping"
    fi
}

# Create systemd service
create_service() {
    log_step "Creating systemd service..."
    
    cat > "$SERVICE_DIR/clawnet.service" << EOF
[Unit]
Description=CLAWNET - Distributed AI Mesh Network
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
User=clawnet
Group=clawnet
WorkingDirectory=$DATA_DIR
ExecStart=$INSTALL_DIR/clawnet --config $CONFIG_DIR/config.yaml
Restart=on-failure
RestartSec=5
LimitNOFILE=65536
LimitNPROC=4096
NoNewPrivileges=true
ProtectSystem=strict
ProtectHome=true
ReadWritePaths=$DATA_DIR
StandardOutput=journal
StandardError=journal

[Install]
WantedBy=multi-user.target
EOF
    
    log_info "Service created"
}

# Create user
create_user() {
    log_step "Creating clawnet user..."
    
    if ! id -u clawnet &> /dev/null; then
        useradd -r -s /bin/false -d "$DATA_DIR" clawnet
        log_info "User created"
    else
        log_warn "User already exists"
    fi
    
    chown -R clawnet:clawnet "$DATA_DIR"
    chmod 750 "$DATA_DIR"
}

# Configure firewall
configure_firewall() {
    log_step "Configuring firewall..."
    
    if command -v ufw &> /dev/null; then
        ufw allow 4001/udp
        ufw allow 4002/tcp
        log_info "UFW rules added"
    elif command -v firewall-cmd &> /dev/null; then
        firewall-cmd --permanent --add-port=4001/udp
        firewall-cmd --permanent --add-port=4002/tcp
        firewall-cmd --reload
        log_info "FirewallD rules added"
    elif command -v iptables &> /dev/null; then
        iptables -A INPUT -p udp --dport 4001 -j ACCEPT
        iptables -A INPUT -p tcp --dport 4002 -j ACCEPT
        log_info "iptables rules added"
    else
        log_warn "No firewall detected, please configure manually"
    fi
}

# Start service
start_service() {
    log_step "Starting CLAWNET service..."
    
    systemctl daemon-reload
    systemctl enable clawnet
    systemctl start clawnet
    
    sleep 2
    
    if systemctl is-active --quiet clawnet; then
        log_info "CLAWNET is running!"
        systemctl status clawnet --no-pager
    else
        log_error "Failed to start CLAWNET"
        systemctl status clawnet --no-pager || true
        exit 1
    fi
}

# Cleanup
cleanup() {
    if [ -n "$BUILD_DIR" ] && [ -d "$BUILD_DIR" ]; then
        rm -rf "$BUILD_DIR"
    fi
}

# Print summary
print_summary() {
    echo ""
    echo "======================================"
    echo "    CLAWNET Installation Complete!"
    echo "======================================"
    echo ""
    echo "Configuration: $CONFIG_DIR/config.yaml"
    echo "Data Directory: $DATA_DIR"
    echo "Binary: $INSTALL_DIR/clawnet"
    echo ""
    echo "Commands:"
    echo "  systemctl status clawnet  - Check status"
    echo "  systemctl stop clawnet    - Stop service"
    echo "  systemctl start clawnet   - Start service"
    echo "  journalctl -u clawnet -f  - View logs"
    echo ""
    echo "Edit configuration and restart to customize."
    echo ""
}

# Main
main() {
    echo "======================================"
    echo "    CLAWNET Installer"
    echo "======================================"
    echo ""
    
    detect_os
    
    # macOS doesn't need root for Homebrew
    if [ "$OS" != "macos" ]; then
        check_root
    fi
    
    install_deps
    download_and_build
    install_binary
    setup_directories
    create_config
    
    if [ "$OS" != "macos" ]; then
        create_service
        create_user
        configure_firewall
        start_service
    fi
    
    cleanup
    print_summary
}

# Run
trap cleanup EXIT
main
