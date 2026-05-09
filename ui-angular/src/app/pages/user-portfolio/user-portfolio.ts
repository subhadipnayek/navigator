import { DecimalPipe } from '@angular/common';
import { Component, OnDestroy, OnInit, computed, inject, signal } from '@angular/core';
import { FormsModule } from '@angular/forms';
import { AgGridAngular } from 'ag-grid-angular';
import {
  AllCommunityModule,
  CellValueChangedEvent,
  ColDef,
  GridApi,
  GridReadyEvent,
  ModuleRegistry,
} from 'ag-grid-community';
import { Subject, debounceTime, distinctUntilChanged, of, switchMap, takeUntil } from 'rxjs';
import { MatButtonModule } from '@angular/material/button';
import { MatButtonToggleModule } from '@angular/material/button-toggle';
import { MatAutocompleteModule } from '@angular/material/autocomplete';
import { MatCardModule } from '@angular/material/card';
import { MatCheckboxModule } from '@angular/material/checkbox';
import { MatFormFieldModule } from '@angular/material/form-field';
import { MatInputModule } from '@angular/material/input';
import { MatProgressBarModule } from '@angular/material/progress-bar';
import { MatSelectModule } from '@angular/material/select';
import { MatTableModule } from '@angular/material/table';
import { Fund } from '../../models/fund.model';
import {
  AllocationSlice,
  ContributionPeriod,
  PortfolioBenchmarkResponse,
  PortfolioTransaction,
  UserPortfolioDashboard,
  UserPortfolioSummary,
  YearTransactionSummary,
} from '../../models/portfolio.model';
import { FundService } from '../../services/fund.service';
import { PortfolioService } from '../../services/portfolio.service';
import { ToastService } from '../../shared/toast/toast.service';

ModuleRegistry.registerModules([AllCommunityModule]);

@Component({
  selector: 'app-user-portfolio',
  imports: [
    DecimalPipe,
    FormsModule,
    AgGridAngular,
    MatAutocompleteModule,
    MatButtonModule,
    MatButtonToggleModule,
    MatCardModule,
    MatCheckboxModule,
    MatFormFieldModule,
    MatInputModule,
    MatProgressBarModule,
    MatSelectModule,
    MatTableModule,
  ],
  templateUrl: './user-portfolio.html',
  styleUrl: './user-portfolio.scss',
})
export class UserPortfolio implements OnInit, OnDestroy {
  private readonly portfolioService = inject(PortfolioService);
  private readonly fundService = inject(FundService);
  private readonly toast = inject(ToastService);
  private readonly destroy$ = new Subject<void>();
  private readonly fundSearch$ = new Subject<string>();

  private gridApi: GridApi<PortfolioTransaction> | null = null;

  readonly portfolios = signal<UserPortfolioSummary[]>([]);
  readonly selectedPortfolioId = signal<number | null>(null);
  readonly dashboard = signal<UserPortfolioDashboard | null>(null);
  readonly transactions = signal<PortfolioTransaction[]>([]);
  readonly yearSummary = signal<YearTransactionSummary>({
    totalInvested: 0,
    transactionCount: 0,
    fundsCovered: 0,
  });
  readonly loading = signal(false);
  readonly saving = signal(false);
  readonly showFullyExitedFunds = signal(false);
  readonly selectedFinancialYear = signal(this.currentFinancialYear());
  readonly contributionMode = signal<'financial' | 'calendar'>('financial');
  readonly fundSearch = signal('');
  readonly fundResults = signal<Fund[]>([]);
  readonly selectedFund = signal<Fund | null>(null);
  readonly benchmarkSearch = signal('');
  readonly benchmarkResults = signal<Fund[]>([]);
  readonly selectedBenchmarkFund = signal<Fund | null>(null);
  readonly benchmark = signal<PortfolioBenchmarkResponse | null>(null);

  readonly contributionRows = computed<ContributionPeriod[]>(() => {
    const dashboard = this.dashboard();
    if (!dashboard) return [];
    return this.contributionMode() === 'financial'
      ? dashboard.financialYearContributions
      : dashboard.calendarYearContributions;
  });

