# CLAWNET Deployment Guide

Complete guide for deploying CLAWNET on various platforms.

## Table of Contents

1. [VPS Deployment with Docker](#vps-deployment-with-docker)
2. [Kubernetes Deployment](#kubernetes-deployment)
3. [Systemd Service Deployment](#systemd-service-deployment)
4. [Cloud Platform Deployment](#cloud-platform-deployment)
5. [Production Configuration](#production-configuration)
6. [Monitoring and Maintenance](#monitoring-and-maintenance)
7. [Security Best Practices](#security-best-practices)

---

## VPS Deployment with Docker

### Prerequisites

- VPS with at least 1GB RAM and 1 CPU
- Ubuntu 20.04+ / Debian 11+ / CentOS 8+
- Docker 20.10+ and Docker Compose 2.0+
- Public IP with open ports

### Quick Start

```bash
# Clone repository
git clone https://github.com/Everaldtah/CLAWNET
cd CLAWNET

# Copy example config
cp configs/config.example.yaml config.yaml

# Edit configuration
nano config.yaml

# Start with Docker Compose
docker-compose up -d
```

### Firewall Configuration

```bash
# Ubuntu/Debian (UFW)
sudo ufw allow 22/tcp      # SSH
sudo ufw allow 4001/udp    # QUIC
sudo ufw allow 4002/tcp    # TCP
sudo ufw enable

# CentOS/RHEL (firewalld)
sudo firewall-cmd --permanent --add-service=ssh
sudo firewall-cmd --permanent --add-port=4001/udp
sudo firewall-cmd --permanent --add-port=4002/tcp
sudo firewall-cmd --reload
```

### Docker Compose Configuration

```yaml
version: '3.8'

services:
  clawnet:
    image: clawnet:latest
    container_name: clawnet
    restart: unless-stopped
    ports:
      - "4001:4001/udp"
      - "4002:4002"
    volumes:
      - clawnet-data:/data
      - ./config.yaml:/app/config.yaml:ro
    environment:
      - CLAWNET_NODE_NAME=${CLAWNET_NODE_NAME}
      - CLAWNET_LOG_LEVEL=${CLAWNET_LOG_LEVEL}
    networks:
      - clawnet-net
    logging:
      driver: "json-file"
      options:
        max-size: "10m"
        max-file: "3"

volumes:
  clawnet-data:
    driver: local

networks:
  clawnet-net:
    driver: bridge
```

---

## Production Configuration

### Optimized config.yaml

```yaml
node:
  name: "clawnet-prod"
  data_dir: "/var/lib/clawnet"
  listen_addrs:
    - "/ip4/0.0.0.0/udp/4001/quic-v1"
    - "/ip4/0.0.0.0/tcp/4002"
  capabilities:
    - "compute"
    - "storage"
    - "ai-inference"
  max_peers: 200
  min_peers: 10

network:
  enable_quic: true
  enable_tcp: true
  quic_port: 4001
  tcp_port: 4002
  enable_mdns: true
  enable_dht: true
  connection_timeout: 60s
  ping_interval: 30s
  enable_relay: true
  enable_nat_port_map: true

market:
  enabled: true
  initial_wallet_balance: 10000.0
  bid_timeout: 60s
  task_timeout: 600s
  escrow_required: true

memory:
  enabled: true
  encryption_enabled: true

log:
  level: "info"
  format: "json"
```

---

## Support

For additional help:
- GitHub Issues: https://github.com/Everaldtah/CLAWNET/issues
