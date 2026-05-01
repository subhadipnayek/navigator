import { Component, inject, signal, OnDestroy } from '@angular/core';
import { FormsModule } from '@angular/forms';
import { DecimalPipe } from '@angular/common';
import {
  Subject,
  debounceTime,
  distinctUntilChanged,
  switchMap,
  of,
  takeUntil,
  forkJoin,
} from 'rxjs';
import { FundService } from '../../services/fund.service';
import { NavService } from '../../services/nav.service';
import {
  Fund,
  ReturnsResponse,
  NavDataResponse,
  RollingReturnResponse,
} from '../../models/fund.model';
import { LineChart, ChartSeriesData } from '../../shared/line-chart/line-chart';
import { toChartDate } from '../../shared/date-utils';
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
import { MatTableModule } from '@angular/material/table';

@Component({
  selector: 'app-compare',
  imports: [
    FormsModule,
    LineChart,
    DecimalPipe,
    MatButtonModule,
    MatButtonToggleModule,
    MatCardModule,
    MatChipsModule,
    MatDatepickerModule,
    MatFormFieldModule,
    MatInputModule,
    MatListModule,
    MatNativeDateModule,
    MatProgressBarModule,
    MatTableModule,
  ],
  templateUrl: './compare.html',
})
export class Compare implements OnDestroy {
  private readonly fundService = inject(FundService);
  private readonly navService = inject(NavService);
  private readonly destroy$ = new Subject<void>();
  private readonly searchSubject = new Subject<string>();

  readonly searchResults = signal<Fund[]>([]);
  readonly selectedFunds = signal<Fund[]>([]);
  readonly returnsResults = signal<ReturnsResponse[]>([]);
  readonly growthChartSeries = signal<ChartSeriesData[]>([]);
  readonly rollingResults = signal<RollingReturnResponse[]>([]);
  readonly rollingChartSeries = signal<ChartSeriesData[]>([]);
  readonly loading = signal(false);
  readonly comparing = signal(false);
  readonly searchQuery = signal('');
  readonly error = signal<string | null>(null);
  readonly selectedRollingPeriod = signal(1);

  startDate: Date | null = null;
  endDate: Date | null = null;

  readonly rollingPeriods = [1, 2, 3, 5, 10];
  readonly growthPriceFormat = { type: 'price' as const, precision: 0, minMove: 1 };
  readonly percentFormat = { type: 'price' as const, precision: 2, minMove: 0.01 };

  constructor() {
    const today = new Date();
    const oneYearAgo = new Date();
    oneYearAgo.setFullYear(today.getFullYear() - 1);
    this.endDate = today;
    this.startDate = oneYearAgo;

    this.searchSubject
      .pipe(
        debounceTime(400),
        distinctUntilChanged(),
        switchMap((query) => {
          if (query.trim().length === 0) return of([]);
          this.loading.set(true);
          return this.fundService.searchFunds(query);
        }),
        takeUntil(this.destroy$),
      )
      .subscribe({
        next: (results) => {
          this.searchResults.set(results);
          this.loading.set(false);
        },
        error: () => this.loading.set(false),
      });
  }

  onSearchInput(query: string): void {
    this.searchQuery.set(query);
    this.searchSubject.next(query);
  }

  addFund(fund: Fund): void {
    if (this.selectedFunds().some((f) => f.schemeCode === fund.schemeCode)) return;
    if (this.selectedFunds().length >= 10) {
      this.error.set('Maximum 10 funds allowed.');
      return;
    }
    this.selectedFunds.update((list) => [...list, fund]);
    this.searchQuery.set('');
    this.searchResults.set([]);
    this.error.set(null);
  }

  removeFund(schemeCode: number): void {
    this.selectedFunds.update((list) => list.filter((f) => f.schemeCode !== schemeCode));
    this.clearResults();
  }

