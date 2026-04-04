# NAVigator — Angular Frontend Copilot Instructions

## Overview
Angular frontend for the NAVigator MF/ETF comparison app.
**All business logic, calculations, and external API calls happen on the .NET backend.**
Angular is purely for display, user interaction, and consuming the REST API below.
Never call any external data source directly from Angular.

## Backend API Base URL
```
Development: http://localhost:5038/api
```
CORS is already configured on the backend for `http://localhost:4200`.

---

## TypeScript Interfaces

All dates from the API are strings in `"dd-MM-yyyy"` format (e.g. `"02-04-2026"`)
**except** `latestNavDate` in `Fund` which uses `"dd-MMM-yyyy"` (e.g. `"02-Apr-2026"`).

```ts
// ==================== FUND MODELS ====================

export interface Fund {
  schemeCode: number;
  isinGrowth: string | null;
  isinDivReinvestment: string | null;
  schemeName: string;
  latestNav: number | null;
  latestNavDate: string | null;       // "02-Apr-2026" (dd-MMM-yyyy)
  fundHouse: string | null;
  schemeType: string | null;          // "Open Ended Schemes"
  schemeCategory: string | null;      // "Debt Scheme - Banking and PSU Fund"
}

export interface FundSyncResult {
  totalParsed: number;
  added: number;
  updated: number;
  unchanged: number;
  duration: string;                   // TimeSpan string e.g. "00:00:12.3456789"
}

export interface FundCount {
  count: number;
}

// ==================== NAV HISTORY MODELS ====================

export interface NavHistoryEntry {
  date: string;                       // "dd-MM-yyyy"
  nav: number;
}

export interface NavDataResponse {
  schemeCode: number;
  schemeName: string;
  fundHouse: string | null;
  latestNavDate: string | null;       // "dd-MM-yyyy"
  wasFetchedFromApi: boolean;         // true = fresh data was fetched from external API this request
  isAdjusted: boolean;                // true = NAVs are split-adjusted (adjusted=true was passed)
  data: NavHistoryEntry[];            // sorted newest-first
  corporateActions: CorporateActionDto[]; // all corporate actions for this fund (always included)
}

// ==================== RETURNS MODELS ====================

export interface ReturnsResponse {
  schemeCode: number;
  schemeName: string;
  fundHouse: string | null;
  absoluteReturn: number | null;      // percentage e.g. 12.45 means 12.45%
  cagr: number | null;               // annualized % e.g. 11.82
  startDate: string | null;          // "dd-MM-yyyy"
  startNav: number | null;
  endDate: string | null;            // "dd-MM-yyyy"
  endNav: number | null;
}

// ==================== ROLLING RETURNS MODELS ====================

export interface RollingReturnEntry {
  date: string;                       // "dd-MM-yyyy" (end date of the rolling window)
  returnPercentage: number;           // annualized CAGR %
}

export interface RollingReturnResponse {
  schemeCode: number;
  schemeName: string;
  fundHouse: string | null;
  periodInYears: number;
  dataPoints: number;                 // total data points (before interval sampling)
  minReturn: number | null;
  maxReturn: number | null;
  averageReturn: number | null;
  medianReturn: number | null;
  data: RollingReturnEntry[];         // may be sampled if interval was specified
}

// ==================== CORPORATE ACTIONS MODELS ====================

export interface NavAnomalyDto {
  schemeCode: number;
  schemeName: string;
  fundHouse: string | null;
  date: string;                       // "dd-MM-yyyy"
  previousNav: number;
  currentNav: number;
  changePercent: number;              // e.g. -90.0 for a 10:1 split drop
  direction: string;                  // "Drop" or "Spike"
  hasCorporateAction: boolean;        // true if admin already recorded an action for this date
}

export interface CorporateActionDto {
  id: number;
  schemeCode: number;
  actionDate: string;                 // "dd-MM-yyyy"
  actionType: string;                 // "Split", "Merger", "Bonus", "Other"
  adjustmentFactor: number;           // e.g. 10 for a 10:1 split
  oldFaceValue: number | null;
  newFaceValue: number | null;
  notes: string | null;
}

export interface CreateCorporateActionRequest {
  actionDate: string;                 // "yyyy-MM-dd" (DateOnly input)
  actionType: string;                 // "Split", "Merger", "Bonus", "Other"
  adjustmentFactor: number;
  oldFaceValue?: number | null;
  newFaceValue?: number | null;
  notes?: string | null;
}

// ==================== ERROR ====================

export interface ApiError {
  message: string;
}
```

