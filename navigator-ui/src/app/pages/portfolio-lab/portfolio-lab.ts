import { Component, inject, signal, OnDestroy, ChangeDetectionStrategy } from '@angular/core';
import { DecimalPipe } from '@angular/common';
import { FormsModule } from '@angular/forms';
import {
  Subject,
  debounceTime,
  distinctUntilChanged,
  switchMap,
  of,
  takeUntil,
  finalize,
} from 'rxjs';
import { FundService } from '../../services/fund.service';
import { PortfolioService } from '../../services/portfolio.service';
import { Fund } from '../../models/fund.model';
import {
  BasketDefinition,
  BasketFundAllocation,
  SimulationResponse,
  ComparisonResponse,
  WhatIfResponse,
  RollingComparisonResponse,
  ComparisonEntry,
} from '../../models/portfolio.model';
import { LineChart, ChartSeriesData } from '../../shared/line-chart/line-chart';
import { toChartDate } from '../../shared/date-utils';
import { CHART_COLORS } from '../../shared/chart-colors';

import { MatButtonModule } from '@angular/material/button';
import { MatButtonToggleModule } from '@angular/material/button-toggle';
import { MatCardModule } from '@angular/material/card';
import { MatChipsModule } from '@angular/material/chips';
import { MatNativeDateModule } from '@angular/material/core';
import { MatDatepickerModule } from '@angular/material/datepicker';
import { MatFormFieldModule } from '@angular/material/form-field';
import { MatInputModule } from '@angular/material/input';
import { MatListModule } from '@angular/material/list';
import { MatProgressBarModule } from '@angular/material/progress-bar';
import { MatSelectModule } from '@angular/material/select';
import { MatSliderModule } from '@angular/material/slider';
import { MatTabsModule } from '@angular/material/tabs';
import { MatTableModule } from '@angular/material/table';
import { MatIconModule } from '@angular/material/icon';
import { MatTooltipModule } from '@angular/material/tooltip';
import { MatDividerModule } from '@angular/material/divider';

// ─────────────────────────────────────────────────────────────
// Types
// ─────────────────────────────────────────────────────────────

interface BasketSlot {
  id: number;
  name: string;
  funds: { fund: Fund; allocationPercent: number; locked?: boolean }[];
  sipAmount: number;
  sipDay: number;
  lumpSumAmount: number;
  rebalanceFrequency: 'None' | 'Monthly' | 'Quarterly' | 'Yearly';
}

@Component({
  selector: 'app-portfolio-lab',
  changeDetection: ChangeDetectionStrategy.OnPush,
  imports: [
    FormsModule,
    DecimalPipe,
    LineChart,
    MatButtonModule,
    MatButtonToggleModule,
    MatCardModule,
    MatChipsModule,
    MatDatepickerModule,
    MatNativeDateModule,
    MatFormFieldModule,
    MatInputModule,
    MatListModule,
    MatProgressBarModule,
    MatSelectModule,
    MatSliderModule,
    MatTabsModule,
    MatTableModule,
    MatIconModule,
    MatTooltipModule,
    MatDividerModule,
  ],
  templateUrl: './portfolio-lab.html',
  styleUrl: './portfolio-lab.scss',
})
export class PortfolioLab implements OnDestroy {
  private readonly fundService = inject(FundService);
  private readonly portfolioService = inject(PortfolioService);
  private readonly destroy$ = new Subject<void>();
  private readonly searchSubject = new Subject<{ query: string; basketId: number }>();

  // ── UI state ──
  readonly activeTab = signal<'simulate' | 'compare' | 'whatif' | 'rolling'>('simulate');
  readonly loading = signal(false);
  readonly error = signal<string | null>(null);
  readonly searchResults = signal<Fund[]>([]);
  readonly searchingBasketId = signal<number | null>(null);

  // ── Simulate basket selector ──
  readonly selectedSimBasketId = signal<number | null>(null);

