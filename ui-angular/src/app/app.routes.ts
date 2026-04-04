import { Routes } from '@angular/router';

export const routes: Routes = [
  {
    path: '',
    loadComponent: () => import('./pages/dashboard/dashboard').then((m) => m.Dashboard),
  },
  {
    path: 'funds',
    loadComponent: () => import('./pages/fund-list/fund-list').then((m) => m.FundList),
  },
  {
    path: 'funds/:schemeCode',
    loadComponent: () => import('./pages/fund-detail/fund-detail').then((m) => m.FundDetail),
  },
  {
    path: 'funds/:schemeCode/rolling',
    loadComponent: () =>
      import('./pages/rolling-returns/rolling-returns').then((m) => m.RollingReturns),
  },
  {
    path: 'compare',
    loadComponent: () => import('./pages/compare/compare').then((m) => m.Compare),
  },
  {
    path: 'admin/sync',
    loadComponent: () => import('./pages/admin-sync/admin-sync').then((m) => m.AdminSync),
  },
  {
    path: 'admin/corporate-actions',
    loadComponent: () =>
      import('./pages/corporate-actions/corporate-actions').then((m) => m.CorporateActions),
  },
  {
    path: '**',
    redirectTo: '',
  },
];
