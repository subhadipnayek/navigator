# NAVigator Agent Instructions & Engineering Rules

## 📌 Project Overview
NAVigator (🧭) is a high-performance mutual fund analytics, NAV intelligence, and portfolio backtesting terminal built on a decoupled polyglot architecture:
- **`navigator-ui`**: Angular 21 (Zoneless signals, Angular Material density -3, Lightweight Charts, AG Grid, tabular JetBrains Mono / Inter typography, Port `4000` / `4200` dev).
- **`navigator-api`**: Go 1.26 (Gin REST framework, GORM ORM, PostgreSQL driver, Excelize spreadsheet engine, PBKDF2 password hashing, JWT auth, XIRR solver, AMFI scraper & sync worker, Port `5038` / `8080` dev).
- **`PostgreSQL 17 Alpine`**: Relational persistence on Port `5432` (`5433` on host in Docker Compose), storing fund master metadata, 30+ year daily NAV histories, user portfolios, transactions, corporate actions, and audit logs.
- **`AMFI Integration & Sync Worker`**: Automated background worker and on-demand scraper fetching live and historical NAV data directly from the Association of Mutual Funds in India (AMFI).

---

## 🚨 MANDATORY RULE: Continuous Documentation & README Synchronization

> **CRITICAL INSTRUCTION**: Whenever you make **ANY** change to the codebase (such as updating endpoints, ports, models, financial calculation algorithms, database schemas, dependencies, Docker configurations, or service logic), you **MUST** immediately update the relevant documentation in [`README.md`](./README.md), [`project_structure.md`](./project_structure.md), and corresponding service `README.md` files.

### Documentation Maintenance Checklist
Every code modification must be reflected across corresponding documentation:
1. **Root [`README.md`](./README.md)**: Keep architecture diagrams, port mappings, service registry, API reference tables, and quick-start commands accurate.
2. **[`project_structure.md`](./project_structure.md)**: Master structure document covering backend and frontend directories, models, services, and route mappings.
3. **Service `README.md` Files**: Keep [`navigator-ui/README.md`](./navigator-ui/README.md) and [`navigator-api/README.md`](./navigator-api/README.md) synchronized with local route definitions, endpoints, and testing instructions.

---

## 📋 MANDATORY RULE: GitHub-First & User-Approval Agile Workflow

### 1. Single Source of Truth: GitHub Issues & Milestones
- All backlogs, active milestones, and task statuses are tracked **directly in GitHub** ([`github.com/subhadipnayek/navigator`](https://github.com/subhadipnayek/navigator)) via the GitHub CLI (`gh`).
- Each milestone can contain a maximum of **6 total tasks** (regardless of category: Features `FEAT`, Bug Fixes `BUG`, Performance `PERF`, Security `SEC`, Architecture `ARCH`, or Testing `TEST`).
- When an active milestone reaches 6 tasks, create a new milestone (e.g., `Milestone N+1`).

### 2. Standardized Task Proposal & User Approval
- **Always verify/create the GitHub Issue before writing code**: Whenever you are about to implement a new feature, fix a bug, optimize performance, or make architectural changes, identify or create the GitHub Issue (`gh issue create`).
- **Do NOT start implementing code immediately**: You **MUST** first present the proposed plan to the user using the standardized format:
  - **Issue #, Task ID & Title**: (e.g., `Issue #1: FEAT-01: Multi-Fund CAGR Alpha Matrix & Rolling Spread Analyzer`)
  - **Target Milestone**: (e.g., `Milestone 1: Core Analytics & Fund Intelligence`)
  - **Affected Components**: (e.g., `navigator-api`, `navigator-ui`, `database`)
  - **Acceptance Criteria**: Concrete Given-When-Then behavioral expectations.
  - **Execution Plan**: Step-by-step implementation summary.
- Wait for the user's explicit confirmation/agreement before writing or editing any codebase files.

### 3. Definition of Done (DoD) & Milestone Release Gate
- A task is ONLY marked as completed when:
  1. All target service test suites pass cleanly.
  2. Production bundle builds succeed (`npm run build`, `go build ./...`, etc.).
  3. All documentation in [`README.md`](./README.md), [`project_structure.md`](./project_structure.md), and service READMEs is updated.
  4. The GitHub Issue is directly closed via `gh issue close <issue_number>`.
- When an active milestone reaches **6/6 tasks completed**:
  - Run the full verification matrix across both backend (`navigator-api`) and frontend (`navigator-ui`).
  - Close the milestone in GitHub via `gh api`.

---

## 🧪 Testing & Quality Assurance
- Never consider a task complete without running and verifying test suites across modified services:
  - **Angular Frontend (`navigator-ui`)**: `npm run build` and `npm test -- --watch=false`
  - **Go Backend API (`navigator-api`)**: `go test ./...` and `go build -v ./...`
  - **Docker Compose Stack**: `docker compose build` / `docker compose up -d`
