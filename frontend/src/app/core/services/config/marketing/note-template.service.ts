import { Service} from '@angular/core';

import { NoteTemplateModel } from '@/core/models/config/marketing/note-template.model';
import { BaseApiService } from '@/share/services/base-api.service';

@Service()
export class NoteTemplateService extends BaseApiService<NoteTemplateModel> {
  protected override apiModule = 'config';
  protected override endpoint = 'note-template';
  protected override unixTimesTampFields = ['created_at', 'updated_at'];

  override getInitialObject() {
    return {
      name: '',
      type_id: null,
      content: null,
      status: 1,
    };
  }
}
