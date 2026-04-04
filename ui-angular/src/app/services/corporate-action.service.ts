import { HttpClient, HttpParams } from '@angular/common/http';
import { inject, Injectable } from '@angular/core';
import { Observable } from 'rxjs';
import {
  NavAnomalyDto,
  CorporateActionDto,
  CreateCorporateActionRequest,
} from '../models/fund.model';
import { ApiConfig } from '../core/api-config';

@Injectable({ providedIn: 'root' })
export class CorporateActionService {
  private readonly http = inject(HttpClient);
  private readonly api = inject(ApiConfig);

  getAnomalies(threshold?: number): Observable<NavAnomalyDto[]> {
    let params = new HttpParams();
    if (threshold != null) params = params.set('threshold', threshold);
    return this.http.get<NavAnomalyDto[]>(`${this.api.baseUrl}/api/corporateactions/anomalies`, {
      params,
    });
  }

  getActions(schemeCode: number): Observable<CorporateActionDto[]> {
    return this.http.get<CorporateActionDto[]>(
      `${this.api.baseUrl}/api/corporateactions/${schemeCode}`,
    );
  }

  createAction(
    schemeCode: number,
    request: CreateCorporateActionRequest,
  ): Observable<CorporateActionDto> {
    return this.http.post<CorporateActionDto>(
      `${this.api.baseUrl}/api/corporateactions/${schemeCode}`,
      request,
    );
  }

  deleteAction(schemeCode: number, actionId: number): Observable<void> {
    return this.http.delete<void>(
      `${this.api.baseUrl}/api/corporateactions/${schemeCode}/${actionId}`,
    );
  }
}
