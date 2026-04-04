import { Component, inject, signal } from '@angular/core';
import { FundService } from '../../services/fund.service';
import { FundSyncResult } from '../../models/fund.model';

@Component({
  selector: 'app-admin-sync',
  imports: [],
  templateUrl: './admin-sync.html',
  styleUrl: './admin-sync.scss',
})
export class AdminSync {
  private readonly fundService = inject(FundService);

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
      },
      error: (err) => {
        this.error.set(err.error?.message ?? 'Sync failed');
        this.syncing.set(false);
      },
    });
  }
}
