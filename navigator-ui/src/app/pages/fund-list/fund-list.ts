import { Component, inject, signal, OnInit, OnDestroy } from '@angular/core';
import { ActivatedRoute, RouterLink } from '@angular/router';
import { FormsModule } from '@angular/forms';
import { Subject, debounceTime, distinctUntilChanged, switchMap, of, takeUntil } from 'rxjs';
import { FundService } from '../../services/fund.service';
import { Fund } from '../../models/fund.model';
import { DecimalPipe } from '@angular/common';
import { MatButtonModule } from '@angular/material/button';
import { MatCardModule } from '@angular/material/card';
import { MatFormFieldModule } from '@angular/material/form-field';
import { MatInputModule } from '@angular/material/input';
import { MatProgressBarModule } from '@angular/material/progress-bar';
import { MatTableModule } from '@angular/material/table';
import { MatChipsModule } from '@angular/material/chips';

@Component({
  selector: 'app-fund-list',
  imports: [
    RouterLink,
    FormsModule,
    DecimalPipe,
    MatButtonModule,
    MatCardModule,
    MatFormFieldModule,
    MatInputModule,
    MatProgressBarModule,
    MatTableModule,
    MatChipsModule,
  ],
  templateUrl: './fund-list.html',
})
export class FundList implements OnInit, OnDestroy {
  private readonly fundService = inject(FundService);
  private readonly route = inject(ActivatedRoute);
  private readonly destroy$ = new Subject<void>();
  private readonly searchSubject = new Subject<string>();

  readonly funds = signal<Fund[]>([]);
  readonly loading = signal(false);
  readonly searchQuery = signal('');

  readonly filterPresets = [
    'Flexi Cap',
    'Large Cap',
    'Mid Cap',
    'Small Cap',
    'Index',
    'Corporate Bond',
    'Liquid',
    'ELSS',
  ];

  ngOnInit(): void {
    this.searchSubject
      .pipe(
        debounceTime(350),
        distinctUntilChanged(),
        switchMap((query) => {
          if (query.trim().length === 0) {
            return of([]);
          }
          this.loading.set(true);
          return this.fundService.searchFunds(query);
        }),
        takeUntil(this.destroy$),
      )
      .subscribe({
        next: (results) => {
          this.funds.set(results);
          this.loading.set(false);
        },
        error: () => this.loading.set(false),
      });

    this.route.queryParams.pipe(takeUntil(this.destroy$)).subscribe((params) => {
      const q = params['q'];
      if (q && typeof q === 'string') {
        this.onSearchInput(q);
      }
    });
  }

  onSearchInput(query: string): void {
    this.searchQuery.set(query);
    this.searchSubject.next(query);
  }

  setFilterPreset(preset: string): void {
    this.onSearchInput(preset);
  }

  clearSearch(): void {
    this.onSearchInput('');
  }

  ngOnDestroy(): void {
    this.destroy$.next();
    this.destroy$.complete();
  }
}
