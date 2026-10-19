import { ChangeDetectionStrategy, Component, input, output } from '@angular/core';

import type { DashboardZone } from './dashboard-widgets';
import type { DashboardLayoutItem } from './layout.util';
import { DashboardWidgetEditChrome, type WidgetMoveCommand } from './widget-edit-chrome';
import { WidgetSlotDirective } from './widget-view-pool';

/**
 * 1 khối trong 1 vùng. `cdkDrag` đặt lên CHÍNH phần tử này ngay trong `@for` của vùng (dashboard.html): CdkDrag tìm
 * `cdkDropList` cha qua injector của chỗ KHAI — bọc bằng `ng-template` + `ngTemplateOutlet` khai ở cấp trang thì không
 * tìm thấy vùng. Tay kéo (trong chrome) là con của phần tử `cdkDrag` nên tìm được nó. KHÔNG khai `*cdkDragPlaceholder`
 * riêng: CDK chỉ tự khớp cỡ khi clone phần tử gốc ⇒ khung giữ chỗ cao đúng bằng khối + giữ `md:col-span-2` (QA P1).
 * `[appWidgetSlot]` chỉ gắn view khối từ pool ⇒ đổi vùng không mount lại.
 */
@Component({
  selector: 'app-dashboard-zone-item',
  changeDetection: ChangeDetectionStrategy.OnPush,
  imports: [DashboardWidgetEditChrome, WidgetSlotDirective],
  host: { class: 'block' },
  styleUrl: './zone-item.scss',
  template: `
    @if (editing()) {
      <app-dashboard-widget-edit-chrome [key]="item().key" [zone]="zone()" [size]="item().size" [index]="index()" [count]="count()"
                                        [draggable]="draggable()" [busy]="busy()"
                                        (hide)="hide.emit()" (toggleSize)="toggleSize.emit()" (move)="move.emit($event)" />
    }
    <div [appWidgetSlot]="item().key"></div>
  `,
})
export class DashboardZoneItem {
  readonly item = input.required<DashboardLayoutItem>();
  readonly zone = input.required<DashboardZone>();
  readonly index = input.required<number>();
  readonly count = input.required<number>();
  readonly editing = input(false);
  readonly draggable = input(false);
  readonly busy = input(false);

  readonly hide = output<void>();
  readonly toggleSize = output<void>();
  readonly move = output<WidgetMoveCommand>();
}
