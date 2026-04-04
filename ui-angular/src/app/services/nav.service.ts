import { HttpClient, HttpParams } from '@angular/common/http';
import { inject, Injectable } from '@angular/core';
import { Observable } from 'rxjs';
import { NavDataResponse, ReturnsResponse, RollingReturnResponse } from '../models/fund.model';
import { ApiConfig } from '../core/api-config';

@Injectable({ providedIn: 'root' })
export class NavService {
  private readonly http = inject(HttpClient);
  private readonly api = inject(ApiConfig);

  getNavHistory(
    schemeCode: number,
    options?: {
      from?: string;
      to?: string;
      interval?: 'daily' | 'weekly' | 'monthly';
      adjusted?: boolean;
    },
  ): Observable<NavDataResponse> {
    let params = new HttpParams();
    if (options?.from) params = params.set('from', options.from);
    if (options?.to) params = params.set('to', options.to);
    if (options?.interval) params = params.set('interval', options.interval);
    if (options?.adjusted) params = params.set('adjusted', 'true');
    return this.http.get<NavDataResponse>(`${this.api.baseUrl}/api/nav/${schemeCode}`, { params });
  }

  getReturns(schemeCode: number, startDate: string, endDate: string): Observable<ReturnsResponse> {
    const params = new HttpParams().set('startDate', startDate).set('endDate', endDate);
    return this.http.get<ReturnsResponse>(`${this.api.baseUrl}/api/nav/${schemeCode}/returns`, {
      params,
    });
  }

  getRollingReturns(
    schemeCode: number,
    periodInYears: number,
    options?: { from?: string; to?: string; interval?: 'daily' | 'weekly' | 'monthly' },
  ): Observable<RollingReturnResponse> {
    let params = new HttpParams().set('periodInYears', periodInYears);
    if (options?.from) params = params.set('from', options.from);
    if (options?.to) params = params.set('to', options.to);
    if (options?.interval) params = params.set('interval', options.interval);
    return this.http.get<RollingReturnResponse>(
      `${this.api.baseUrl}/api/nav/${schemeCode}/rolling-returns`,
      { params },
    );
  }
}
