import { Component, inject, signal } from '@angular/core';
import { FundService } from '../../services/fund.service';
import { FundSyncResult } from '../../models/fund.model';
import { ToastService } from '../../shared/toast/toast.service';

@Component({
  selector: 'app-admin-sync',
  imports: [],
  templateUrl: './admin-sync.html',
  styleUrl: './admin-sync.scss',
})
export class AdminSync {
  private readonly fundService = inject(FundService);
  private readonly toast = inject(ToastService);

  readonly syncing = signal(false);
  readonly result = signal<FundSyncResult | null>(null);
  readonly error = signal<string | null>(null);

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
}
