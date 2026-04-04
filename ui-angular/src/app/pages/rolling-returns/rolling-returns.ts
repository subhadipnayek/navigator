import { Component, computed, inject, input, OnInit, signal } from '@angular/core';
import { FormsModule } from '@angular/forms';
import { NavService } from '../../services/nav.service';
import { RollingReturnResponse } from '../../models/fund.model';
import { LineChart, ChartSeriesData } from '../../shared/line-chart/line-chart';
import { toChartDate } from '../../shared/date-utils';
import { StatCard } from '../../shared/stat-card/stat-card';
import { Skeleton } from '../../shared/skeleton/skeleton';

@Component({
  selector: 'app-rolling-returns',
  imports: [LineChart, FormsModule, StatCard, Skeleton],
  templateUrl: './rolling-returns.html',
  styleUrl: './rolling-returns.scss',
})
export class RollingReturns implements OnInit {
  private readonly navService = inject(NavService);

  readonly schemeCode = input.required<string>();

  readonly data = signal<RollingReturnResponse | null>(null);
  readonly loading = signal(true);
  readonly error = signal<string | null>(null);
  readonly selectedPeriod = signal(1);

  fromDate = '';
  toDate = '';

  readonly periods = [1, 2, 3, 5, 10];

  readonly chartSeries = computed<ChartSeriesData[]>(() => {
    const d = this.data();
    if (!d || d.data.length === 0) return [];
    return [
      {
        name: d.schemeName,
        data: d.data.map((pt) => ({ time: toChartDate(pt.date), value: pt.returnPercentage })),
      },
    ];
  });

  readonly percentFormat = { type: 'price' as const, precision: 2, minMove: 0.01 };

  ngOnInit(): void {
    const today = new Date();
    const fiveYearsAgo = new Date();
    fiveYearsAgo.setFullYear(today.getFullYear() - 5);
    this.toDate = today.toISOString().slice(0, 10);
    this.fromDate = fiveYearsAgo.toISOString().slice(0, 10);

    this.loadData();
  }

  selectPeriod(period: number): void {
    this.selectedPeriod.set(period);
    this.loadData();
  }

  loadData(): void {
    this.loading.set(true);
    this.error.set(null);

    this.navService
      .getRollingReturns(Number(this.schemeCode()), this.selectedPeriod(), {
        from: this.fromDate || undefined,
        to: this.toDate || undefined,
        interval: 'monthly',
      })
      .subscribe({
        next: (res) => {
          this.data.set(res);
          this.loading.set(false);
        },
        error: (err) => {
          this.error.set(err.error?.message ?? 'Failed to load rolling returns');
          this.loading.set(false);
        },
      });
  }

  formatPercent(value: number | null): string {
    if (value === null) return 'N/A';
    return value.toFixed(2) + '%';
  }
}
