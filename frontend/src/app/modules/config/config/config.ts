import { AfterViewInit, ChangeDetectionStrategy, Component, computed, effect, ElementRef, signal, viewChild } from '@angular/core';
import { toSignal } from '@angular/core/rxjs-interop';
import { LangChangeEvent } from '@ngx-translate/core';

import { BaseApp } from '@/share/base/BaseApp';
import { PERMISSION_SELECT, STORAGE_KEY_SELECT } from '@/define';
import { ListShareImports } from '@/share/import/list-share-imports';
import { AppConfigPanel } from '@/modules/config/config/app-config-panel';
import { CONFIG_MENU, ConfigMenuGroup } from '@/modules/config/config/config-menu.registry';
import {
  buildMenuIndex,
  ConfigMenuHit,
  isItemVisible,
  PermissionContext,
  searchMenu,
} from '@/modules/config/config/config-menu-search';
import { isSuperUser } from '@/core/services/auth';
import { userPermissions } from '@/core/services/auth/auth.state';

/** Số mục "Truy cập gần đây" giữ lại — đủ 1 hàng chip trên màn hình hẹp, không đẩy panel xuống. */
const CONFIG_RECENT_LIMIT = 8;

export function getTableConfig(): number {
  const width = window.innerWidth;

  if (width >= 2560) return 130;
  if (width >= 1920) return 120;
  if (width >= 1512) return 110;
  if (width >= 1440) return 95;
  if (width >= 1024) return 110;
  if (width >= 768) return 95;
  if (width >= 640) return 95;

  return 95;
}

@Component({
  changeDetection: ChangeDetectionStrategy.OnPush,
  selector: 'app-config-list',
  templateUrl: './config.html',
  imports: [...ListShareImports, AppConfigPanel],
  standalone: true,
})
export class Config extends BaseApp implements AfterViewInit {
  public permission = PERMISSION_SELECT;
  tableConfig = signal(getTableConfig());

  readonly searchInput = viewChild<ElementRef<HTMLInputElement>>('searchInput');

  /** Từ khoá đang gõ. 131 mục nên lọc đồng bộ — debounce chỉ làm gõ xong thấy trễ. */
  readonly query = signal('');
  /** Dòng đang chọn bằng bàn phím trong danh sách kết quả. */
  readonly activeIndex = signal(0);
  private readonly recentLinks = signal<string[]>(this.loadRecent());

  /**
   * Đổi ngôn ngữ phải dựng lại chỉ số (nhãn hiển thị đổi theo). `onLangChange` chỉ bắn khi CÓ đổi,
   * nên giá trị khởi tạo là `currentLang` đọc trực tiếp.
   */
  private readonly lang = toSignal(this.translate.onLangChange, {
    initialValue: { lang: this.translate.currentLang } as LangChangeEvent,
  });

  readonly permissionContext = computed<PermissionContext>(() => ({
    isSuperUser: isSuperUser(),
    granted: userPermissions(),
  }));

  private readonly menuIndex = computed(() => {
    this.lang();
    return buildMenuIndex(CONFIG_MENU, this.permissionContext(), (key) => this.translate.instant(key));
  });

  readonly results = computed(() => searchMenu(this.menuIndex(), this.query()));
  readonly isSearching = computed(() => this.query().trim().length > 0);

  /** Nhóm hiển thị theo cột khi KHÔNG tìm kiếm — bỏ nhóm rỗng quyền để không còn panel trắng. */
  readonly columns = computed(() => {
    const ctx = this.permissionContext();
    const visible = CONFIG_MENU.filter((g) => g.items.some((i) => isItemVisible(g, i, ctx)));
    return [1, 2, 3].map((col) => visible.filter((g) => g.column === col));
  });

  /** Mục vừa mở, dựng lại từ link đã lưu — link không còn/không đủ quyền thì tự rụng. */
  readonly recent = computed(() => {
    const index = this.menuIndex();
    return this.recentLinks()
      .map((link) => index.find((e) => e.item.link === link))
      .filter((e): e is NonNullable<typeof e> => !!e);
  });

  constructor() {
    super();
    // Kết quả đổi → con trỏ bàn phím về dòng đầu, tránh trỏ vào chỉ số đã vượt mảng.
    effect(() => {
      this.results();
      this.activeIndex.set(0);
    });
  }

  // Vào /config là để tìm cấu hình — tự focus ô search luôn, khỏi bắt user trỏ chuột vào mới gõ được.
  ngAfterViewInit(): void {
    this.focusSearch();
  }

  trackGroup = (_: number, group: ConfigMenuGroup) => group.key;

  onQueryInput(value: string): void {
    this.query.set(value);
  }

  clearQuery(): void {
    this.query.set('');
    this.focusSearch();
  }

  focusSearch(): void {
    queueMicrotask(() => this.searchInput()?.nativeElement?.focus());
  }

  /** Ctrl/Cmd+K từ bất kỳ đâu trong trang → nhảy vào ô tìm kiếm. */
  onHotkey(event: KeyboardEvent): void {
    if ((event.ctrlKey || event.metaKey) && event.key.toLowerCase() === 'k') {
      event.preventDefault();
      this.focusSearch();
    }
  }

  onSearchKeydown(event: KeyboardEvent): void {
    const hits = this.results();

    switch (event.key) {
      case 'ArrowDown':
        event.preventDefault();
        if (hits.length) this.activeIndex.set((this.activeIndex() + 1) % hits.length);
        break;
      case 'ArrowUp':
        event.preventDefault();
        if (hits.length) this.activeIndex.set((this.activeIndex() - 1 + hits.length) % hits.length);
        break;
      case 'Enter': {
        const hit = hits[this.activeIndex()];
        if (hit) {
          event.preventDefault();
          this.openHit(hit);
        }
        break;
      }
      case 'Escape':
        event.preventDefault();
        this.clearQuery();
        break;
    }
  }

  openHit(hit: ConfigMenuHit): void {
    this.rememberVisit(hit.item.link);
    void this.route.navigate([hit.item.link], { queryParams: hit.item.queryParams });
  }

  /** Panel/chip điều hướng bằng routerLink — chỉ cần ghi nhận lượt mở. */
  rememberVisit(link: string): void {
    const next = [link, ...this.recentLinks().filter((l) => l !== link)].slice(0, CONFIG_RECENT_LIMIT);
    this.recentLinks.set(next);
    this.storageService.saveStorage(STORAGE_KEY_SELECT.ConfigMenuRecent, next);
  }

  clearRecent(): void {
    this.recentLinks.set([]);
    this.storageService.saveStorage(STORAGE_KEY_SELECT.ConfigMenuRecent, []);
  }

  private loadRecent(): string[] {
    const stored = this.storageService.loadStorage<string[]>(STORAGE_KEY_SELECT.ConfigMenuRecent);
    return Array.isArray(stored) ? stored.filter((l): l is string => typeof l === 'string') : [];
  }
}