  // ── Global date range ──
  startDate: Date;
  endDate: Date;
  readonly maxSelectableDate = new Date();

  // ── Baskets ──
  nextId = 1;
  readonly baskets = signal<BasketSlot[]>([this.newBasket('Basket A'), this.newBasket('Basket B')]);

  // ── Results ──
  readonly simResult = signal<SimulationResponse | null>(null);
  readonly compareResult = signal<ComparisonResponse | null>(null);
  readonly whatIfResult = signal<WhatIfResponse | null>(null);
  readonly rollingResult = signal<RollingComparisonResponse | null>(null);

  // ── Chart series ──
  readonly growthChartSeries = signal<ChartSeriesData[]>([]);
  readonly rollingChartSeries = signal<ChartSeriesData[]>([]);

  // ── Rolling period ──
  readonly rollingPeriod = signal(3);

  // ── Comparison normalization toggle ──
  normalizeComparison = false;

  readonly chartColors = CHART_COLORS;

  constructor() {
    const today = new Date();
    this.endDate = today;
    this.startDate = new Date(today.getFullYear() - 5, today.getMonth(), today.getDate());

    this.searchSubject
      .pipe(
        debounceTime(350),
        distinctUntilChanged((a, b) => a.query === b.query && a.basketId === b.basketId),
        switchMap(({ query, basketId }) => {
          if (query.trim().length < 2) return of({ results: [], basketId });
          this.loading.set(true);
          return this.fundService
            .searchFunds(query)
            .pipe(switchMap((results) => of({ results, basketId })));
        }),
        takeUntil(this.destroy$),
      )
      .subscribe({
        next: ({ results, basketId }) => {
          this.searchResults.set(results);
          this.searchingBasketId.set(basketId);
          this.loading.set(false);
        },
        error: () => this.loading.set(false),
      });
  }

  // ──────────────────────────────────────
  // Basket CRUD
  // ──────────────────────────────────────

  addBasket(): void {
    const label = String.fromCharCode(65 + this.baskets().length);
    this.baskets.update((b) => [...b, this.newBasket(`Basket ${label}`)]);
  }

  removeBasket(id: number): void {
    this.baskets.update((b) => b.filter((bk) => bk.id !== id));
    if (this.selectedSimBasketId() === id) {
      this.selectedSimBasketId.set(this.baskets()[0]?.id ?? null);
    }
  }

  private rebalanceBasket(basket: BasketSlot, triggerSchemeCode?: number): BasketSlot {
    const funds = basket.funds.map((f) => ({ ...f }));
    if (funds.length === 0) return { ...basket, funds };

    let lockedSum = 0;
    let unlockedCount = 0;

    for (const f of funds) {
      if (f.locked || f.fund.schemeCode === triggerSchemeCode) {
        lockedSum += f.allocationPercent || 0;
      } else {
        unlockedCount++;
      }
    }

    if (unlockedCount > 0) {
      const remaining = Math.max(0, 100 - lockedSum);
      const evenShare = remaining / unlockedCount;

      for (const f of funds) {
        if (!f.locked && f.fund.schemeCode !== triggerSchemeCode) {
          f.allocationPercent = parseFloat(evenShare.toFixed(2));
        }
      }

      // Fix rounding errors so the total exactly equals 100
      const currentSum = funds.reduce((s, f) => s + (f.allocationPercent || 0), 0);
      const diff = 100 - currentSum;
      if (Math.abs(diff) > 0.001) {
        const firstUnlocked = funds.find(
          (f) => !f.locked && f.fund.schemeCode !== triggerSchemeCode,
        );
        if (firstUnlocked) {
          firstUnlocked.allocationPercent = parseFloat(
            (firstUnlocked.allocationPercent + diff).toFixed(2),
          );
        }
      }
    }

    return { ...basket, funds };
  }

