import { HttpClient } from '@angular/common/http';
import { inject, Injectable } from '@angular/core';
import { Observable } from 'rxjs';
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
    return this.http.get<UserPortfolioSummary[]>(this.userPortfolioUrl());
  }

  createUserPortfolio(name: string): Observable<UserPortfolioSummary> {
    return this.http.post<UserPortfolioSummary>(this.userPortfolioUrl(), { name });
  }

  getUserPortfolioDashboard(
    portfolioId: number,
    showFullyExitedFunds: boolean,
  ): Observable<UserPortfolioDashboard> {
    return this.http.get<UserPortfolioDashboard>(this.userPortfolioUrl(`${portfolioId}/dashboard`), {
      params: { showFullyExitedFunds },
    });
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

  compareUserPortfolioBenchmark(
    portfolioId: number,
    request: PortfolioBenchmarkRequest,
  ): Observable<PortfolioBenchmarkResponse> {
    return this.http.post<PortfolioBenchmarkResponse>(
      this.userPortfolioUrl(`${portfolioId}/benchmark`),
      request,
    );
  }
}
