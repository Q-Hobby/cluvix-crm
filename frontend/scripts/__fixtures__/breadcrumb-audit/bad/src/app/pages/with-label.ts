// @ts-nocheck
import {inject} from '@angular/core';

import {BreadcrumbLabelService} from '@/share/services/breadcrumb-label.service';

export class WithLabel {
  private readonly breadcrumbLabelService = inject(BreadcrumbLabelService);
}
