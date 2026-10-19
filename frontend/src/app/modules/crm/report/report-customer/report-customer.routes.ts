import {Routes} from '@angular/router';

import {BranchStatistics} from '@/modules/crm/report/report-customer/branch-statistics/branch-statistics';
import {CategoryStatistics} from '@/modules/crm/report/report-customer/category-statistics/category-statistics';
import {CountryStatistics} from '@/modules/crm/report/report-customer/country-statistics/country-statistics';
import {InterestedStatistics} from '@/modules/crm/report/report-customer/interested-statistics/interested-statistics';
import {ProvinceStatistics} from '@/modules/crm/report/report-customer/province-statistics/province-statistics';
import {SourceStatistics} from '@/modules/crm/report/report-customer/source-statistics/source-statistics';

export default [
  {
    path: 'group-source',
    loadComponent: () => import('./group-source-statistics/group-source-statistics').then(m => m.GroupSourceStatistics),
    data: {breadcrumb: 'common.label.customer_source_group',},
  },
  {
    path: 'customer-interested',
    component: InterestedStatistics,
    data: {
      breadcrumb: 'common.label.service_group_of_interest',
    },
  },
  {
    path: 'customer-source',
    component: SourceStatistics,
    data: {
      breadcrumb: 'common.label.source',
    },
  },
  {
    path: 'province-statistics',
    component: ProvinceStatistics,
    data: {
      breadcrumb: 'crm.report.customer_stats.h_province',
    },
  },
  {
    path: 'country-statistics',
    component: CountryStatistics,
    data: {
      breadcrumb: 'common.label.province_city',
    },
  },
  {
    path: 'branch-statistics',
    component: BranchStatistics,
    data: {
      breadcrumb: 'common.label.branch',
    },
  },
  {
    path: 'category-statistics',
    component: CategoryStatistics,
    data: {
      breadcrumb: 'common.label.customer_category',
    },
  },
] as Routes;
