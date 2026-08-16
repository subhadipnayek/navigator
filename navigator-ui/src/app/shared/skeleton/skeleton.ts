import { Component } from '@angular/core';
import { MatProgressBarModule } from '@angular/material/progress-bar';

@Component({
  selector: 'app-skeleton',
  imports: [MatProgressBarModule],
  template: `<mat-progress-bar mode="indeterminate" />`,
})
export class Skeleton {}
