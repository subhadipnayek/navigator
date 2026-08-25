import { Component, computed, inject, signal, ChangeDetectionStrategy } from '@angular/core';
import { FormsModule } from '@angular/forms';
import { DecimalPipe } from '@angular/common';
import { RouterLink, RouterLinkActive } from '@angular/router';
import { catchError, of } from 'rxjs';
import { CorporateActionService } from '../../services/corporate-action.service';
import { NavService } from '../../services/nav.service';
import { FundService } from '../../services/fund.service';
import {
  NavAnomalyDto,
  CorporateActionDto,
  CreateCorporateActionRequest,
  NavGapDto,
  NavGapRepairResultDto,
  CorporateActionReviewRowDto,
  CorporateActionReviewSummaryDto,
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

type ReviewFilter =
  | 'all'
  | 'factor-actions'
  | 'gaps'
  | 'missing-data'
  | 'large-moves'
  | 'recorded'
  | 'needs-review';

@Component({
  selector: 'app-corporate-actions',
  changeDetection: ChangeDetectionStrategy.OnPush,
  imports: [
    FormsModule,
    DecimalPipe,
    RouterLink,
    RouterLinkActive,
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
  private readonly navService = inject(NavService);
  private readonly fundService = inject(FundService);
  private readonly toast = inject(ToastService);

  threshold = 30;
  minConfidence = 0.9;
  minGapDays = 1;
  readonly reviewFilter = signal<ReviewFilter>('all');
  readonly rows = signal<CorporateActionReviewRowDto[]>([]);
  readonly anomalies = signal<NavAnomalyDto[]>([]);
  readonly gaps = signal<NavGapDto[]>([]);
  readonly anomaliesLoading = signal(false);
  readonly autoRecording = signal(false);
  readonly repairingSchemeCode = signal<number | null>(null);
  readonly syncingSchemeCode = signal<number | null>(null);
  readonly anomaliesError = signal<string | null>(null);
  readonly autoRecordSummary = signal<string | null>(null);
  readonly repairResult = signal<NavGapRepairResultDto | null>(null);
  readonly summary = signal<CorporateActionReviewSummaryDto>({
    pendingVisible: 0,
    navGaps: 0,
    autoRecordable: 0,
    highPriority: 0,
    preReview: 0,
    recordedItems: 0,
  });

  readonly pendingCount = computed(() => this.summary().pendingVisible);
  readonly gapCount = computed(() => this.summary().navGaps);
  readonly recordedCount = computed(() => this.summary().recordedItems);
  readonly autoRecordableCount = computed(() => this.summary().autoRecordable);
  readonly highPriorityCount = computed(() => this.summary().highPriority);
  readonly preReviewCount = computed(() => this.summary().preReview);

  readonly showForm = signal<NavAnomalyDto | null>(null);
  actionType = 'Split';
  actionDate = '';
  adjustmentFactor = 10;
  notes = '';
  readonly formLoading = signal(false);
  readonly formError = signal<string | null>(null);
  readonly editingAction = signal<CorporateActionDto | null>(null);

  readonly selectedSchemeCode = signal<number | null>(null);
  readonly existingActions = signal<CorporateActionDto[]>([]);
  readonly actionsLoading = signal(false);

  scanAnomalies(preserveSummary = false): void {
    this.anomaliesLoading.set(true);
    this.anomaliesError.set(null);
    this.repairResult.set(null);
    if (!preserveSummary) {
      this.autoRecordSummary.set(null);
    }
    this.anomalies.set([]);
    this.gaps.set([]);
    this.rows.set([]);
    this.resetSummary();

    this.caService
      .getReviewQueue({
        threshold: this.threshold,
        minGapDays: this.minGapDays,
        filter: this.reviewFilter(),
        minAutoRecordConfidence: this.minConfidence,
      })
      .pipe(
        catchError((err) => {
          this.anomaliesError.set(err.error?.message ?? 'Failed to scan review queue');
          return of({
            rows: [] as CorporateActionReviewRowDto[],
            summary: this.emptySummary(),
            anomalies: [] as NavAnomalyDto[],
            gaps: [] as NavGapDto[],
          });
        }),
      )
      .subscribe(({ rows, summary, anomalies, gaps }) => {
        this.rows.set(rows);
        this.summary.set(summary);
        this.anomalies.set(anomalies);
        this.gaps.set(gaps);
        this.anomaliesLoading.set(false);
      });
  }

  setReviewFilter(filter: ReviewFilter): void {
    this.reviewFilter.set(filter);
    if (this.rows().length > 0) {
      this.scanAnomalies(true);
    }
  }

  openForm(anomaly: NavAnomalyDto): void {
    this.showForm.set(anomaly);
    this.formError.set(null);

    this.actionType = anomaly.suggestedActionType ?? 'Split';
    this.actionDate = anomaly.date;
    this.adjustmentFactor = anomaly.suggestedAdjustmentFactor ?? 10;
    this.notes = anomaly.suggestedNotes ?? '';
  }

  openGapForm(gap: NavGapDto): void {
    this.openForm({
      schemeCode: gap.schemeCode,
      schemeName: gap.schemeName,
      fundHouse: gap.fundHouse,
      date: gap.nextDate,
      previousDate: gap.previousDate,
      daysSincePrevious: gap.gapDays,
      previousNav: 0,
      currentNav: 0,
      changePercent: 0,
      direction: 'Gap',
      hasCorporateAction: false,
      suggestedActionType: 'MissingData',
      suggestedAdjustmentFactor: 1,
      suggestedNotes: `${gap.gapDays}-day NAV gap from ${gap.missingFrom} to ${gap.missingTo}. Repair from AMFI first when possible; otherwise mark MissingData so returns do not cross it.`,
      confidence: 0.78,
      reviewPriority: 'High',
      isAutoRecordable: false,
      requiresPreReview: true,
    });
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

    const request: CreateCorporateActionRequest = {
      actionDate: this.toApiDate(this.actionDate || anomaly.date),
      actionType: this.actionType,
      adjustmentFactor: this.adjustmentFactor,
      notes: this.notes || null,
    };

    this.caService.createAction(anomaly.schemeCode, request).subscribe({
      next: () => {
        this.formLoading.set(false);
        this.showForm.set(null);
        this.toast.success('Corporate action recorded');
        this.anomalies.update((list) =>
          list.map((a) =>
            a.schemeCode === anomaly.schemeCode && a.date === anomaly.date
              ? { ...a, hasCorporateAction: true }
              : a,
          ),
        );
        if (this.isSeriesBreakAction(request.actionType)) {
          this.gaps.update((list) =>
            list.filter((g) => !(g.schemeCode === anomaly.schemeCode && g.nextDate === anomaly.date)),
          );
        }
        this.scanAnomalies(true);
        this.viewActions(anomaly.schemeCode);
      },
      error: (err) => {
        this.formError.set(err.error?.message ?? 'Failed to create action');
        this.formLoading.set(false);
        this.toast.error('Failed to create action');
      },
    });
  }

  repairGap(gap: NavGapDto): void {
    this.repairingSchemeCode.set(gap.schemeCode);
    this.anomaliesError.set(null);
    this.repairResult.set(null);

    this.navService.repairNavGaps(gap.schemeCode, this.gapDetectionDays()).subscribe({
      next: (result) => {
        this.repairResult.set(result);
        this.repairingSchemeCode.set(null);
        this.toast.success(`Inserted ${result.insertedRecords} NAV records`);
        this.scanAnomalies(true);
      },
      error: (err) => {
        this.anomaliesError.set(err.error?.message ?? 'Gap repair failed');
        this.repairingSchemeCode.set(null);
        this.toast.error('Gap repair failed');
      },
    });
  }

  syncNav(row: CorporateActionReviewRowDto): void {
    if (row.gap) {
      this.repairGap(row.gap);
      return;
    }

    this.syncingSchemeCode.set(row.schemeCode);
    this.anomaliesError.set(null);

    this.fundService.syncFundNavHistory(row.schemeCode).subscribe({
      next: (result) => {
        this.syncingSchemeCode.set(null);
        this.toast.success(`NAV synced - ${result.data.length.toLocaleString()} stored rows available`);
        this.scanAnomalies(true);
      },
      error: (err) => {
        this.syncingSchemeCode.set(null);
        this.anomaliesError.set(err.error?.message ?? 'NAV sync failed');
        this.toast.error('NAV sync failed');
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

  priorityClass(priorityOrAnomaly: string | NavAnomalyDto): string {
    const priority =
      typeof priorityOrAnomaly === 'string'
        ? priorityOrAnomaly
        : priorityOrAnomaly.reviewPriority || 'Medium';
    return `priority-${priority.toLowerCase()}`;
  }

  private gapDetectionDays(): number {
    return Math.max(this.minGapDays, 7);
  }

  private resetSummary(): void {
    this.summary.set(this.emptySummary());
  }

  private emptySummary(): CorporateActionReviewSummaryDto {
    return {
      pendingVisible: 0,
      navGaps: 0,
      autoRecordable: 0,
      highPriority: 0,
      preReview: 0,
      recordedItems: 0,
    };
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
