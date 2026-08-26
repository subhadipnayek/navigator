import { HttpErrorResponse, HttpInterceptorFn } from '@angular/common/http';
import { inject } from '@angular/core';
import { Router } from '@angular/router';
import { catchError, throwError } from 'rxjs';
import { AuthService } from '../services/auth.service';
import { ToastService } from '../shared/toast/toast.service';

export const authInterceptor: HttpInterceptorFn = (req, next) => {
  const authService = inject(AuthService);
  const router = inject(Router);
  const toast = inject(ToastService);
  const token = authService.token();

  const authReq = token
    ? req.clone({
        setHeaders: {
          Authorization: `Bearer ${token}`,
        },
      })
    : req;

  return next(authReq).pipe(
    catchError((error: HttpErrorResponse) => {
      // If unauthorized and not a login/registration attempt, auto logout and navigate to login
      if (
        error.status === 401 &&
        !req.url.includes('/api/auth/login') &&
        !req.url.includes('/api/auth/register')
      ) {
        authService.logout();
        toast.info('Your session has expired. Please log in again.');
        router.navigate(['/login']);
      }
      return throwError(() => error);
    }),
  );
};
