# TxRadar

A high-performance Ethereum Mempool monitoring and transaction relay system.

## Features

- 📡 Real-time monitoring of pending Mempool transactions and new blocks
- 🔥 Detects high-value transactions (DEX swaps, liquidations, NFT Mints)
- 📊 GasPrice sliding window + P95 percentile calculation
- 💾 Hot/cold data separated storage (PostgreSQL + InfluxDB)
- 🚀 Real-time push (WebSocket / Webhook / MQ)
- 📈 Prometheus + Grafana observability
- 🐳 One-click deployment with Docker Compose

## Quick Start

### 1. Clone the repository

```bash
git clone https://github.com/jetsea/tx-radar.git
cd txradar