---

## API Endpoints — Complete Reference

### FUNDS — `/api/funds`

---

#### 1. GET `/api/funds`
Get all funds (~20,000+ records).

**Response:** `Fund[]`

```json
[
  {
    "schemeCode": 119551,
    "isinGrowth": "INF209KA12Z1",
    "isinDivReinvestment": "INF209KA13Z9",
    "schemeName": "Aditya Birla Sun Life Banking & PSU Debt Fund - DIRECT - IDCW",
    "latestNav": 103.8813,
    "latestNavDate": "02-Apr-2026",
    "fundHouse": "Aditya Birla Sun Life Mutual Fund",
    "schemeType": "Open Ended Schemes",
    "schemeCategory": "Debt Scheme - Banking and PSU Fund"
  }
]
```

> ⚠️ Large payload. Prefer the search endpoint for filtering.

---

#### 2. GET `/api/funds/{schemeCode}`
Get a single fund by scheme code.

| Param | Type | In |
|-------|------|----|
| schemeCode | int | path |

**200:** `Fund`
**404:** empty body

---

#### 3. GET `/api/funds/search?q={query}`
Search funds by name, fund house, or scheme code.

| Param | Type | In | Required |
|-------|------|----|----------|
| q | string | query | yes |

**200:** `Fund[]`
**400:** if `q` is empty

> 💡 Debounce this in the UI (300–500ms).

---

#### 4. POST `/api/funds/sync`
Trigger manual sync of fund master list from AMFI. Long-running (~10–15s).

**Request body:** none

**200:**
```json
{
  "totalParsed": 20347,
  "added": 150,
  "updated": 20197,
  "unchanged": 0,
  "duration": "00:00:14.5623401"
}
```

> 💡 Show a loading spinner. This also runs automatically on startup (if DB is empty) and daily at 1 AM IST via a background service.

---

#### 5. GET `/api/funds/count`
Get total fund count.

**200:**
```json
{ "count": 20347 }
```

---

### NAV — `/api/nav`

---

#### 6. GET `/api/nav/{schemeCode}?from={date}&to={date}&interval={interval}&adjusted={bool}`
Get NAV history for a fund. Backend handles smart caching with 1-hour cooldown.
Supports optional date range filtering, interval-based sampling, and split-adjusted NAVs.

| Param | Type | In | Required | Default | Format / Values |
|-------|------|----|----------|---------|-----------------|
| schemeCode | int | path | yes | — | — |
| from | DateOnly | query | no | — | `yyyy-MM-dd` |
| to | DateOnly | query | no | — | `yyyy-MM-dd` |
| interval | string | query | no | `daily` | `daily`, `weekly`, `monthly` |
| adjusted | bool | query | no | `false` | `true` or `false` |

> `from`/`to` filter the date range. `interval` samples data: `weekly` = last trading day per ISO week, `monthly` = last trading day per month. `latestNavDate` always reflects the overall latest date in DB (unaffected by filtering). `adjusted=true` divides pre-action NAVs by the cumulative corporate action factor — use this for charts that should show a continuous line across splits/mergers. The response always includes `corporateActions` (the full list for this fund) regardless of the `adjusted` flag.

