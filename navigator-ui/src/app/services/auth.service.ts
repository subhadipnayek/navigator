import { isPlatformBrowser } from '@angular/common';
import { HttpClient } from '@angular/common/http';
import { Injectable, PLATFORM_ID, computed, inject, signal, untracked } from '@angular/core';
import { Observable, tap } from 'rxjs';
import { ApiConfig } from '../core/api-config';
import {
  AuthResponse,
  AuthUser,
  LoginRequest,
  RegisterRequest,
  UpdateUserRoleRequest,
} from '../models/auth.model';

const STORAGE_KEY = 'navigator_auth';

@Injectable({ providedIn: 'root' })
export class AuthService {
  private readonly http = inject(HttpClient);
  private readonly api = inject(ApiConfig);
  private readonly platformId = inject(PLATFORM_ID);
  private readonly state = signal<AuthResponse | null>(this.readStoredAuth());

  // These computed signals are now safe because currentAuth() handles side-effects silently
  readonly user = computed<AuthUser | null>(() => this.currentAuth()?.user ?? null);
  readonly token = computed<string | null>(() => this.currentAuth()?.token ?? null);
  readonly isAuthenticated = computed(() => this.token() !== null);
  readonly isAdmin = computed(() => this.user()?.role === 'Admin');

  login(request: LoginRequest): Observable<AuthResponse> {
    return this.http
      .post<AuthResponse>(`${this.api.baseUrl}/api/auth/login`, request)
      .pipe(tap((response) => this.setAuth(response)));
  }

  register(request: RegisterRequest): Observable<AuthResponse> {
    return this.http
      .post<AuthResponse>(`${this.api.baseUrl}/api/auth/register`, request)
      .pipe(tap((response) => this.setAuth(response)));
  }

  getUsers(): Observable<AuthUser[]> {
    return this.http.get<AuthUser[]>(`${this.api.baseUrl}/api/auth/users`);
  }

  updateUserRole(userId: number, request: UpdateUserRoleRequest): Observable<AuthUser> {
    return this.http.put<AuthUser>(`${this.api.baseUrl}/api/auth/users/${userId}/role`, request);
  }

  logout(): void {
    this.state.set(null);
    if (isPlatformBrowser(this.platformId)) {
      localStorage.removeItem(STORAGE_KEY);
    }
  }

  private setAuth(response: AuthResponse): void {
    this.state.set(response);
    if (isPlatformBrowser(this.platformId)) {
      localStorage.setItem(STORAGE_KEY, JSON.stringify(response));
    }
  }

  /**
   * REPAIRED: Logic wrapped in untracked()
   */
  private currentAuth(): AuthResponse | null {
    const auth = this.state();

    // Check if auth exists and if it is expired
    const isExpired = auth && new Date(auth.expiresAtUtc).getTime() <= Date.now();

    if (!auth || isExpired) {
      if (auth) {
        // untracked allows us to call logout (which modifies state)
        // without crashing the computed signal chain.
        untracked(() => this.logout());
      }
      return null;
    }

    return auth;
  }

  private readStoredAuth(): AuthResponse | null {
    if (!isPlatformBrowser(this.platformId)) return null;

    try {
      const raw = localStorage.getItem(STORAGE_KEY);
      return raw ? (JSON.parse(raw) as AuthResponse) : null;
    } catch {
      localStorage.removeItem(STORAGE_KEY);
      return null;
    }
  }
}
