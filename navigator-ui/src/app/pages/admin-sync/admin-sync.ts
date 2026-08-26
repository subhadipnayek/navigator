import { Component, OnInit, inject, signal, ChangeDetectionStrategy } from '@angular/core';
import { DecimalPipe } from '@angular/common';
import { FormsModule } from '@angular/forms';
import { RouterLink, RouterLinkActive } from '@angular/router';
import { FundService } from '../../services/fund.service';
import {
  AdminDatabaseSummary,
  AdminFundStorage,
  FundSyncResult,
  PagedResponse,
} from '../../models/fund.model';
import { ToastService } from '../../shared/toast/toast.service';
import { MatButtonModule } from '@angular/material/button';
import { MatCardModule } from '@angular/material/card';
import { MatChipsModule } from '@angular/material/chips';
import { MatFormFieldModule } from '@angular/material/form-field';
import { MatInputModule } from '@angular/material/input';
import { MatListModule } from '@angular/material/list';
import { MatProgressBarModule } from '@angular/material/progress-bar';
import { MatSelectModule } from '@angular/material/select';
import { MatTableModule } from '@angular/material/table';

type NavStatus = 'all' | 'with-nav' | 'without-nav';

@Component({
  selector: 'app-admin-sync',
  changeDetection: ChangeDetectionStrategy.OnPush,
  imports: [
    FormsModule,
    DecimalPipe,
    RouterLink,
    RouterLinkActive,
    MatButtonModule,
    MatCardModule,
    MatChipsModule,
    MatFormFieldModule,
    MatInputModule,
    MatListModule,
    MatProgressBarModule,
    MatSelectModule,
    MatTableModule,
  ],
  templateUrl: './admin-sync.html',
})
export class AdminSync implements OnInit {
  private readonly fundService = inject(FundService);
  private readonly toast = inject(ToastService);

  readonly syncing = signal(false);
  readonly loadingSummary = signal(false);
  readonly loadingFunds = signal(false);
  readonly actionBusy = signal<string | null>(null);
  readonly result = signal<FundSyncResult | null>(null);
  readonly error = signal<string | null>(null);
  readonly summary = signal<AdminDatabaseSummary | null>(null);
  readonly fundPage = signal<PagedResponse<AdminFundStorage> | null>(null);

  search = '';
  navStatus: NavStatus = 'all';
  page = 1;
  pageSize = 15;

  ngOnInit(): void {
    this.refreshAdminData();
  }

  refreshAdminData(): void {
    this.loadSummary();
    this.loadFunds();
  }

  loadSummary(): void {
    this.loadingSummary.set(true);
    this.fundService.getAdminSummary().subscribe({
      next: (summary) => {
        this.summary.set(summary);
        this.loadingSummary.set(false);
      },
      error: (err) => {
        this.error.set(err.error?.message ?? 'Failed to load admin summary');
        this.loadingSummary.set(false);
      },
    });
  }

  loadFunds(resetPage = false): void {
    if (resetPage) this.page = 1;
    this.loadingFunds.set(true);
    this.error.set(null);

    this.fundService
      .getAdminFunds({
        q: this.search,
        navStatus: this.navStatus,
        page: this.page,
        pageSize: this.pageSize,
      })
      .subscribe({
        next: (fundPage) => {
          this.fundPage.set(fundPage);
          this.loadingFunds.set(false);
        },
        error: (err) => {
          this.error.set(err.error?.message ?? 'Failed to load funds');
          this.loadingFunds.set(false);
        },
      });
  }

  sync(): void {
    this.syncing.set(true);
    this.result.set(null);
    this.error.set(null);

    this.fundService.syncAdminFunds().subscribe({
      next: (res) => {
        this.result.set(res);
        this.syncing.set(false);
        const added = res.added ?? res.inserted ?? 0;
        const updated = res.updated ?? 0;
        this.toast.success(`Sync complete - ${added} added, ${updated} updated`);
        this.refreshAdminData();
      },
      error: (err) => {
        this.error.set(err.error?.message ?? 'Sync failed');
        this.syncing.set(false);
        this.toast.error(err.error?.message ?? 'Sync failed');
      },
    });
  }

  syncNav(fund: AdminFundStorage): void {
    const key = this.busyKey('sync-nav', fund.schemeCode);
    this.actionBusy.set(key);
    this.error.set(null);

    this.fundService.syncFundNavHistory(fund.schemeCode).subscribe({
      next: (res) => {
        this.actionBusy.set(null);
        const count = res.data?.length ?? 0;
        this.toast.success(
          `NAV synced - ${count.toLocaleString()} stored rows available`,
        );
        this.refreshAdminData();
      },
      error: (err) => {
        this.actionBusy.set(null);
        this.error.set(err.error?.message ?? 'NAV sync failed');
        this.toast.error('NAV sync failed');
      },
    });
  }

  clearNav(fund: AdminFundStorage): void {
    if (!window.confirm(`Clear stored NAV history for ${fund.schemeName}?`)) return;

    const key = this.busyKey('clear-nav', fund.schemeCode);
    this.actionBusy.set(key);
    this.error.set(null);

    this.fundService.deleteFundNavHistory(fund.schemeCode).subscribe({
      next: () => {
        this.actionBusy.set(null);
        this.toast.success('Stored NAV history cleared');
        this.refreshAdminData();
      },
      error: (err) => {
        this.actionBusy.set(null);
        this.error.set(err.error?.message ?? 'Failed to clear NAV history');
        this.toast.error('Failed to clear NAV history');
      },
    });
  }

  deleteFund(fund: AdminFundStorage): void {
    if (
      !window.confirm(`Delete ${fund.schemeName} and all related NAV/actions from the database?`)
    ) {
      return;
    }

    const key = this.busyKey('delete-fund', fund.schemeCode);
    this.actionBusy.set(key);
    this.error.set(null);

    this.fundService.deleteFund(fund.schemeCode).subscribe({
      next: () => {
        this.actionBusy.set(null);
        this.toast.success('Fund deleted from database');
        this.refreshAdminData();
      },
      error: (err) => {
        this.actionBusy.set(null);
        this.error.set(err.error?.message ?? 'Failed to delete fund');
        this.toast.error('Failed to delete fund');
      },
    });
  }

  deleteAllFunds(): void {
    if (
      !window.confirm(
        `Delete ALL funds and related NAV/actions from the database? This cannot be undone!`,
      )
    ) {
      return;
    }

    const key = this.busyKey('delete-fund', -1);
    this.actionBusy.set(key);
    this.error.set(null);

    this.fundService.deleteAllFunds().subscribe({
      next: () => {
        this.actionBusy.set(null);
        this.toast.success('All funds deleted from database');
        this.refreshAdminData();
      },
      error: (err) => {
        this.actionBusy.set(null);
        this.error.set(err.error?.message ?? 'Failed to delete all funds');
        this.toast.error('Failed to delete all funds');
      },
    });
  }

  nextPage(): void {
    const page = this.fundPage();
    if (!page || page.page >= page.totalPages) return;
    this.page += 1;
    this.loadFunds();
  }

  previousPage(): void {
    if (this.page <= 1) return;
    this.page -= 1;
    this.loadFunds();
  }

  busyKey(action: string, schemeCode: number): string {
    return `${action}:${schemeCode}`;
  }
}
