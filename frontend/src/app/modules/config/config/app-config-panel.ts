import { ChangeDetectionStrategy, Component, computed, input, output } from '@angular/core';

import { ListShareImports } from '@/share/import/list-share-imports';
import { ConfigMenuGroup, ConfigMenuItem } from '@/modules/config/config/config-menu.registry';
import { isItemVisible, PermissionContext } from '@/modules/config/config/config-menu-search';

@Component({
  changeDetection: ChangeDetectionStrategy.OnPush,
  selector: 'app-config-panel',
  standalone: true,
  templateUrl: './app-config-panel.html',
  imports: [...ListShareImports],
})
export class AppConfigPanel {
  readonly group = input.required<ConfigMenuGroup>();
  /**
   * Ngữ cảnh quyền truyền từ trang cha — panel KHÔNG tự đọc signal auth để cùng một phép lọc
   * (`isItemVisible`) chạy chung với thanh tìm kiếm, tránh hai nơi lệch nhau khi đổi quyền item.
   */
  readonly permissionContext = input.required<PermissionContext>();
  /** Bắn khi user mở một mục — trang cha ghi vào "Truy cập gần đây". */
  readonly visit = output<ConfigMenuItem>();

  readonly visibleItems = computed(() =>
    this.group().items.filter((item) => isItemVisible(this.group(), item, this.permissionContext())),
  );
}
