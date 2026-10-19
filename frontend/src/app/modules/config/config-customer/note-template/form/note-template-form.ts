import { ChangeDetectionStrategy, Component, inject } from '@angular/core';
import { FormField, required, SchemaPathTree } from '@angular/forms/signals';

import { NoteTemplateModel } from '@/core/models/config/marketing/note-template.model';
import { NoteTemplateService } from '@/core/services/config/marketing/note-template.service';
import { BaseSignalForm } from '@/share/base/base-signal-form';
import { FormShareImports } from '@/share/import/form-share-imports';
import { NOTE_TEMPLATE_TYPE_OPTIONS } from '@/define';

@Component({
  changeDetection: ChangeDetectionStrategy.OnPush,
  selector: 'app-note-template-form',
  templateUrl: './note-template-form.html',
  styles: [],
  imports: [...FormShareImports, FormField],
  standalone: true,
})
export class NoteTemplateForm extends BaseSignalForm<NoteTemplateModel> {
  modelService = inject(NoteTemplateService);
  protected readonly NOTE_TEMPLATE_TYPE_OPTIONS = NOTE_TEMPLATE_TYPE_OPTIONS;

  // `content` bind `p-editor` — chặn base64 nhúng lọt xuống DB (upload S3 trước submit, base onSubmit() lo phần còn lại)
  protected override richTextImageFields(): (keyof NoteTemplateModel & string)[] {
    return ['content'];
  }

  protected override buildSchema(p: SchemaPathTree<NoteTemplateModel>): void {
    required(p.name!);
  }
}
