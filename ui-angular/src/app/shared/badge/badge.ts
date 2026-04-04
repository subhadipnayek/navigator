import { Component, input } from '@angular/core';

@Component({
  selector: 'app-badge',
  template: `
    <span
      class="badge"
      [class.badge-positive]="variant() === 'positive'"
      [class.badge-negative]="variant() === 'negative'"
      [class.badge-warning]="variant() === 'warning'"
      [class.badge-neutral]="variant() === 'neutral'"
    >
      <ng-content />
    </span>
  `,
  styles: `
    .badge {
      display: inline-block;
      padding: 0.2rem 0.5rem;
      border-radius: var(--radius-sm);
      font-size: var(--font-size-xs);
      font-weight: 600;
      line-height: 1.4;
    }

    .badge-positive {
      background: var(--color-positive-bg);
      color: var(--color-positive);
    }

    .badge-negative {
      background: var(--color-negative-bg);
      color: var(--color-negative);
    }

    .badge-warning {
      background: var(--color-warning-bg);
      color: var(--color-warning);
    }

    .badge-neutral {
      background: var(--color-bg-subtle);
      color: var(--color-text-secondary);
    }
  `,
})
export class Badge {
  readonly variant = input<'positive' | 'negative' | 'warning' | 'neutral'>('neutral');
}
