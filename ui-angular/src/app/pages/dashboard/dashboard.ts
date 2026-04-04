import { Component, inject, OnInit, signal } from '@angular/core';
import { RouterLink } from '@angular/router';
import { FundService } from '../../services/fund.service';

@Component({
  selector: 'app-dashboard',
  imports: [RouterLink],
  templateUrl: './dashboard.html',
  styleUrl: './dashboard.scss',
})
export class Dashboard implements OnInit {
  private readonly fundService = inject(FundService);

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
