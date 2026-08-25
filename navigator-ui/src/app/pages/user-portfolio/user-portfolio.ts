import { DatePipe, DecimalPipe } from '@angular/common';
import { Component, OnDestroy, OnInit, computed, inject, signal, ChangeDetectionStrategy } from '@angular/core';
import { FormsModule } from '@angular/forms';
import { AgGridAngular } from 'ag-grid-angular';
import * as XLSX from 'xlsx';
import {
  AllCommunityModule,
  CellValueChangedEvent,
  ColDef,
  Column,
  ColumnState,
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
  ImportCommitRequest,
  ImportPortfolioTransactionsResponse,
  ImportPreviewResult,
  ImportStrategy,
  PortfolioBenchmarkResponse,
  PortfolioTransaction,
  ProcessedTradeRecord,
  UserPortfolioDashboard,
  UserPortfolioSummary,
  YearTransactionSummary,
} from '../../models/portfolio.model';
import { FundService } from '../../services/fund.service';
import { PortfolioService } from '../../services/portfolio.service';
import { ToastService } from '../../shared/toast/toast.service';

ModuleRegistry.registerModules([AllCommunityModule]);

type TransactionGridField = 'transactionDate' | 'transactionType' | 'units' | 'nav' | 'notes';

interface PersistedGridState {
  columnState: ColumnState[];
  filterModel: unknown;
}

