import { HttpClient, HttpParams } from '@angular/common/http';
import { inject, Injectable } from '@angular/core';
import { Observable } from 'rxjs';
import {
  NavAnomalyDto,
  CorporateActionDto,
  CreateCorporateActionRequest,
  AnomalyAutoRecordResultDto,
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

  autoRecordAnomalies(
    threshold?: number,
    minConfidence?: number,
  ): Observable<AnomalyAutoRecordResultDto> {
    let params = new HttpParams();
    if (threshold != null) params = params.set('threshold', threshold);
    if (minConfidence != null) params = params.set('minConfidence', minConfidence);
    return this.http.post<AnomalyAutoRecordResultDto>(
      `${this.api.baseUrl}/api/corporateactions/anomalies/auto-record`,
      null,
      { params },
    );
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

  updateAction(
    schemeCode: number,
    actionId: number,
    request: CreateCorporateActionRequest,
  ): Observable<CorporateActionDto> {
    return this.http.put<CorporateActionDto>(
      `${this.api.baseUrl}/api/corporateactions/${schemeCode}/${actionId}`,
      request,
    );
  }

  deleteAction(schemeCode: number, actionId: number): Observable<void> {
    return this.http.delete<void>(
      `${this.api.baseUrl}/api/corporateactions/${schemeCode}/${actionId}`,
    );
  }
}
