import { inject, Injectable } from '@angular/core';
import { MatSnackBar } from '@angular/material/snack-bar';

@Injectable({ providedIn: 'root' })
export class ToastService {
  private readonly snackBar = inject(MatSnackBar);

  success(message: string): void {
    this.snackBar.open(message, 'Close', { duration: 4000 });
  }

  error(message: string): void {
    this.snackBar.open(message, 'Close', { duration: 6000 });
  }

  info(message: string): void {
    this.snackBar.open(message, 'Close', { duration: 4000 });
  }
}
