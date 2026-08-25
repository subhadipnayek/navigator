import { Component, ChangeDetectionStrategy } from '@angular/core';
import { MatProgressBarModule } from '@angular/material/progress-bar';

@Component({
  selector: 'app-skeleton',
  changeDetection: ChangeDetectionStrategy.OnPush,
  imports: [MatProgressBarModule],
  template: `<mat-progress-bar mode="indeterminate" />`,
})
export class Skeleton {}
