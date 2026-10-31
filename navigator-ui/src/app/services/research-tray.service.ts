import { Injectable, signal, computed, effect } from '@angular/core';
import { Fund } from '../models/fund.model';

const STORAGE_KEY = 'navigator_research_basket';
const MAX_BASKET_SIZE = 5;

@Injectable({
  providedIn: 'root',
})
export class ResearchTrayService {
  readonly pinnedFunds = signal<Fund[]>(this.loadFromStorage());
  readonly count = computed(() => this.pinnedFunds().length);
  readonly maxCapacity = MAX_BASKET_SIZE;
  readonly isFull = computed(() => this.pinnedFunds().length >= MAX_BASKET_SIZE);

  constructor() {
    effect(() => {
      const funds = this.pinnedFunds();
      try {
        localStorage.setItem(STORAGE_KEY, JSON.stringify(funds));
      } catch {
        // Ignore localStorage quota errors
      }
    });
  }

  isInTray(schemeCode: number): boolean {
    return this.pinnedFunds().some((f) => f.schemeCode === schemeCode);
  }

  addFund(fund: Fund): boolean {
    if (this.isInTray(fund.schemeCode)) {
      return false;
    }
    if (this.pinnedFunds().length >= MAX_BASKET_SIZE) {
      return false;
    }
    this.pinnedFunds.update((list) => [...list, fund]);
    return true;
  }

  removeFund(schemeCode: number): void {
    this.pinnedFunds.update((list) => list.filter((f) => f.schemeCode !== schemeCode));
  }

  toggleFund(fund: Fund): boolean {
    if (this.isInTray(fund.schemeCode)) {
      this.removeFund(fund.schemeCode);
      return false;
    }
    return this.addFund(fund);
  }

  clear(): void {
    this.pinnedFunds.set([]);
  }

  private loadFromStorage(): Fund[] {
    try {
      const raw = localStorage.getItem(STORAGE_KEY);
      if (!raw) return [];
      const parsed = JSON.parse(raw);
      if (Array.isArray(parsed)) {
        return parsed.slice(0, MAX_BASKET_SIZE);
      }
    } catch {
      // Fallback on corrupt JSON
    }
    return [];
  }
}
