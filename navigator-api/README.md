# navigator-api 🧭

> **Go 1.26 Backend Microservice for NAVigator Mutual Fund Analytics**

`navigator-api` is the core high-performance financial analytics and data ingestion engine for NAVigator. It handles direct AMFI NAV synchronization, historical scraping, portfolio backtesting, XIRR calculations, Excel trade ledger parsing, user authentication, and dual database persistence (SQLite out-of-the-box for zero-setup local runs, and PostgreSQL 17 for containerized production).

---

## 🛠 Tech Stack

- **Language**: Go 1.26
- **Web Framework**: [Gin Web Framework](https://github.com/gin-gonic/gin)
- **ORM / Database**: [GORM](https://gorm.io/) with dual-database engine:
  - **SQLite**: `github.com/glebarez/sqlite` (Pure Go, CGO-free, embedded — default for local runs with WAL mode).
  - **PostgreSQL 17**: `gorm.io/driver/postgres` with `pg_trgm` GIN indexing for production environments.
- **Authentication**: JWT (`golang-jwt/jwt/v5`) with PBKDF2 password hashing (`golang.org/x/crypto`)
- **Excel Engine**: [Excelize](https://github.com/qax-os/excelize) for CAMS & KFintech statement parsing
- **Port**: `5038` (Docker) / `8080` (Local default)

---

## ⚙️ Environment Configuration

| Variable | Default | Description |
|---|---|---|
| `PORT` | `8080` | HTTP port on which the Gin server listens. |
| `DB_TYPE` | `sqlite` | Database dialect to use: `sqlite` (default for local runs) or `postgres` (used in Docker Compose). |
| `DATABASE_URL` | `navigator.db` | SQLite file path OR PostgreSQL connection string. |
| `DATABASE_PATH` | `""` | Dedicated path for SQLite database file (e.g., `./data/navigator.db`). |
| `JWT_KEY` | `dev-only-...` | Secret key used for signing JWT authentication tokens. |
| `JWT_EXPIRES_HOURS` | `8` | Expiry duration for authentication tokens in hours. |
| `CORS_ORIGINS` | `http://localhost:4000,...` | Comma-separated list of allowed CORS browser origins. |

### Running Zero-Config with SQLite (Zero Docker Required)
No Docker, PostgreSQL, or external SQLite installation needed:
```bash
# Directly from repository root:
go run ./navigator-api/cmd/server

# Or from inside navigator-api:
cd navigator-api
go run cmd/server/main.go
```

---

## 📁 Directory Structure

```
navigator-api/
├── cmd/
│   └── server/
│       └── main.go              # Application entry point & router initialization
├── internal/
│   ├── config/                  # Environment variable configuration & dialect resolver
│   │   └── config_test.go       # Config resolution & database type tests
│   ├── database/                # Dual PostgreSQL/SQLite GORM connection & migrations
│   │   └── db_test.go           # SQLite WAL, indexes, and schema verification tests
│   ├── handlers/                # REST HTTP route controllers
│   ├── middleware/              # Auth & CORS middleware
│   ├── models/                  # GORM database entities & API contracts
│   ├── services/                # Analytics engine (XIRR, AMFI scraper, CAGR)
│   │   └── importer/            # Multi-broker parser (CAMS, KFintech, Zerodha), fuzzy asset resolver & deduplication
│   └── worker/                  # Background daily AMFI NAV sync worker
├── Dockerfile                   # Multi-stage lightweight Alpine container (~20MB)
├── go.mod
└── go.sum
```

---

## 📡 REST API Routes

### Authentication (`/api/auth`)
- `POST /api/auth/register`: Create a new user account (automatically promotes the first registered user to `Admin`/root).
- `POST /api/auth/login`: Authenticate and receive JWT access token.
- `GET /api/auth/me`: Get current authenticated user profile and role (`Admin` or `User`).
- `GET /api/auth/users`: List registered users (Admin only).
- `PUT /api/auth/users/:userId/role`: Modify user role permissions (Admin only).

### Funds (`/api/funds`)
- `GET /api/funds`: List mutual fund schemes (paginated).
- `GET /api/funds/search`: Search schemes by name, AMC, category, or AMFI code (`?q=...`).
- `GET /api/funds/:code`: Get scheme metadata and factsheet details.
- `GET /api/funds/count`: Total fund schemes count in database.
- `POST /api/funds/sync`: Trigger on-demand sync with AMFI master.

### NAV History & Returns (`/api/nav`)
- `GET /api/nav/:code`: Fetch NAV historical points with standardized `NavDataResponse` schema (`latestNavDate`, `navStartDate`, `navEndDate`, `corporateActions`, `isAdjusted`, `wasFetchedFromApi`, `data` with `dd-MM-yyyy` string formatting). Powered by a smart delta range resolver (0 HTTP calls on local DB hit), parallel multi-chunk 5-year interval ingestion (bounded 3-worker concurrency), and a 10-day missing NAV inception cutoff.
- `GET /api/nav/:code/returns`: Calculate point-to-point CAGR and absolute returns with automatic historical delta lazy-loading fallback (`absoluteReturn`, `absoluteReturnPercent`, `cagr`, `startDate`, `endDate`, `startNav`, `endNav`).
- `GET /api/nav/:code/rolling-returns`: Compute rolling return series and statistics (with on-demand smart delta lazy-loading fallback).
- `GET /api/nav/:code/gaps`: Detect missing NAV dates.
- `POST /api/nav/:code/repair-gaps`: Automatically backfill verified data gaps via parallel multi-chunk AMFI time-series queries.

### Portfolio Backtesting & Lab (`/api/portfolio`)
- `POST /api/portfolio/simulate`: Simulate custom multi-asset basket with SIP/Lump-sum.
- `POST /api/portfolio/compare`: Multi-basket comparison backtest.
- `POST /api/portfolio/what-if`: Capital efficiency and allocation what-if scenarios.
- `POST /api/portfolio/rolling`: Multi-basket rolling return comparison.

### User Portfolios & Ledger (`/api/user-portfolios`)
- `GET /api/user-portfolios`: List authenticated user's portfolios.
- `POST /api/user-portfolios`: Create new portfolio.
- `GET /api/user-portfolios/:id/dashboard`: Holdings valuation, allocation breakdown, XIRR, and SIP consistency.
- `GET /api/user-portfolios/:id/transactions`: List portfolio transactions with financial year filtering.
- `PUT /api/user-portfolios/:id/transactions`: Batch update transaction ledger.
- `POST /api/user-portfolios/:id/transactions/import-preview`: Multi-broker dry-run statement parser & deduplication preview (CAMS, KFintech, Zerodha, CSV).
- `POST /api/user-portfolios/:id/transactions/import-commit`: Atomic ingestion commit with 4 strategies (Smart Merge, Append, Window Replace, Overwrite).
- `POST /api/user-portfolios/:id/benchmark`: Compare portfolio performance against market proxy.

### Corporate Actions (`/api/corporateactions`)
- `GET /api/corporateactions/anomalies`: Detect unit-split anomalies and sudden NAV jumps.
- `GET /api/corporateactions/review-queue`: Admin review queue for corporate action candidates.
- `POST /api/corporateactions/anomalies/auto-record`: Auto-record high-confidence split events.
- `GET /api/corporateactions/:code`: List recorded actions for a fund.
- `POST /api/corporateactions/:code`: Create a corporate action override.

### Admin Operations (`/api/admin`)
- `GET /api/admin/funds/summary`: Database health, total records, and coverage statistics.
- `GET /api/admin/funds`: Paginated fund maintenance table.
- `POST /api/admin/funds/sync`: Master sync trigger.
- `POST /api/admin/funds/:code/nav-history/sync`: Scrape full 30+ year NAV history for a single fund.
- `GET /api/admin/integrations`: List upstream API configurations and live health metrics.
- `PUT /api/admin/integrations/:key`: Update URL, HTTP method, headers, and payload template.
- `POST /api/admin/integrations/:key/test`: Live test / diagnostic health check of target endpoint.
- `POST /api/admin/integrations/:key/reset`: Restore factory default endpoint URL.
- `POST /api/admin/integrations/health-all`: Batch health check all upstream integrations.

---

## 🧪 Testing

```bash
# Run unit tests across all packages
go test -v ./navigator-api/...

# Run database engine tests
go test -v ./navigator-api/internal/database

# Verify production binary compilation
go build -v ./navigator-api/cmd/server
```
