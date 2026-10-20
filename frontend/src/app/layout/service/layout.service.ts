import { computed, effect, inject, Service, signal } from '@angular/core';
import { Observable, Subject } from 'rxjs';

import { STORAGE_KEY_SELECT } from '@/define';
import { StorageService } from '@/share/services/storage.service';

/**
 * Chế độ menu. Khung layout hiện chỉ dựng `static` và `overlay`; các giá trị còn lại giữ trong kiểu để
 * code gọi (vd `menuModeOverride` ở chi tiết khách hàng) và dữ liệu đã lưu không phải đổi.
 */
export type MenuMode =
  'static' | 'overlay' | 'slim-plus' | 'slim' | 'horizontal' | 'reveal' | 'drawer';

/** Cấu hình giao diện lưu ở localStorage (`layout_config`). Chỉ `darkTheme` được đọc lại khi tải. */
export interface layoutConfig {
  primary: string;
  surface: string | undefined | null;
  darkTheme: boolean;
  menuMode: MenuMode;
  menuTheme: string;
  topbarTheme: string;
  menuProfilePosition: string;
}

/** Trạng thái tạm thời của khung (không lưu). Sidebar/topbar cập nhật theo tên field bằng spread. */
export interface LayoutState {
  staticMenuDesktopInactive: boolean;
  overlayMenuActive: boolean;
  configSidebarVisible: boolean;
  staticMenuMobileActive: boolean;
  menuHoverActive: boolean;
  rightMenuActive: boolean;
  topbarMenuActive: boolean;
  sidebarActive: boolean;
  activeMenuItem: any;
  overlaySubmenuActive: boolean;
  anchored: boolean;
  menuProfileActive: boolean;
}

/** Sự kiện mở một nhánh menu. `routeEvent` = mở do route hiện tại khớp (mở cả chuỗi cha). */
export interface MenuChangeEvent {
  key: string;
  routeEvent?: boolean;
}

/** Bề rộng tối đa coi là mobile — khớp media query `991px` trong `styles.scss`. */
const MOBILE_MAX_WIDTH = 991;
const DARK_CLASS = 'app-dark';

const DEFAULT_CONFIG: layoutConfig = {
  primary: 'indigo',
  surface: null,
  darkTheme: false,
  menuMode: 'static',
  menuTheme: 'light',
  topbarTheme: 'cluvix',
  menuProfilePosition: 'end',
};

const DEFAULT_STATE: LayoutState = {
  staticMenuDesktopInactive: false,
  overlayMenuActive: false,
  configSidebarVisible: false,
  staticMenuMobileActive: false,
  menuHoverActive: false,
  rightMenuActive: false,
  topbarMenuActive: false,
  sidebarActive: false,
  activeMenuItem: null,
  overlaySubmenuActive: false,
  anchored: false,
  menuProfileActive: false,
};

function menuThemeFor(dark: boolean): string {
  return dark ? 'dark' : 'light';
}

/** Đặt/bỏ class tối trên `<html>` — theme PrimeNG dùng `darkModeSelector: '.app-dark'`. */
function applyDarkClass(dark: boolean): void {
  document.documentElement.classList.toggle(DARK_CLASS, dark);
}

/**
 * Trạng thái khung layout: chế độ menu (static/overlay), sidebar đóng/mở, sáng/tối, và kênh sự kiện
 * giữa các mục menu.
 */
@Service()
export class LayoutService {
  private readonly storage = inject(StorageService);

  readonly layoutConfig = signal<layoutConfig>(this.loadConfig());

  readonly layoutState = signal<LayoutState>({ ...DEFAULT_STATE });

  /** Chế độ menu tạm theo TRANG (không lưu). `null` = theo cấu hình. */
  readonly menuModeOverride = signal<MenuMode | null>(null);

  readonly effectiveMenuMode = computed<MenuMode>(
    () => this.menuModeOverride() ?? this.layoutConfig().menuMode,
  );

