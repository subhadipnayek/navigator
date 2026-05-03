import { Component, computed, inject, input, OnInit, signal } from '@angular/core';
import { RouterLink } from '@angular/router';
import { DecimalPipe, TitleCasePipe } from '@angular/common';
import { FormsModule } from '@angular/forms';
import { FundService } from '../../services/fund.service';
import { NavService } from '../../services/nav.service';
import {
  Fund,
  NavDataResponse,
  ReturnsResponse,
  CorporateActionDto,
} from '../../models/fund.model';
import { LineChart, ChartSeriesData } from '../../shared/line-chart/line-chart';
import { toChartDate } from '../../shared/date-utils';
import { MatButtonModule } from '@angular/material/button';
import { MatButtonToggleModule } from '@angular/material/button-toggle';
import { MatCardModule } from '@angular/material/card';
import { MatCheckboxModule } from '@angular/material/checkbox';
import { MatDividerModule } from '@angular/material/divider';
import { MatFormFieldModule } from '@angular/material/form-field';
import { MatInputModule } from '@angular/material/input';
import { MatListModule } from '@angular/material/list';
import { MatNativeDateModule } from '@angular/material/core';
import { MatDatepickerModule } from '@angular/material/datepicker';
import { MatProgressBarModule } from '@angular/material/progress-bar';

@Component({
  selector: 'app-fund-detail',
  imports: [
    RouterLink,
    DecimalPipe,
    TitleCasePipe,
    LineChart,
    FormsModule,
    MatButtonModule,
    MatButtonToggleModule,
    MatCardModule,
    MatCheckboxModule,
    MatDividerModule,
    MatDatepickerModule,
    MatFormFieldModule,
    MatInputModule,
    MatListModule,
    MatNativeDateModule,
    MatProgressBarModule,
  ],
  templateUrl: './fund-detail.html',
})
export class FundDetail implements OnInit {
  private readonly fundService = inject(FundService);
  private readonly navService = inject(NavService);

  readonly schemeCode = input.required<string>();

  readonly fund = signal<Fund | null>(null);
  readonly navData = signal<NavDataResponse | null>(null);
  readonly returns = signal<ReturnsResponse | null>(null);
  readonly loading = signal(true);
  readonly error = signal<string | null>(null);
  readonly returnsLoading = signal(false);
  readonly returnsError = signal<string | null>(null);
  readonly adjusted = signal(false);
  readonly selectedInterval = signal<'daily' | 'weekly' | 'monthly'>('daily');

  startDate: Date | null = null;
  endDate: Date | null = null;

  readonly corporateActions = computed<CorporateActionDto[]>(() => {
    return this.navData()?.corporateActions ?? [];
  });

  readonly minSelectableDate = computed<Date | null>(() =>
    this.parseApiDisplayDate(this.navData()?.navStartDate ?? this.fund()?.navStartDate ?? null),
  );

  readonly maxSelectableDate = computed<Date | null>(() =>
    this.parseApiDisplayDate(this.navData()?.navEndDate ?? this.fund()?.navEndDate ?? null),
  );

  readonly navChartSeries = computed<ChartSeriesData[]>(() => {
    const nav = this.navData();
    if (!nav || nav.data.length === 0) return [];
    return [
      {
        name: nav.schemeName,
        data: nav.data.map((d) => ({ time: toChartDate(d.date), value: d.nav })),
      },
    ];
  });

  ngOnInit(): void {
    const code = Number(this.schemeCode());

    // Default date range: 1 year ago to today
    const today = new Date();
    const oneYearAgo = new Date();
    oneYearAgo.setFullYear(today.getFullYear() - 1);
    this.endDate = today;
    this.startDate = oneYearAgo;

    this.fundService.getFund(code).subscribe({
      next: (fund) => {
        this.fund.set(fund);
        this.clampSelectedDateRange();
      },
      error: (err) => {
        this.error.set(err.error?.message ?? 'Fund not found');
        this.loading.set(false);
      },
    });

    this.navService
      .getNavHistory(code, { adjusted: this.adjusted(), interval: this.selectedInterval() })
      .subscribe({
        next: (data) => {
          this.navData.set(data);
          this.clampSelectedDateRange();
          this.loading.set(false);
        },
        error: () => this.loading.set(false),
      });

    this.fetchReturns();
  }

  reloadNav(): void {
    const code = Number(this.schemeCode());
    this.navService
      .getNavHistory(code, { adjusted: this.adjusted(), interval: this.selectedInterval() })
      .subscribe({
        next: (data) => {
          this.navData.set(data);
          this.clampSelectedDateRange();
        },
      });
  }

  toggleAdjusted(): void {
    this.adjusted.update((v) => !v);
    this.reloadNav();
  }

  setInterval(interval: 'daily' | 'weekly' | 'monthly'): void {
    this.selectedInterval.set(interval);
    this.reloadNav();
  }

  fetchReturns(): void {
    const startDate = this.toApiDate(this.startDate);
    const endDate = this.toApiDate(this.endDate);
    if (!startDate || !endDate) return;
    this.returnsLoading.set(true);
    this.returnsError.set(null);

    this.navService.getReturns(Number(this.schemeCode()), startDate, endDate).subscribe({
      next: (data) => {
        this.returns.set(data);
        this.returnsLoading.set(false);
      },
      error: (err) => {
        this.returnsError.set(err.error?.message ?? 'Failed to load returns');
        this.returnsLoading.set(false);
      },
    });
  }

  formatReturn(value: number | null): string {
    if (value === null) return 'N/A';
    return value.toFixed(2) + '%';
  }

  returnClass(value: number | null): string {
    if (value === null) return '';
    return value >= 0 ? 'positive' : 'negative';
  }

  returnVariant(value: number | null): 'default' | 'positive' | 'negative' {
    if (value === null) return 'default';
    return value >= 0 ? 'positive' : 'negative';
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

    if (minDate && this.startDate && this.startDate < minDate) this.startDate = minDate;
    if (maxDate && this.endDate && this.endDate > maxDate) this.endDate = maxDate;
    if (this.startDate && this.endDate && this.startDate > this.endDate) this.startDate = this.endDate;
  }

  private parseApiDisplayDate(value: string | null): Date | null {
    if (!value) return null;

    const [day, month, year] = value.split('-').map(Number);
    if (!day || !month || !year) return null;

    return new Date(year, month - 1, day);
  }
}
