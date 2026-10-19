import { inject } from '@angular/core';
import { MasterDataRegistryService } from '@/indexeddb/config/service/master-data-registry.service';
import { CacheInvalidationRegistry } from '@/indexeddb/config/service/cache-invalidation-registry.service';

export interface BaseCacheFilterOptions {
  activeOnly?: boolean;
  includeItemId?: number;
  includeItemIds?: number[];
}

export abstract class BaseCacheService<T extends { id?: number; status?: number }> {
  protected registry = inject(MasterDataRegistryService);
  private invalidationRegistry = inject(CacheInvalidationRegistry);
  private registeredForInvalidation = false;
  protected cachedItems: T[] | null = null;
  protected itemsMap = new Map<number, T>();
  protected isLoaded = false;
  protected filterCache = new Map<string, number[]>(); // Chỉ cache IDs thay vì objects
  protected readonly MAX_CACHE_SIZE = 100; // Tăng vì chỉ lưu IDs
  /** Tăng mỗi lần invalidateMemoryCache() — component ngoài dùng để tự invalidate cache filter riêng của nó. */
  public dataVersion = 0;
  /** Chỉ warn 1 lần khi getAllItems rơi vào nhánh trả rỗng im lặng — tránh spam console. */
  private warnedEmptyRead = false;
  /** Memoize giá trị derived tuỳ ý từ cache gốc (vd danh sách đã precompute search field) — tự
   * invalidate qua invalidateMemoryCache(), subclass KHÔNG cần tự viết onCacheCleared() thủ công. */
  private derivedCache = new Map<string, unknown>();

  protected abstract tableName: string;
  protected abstract activeStatus: number;
  protected abstract allowedStatuses: number[];

  async loadItems(forceReload = false) {
    if (!this.registeredForInvalidation) {
      this.registeredForInvalidation = true;
      this.invalidationRegistry.register(this.tableName, this);
    }

    if (!this.isLoaded || forceReload) {
      // Clear cache trước khi load
      if (forceReload) {
        this.cachedItems = null;
        this.filterCache.clear();
      }

      await this.registry.loadTableOnDemand(this.tableName);
      this.isLoaded = true;
      // Trong lúc await ở trên, caller khác có thể đã populate cachedItems + build các index
      // dẫn xuất (departmentIndex). Invalidate đầy đủ (base + subclass, atomic)
      // để lần đọc kế tiếp rebuild đồng bộ toàn bộ — xoá lệch nửa là index con chết vĩnh viễn
      // trong khi cachedItems còn ấm (bug dropdown Bác sĩ/Phụ tá rỗng sau login).
      this.invalidateMemoryCache();
    }
  }

  /**
   * Invalidate in-memory cache mà không reset isLoaded — dùng khi registry data đã refresh
   * (sau IndexedDB sync) nhưng các cache trong BaseCacheService có thể stale.
   * Lần `getAllItems()` tiếp theo sẽ pull fresh data từ registry signal.
   *
   * Đây là entry point invalidation DUY NHẤT: xoá cả cache base lẫn cache dẫn xuất của
   * subclass (qua onCacheCleared) trong cùng một lời gọi. KHÔNG gọi riêng onCacheCleared()
   * để invalidate — index con bị xoá trong khi cachedItems còn ấm sẽ không bao giờ được
   * rebuild (bug dropdown Bác sĩ rỗng sau login, fix 2026-07-27).
   */
  invalidateMemoryCache(): void {
    this.cachedItems = null;
    this.itemsMap.clear();
    this.filterCache.clear();
    this.derivedCache.clear();
    this.dataVersion++;
    this.onCacheCleared();
  }

  /** Memoize 1 giá trị derived từ cache gốc (vd danh sách đã precompute search field), tự
   * invalidate khi invalidateMemoryCache() chạy — KHÔNG cần subclass tự override onCacheCleared(). */
  protected getOrBuildDerived<R>(key: string, builder: () => R): R {
    if (!this.derivedCache.has(key)) {
      this.derivedCache.set(key, builder());
    }
    return this.derivedCache.get(key) as R;
  }

  /**
   * Cache NGUỒN đã load xong hay chưa (public — `isLoaded` là field protected).
   * Component ngoài cần nó để phân biệt "danh sách lọc rỗng vì cache chưa load" với "thương hiệu
   * mới thật sự chưa có item nào": cả hai đều cho mảng rỗng, đo bằng `length` là sai.
   */
  public get hasLoaded(): boolean {
    return this.isLoaded;
  }

