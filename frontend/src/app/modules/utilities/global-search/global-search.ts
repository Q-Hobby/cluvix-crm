import {ChangeDetectionStrategy, AfterViewInit,
  Component,
  DestroyRef,
  effect,
  ElementRef,
  inject,
  signal,
  ViewChild,
  WritableSignal,
} from '@angular/core';
import { takeUntilDestroyed } from '@angular/core/rxjs-interop';
import { firstValueFrom } from 'rxjs';

import { CustomerFormV2 } from '@/modules/crm/customer/form/form-customer/customer-form-v2';
import { CustomerService } from '@/core/services/crm/customer.service';
import { CustomerModel } from '@/core/models/crm/customer.model';
import { GlobalSearchService } from '@/modules/utilities/global-search/global-search.service';
import { BaseApp } from '@/share/base/BaseApp';
import { STORAGE_KEY_SELECT } from '@/define';
import { ListShareImports } from '@/share/import/list-share-imports';

interface RecentCustomerItem {
  customer: CustomerModel;
  favourite?: boolean;
}

@Component({
  changeDetection: ChangeDetectionStrategy.OnPush,
  selector: 'app-utilities-global-search',
  standalone: true,
  imports: [...ListShareImports],
  templateUrl: './global-search.html',
  styleUrl: './global-search.scss',
})
export class GlobalSearch extends BaseApp implements AfterViewInit {
  @ViewChild('searchInput') searchInput!: ElementRef<HTMLInputElement>;
  readonly skeletonRows = Array.from({ length: 8 });
  textSearch = signal('');
  isSearched = signal(false);
  filteredCustomers: WritableSignal<CustomerModel[]> = signal([]);
  selected: WritableSignal<CustomerModel | null> = signal(null);
  recent: WritableSignal<RecentCustomerItem[]> = signal([]);
  favourite: WritableSignal<RecentCustomerItem[]> = signal([]);
  customerService = inject(CustomerService);
  globalSearchService = inject(GlobalSearchService);
  private readonly destroyRef = inject(DestroyRef);
  private inputTimeout: any;
  private searchGeneration = 0;

  constructor() {
    super();
    this.loadStorage();

    // ✅ Debounce search sau 500ms
    effect(() => {
      const value = this.textSearch().trim();

      clearTimeout(this.inputTimeout);
      this.inputTimeout = setTimeout(() => {
        this.selected.set(null);
        this.isSearched.set(false);
        if (value) {
          void this.triggerSearch(); // auto gọi tìm kiếm
        } else {
          this.filteredCustomers.set([]);
        }
      }, 500);
    });

    // Component sống suốt phiên (mount 1 lần trong app.topbar.ts) nên trong thực tế
    // hiếm khi destroy, nhưng vẫn clear để không bắn HTTP thừa nếu sau này lazy-mount.
    this.destroyRef.onDestroy(() => clearTimeout(this.inputTimeout));
  }

  loadStorage() {
    const recentStr = this.storageService.loadStorage(STORAGE_KEY_SELECT.Recent, false);
    const favouriteStr = this.storageService.loadStorage(STORAGE_KEY_SELECT.Favourite, false);

    try { this.recent.set(recentStr ? JSON.parse(recentStr) : []); }
    catch { this.recent.set([]); }
    try { this.favourite.set(favouriteStr ? JSON.parse(favouriteStr) : []); }
    catch { this.favourite.set([]); }
  }

  async triggerSearch() {
    clearTimeout(this.inputTimeout);
    const value = this.textSearch().trim();
    if (!value) return;

    this.filteredCustomers.set([]);
    this.isSearched.set(true);
    try {
      await this.searchCustomer(value);
    } catch {
      // Interceptor đã tự hiện toast lỗi — chỉ cần chặn unhandled promise rejection.
    }
  }

  open() {
    this.globalSearchService.visible.set(true);
  }

  ngAfterViewInit(): void {
    queueMicrotask(() => this.searchInput?.nativeElement?.focus());
  }

  async searchCustomer(value: string) {
    const gen = ++this.searchGeneration;
    try {
      this.loading.set(true);
      const customers = await firstValueFrom(this.customerService.searchGlobal(value));
      if (gen !== this.searchGeneration) return;
      this.filteredCustomers.set(customers ?? []);
    } finally {
      if (gen === this.searchGeneration) {
        this.loading.set(false);
      }
    }
  }

  onHide() {
    this.textSearch.set('');
  }

  onShow() {
    setTimeout(() => {
      this.searchInput?.nativeElement?.focus();
    });
  }

  goToCustomer(customer: CustomerModel) {
    this.addToRecent(customer);

    if (location.pathname === `/customer/detail/${customer.id}`) {
      this.globalSearchService.visible.set(false);
      return;
    }

    // Truyền customer qua state để detail page paint instant, skip blank trong khi chờ HTTP.
    void this.route.navigate([`/customer/detail/${customer.id}`], { state: { customer } });
    this.globalSearchService.visible.set(false);
    this.textSearch.set('');
  }

  openNewTab(customer: CustomerModel) {
    this.addToRecent(customer);
    window.open(`/customer/detail/${customer.id}`);
  }

  private addToRecent(customer: CustomerModel) {
    const recentList = this.recent();
    if (
      !recentList.find((item) => item.customer.id === customer.id) &&
      !this.favourite().find((item) => item.customer.id === customer.id)
    ) {
      this.recent.set([{ customer }, ...recentList].slice(0, 5));
      this.saveRecent();
    }
  }

  removeFavorite($event: MouseEvent, recentItem: RecentCustomerItem) {
    $event.stopImmediatePropagation();
    this.favourite.set(
      this.favourite().filter((item) => item.customer.id !== recentItem.customer?.id),
    );
    this.saveFavourite();
  }

  removeRecent($event: MouseEvent, recentItem: RecentCustomerItem) {
    $event.stopImmediatePropagation();
    this.recent.set(this.recent().filter((item) => item.customer?.id !== recentItem.customer?.id));
    this.saveRecent();
  }

  markFavorite($event: MouseEvent, recentItem: RecentCustomerItem) {
    $event.stopImmediatePropagation();

    // Xóa khỏi recent
    this.recent.set(this.recent().filter((item) => item.customer.id !== recentItem.customer?.id));
    this.saveRecent();

    // Thêm vào đầu favourite
    const old = this.favourite().filter((item) => item.customer.id !== recentItem.customer.id);
    this.favourite.set([recentItem, ...old].slice(0, 10));
    this.saveFavourite();
  }

  createCustomer() {
    const dialogRef = this.dialogWrapperService.open(CustomerFormV2, {
      header: this.translate.instant('utilities.global_search.dialog'),
      width: '840px',
      data: { phone: this.textSearch().trim() },
    });

    dialogRef.onClose.pipe(takeUntilDestroyed(this.destroyRef)).subscribe((result: any) => {
      if (result) {
        void this.route.navigate([`/customer/detail/${result.id}`], { state: { customer: result } });
        this.globalSearchService.visible.set(false);
      }
    });
  }

  closeDialog() {
    this.globalSearchService.visible.set(false);
    this.textSearch.set('');
    this.isSearched.set(false);
    this.filteredCustomers.set([]);
    this.selected.set(null);
  }

  saveRecent() {
    this.storageService.saveStorage(STORAGE_KEY_SELECT.Recent, this.recent());
  }

  saveFavourite() {
    this.storageService.saveStorage(STORAGE_KEY_SELECT.Favourite, this.favourite());
  }
}
