NAVigator/
├── navigator-api/                   # Go 1.26 Backend Microservice
│   ├── cmd/server/                 # Application entry point (main.go)
│   ├── internal/
│   │   ├── config/                 # Environment & configuration loader
│   │   ├── database/               # PostgreSQL GORM connection & migration
│   │   ├── handlers/               # Gin HTTP route handlers
│   │   ├── middleware/             # JWT auth & CORS middleware
│   │   ├── models/                 # GORM entities & API DTO contracts
│   │   ├── services/               # Core business logic (XIRR, AMFI, Simulations)
│   │   └── worker/                 # Background AMFI daily NAV sync worker
│   ├── Dockerfile                  # Lightweight multi-stage Docker build (~20MB)
│   ├── go.mod
│   └── go.sum
├── navigator-ui/                    # Angular 19 Client SPA
│   ├── src/app/
│   ├── Dockerfile                  # Multi-stage Nginx Alpine build
│   ├── nginx.conf
│   └── package.json
├── docker-compose.yml              # PostgreSQL 17 Alpine + Go API + Angular UI
├── docker-compose.bat
└── README.md
