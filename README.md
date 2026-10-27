# Tx-Radar: High-Performance Ethereum Mempool Monitor & Relayer

[![Go Version](https://img.shields.io/badge/Go-1.21+-00ADD8?style=flat&logo=go)](https://go.dev/)
[![Docker](https://img.shields.io/badge/Docker-Ready-2496ED?style=flat&logo=docker)](https://www.docker.com/)
<!-- [![CI](https://github.com/YOUR_USERNAME/tx_radar/actions/workflows/ci.yml/badge.svg)](https://github.com/YOUR_USERNAME/tx_radar/actions) -->
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)

A high-performance Ethereum Mempool monitoring and transaction relaying system built with Go. By utilizing a hybrid ingestion approach—combining **outbound devp2p connections to a local Geth node** with **multiple remote RPC WebSocket endpoints**—it achieves transaction capture latency of `< 200ms` and sustains a throughput of `5000+ TPS` on a single machine. It is highly optimized for arbitrage bots, MEV searchers, and whale address tracking.

## 📖 Table of Contents
- [Core Features](#-core-features)
- [System Architecture](#-system-architecture)
- [Tech Stack](#-tech-stack)
- [Quick Start](#-quick-start)
- [Monitoring & Observability](#-monitoring--observability)
- [Engineering Design Decisions (Q&A)](#-engineering-design-decisions-qa)

---

## ✨ Core Features

*   **⚡ Ultra-Low Latency Ingestion**: Combines `go-ethereum/p2p` TxPool event subscriptions with multi-node (Infura/Alchemy) WebSocket connections to ensure data capture latency `< 200ms`.
*   **🛡️ High Availability & Failover**: Built-in health checks and Circuit Breaker pattern ensure automatic, seamless failover to backup nodes when the primary RPC node disconnects or times out, achieving zero downtime.
*   **🚀 High-Concurrency Processing**: A Worker Pool model based on `goroutine + channel` (N = CPU cores * 2), coupled with Redis for idempotent deduplication, stably processes `5000+ TPS` on a single machine.
*   **🧠 Smart Parsing & Tagging**: Real-time filtering and tagging (HIGH/MEDIUM/LOW) of high-value transactions based on function signatures (e.g., `swap`, `liquidationCall`) and GasPrice percentiles.
*   **💾 Hot/Cold Data Separation**: PostgreSQL stores structured transaction details (supporting Reorg rollback), while InfluxDB stores monitoring metrics (e.g., Gas price trends), improving query efficiency by 3x.
*   **📡 Real-time Push & Relaying**: Provides WebSocket real-time pushes and Webhook callbacks. Supports integration with Kafka for message queuing, and natively supports constructing Flashbots Bundles for competitive bidding.
*   **📊 Production-Grade Observability**: Integrated with Prometheus + Grafana, monitoring 10+ core metrics (TPS, Channel backlog, RPC latency) with automated alerts (DingTalk/WeCom/Email).

---

## 🌐 System Architecture

The system utilizes a modular pipeline design to ensure high cohesion and low coupling.

```mermaid
flowchart LR
    %% 数据源
    Geth["Local Geth Node (devp2p)"] --> FM["Failover Manager"]
    RPC1["Infura WSS"] --> FM
    RPC2["Alchemy WSS"] --> FM

    %% 接入层
    subgraph Ingestion Layer
        FM --> Pipe["Go Channel Buffer"]
    end

    %% 处理层
    subgraph Processing Layer
        Pipe --> WP["Worker Pool"]
        WP --> Parse["Tx Parser"]
        Parse --> Sig["Sig Recognizer"]
        Sig --> Dedup["Redis Dedup"]
        Dedup --> Marker["High-Value Marker"]
    end

    %% 存储层
    subgraph Storage Layer
        Marker -->|Batch Insert| PG[("PostgreSQL")]
        Marker -->|Metrics Data| Influx[("InfluxDB")]
    end

    %% 中继层
    subgraph Relayer Layer
        Marker --> WS["WebSocket Server"]
        Marker --> WH["Webhook"]
        Marker --> MQ["Kafka Producer"]
    end

    %% 可观测性系统 (贯穿全局)
    subgraph Observability Layer
        Prom{{"Prometheus"}}
        Graf["Grafana"]
        Prom --> Graf
    end

    %% 贯穿每一层的监控数据流向 (用虚线表示 Metrics 上报)
    FM -.->|RPC Latency| Prom
    Pipe -.->|Channel Usage| Prom
    WP -.->|Active Goroutines| Prom
    Dedup -.->|Redis Hit Rate| Prom
    PG -.->|DB Query Time| Prom
    Influx -.->|Write QPS| Prom
    WS & WH & MQ -.->|Push Success Rate| Prom
```

---

## 🚀 Quick Start

The project loads the `.env` file via `godotenv.Load()`, which resolves it relative to the **current working directory** of the process. Because the `.env` file is located in the project root, so you **must run the program from the project root** (`tx-radar` directory).

- ✅ **Correct**: start from the project root with `go run ./cmd`.
- ❌ **Wrong**: `cd` into the `cmd` subdirectory and run `go run .` — `godotenv.Load()` won't find the root `.env` then.

> 💡 Tip: keep the `.env` file in the project root and fill it in by copying
> the `.env.example` template before you start.