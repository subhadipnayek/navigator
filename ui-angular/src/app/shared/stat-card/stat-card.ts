import { Component, input } from '@angular/core';

@Component({
  selector: 'app-stat-card',
  template: `
    <div class="stat-card" [class.fresh]="variant() === 'fresh'">
      <span class="label">{{ label() }}</span>
      <span
        class="value"
        [class.positive]="variant() === 'positive'"
        [class.negative]="variant() === 'negative'"
      >
        <ng-content />
      </span>
    </div>
  `,
  styles: `
    .stat-card {
      display: flex;
      flex-direction: column;
      gap: var(--space-xs);
      padding: var(--space-lg);
      border: 1px solid var(--color-border);
      border-radius: var(--radius-lg);
      background: var(--color-bg-subtle);
    }

    .label {
      font-size: var(--font-size-xs);
      color: var(--color-text-secondary);
      text-transform: uppercase;
      letter-spacing: 0.04em;
    }

    .value {
      font-size: var(--font-size-xl);
      font-weight: 600;
    }

    .positive {
      color: var(--color-positive);
    }

    .negative {
      color: var(--color-negative);
    }

    .fresh .value {
      color: var(--color-positive);
    }
  `,
})
export class StatCard {
  readonly label = input.required<string>();
  readonly variant = input<'default' | 'positive' | 'negative' | 'fresh'>('default');
}
