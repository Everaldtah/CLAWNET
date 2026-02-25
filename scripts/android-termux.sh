#!/data/data/com.termux/files/usr/bin/bash
#
# CLAWNET Android/Termux Startup Script
# 
# Installation:
# 1. Install Termux from F-Droid
# 2. pkg update && pkg upgrade
# 3. pkg install golang git
# 4. git clone https://github.com/clawnet/clawnet
# 5. cd clawnet && make build
# 6. ./scripts/android-termux.sh
#

set -e

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Configuration
CLAWNET_DIR="$HOME/clawnet"
DATA_DIR="$HOME/.clawnet"
CONFIG_FILE="$DATA_DIR/config.yaml"
LOG_FILE="$DATA_DIR/clawnet.log"
PID_FILE="$DATA_DIR/clawnet.pid"

# Functions
log_info() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

log_warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

check_termux() {
    if [ -z "$TERMUX_VERSION" ]; then
        log_error "This script must run in Termux"
        exit 1
    fi
}

check_dependencies() {
    log_info "Checking dependencies..."
    
    # Check Go
    if ! command -v go &> /dev/null; then
        log_warn "Go not found, installing..."
        pkg install -y golang
    fi
    
    # Check git
    if ! command -v git &> /dev/null; then
        log_warn "Git not found, installing..."
        pkg install -y git
    fi
    
    log_info "Dependencies OK"
}

setup_directories() {
    log_info "Setting up directories..."
    mkdir -p "$DATA_DIR"
    mkdir -p "$DATA_DIR/logs"
    log_info "Directories created"
}

create_config() {
    if [ ! -f "$CONFIG_FILE" ]; then
        log_info "Creating default configuration..."
        cat > "$CONFIG_FILE" << 'EOF'
node:
  name: "clawnet-android"
  data_dir: ".clawnet"
  listen_addrs:
    - "/ip4/0.0.0.0/udp/0/quic-v1"
    - "/ip4/0.0.0.0/tcp/0"
  bootstrap_peers: []
  capabilities:
    - "compute"
    - "storage"
    - "ai-inference"
  max_peers: 50
  min_peers: 3

network:
  enable_quic: true
  enable_tcp: true
  enable_mdns: true
  enable_dht: true
  dht_mode: "client"
  connection_timeout: "30s"
  ping_interval: "30s"
  enable_relay: true
  enable_nat_portmap: false

market:
  enabled: true
  initial_wallet_balance: 500.0
  min_bid_increment: 0.1
  max_concurrent_auctions: 5
  bid_timeout: "10s"
  task_timeout: "120s"
  weights:
    price: 0.4
    reputation: 0.3
    latency: 0.1
    confidence: 0.2
  escrow_required: true
  reputation_decay_rate: 0.01
  enable_swarm_mode: true
  anti_collusion_checks: true
  rate_limit_bids: 50
  fraud_detection: true
  consensus_mode: false
  consensus_threshold: 2

memory:
  enabled: true
  sync_interval: "60s"
  max_memory_size: 268435456  # 256MB for mobile
  compression_level: 6
  encryption_enabled: true
  gc_interval: "10m"

openclaw:
  enabled: false  # Set to true if OpenClaw server is available
  api_endpoint: "http://localhost:8080"
  api_timeout: "60s"
  max_tokens: 2048
  temperature: 0.7
  model: "default"
  enable_streaming: true

tui:
  enabled: true
  theme: "dark"
  refresh_rate: 5
  show_peers_panel: true
  show_market_panel: true
  show_memory_panel: true
  show_log_panel: true

log:
  level: "info"
  format: "text"
  output: "file"
  max_size: 50
  max_backups: 2
  max_age: 3
EOF
        log_info "Configuration created at $CONFIG_FILE"
    fi
}

build_clawnet() {
    if [ ! -f "$CLAWNET_DIR/clawnet" ]; then
        log_info "Building CLAWNET..."
        cd "$CLAWNET_DIR"
        
        # Set Go environment for Termux
        export CGO_ENABLED=1
        export GOOS=android
        export GOARCH=arm64
        
        go build -o clawnet cmd/clawnet/main.go
        log_info "Build complete"
    fi
}

