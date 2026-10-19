import { ChangeDetectionStrategy, Component, inject } from '@angular/core';
import { FormField, required, SchemaPathTree } from '@angular/forms/signals';

import { BaseSignalForm } from '@/share/base/base-signal-form';
import { FormShareImports } from '@/share/import/form-share-imports';
import { JobModel } from '@/core/models/config/marketing/job.model';
import { JobService } from '@/core/services/config/marketing/job.service';

@Component({
  changeDetection: ChangeDetectionStrategy.OnPush,
  selector: 'app-job-form',
  templateUrl: './job-form.html',
  imports: [...FormShareImports, FormField],
  standalone: true,
})
export class JobForm extends BaseSignalForm<JobModel> {
  modelService = inject(JobService);

  protected override buildSchema(p: SchemaPathTree<JobModel>): void {
    required(p.name!);
  }
}
