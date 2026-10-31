# navigator 🧭

> **Mutual Fund Analytics, Portfolio Simulation & NAV Intelligence Platform**

navigator is a full-stack financial analytics application designed to track mutual fund Net Asset Values (NAV) directly from AMFI (Association of Mutual Funds in India), simulate SIP & lump-sum portfolio investments, analyze rolling returns, handle corporate actions, and manage personalized multi-asset portfolios with automated XIRR calculation.

---

## 🏗 System Architecture

```
                               ┌─────────────────────────┐
                               │   navigator-ui (SPA)    │
                               │   Angular 22 + SCSS     │
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
       │   Dual Database Engine    │                 │       AMFI India API      │
       │ PostgreSQL 17 (Container) │                 │    (Daily NAV & History)  │
       │   OR SQLite (Embedded)    │                 └───────────────────────────┘
       └───────────────────────────┘
```

### Key Modules & Capabilities

- **AMFI Integration & Modern NAV Timeseries Engine**: Automated background sync worker, real-time lazy-loaded historical backfills backwards from current date with early inception detection, aligned `NavDataResponse` contracts (`dd-MM-yyyy` date formatting, metadata ribbons, corporate actions), and multi-chunk sliding window retrieval ($\le 4$ years per chunk) for full 30+ year daily NAV histories from `amfiindia.com/api/nav-history`.
- **Dual-Database Engine (PostgreSQL & SQLite)**: Seamlessly toggle between enterprise containerized PostgreSQL 17 with `pg_trgm` GIN indexing and zero-config embedded SQLite with WAL concurrency via environment settings (`DB_TYPE=sqlite` or `DB_TYPE=postgres`).
- **Portfolio Lab & Simulation**: Backtest SIP and lump-sum investments across custom asset allocation baskets, calculate CAGR/XIRR, drawdowns, and side-by-side fund comparisons.
- **Rolling Returns Engine**: Compute rolling return windows (1Y, 3Y, 5Y) with median, min, max, standard deviation, and quartile breakdowns.
- **User Portfolios & Transaction Ledger**: Track real user portfolios, parse CAMS/KFintech Excel statements, track historical valuations, and compute realized/unrealized PnL.
- **Corporate Actions**: Dividend reinvestments, fund mergers, bonuses, and split tracking.
- **Authentication & RBAC**: Secure JWT-based authentication with PBKDF2 password hashing and Admin sync controls.

---

## 📁 Project Structure

```
NAVigator/
├── go.work                          # Go multi-module workspace root
├── navigator-api/                   # Go 1.26 Backend Microservice
│   ├── cmd/server/                 # Entry point (main.go)
│   ├── internal/
│   │   ├── config/                 # Environment & config loader (dual DB dialect resolution)
│   │   ├── database/               # PostgreSQL & SQLite GORM connection & auto-migrations
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

### Option A: Self-Contained SQLite (Zero Docker & Zero Setup)
Run the backend completely self-contained with embedded SQLite (no Docker, PostgreSQL, or external SQLite installation required):

```bash
# Directly from the repository root:
go run ./navigator-api/cmd/server

# Or from inside navigator-api:
cd navigator-api
go run cmd/server/main.go
```
The API automatically creates and manages `navigator.db` in high-concurrency WAL mode on Port `8080`.

### Option B: PostgreSQL (Containerized)

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

Then start the API with PostgreSQL:
```bash
export DB_TYPE=postgres
go run ./navigator-api/cmd/server
```

API runs on `http://localhost:8080` (or `http://localhost:5038` via docker compose).

### Frontend (`navigator-ui`)

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

## 🧪 Testing & CI/CD Pipeline

The project includes an automated GitHub Actions CI pipeline (`.github/workflows/ci.yml`) and comprehensive unit testing across financial calculation engines, multi-broker parsers, and authentication:

```bash
# Run backend test suite (including SQLite WAL & config dialect tests)
go test -v ./navigator-api/...

# Verify Go API binary build
go build -v ./navigator-api/cmd/server
```

---

## 📄 License

This project is licensed under the terms of the MIT license.
