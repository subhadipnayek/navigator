export interface ApiEndpointConfig {
  key: string;
  name: string;
  description: string;
  url: string;
  defaultUrl: string;
  method: string;
  headersJson: string;
  payloadTemplate: string;
  isActive: boolean;
  lastHealthCheckAtUtc?: string | null;
  lastHealthStatus: 'Healthy' | 'Degraded' | 'Down' | 'Unknown';
  lastStatusCode: number;
  lastLatencyMs: number;
  lastErrorMessage?: string | null;
  updatedAtUtc: string;
}

export interface UpdateApiEndpointConfigRequest {
  url: string;
  method?: string;
  headersJson?: string;
  payloadTemplate?: string;
  isActive?: boolean;
}

export interface ApiEndpointTestResult {
  key: string;
  url: string;
  method: string;
  statusCode: number;
  latencyMs: number;
  healthStatus: 'Healthy' | 'Degraded' | 'Down';
  responseSnippet: string;
  errorMessage?: string;
  success: boolean;
}

export interface ApiEndpointsSummary {
  totalEndpoints: number;
  healthyCount: number;
  degradedCount: number;
  downCount: number;
  avgLatencyMs: number;
  endpoints: ApiEndpointConfig[];
}
