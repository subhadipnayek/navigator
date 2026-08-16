import { Component, input } from '@angular/core';
import { MatCardModule } from '@angular/material/card';

@Component({
  selector: 'app-stat-card',
  imports: [MatCardModule],
  template: `
    <mat-card appearance="outlined">
      <mat-card-header>
        <mat-card-subtitle>{{ label() }}</mat-card-subtitle>
      </mat-card-header>
      <mat-card-content>
        <h2><ng-content /></h2>
      </mat-card-content>
    </mat-card>
  `,
})
export class StatCard {
  readonly label = input.required<string>();
  readonly variant = input<'default' | 'positive' | 'negative' | 'fresh'>('default');
}
