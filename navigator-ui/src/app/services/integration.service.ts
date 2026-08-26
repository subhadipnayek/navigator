import { HttpClient } from '@angular/common/http';
import { inject, Injectable } from '@angular/core';
import { Observable } from 'rxjs';
import { ApiConfig } from '../core/api-config';
import {
  ApiEndpointConfig,
  ApiEndpointsSummary,
  ApiEndpointTestResult,
  UpdateApiEndpointConfigRequest,
} from '../models/integration.model';

@Injectable({ providedIn: 'root' })
export class IntegrationService {
  private readonly http = inject(HttpClient);
  private readonly api = inject(ApiConfig);

  getEndpoints(): Observable<ApiEndpointsSummary> {
    return this.http.get<ApiEndpointsSummary>(`${this.api.baseUrl}/api/admin/integrations`);
  }

  updateEndpoint(key: string, req: UpdateApiEndpointConfigRequest): Observable<ApiEndpointConfig> {
    return this.http.put<ApiEndpointConfig>(`${this.api.baseUrl}/api/admin/integrations/${key}`, req);
  }

  resetEndpoint(key: string): Observable<ApiEndpointConfig> {
    return this.http.post<ApiEndpointConfig>(`${this.api.baseUrl}/api/admin/integrations/${key}/reset`, null);
  }

  testEndpoint(key: string): Observable<ApiEndpointTestResult> {
    return this.http.post<ApiEndpointTestResult>(`${this.api.baseUrl}/api/admin/integrations/${key}/test`, null);
  }

  healthCheckAll(): Observable<ApiEndpointsSummary> {
    return this.http.post<ApiEndpointsSummary>(`${this.api.baseUrl}/api/admin/integrations/health-all`, null);
  }
}