  readonly isDarkTheme = computed(() => this.layoutConfig().darkTheme);

  readonly isOverlay = computed(() => this.effectiveMenuMode() === 'overlay');

  /** Giữ cho code cũ — khung không dựng các chế độ này. */
  readonly isSlim = computed(() => false);
  readonly isSlimPlus = computed(() => false);
  readonly isHorizontal = computed(() => false);

  private readonly overlayOpen = new Subject<void>();
  private readonly menuSource = new Subject<MenuChangeEvent>();
  private readonly resetSource = new Subject<boolean>();

  readonly overlayOpen$: Observable<void> = this.overlayOpen.asObservable();
  readonly menuSource$: Observable<MenuChangeEvent> = this.menuSource.asObservable();
  readonly resetSource$: Observable<boolean> = this.resetSource.asObservable();

  constructor() {
    // Áp class tối ĐỒNG BỘ trước khi vẽ để lần tải không nháy nền sáng.
    applyDarkClass(this.layoutConfig().darkTheme);

    effect(() => {
      this.storage.saveStorage(STORAGE_KEY_SELECT.LayoutConfig, this.layoutConfig());
    });

    // Lần chạy đầu bỏ qua: class đã áp ở trên, không crossfade lúc tải trang.
    let initialized = false;
    effect(() => {
      const dark = this.layoutConfig().darkTheme;
      if (!initialized) {
        initialized = true;
        return;
      }
      this.switchTheme(dark);
    });
  }

  onMenuToggle(): void {
    // Mobile xét TRƯỚC: sidebar mobile chỉ hiện qua `staticMenuMobileActive`, kể cả khi trang ép overlay.
    if (this.isMobile()) {
      const next = !this.layoutState().staticMenuMobileActive;
      // Hai cờ sidebar nổi không bao giờ cùng bật ⇒ effect dọn dẹp ở AppLayout luôn gỡ được.
      this.layoutState.update((s) => ({
        ...s,
        staticMenuMobileActive: next,
        overlayMenuActive: false,
      }));
      if (next) this.overlayOpen.next();
      return;
    }
    if (this.isOverlay()) {
      const next = !this.layoutState().overlayMenuActive;
      this.layoutState.update((s) => ({
        ...s,
        overlayMenuActive: next,
        staticMenuMobileActive: false,
      }));
      if (next) this.overlayOpen.next();
      return;
    }
    this.layoutState.update((s) => ({
      ...s,
      staticMenuDesktopInactive: !s.staticMenuDesktopInactive,
      staticMenuMobileActive: false,
    }));
  }

  toggleDarkTheme(): void {
    this.layoutConfig.update((c) => ({
      ...c,
      darkTheme: !c.darkTheme,
      menuTheme: menuThemeFor(!c.darkTheme),
    }));
  }

  onMenuStateChange(event: MenuChangeEvent): void {
    this.menuSource.next(event);
  }

  reset(): void {
    this.resetSource.next(true);
  }

  isDesktop(): boolean {
    return typeof window !== 'undefined' && window.innerWidth > MOBILE_MAX_WIDTH;
  }

  isMobile(): boolean {
    return !this.isDesktop();
  }

  /** Bản lưu chỉ góp `darkTheme`; field khác (vd `menuMode` cũ) bỏ qua để menu luôn chạy static. */
  private loadConfig(): layoutConfig {
    const saved = this.storage.loadStorage<Partial<layoutConfig>>(STORAGE_KEY_SELECT.LayoutConfig);
    const dark = typeof saved === 'object' && saved !== null && saved.darkTheme === true;
    return { ...DEFAULT_CONFIG, darkTheme: dark, menuTheme: menuThemeFor(dark) };
  }

  private switchTheme(dark: boolean): void {
    const doc = document as Document & { startViewTransition?: (cb: () => void) => unknown };
    if (typeof doc.startViewTransition === 'function') {
      doc.startViewTransition(() => applyDarkClass(dark));
    } else {
      applyDarkClass(dark);
    }
  }
}
