import { ChangeDetectionStrategy, ChangeDetectorRef, Component, inject } from '@angular/core';
import { FormField, required, SchemaPathTree } from '@angular/forms/signals';

import { GroupSourceModel } from '@/core/models/config/marketing/group-source.model';
import { GroupSourceService } from '@/core/services/config/marketing/group-source.service';
import { BaseSignalForm } from '@/share/base/base-signal-form';
import { FormShareImports } from '@/share/import/form-share-imports';

@Component({
  changeDetection: ChangeDetectionStrategy.OnPush,
  selector: 'app-group-source-form-in-source',
  templateUrl: './group-source-form-in-source.html',
  styles: [],
  imports: [...FormShareImports, FormField],
  standalone: true,
})
export class GroupSourceFormInSource extends BaseSignalForm<GroupSourceModel> {
  modelService = inject(GroupSourceService);
  private cdr = inject(ChangeDetectorRef);

  constructor() {
    super();

    // 💥 Fix lỗi ExpressionChanged... (đối chiếu bản gốc).
    this.cdr.detectChanges();
  }

  protected override buildSchema(p: SchemaPathTree<GroupSourceModel>): void {
    required(p.name!);
  }
}
