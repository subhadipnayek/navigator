import { Component, computed, inject, input, OnInit, signal } from '@angular/core';
import { FormsModule } from '@angular/forms';
import { NavService } from '../../services/nav.service';
import { RollingReturnResponse } from '../../models/fund.model';
import { LineChart, ChartSeriesData } from '../../shared/line-chart/line-chart';
import { toChartDate } from '../../shared/date-utils';
import { MatButtonModule } from '@angular/material/button';
import { MatButtonToggleModule } from '@angular/material/button-toggle';
import { MatCardModule } from '@angular/material/card';
import { MatNativeDateModule } from '@angular/material/core';
import { MatDatepickerModule } from '@angular/material/datepicker';
import { MatFormFieldModule } from '@angular/material/form-field';
import { MatInputModule } from '@angular/material/input';
import { MatListModule } from '@angular/material/list';
import { MatProgressBarModule } from '@angular/material/progress-bar';

@Component({
  selector: 'app-rolling-returns',
  imports: [
    LineChart,
    FormsModule,
    MatButtonModule,
    MatButtonToggleModule,
    MatCardModule,
    MatDatepickerModule,
    MatFormFieldModule,
    MatInputModule,
    MatListModule,
    MatNativeDateModule,
    MatProgressBarModule,
  ],
  templateUrl: './rolling-returns.html',
})
export class RollingReturns implements OnInit {
  private readonly navService = inject(NavService);

  readonly schemeCode = input.required<string>();

  readonly data = signal<RollingReturnResponse | null>(null);
  readonly loading = signal(true);
  readonly error = signal<string | null>(null);
  readonly selectedPeriod = signal(1);

  fromDate: Date | null = null;
  toDate: Date | null = null;

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

  readonly minSelectableDate = computed<Date | null>(() =>
    this.parseApiDisplayDate(this.data()?.navStartDate ?? null),
  );

  readonly maxSelectableDate = computed<Date>(() => new Date());

  readonly percentFormat = { type: 'price' as const, precision: 2, minMove: 0.01 };

  ngOnInit(): void {
    const today = new Date();
    const fiveYearsAgo = new Date();
    fiveYearsAgo.setFullYear(today.getFullYear() - 5);
    this.toDate = today;
    this.fromDate = fiveYearsAgo;

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
        from: this.toApiDate(this.fromDate) || undefined,
        to: this.toApiDate(this.toDate) || undefined,
        interval: 'monthly',
      })
      .subscribe({
        next: (res) => {
          this.data.set(res);
          this.clampSelectedDateRange();
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

  private toApiDate(d: Date | null): string {
    if (!d) return '';
    const year = d.getFullYear();
    const month = String(d.getMonth() + 1).padStart(2, '0');
    const day = String(d.getDate()).padStart(2, '0');
    return `${year}-${month}-${day}`;
  }

  private clampSelectedDateRange(): void {
    const minDate = this.minSelectableDate();
    const maxDate = this.maxSelectableDate();

    if (minDate && this.fromDate && this.fromDate < minDate) this.fromDate = minDate;
    if (maxDate && this.toDate && this.toDate > maxDate) this.toDate = maxDate;
    if (this.fromDate && this.toDate && this.fromDate > this.toDate) this.fromDate = this.toDate;
  }

  private parseApiDisplayDate(value: string | null): Date | null {
    if (!value) return null;

    const [day, month, year] = value.split('-').map(Number);
    if (!day || !month || !year) return null;

    return new Date(year, month - 1, day);
  }
}