**200:**
```json
{
  "schemeCode": 140085,
  "schemeName": "Nippon India ETF Nifty Next 50 Junior BeES",
  "fundHouse": "Nippon India Mutual Fund",
  "latestNavDate": "01-04-2026",
  "wasFetchedFromApi": true,
  "isAdjusted": false,
  "data": [
    { "date": "01-04-2026", "nav": 667.16510 },
    { "date": "31-03-2026", "nav": 650.35830 }
  ],
  "corporateActions": [
    {
      "id": 1,
      "schemeCode": 140085,
      "actionDate": "15-06-2023",
      "actionType": "Split",
      "adjustmentFactor": 10.0,
      "oldFaceValue": 10.0,
      "newFaceValue": 1.0,
      "notes": "Face value split 10:1"
    }
  ]
}
```

**400:** `{ "message": "'from' must be before 'to'." }`
**400:** `{ "message": "interval must be 'daily', 'weekly', or 'monthly'." }`
**404:** `{ "message": "Fund with scheme code 999999 not found." }`

> `data` is sorted **newest-first**. `wasFetchedFromApi` = true means fresh data was pulled. Use `interval=monthly` for charts spanning many years to reduce payload. `corporateActions` is always included (empty array if none). When `adjusted=true`, `isAdjusted` is `true` and the `nav` values in `data` are divided by cumulative adjustment factors — the chart will show a continuous line across splits.

---

#### 7. GET `/api/nav/{schemeCode}/returns?startDate={date}&endDate={date}`
Point-to-point returns (absolute + CAGR) between two precise dates.

| Param | Type | In | Required | Format |
|-------|------|----|----------|--------|
| schemeCode | int | path | yes | — |
| startDate | DateOnly | query | yes | `yyyy-MM-dd` |
| endDate | DateOnly | query | yes | `yyyy-MM-dd` |

> `startDate` must be before `endDate`. The backend finds the closest available NAV date (within 7 days) for each. If the requested date falls outside the available data range entirely (e.g. before fund inception or after the latest NAV), the backend **clamps to the earliest/latest available NAV** and returns the maximum available data instead of null.

**200:**
```json
{
  "schemeCode": 140085,
  "schemeName": "Nippon India ETF Nifty Next 50 Junior BeES",
  "fundHouse": "Nippon India Mutual Fund",
  "absoluteReturn": 145.67,
  "cagr": 14.23,
  "startDate": "15-03-2019",
  "startNav": 271.56,
  "endDate": "01-04-2026",
  "endNav": 667.17
}
```

**400:** `{ "message": "startDate must be before endDate." }`
**404:** `{ "message": "Fund with scheme code 999999 not found." }`

> The response `startDate`/`endDate` reflect the **actual** dates used (which may differ from requested dates due to clamping or closest-date matching). Display these actual dates in the UI so the user knows the exact range computed. If no valid range exists (e.g. fund has no data at all), the nullable fields will be `null` — show "N/A" in the UI.

---

#### 8. GET `/api/nav/{schemeCode}/rolling-returns?periodInYears={n}&from={date}&to={date}&interval={interval}`
Rolling returns for a single fund. One data point per trading day (or per week/month if interval is specified) that has enough lookback history.
Supports optional date range filtering to align comparison across funds.

| Param | Type | In | Required | Default | Range / Values |
|-------|------|----|----------|---------|---------|
| schemeCode | int | path | yes | — | — |
| periodInYears | int | query | no | 1 | 1–30 |
| from | DateOnly | query | no | — | `yyyy-MM-dd` |
| to | DateOnly | query | no | — | `yyyy-MM-dd` |
| interval | string | query | no | `daily` | `daily`, `weekly`, `monthly` |

> `from`/`to` filter the rolling return data points by date range. If the requested range exceeds available data, the backend returns the max available range (no nulls). Summary stats (`minReturn`, `maxReturn`, etc.) and `dataPoints` are computed on the **filtered** date range before interval sampling. `interval` further samples the `data` array only.

