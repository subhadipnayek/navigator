import { HttpClient } from '@angular/common/http';
import { inject, Injectable } from '@angular/core';
import { Observable, map } from 'rxjs';
import {
  SimulationRequest,
  SimulationResponse,
  ComparisonRequest,
  ComparisonResponse,
  WhatIfRequest,
  WhatIfResponse,
  RollingComparisonRequest,
  RollingComparisonResponse,
  UserPortfolioSummary,
  UserPortfolioDashboard,
  PortfolioTransaction,
  SavePortfolioTransactionsResponse,
  PortfolioBenchmarkRequest,
  PortfolioBenchmarkResponse,
  ImportDeletePreviewResponse,
  ImportPortfolioTransactionsResponse,
  ImportCommitRequest,
  ImportPreviewResult,
  ImportStrategy,
  YearTransactionSummary,
  UserGridPreference,
  MatchPortfolioFundRow,
  MatchPortfolioFundResponse,
} from '../models/portfolio.model';
import { ApiConfig } from '../core/api-config';

@Injectable({ providedIn: 'root' })
export class PortfolioService {
  private readonly http = inject(HttpClient);
  private readonly api = inject(ApiConfig);

  private url(path: string): string {
    return `${this.api.baseUrl}/api/portfolio/${path}`;
  }

  private userPortfolioUrl(path = ''): string {
    return `${this.api.baseUrl}/api/user-portfolios${path ? `/${path}` : ''}`;
  }

  simulate(request: SimulationRequest): Observable<SimulationResponse> {
    return this.http.post<SimulationResponse>(this.url('simulate'), request);
  }

  compare(request: ComparisonRequest): Observable<ComparisonResponse> {
    return this.http.post<ComparisonResponse>(this.url('compare'), request);
  }

  whatIf(request: WhatIfRequest): Observable<WhatIfResponse> {
    return this.http.post<WhatIfResponse>(this.url('what-if'), request);
  }

  rollingComparison(request: RollingComparisonRequest): Observable<RollingComparisonResponse> {
    return this.http.post<RollingComparisonResponse>(this.url('rolling-comparison'), request);
  }

  getUserPortfolios(): Observable<UserPortfolioSummary[]> {
    return this.http.get<UserPortfolioSummary[]>(this.userPortfolioUrl()).pipe(
      map((rows) => rows ?? []),
    );
  }

  createUserPortfolio(name: string): Observable<UserPortfolioSummary> {
    return this.http.post<UserPortfolioSummary>(this.userPortfolioUrl(), { name });
  }

  getUserPortfolioDashboard(
    portfolioId: number,
    showFullyExitedFunds: boolean,
  ): Observable<UserPortfolioDashboard> {
    return this.http
      .get<any>(this.userPortfolioUrl(`${portfolioId}/dashboard`), {
        params: { showFullyExitedFunds },
      })
      .pipe(
        map((raw) => {
          if (!raw) return raw;
          const fyInflows = (raw.financialYearInflows ?? raw.financialYearContributions ?? []).map((c: any) => ({
            period: c.periodLabel ?? c.period ?? '',
            invested: c.totalInflow ?? c.invested ?? 0,
            redemptions: c.outflow ?? c.redemptions ?? 0,
            netContribution: c.netInflow ?? c.netContribution ?? 0,
          }));
          const cyInflows = (raw.calendarYearInflows ?? raw.calendarYearContributions ?? []).map((c: any) => ({
            period: c.periodLabel ?? c.period ?? '',
            invested: c.totalInflow ?? c.invested ?? 0,
            redemptions: c.outflow ?? c.redemptions ?? 0,
            netContribution: c.netInflow ?? c.netContribution ?? 0,
          }));
          const assetAlloc = (raw.assetAllocation ?? []).map((a: any) => ({
            label: a.label ?? '',
            value: a.amount ?? a.value ?? 0,
            percent: a.percentage ?? a.percent ?? 0,
          }));
          const fundAlloc = (raw.fundAllocation ?? []).map((a: any) => ({
            label: a.label ?? '',
            value: a.amount ?? a.value ?? 0,
            percent: a.percentage ?? a.percent ?? 0,
          }));
          const emptyStates = raw.emptyStates ?? raw.emptyStateMessages ?? [];

          return {
            portfolioId: raw.id ?? raw.portfolioId ?? 0,
            portfolioName: raw.name ?? raw.portfolioName ?? '',
            metrics: {
              investedAmount: raw.metrics?.investedAmount ?? 0,
              currentValue: raw.metrics?.currentValue ?? 0,
              realizedPnl: raw.metrics?.realizedGain ?? raw.metrics?.realizedPnl ?? 0,
              unrealizedPnl: raw.metrics?.unrealizedGain ?? raw.metrics?.unrealizedPnl ?? 0,
              absoluteReturnPercent: raw.metrics?.gainPercentage ?? raw.metrics?.absoluteReturnPercent ?? 0,
              cagrPercent: raw.metrics?.cagrPercent ?? null,
              xirrPercent: raw.metrics?.xirr ?? raw.metrics?.xirrPercent ?? null,
              fundCount: raw.metrics?.activeFundsCount ?? raw.metrics?.fundCount ?? 0,
              activeFundCount: raw.metrics?.activeFundsCount ?? raw.metrics?.activeFundCount ?? 0,
              fullyExitedFundCount: raw.metrics?.fullyExitedFundsCount ?? raw.metrics?.fullyExitedFundCount ?? 0,
            },
            funds: (raw.funds ?? []).map((f: any) => ({
              schemeCode: f.schemeCode,
              schemeName: f.schemeName,
              fundHouse: f.fundHouse,
              schemeCategory: f.schemeCategory,
              assetClass: f.assetClass ?? 'Equity',
              units: f.units ?? 0,
              investedAmount: f.investedAmount ?? 0,
              currentValue: f.currentValue ?? 0,
              realizedPnl: f.realizedGain ?? f.realizedPnl ?? 0,
              unrealizedPnl: f.unrealizedGain ?? f.unrealizedPnl ?? 0,
              absoluteReturnPercent: f.gainPercentage ?? f.absoluteReturnPercent ?? 0,
              cagrPercent: f.cagr ?? f.cagrPercent ?? null,
              xirrPercent: f.xirr ?? f.xirrPercent ?? null,
              holdingPeriodDays: f.holdingPeriodDays ?? 0,
              isFullyExited: f.isFullyExited ?? false,
              latestNav: f.latestNav ?? null,
              latestNavDate: f.latestNavDate ?? null,
              hasNav: f.latestNav != null && f.latestNav > 0,
            })),
            assetAllocation: assetAlloc,
            fundAllocation: fundAlloc,
            financialYearContributions: fyInflows,
            calendarYearContributions: cyInflows,
            sipConsistency: {
              activeMonths: raw.sipConsistency?.expectedMonthlySips ?? raw.sipConsistency?.activeMonths ?? 0,
              investedMonths: raw.sipConsistency?.completedSips ?? raw.sipConsistency?.investedMonths ?? 0,
              consistencyPercent: raw.sipConsistency?.consistencyScore ?? raw.sipConsistency?.consistencyPercent ?? 100,
              averageMonthlyInvestment: raw.sipConsistency?.averageMonthlyInvestment ?? 0,
            },
            emptyStateMessages: emptyStates,
          } as UserPortfolioDashboard;
        }),
      );
  }

