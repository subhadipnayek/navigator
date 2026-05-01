import { Component, inject, signal } from '@angular/core';
import { FormsModule } from '@angular/forms';
import { DecimalPipe } from '@angular/common';
import { CorporateActionService } from '../../services/corporate-action.service';
import {
  NavAnomalyDto,
  CorporateActionDto,
  CreateCorporateActionRequest,
} from '../../models/fund.model';
import { ToastService } from '../../shared/toast/toast.service';
import { MatButtonModule } from '@angular/material/button';
import { MatCardModule } from '@angular/material/card';
import { MatChipsModule } from '@angular/material/chips';
import { MatFormFieldModule } from '@angular/material/form-field';
import { MatInputModule } from '@angular/material/input';
import { MatProgressBarModule } from '@angular/material/progress-bar';
import { MatSelectModule } from '@angular/material/select';
import { MatTableModule } from '@angular/material/table';

@Component({
  selector: 'app-corporate-actions',
  imports: [
    FormsModule,
    DecimalPipe,
    MatButtonModule,
    MatCardModule,
    MatChipsModule,
    MatFormFieldModule,
    MatInputModule,
    MatProgressBarModule,
    MatSelectModule,
    MatTableModule,
  ],
  templateUrl: './corporate-actions.html',
})
export class CorporateActions {
  private readonly caService = inject(CorporateActionService);
  private readonly toast = inject(ToastService);

  // Anomaly detection
  threshold = 30;
  readonly anomalies = signal<NavAnomalyDto[]>([]);
  readonly anomaliesLoading = signal(false);
  readonly anomaliesError = signal<string | null>(null);

  // Action form
  readonly showForm = signal<NavAnomalyDto | null>(null);
  actionType = 'Split';
  adjustmentFactor = 10;
  oldFaceValue: number | null = null;
  newFaceValue: number | null = null;
  notes = '';
  readonly formLoading = signal(false);
  readonly formError = signal<string | null>(null);

  // Existing actions for a scheme
  readonly selectedSchemeCode = signal<number | null>(null);
  readonly existingActions = signal<CorporateActionDto[]>([]);
  readonly actionsLoading = signal(false);

  scanAnomalies(): void {
    this.anomaliesLoading.set(true);
    this.anomaliesError.set(null);
    this.anomalies.set([]);

    this.caService.getAnomalies(this.threshold).subscribe({
      next: (data) => {
        this.anomalies.set(data);
        this.anomaliesLoading.set(false);
      },
      error: (err) => {
        this.anomaliesError.set(err.error?.message ?? 'Failed to scan anomalies');
        this.anomaliesLoading.set(false);
      },
    });
  }

  openForm(anomaly: NavAnomalyDto): void {
    this.showForm.set(anomaly);
    this.formError.set(null);

    // Auto-suggest Discontinuity for large gaps (likely data gap, not a split)
    if (anomaly.daysSincePrevious > 30) {
      this.actionType = 'Discontinuity';
      this.adjustmentFactor = 1;
      this.notes = `~${anomaly.daysSincePrevious}-day data gap, likely fund reconstitution`;
    } else {
      this.actionType = 'Split';
      this.adjustmentFactor = 10;
      this.notes = '';
    }
    this.oldFaceValue = null;
    this.newFaceValue = null;
  }

  closeForm(): void {
    this.showForm.set(null);
  }

  submitAction(): void {
    const anomaly = this.showForm();
    if (!anomaly) return;

    this.formLoading.set(true);
    this.formError.set(null);

    // Convert dd-MM-yyyy to yyyy-MM-dd
    const parts = anomaly.date.split('-');
    const actionDate = `${parts[2]}-${parts[1]}-${parts[0]}`;

    const request: CreateCorporateActionRequest = {
      actionDate,
      actionType: this.actionType,
      adjustmentFactor: this.adjustmentFactor,
      oldFaceValue: this.oldFaceValue,
      newFaceValue: this.newFaceValue,
      notes: this.notes || null,
    };

    this.caService.createAction(anomaly.schemeCode, request).subscribe({
      next: () => {
        this.formLoading.set(false);
        this.showForm.set(null);
        this.toast.success('Corporate action recorded');
        // Mark the anomaly as having a corporate action
        this.anomalies.update((list) =>
          list.map((a) =>
            a.schemeCode === anomaly.schemeCode && a.date === anomaly.date
              ? { ...a, hasCorporateAction: true }
              : a,
          ),
        );
      },
      error: (err) => {
        this.formError.set(err.error?.message ?? 'Failed to create action');
        this.formLoading.set(false);
        this.toast.error('Failed to create action');
      },
    });
  }

  viewActions(schemeCode: number): void {
    this.selectedSchemeCode.set(schemeCode);
    this.actionsLoading.set(true);
    this.existingActions.set([]);

    this.caService.getActions(schemeCode).subscribe({
      next: (data) => {
        this.existingActions.set(data);
        this.actionsLoading.set(false);
      },
      error: () => this.actionsLoading.set(false),
    });
  }

  closeActions(): void {
    this.selectedSchemeCode.set(null);
    this.existingActions.set([]);
  }

  deleteAction(schemeCode: number, actionId: number): void {
    this.caService.deleteAction(schemeCode, actionId).subscribe({
      next: () => {
        this.toast.success('Corporate action deleted');
        this.existingActions.update((list) => list.filter((a) => a.id !== actionId));
        // If we deleted the last action for this scheme+date, update anomalies
        this.anomalies.update((list) =>
          list.map((a) =>
            a.schemeCode === schemeCode
              ? { ...a, hasCorporateAction: this.existingActions().length > 0 }
              : a,
          ),
        );
      },
    });
  }
}
