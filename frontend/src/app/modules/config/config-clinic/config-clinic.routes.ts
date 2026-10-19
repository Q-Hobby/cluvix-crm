import { Routes } from '@angular/router';

import { ServiceList } from '@/modules/config/config-clinic/service/list/service-list';
import { ServiceTypeList } from '@/modules/config/config-clinic/service-type/list/service-type-list';

// story-20: config-clinic trim theo architecture.md §1 keep-list (`service`, `service_type`) —
// treatment_course_type/treatment_category/warranty_card_*/service_combo*/service_treatment_step
// nằm ở drop-list (treatment_*, service_{combo,combo_type,material,treatment_step}, warranty_*),
// đã xoá cùng thư mục con.
export default [
  {
    path: 'service-type',
    component: ServiceTypeList,
    data: { breadcrumb: 'common.label.service_product_group' },
  },
  {
    path: 'service',
    component: ServiceList,
    data: { breadcrumb: 'common.label.service_product' },
  },
] as Routes;