**200:**
```json
{
  "schemeCode": 140085,
  "schemeName": "Nippon India ETF Nifty Next 50 Junior BeES",
  "fundHouse": "Nippon India Mutual Fund",
  "periodInYears": 3,
  "dataPoints": 1024,
  "minReturn": -8.45,
  "maxReturn": 32.67,
  "averageReturn": 14.23,
  "medianReturn": 13.87,
  "data": [
    { "date": "01-04-2026", "returnPercentage": 18.45 },
    { "date": "31-03-2026", "returnPercentage": 17.92 }
  ]
}
```

**400:** `{ "message": "periodInYears must be between 1 and 30." }`
**400:** `{ "message": "'from' must be before 'to'." }`
**400:** `{ "message": "interval must be 'daily', 'weekly', or 'monthly'." }`
**404:** `{ "message": "Fund with scheme code 999999 not found." }`

> `data` can have thousands of entries at daily interval. **Always render as a line chart, not a table.** Use `interval=weekly` or `interval=monthly` for long periods. Summary stats (min/max/avg/median) can be shown in a legend/card. When comparing funds, pass the same `from`/`to` range to each call so the stats and data align.

---

### CORPORATE ACTIONS — `/api/corporateactions`

---

#### 9. GET `/api/corporateactions/anomalies?threshold={percent}`
Detect NAV anomalies across all funds that have NAV history loaded. Returns large day-over-day changes that may indicate splits, mergers, or bonuses. Admin reviews these and records corporate actions.

| Param | Type | In | Required | Default | Range |
|-------|------|----|----------|---------|-------|
| threshold | decimal | query | no | 30 | 5–99 |

**200:** `NavAnomalyDto[]` — sorted by absolute change % descending.

```json
[
  {
    "schemeCode": 112272,
    "schemeName": "Nippon India ETF Nifty BeES",
    "fundHouse": "Nippon India Mutual Fund",
    "date": "20-12-2019",
    "previousNav": 1178.5432,
    "currentNav": 117.8543,
    "changePercent": -90.0,
    "direction": "Drop",
    "hasCorporateAction": false
  }
]
```

**400:** `{ "message": "threshold must be between 5 and 99." }`

> ⚠️ This scans all NAV history in the DB — can be slow. Only call from the admin page. `hasCorporateAction: true` means the admin already recorded an action for that fund on that date.

---

#### 10. GET `/api/corporateactions/{schemeCode}`
Get all recorded corporate actions for a fund.

| Param | Type | In |
|-------|------|----|  
| schemeCode | int | path |

**200:** `CorporateActionDto[]` — sorted by action date ascending.

```json
[
  {
    "id": 1,
    "schemeCode": 112272,
    "actionDate": "19-12-2019",
    "actionType": "Split",
    "adjustmentFactor": 10.0,
    "oldFaceValue": 10.0,
    "newFaceValue": 1.0,
    "notes": "Face value split 10:1"
  }
]
```

---

#### 11. POST `/api/corporateactions/{schemeCode}`
Record a new corporate action for a fund.

| Param | Type | In |
|-------|------|----|  
| schemeCode | int | path |

**Request body:** `CreateCorporateActionRequest`

```json
{
  "actionDate": "2019-12-19",
  "actionType": "Split",
  "adjustmentFactor": 10.0,
  "oldFaceValue": 10.0,
  "newFaceValue": 1.0,
  "notes": "Face value split 10:1"
}
```

**201:** `CorporateActionDto` (the created action)
**400:** `{ "message": "actionType must be 'Split', 'Merger', 'Bonus', or 'Other'." }`
**404:** `{ "message": "Fund with scheme code 999999 not found." }`

> `adjustmentFactor` — the factor to divide pre-action NAVs by: **10** for a 10:1 split (FV 10→1), **0.2** for a 1:5 reverse split, **2** for a 1:1 bonus. `actionDate` uses `yyyy-MM-dd` format in the request body.

---

#### 12. DELETE `/api/corporateactions/{schemeCode}/{actionId}`
Delete a corporate action.

| Param | Type | In |
|-------|------|----|  
| schemeCode | int | path |
| actionId | int | path |

**204:** No content (success)
**404:** `{ "message": "Corporate action {id} not found for scheme {code}." }`