  getUserPortfolioTransactions(portfolioId: number, financialYear: string): Observable<{
    transactions: PortfolioTransaction[];
    summary: { totalInvested: number; transactionCount: number; fundsCovered: number };
  }> {
    return this.http.get<{
      transactions: PortfolioTransaction[];
      summary: { totalInvested: number; transactionCount: number; fundsCovered: number };
    }>(this.userPortfolioUrl(`${portfolioId}/transactions`), { params: { financialYear } });
  }

  saveUserPortfolioTransactions(
    portfolioId: number,
    financialYear: string,
    transactions: PortfolioTransaction[],
  ): Observable<SavePortfolioTransactionsResponse> {
    return this.http.put<SavePortfolioTransactionsResponse>(
      this.userPortfolioUrl(`${portfolioId}/transactions`),
      { financialYear, transactions },
    );
  }

  importPreview(portfolioId: number, file: File): Observable<ImportPreviewResult> {
    const formData = new FormData();
    formData.append('file', file);
    return this.http.post<ImportPreviewResult>(
      this.userPortfolioUrl(`${portfolioId}/transactions/import-preview`),
      formData,
    );
  }

  importCommit(
    portfolioId: number,
    request: ImportCommitRequest,
    financialYear?: string,
  ): Observable<{
    insertedCount: number;
    strategy: ImportStrategy;
    transactions: PortfolioTransaction[];
    summary: YearTransactionSummary;
  }> {
    return this.http.post<{
      insertedCount: number;
      strategy: ImportStrategy;
      transactions: PortfolioTransaction[];
      summary: YearTransactionSummary;
    }>(this.userPortfolioUrl(`${portfolioId}/transactions/import-commit`), request, {
      params: financialYear ? { financialYear } : {},
    });
  }

  importUserPortfolioTransactions(
    portfolioId: number,
    financialYear: string,
    importMode: 'Append' | 'Replace',
    startDate: string,
    endDate: string,
    file: File,
  ): Observable<ImportPortfolioTransactionsResponse> {
    const formData = new FormData();
    formData.append('file', file);
    return this.http.post<ImportPortfolioTransactionsResponse>(
      this.userPortfolioUrl(`${portfolioId}/transactions/import-excel`),
      formData,
      { params: { financialYear, importMode, startDate, endDate } },
    );
  }

  getImportDeletePreview(
    portfolioId: number,
    startDate: string,
    endDate: string,
  ): Observable<ImportDeletePreviewResponse> {
    return this.http.get<ImportDeletePreviewResponse>(
      this.userPortfolioUrl(`${portfolioId}/transactions/import-delete-preview`),
      { params: { startDate, endDate } },
    );
  }

  compareUserPortfolioBenchmark(
    portfolioId: number,
    request: PortfolioBenchmarkRequest,
  ): Observable<PortfolioBenchmarkResponse> {
    return this.http.post<PortfolioBenchmarkResponse>(
      this.userPortfolioUrl(`${portfolioId}/benchmark`),
      request,
    );
  }

  getGridPreference(gridKey: string): Observable<UserGridPreference> {
    return this.http.get<UserGridPreference>(this.userPortfolioUrl(`grid-preferences/${gridKey}`));
  }

  saveGridPreference(gridKey: string, stateJson: string): Observable<UserGridPreference> {
    return this.http.put<UserGridPreference>(this.userPortfolioUrl(`grid-preferences/${gridKey}`), {
      stateJson,
    });
  }

  matchPortfolioFunds(rows: MatchPortfolioFundRow[]): Observable<MatchPortfolioFundResponse> {
    return this.http.post<MatchPortfolioFundResponse>(this.userPortfolioUrl('fund-matches'), { rows });
  }
}
