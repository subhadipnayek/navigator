# NAVigator Project Structure 🧭

```
NAVigator/
├── .github/
│   └── workflows/
│       └── ci.yml                      # GitHub Actions CI pipeline (Go tests + Angular build)
├── go.work                          # Root Go workspace file
├── navigator-api/                   # Go 1.26 Backend Microservice
│   ├── cmd/server/                 # Application entry point (main.go)
│   ├── internal/
│   │   ├── config/                 # Environment & configuration loader (Postgres/SQLite dialect resolution)
│   │   │   └── config_test.go      # Config resolution unit tests
│   │   ├── database/               # Dual PostgreSQL 17 & SQLite GORM connection & migration
│   │   │   └── db_test.go          # SQLite WAL, schema migration, and partial index tests
│   │   ├── handlers/               # Gin HTTP route handlers
│   │   ├── middleware/             # JWT auth & CORS middleware
│   │   ├── models/                 # GORM entities & API DTO contracts
│   │   ├── services/               # Core business logic (XIRR, AMFI scraper, Simulations)
│   │   │   ├── auth_service_test.go         # PBKDF2 & JWT token unit tests
│   │   │   ├── calculation_rules_test.go    # Date parsing, binary search & CAGR tests
│   │   │   ├── nav_service_test.go          # NAV history DTO & point-to-point returns tests
│   │   │   ├── simulation_service_test.go   # SIP generator, rebalance & risk metric tests
│   │   │   ├── xirr_calculator_test.go      # XIRR bisection & cash flow precision tests
│   │   │   └── importer/           # Extensible SOLID multi-broker parser & deduplication engine
│   │   └── worker/                 # Background AMFI daily NAV sync worker
│   ├── Dockerfile                  # Lightweight multi-stage Docker build (~20MB)
│   ├── go.mod
│   ├── go.sum
│   └── README.md                   # API service route & testing documentation
│
├── navigator-ui/                    # Angular 22 Frontend Client
│   ├── src/
│   │   ├── app/
│   │   │   ├── core/               # Auth guards, HTTP interceptors, API config
│   │   │   ├── models/             # TypeScript DTO models
│   │   │   ├── pages/              # Workspaces (Dashboard, Research Hub [Screener, Compare, Lab], Ledger)
│   │   │   ├── services/           # HTTP services & state stores (ResearchTrayService)
│   │   │   └── shared/             # Lightweight charts, utilities
│   │   ├── index.html              # Inter & JetBrains Mono font definitions
│   │   └── styles.scss             # High-density Material design system (density -3)
│   ├── Dockerfile                  # Multi-stage Nginx Alpine build
│   ├── nginx.conf                  # Production reverse proxy configuration
│   ├── package.json
│   └── README.md                   # UI service & development documentation
│
├── docker-compose.yml              # PostgreSQL 17 Alpine + Go API + Angular UI
├── docker-compose.bat              # Quick startup batch script (Windows)
├── .gitignore                      # Git ignore configurations for Go, Node, Angular
├── AGENTS.md                       # Agent instructions, engineering rules & DoD
├── GEMINI.md                       # Gemini-specific agent instructions
└── README.md                       # Master platform documentation & quickstart
```
