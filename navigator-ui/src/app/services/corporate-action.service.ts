import { HttpClient, HttpParams } from '@angular/common/http';
import { inject, Injectable } from '@angular/core';
import { Observable } from 'rxjs';
import {
  NavAnomalyDto,
  CorporateActionDto,
  CreateCorporateActionRequest,
  AnomalyAutoRecordResultDto,
  CorporateActionReviewQueueDto,
} from '../models/fund.model';
import { ApiConfig } from '../core/api-config';

@Injectable({ providedIn: 'root' })
export class CorporateActionService {
  private readonly http = inject(HttpClient);
  private readonly api = inject(ApiConfig);

  getAnomalies(threshold?: number, minDaysSincePrevious?: number): Observable<NavAnomalyDto[]> {
    let params = new HttpParams();
    if (threshold != null) params = params.set('threshold', threshold);
    if (minDaysSincePrevious != null) params = params.set('minDaysSincePrevious', minDaysSincePrevious);
    return this.http.get<NavAnomalyDto[]>(`${this.api.baseUrl}/api/corporateactions/anomalies`, {
      params,
    });
  }

  getReviewQueue(options: {
    threshold?: number;
    minGapDays?: number;
    filter?: string;
    includeRecorded?: boolean;
    schemeCode?: number;
    q?: string;
    actionType?: string;
    priority?: string;
    minConfidence?: number;
    minAutoRecordConfidence?: number;
    limit?: number;
  }): Observable<CorporateActionReviewQueueDto> {
    let params = new HttpParams();
    if (options.threshold != null) params = params.set('threshold', options.threshold);
    if (options.minGapDays != null) params = params.set('minGapDays', options.minGapDays);
    if (options.filter) params = params.set('filter', options.filter);
    if (options.includeRecorded != null) params = params.set('includeRecorded', options.includeRecorded);
    if (options.schemeCode != null) params = params.set('schemeCode', options.schemeCode);
    if (options.q?.trim()) params = params.set('q', options.q.trim());
    if (options.actionType) params = params.set('actionType', options.actionType);
    if (options.priority) params = params.set('priority', options.priority);
    if (options.minConfidence != null) params = params.set('minConfidence', options.minConfidence);
    if (options.minAutoRecordConfidence != null) {
      params = params.set('minAutoRecordConfidence', options.minAutoRecordConfidence);
    }
    if (options.limit != null) params = params.set('limit', options.limit);
    return this.http.get<CorporateActionReviewQueueDto>(
      `${this.api.baseUrl}/api/corporateactions/review-queue`,
      { params },
    );
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