  addFundToBasket(basketId: number, fund: Fund): void {
    this.baskets.update((baskets) =>
      baskets.map((bk) => {
        if (bk.id !== basketId) return bk;
        if (bk.funds.some((f) => f.fund.schemeCode === fund.schemeCode)) return bk;

        const newBk = {
          ...bk,
          funds: [...bk.funds, { fund, allocationPercent: 0, locked: false }],
        };
        return this.rebalanceBasket(newBk);
      }),
    );
    this.searchResults.set([]);
    this.searchingBasketId.set(null);
  }

  removeFundFromBasket(basketId: number, schemeCode: number): void {
    this.baskets.update((baskets) =>
      baskets.map((bk) => {
        if (bk.id !== basketId) return bk;
        const newBk = {
          ...bk,
          funds: bk.funds.filter((f) => f.fund.schemeCode !== schemeCode),
        };
        return this.rebalanceBasket(newBk);
      }),
    );
  }

  onAllocationChange(basketId: number, schemeCode: number, value: number): void {
    this.baskets.update((baskets) =>
      baskets.map((bk) => {
        if (bk.id !== basketId) return bk;
        const newBk = {
          ...bk,
          funds: bk.funds.map((f) =>
            f.fund.schemeCode === schemeCode ? { ...f, allocationPercent: value } : f,
          ),
        };
        return this.rebalanceBasket(newBk, schemeCode);
      }),
    );
  }

  toggleFundLock(basketId: number, schemeCode: number): void {
    this.baskets.update((baskets) =>
      baskets.map((bk) => {
        if (bk.id !== basketId) return bk;
        return {
          ...bk,
          funds: bk.funds.map((f) =>
            f.fund.schemeCode === schemeCode ? { ...f, locked: !f.locked } : f,
          ),
        };
      }),
    );
  }

  allocationSum(basket: BasketSlot): number {
    return basket.funds.reduce((s, f) => s + (f.allocationPercent || 0), 0);
  }

  allocationValid(basket: BasketSlot): boolean {
    return Math.abs(this.allocationSum(basket) - 100) < 0.01;
  }

  onSearch(query: string, basketId: number): void {
    this.searchSubject.next({ query, basketId });
  }

  // ──────────────────────────────────────
  // Simulate (user-selected basket)
  // ──────────────────────────────────────

  simulate(): void {
    const simId = this.selectedSimBasketId() ?? this.baskets()[0]?.id;
    const basket = this.baskets().find((bk) => bk.id === simId) ?? this.baskets()[0];
    if (!basket) {
      this.error.set('No basket available.');
      return;
    }
    if (!this.validateBasket(basket)) return;
    this.loading.set(true);
    this.error.set(null);
    this.simResult.set(null);
    this.growthChartSeries.set([]);

    const req = {
      basket: this.toApiBasket(basket),
      sipAmount: basket.sipAmount || 0,
      sipDay: basket.sipDay || 1,
      lumpSumAmount: basket.lumpSumAmount || 0,
      startDate: this.toApiDate(this.startDate),
      endDate: this.toApiDate(this.endDate),
    };

    this.portfolioService
      .simulate(req)
      .pipe(finalize(() => this.loading.set(false)))
      .subscribe({
        next: (res) => {
          this.simResult.set(res);
          this.buildSimGrowthChart(res);
        },
        error: (err) => this.error.set(err.error?.message ?? 'Simulation failed'),
      });
  }

  // ──────────────────────────────────────
  // Compare (all baskets)
  // ──────────────────────────────────────

