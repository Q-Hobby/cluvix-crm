import { Service } from '@angular/core';

export interface InvalidatableCache {
  invalidateMemoryCache(): void;
}

/**
 * Cầu nối giữa MasterDataRegistryService (nguồn sync IndexedDB) và các BaseCacheService
 * con (~80 cache service dùng cho autocomplete/select). Mỗi BaseCacheService tự register
 * theo tableName; khi registry sync xong 1 table, gọi invalidate(tableName) để các cache
 * đang giữ snapshot cũ biết đọc lại từ registry ở lần getAllItems() tiếp theo.
 */
@Service()
export class CacheInvalidationRegistry {
  private byTable = new Map<string, Set<InvalidatableCache>>();

  register(tableName: string, cache: InvalidatableCache): void {
    let caches = this.byTable.get(tableName);
    if (!caches) {
      caches = new Set();
      this.byTable.set(tableName, caches);
    }
    caches.add(cache);
  }

  invalidate(tableName: string): void {
    const caches = this.byTable.get(tableName);
    if (!caches) return;
    for (const cache of caches) cache.invalidateMemoryCache();
  }
}