@Component({
  selector: 'app-user-portfolio',
  changeDetection: ChangeDetectionStrategy.OnPush,
  imports: [
    DatePipe,
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
  private readonly gridStateChanged$ = new Subject<void>();
  private readonly transactionAutosave$ = new Subject<void>();
  private readonly transactionGridKey = 'portfolio-transactions';

  private gridApi: GridApi<PortfolioTransaction> | null = null;
  private applyingGridState = false;

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
  readonly importingExcel = signal(false);
  readonly selectedExcelFile = signal<File | null>(null);
  readonly importMode = signal<'Append' | 'Replace'>('Append');
  readonly importStartDate = signal('');
  readonly importEndDate = signal('');
  readonly importSummary = signal<ImportPortfolioTransactionsResponse | null>(null);
  readonly showTransactionEditor = signal(false);
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
  readonly benchmarkDurationYears = signal<number | null>(null);
  readonly benchmarkRollingPeriodYears = signal<number>(3);
  readonly excelImportMessages = signal<string[]>([]);

  // Multi-broker Ingestion Workbench Signals
  readonly dryRunPreview = signal<ImportPreviewResult | null>(null);
  readonly selectedStrategy = signal<ImportStrategy>('SmartMerge');
  readonly previewFilterTab = signal<'all' | 'new' | 'duplicate' | 'ignored'>('all');
  readonly previewingFile = signal<boolean>(false);
  readonly committingImport = signal<boolean>(false);

  readonly filteredPreviewTrades = computed<ProcessedTradeRecord[]>(() => {
    const preview = this.dryRunPreview();
    if (!preview) return [];
    const tab = this.previewFilterTab();
    if (tab === 'all') return preview.transactions;
    if (tab === 'new') return preview.transactions.filter((t) => t.status === 'New');
    if (tab === 'duplicate') return preview.transactions.filter((t) => t.status === 'Duplicate');
    if (tab === 'ignored') return preview.transactions.filter((t) => t.status === 'Ignored');
    return preview.transactions;
  });

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
      field: 'schemeName',
      headerName: 'Fund',
      editable: false,
      minWidth: 260,
      flex: 1,
      tooltipField: 'schemeName',
    },
    { field: 'nav', headerName: 'NAV', editable: true, width: 120, valueParser: numberParser },
    { field: 'units', headerName: 'Units', editable: true, width: 120, valueParser: numberParser },
    {
      field: 'amount',
      headerName: 'Amount',
      editable: false,
      width: 130,
      valueFormatter: (params) => Number(params.value ?? 0).toFixed(2),
    },
    {
      field: 'transactionType',
      headerName: 'Type',
      editable: true,
      width: 120,
      cellEditor: 'agSelectCellEditor',
      cellEditorParams: { values: ['Buy', 'Sell'] },
    },
    { field: 'notes', headerName: 'Notes', editable: true, flex: 1, minWidth: 180 },
    {
      field: 'error',
      headerName: 'Error',
      minWidth: 240,
      tooltipField: 'error',
      cellClass: (params) => (params.value ? 'grid-error-cell' : ''),
    },
  ];

  readonly defaultColDef: ColDef = {
    sortable: true,
    resizable: true,
    filter: true,
    tooltipValueGetter: (params) => {
      if (params.value === null || params.value === undefined || params.value === '') return null;
      return String(params.value);
    },
  };

  private readonly pasteFields: TransactionGridField[] = [
    'transactionDate',
    'nav',
    'units',
    'transactionType',
    'notes',
  ];

  readonly gridErrors = computed(() =>
    this.transactions()
      .map((row, index) => ({ rowNumber: index + 1, fund: row.schemeName, message: row.error }))
      .filter((row): row is { rowNumber: number; fund: string; message: string } => !!row.message),
  );

  readonly canImportExcel = computed(() => {
    const start = this.importStartDate();
    const end = this.importEndDate();
    return !!this.selectedExcelFile() && !!start && !!end && end >= start && !this.importingExcel();
  });

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

    this.gridStateChanged$
      .pipe(debounceTime(700), takeUntil(this.destroy$))
      .subscribe(() => this.saveGridState());

    this.transactionAutosave$
      .pipe(debounceTime(900), takeUntil(this.destroy$))
      .subscribe(() => this.saveTransactions());
  }

  ngOnDestroy(): void {
    this.destroy$.next();
    this.destroy$.complete();
  }

  onGridReady(event: GridReadyEvent<PortfolioTransaction>): void {
    this.gridApi = event.api;
    this.restoreGridState();
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
    const row = this.createBlankTransaction(fund);
    this.transactions.set([...this.collectGridRows(), row]);
    this.recomputeSummary();
  }

  deleteSelectedRows(): void {
    const selected = new Set((this.gridApi?.getSelectedRows() ?? []).map((r) => r.clientRowId));
    if (selected.size === 0) return;
    this.transactions.set(this.collectGridRows().filter((r) => !selected.has(r.clientRowId)));
    this.recomputeSummary();
    this.scheduleTransactionAutosave();
  }

  downloadExcelTemplate(): void {
    const headers = [
      'TradeDate',
      'TradeType',
      'Price',
      'Quantity',
      'SchemeCode',
      'ISIN',
      'SchemeName',
      'Notes',
    ];
    const sampleRows = [
      {
        TradeDate: '2024-04-01',
        TradeType: 'Buy',
        Price: 25.34,
        Quantity: 100,
        SchemeCode: '120503',
        ISIN: 'INF209K01UN8',
        SchemeName: 'Axis Bluechip Fund - Direct Growth',
        Notes: 'SIP April',
      },
      {
        TradeDate: '2024-09-15',
        TradeType: 'Sell',
        Price: 31.20,
        Quantity: 50,
        SchemeCode: '120503',
        ISIN: 'INF209K01UN8',
        SchemeName: 'Axis Bluechip Fund - Direct Growth',
        Notes: '',
      },
    ];

    const worksheet = XLSX.utils.json_to_sheet(sampleRows, { header: headers });

    // Style the header row with a wider column width for readability
    const columnWidths = [14, 12, 10, 10, 14, 16, 45, 20];
    worksheet['!cols'] = columnWidths.map((wch) => ({ wch }));

    const workbook = XLSX.utils.book_new();
    XLSX.utils.book_append_sheet(workbook, worksheet, 'Transactions');
    XLSX.writeFile(workbook, 'transaction-template.xlsx');
  }

  onStatementFileSelected(event: Event): void {
    const input = event.target as HTMLInputElement;
    const file = input.files?.[0];
    input.value = '';
    if (!file) return;
    this.executeDryRunPreview(file);
  }

  onStatementFileDropped(event: DragEvent): void {
    event.preventDefault();
    const file = event.dataTransfer?.files?.[0];
    if (!file) return;
    this.executeDryRunPreview(file);
  }

  executeDryRunPreview(file: File): void {
    const portfolioId = this.selectedPortfolioId();
    if (!portfolioId) {
      this.toast.error('Select a portfolio first.');
      return;
    }

    this.previewingFile.set(true);
    this.portfolioService.importPreview(portfolioId, file).subscribe({
      next: (res) => {
        this.dryRunPreview.set(res);
        this.selectedStrategy.set(res.recommendedStrategy ?? 'SmartMerge');
        this.previewFilterTab.set('all');
        this.previewingFile.set(false);
        this.toast.info(`Parsed ${res.totalRowsParsed} rows via ${res.detectedFormatLabel}`);
      },
      error: (err) => {
        this.previewingFile.set(false);
        this.toast.error(err.error?.message ?? 'Failed to parse trade statement file.');
      },
    });
  }

  commitIngestion(): void {
    const portfolioId = this.selectedPortfolioId();
    const preview = this.dryRunPreview();
    if (!portfolioId || !preview) return;

    this.committingImport.set(true);
    const req: ImportCommitRequest = {
      strategy: this.selectedStrategy(),
      transactions: preview.transactions,
    };

    this.portfolioService.importCommit(portfolioId, req, this.selectedFinancialYear()).subscribe({
      next: (res) => {
        this.committingImport.set(false);
        this.toast.success(`Successfully imported ${res.insertedCount} transactions.`);
        this.dryRunPreview.set(null);
        this.loadDashboard();
        this.loadTransactions();
      },
      error: (err) => {
        this.committingImport.set(false);
        this.toast.error(err.error?.message ?? 'Transaction commit failed.');
      },
    });
  }

  closeDryRunModal(): void {
    this.dryRunPreview.set(null);
  }

  onExcelFileSelected(event: Event): void {
    const input = event.target as HTMLInputElement;
    const file = input.files?.[0];
    input.value = '';
    if (!file) return;
    this.selectedExcelFile.set(file);
    this.importSummary.set(null);
    this.excelImportMessages.set([]);
  }

  importExcelFile(): void {
    const portfolioId = this.selectedPortfolioId();
    const file = this.selectedExcelFile();
    const startDate = this.importStartDate();
    const endDate = this.importEndDate();
    if (!portfolioId || !file) return;
    if (!startDate || !endDate || endDate < startDate) {
      this.toast.info('Select a valid import date range before uploading.');
      return;
    }

    if (this.importMode() === 'Replace') {
      this.portfolioService.getImportDeletePreview(portfolioId, startDate, endDate).subscribe({
        next: (preview) => {
          const confirmed = window.confirm(
            `Replace transactions from ${startDate} to ${endDate}?\n\n${preview.deleteCount} existing transaction${preview.deleteCount === 1 ? '' : 's'} will be deleted before importing.`,
          );
          if (confirmed) this.submitExcelImport(portfolioId, file, startDate, endDate);
        },
        error: () => this.toast.error('Unable to preview transactions for the selected range.'),
      });
      return;
    }

    this.submitExcelImport(portfolioId, file, startDate, endDate);
  }

  onCellValueChanged(event: CellValueChangedEvent<PortfolioTransaction>): void {
    const row = event.data;
    if ((event.colDef.field === 'units' || event.colDef.field === 'nav') && row.units > 0 && row.nav > 0) {
      row.amount = round2(row.units * row.nav);
      event.api.refreshCells({ rowNodes: [event.node], columns: ['amount'] });
    }
    this.transactions.set(this.collectGridRows());
    this.recomputeSummary();
    this.scheduleTransactionAutosave();
  }

  onPaste(event: ClipboardEvent): void {
    const text = event.clipboardData?.getData('text/plain');
    if (!text?.trim()) return;
    event.preventDefault();
    this.applyClipboardToGrid(text);
  }

  onGridStateChanged(): void {
    if (!this.applyingGridState) {
      this.gridStateChanged$.next();
    }
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
          this.gridApi?.refreshCells({ force: true });
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
        durationYears: this.benchmarkDurationYears(),
        rollingPeriodYears: this.benchmarkRollingPeriodYears(),
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

  private applyClipboardToGrid(text: string): void {
    const focusedCell = this.gridApi?.getFocusedCell();
    const startRowIndex = focusedCell?.rowIndex ?? 0;
    const startField = this.toPasteField(focusedCell?.column) ?? 'transactionDate';
    const visiblePasteFields = this.getVisiblePasteFields();
    const startFieldIndex = Math.max(0, visiblePasteFields.indexOf(startField));
    const matrix = text
      .replace(/\r$/, '')
      .split(/\r?\n/)
      .filter((line) => line.length > 0)
      .map((line) => line.split('\t'));

    if (matrix.length === 0) {
      this.toast.info('Paste did not contain any cells.');
      return;
    }

    const rows = this.collectGridRows();
    const templateRow = rows[startRowIndex] ?? rows.at(-1) ?? null;
    const selectedFund = this.selectedFund();
    if (!templateRow && !selectedFund) {
      this.toast.info('Select a fund before pasting into an empty grid.');
      return;
    }

    const requiredRows = startRowIndex + matrix.length;
    while (rows.length < requiredRows) {
      rows.push(this.createBlankTransaction(selectedFund, templateRow));
    }

    let rejectedCells = 0;
    const touchedRows = new Set<number>();
    matrix.forEach((pastedRow, pasteRowIndex) => {
      const rowIndex = startRowIndex + pasteRowIndex;
      const targetRow = rows[rowIndex];
      const changedFields = new Set<TransactionGridField>();
      let rowHadError = false;

      pastedRow.forEach((rawValue, pasteColumnIndex) => {
        const field = visiblePasteFields[startFieldIndex + pasteColumnIndex];
        if (!field) return;

        const parsed = parsePastedCell(field, rawValue);
        if (!parsed.ok) {
          targetRow.error = parsed.error;
          rejectedCells++;
          rowHadError = true;
          return;
        }

        applyParsedCell(targetRow, field, parsed.value);
        changedFields.add(field);
      });

      if (
        (changedFields.has('units') || changedFields.has('nav')) &&
        targetRow.units > 0 &&
        targetRow.nav > 0
      ) {
        targetRow.amount = round2(targetRow.units * targetRow.nav);
      }

      if (!rowHadError) targetRow.error = null;
      touchedRows.add(rowIndex);
    });

    this.transactions.set(rows);
    this.recomputeSummary();
    this.gridApi?.refreshCells({ force: true });
    this.scheduleTransactionAutosave();

    const pastedCells = matrix.reduce((sum, row) => sum + row.length, 0);
    if (rejectedCells > 0) {
      this.toast.error(`${rejectedCells} of ${pastedCells} pasted cells did not match the target column type.`);
    } else {
      this.toast.success(`${touchedRows.size} row${touchedRows.size === 1 ? '' : 's'} updated from paste.`);
    }
  }

  private currentFinancialYear(): string {
    const today = new Date();
    const startYear = today.getMonth() >= 3 ? today.getFullYear() : today.getFullYear() - 1;
    return `${startYear}-${String((startYear + 1) % 100).padStart(2, '0')}`;
  }

  private yearStartDate(): string {
    return `${this.selectedFinancialYear().slice(0, 4)}-04-01`;
  }

  private createBlankTransaction(fund: Fund | null, templateRow: PortfolioTransaction | null = null): PortfolioTransaction {
    return ensureClientRowId({
      id: null,
      clientRowId: null,
      schemeCode: fund?.schemeCode ?? templateRow?.schemeCode ?? 0,
      schemeName: fund?.schemeName ?? templateRow?.schemeName ?? '',
      transactionDate: this.yearStartDate(),
      transactionType: 'Buy',
      units: 0,
      nav: 0,
      amount: 0,
      notes: null,
      error: null,
    });
  }

  private toPasteField(column: Column | null | undefined): TransactionGridField | null {
    const colId = column?.getColId();
    return this.pasteFields.includes(colId as TransactionGridField)
      ? (colId as TransactionGridField)
      : null;
  }

  private getVisiblePasteFields(): TransactionGridField[] {
    const columns = this.gridApi?.getAllDisplayedColumns?.() ?? [];
    const visibleFields = columns
      .map((column) => this.toPasteField(column))
      .filter((field): field is TransactionGridField => field !== null);
    return visibleFields.length > 0 ? visibleFields : this.pasteFields;
  }

  private restoreGridState(): void {
    if (!this.gridApi) return;

    this.portfolioService.getGridPreference(this.transactionGridKey).subscribe({
      next: (preference) => {
        if (!preference.stateJson || !this.gridApi) return;
        const state = parseGridState(preference.stateJson);
        if (!state) return;

        this.applyingGridState = true;
        this.gridApi.applyColumnState({
          state: state.columnState,
          applyOrder: true,
        });
        this.gridApi.setFilterModel(state.filterModel ?? null);
        this.applyingGridState = false;
      },
      error: () => {
        this.applyingGridState = false;
      },
    });
  }

  private saveGridState(): void {
    if (!this.gridApi) return;

    const state: PersistedGridState = {
      columnState: this.gridApi.getColumnState(),
      filterModel: this.gridApi.getFilterModel(),
    };

    this.portfolioService
      .saveGridPreference(this.transactionGridKey, JSON.stringify(state))
      .subscribe({ error: () => this.toast.error('Unable to save grid layout.') });
  }

  private scheduleTransactionAutosave(): void {
    this.transactionAutosave$.next();
  }

  private submitExcelImport(portfolioId: number, file: File, startDate: string, endDate: string): void {
    this.importingExcel.set(true);
    this.excelImportMessages.set([]);
    this.importSummary.set(null);
    this.portfolioService
      .importUserPortfolioTransactions(portfolioId, this.selectedFinancialYear(), this.importMode(), startDate, endDate, file)
      .subscribe({
        next: (response) => {
          this.importSummary.set(response);
          this.excelImportMessages.set(response.rowErrors);
          this.transactions.set(response.transactions.map(ensureClientRowId));
          this.yearSummary.set(response.summary);
          this.importingExcel.set(false);
          this.toast.success(response.message ?? `${response.importedCount} Excel transactions imported.`);
          this.loadDashboard();
        },
        error: (err) => {
          const response = err.error;
          if (response?.transactions) {
            this.transactions.set(response.transactions.map(ensureClientRowId));
            this.yearSummary.set(response.summary);
          }
          if (response?.rowErrors) {
            this.importSummary.set(response);
          }
          this.excelImportMessages.set(response?.rowErrors ?? [response?.message ?? 'Unable to import the Excel file.']);
          this.importingExcel.set(false);
          this.toast.error(response?.message ?? 'Unable to import the Excel file.');
        },
      });
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

function parsePastedCell(
  field: TransactionGridField,
  rawValue: string,
): { ok: true; value: string | number | null } | { ok: false; error: string } {
  const value = rawValue.trim();
  switch (field) {
    case 'transactionDate': {
      const date = parseDateCell(value);
      return date
        ? { ok: true, value: date }
        : { ok: false, error: `"${rawValue}" is not a valid date.` };
    }
    case 'transactionType': {
      const type = parseTransactionType(value);
      return type
        ? { ok: true, value: type }
        : { ok: false, error: `"${rawValue}" is not a valid transaction type.` };
    }
    case 'units':
    case 'nav': {
      const amount = parseNumberCell(value);
      return amount !== null
        ? { ok: true, value: amount }
        : { ok: false, error: `"${rawValue}" is not a valid number.` };
    }
    case 'notes':
      return { ok: true, value: value.length ? value : null };
  }
}

function applyParsedCell(
  row: PortfolioTransaction,
  field: TransactionGridField,
  value: string | number | null,
): void {
  if (field === 'notes') {
    row.notes = value === null ? null : String(value);
    return;
  }
  if (field === 'transactionDate' || field === 'transactionType') {
    row[field] = String(value);
    return;
  }
  row[field] = Number(value);
}

function parseDateCell(value: string): string | null {
  const normalized = normalizeDate(value);
  const date = new Date(`${normalized}T00:00:00`);
  if (!/^\d{4}-\d{2}-\d{2}$/.test(normalized) || Number.isNaN(date.getTime())) return null;
  return normalized;
}

function parseTransactionType(value: string): string | null {
  const normalized = value.toLowerCase().replace(/\s/g, '');
  if (['buy', 'purchase', 'sip', 'switchin'].includes(normalized)) return 'Buy';
  if (['sell', 'redeem', 'redemption', 'switchout'].includes(normalized)) return 'Sell';
  return null;
}

function parseNumberCell(value: string): number | null {
  if (!value) return null;
  const normalized = value.replace(/,/g, '');
  const parsed = Number(normalized);
  return Number.isFinite(parsed) ? parsed : null;
}

function parseGridState(value: string): PersistedGridState | null {
  try {
    const parsed = JSON.parse(value) as Partial<PersistedGridState>;
    return Array.isArray(parsed.columnState)
      ? { columnState: parsed.columnState, filterModel: parsed.filterModel ?? null }
      : null;
  } catch {
    return null;
  }
}
