import { NgClass, NgTemplateOutlet } from '@angular/common';
import {
  ChangeDetectionStrategy,
  Component,
  computed,
  DestroyRef,
  inject,
  input,
  OnInit,
  Signal,
  signal,
} from '@angular/core';
import { takeUntilDestroyed } from '@angular/core/rxjs-interop';
import {
  IsActiveMatchOptions,
  isActive,
  NavigationEnd,
  Router,
  RouterLink,
  RouterLinkActive,
} from '@angular/router';
import { TranslatePipe } from '@ngx-translate/core';
import { MenuItem } from 'primeng/api';
import { filter } from 'rxjs';

import { LayoutService } from '@/layout/service/layout.service';

/** Mục menu sidebar: `MenuItem` của PrimeNG cộng các field riêng của `menu.constant.ts`. */
export interface AppMenuItemModel extends MenuItem {
  /** Key i18n của nhãn; không có thì dùng `label`. */
  labelKey?: string;
  /** Màu nhấn của icon: ô icon có nền và viền nhạt cùng màu. */
  iconColor?: string;
  /** Có trong dữ liệu menu nhưng khung không dùng tới. */
  routerLinkExact?: boolean;
  items?: AppMenuItemModel[];
}

/** Khớp route cho link lá — mặc định khi item không tự khai `routerLinkActiveOptions`. */
const LEAF_ACTIVE_OPTIONS: IsActiveMatchOptions = {
  paths: 'exact',
  queryParams: 'ignored',
  matrixParams: 'ignored',
  fragment: 'ignored',
};

/** Khớp route để mở nhánh cha: route hiện tại nằm dưới link của item. */
const BRANCH_ACTIVE_OPTIONS: IsActiveMatchOptions = {
  paths: 'subset',
  queryParams: 'ignored',
  matrixParams: 'ignored',
  fragment: 'ignored',
};

/** Hậu tố alpha (hex) cho nền và viền ô icon. */
const ICON_BG_ALPHA = '1A';
const ICON_BORDER_ALPHA = '30';

/**
 * Một mục của menu sidebar, tự đệ quy cho `items`. Mỗi mục có khoá theo vị trí (`0`, `0-2`, `0-2-1`…);
 * khoá dùng để chỉ giữ MỘT nhánh mở mỗi lần qua `LayoutService.menuSource$`.
 */
@Component({
  selector: '[app-menuitem]',
  standalone: true,
  imports: [NgClass, NgTemplateOutlet, RouterLink, RouterLinkActive, TranslatePipe],
  changeDetection: ChangeDetectionStrategy.OnPush,
  host: {
    '[class.layout-root-menuitem]': 'root()',
    '[class.active-menuitem]': 'active()',
  },
  template: `
    @let it = item();
    @if (it.visible !== false) {
      <ng-template #content>
        @if (it.icon) {
          @if (it.iconColor) {
            <span
              class="layout-menuitem-icon-box"
              [style.background-color]="it.iconColor + iconBgAlpha"
              [style.border-color]="it.iconColor + iconBorderAlpha"
            >
              <!-- Không mang class layout-menuitem-icon: class đó tự vẽ thành ô riêng ⇒ ô lồng ô. -->
              <i [ngClass]="it.icon" [style.color]="it.iconColor"></i>
            </span>
          } @else {
            <i class="layout-menuitem-icon" [ngClass]="it.icon"></i>
          }
        }
        <span class="layout-menuitem-text">{{ it.labelKey || it.label | translate }}</span>
        @if (hasChildren()) {
          <i class="pi pi-fw pi-angle-down layout-submenu-toggler"></i>
        }
      </ng-template>

      @if (isRootGroup()) {
        <!-- Nhóm gốc luôn mở: tiêu đề là nhãn chữ, không bấm được (không phải "> a"). -->
        <div class="layout-menuitem-root-text">
          <span>{{ it.labelKey || it.label | translate }}</span>
        </div>
      } @else if (it.routerLink && !hasChildren() && !it.disabled) {
        <a
          [routerLink]="it.routerLink"
          routerLinkActive="active-route"
          [routerLinkActiveOptions]="it.routerLinkActiveOptions || leafActiveOptions"
          [queryParams]="it.queryParams"
          [fragment]="it.fragment"
          [queryParamsHandling]="it.queryParamsHandling"
          [preserveFragment]="!!it.preserveFragment"
          [skipLocationChange]="!!it.skipLocationChange"
          [replaceUrl]="!!it.replaceUrl"
          [state]="it.state"
          [attr.target]="it.target"
          [ngClass]="it.class"
          tabindex="0"
          (click)="onClick($event)"
        >
          <ng-container [ngTemplateOutlet]="content" />
        </a>
      } @else if (it.url && !hasChildren() && !it.disabled) {
        <a
          [attr.href]="it.url"
          [attr.target]="it.target"
          [ngClass]="it.class"
          tabindex="0"
          (click)="onClick($event)"
        >
          <ng-container [ngTemplateOutlet]="content" />
        </a>
      } @else {
        <a
          role="button"
          [ngClass]="it.class"
          [class.p-disabled]="it.disabled"
          [attr.tabindex]="it.disabled ? -1 : 0"
          [attr.aria-disabled]="it.disabled ? true : null"
          [attr.aria-expanded]="hasChildren() ? active() : null"
          (click)="onClick($event)"
          (keydown.enter)="onClick($event)"
        >
          <ng-container [ngTemplateOutlet]="content" />
        </a>
      }

      @if (hasChildren()) {
        <!-- Luôn dựng con (chỉ ẩn) để mục lá kiểm được route khi tải trang và mở ngược nhánh cha. -->
        <ul class="layout-submenu" [style.display]="expanded() ? null : 'none'">
          @for (child of it.items; track child; let i = $index) {
            <li app-menuitem [item]="child" [index]="i" [root]="false" [parentKey]="key()"></li>
          }
        </ul>
      }
    }
  `,
  styles: [
    `
      .layout-menuitem-icon-box {
        display: inline-flex;
        align-items: center;
        justify-content: center;
        flex-shrink: 0;
        width: 1.75rem;
        height: 1.75rem;
        margin-right: 0.5rem;
        border: 1px solid transparent;
        border-radius: 0.5rem;
      }

      .layout-menuitem-icon-box > i {
        font-size: 0.875rem;
      }
    `,
  ],
})
export class AppMenuitem implements OnInit {
  readonly item = input.required<AppMenuItemModel>();
  readonly index = input<number>(0);
  readonly root = input<boolean>(false);
  readonly parentKey = input<string>('');

