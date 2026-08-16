// ---------- Portfolio / Basket Models ----------

export interface BasketFundAllocation {
  schemeCode: number;
  allocationPercent: number;
}

export interface BasketDefinition {
  name: string;
  funds: BasketFundAllocation[];
  rebalanceFrequency: 'None' | 'Monthly' | 'Quarterly' | 'Yearly';
}

export interface SimulationRequest {
  basket: BasketDefinition;
  sipAmount: number;
  sipDay: number;
  lumpSumAmount: number;
  startDate: string; // dd-MM-yyyy
  endDate: string;   // dd-MM-yyyy
}

export interface FundSimulationDetail {
  schemeCode: number;
  schemeName: string;
  allocationPercent: number;
  totalUnits: number;
  totalInvested: number;
  finalValue: number;
  absoluteReturnPercent: number;
}

export interface FundSnapshot {
  schemeCode: number;
  value: number;
  units: number;
}

export interface PortfolioSnapshot {
  date: string;
  portfolioValue: number;
  totalInvested: number;
  gainPercent: number;
  funds: FundSnapshot[];
}

export interface SimulationResponse {
  basketName: string;
  totalInvested: number;
  finalCorpusValue: number;
  absoluteReturnPercent: number;
  cagr: number | null;
  xirr: number | null;
  maxDrawdownPercent: number | null;
  volatility: number | null;
  startDate: string;
  endDate: string;
  effectiveStartDate: string;
  effectiveEndDate: string;
  wasDateRangeClamped: boolean;
  dateRangeNote: string | null;
  sipInstallments: number;
  sipAmount: number;
  lumpSumAmount: number;
  fundDetails: FundSimulationDetail[];
  timeSeries: PortfolioSnapshot[];
}

// ---------- Comparison Models ----------

export interface ComparisonEntry {
  name: string;
  sipAmount: number;
  sipDay: number;
  lumpSumAmount: number;
  basket?: BasketDefinition;
  singleFundSchemeCode?: number;
}

export interface ComparisonRequest {
  entries: ComparisonEntry[];
  startDate: string;
  endDate: string;
  normalizeSipAmounts: boolean;
}

export interface ComparisonEntryResult {
  name: string;
  totalInvested: number;
  finalCorpusValue: number;
  absoluteReturnPercent: number;
  cagr: number | null;
  xirr: number | null;
  maxDrawdownPercent: number | null;
  volatility: number | null;
  alphaVsFirst: number | null;
}

export interface ComparisonSeriesValue {
  name: string;
  portfolioValue: number;
  normalizedValue: number;
}

export interface ComparisonTimePoint {
  date: string;
  values: ComparisonSeriesValue[];
}

export interface ComparisonResponse {
  startDate: string;
  endDate: string;
  effectiveStartDate: string;
  effectiveEndDate: string;
  wasDateRangeClamped: boolean;
  dateRangeNote: string | null;
  isNormalized: boolean;
  results: ComparisonEntryResult[];
  timeSeries: ComparisonTimePoint[];
}

// ---------- What-If Models ----------

export interface WhatIfScenario {
  name: string;
  basket: BasketDefinition;
  sipAmount: number;
  sipDay: number;
  lumpSumAmount: number;
}

export interface WhatIfRequest {
  scenarios: WhatIfScenario[];
  startDate: string;
  endDate: string;
}

export interface WhatIfScenarioResult {
  name: string;
  sipAmount: number;
  lumpSumAmount: number;
  totalInvested: number;
  finalCorpusValue: number;
  absoluteReturnPercent: number;
  cagr: number | null;
  xirr: number | null;
  maxDrawdownPercent: number | null;
  volatility: number | null;
  corpusPerRupeeInvested: number;
}

export interface WhatIfInsights {
  bestAbsoluteReturner: string | null;
  bestCapitalEfficiency: string | null;
  lowestDrawdown: string | null;
  allocationBeatCapital: boolean;
  allocationBeatCapitalDetails: string | null;
}

export interface WhatIfResponse {
  startDate: string;
  endDate: string;
  effectiveStartDate: string;
  effectiveEndDate: string;
  wasDateRangeClamped: boolean;
  dateRangeNote: string | null;
  scenarios: WhatIfScenarioResult[];
  insights: WhatIfInsights;
}

// ---------- Rolling Comparison Models ----------

export interface RollingComparisonRequest {
  entries: ComparisonEntry[];
  periodInYears: number;
  startDate: string;
  endDate: string;
  interval?: 'daily' | 'weekly' | 'monthly';
}

export interface RollingComparisonEntryResult {
  name: string;
  dataPoints: number;
  minReturn: number | null;
  maxReturn: number | null;
  averageReturn: number | null;
  medianReturn: number | null;
}

export interface RollingComparisonSeriesValue {
  name: string;
  returnPercent: number | null;
}

export interface RollingComparisonTimePoint {
  date: string;
  values: RollingComparisonSeriesValue[];
}

