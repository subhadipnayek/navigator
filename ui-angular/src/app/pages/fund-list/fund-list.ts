import { Component, inject, signal, OnInit, OnDestroy } from '@angular/core';
import { RouterLink } from '@angular/router';
import { FormsModule } from '@angular/forms';
import { Subject, debounceTime, distinctUntilChanged, switchMap, of, takeUntil } from 'rxjs';
import { FundService } from '../../services/fund.service';
import { Fund } from '../../models/fund.model';
import { DecimalPipe } from '@angular/common';

@Component({
  selector: 'app-fund-list',
  imports: [RouterLink, FormsModule, DecimalPipe],
  templateUrl: './fund-list.html',
  styleUrl: './fund-list.scss',
})
export class FundList implements OnInit, OnDestroy {
  private readonly fundService = inject(FundService);
  private readonly destroy$ = new Subject<void>();
  private readonly searchSubject = new Subject<string>();

  readonly funds = signal<Fund[]>([]);
  readonly loading = signal(false);
  readonly searchQuery = signal('');

  ngOnInit(): void {
    this.searchSubject
      .pipe(
        debounceTime(400),
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
  }

  onSearchInput(query: string): void {
    this.searchQuery.set(query);
    this.searchSubject.next(query);
  }

  ngOnDestroy(): void {
    this.destroy$.next();
    this.destroy$.complete();
  }
}
