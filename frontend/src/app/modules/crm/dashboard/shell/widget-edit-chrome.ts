import { CdkDragHandle } from '@angular/cdk/drag-drop';
import { ChangeDetectionStrategy, Component, computed, inject, input, output } from '@angular/core';
import { toSignal } from '@angular/core/rxjs-interop';
import { TranslatePipe, TranslateService } from '@ngx-translate/core';
import { MenuItem } from 'primeng/api';
import { MenuModule } from 'primeng/menu';
import type { Observable } from 'rxjs';

import { CRM_DASHBOARD_WIDGET_MAP, type DashboardSize, type DashboardZone } from './dashboard-widgets';

/** Lệnh của menu di chuyển — thay kéo thả cho bàn phím / màn hình nhỏ (story-10 AC3, PRD FR26). */
export type WidgetMoveCommand = { kind: 'up' } | { kind: 'down' } | { kind: 'zone'; zone: DashboardZone };

const LABEL_KEYS = [
  'dashboard.crm.layout.move_up',
  'dashboard.crm.layout.move_down',
  'dashboard.crm.layout.to_main',
  'dashboard.crm.layout.to_side',
  'dashboard.crm.layout.to_top',
] as const;

/** Mục menu di chuyển hợp lệ theo vị trí hiện tại (thuần — test được). */
export function moveCommands(zone: DashboardZone, index: number, count: number): { cmd: WidgetMoveCommand; labelKey: string; icon: string }[] {
  const out: { cmd: WidgetMoveCommand; labelKey: string; icon: string }[] = [];
  if (index > 0) out.push({ cmd: { kind: 'up' }, labelKey: LABEL_KEYS[0], icon: 'pi pi-arrow-up' });
  if (index < count - 1) out.push({ cmd: { kind: 'down' }, labelKey: LABEL_KEYS[1], icon: 'pi pi-arrow-down' });
  if (zone !== 'main') out.push({ cmd: { kind: 'zone', zone: 'main' }, labelKey: LABEL_KEYS[2], icon: 'pi pi-arrow-left' });
  if (zone !== 'side') out.push({ cmd: { kind: 'zone', zone: 'side' }, labelKey: LABEL_KEYS[3], icon: 'pi pi-arrow-right' });
  if (zone !== 'top') out.push({ cmd: { kind: 'zone', zone: 'top' }, labelKey: LABEL_KEYS[4], icon: 'pi pi-arrow-up-right' });
  return out;
}

/**
 * Thanh điều khiển 1 khối ở chế độ sửa: tay kéo (`<button cdkDragHandle>` — phải nằm TRONG phần tử `cdkDrag` để tìm được
 * khối cha qua injector, nên là component chứ không phải ng-template), tên khối, đổi cỡ (chỉ vùng chính), menu di chuyển,
 * ẩn. Tay kéo ẩn ở < md (không kéo thả trên mobile — dùng menu).
 */
@Component({
  selector: 'app-dashboard-widget-edit-chrome',
  changeDetection: ChangeDetectionStrategy.OnPush,
  imports: [CdkDragHandle, MenuModule, TranslatePipe],
  host: { class: 'flex items-center gap-1 mb-1.5 min-w-0' },
  template: `
    @if (draggable()) {
      <button type="button" cdkDragHandle
              class="dash-chrome-btn cursor-grab active:cursor-grabbing"
              [attr.aria-label]="'dashboard.crm.layout.drag_handle' | translate: { name: (titleKey() | translate) }">
        <i class="pi pi-arrows-alt" aria-hidden="true"></i>
      </button>
    }
    <span class="min-w-0 flex-1 truncate text-xs font-medium text-[var(--p-text-muted-color)]" [attr.title]="titleKey() | translate">{{ titleKey() | translate }}</span>
    @if (zone() === 'main') {
      <button type="button" class="dash-chrome-btn" [disabled]="busy()" (click)="toggleSize.emit()"
              [attr.aria-label]="(size() === 'full' ? 'dashboard.crm.layout.size_half' : 'dashboard.crm.layout.size_full') | translate"
              [attr.title]="(size() === 'full' ? 'dashboard.crm.layout.size_half' : 'dashboard.crm.layout.size_full') | translate">
        <i [class]="size() === 'full' ? 'pi pi-window-minimize' : 'pi pi-window-maximize'" aria-hidden="true"></i>
      </button>
    }
    <button type="button" class="dash-chrome-btn" data-move-btn [disabled]="busy()" aria-haspopup="menu"
            [attr.aria-label]="'dashboard.crm.layout.move' | translate" [attr.title]="'dashboard.crm.layout.move' | translate"
            (click)="moveMenuRef.toggle($event)">
      <i class="pi pi-sort-alt" aria-hidden="true"></i>
    </button>
    <button type="button" class="dash-chrome-btn" [disabled]="busy()" (click)="hide.emit()"
            [attr.aria-label]="('common.label.hide' | translate) + ': ' + (titleKey() | translate)" [attr.title]="'common.label.hide' | translate">
      <i class="pi pi-eye-slash" aria-hidden="true"></i>
    </button>
    <p-menu #moveMenuRef [model]="menuItems()" [popup]="true" appendTo="body" />
  `,
  styleUrl: './widget-edit-chrome.scss',
})
export class DashboardWidgetEditChrome {
  readonly key = input.required<string>();
  readonly zone = input.required<DashboardZone>();
  readonly size = input.required<DashboardSize>();
  readonly index = input.required<number>();
  readonly count = input.required<number>();
  readonly draggable = input(true);
  readonly busy = input(false);

  readonly hide = output<void>();
  readonly toggleSize = output<void>();
  readonly move = output<WidgetMoveCommand>();

  protected readonly titleKey = computed(() => CRM_DASHBOARD_WIDGET_MAP.get(this.key())?.titleKey ?? this.key());

  // Nhãn MenuItem là chuỗi ⇒ `stream` để theo ngôn ngữ và bản dịch nạp muộn (khuôn widget-frame).
  private readonly labels = toSignal(inject(TranslateService).stream([...LABEL_KEYS]) as Observable<Record<string, string>>, {
    initialValue: {} as Record<string, string>,
  });
  protected readonly menuItems = computed<MenuItem[]>(() => {
    const labels = this.labels();
    return moveCommands(this.zone(), this.index(), this.count()).map((m) => ({
      label: labels[m.labelKey] ?? m.labelKey,
      icon: m.icon,
      command: () => this.move.emit(m.cmd),
    }));
  });
}
