// ---------- Fund Models ----------

export interface Fund {
  schemeCode: number;
  isinGrowth: string | null;
  isinDivReinvestment: string | null;
  schemeName: string;
  latestNav: number | null;
  latestNavDate: string | null; // "02-Apr-2026"
  fundHouse: string | null;
  schemeType: string | null; // "Open Ended Schemes"
  schemeCategory: string | null; // "Debt Scheme - Banking and PSU Fund"
}

export interface FundSyncResult {
  totalParsed: number;
  added: number;
  updated: number;
  unchanged: number;
  duration: string; // TimeSpan e.g. "00:00:12.3456789"
}

export interface FundCount {
  count: number;
}

// ---------- NAV History Models ----------

export interface NavHistoryEntry {
  date: string; // "dd-MM-yyyy"
  nav: number;
}

export interface NavDataResponse {
  schemeCode: number;
  schemeName: string;
  fundHouse: string | null;
  latestNavDate: string | null; // "dd-MM-yyyy"
  wasFetchedFromApi: boolean;
  isAdjusted: boolean;
  data: NavHistoryEntry[]; // sorted newest-first
  corporateActions: CorporateActionDto[];
}

export interface NavGapDto {
  schemeCode: number;
  schemeName: string;
  fundHouse: string | null;
  previousDate: string;
  nextDate: string;
  missingFrom: string;
  missingTo: string;
  gapDays: number;
  missingCalendarDays: number;
  status?: string;
  insertedRecords?: number;
}

export interface NavGapRepairItemDto extends NavGapDto {
  insertedRecords: number;
  status: string;
}

export interface NavGapRepairResultDto {
  schemeCode: number;
  schemeName: string;
  detectedGaps: number;
  repairedGaps: number;
  verifiedEmptyGaps: number;
  insertedRecords: number;
  gaps: NavGapRepairItemDto[];
}

// ---------- Returns Models ----------

export interface ReturnsResponse {
  schemeCode: number;
  schemeName: string;
  fundHouse: string | null;
  absoluteReturn: number | null; // percentage e.g. 12.45 means 12.45%
  cagr: number | null; // annualized %
  startDate: string | null; // "dd-MM-yyyy"
  startNav: number | null;
  endDate: string | null; // "dd-MM-yyyy"
  endNav: number | null;
  hasDiscontinuity: boolean;
}

// ---------- Rolling Returns Models ----------

export interface RollingReturnEntry {
  date: string; // "dd-MM-yyyy"
  returnPercentage: number;
}

export interface RollingReturnResponse {
  schemeCode: number;
  schemeName: string;
  fundHouse: string | null;
  periodInYears: number;
  dataPoints: number;
  minReturn: number | null;
  maxReturn: number | null;
  averageReturn: number | null;
  medianReturn: number | null;
  data: RollingReturnEntry[];
}

// ---------- Corporate Actions Models ----------

export interface NavAnomalyDto {
  schemeCode: number;
  schemeName: string;
  fundHouse: string | null;
  date: string; // "dd-MM-yyyy"
  previousDate: string; // "dd-MM-yyyy"
  daysSincePrevious: number;
  previousNav: number;
  currentNav: number;
  changePercent: number;
  direction: string; // "Drop" or "Spike"
  hasCorporateAction: boolean;
}

export interface CorporateActionDto {
  id: number;
  schemeCode: number;
  actionDate: string; // "dd-MM-yyyy"
  actionType: string; // "Split", "Merger", "Bonus", "Discontinuity", "Other"
  adjustmentFactor: number;
  oldFaceValue: number | null;
  newFaceValue: number | null;
  notes: string | null;
}

export interface CreateCorporateActionRequest {
  actionDate: string; // "yyyy-MM-dd"
  actionType: string;
  adjustmentFactor: number;
  oldFaceValue?: number | null;
  newFaceValue?: number | null;
  notes?: string | null;
}

// ---------- Error Response ----------

export interface ApiError {
  message: string;
}
