import { Routes } from '@angular/router';

export default [
  {
    path: 'accessdenied',
    loadComponent: () => import('./accessdenied/accessdenied').then((c) => c.AccessDenied),
  },
  {
    path: 'notfound',
    loadComponent: () => import('./notfound/notfound').then((c) => c.Notfound),
  },
  {
    path: 'lockscreen',
    loadComponent: () => import('./lockscreen/lockscreen').then((c) => c.LockScreen),
  },
] as Routes;