start_clawnet() {
    if [ -f "$PID_FILE" ]; then
        PID=$(cat "$PID_FILE")
        if ps -p "$PID" > /dev/null 2>&1; then
            log_warn "CLAWNET is already running (PID: $PID)"
            return
        fi
    fi
    
    log_info "Starting CLAWNET..."
    
    # Wake lock to prevent sleep
    termux-wake-lock
    
    # Start CLAWNET
    nohup "$CLAWNET_DIR/clawnet" --config "$CONFIG_FILE" > "$LOG_FILE" 2>&1 &
    echo $! > "$PID_FILE"
    
    sleep 2
    
    if ps -p $(cat "$PID_FILE") > /dev/null 2>&1; then
        log_info "CLAWNET started successfully (PID: $(cat $PID_FILE))"
        log_info "Logs: tail -f $LOG_FILE"
    else
        log_error "Failed to start CLAWNET"
        rm -f "$PID_FILE"
        return 1
    fi
}

stop_clawnet() {
    if [ -f "$PID_FILE" ]; then
        PID=$(cat "$PID_FILE")
        if ps -p "$PID" > /dev/null 2>&1; then
            log_info "Stopping CLAWNET (PID: $PID)..."
            kill "$PID"
            
            # Wait for shutdown
            for i in {1..10}; do
                if ! ps -p "$PID" > /dev/null 2>&1; then
                    break
                fi
                sleep 1
            done
            
            # Force kill if still running
            if ps -p "$PID" > /dev/null 2>&1; then
                log_warn "Force stopping CLAWNET..."
                kill -9 "$PID"
            fi
            
            log_info "CLAWNET stopped"
        fi
        rm -f "$PID_FILE"
    else
        log_warn "CLAWNET is not running"
    fi
    
    # Release wake lock
    termux-wake-unlock
}

status_clawnet() {
    if [ -f "$PID_FILE" ]; then
        PID=$(cat "$PID_FILE")
        if ps -p "$PID" > /dev/null 2>&1; then
            log_info "CLAWNET is running (PID: $PID)"
            log_info "Log file: $LOG_FILE"
            
            # Show recent logs
            if [ -f "$LOG_FILE" ]; then
                echo ""
                echo "Recent logs:"
                tail -n 10 "$LOG_FILE"
            fi
        else
            log_warn "CLAWNET is not running (stale PID file)"
            rm -f "$PID_FILE"
        fi
    else
        log_warn "CLAWNET is not running"
    fi
}

show_menu() {
    echo ""
    echo "================================"
    echo "    CLAWNET Android Manager"
    echo "================================"
    echo ""
    echo "1. Start CLAWNET"
    echo "2. Stop CLAWNET"
    echo "3. Restart CLAWNET"
    echo "4. View Status"
    echo "5. View Logs"
    echo "6. Edit Configuration"
    echo "7. Check for Updates"
    echo "8. Exit"
    echo ""
}

main_loop() {
    while true; do
        show_menu
        read -p "Select option: " choice
        
        case $choice in
            1)
                start_clawnet
                ;;
            2)
                stop_clawnet
                ;;
            3)
                stop_clawnet
                sleep 2
                start_clawnet
                ;;
            4)
                status_clawnet
                ;;
            5)
                if [ -f "$LOG_FILE" ]; then
                    tail -f "$LOG_FILE"
                else
                    log_warn "Log file not found"
                fi
                ;;
            6)
                ${EDITOR:-nano} "$CONFIG_FILE"
                ;;
            7)
                log_info "Checking for updates..."
                cd "$CLAWNET_DIR"
                git fetch origin
                LOCAL=$(git rev-parse HEAD)
                REMOTE=$(git rev-parse origin/main)
                if [ "$LOCAL" != "$REMOTE" ]; then
                    log_info "Update available! Run: cd $CLAWNET_DIR && git pull && make build"
                else
                    log_info "Already up to date"
                fi
                ;;
            8)
                log_info "Goodbye!"
                exit 0
                ;;
            *)
                log_error "Invalid option"
                ;;
        esac
        
        echo ""
        read -p "Press Enter to continue..."
    done
}

# Main
main() {
    check_termux
    check_dependencies
    setup_directories
    create_config
    
    # Handle command line arguments
    case "${1:-menu}" in
        start)
            start_clawnet
            ;;
        stop)
            stop_clawnet
            ;;
        restart)
            stop_clawnet
            sleep 2
            start_clawnet
            ;;
        status)
            status_clawnet
            ;;
        logs)
            tail -f "$LOG_FILE"
            ;;
        build)
            build_clawnet
            ;;
        *)
            main_loop
            ;;
    esac
}

main "$@"
