# navigator-ui 🧭

> **Angular 21 Frontend Client for NAVigator Mutual Fund Analytics Terminal**

`navigator-ui` is a high-density, real-time financial workstation and factsheet client built for mutual fund screening, multi-fund alpha comparisons, portfolio backtesting, and transaction ledger management.

---

## 🛠 Tech Stack

- **Framework**: Angular 21 (Zoneless signals, standalone components)
- **UI Components**: Angular Material 21 (configured at density `-3` for maximum information density)
- **Data Visualization**: [Lightweight Charts](https://tradingview.github.io/lightweight-charts/) (TradingView)
- **Data Tables**: [AG Grid Community](https://www.ag-grid.com/) with Quartz theme
- **Typography**: [Inter](https://fonts.google.com/specimen/Inter) & [JetBrains Mono](https://fonts.google.com/specimen/JetBrains+Mono) with tabular numeric alignment
- **Spreadsheets**: XLSX / Excelize data exchange

---

## 📁 Directory Structure

```
navigator-ui/
├── src/
│   ├── app/
│   │   ├── core/                 # Auth guard, HTTP interceptor, API config
│   │   ├── models/               # TypeScript interfaces & DTOs
│   │   ├── pages/                # Route components
│   │   │   ├── dashboard/        # Command terminal hub & instant lookup
│   │   │   ├── fund-list/        # High-density fund screener & table
│   │   │   ├── fund-detail/      # Factsheet & NAV history workbench
│   │   │   ├── compare/          # Multi-fund CAGR & alpha comparator
│   │   │   ├── portfolio-lab/    # SIP & lump-sum basket simulation lab
│   │   │   ├── user-portfolio/   # Personal ledger, AG Grid, & XIRR engine
│   │   │   ├── rolling-returns/  # Statistical rolling return distribution
│   │   │   ├── corporate-actions/# Anomaly detection & data hygiene queue
│   │   │   ├── admin-sync/       # Master sync & fund operations console
│   │   │   ├── admin-integrations/# Upstream third-party API control center & health checker
│   │   │   ├── landing/          # Institutional welcome portal
│   │   │   └── login/register/   # Terminal authentication views
│   │   ├── services/             # Angular HTTP services
│   │   └── shared/               # Line charts, stat cards, date utilities
│   ├── index.html                # Inter & JetBrains Mono font imports
│   ├── styles.scss               # Global high-density design system
│   └── main.ts
├── angular.json
├── package.json
└── nginx.conf                    # Production reverse proxy configuration
```

---

## 🚀 Development & Build Commands

```bash
# Install dependencies
npm install

# Start development server (http://localhost:4200)
npm start

# Run unit tests
npm test -- --watch=false

# Compile production bundle (dist/NAVigator)
npm run build
```
