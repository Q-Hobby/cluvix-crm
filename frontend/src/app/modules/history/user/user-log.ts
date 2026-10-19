import { ChangeDetectionStrategy, Component, effect, inject, OnInit, ViewChild } from '@angular/core';
import { Table } from 'primeng/table';
import { Observable, switchMap } from 'rxjs';

import { BaseListV2 } from '@/share/base/base-list-v2';
import { FilterFieldDef } from '@/share/base/list-filter.model';
import { ListApiResponse } from '@/share/models/list-api-response';
import { ListShareImports } from '@/share/import/list-share-imports';
import { LogsService } from '@/core/services/history/logs.service';
import { ActivatedRoute } from '@angular/router';
import { BranchHelperService } from '@/indexeddb/config/components/branch';
import { getUserStatusLabel } from '@/define';
import { FillHeightDirective } from '@/share/directive/fill-height.directive';

@Component({
  changeDetection: ChangeDetectionStrategy.OnPush,
  selector: 'app-user-log',
  templateUrl: './user-log.html',
  standalone: true,
  imports: [FillHeightDirective, ...ListShareImports],
})
export class UserLog extends BaseListV2<any> implements OnInit {
  modelService = inject(LogsService);
  router = inject(ActivatedRoute);
  @ViewChild('dt') dt!: Table;
  protected override URL_STATE_PREFIX = 'lu';
  private branchHelper = inject(BranchHelperService);
  private fieldLabels: Record<string, string> = {
    username: 'Tên đăng nhập',
    status: 'Trạng thái',
    branchId: 'Chi nhánh',
    allowed_ips: 'Danh sách IP cho phép',
  };

  constructor() {
    super();
    effect(() => {
      this.padCols.set(7);
    });
  }

  ngOnInit() {
    this.requiredSearch.set({ paramUrl: this.router.snapshot.url.at(-1)?.path });
    this.initListV2();
  }

  protected filterFields(): FilterFieldDef[] {
    return [{ name: '_keyword', kind: 'text' }]; // urlKey tự suy → 'keyword'
  }

  protected override fetchList(): Observable<ListApiResponse<any>> {
    return this.modelService.listLogs({ params: this.searchParams() }).pipe(
      switchMap(async (response: any) => {
        if (response.items) {
          const items = response.items ?? [];
          response.items = await this.modelService.addLabels(items);
          response.items.forEach((item: any) => {
            item.changed_fields = item.changed_fields.map((field: any) => ({
              key: this.fieldLabels[field.key] || field.key,
              value: {
                from: this.formatFieldValue(field.key, field.value?.from),
                to: this.formatFieldValue(field.key, field.value?.to),
              },
            }));
          });
        }
        return response;
      }),
    );
  }

  private parseJsonArray(value: any): any[] {
    if (Array.isArray(value)) return value;
    if (typeof value === 'string') {
      try {
        const parsed = JSON.parse(value);
        return Array.isArray(parsed) ? parsed : [];
      } catch {
        return [];
      }
    }
    return [];
  }

  private formatFieldValue(key: string, value: any): any {
    if (value === null || value === undefined) {
      return value;
    }
    if (key === 'status' && typeof value === 'number') {
      return getUserStatusLabel(value);
    }
    if (key === 'branchId') {
      return this.parseJsonArray(value)
        .map((id) => this.branchHelper.getBranchName(id))
        .join(', ');
    }
    if (key === 'allowed_ips') {
      const ips = this.parseJsonArray(value);
      return ips.length ? ips.join(', ') : value;
    }
    return value;
  }
}
