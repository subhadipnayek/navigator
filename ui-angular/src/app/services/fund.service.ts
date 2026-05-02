import { HttpClient, HttpParams } from '@angular/common/http';
import { inject, Injectable } from '@angular/core';
import { Observable } from 'rxjs';
import {
  AdminDatabaseSummary,
  AdminFundStorage,
  Fund,
  FundSyncResult,
  FundCount,
  NavDataResponse,
  PagedResponse,
} from '../models/fund.model';
import { ApiConfig } from '../core/api-config';

@Injectable({ providedIn: 'root' })
export class FundService {
  private readonly http = inject(HttpClient);
  private readonly api = inject(ApiConfig);

  searchFunds(query: string): Observable<Fund[]> {
    const params = new HttpParams().set('q', query);
    return this.http.get<Fund[]>(`${this.api.baseUrl}/api/funds/search`, { params });
  }

  getFund(schemeCode: number): Observable<Fund> {
    return this.http.get<Fund>(`${this.api.baseUrl}/api/funds/${schemeCode}`);
  }

  getAllFunds(): Observable<Fund[]> {
    return this.http.get<Fund[]>(`${this.api.baseUrl}/api/funds`);
  }

  getFundCount(): Observable<FundCount> {
    return this.http.get<FundCount>(`${this.api.baseUrl}/api/funds/count`);
  }

  syncFunds(): Observable<FundSyncResult> {
    return this.http.post<FundSyncResult>(`${this.api.baseUrl}/api/funds/sync`, null);
  }

  getAdminSummary(): Observable<AdminDatabaseSummary> {
    return this.http.get<AdminDatabaseSummary>(`${this.api.baseUrl}/api/admin/funds/summary`);
  }

  getAdminFunds(options: {
    q?: string;
    navStatus?: 'all' | 'with-nav' | 'without-nav';
    page?: number;
    pageSize?: number;
  }): Observable<PagedResponse<AdminFundStorage>> {
    let params = new HttpParams()
      .set('navStatus', options.navStatus ?? 'all')
      .set('page', options.page ?? 1)
      .set('pageSize', options.pageSize ?? 50);
    if (options.q?.trim()) params = params.set('q', options.q.trim());
    return this.http.get<PagedResponse<AdminFundStorage>>(`${this.api.baseUrl}/api/admin/funds`, {
      params,
    });
  }

  syncAdminFunds(): Observable<FundSyncResult> {
    return this.http.post<FundSyncResult>(`${this.api.baseUrl}/api/admin/funds/sync`, null);
  }

  syncFundNavHistory(schemeCode: number): Observable<NavDataResponse> {
    return this.http.post<NavDataResponse>(
      `${this.api.baseUrl}/api/admin/funds/${schemeCode}/nav-history/sync`,
      null,
    );
  }

  deleteFundNavHistory(schemeCode: number): Observable<void> {
    return this.http.delete<void>(`${this.api.baseUrl}/api/admin/funds/${schemeCode}/nav-history`);
  }

  deleteFund(schemeCode: number): Observable<void> {
    return this.http.delete<void>(`${this.api.baseUrl}/api/admin/funds/${schemeCode}`);
  }
}
