import { Component, inject, signal } from '@angular/core';
import { FormsModule } from '@angular/forms';
import { RouterLink } from '@angular/router';
import { FundService } from '../../services/fund.service';
import { NavService } from '../../services/nav.service';
import { FundSyncResult, NavGapDto, NavGapRepairResultDto } from '../../models/fund.model';
import { ToastService } from '../../shared/toast/toast.service';
import { MatButtonModule } from '@angular/material/button';
import { MatCardModule } from '@angular/material/card';
import { MatChipsModule } from '@angular/material/chips';
import { MatFormFieldModule } from '@angular/material/form-field';
import { MatInputModule } from '@angular/material/input';
import { MatListModule } from '@angular/material/list';
import { MatProgressBarModule } from '@angular/material/progress-bar';
import { MatTableModule } from '@angular/material/table';

@Component({
  selector: 'app-admin-sync',
  imports: [
    FormsModule,
    RouterLink,
    MatButtonModule,
    MatCardModule,
    MatChipsModule,
    MatFormFieldModule,
    MatInputModule,
    MatListModule,
    MatProgressBarModule,
    MatTableModule,
  ],
  templateUrl: './admin-sync.html',
})
export class AdminSync {
  private readonly fundService = inject(FundService);
  private readonly navService = inject(NavService);
  private readonly toast = inject(ToastService);

  readonly syncing = signal(false);
  readonly result = signal<FundSyncResult | null>(null);
  readonly error = signal<string | null>(null);
  readonly gapLoading = signal(false);
  readonly gapRepairing = signal(false);
  readonly gapError = signal<string | null>(null);
  readonly gaps = signal<NavGapDto[]>([]);
  readonly repairResult = signal<NavGapRepairResultDto | null>(null);

  gapSchemeCode: number | null = null;
  minGapDays = 30;

  sync(): void {
    this.syncing.set(true);
    this.result.set(null);
    this.error.set(null);

    this.fundService.syncFunds().subscribe({
      next: (res) => {
        this.result.set(res);
        this.syncing.set(false);
        this.toast.success(`Sync complete — ${res.added} added, ${res.updated} updated`);
      },
      error: (err) => {
        this.error.set(err.error?.message ?? 'Sync failed');
        this.syncing.set(false);
        this.toast.error('Sync failed');
      },
    });
  }

  detectGaps(): void {
    const schemeCode = this.getGapSchemeCode();
    if (!schemeCode) return;

    this.gapLoading.set(true);
    this.gapError.set(null);
    this.repairResult.set(null);
    this.gaps.set([]);

    this.navService.getNavGaps(schemeCode, this.minGapDays).subscribe({
      next: (gaps) => {
        this.gaps.set(gaps);
        this.gapLoading.set(false);
        this.toast.success(gaps.length ? `${gaps.length} NAV gaps found` : 'No NAV gaps found');
      },
      error: (err) => {
        this.gapError.set(err.error?.message ?? 'Gap scan failed');
        this.gapLoading.set(false);
        this.toast.error('Gap scan failed');
      },
    });
  }

  repairGaps(): void {
    const schemeCode = this.getGapSchemeCode();
    if (!schemeCode) return;

    this.gapRepairing.set(true);
    this.gapError.set(null);
    this.repairResult.set(null);

    this.navService.repairNavGaps(schemeCode, this.minGapDays).subscribe({
      next: (result) => {
        this.repairResult.set(result);
        this.gaps.set(result.gaps);
        this.gapRepairing.set(false);
        this.toast.success(`Inserted ${result.insertedRecords} NAV records`);
      },
      error: (err) => {
        this.gapError.set(err.error?.message ?? 'Gap repair failed');
        this.gapRepairing.set(false);
        this.toast.error('Gap repair failed');
      },
    });
  }

  private getGapSchemeCode(): number | null {
    const schemeCode = Number(this.gapSchemeCode);
    if (!Number.isInteger(schemeCode) || schemeCode <= 0) {
      this.gapError.set('Enter a valid scheme code.');
      return null;
    }

    return schemeCode;
  }
}
