import { Injectable, PLATFORM_ID, inject } from '@angular/core';
import { isPlatformBrowser } from '@angular/common';
import { environment } from '../../environments/environment';

@Injectable({ providedIn: 'root' })
export class ApiConfig {
  private platformId = inject(PLATFORM_ID);

  get baseUrl(): string {
    if (isPlatformBrowser(this.platformId)) {
      return environment.apiBaseUrl;
    }
    // Server-side (Node.js SSR in Docker): use internal Docker service hostname
    return (typeof process !== 'undefined' && process.env['API_URL']) || environment.apiBaseUrl;
  }
}
