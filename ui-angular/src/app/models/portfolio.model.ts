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