  compare(): void {
    if (this.baskets().length < 2) {
      this.error.set('Add at least 2 baskets to compare.');
      return;
    }
    for (const bk of this.baskets()) {
      if (!this.validateBasket(bk)) return;
    }
    this.loading.set(true);
    this.error.set(null);
    this.compareResult.set(null);
    this.growthChartSeries.set([]);

    const entries: ComparisonEntry[] = this.baskets().map((bk) => ({
      name: bk.name,
      sipAmount: bk.sipAmount || 0,
      sipDay: bk.sipDay || 1,
      lumpSumAmount: bk.lumpSumAmount || 0,
      basket: this.toApiBasket(bk),
    }));

    this.portfolioService
      .compare({
        entries,
        startDate: this.toApiDate(this.startDate),
        endDate: this.toApiDate(this.endDate),
        normalizeSipAmounts: this.normalizeComparison,
      })
      .pipe(finalize(() => this.loading.set(false)))
      .subscribe({
        next: (res) => {
          this.compareResult.set(res);
          this.buildCompareChart(res);
        },
        error: (err) => this.error.set(err.error?.message ?? 'Comparison failed'),
      });
  }

  // ──────────────────────────────────────
  // What-If (all baskets)
  // ──────────────────────────────────────

  whatIf(): void {
    if (this.baskets().length < 2) {
      this.error.set('Add at least 2 scenarios (baskets) for what-if analysis.');
      return;
    }
    for (const bk of this.baskets()) {
      if (!this.validateBasket(bk)) return;
    }
    this.loading.set(true);
    this.error.set(null);
    this.whatIfResult.set(null);

    this.portfolioService
      .whatIf({
        scenarios: this.baskets().map((bk) => ({
          name: bk.name,
          basket: this.toApiBasket(bk),
          sipAmount: bk.sipAmount || 0,
          sipDay: bk.sipDay || 1,
          lumpSumAmount: bk.lumpSumAmount || 0,
        })),
        startDate: this.toApiDate(this.startDate),
        endDate: this.toApiDate(this.endDate),
      })
      .pipe(finalize(() => this.loading.set(false)))
      .subscribe({
        next: (res) => this.whatIfResult.set(res),
        error: (err) => this.error.set(err.error?.message ?? 'What-if analysis failed'),
      });
  }

  // ──────────────────────────────────────
  // Rolling Comparison
  // ──────────────────────────────────────

  rollingComparison(): void {
    if (this.baskets().length < 2) {
      this.error.set('Add at least 2 baskets for rolling comparison.');
      return;
    }
    this.loading.set(true);
    this.error.set(null);
    this.rollingResult.set(null);
    this.rollingChartSeries.set([]);

    this.portfolioService
      .rollingComparison({
        entries: this.baskets().map((bk) => ({
          name: bk.name,
          sipAmount: 0,
          sipDay: 1,
          lumpSumAmount: 0,
          basket: this.toApiBasket(bk),
        })),
        periodInYears: this.rollingPeriod(),
        startDate: this.toApiDate(this.startDate),
        endDate: this.toApiDate(this.endDate),
        interval: 'monthly',
      })
      .pipe(finalize(() => this.loading.set(false)))
      .subscribe({
        next: (res) => {
          this.rollingResult.set(res);
          this.buildRollingChart(res);
        },
        error: (err) => this.error.set(err.error?.message ?? 'Rolling comparison failed'),
      });
  }

  // ──────────────────────────────────────
  // Chart builders
  // ──────────────────────────────────────

  private buildSimGrowthChart(res: SimulationResponse): void {
    if (!res.timeSeries.length) return;
    this.growthChartSeries.set([
      {
        name: `${res.basketName} — Portfolio Value`,
        data: res.timeSeries.map((pt) => ({
          time: toChartDate(pt.date),
          value: pt.portfolioValue,
        })),
      },
      {
        name: 'Amount Invested',
        data: res.timeSeries.map((pt) => ({
          time: toChartDate(pt.date),
          value: pt.totalInvested,
        })),
      },
    ]);
  }

