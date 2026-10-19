import { ChangeDetectionStrategy, Component, inject } from '@angular/core';
import { FormField, required, SchemaPathTree } from '@angular/forms/signals';

import { SourceModel } from '@/core/models/config/marketing/source.model';
import { SourceService } from '@/core/services/config/marketing/source.service';
import { BaseSignalForm } from '@/share/base/base-signal-form';
import { FormShareImports } from '@/share/import/form-share-imports';

@Component({
  changeDetection: ChangeDetectionStrategy.OnPush,
  selector: 'app-source-form',
  templateUrl: './source-form.html',
  imports: [...FormShareImports, FormField],
  standalone: true,
})
export class SourceForm extends BaseSignalForm<SourceModel> {
  modelService = inject(SourceService);

  protected override buildSchema(p: SchemaPathTree<SourceModel>): void {
    required(p.name!);
  }
}
