import { Service} from '@angular/core';
import { BaseApiService } from '@/share/services/base-api.service';
import { PrintTemplateModel } from '@/core/models/config/print-template.model';
import { PRINT_TEMPLATE_TYPE_OPTIONS } from '@/define/print-template.define';

@Service()
export class PrintTemplateService extends BaseApiService<PrintTemplateModel> {
  protected override apiModule = 'config';
  protected override endpoint = 'print-template';

  addLabel(item: PrintTemplateModel): PrintTemplateModel {
    const opt = PRINT_TEMPLATE_TYPE_OPTIONS.find((o) => o.value === item.template_type);
    return { ...item, templateTypeLabel: opt?.label ?? item.template_type };
  }

  async addLabels(items: PrintTemplateModel[]): Promise<PrintTemplateModel[]> {
    if (!Array.isArray(items)) return [];
    return items.map((i) => this.addLabel(i));
  }
}