  readonly transactionColumnDefs: ColDef<PortfolioTransaction>[] = [
    { field: 'transactionDate', headerName: 'Date', editable: true, width: 130 },
    {
      field: 'transactionType',
      headerName: 'Type',
      editable: true,
      width: 120,
      cellEditor: 'agSelectCellEditor',
      cellEditorParams: { values: ['Buy', 'Sell'] },
    },
    { field: 'units', headerName: 'Units', editable: true, width: 120, valueParser: numberParser },
    { field: 'nav', headerName: 'NAV', editable: true, width: 120, valueParser: numberParser },
    { field: 'amount', headerName: 'Amount', editable: true, width: 130, valueParser: numberParser },
    { field: 'notes', headerName: 'Notes', editable: true, flex: 1, minWidth: 180 },
    {
      field: 'error',
      headerName: 'Error',
      minWidth: 240,
      cellClass: (params) => (params.value ? 'grid-error-cell' : ''),
    },
  ];

  readonly defaultColDef: ColDef = {
    sortable: true,
    resizable: true,
    filter: true,
  };

  ngOnInit(): void {
    this.loadPortfolios();
    this.fundSearch$
      .pipe(
        debounceTime(250),
        distinctUntilChanged(),
        switchMap((query) => (query.trim().length >= 2 ? this.fundService.searchFunds(query) : of([]))),
        takeUntil(this.destroy$),
      )
      .subscribe((funds) => {
        this.fundResults.set(funds);
        this.benchmarkResults.set(funds);
      });
  }

  ngOnDestroy(): void {
    this.destroy$.next();
    this.destroy$.complete();
  }

  onGridReady(event: GridReadyEvent<PortfolioTransaction>): void {
    this.gridApi = event.api;
  }

  loadPortfolios(): void {
    this.loading.set(true);
    this.portfolioService.getUserPortfolios().subscribe({
      next: (rows) => {
        this.portfolios.set(rows);
        if (rows.length === 0) {
          this.createPortfolio('My Portfolio');
          return;
        }
        this.selectedPortfolioId.set(rows[0].id);
        this.loadDashboard();
        this.loadTransactions();
      },
      error: () => {
        this.loading.set(false);
        this.toast.error('Unable to load portfolios.');
      },
    });
  }

  createPortfolio(name: string): void {
    this.portfolioService.createUserPortfolio(name).subscribe({
      next: (portfolio) => {
        this.portfolios.set([portfolio, ...this.portfolios()]);
        this.selectedPortfolioId.set(portfolio.id);
        this.loading.set(false);
        this.loadDashboard();
        this.loadTransactions();
      },
      error: () => {
        this.loading.set(false);
        this.toast.error('Unable to create portfolio.');
      },
    });
  }

  onPortfolioChange(portfolioId: number): void {
    this.selectedPortfolioId.set(Number(portfolioId));
    this.loadDashboard();
    this.loadTransactions();
  }

  onYearChange(year: string): void {
    this.selectedFinancialYear.set(year);
    this.loadTransactions();
  }

  toggleExited(value: boolean): void {
    this.showFullyExitedFunds.set(value);
    this.loadDashboard();
  }

  loadDashboard(): void {
    const portfolioId = this.selectedPortfolioId();
    if (!portfolioId) return;
    this.loading.set(true);
    this.portfolioService.getUserPortfolioDashboard(portfolioId, this.showFullyExitedFunds()).subscribe({
      next: (dashboard) => {
        this.dashboard.set(dashboard);
        this.loading.set(false);
      },
      error: () => {
        this.loading.set(false);
        this.toast.error('Unable to load portfolio dashboard.');
      },
    });
  }

  loadTransactions(): void {
    const portfolioId = this.selectedPortfolioId();
    if (!portfolioId) return;
    this.portfolioService
      .getUserPortfolioTransactions(portfolioId, this.selectedFinancialYear())
      .subscribe({
        next: (response) => {
          this.transactions.set(response.transactions.map(ensureClientRowId));
          this.yearSummary.set(response.summary);
        },
        error: () => this.toast.error('Unable to load transactions for the selected year.'),
      });
  }

  onFundSearch(query: string): void {
    this.fundSearch.set(query);
    this.fundSearch$.next(query);
  }

  selectFund(fund: Fund): void {
    this.selectedFund.set(fund);
    this.fundSearch.set(`${fund.schemeCode} - ${fund.schemeName}`);
  }

