import { TestBed } from '@angular/core/testing';
import { provideHttpClient } from '@angular/common/http';
import { provideHttpClientTesting, HttpTestingController } from '@angular/common/http/testing';
import { AuthService } from './auth.service';
import { ApiConfig } from '../core/api-config';
import { AuthResponse } from '../models/auth.model';

describe('AuthService', () => {
  let service: AuthService;
  let httpMock: HttpTestingController;

  beforeEach(() => {
    localStorage.clear();
    TestBed.configureTestingModule({
      providers: [
        AuthService,
        ApiConfig,
        provideHttpClient(),
        provideHttpClientTesting(),
      ],
    });
    service = TestBed.inject(AuthService);
    httpMock = TestBed.inject(HttpTestingController);
  });

  afterEach(() => {
    httpMock.verify();
    localStorage.clear();
  });

  it('should initialize with no user and unauthenticated state', () => {
    expect(service.user()).toBeNull();
    expect(service.token()).toBeNull();
    expect(service.isAuthenticated()).toBeFalse();
    expect(service.isAdmin()).toBeFalse();
  });

  it('should log in and update reactive state', () => {
    const mockAuth: AuthResponse = {
      token: 'header.eyJleHAiOjI1Mzc2ODk2MDB9.sig',
      user: {
        id: 1,
        username: 'testuser',
        email: 'user@example.com',
        role: 'Admin',
      },
      expiresAtUtc: new Date(Date.now() + 3600000).toISOString(),
    };

    service.login({ usernameOrEmail: 'user@example.com', password: 'password123' }).subscribe((res) => {
      expect(res).toEqual(mockAuth);
    });

    const req = httpMock.expectOne((r) => r.url.endsWith('/api/auth/login'));
    expect(req.request.method).toBe('POST');
    req.flush(mockAuth);

    expect(service.isAuthenticated()).toBeTrue();
    expect(service.isAdmin()).toBeTrue();
    expect(service.user()?.email).toBe('user@example.com');
  });

  it('should log out and clear stored auth', () => {
    service.logout();
    expect(service.isAuthenticated()).toBeFalse();
    expect(service.user()).toBeNull();
  });
});
