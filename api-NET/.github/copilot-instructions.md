# MF/ETF Comparison App — Backend Context (.NET 10)

## Overview
A .NET 10 Web API that syncs Indian Mutual Fund/ETF master data daily, serves 
fund listings, fetches NAV history on demand, computes split-adjusted returns 
(point-to-point and rolling), and exposes corporate action management APIs.
The Angular frontend consumes this API — never calls external data sources directly.
**Comparison is UI-driven** — the backend serves individual fund data; the frontend
calls endpoints in parallel and composes comparison views client-side.

## Tech Stack
- .NET 10 Web API (Controllers)
- C# 14.0
- SQL Server (EF Core for ORM)
- Polly (HTTP retry/rate-limiting for external API calls)
- BackgroundService for scheduled sync

## Project Structure
```
Controllers/
  FundsController.cs          — Fund CRUD + search + sync (5 endpoints)
  NavController.cs             — NAV history, returns, rolling returns (3 endpoints)
  CorporateActionsController.cs — Anomaly detection + action CRUD (4 endpoints)
Models/
  Database/
    Fund.cs                    — Master fund entity (SchemeCode PK)
    NavHistory.cs              — Daily NAV entries (SchemeCode + Date unique)
    NavSyncTracker.cs          — Per-fund API cooldown tracker
    CorporateAction.cs         — Split/merger/bonus records with adjustment factors
  Dto/
    NavDataResponse.cs         — NAV history response (with from/to/interval filtering)
    ReturnsResponse.cs         — Point-to-point returns (split-adjusted)
    RollingReturnResponse.cs   — Rolling returns with summary stats
    NavAnomalyDto.cs           — Detected NAV anomalies
    CorporateActionDto.cs      — Corporate action response
    CreateCorporateActionRequest.cs — Corporate action creation input
    ...
Repositories/
    IFundRepository / FundRepository
    INavHistoryRepository / NavHistoryRepository
    ICorporateActionRepository / CorporateActionRepository
Services/
    IAmfiService / AmfiService           — External API calls (AMFI + mfapi.in)
    IFundService / FundService           — Fund business logic + sync
    INavService / NavService             — NAV caching, returns, rolling returns
    ICorporateActionService / CorporateActionService — Anomaly detection, action CRUD
Data/
    AppDbContext.cs             — EF Core context with 4 DbSets
```

## API Endpoints (12 total)

### Funds — `/api/funds`
| # | Method | Route | Purpose |
|---|--------|-------|---------|
| 1 | GET | `/api/funds` | All funds (~20K records) |
| 2 | GET | `/api/funds/{schemeCode}` | Single fund |
| 3 | GET | `/api/funds/search?q=` | Search by name/house/code |
| 4 | POST | `/api/funds/sync` | Manual AMFI sync |
| 5 | GET | `/api/funds/count` | Fund count |

### NAV — `/api/nav`
| # | Method | Route | Purpose |
|---|--------|-------|---------|
| 6 | GET | `/api/nav/{schemeCode}?from=&to=&interval=&adjusted=` | NAV history (filterable, samplable, adjustable) |
| 7 | GET | `/api/nav/{schemeCode}/returns?startDate=&endDate=` | Point-to-point returns (split-adjusted) |
| 8 | GET | `/api/nav/{schemeCode}/rolling-returns?periodInYears=&from=&to=&interval=` | Rolling returns (filterable, samplable, split-adjusted) |

### Corporate Actions — `/api/corporateactions`
| # | Method | Route | Purpose |
|---|--------|-------|---------|
| 9 | GET | `/api/corporateactions/anomalies?threshold=` | Detect NAV anomalies |
| 10 | GET | `/api/corporateactions/{schemeCode}` | List corporate actions |
| 11 | POST | `/api/corporateactions/{schemeCode}` | Add corporate action |
| 12 | DELETE | `/api/corporateactions/{schemeCode}/{actionId}` | Delete corporate action |

## Key Architecture Decisions

### NAV Data Caching (Smart On-Demand)
- NAV history is fetched from `api.mfapi.in` on first access, then cached in SQL Server.
- Per-scheme `SemaphoreSlim` locks prevent concurrent API fetches for the same fund.
- `NavSyncTracker` enforces a 1-hour cooldown after failed/no-new-data API checks.
- **Cooldown is bypassed when DB has zero data** for a fund — always retries the API.
- If the latest NAV in DB matches today's date, serves from DB without any API call.

### Interval Sampling
- `interval` param on NAV history and rolling returns: `daily` (default), `weekly`, `monthly`.
- `weekly` = last trading day per ISO week (`ISOWeek`), `monthly` = last per calendar month.
- Generic `SampleByInterval<T>` helper handles both.
- For rolling returns, summary stats are computed on the filtered range before sampling.

### Date Range Behavior
- **NAV history**: `from`/`to` filter data; `latestNavDate` is always the overall latest (unaffected).
- **Returns**: `startDate`/`endDate` required. Closest NAV within 7 days is used. If dates exceed
  available range, clamps to earliest/latest available NAV (returns max data, not null).
- **Rolling returns**: optional `from`/`to` filter the output data points. Stats computed on filtered set.

### Corporate Actions (Split/Merger/Bonus Adjustment)
- `CorporateAction` entity stores: ActionDate, ActionType, AdjustmentFactor, OldFaceValue, NewFaceValue.
- **AdjustmentFactor**: For a 10:1 split (FV 10→1), factor = 10. Pre-split NAVs are divided by this.
- **Returns calculation**: `startNav` is adjusted by cumulative factor for all actions between start and end dates.
- **Rolling returns**: Pre-loads all actions for the fund, applies `ComputeAdjustmentFactor` per window.
- **NAV history endpoint**: Supports opt-in `adjusted=true` query param. When true, all pre-action NAVs
  are divided by the cumulative factor from that date to the latest NAV date, producing a continuous line.
  Default (`adjusted=false`) returns raw/unadjusted NAVs.
- **`NavDataResponse`** always includes `corporateActions` (list of `CorporateActionDto`) and `isAdjusted` flag.
- **Anomaly detection**: Scans all NAV history for day-over-day changes exceeding a threshold (default 30%).
  Flags whether each anomaly already has a recorded corporate action.

## External Data Sources

### 1. Fund Master List
- **URL**: https://portal.amfiindia.com/spages/NAVAll.txt
- **Format**: Semicolon-delimited plain text, grouped by sections
- **Sync**: Daily via BackgroundService + manual trigger endpoint
- **Parsing rules**:
  - Blank lines separate sections
  - Line matching `Open Ended Schemes(...)` or `Close Ended Schemes(...)` = header
    → Extract SchemeType = "Open Ended Schemes", SchemeCategory = "Debt Scheme - Banking and PSU Fund"
  - Standalone text line (no semicolons) after a blank line = Fund House name
  - Line with semicolons = fund data row, split by ";"
  - Columns: SchemeCode;ISIN Div Payout/ISIN Growth;ISIN Div Reinvestment;Scheme Name;Net Asset Value;Date
  - "-" means null/not available
  - ~20,000+ fund entries

### 2. NAV History (per fund)
- **URL**: https://api.mfapi.in/mf/{scheme_code}
- **Format**: JSON
- **Response**:
  ```json
  {
    "meta": {
      "fund_house": "string",
      "scheme_type": "string",
      "scheme_category": "string",
      "scheme_code": 140085,
      "scheme_name": "string",
      "isin_growth": "string",
      "isin_div_reinvestment": "string|null"
    },
    "data": [
      { "date": "01-04-2026", "nav": "667.16510" },
      { "date": "31-03-2026", "nav": "650.35830" }
    ]
  }