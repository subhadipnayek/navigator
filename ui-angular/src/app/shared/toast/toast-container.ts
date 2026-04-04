import { Component, inject } from '@angular/core';
import { ToastService } from './toast.service';

@Component({
  selector: 'app-toast-container',
  template: `
    @for (toast of toastService.toasts(); track toast.id) {
      <div class="toast" [class]="'toast-' + toast.type" (click)="toastService.dismiss(toast.id)">
        <span class="toast-icon">
          @switch (toast.type) {
            @case ('success') {
              ✓
            }
            @case ('error') {
              ✕
            }
            @case ('info') {
              ℹ
            }
          }
        </span>
        <span class="toast-message">{{ toast.message }}</span>
      </div>
    }
  `,
  styles: `
    :host {
      position: fixed;
      bottom: var(--space-xl);
      right: var(--space-xl);
      z-index: 200;
      display: flex;
      flex-direction: column-reverse;
      gap: var(--space-sm);
      pointer-events: none;
    }

    .toast {
      display: flex;
      align-items: center;
      gap: var(--space-sm);
      padding: var(--space-md) var(--space-lg);
      border-radius: var(--radius-lg);
      font-size: var(--font-size-base);
      color: #fff;
      pointer-events: auto;
      cursor: pointer;
      animation: slide-in 0.25s ease-out;
      box-shadow: 0 4px 12px rgba(0, 0, 0, 0.15);
      max-width: 380px;
    }

    .toast-success {
      background: var(--color-positive);
    }

    .toast-error {
      background: var(--color-negative);
    }

    .toast-info {
      background: var(--color-primary);
    }

    .toast-icon {
      font-weight: 700;
      font-size: var(--font-size-lg);
      flex-shrink: 0;
    }

    .toast-message {
      line-height: 1.4;
    }

    @keyframes slide-in {
      from {
        opacity: 0;
        transform: translateX(100%);
      }
      to {
        opacity: 1;
        transform: translateX(0);
      }
    }
  `,
})
export class ToastContainer {
  readonly toastService = inject(ToastService);
}
