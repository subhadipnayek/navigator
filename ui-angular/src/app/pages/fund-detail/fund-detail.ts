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
import { StatCard } from '../../shared/stat-card/stat-card';
import { Skeleton } from '../../shared/skeleton/skeleton';

@Component({
  selector: 'app-fund-detail',
  imports: [RouterLink, DecimalPipe, TitleCasePipe, LineChart, FormsModule, StatCard, Skeleton],
  templateUrl: './fund-detail.html',
  styleUrl: './fund-detail.scss',
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

  startDate = '';
  endDate = '';

  readonly corporateActions = computed<CorporateActionDto[]>(() => {
    return this.navData()?.corporateActions ?? [];
  });

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
    this.endDate = this.toInputDate(today);
    this.startDate = this.toInputDate(oneYearAgo);

    this.fundService.getFund(code).subscribe({
      next: (fund) => this.fund.set(fund),
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
        next: (data) => this.navData.set(data),
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
    if (!this.startDate || !this.endDate) return;
    this.returnsLoading.set(true);
    this.returnsError.set(null);

    this.navService.getReturns(Number(this.schemeCode()), this.startDate, this.endDate).subscribe({
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

  private toInputDate(d: Date): string {
    return d.toISOString().slice(0, 10);
  }
}
