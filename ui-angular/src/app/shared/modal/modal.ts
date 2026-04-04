import { Component, output } from '@angular/core';

@Component({
  selector: 'app-modal',
  template: `
    <div class="modal-overlay" (click)="closed.emit()">
      <div class="modal" (click)="$event.stopPropagation()">
        <ng-content />
      </div>
    </div>
  `,
  styles: `
    .modal-overlay {
      position: fixed;
      inset: 0;
      background: rgba(0, 0, 0, 0.4);
      display: flex;
      align-items: center;
      justify-content: center;
      z-index: 100;
    }

    .modal {
      background: var(--color-bg);
      border-radius: var(--radius-xl);
      padding: var(--space-2xl);
      max-width: 600px;
      width: 90%;
      max-height: 80vh;
      overflow-y: auto;
    }
  `,
})
export class Modal {
  readonly closed = output();
}
