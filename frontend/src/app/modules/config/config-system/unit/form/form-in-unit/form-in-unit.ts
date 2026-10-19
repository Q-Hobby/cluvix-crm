import { ChangeDetectionStrategy, Component, inject } from '@angular/core';
import { FormField, required, requiredError, SchemaPathTree, validate } from '@angular/forms/signals';

import { UnitModel } from '@/core/models/config/system/unit.model';
import { UnitService } from '@/core/services/config/system/unit.service';
import { BaseSignalForm } from '@/share/base/base-signal-form';
import { FormShareImports } from '@/share/import/form-share-imports';
import { UNIT_CATEGORY_OPTIONS } from '@/define';

@Component({
  changeDetection: ChangeDetectionStrategy.OnPush,
  selector: 'app-unit-form',
  templateUrl: './form-in-unit.html',
  styles: [],
  imports: [...FormShareImports, FormField],
  standalone: true,
})
export class FormInUnit extends BaseSignalForm<UnitModel> {
  modelService = inject(UnitService);
  protected readonly UNIT_CATEGORY_OPTIONS = UNIT_CATEGORY_OPTIONS;

  protected override buildSchema(p: SchemaPathTree<UnitModel>): void {
    required(p.name!);
    // `categoryId` mảng bắt buộc chọn ≥1 — `required()` coi mảng rỗng là hợp lệ (gotcha #10
    // angular22.md), khác `Validators.required` gốc. Dùng `validate()` thủ công.
    validate(p.categoryId!, ({ value }) => ((value()?.length ?? 0) === 0 ? requiredError() : null));
  }
}
