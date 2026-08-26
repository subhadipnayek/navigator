# navigator 🧭

> **Mutual Fund Analytics, Portfolio Simulation & NAV Intelligence Platform**

navigator is a full-stack financial analytics application designed to track mutual fund Net Asset Values (NAV) directly from AMFI (Association of Mutual Funds in India), simulate SIP & lump-sum portfolio investments, analyze rolling returns, handle corporate actions, and manage personalized multi-asset portfolios with automated XIRR calculation.

---

## 🏗 System Architecture

```
                               ┌─────────────────────────┐
                               │   navigator-ui (SPA)    │
                               │   Angular 21 + SCSS     │
                               │      (Port 4000)        │
                               └────────────┬────────────┘
                                            │ HTTP / JSON
                                            ▼
                               ┌─────────────────────────┐
                               │   navigator-api         │
                               │   Go 1.26 + Gin + GORM  │
                               │      (Port 5038)        │
                               └────────────┬────────────┘
                                            │
                     ┌──────────────────────┴──────────────────────┐
                     ▼                                             ▼
       ┌───────────────────────────┐                 ┌───────────────────────────┐
       │   PostgreSQL 17 Alpine    │                 │       AMFI India API      │
       │     (Port 5432)           │                 │    (Daily NAV & History)  │
       └───────────────────────────┘                 └───────────────────────────┘
```

### Key Modules & Capabilities

- **AMFI Integration & Sync Engine**: Automated background sync worker and on-demand synchronization for mutual fund schemes, daily NAVs, and historical data.
- **Portfolio Lab & Simulation**: Backtest SIP and lump-sum investments across custom asset allocation baskets, calculate CAGR/XIRR, drawdowns, and side-by-side fund comparisons.
- **Rolling Returns Engine**: Compute rolling return windows (1Y, 3Y, 5Y) with median, min, max, standard deviation, and quartile breakdowns.
- **User Portfolios & Transaction Ledger**: Track real user portfolios, parse CAMS/KFintech Excel statements, track historical valuations, and compute realized/unrealized PnL.
- **Corporate Actions**: Dividend reinvestments, fund mergers, bonuses, and split tracking.
- **Authentication & RBAC**: Secure JWT-based authentication with PBKDF2 password hashing and Admin sync controls.

---

## 📁 Project Structure

```
NAVigator/
├── navigator-api/                   # Go 1.26 Backend Microservice
│   ├── cmd/server/                 # Entry point (main.go)
│   ├── internal/
│   │   ├── config/                 # Environment & config loader
│   │   ├── database/               # PostgreSQL GORM connection & auto-migrations
│   │   ├── handlers/               # Gin REST route handlers
│   │   ├── middleware/             # JWT auth & CORS middleware
│   │   ├── models/                 # Database entities & DTO contracts
│   │   ├── services/               # Core analytics (XIRR, AMFI scraper, Simulations)
│   │   └── worker/                 # Background AMFI daily NAV sync worker
│   ├── Dockerfile                  # Multi-stage lightweight build (~20MB)
│   ├── go.mod
│   └── go.sum
│
├── navigator-ui/                    # Frontend Client
│   ├── src/app/
│   │   ├── core/                   # Auth guards, HTTP interceptors, API config
│   │   ├── models/                 # TypeScript interfaces & DTOs
│   │   ├── pages/                  # Route views (Dashboard, Portfolio Lab, Fund Detail, etc.)
│   │   ├── services/               # Angular HTTP services
│   │   └── shared/                 # Reusable components, charts, UI controls
│   ├── Dockerfile                  # Multi-stage build (Node Alpine -> Nginx Alpine)
│   ├── nginx.conf                  # Production reverse proxy config
│   └── package.json
│
├── docker-compose.yml              # Complete multi-container orchestration stack
├── docker-compose.bat              # Quick startup batch script (Windows)
└── README.md
```

---

## 🚀 Quick Start (Docker Compose)

The fastest way to launch the entire stack (PostgreSQL 17 Alpine + Go API + Angular UI) is with Docker Compose:

### 1. Prerequisites

- [Docker Desktop](https://www.docker.com/products/docker-desktop/) (ensure Docker daemon is running)

### 2. Start the Stack

```bash
# Clone the repository
git clone https://github.com/subhadipnayek/navigator.git
cd navigator

# Spin up all containers
docker compose up -d --build
```

_(On Windows, you can also double-click `docker-compose.bat`)_

### 3. Service Endpoints

| Service        | URL                                            | Default Credentials / Note                                     |
| -------------- | ---------------------------------------------- | -------------------------------------------------------------- |
| **Web UI**     | [http://localhost:4000](http://localhost:4000) | Frontend Client Application                                    |
| **API**        | [http://localhost:5038](http://localhost:5038) | REST API & `/health` endpoint                                  |
| **PostgreSQL** | `localhost:5433` (Host) / `5432` (Docker) | User: `devuser`, Password: `devpassword`, DB: `NAVigatorDb_QA` |

---

## 💻 Local Development Setup

### 1. Database (PostgreSQL)

Run a lightweight PostgreSQL instance:

```bash
docker run -d \
  --name navigator-postgres \
  -e POSTGRES_DB=NAVigatorDb_QA \
  -e POSTGRES_USER=devuser \
  -e POSTGRES_PASSWORD=devpassword \
  -p 5433:5432 \
  postgres:17-alpine
```

### 2. Backend (`navigator-api`)

```bash
cd navigator-api

# Download dependencies
go mod download

# Run the API
go run cmd/server/main.go
```

API runs on `http://localhost:8080` (or `http://localhost:5038` via docker compose).

### 3. Frontend (`navigator-ui`)

```bash
cd navigator-ui

# Install dependencies
npm install

# Start the Angular dev server
npm start
```

Navigate to `http://localhost:4200/`. The app will automatically reload on code changes.

---

## 📡 API Reference Overview

| Endpoint Group           | Description                                                                      |
| ------------------------ | -------------------------------------------------------------------------------- |
| `/api/auth`              | User registration, login, and JWT token issuance                                 |
| `/api/funds`             | Search mutual funds, list fund categories, view fund metadata                    |
| `/api/nav`               | Fetch current & historical NAVs, rolling return analytics                        |
| `/api/portfolio`         | Run SIP/Lump-sum simulation backtests, benchmark comparisons                     |
| `/api/user-portfolios`   | Manage personal portfolios, upload transaction Excel files, view holdings & XIRR |
| `/api/corporate-actions` | Query and track corporate actions (dividends, splits, mergers)                   |
| `/api/admin`             | Trigger AMFI sync, manage third-party API configurations & live health checks    |

---

## 📄 License

This project is licensed under the terms of the MIT license.