  private readonly layoutService = inject(LayoutService);
  private readonly router = inject(Router);
  private readonly destroyRef = inject(DestroyRef);

  protected readonly leafActiveOptions = LEAF_ACTIVE_OPTIONS;
  protected readonly iconBgAlpha = ICON_BG_ALPHA;
  protected readonly iconBorderAlpha = ICON_BORDER_ALPHA;

  /** Nhánh đang mở (với mục lá: route đang nằm dưới mục này). */
  protected readonly active = signal(false);

  readonly key = computed(() => {
    const parent = this.parentKey();
    const index = String(this.index());
    return parent ? `${parent}-${index}` : index;
  });

  protected readonly hasChildren = computed(() => (this.item().items?.length ?? 0) > 0);

  /** Nhóm cấp gốc có con: luôn mở, không đóng được. Chỉ nhánh cấp 2 trở xuống mới mở/đóng. */
  protected readonly isRootGroup = computed(() => this.root() && this.hasChildren());

  /** Danh sách con đang hiện. */
  protected readonly expanded = computed(() => this.isRootGroup() || this.active());

  /** Route hiện tại nằm dưới link của mục (null khi mục không có `routerLink`). */
  private routeMatch: Signal<boolean> | null = null;

  constructor() {
    this.layoutService.menuSource$
      .pipe(takeUntilDestroyed(this.destroyRef))
      .subscribe(({ key, routeEvent }) => {
        const mine = this.key();
        const inMyChain = key === mine || key.startsWith(`${mine}-`);
        if (inMyChain) {
          if (routeEvent) this.active.set(true);
          return;
        }
        this.active.set(false);
      });

    this.layoutService.resetSource$
      .pipe(takeUntilDestroyed(this.destroyRef))
      .subscribe(() => this.active.set(false));

    this.router.events
      .pipe(
        filter((e) => e instanceof NavigationEnd),
        takeUntilDestroyed(this.destroyRef),
      )
      .subscribe(() => this.syncWithRoute());
  }

  ngOnInit(): void {
    const link = this.item().routerLink;
    const path = Array.isArray(link) ? link[0] : link;
    if (typeof path === 'string') {
      this.routeMatch = isActive(path, this.router, BRANCH_ACTIVE_OPTIONS);
    }
    // F5 vào thẳng URL: NavigationEnd đã phát trước khi menu được dựng.
    this.syncWithRoute();
  }

  protected onClick(event: Event): void {
    const it = this.item();
    if (it.disabled) {
      event.preventDefault();
      return;
    }

    if (this.hasChildren()) {
      event.preventDefault();
      const next = !this.active();
      this.active.set(next);
      if (next) this.layoutService.onMenuStateChange({ key: this.key() });
      return;
    }

    it.command?.({ originalEvent: event, item: it, index: this.index() });
  }

  private syncWithRoute(): void {
    if (this.routeMatch?.()) {
      this.layoutService.onMenuStateChange({ key: this.key(), routeEvent: true });
    }
  }
}
