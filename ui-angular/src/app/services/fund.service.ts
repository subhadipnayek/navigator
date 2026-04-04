import { HttpClient, HttpParams } from '@angular/common/http';
import { inject, Injectable } from '@angular/core';
import { Observable } from 'rxjs';
import { Fund, FundSyncResult, FundCount } from '../models/fund.model';
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
}
