import { ChangeDetectionStrategy, Component, inject } from '@angular/core';
import { FormField, required, requiredError, SchemaPathTree, validate } from '@angular/forms/signals';

import { GroupSourceModel } from '@/core/models/config/marketing/group-source.model';
import { GroupSourceService } from '@/core/services/config/marketing/group-source.service';
import { BaseSignalForm } from '@/share/base/base-signal-form';
import { FormShareImports } from '@/share/import/form-share-imports';
import { SourceMultiselectFormControl } from '@/indexeddb/config/components/source';

@Component({
  changeDetection: ChangeDetectionStrategy.OnPush,
  selector: 'app-group-source-form',
  templateUrl: './group-source-form.html',
  styles: [],
  imports: [...FormShareImports, FormField, SourceMultiselectFormControl],
  standalone: true,
})
export class GroupSourceForm extends BaseSignalForm<GroupSourceModel> {
  modelService = inject(GroupSourceService);

  protected override buildSchema(p: SchemaPathTree<GroupSourceModel>): void {
    required(p.name!);
    // required() KHÔNG coi mảng rỗng là invalid (gotcha #10 dev-notes-common.md) — sourceId là
    // number[] bắt buộc chọn ≥1 (gốc `Validators.required`), phải tự validate().
    validate(p.sourceId!, ({ value }) => ((value()?.length ?? 0) === 0 ? requiredError() : null));
  }
}
