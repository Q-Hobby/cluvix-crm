import { ChangeDetectionStrategy, Component, inject } from '@angular/core';
import { disabled, FormField, min, required, SchemaPathTree } from '@angular/forms/signals';

import { ConfigCodeModel } from '@/core/models/config/system/config-code.model';
import { ConfigCodeService } from '@/core/services/config/system/config-code.service';
import { BaseSignalForm } from '@/share/base/base-signal-form';
import { FormShareImports } from '@/share/import/form-share-imports';

@Component({
  changeDetection: ChangeDetectionStrategy.OnPush,
  selector: 'app-config-code-form',
  templateUrl: './config-code-form.html',
  styles: [],
  imports: [...FormShareImports, FormField],
  standalone: true,
})
export class ConfigCodeForm extends BaseSignalForm<ConfigCodeModel> {
  modelService = inject(ConfigCodeService);
  // Giữ nguyên logic gốc (`BaseForm.blockSpace`) — chặn ký tự khoảng trắng khi gõ infix_code.
  protected readonly blockSpace: RegExp = /[^\s]/;

  protected override buildSchema(p: SchemaPathTree<ConfigCodeModel>): void {
    required(p.name!);
    required(p.parent_table!);
    required(p.infix_code!);
    required(p.serial_length!);
    disabled(p.name!);
    disabled(p.parent_table!);
    min(p.serial_length!, 1);
  }
}
