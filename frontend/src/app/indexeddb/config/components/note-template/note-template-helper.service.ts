import { inject, Service} from '@angular/core';
import { NoteTemplateCacheService } from './note-template-cache.service';

@Service()
export class NoteTemplateHelperService {
  private cache = inject(NoteTemplateCacheService);

  async ensureNoteTemplateLoaded(): Promise<void> {
    await this.cache.ensureNoteTemplateLoaded();
  }

  getNoteTemplateName(id: number | null | undefined): string {
    if (!id) return '';
    const item = this.cache.getById(id);
    return item?.name || '-';
  }

  async getNoteTemplateByTypeIdNewest(typeId: number | null | undefined) {
    await this.cache.ensureNoteTemplateLoaded();
    return this.cache.getByTypeIdNewest(typeId!);
  }

  getNoteTemplateInfo(id: number) {
    const item = this.cache.getById(id);
    if (!item) return null;
    return {
      id: item.id,
      name: item.name,
      content: item.content,
    };
  }

  mapNoteTemplateNames(ids: number[]): string {
    if (!ids || ids.length === 0) return '';
    return ids
      .map((id) => this.getNoteTemplateName(id))
      .filter((name) => name)
      .join(', ');
  }
}
