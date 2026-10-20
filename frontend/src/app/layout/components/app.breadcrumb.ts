import { ChangeDetectionStrategy, Component, DestroyRef, computed, inject, signal } from '@angular/core';
import { takeUntilDestroyed } from '@angular/core/rxjs-interop';
import { ActivatedRouteSnapshot, NavigationEnd, RouterLink } from '@angular/router';
import { TranslatePipe } from '@ngx-translate/core';
import { ButtonModule } from 'primeng/button';
import { TooltipModule } from 'primeng/tooltip';
import { filter, firstValueFrom } from 'rxjs';

import { LayoutService } from '@/layout/service/layout.service';
import { CurrencyComponent } from '@/modules/utilities/currency/currency.component';
import { BaseApp } from '@/share/base/BaseApp';
import { BreadcrumbLabelService } from '@/share/services/breadcrumb-label.service';

/** Một mục trên đường dẫn breadcrumb. */
interface BreadcrumbItem {
  label: string;
  url: string;
}

const BREADCRUMB_DATA_KEY = 'breadcrumb';
const ROUTER_LINK_DATA_KEY = 'routerLink';
const CURRENCY_DIALOG_WIDTH = '700px';

/**
 * Thanh breadcrumb của khung layout: icon home → các cấp route có `data.breadcrumb`.
 * Mục cuối có thể được màn chi tiết thay bằng tên bản ghi qua `BreadcrumbLabelService`.
 */
@Component({
  selector: '[app-breadcrumb]',
  standalone: true,
  imports: [RouterLink, ButtonModule, TooltipModule, TranslatePipe],
  changeDetection: ChangeDetectionStrategy.OnPush,
  host: { class: 'layout-breadcrumb-container' },
  template: `
    <nav class="layout-breadcrumb items-center" [class.flex]="layoutService.isDesktop()" [class.hidden]="!layoutService.isDesktop()">
      <a routerLink="/" class="flex items-center text-primary" [attr.aria-label]="'auth.notfound.home_label' | translate">
        <i class="pi pi-home"></i>
      </a>
      @for (item of breadcrumbs(); track item.url + item.label; let last = $last) {
        <i class="pi pi-angle-right mx-2 text-muted-color" aria-hidden="true"></i>
        @if (last) {
          <span class="min-w-0 truncate text-color" aria-current="page">{{ item.label }}</span>
        } @else {
          <a [routerLink]="item.url" class="min-w-0 truncate text-primary">{{ item.label }}</a>
        }
      }
    </nav>
    <div class="layout-breadcrumb-buttons">
      <button
        pButton
        type="button"
        [text]="true"
        [pTooltip]="'common.label.currency_converter' | translate"
        [attr.aria-label]="'common.label.currency_converter' | translate"
        (click)="openCurrencyConverter()"
      >
        <i pButtonIcon class="pi pi-globe"></i>
      </button>
    </div>
  `,
})
export class AppBreadcrumb extends BaseApp {
  protected readonly layoutService = inject(LayoutService);
  private readonly labelService = inject(BreadcrumbLabelService);
  private readonly destroyRef = inject(DestroyRef);

  private readonly built = signal<BreadcrumbItem[]>([]);
  /** Số thứ tự lượt build mới nhất — lượt cũ về muộn thì bỏ kết quả. */
  private buildSeq = 0;

  /** Mục cuối lấy tên bản ghi từ `BreadcrumbLabelService` khi màn chi tiết đã đặt. */
  readonly breadcrumbs = computed<BreadcrumbItem[]>(() => {
    const items = this.built();
    const override = this.labelService.label();
    if (!override || items.length === 0) return items;
    return [...items.slice(0, -1), { ...items[items.length - 1], label: override }];
  });

  constructor() {
    super();
    this.route.events
      .pipe(
        filter((e) => e instanceof NavigationEnd),
        takeUntilDestroyed(this.destroyRef),
      )
      .subscribe(() => void this.rebuild());
    // F5 vào thẳng URL: NavigationEnd đã phát trước khi component subscribe.
    void this.rebuild();
  }

  protected openCurrencyConverter(): void {
    this.dialogWrapperService.open(CurrencyComponent, {
      width: CURRENCY_DIALOG_WIDTH,
      header: this.translate.instant('common.label.currency_converter'),
    });
  }

  private async rebuild(): Promise<void> {
    const seq = ++this.buildSeq;
    const entries = this.collect(this.route.routerState.snapshot.root);
    const items = await Promise.all(
      entries.map(async (e) => ({
        label: await firstValueFrom(this.translate.get(e.key)),
        url: e.url,
      })),
    );
    if (seq !== this.buildSeq) return;
    this.built.set(items);
  }

  /** Duyệt theo `firstChild`, gom các cấp có `data.breadcrumb` khác cấp cha. */
  private collect(root: ActivatedRouteSnapshot): { key: string; url: string }[] {
    const result: { key: string; url: string }[] = [];
    const segments: string[] = [];
    let parent: ActivatedRouteSnapshot | null = null;
    let node: ActivatedRouteSnapshot | null = root;
    while (node) {
      segments.push(...node.url.map((s) => s.path));
      const key = node.data[BREADCRUMB_DATA_KEY] as string | undefined;
      if (key && key !== parent?.data[BREADCRUMB_DATA_KEY]) {
        const link = node.data[ROUTER_LINK_DATA_KEY] as string | undefined;
        result.push({ key, url: link ?? '/' + segments.join('/') });
      }
      parent = node;
      node = node.firstChild;
    }
    return result;
  }
}
