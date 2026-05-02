import { Component, computed, inject, signal } from '@angular/core';
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
  minConfidence = 0.9;
  readonly anomalies = signal<NavAnomalyDto[]>([]);
  readonly anomaliesLoading = signal(false);
  readonly anomaliesError = signal<string | null>(null);
  readonly autoRecording = signal(false);
  readonly autoRecordSummary = signal<string | null>(null);
  readonly pendingCount = computed(() => this.anomalies().filter((a) => !a.hasCorporateAction).length);
  readonly recordedCount = computed(() => this.anomalies().filter((a) => a.hasCorporateAction).length);
  readonly autoRecordableCount = computed(
    () =>
      this.anomalies().filter(
        (a) => !a.hasCorporateAction && a.isAutoRecordable && a.confidence >= this.minConfidence,
      ).length,
  );
  readonly highPriorityCount = computed(
    () => this.anomalies().filter((a) => a.reviewPriority === 'High' && !a.hasCorporateAction).length,
  );
  readonly preReviewCount = computed(
    () => this.anomalies().filter((a) => a.requiresPreReview && !a.hasCorporateAction).length,
  );

  // Action form
  readonly showForm = signal<NavAnomalyDto | null>(null);
  actionType = 'Split';
  actionDate = '';
  adjustmentFactor = 10;
  notes = '';
  readonly formLoading = signal(false);
  readonly formError = signal<string | null>(null);
  readonly editingAction = signal<CorporateActionDto | null>(null);

  // Existing actions for a scheme
  readonly selectedSchemeCode = signal<number | null>(null);
  readonly existingActions = signal<CorporateActionDto[]>([]);
  readonly actionsLoading = signal(false);

  scanAnomalies(preserveSummary = false): void {
    this.anomaliesLoading.set(true);
    this.anomaliesError.set(null);
    if (!preserveSummary) {
      this.autoRecordSummary.set(null);
    }
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

    this.actionType = anomaly.suggestedActionType ?? 'Split';
    this.actionDate = anomaly.date;
    this.adjustmentFactor = anomaly.suggestedAdjustmentFactor ?? 10;
    this.notes = anomaly.suggestedNotes ?? '';
  }

  closeForm(): void {
    this.showForm.set(null);
    this.editingAction.set(null);
    this.formError.set(null);
  }

  submitAction(): void {
    const editingAction = this.editingAction();
    if (editingAction) {
      this.updateExistingAction(editingAction);
      return;
    }

    const anomaly = this.showForm();
    if (!anomaly) return;

    this.formLoading.set(true);
    this.formError.set(null);

    // Convert dd-MM-yyyy to yyyy-MM-dd
    const actionDate = this.toApiDate(this.actionDate || anomaly.date);

    const request: CreateCorporateActionRequest = {
      actionDate,
      actionType: this.actionType,
      adjustmentFactor: this.adjustmentFactor,
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
        this.viewActions(anomaly.schemeCode);
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
    this.editingAction.set(null);
  }

  deleteAction(schemeCode: number, actionId: number): void {
    const deletedAction = this.existingActions().find((a) => a.id === actionId);

    this.caService.deleteAction(schemeCode, actionId).subscribe({
      next: () => {
        this.toast.success('Corporate action deleted');
        this.existingActions.update((list) => list.filter((a) => a.id !== actionId));
        if (deletedAction) {
          const stillRecorded = this.existingActions().some(
            (a) => a.actionDate === deletedAction.actionDate,
          );
          this.anomalies.update((list) =>
            list.map((a) =>
              a.schemeCode === schemeCode && a.date === deletedAction.actionDate
                ? { ...a, hasCorporateAction: stillRecorded, isAutoRecordable: !stillRecorded && a.isAutoRecordable }
                : a,
            ),
          );
        }
      },
    });
  }

  editAction(action: CorporateActionDto): void {
    this.editingAction.set(action);
    this.showForm.set(null);
    this.formError.set(null);
    this.actionType = action.actionType;
    this.actionDate = action.actionDate;
    this.adjustmentFactor = action.adjustmentFactor;
    this.notes = (action.notes ?? '').replace('[Auto-recorded] ', '').replace('[Auto-recorded]', '');
  }

  needsPostRecordReview(action: CorporateActionDto): boolean {
    return action.notes?.includes('[Auto-recorded]') ?? false;
  }

  isSeriesBreakAction(actionType = this.actionType): boolean {
    return actionType === 'Discontinuity' || actionType === 'MissingData';
  }

  autoRecordSafeSuggestions(): void {
    this.autoRecording.set(true);
    this.autoRecordSummary.set(null);
    this.anomaliesError.set(null);

    this.caService.autoRecordAnomalies(this.threshold, this.minConfidence).subscribe({
      next: (result) => {
        this.autoRecording.set(false);
        this.autoRecordSummary.set(
          `${result.recordedActions.toLocaleString()} of ${result.eligibleAnomalies.toLocaleString()} eligible suggestions recorded and applied to calculations.`,
        );
        this.toast.success('Obvious anomaly suggestions recorded');
        this.scanAnomalies(true);
      },
      error: (err) => {
        this.autoRecording.set(false);
        this.anomaliesError.set(err.error?.message ?? 'Failed to auto-record suggestions');
        this.toast.error('Failed to auto-record suggestions');
      },
    });
  }

  confidencePercent(anomaly: NavAnomalyDto): string {
    return `${Math.round((anomaly.confidence ?? 0) * 100)}%`;
  }

  priorityClass(anomaly: NavAnomalyDto): string {
    return `priority-${(anomaly.reviewPriority || 'Medium').toLowerCase()}`;
  }

  private toApiDate(date: string): string {
    const parts = date.split('-');
    return `${parts[2]}-${parts[1]}-${parts[0]}`;
  }

  private updateExistingAction(action: CorporateActionDto): void {
    const schemeCode = this.selectedSchemeCode();
    if (schemeCode === null) return;

    this.formLoading.set(true);
    this.formError.set(null);

    const request: CreateCorporateActionRequest = {
      actionDate: this.toApiDate(this.actionDate || action.actionDate),
      actionType: this.actionType,
      adjustmentFactor: this.adjustmentFactor,
      notes: this.notes || null,
    };

    this.caService.updateAction(schemeCode, action.id, request).subscribe({
      next: (updated) => {
        this.formLoading.set(false);
        this.editingAction.set(null);
        this.toast.success('Corporate action updated');
        this.existingActions.update((list) => list.map((a) => (a.id === updated.id ? updated : a)));
      },
      error: (err) => {
        this.formError.set(err.error?.message ?? 'Failed to update action');
        this.formLoading.set(false);
        this.toast.error('Failed to update action');
      },
    });
  }
}
