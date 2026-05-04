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
} from '../models/portfolio.model';
import { ApiConfig } from '../core/api-config';

@Injectable({ providedIn: 'root' })
export class PortfolioService {
  private readonly http = inject(HttpClient);
  private readonly api = inject(ApiConfig);

  private url(path: string): string {
    return `${this.api.baseUrl}/api/portfolio/${path}`;
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
}