  addRow(): void {
    const fund = this.selectedFund();
    if (!fund) {
      this.toast.info('Select a fund from the database before adding a row.');
      return;
    }
    const row = ensureClientRowId({
      id: null,
      clientRowId: null,
      schemeCode: fund.schemeCode,
      schemeName: fund.schemeName,
      transactionDate: this.yearStartDate(),
      transactionType: 'Buy',
      units: 0,
      nav: 0,
      amount: 0,
      notes: null,
      error: null,
    });
    this.transactions.set([...this.collectGridRows(), row]);
    this.recomputeSummary();
  }

  deleteSelectedRows(): void {
    const selected = new Set((this.gridApi?.getSelectedRows() ?? []).map((r) => r.clientRowId));
    if (selected.size === 0) return;
    this.transactions.set(this.collectGridRows().filter((r) => !selected.has(r.clientRowId)));
    this.recomputeSummary();
  }

  onCellValueChanged(event: CellValueChangedEvent<PortfolioTransaction>): void {
    const row = event.data;
    if ((event.colDef.field === 'units' || event.colDef.field === 'nav') && row.units > 0 && row.nav > 0) {
      row.amount = round2(row.units * row.nav);
      event.api.refreshCells({ rowNodes: [event.node], columns: ['amount'] });
    }
    this.transactions.set(this.collectGridRows());
    this.recomputeSummary();
  }

  onPaste(event: ClipboardEvent): void {
    const text = event.clipboardData?.getData('text/plain');
    if (!text?.trim()) return;
    event.preventDefault();
    const pastedRows = this.parseClipboard(text);
    if (pastedRows.length === 0) {
      this.toast.info('Paste did not contain recognizable transaction rows.');
      return;
    }
    this.transactions.set([...this.collectGridRows(), ...pastedRows]);
    this.recomputeSummary();
    this.toast.success(`${pastedRows.length} pasted rows added.`);
  }

  saveTransactions(): void {
    const portfolioId = this.selectedPortfolioId();
    if (!portfolioId) return;
    this.saving.set(true);
    this.portfolioService
      .saveUserPortfolioTransactions(portfolioId, this.selectedFinancialYear(), this.collectGridRows())
      .subscribe({
        next: (response) => {
          this.transactions.set(response.transactions.map(ensureClientRowId));
          this.yearSummary.set(response.summary);
          this.saving.set(false);
          this.toast.success(response.message ?? 'Transactions saved.');
          this.loadDashboard();
        },
        error: (err) => {
          const response = err.error;
          if (response?.transactions) {
            this.transactions.set(response.transactions.map(ensureClientRowId));
            this.yearSummary.set(response.summary);
          }
          this.saving.set(false);
          this.toast.error(response?.message ?? 'Fix row errors before saving.');
        },
      });
  }

  cancelTransactions(): void {
    this.loadTransactions();
  }

  selectBenchmarkFund(fund: Fund): void {
    this.selectedBenchmarkFund.set(fund);
    this.benchmarkSearch.set(`${fund.schemeCode} - ${fund.schemeName}`);
  }

  runBenchmark(): void {
    const portfolioId = this.selectedPortfolioId();
    const fund = this.selectedBenchmarkFund();
    if (!portfolioId || !fund) {
      this.toast.info('Select a benchmark fund first.');
      return;
    }
    this.portfolioService
      .compareUserPortfolioBenchmark(portfolioId, {
        benchmarkType: 'fund',
        schemeCode: fund.schemeCode,
      })
      .subscribe({
        next: (result) => this.benchmark.set(result),
        error: () => this.toast.error('Unable to compare benchmark.'),
      });
  }

  financialYears(): string[] {
    const current = Number(this.currentFinancialYear().slice(0, 4));
    return Array.from({ length: 12 }, (_, index) => {
      const year = current - index;
      return `${year}-${(year + 1) % 100}`;
    });
  }

  maxSlicePercent(rows: AllocationSlice[]): number {
    return Math.max(1, ...rows.map((r) => r.percent));
  }

