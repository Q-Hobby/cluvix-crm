import { Routes } from '@angular/router';

export default [
  {
    path: 'group-local',
    loadComponent: () =>
      import('./group-local/list/group-local-list').then((c) => c.GroupLocalList),
    data: { breadcrumb: 'common.label.domestic_area' },
  },
  {
    path: 'group-country',
    loadComponent: () =>
      import('./group-country/list/group-country-list').then((c) => c.GroupCountryList),
    data: { breadcrumb: 'common.label.international_area' },
  },
  {
    path: 'location-country',
    loadComponent: () =>
      import('./location-country/list/location-country-list').then((c) => c.LocationCountryList),
    data: { breadcrumb: 'common.label.province_city' },
  },
  {
    path: 'location-province',
    loadComponent: () =>
      import('./location-province/list/location-province-list').then((c) => c.LocationProvinceList),
    data: { breadcrumb: 'common.label.province_city' },
  },
  {
    path: 'location-ward',
    loadComponent: () =>
      import('./location-ward/list/location-ward-list').then((c) => c.LocationWardList),
    data: { breadcrumb: 'common.label.ward_commune' },
  },
] as Routes;