  private buildCompareChart(res: ComparisonResponse): void {
    if (!res.timeSeries.length) return;
    const names = res.results.map((r) => r.name);
    this.growthChartSeries.set(
      names.map((name) => ({
        name,
        data: res.timeSeries
          .filter((pt) => pt.values.some((v) => v.name === name))
          .map((pt) => {
            const val = pt.values.find((v) => v.name === name);
            return {
              time: toChartDate(pt.date),
              value: this.normalizeComparison
                ? (val?.normalizedValue ?? 0)
                : (val?.portfolioValue ?? 0),
            };
          }),
      })),
    );
  }

  private buildRollingChart(res: RollingComparisonResponse): void {
    if (!res.timeSeries.length) return;
    const names = res.results.map((r) => r.name);
    this.rollingChartSeries.set(
      names.map((name) => ({
        name,
        data: res.timeSeries
          .map((pt) => {
            const val = pt.values.find((v) => v.name === name);
            return val?.returnPercent != null
              ? { time: toChartDate(pt.date), value: val.returnPercent }
              : null;
          })
          .filter((d): d is { time: string; value: number } => d !== null),
      })),
    );
  }

  // ──────────────────────────────────────
  // Helpers / validation
  // ──────────────────────────────────────

  private newBasket(name: string): BasketSlot {
    const bk: BasketSlot = {
      id: this.nextId++,
      name,
      funds: [],
      sipAmount: 5000,
      sipDay: 1,
      lumpSumAmount: 0,
      rebalanceFrequency: 'None',
    };
    // Auto-select first basket for simulate
    if (this.selectedSimBasketId() === null) {
      this.selectedSimBasketId.set(bk.id);
    }
    return bk;
  }

  private validateBasket(bk: BasketSlot): boolean {
    if (bk.funds.length === 0) {
      this.error.set(`${bk.name}: add at least one fund.`);
      return false;
    }
    if (!this.allocationValid(bk)) {
      this.error.set(
        `${bk.name}: allocations must sum to 100% (currently ${this.allocationSum(bk).toFixed(1)}%).`,
      );
      return false;
    }
    if ((bk.sipAmount || 0) === 0 && (bk.lumpSumAmount || 0) === 0) {
      this.error.set(`${bk.name}: SIP amount or lump sum must be greater than zero.`);
      return false;
    }
    return true;
  }

  private toApiBasket(bk: BasketSlot): BasketDefinition {
    return {
      name: bk.name,
      funds: bk.funds.map<BasketFundAllocation>((f) => ({
        schemeCode: f.fund.schemeCode,
        allocationPercent: f.allocationPercent,
      })),
      rebalanceFrequency: bk.rebalanceFrequency,
    };
  }

  private toApiDate(d: Date): string {
    const dd = String(d.getDate()).padStart(2, '0');
    const mm = String(d.getMonth() + 1).padStart(2, '0');
    return `${dd}-${mm}-${d.getFullYear()}`;
  }

  formatPct(v: number | null): string {
    return v == null ? 'N/A' : v.toFixed(2) + '%';
  }

  formatCr(v: number): string {
    if (v >= 1e7) return '₹' + (v / 1e7).toFixed(2) + ' Cr';
    if (v >= 1e5) return '₹' + (v / 1e5).toFixed(2) + ' L';
    return '₹' + v.toLocaleString('en-IN', { maximumFractionDigits: 0 });
  }

  pctClass(v: number | null): string {
    if (v == null) return '';
    return v >= 0 ? 'pos' : 'neg';
  }

  readonly simColumns = ['fund', 'allocation', 'invested', 'finalValue', 'return'];
  readonly compareColumns = [
    'name',
    'invested',
    'corpus',
    'absReturn',
    'cagr',
    'xirr',
    'drawdown',
    'volatility',
    'alpha',
  ];
  readonly whatIfColumns = ['name', 'sip', 'invested', 'corpus', 'cagr', 'xirr', 'efficiency'];
  readonly rollingColumns = ['name', 'min', 'max', 'avg', 'median', 'points'];

  ngOnDestroy(): void {
    this.destroy$.next();
    this.destroy$.complete();
  }
}
