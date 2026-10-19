import { Service} from '@angular/core';
import { BaseCacheService } from '@/indexeddb/config/components/base';
import { NoteTemplateModel } from '@/core/models/config/marketing/note-template.model';

@Service()
export class NoteTemplateCacheService extends BaseCacheService<NoteTemplateModel> {
  protected tableName = 'noteTemplates';
  protected activeStatus = 1;
  protected allowedStatuses = [1];

  async ensureNoteTemplateLoaded(): Promise<void> {
    return this.loadItems();
  }

  getAll(): NoteTemplateModel[] {
    return this.getAllItems()
      .filter((item) => item.status === 1)
      .sort((a, b) => (b.SortOrder ?? 0) - (a.SortOrder ?? 0));
  }

  getByTypeIdNewest(typeId: number): NoteTemplateModel | undefined {
    return this.getAllItems()
      .filter((item) => item.type_id === typeId && item.status === 1)
      .sort((a, b) => (b.SortOrder ?? 0) - (a.SortOrder ?? 0))[0];
  }

  protected override generateCacheKey(): string {
    return 'all';
  }
}
