# navigator-api 🧭

> **Go 1.26 Backend Microservice for NAVigator Mutual Fund Analytics**

`navigator-api` is the core high-performance financial analytics and data ingestion engine for NAVigator. It handles direct AMFI NAV synchronization, historical scraping, portfolio backtesting, XIRR calculations, Excel trade ledger parsing, and user authentication.

---

## 🛠 Tech Stack

- **Language**: Go 1.26
- **Web Framework**: [Gin Web Framework](https://github.com/gin-gonic/gin)
- **ORM / Database**: [GORM](https://gorm.io/) with PostgreSQL 17 driver
- **Authentication**: JWT (`golang-jwt/jwt/v5`) with PBKDF2 password hashing (`golang.org/x/crypto`)
- **Excel Engine**: [Excelize](https://github.com/qax-os/excelize) for CAMS & KFintech statement parsing
- **Port**: `5038` (Docker) / `8080` (Local default)

---

## 📁 Directory Structure

```
navigator-api/
├── cmd/
│   └── server/
│       └── main.go              # Application entry point & router initialization
├── internal/
│   ├── config/                  # Environment variable configuration
│   ├── database/                # PostgreSQL GORM connection & migrations
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
- `GET /api/nav/:code`: Fetch NAV historical points (with interval & split-adjustment options).
- `GET /api/nav/:code/returns`: Calculate point-to-point CAGR and absolute returns.
- `GET /api/nav/:code/rolling-returns`: Compute rolling return series and statistics.
- `GET /api/nav/:code/gaps`: Detect missing NAV dates.
- `POST /api/nav/:code/repair-gaps`: Automatically backfill verified data gaps.

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

---

## 🧪 Testing & Building

```bash
# Run unit & integration test suites
go test ./...

# Verify production binary compilation
go build -v ./...
```
