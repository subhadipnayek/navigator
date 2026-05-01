import { Component, inject, OnInit, signal } from '@angular/core';
import { RouterLink } from '@angular/router';
import { FundService } from '../../services/fund.service';
import { AuthService } from '../../services/auth.service';
import { MatButtonModule } from '@angular/material/button';
import { MatCardModule } from '@angular/material/card';
import { MatProgressBarModule } from '@angular/material/progress-bar';

@Component({
  selector: 'app-dashboard',
  imports: [RouterLink, MatButtonModule, MatCardModule, MatProgressBarModule],
  templateUrl: './dashboard.html',
})
export class Dashboard implements OnInit {
  private readonly fundService = inject(FundService);
  readonly auth = inject(AuthService);

  readonly fundCount = signal<number | null>(null);
  readonly loading = signal(true);

  ngOnInit(): void {
    this.fundService.getFundCount().subscribe({
      next: (res) => {
        this.fundCount.set(res.count);
        this.loading.set(false);
      },
      error: () => this.loading.set(false),
    });
  }
}