export interface RollingComparisonResponse {
  startDate: string;
  endDate: string;
  periodInYears: number;
  results: RollingComparisonEntryResult[];
  timeSeries: RollingComparisonTimePoint[];
}

// ---------- User Portfolio Models ----------

export interface UserPortfolioSummary {
  id: number;
  name: string;
  fundCount: number;
  updatedAtUtc: string;
}

export interface PortfolioTransaction {
  id: number | null;
  clientRowId: string | null;
  schemeCode: number;
  schemeName: string;
  transactionDate: string;
  transactionType: 'Buy' | 'Sell' | string;
  units: number;
  nav: number;
  amount: number;
  notes: string | null;
  error: string | null;
}

export interface YearTransactionSummary {
  totalInvested: number;
  transactionCount: number;
  fundsCovered: number;
}

export interface PortfolioMetricSet {
  investedAmount: number;
  currentValue: number;
  realizedPnl: number;
  unrealizedPnl: number;
  absoluteReturnPercent: number;
  cagrPercent: number | null;
  xirrPercent: number | null;
  fundCount: number;
  activeFundCount: number;
  fullyExitedFundCount: number;
}

export interface PortfolioFundInsight {
  schemeCode: number;
  schemeName: string;
  fundHouse: string | null;
  schemeCategory: string | null;
  assetClass: string;
  units: number;
  investedAmount: number;
  currentValue: number;
  realizedPnl: number;
  unrealizedPnl: number;
  absoluteReturnPercent: number;
  cagrPercent: number | null;
  xirrPercent: number | null;
  holdingPeriodDays: number;
  isFullyExited: boolean;
  latestNav: number | null;
  latestNavDate: string | null;
  hasNav: boolean;
}

export interface AllocationSlice {
  label: string;
  value: number;
  percent: number;
}

export interface ContributionPeriod {
  period: string;
  invested: number;
  redemptions: number;
  netContribution: number;
}

export interface SipConsistency {
  activeMonths: number;
  investedMonths: number;
  consistencyPercent: number;
  averageMonthlyInvestment: number;
}

export interface UserPortfolioDashboard {
  portfolioId: number;
  portfolioName: string;
  metrics: PortfolioMetricSet;
  funds: PortfolioFundInsight[];
  assetAllocation: AllocationSlice[];
  fundAllocation: AllocationSlice[];
  financialYearContributions: ContributionPeriod[];
  calendarYearContributions: ContributionPeriod[];
  sipConsistency: SipConsistency;
  emptyStateMessages: string[];
}

export interface SavePortfolioTransactionsResponse {
  isValid: boolean;
  message: string | null;
  savedCount: number;
  transactions: PortfolioTransaction[];
  summary: YearTransactionSummary;
}

export interface ImportPortfolioTransactionsResponse {
  isValid: boolean;
  message: string | null;
  totalRows: number;
  importedCount: number;
  replacedCount: number;
  skippedOutOfRangeCount: number;
  importedDateStart: string | null;
  importedDateEnd: string | null;
  totalAmountImported: number;
  fundCount: number;
  rowErrors: string[];
  rowWarnings: string[];
  transactions: PortfolioTransaction[];
  summary: YearTransactionSummary;
}

export interface ImportDeletePreviewResponse {
  deleteCount: number;
}

export interface PortfolioBenchmarkRequest {
  benchmarkType: 'fund' | 'basket';
  schemeCode?: number | null;
  basket?: BasketDefinition | null;
  durationYears?: number | null;
  startDate?: string | null;
  endDate?: string | null;
  rollingPeriodYears?: number | null;
}

export interface PortfolioBenchmarkMetric {
  investedAmount: number;
  currentValue: number;
  absoluteReturnPercent: number;
  xirrPercent: number | null;
}

export interface PortfolioBenchmarkResponse {
  benchmarkName: string;
  effectiveStartDate: string;
  effectiveEndDate: string;
  comparisonNote: string | null;
  portfolio: PortfolioBenchmarkMetric;
  benchmark: PortfolioBenchmarkMetric;
  benchmarkRollingReturn: BenchmarkRollingReturn | null;
  alphaPercent: number | null;
  verdict: string;
}

export interface BenchmarkRollingReturn {
  periodYears: number;
  dataPoints: number;
  minimumPercent: number | null;
  maximumPercent: number | null;
  averagePercent: number | null;
}

export interface UserGridPreference {
  gridKey: string;
  stateJson: string | null;
}

export interface MatchPortfolioFundRow {
  rowNumber: number;
  isin: string | null;
  schemeCode: string | null;
  schemeName: string | null;
}

export interface MatchedPortfolioFund {
  rowNumber: number;
  schemeCode: number | null;
  schemeName: string | null;
  error: string | null;
}

export interface MatchPortfolioFundResponse {
  rows: MatchedPortfolioFund[];
}
