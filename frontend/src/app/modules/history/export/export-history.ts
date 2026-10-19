import { ChangeDetectionStrategy, Component, effect, inject, OnInit, ViewChild } from '@angular/core';
import { Table } from 'primeng/table';
import { Observable, switchMap } from 'rxjs';

import { ExportHistoryModel } from '@/core/models/history/export-history.model';
import { ExportHistoryService } from '@/core/services/history/export-history.service';
import { BaseListV2 } from '@/share/base/base-list-v2';
import { FilterFieldDef } from '@/share/base/list-filter.model';
import { ListApiResponse } from '@/share/models/list-api-response';
import { ListShareImports } from '@/share/import/list-share-imports';
import { FillHeightDirective } from '@/share/directive/fill-height.directive';

@Component({
  changeDetection: ChangeDetectionStrategy.OnPush,
  selector: 'app-history-export',
  templateUrl: './export-history.html',
  standalone: true,
  imports: [FillHeightDirective, ...ListShareImports],
})
export class ExportHistory extends BaseListV2<ExportHistoryModel> implements OnInit {
  modelService = inject(ExportHistoryService);
  @ViewChild('dt') dt!: Table;
  protected override URL_STATE_PREFIX = 'ex';

  constructor() {
    super();
    effect(() => {
      this.padCols.set(7);
    });
  }

  ngOnInit() {
    this.initListV2();
  }

  // D-1: BE ListHistoryExport nay đã đọc _keyword qua query.ApplyKeywordFilter (positionExport) —
  // khai lại control đã gỡ ở story-06 lúc BE còn chưa hỗ trợ (xem deferred-work.md D-1).
  protected filterFields(): FilterFieldDef[] {
    return [{ name: '_keyword', kind: 'text' }];
  }

  protected override fetchList(): Observable<ListApiResponse<ExportHistoryModel>> {
    return this.modelService.list({ params: this.searchParams() }).pipe(
      switchMap(async (response: any) => {
        if (response.items) {
          response.items = await this.modelService.addLabels(response.items ?? []);
        }
        return response;
      }),
    );
  }
}
