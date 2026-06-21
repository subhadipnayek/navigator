import { Routes } from '@angular/router';
import { adminGuard, authGuard } from './core/auth.guard';

export const routes: Routes = [
  {
    path: '',
    loadComponent: () => import('./pages/landing/landing').then((m) => m.Landing),
  },
  {
    path: 'dashboard',
    canActivate: [authGuard],
    loadComponent: () => import('./pages/dashboard/dashboard').then((m) => m.Dashboard),
  },
  {
    path: 'funds',
    canActivate: [authGuard],
    loadComponent: () => import('./pages/fund-list/fund-list').then((m) => m.FundList),
  },
  {
    path: 'funds/:schemeCode',
    canActivate: [authGuard],
    loadComponent: () => import('./pages/fund-detail/fund-detail').then((m) => m.FundDetail),
  },
  {
    path: 'funds/:schemeCode/rolling',
    canActivate: [authGuard],
    loadComponent: () =>
      import('./pages/rolling-returns/rolling-returns').then((m) => m.RollingReturns),
  },
  {
    path: 'compare',
    canActivate: [authGuard],
    loadComponent: () => import('./pages/compare/compare').then((m) => m.Compare),
  },
  {
    path: 'portfolio-lab',
    canActivate: [authGuard],
    loadComponent: () => import('./pages/portfolio-lab/portfolio-lab').then((m) => m.PortfolioLab),
  },
  {
    path: 'portfolio-dashboard',
    canActivate: [authGuard],
    loadComponent: () =>
      import('./pages/user-portfolio/user-portfolio').then((m) => m.UserPortfolio),
  },
  {
    path: 'login',
    loadComponent: () => import('./pages/login/login').then((m) => m.Login),
  },
  {
    path: 'register',
    loadComponent: () => import('./pages/register/register').then((m) => m.Register),
  },
  {
    path: 'admin',
    canActivate: [authGuard, adminGuard],
    children: [
      {
        path: '',
        redirectTo: 'anomalies',
        pathMatch: 'full',
      },
      {
        path: 'funds',
        loadComponent: () => import('./pages/admin-sync/admin-sync').then((m) => m.AdminSync),
      },
      {
        path: 'anomalies',
        loadComponent: () =>
          import('./pages/corporate-actions/corporate-actions').then((m) => m.CorporateActions),
      },
    ],
  },

  {
    path: '**',
    redirectTo: '',
  },
];
