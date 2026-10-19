import { ChangeDetectionStrategy, Component, inject } from '@angular/core';
import { FormField, required, SchemaPathTree } from '@angular/forms/signals';

import { HrDepartmentModel } from '@/core/models/config/hr/hr-department.model';
import { HrDepartmentService } from '@/core/services/config/hr/hr-department.service';
import { BaseSignalForm } from '@/share/base/base-signal-form';
import { FormShareImports } from '@/share/import/form-share-imports';
import { UserFormControl } from '@/indexeddb/config/components/user/user-form-control';

@Component({
  changeDetection: ChangeDetectionStrategy.OnPush,
  selector: 'app-hr-department-form',
  templateUrl: './department-form.html',
  styles: [],
  imports: [...FormShareImports, FormField, UserFormControl],
  standalone: true,
})
export class DepartmentForm extends BaseSignalForm<HrDepartmentModel> {
  modelService = inject(HrDepartmentService);

  protected override buildSchema(p: SchemaPathTree<HrDepartmentModel>): void {
    required(p.name!);
  }
}
