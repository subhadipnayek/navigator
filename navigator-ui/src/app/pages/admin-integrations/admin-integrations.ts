import { Component, OnInit, inject, signal, ChangeDetectionStrategy } from '@angular/core';
import { DecimalPipe, DatePipe } from '@angular/common';
import { RouterLink, RouterLinkActive } from '@angular/router';
import { IntegrationService } from '../../services/integration.service';
import {
  ApiEndpointConfig,
  ApiEndpointsSummary,
  ApiEndpointTestResult,
} from '../../models/integration.model';
import { ToastService } from '../../shared/toast/toast.service';
import { MatButtonModule } from '@angular/material/button';
import { MatCardModule } from '@angular/material/card';
import { MatChipsModule } from '@angular/material/chips';
import { MatProgressBarModule } from '@angular/material/progress-bar';

@Component({
  selector: 'app-admin-integrations',
  changeDetection: ChangeDetectionStrategy.OnPush,
  imports: [
    DecimalPipe,
    DatePipe,
    RouterLink,
    RouterLinkActive,
    MatButtonModule,
    MatCardModule,
    MatChipsModule,
    MatProgressBarModule,
  ],
  templateUrl: './admin-integrations.html',
})
export class AdminIntegrations implements OnInit {
  private readonly integrationService = inject(IntegrationService);
  private readonly toast = inject(ToastService);

  readonly loading = signal(false);
  readonly testingKey = signal<string | null>(null);
  readonly testingAll = signal(false);
  readonly summary = signal<ApiEndpointsSummary | null>(null);
  readonly testResult = signal<ApiEndpointTestResult | null>(null);

  ngOnInit(): void {
    this.loadEndpoints();
  }

  loadEndpoints(): void {
    this.loading.set(true);
    this.integrationService.getEndpoints().subscribe({
      next: (data) => {
        this.summary.set(data);
        this.loading.set(false);
      },
      error: (err) => {
        this.loading.set(false);
        this.toast.error(err.error?.message ?? 'Failed to load API integrations');
      },
    });
  }

  testConnection(key: string): void {
    this.testingKey.set(key);
    this.testResult.set(null);

    this.integrationService.testEndpoint(key).subscribe({
      next: (res) => {
        this.testingKey.set(null);
        this.testResult.set(res);
        if (res.success) {
          this.toast.success(`Connection test passed: ${res.statusCode} (${res.latencyMs}ms)`);
        } else {
          this.toast.error(`Connection test failed: ${res.errorMessage || res.statusCode}`);
        }
        this.loadEndpoints();
      },
      error: (err) => {
        this.testingKey.set(null);
        this.toast.error(err.error?.message ?? 'Health check request failed');
      },
    });
  }

  pingAll(): void {
    this.testingAll.set(true);
    this.integrationService.healthCheckAll().subscribe({
      next: (data) => {
        this.testingAll.set(false);
        this.summary.set(data);
        this.toast.success(`Health checks complete: ${data.healthyCount}/${data.totalEndpoints} healthy`);
      },
      error: (err) => {
        this.testingAll.set(false);
        this.toast.error(err.error?.message ?? 'Health check batch failed');
      },
    });
  }

  dismissTestResult(): void {
    this.testResult.set(null);
  }
}