  compare(): void {
    const funds = this.selectedFunds();
    if (funds.length < 2) {
      this.error.set('Select at least 2 funds to compare.');
      return;
    }
    this.comparing.set(true);
    this.error.set(null);
    this.clearResults();
    const startDate = this.toApiDate(this.startDate);
    const endDate = this.toApiDate(this.endDate);

    // 1. Point-to-point returns for each fund
    forkJoin(funds.map((f) => this.navService.getReturns(f.schemeCode, startDate, endDate)))
      .subscribe({
        next: (results) => {
          this.returnsResults.set(results);
          this.comparing.set(false);
        },
        error: (err) => {
          this.error.set(err.error?.message ?? 'Comparison failed');
          this.comparing.set(false);
        },
      });

    // 2. Growth of ₹10,000 chart — fetch split-adjusted NAV history per fund with monthly interval
    forkJoin(
      funds.map((f) =>
        this.navService.getNavHistory(f.schemeCode, {
          from: startDate,
          to: endDate,
          interval: 'monthly',
          adjusted: true,
        }),
      ),
    ).subscribe({
      next: (navResults) => this.buildGrowthChart(navResults),
    });

    // 3. Rolling returns comparison
    this.loadRollingComparison(funds, this.selectedRollingPeriod());
  }

  selectRollingPeriod(period: number): void {
    this.selectedRollingPeriod.set(period);
    const funds = this.selectedFunds();
    if (funds.length >= 2 && this.returnsResults().length > 0) {
      this.loadRollingComparison(funds, period);
    }
  }

  private loadRollingComparison(funds: Fund[], period: number): void {
    forkJoin(
      funds.map((f) =>
        this.navService.getRollingReturns(f.schemeCode, period, {
          from: this.toApiDate(this.startDate),
          to: this.toApiDate(this.endDate),
          interval: 'monthly',
        }),
      ),
    ).subscribe({
      next: (results) => {
        this.rollingResults.set(results);
        this.rollingChartSeries.set(
          results.map((r) => ({
            name: r.schemeName,
            data: r.data.map((pt) => ({
              time: toChartDate(pt.date),
              value: pt.returnPercentage,
            })),
          })),
        );
      },
    });
  }

  private buildGrowthChart(navResults: NavDataResponse[]): void {
    // Find the common start date (latest first date across all funds)
    const firstDates = navResults.map((r) => {
      const sorted = [...r.data].sort((a, b) => {
        const [da, ma, ya] = a.date.split('-').map(Number);
        const [db, mb, yb] = b.date.split('-').map(Number);
        return ya * 10000 + ma * 100 + da - (yb * 10000 + mb * 100 + db);
      });
      return sorted.length > 0 ? sorted[0] : null;
    });

    if (firstDates.some((d) => d === null)) return;

    const toSortable = (ddmmyyyy: string) => {
      const [d, m, y] = ddmmyyyy.split('-');
      return `${y}-${m}-${d}`;
    };

    const commonStartSortable = firstDates
      .map((d) => toSortable(d!.date))
      .sort()
      .pop()!;

    this.growthChartSeries.set(
      navResults.map((nav) => {
        // Sort data oldest-first
        const sorted = [...nav.data].sort((a, b) =>
          toSortable(a.date).localeCompare(toSortable(b.date)),
        );

        // Filter from common start date
        const filtered = sorted.filter((pt) => toSortable(pt.date) >= commonStartSortable);

        if (filtered.length === 0) return { name: nav.schemeName, data: [] };

        const baseNav = filtered[0].nav;
        return {
          name: nav.schemeName,
          data: filtered.map((pt) => ({
            time: toChartDate(pt.date),
            value: Math.round((pt.nav / baseNav) * 10000),
          })),
        };
      }),
    );
  }

  private clearResults(): void {
    this.returnsResults.set([]);
    this.growthChartSeries.set([]);
    this.rollingResults.set([]);
    this.rollingChartSeries.set([]);
  }

  private toApiDate(d: Date | null): string {
    if (!d) return '';
    const year = d.getFullYear();
    const month = String(d.getMonth() + 1).padStart(2, '0');
    const day = String(d.getDate()).padStart(2, '0');
    return `${year}-${month}-${day}`;
  }

  formatReturn(value: number | null): string {
    if (value === null) return 'N/A';
    return value.toFixed(2) + '%';
  }

  returnClass(value: number | null): string {
    if (value === null) return '';
    return value >= 0 ? 'positive' : 'negative';
  }

  isAlreadySelected(schemeCode: number): boolean {
    return this.selectedFunds().some((f) => f.schemeCode === schemeCode);
  }

  ngOnDestroy(): void {
    this.destroy$.next();
    this.destroy$.complete();
  }
}