---

## Corporate Actions — How They Affect Returns

**Splits, mergers, and bonuses cause sudden NAV discontinuities.** Without adjustment, a 10:1 split looks like a -90% crash.

The backend automatically adjusts returns calculations:
- **Point-to-point returns** (`/nav/{code}/returns`) — the `startNav` in the response is the **adjusted** NAV (original NAV ÷ cumulative adjustment factor). `absoluteReturn` and `cagr` are computed on the adjusted value.
- **Rolling returns** (`/nav/{code}/rolling-returns`) — each rolling window's start NAV is adjusted for any corporate actions within that window.
- **NAV history** (`/nav/{code}?adjusted=true`) — when `adjusted=true`, all pre-action NAVs are divided by the cumulative factor so the chart shows a continuous line. When `adjusted=false` (default), raw/unadjusted NAVs are returned. The `corporateActions` array is always included in the response — use it to display action markers or tooltips on the chart.

### Admin Workflow
1. Navigate to the admin page → call `GET /api/corporateactions/anomalies?threshold=30`
2. Review each anomaly — if it's a split/merger/bonus, record it via `POST /api/corporateactions/{schemeCode}`
3. Once recorded, `hasCorporateAction` will be `true` for that anomaly on the next scan
4. Returns calculations automatically account for the recorded action

---

## Comparison Philosophy

**The backend serves individual fund data. The UI owns comparison logic.**

To compare multiple funds, the Angular frontend should:
1. Call the individual endpoints (`/nav/{code}`, `/nav/{code}/returns`, `/nav/{code}/rolling-returns`) for each fund in parallel using `forkJoin`.
2. Compose the comparison view client-side (tables, overlaid charts, growth rebasing).
3. For "Growth of ₹10,000" charts, fetch NAV history per fund with `interval=monthly` (or `weekly`), then rebase to 10,000 in the UI using the first common date.

---

## Error Handling Summary

| Status | Shape | When |
|--------|-------|------|
| 200 | response body as documented above | Success |
| 400 | `{ "message": "..." }` | Invalid/missing query params |
| 404 | `{ "message": "Fund with scheme code {x} not found." }` | Fund not in DB (NavController) |
| 404 | empty body | Fund not in DB (FundsController) |
| 201 | `CorporateActionDto` | Corporate action created |
| 204 | empty body | Corporate action deleted |

---

## Important Behaviors & UI Rules

1. **All dates** from the API are `"dd-MM-yyyy"` strings except `Fund.latestNavDate` which is `"dd-MMM-yyyy"`.
2. **NAV caching is server-side** — the backend manages a smart cache with 1-hour cooldown. The frontend does NOT need to implement caching for NAV data.
3. **`wasFetchedFromApi`** in `NavDataResponse` — use this to show a "freshly updated" indicator if desired.
4. **`null` return values** — when a fund doesn't have NAV data covering the requested date range, `absoluteReturn`, `cagr`, `startDate`, `startNav`, `endDate`, `endNav` will all be `null`. Display "N/A".
5. **Rolling returns `data` arrays** can have thousands of entries at daily interval. **Always render as line charts, never as tables.** Use `interval=weekly` or `interval=monthly` to reduce data points.
6. **`interval` parameter** — `daily` (default) returns every trading day, `weekly` returns the last trading day of each ISO week, `monthly` returns the last trading day of each month. For rolling returns, summary stats are computed on the filtered date range (after `from`/`to`) but before interval sampling.
7. **Fund sync** (`POST /api/funds/sync`) takes ~10–15 seconds. Show a loading spinner with a message like "Syncing fund data...".
8. **Search** (`GET /api/funds/search?q=`) — debounce input at 300–500ms.
9. **`schemeCode`** (integer) is the primary identifier used across all endpoints.
10. **Comparison is UI-driven** — there are no multi-fund comparison endpoints. The frontend calls individual fund endpoints in parallel (`forkJoin`) and composes the comparison view (tables, overlaid charts, growth rebasing) client-side.
11. **Fund master list** auto-syncs on first startup and daily at 1 AM IST. The manual sync endpoint is for ad-hoc refreshes.
12. **Corporate actions (splits/mergers/bonuses)** — the backend automatically adjusts returns calculations. The `startNav` in returns responses is the **adjusted** value. NAV history supports opt-in `adjusted=true` for split-adjusted charts. The `corporateActions` array is always included in `NavDataResponse` — use it to show action markers on charts or details in a tooltip.
13. **Anomaly detection** (`GET /api/corporateactions/anomalies`) scans all NAV data in the DB. Only call from the admin page, not on regular user-facing pages.

