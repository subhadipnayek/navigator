import { Component, inject, OnInit, signal } from '@angular/core';
import { Router, RouterLink } from '@angular/router';
import { DecimalPipe } from '@angular/common';
import { FormsModule } from '@angular/forms';
import { FundService } from '../../services/fund.service';
import { AuthService } from '../../services/auth.service';
import { MatButtonModule } from '@angular/material/button';
import { MatCardModule } from '@angular/material/card';
import { MatProgressBarModule } from '@angular/material/progress-bar';
import { MatFormFieldModule } from '@angular/material/form-field';
import { MatInputModule } from '@angular/material/input';
import { MatChipsModule } from '@angular/material/chips';

@Component({
  selector: 'app-dashboard',
  imports: [
    RouterLink,
    DecimalPipe,
    FormsModule,
    MatButtonModule,
    MatCardModule,
    MatProgressBarModule,
    MatFormFieldModule,
    MatInputModule,
    MatChipsModule,
  ],
  templateUrl: './dashboard.html',
})
export class Dashboard implements OnInit {
  private readonly fundService = inject(FundService);
  private readonly router = inject(Router);
  readonly auth = inject(AuthService);

  readonly fundCount = signal<number | null>(null);
  readonly loading = signal(true);
  readonly searchKeyword = signal('');

  readonly quickCategories = [
    { label: 'Flexi Cap', query: 'Flexi Cap' },
    { label: 'Large & Mid Cap', query: 'Large & Mid Cap' },
    { label: 'Small Cap', query: 'Small Cap' },
    { label: 'Index & ETFs', query: 'Index' },
    { label: 'Corporate Debt', query: 'Corporate Bond' },
    { label: 'Liquid & Overnight', query: 'Liquid' },
    { label: 'HDFC', query: 'HDFC' },
    { label: 'SBI', query: 'SBI' },
    { label: 'ICICI', query: 'ICICI Prudential' },
  ];

  ngOnInit(): void {
    this.fundService.getFundCount().subscribe({
      next: (res) => {
        this.fundCount.set(res.count);
        this.loading.set(false);
      },
      error: () => this.loading.set(false),
    });
  }

  onSearch(): void {
    const q = this.searchKeyword().trim();
    if (q) {
      this.router.navigate(['/funds'], { queryParams: { q } });
    } else {
      this.router.navigate(['/funds']);
    }
  }

  searchCategory(catQuery: string): void {
    this.router.navigate(['/funds'], { queryParams: { q: catQuery } });
  }
}