  private collectGridRows(): PortfolioTransaction[] {
    const rows: PortfolioTransaction[] = [];
    this.gridApi?.forEachNode((node) => {
      if (node.data) rows.push(ensureClientRowId(node.data));
    });
    return rows.length ? rows : this.transactions();
  }

  private recomputeSummary(): void {
    const rows = this.collectGridRows();
    this.yearSummary.set({
      totalInvested: rows.filter((r) => isBuy(r.transactionType)).reduce((sum, r) => sum + Number(r.amount || 0), 0),
      transactionCount: rows.length,
      fundsCovered: new Set(rows.map((r) => r.schemeCode).filter(Boolean)).size,
    });
  }

  private parseClipboard(text: string): PortfolioTransaction[] {
    const fund = this.selectedFund();
    if (!fund) {
      this.toast.info('Select the fund these pasted transactions belong to first.');
      return [];
    }

    const lines = text.trim().split(/\r?\n/).filter(Boolean);
    const cells = lines.map((line) => line.split('\t').map((cell) => cell.trim()));
    if (cells.length === 0) return [];
    const headerMap = detectHeader(cells[0]);
    const startIndex = headerMap ? 1 : 0;
    const map = headerMap ?? defaultColumnMap();

    return cells.slice(startIndex).map((row) => {
      const units = Number(row[map['units']] ?? 0);
      const nav = Number(row[map['nav']] ?? 0);
      const amount = Number(row[map['amount']] ?? units * nav);
      return ensureClientRowId({
        id: null,
        clientRowId: null,
        schemeCode: fund.schemeCode,
        schemeName: fund.schemeName,
        transactionDate: normalizeDate(row[map['transactionDate']] ?? this.yearStartDate()),
        transactionType: normalizeType(row[map['transactionType']] ?? 'Buy'),
        units,
        nav,
        amount: round2(amount),
        notes: row[map['notes']] ?? null,
        error: null,
      });
    });
  }

  private currentFinancialYear(): string {
    const today = new Date();
    const startYear = today.getMonth() >= 3 ? today.getFullYear() : today.getFullYear() - 1;
    return `${startYear}-${String((startYear + 1) % 100).padStart(2, '0')}`;
  }

  private yearStartDate(): string {
    return `${this.selectedFinancialYear().slice(0, 4)}-04-01`;
  }
}

function numberParser(params: { newValue: unknown }): number {
  return Number(params.newValue) || 0;
}

function ensureClientRowId(row: PortfolioTransaction): PortfolioTransaction {
  return { ...row, clientRowId: row.clientRowId ?? crypto.randomUUID() };
}

function isBuy(type: string): boolean {
  return ['buy', 'purchase', 'sip', 'switchin'].includes(type.toLowerCase());
}

function normalizeType(type: string): string {
  return ['sell', 'redeem', 'redemption', 'switchout'].includes(type.toLowerCase()) ? 'Sell' : 'Buy';
}

function round2(value: number): number {
  return Math.round((Number(value) + Number.EPSILON) * 100) / 100;
}

function normalizeDate(value: string): string {
  const ddmmyyyy = value.match(/^(\d{1,2})[-/](\d{1,2})[-/](\d{4})$/);
  if (ddmmyyyy) {
    return `${ddmmyyyy[3]}-${ddmmyyyy[2].padStart(2, '0')}-${ddmmyyyy[1].padStart(2, '0')}`;
  }
  return value;
}

function detectHeader(row: string[]): Record<string, number> | null {
  const aliases: Record<string, string[]> = {
    transactionDate: ['date', 'transaction date', 'txn date'],
    transactionType: ['type', 'transaction type', 'txn type'],
    units: ['units', 'unit'],
    nav: ['nav', 'price'],
    amount: ['amount', 'value'],
    notes: ['notes', 'remarks'],
  };
  const normalized = row.map((cell) => cell.toLowerCase());
  const map: Record<string, number> = {};
  for (const [field, names] of Object.entries(aliases)) {
    const index = normalized.findIndex((cell) => names.includes(cell));
      if (index >= 0) map[field] = index;
  }
  return map['transactionDate'] !== undefined ? map : null;
}

function defaultColumnMap(): Record<string, number> {
  return {
    transactionDate: 0,
    transactionType: 1,
    units: 2,
    nav: 3,
    amount: 4,
    notes: 5,
  };
}