---

## Suggested Angular Architecture

### Services
```
FundService             → handles all /api/funds/* calls
NavService              → handles all /api/nav/* calls
CorporateActionService  → handles all /api/corporateactions/* calls
```

### Suggested Routes

| Route | Page | Primary Endpoints |
|-------|------|-------------------|
| `/` | Dashboard | `GET /api/funds/count` |
| `/funds` | Fund List + Search | `GET /api/funds`, `GET /api/funds/search?q=` |
| `/funds/:schemeCode` | Fund Detail (NAV chart + returns) | `GET /api/funds/{id}`, `GET /api/nav/{id}?interval=`, `GET /api/nav/{id}/returns?startDate=&endDate=` |
| `/funds/:schemeCode/rolling` | Rolling Returns Chart | `GET /api/nav/{id}/rolling-returns?periodInYears=&interval=` |
| `/compare` | Comparison Page (UI-driven) | Calls individual fund endpoints in parallel via `forkJoin` |
| `/admin/sync` | Manual Sync Trigger | `POST /api/funds/sync` |
| `/admin/corporate-actions` | Anomaly Detection + Action Management | `GET /api/corporateactions/anomalies`, `POST /api/corporateactions/{code}` |

### Chart Recommendations

| Chart | Type | Library | X-Axis | Y-Axis |
|-------|------|---------|--------|--------|
| Single fund NAV history | Line | ngx-charts or Chart.js | Date | NAV (₹) |
| Growth of ₹10,000 comparison | Multi-line | ngx-charts or Chart.js | Date | Value (₹) |
| Rolling returns (single) | Line | ngx-charts or Chart.js | Date | Return % |
| Rolling returns comparison | Multi-line | ngx-charts or Chart.js | Date | Return % |
| Returns comparison | Table or bar chart | HTML table or ngx-charts | Fund | Return % |

> For **Growth of ₹10,000**: fetch `/api/nav/{code}?from=&to=&interval=monthly` for each fund, find the common start date, then rebase each fund's NAVs to 10,000 in the UI.

---

## Endpoint Quick Reference

| # | Method | Route | Purpose |
|---|--------|-------|---------|
| 1 | GET | `/api/funds` | All funds |
| 2 | GET | `/api/funds/{schemeCode}` | Single fund |
| 3 | GET | `/api/funds/search?q=` | Search funds |
| 4 | POST | `/api/funds/sync` | Manual AMFI sync |
| 5 | GET | `/api/funds/count` | Fund count |
| 6 | GET | `/api/nav/{schemeCode}?from=&to=&interval=&adjusted=` | NAV history (filterable, samplable, adjustable) |
| 7 | GET | `/api/nav/{schemeCode}/returns?startDate=&endDate=` | Point-to-point returns |
| 8 | GET | `/api/nav/{schemeCode}/rolling-returns?periodInYears=&from=&to=&interval=` | Rolling returns (filterable, samplable) |
| 9 | GET | `/api/corporateactions/anomalies?threshold=` | Detect NAV anomalies |
| 10 | GET | `/api/corporateactions/{schemeCode}` | List corporate actions |
| 11 | POST | `/api/corporateactions/{schemeCode}` | Add corporate action |
| 12 | DELETE | `/api/corporateactions/{schemeCode}/{actionId}` | Delete corporate action |