  getAllItems(): T[] {
    // Nếu cache đã bị clear (cachedItems = null), lấy data mới từ registry
    if (!this.cachedItems || this.cachedItems.length === 0) {
      const service = this.registry.getTableService(this.tableName);
      // Kiểm tra xem service có data không trước khi lấy
      if (service?.isLoaded && service.isLoaded()) {
        const freshItems = (service.getAll()() || []) as T[];
        this.cachedItems = freshItems;
        if (freshItems.length > 0) {
          this.buildItemsMap();
          this.onItemsUpdated(freshItems);
        }
      } else {
        // Nếu service chưa load, trigger load
        if (!this.isLoaded) {
          void this.loadItems();
        } else if (!this.warnedEmptyRead) {
          // isLoaded=true nhưng registry service không có data → trạng thái bất thường,
          // trả rỗng im lặng từng che giấu bug rất khó truy — log 1 lần để lộ diện sớm.
          this.warnedEmptyRead = true;
          console.warn(`[BaseCache] getAllItems(${this.tableName}): cache đã init nhưng registry service chưa loaded — trả rỗng`);
        }
        return [];
      }
    }

    return this.cachedItems || [];
  }

  getById(id: number): T | undefined {
    // Nếu Map đã có data, return luôn
    if (this.itemsMap.size > 0) {
      return this.itemsMap.get(id);
    }

    // Chỉ gọi getAllItems() khi Map rỗng
    const items = this.getAllItems();

    // Build Map nếu có items
    if (items.length > 0) {
      this.buildItemsMap();
    }

    return this.itemsMap.get(id);
  }

  private buildItemsMap(): void {
    this.itemsMap.clear();
    // Sử dụng trực tiếp cachedItems thay vì gọi getAllItems() để tránh circular
    const items = this.cachedItems || [];
    items.forEach(item => {
      if (item.id !== undefined) {
        this.itemsMap.set(item.id, item);
      }
    });
  }

  protected filterItems<TOptions extends BaseCacheFilterOptions>(
    options: TOptions,
    customFilter?: (items: T[], options: TOptions) => T[],
  ): T[] {
    const cacheKey = this.generateCacheKey(options);

    // Kiểm tra cache IDs
    if (this.filterCache.has(cacheKey)) {
      const cachedIds = this.filterCache.get(cacheKey)!;
      return cachedIds.map(id => this.itemsMap.get(id)!).filter((x): x is T => x !== undefined);
    }

    // Clear cache nếu quá lớn (FIFO)
    if (this.filterCache.size >= this.MAX_CACHE_SIZE) {
      const firstKey = this.filterCache.keys().next().value;
      this.filterCache.delete(firstKey!);
    }

    let items = this.getAllItems();

    // Custom filter logic
    if (customFilter) {
      items = customFilter(items, options);
    } else {
      if (options.activeOnly) {
        items = items.filter((item) => item.status === this.activeStatus);
      } else {
        items = items.filter((item) => this.allowedStatuses.includes(item.status!));
      }
    }

    // Include selected items
    items = this.includeSelectedItems(items, options);

    // Chỉ cache khi data đã load — tránh stale empty cache khi sync chưa kịp
    if (this.cachedItems && this.cachedItems.length > 0) {
      this.filterCache.set(cacheKey, items.map(item => item.id!).filter(id => id !== undefined));
    }
    return items;
  }

  protected abstract generateCacheKey(options: any): string;

  protected onCacheCleared(): void {
    // Override nếu cần
  }

  protected onItemsUpdated(items: T[]): void {
    // Override nếu cần
  }

  private includeSelectedItems(items: T[], options: BaseCacheFilterOptions): T[] {
    // Include selected item (dù có bị inactive/deleted)
    if (options.includeItemId && !items.find((item) => item.id === options.includeItemId)) {
      const selectedItem = this.getAllItems().find((item) => item.id === options.includeItemId);
      if (selectedItem) {
        items = [selectedItem, ...items];
      }
    }

    // Include selected items for multiselect (dù có bị inactive/deleted)
    if (options.includeItemIds && Array.isArray(options.includeItemIds)) {
      // Tách selected items ra khỏi danh sách hiện tại
      const selectedItems: T[] = [];
      const nonSelectedItems: T[] = [];

      // Phân loại items hiện tại
      items.forEach((item) => {
        if (options.includeItemIds!.includes(item.id!)) {
          selectedItems.push(item);
        } else {
          nonSelectedItems.push(item);
        }
      });

      // Thêm các selected items chưa có trong danh sách
      const missingSelectedItems = options.includeItemIds
        .filter((id) => !items.find((item) => item.id === id))
        .map((id) => this.getAllItems().find((item) => item.id === id))
        .filter((x): x is T => x !== undefined);

      // Đưa selected items lên đầu
      items = [...selectedItems, ...missingSelectedItems, ...nonSelectedItems];
    }

    return items;
  }
}
