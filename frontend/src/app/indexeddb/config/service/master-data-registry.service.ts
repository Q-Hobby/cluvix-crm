import { inject, Service, signal } from '@angular/core';
import { HttpClient } from '@angular/common/http';
import { firstValueFrom } from 'rxjs';
import { environment } from '@env/environment';
import { TableServiceFactory } from './table-service-factory';
import { TABLE_REGISTRY } from '@/indexeddb/config/table-registry';
import { CacheInvalidationRegistry } from '@/indexeddb/config/service/cache-invalidation-registry.service';

@Service()
export class MasterDataRegistryService {
  private http = inject(HttpClient);
  private factory = inject(TableServiceFactory);
  private tableConfigs = inject(TABLE_REGISTRY);
  private invalidationRegistry = inject(CacheInvalidationRegistry);

  private loadedTables = signal<Set<string>>(new Set());
  private loadingPromises = new Map<string, Promise<void>>();

  async loadTableOnDemand(tableName: string): Promise<void> {
    // Chặn store "undefined" khi component wrapper (vd UserSelect) quên override tableName
    if (!tableName) {
      console.warn('[MasterData] loadTableOnDemand gọi với tableName rỗng — bỏ qua');
      return;
    }

    if (this.isTableLoaded(tableName)) {
      return;
    }

    // Prevent duplicate loading
    if (this.loadingPromises.has(tableName)) {
      return this.loadingPromises.get(tableName)!;
    }

    const promise = this.loadTable(tableName);
    this.loadingPromises.set(tableName, promise);

    try {
      await promise;
    } finally {
      this.loadingPromises.delete(tableName);
    }
  }

  // Invalidate cache và force sync từ server
  async invalidateAndSync(tableName: string): Promise<void> {
    // QA Pass 1 story-11 (F2): store per-user (`localOnly`, vd `notifications`) KHÔNG có bên tương
    // ứng ở BE — `service.save([])` bên dưới XOÁ TRẮNG dữ liệu của người đang dùng trước khi
    // `syncSingleTable` kịp trả về sớm. Chặn ngay tại đây, cùng lý do với guard trong
    // `syncSingleTable`/`syncAllTables`.
    const config = this.tableConfigs.find((c) => c.name === tableName);
    if (config?.localOnly) return;
    // Tên version KHÔNG phải bảng IndexedDB (vd `business_profile` — chỉ là tín hiệu reload cho
    // `HeaderSyncService`): `service.save([])` trên store không tồn tại sẽ ném lỗi Dexie.
    if (!config) return;

    this.markTableAsUnloaded(tableName);

    // Clear service data
    const service = this.factory.getService(tableName);
    await service.save([]); // Clear local data

    // Force sync from server
    await this.syncSingleTable(tableName);
  }

  // Sync chỉ 1 table cụ thể
  async syncSingleTable(tableName: string): Promise<void> {
    const config = this.tableConfigs.find((c) => c.name === tableName);
    // Store per-user (`localOnly`, vd `notifications`) KHÔNG có bên tương ứng ở BE: gọi
    // `/crm/indexeddb/sync?tables=notifications` sẽ trả rỗng, và `syncTable` sẽ `save([])` = XOÁ
    // TRẮNG thông báo của người đang dùng. Chặn ngay tại cửa thay vì tin rằng header BE không bao
    // giờ nhắc tên nó.
    if (config?.localOnly) return;
    if (!config) {
      console.warn(
        `⚠️ Table ${tableName} not found in registry. Available tables:`,
        this.tableConfigs.map((c) => c.name),
      );
      return;
    }

    try {
      const serverData = await this.fetchTableFromServer(tableName);
      if (serverData && serverData.length > 0) {
        const service = this.factory.getService(tableName);
        await service.save(serverData);
        this.markTableAsLoaded(tableName);
        this.invalidationRegistry.invalidate(tableName);
      } else {
        console.warn(`⚠️ Không có data để sync cho table ${tableName}`);
      }
    } catch (error) {
      console.error(`❌ Lỗi sync table ${tableName}:`, error);
      throw error;
    }
  }

  getTableService(tableName: string) {
    return this.factory.getService(tableName);
  }

  // Sync tất cả bảng
  async syncAllTables(response: any): Promise<void> {
    const syncPromises: Promise<void>[] = [];

    for (const config of this.tableConfigs) {
      if (config.localOnly) continue; // xem `syncSingleTable` — store per-user không đi qua sync BE
      if (response[config.name]) {
        const syncPromise = this.syncTable(config.name, response[config.name]);
        syncPromises.push(syncPromise);
      }
    }

    await Promise.all(syncPromises);
  }

  private async loadTable(tableName: string): Promise<void> {
    try {
      const service = this.factory.getService(tableName);
      await service.load();
      this.markTableAsLoaded(tableName);
    } catch (error) {
      console.error(`❌ Failed to load table ${tableName}:`, error);

      if (error instanceof DOMException && error.name === 'InvalidStateError') {
        this.markTableAsUnloaded(tableName);

        try {
          const service = this.factory.getService(tableName);
          await service.load();
          this.markTableAsLoaded(tableName);
        } catch (retryError) {
          console.error(`❌ Retry failed for table ${tableName}:`, retryError);
          throw retryError;
        }
      } else {
        throw error;
      }
    }
  }

  private async fetchTableFromServer(tableName: string): Promise<any[]> {
    const response = await firstValueFrom(
      this.http.get<any>(`${environment.apiUrl}/crm/indexeddb/sync?tables=${tableName}`),
    );
    return response[tableName] || [];
  }

  private isTableLoaded(tableName: string): boolean {
    return this.loadedTables().has(tableName);
  }

  private markTableAsLoaded(tableName: string): void {
    this.loadedTables.update(tables => {
      const updated = new Set(tables);
      updated.add(tableName);
      return updated;
    });
  }

  private markTableAsUnloaded(tableName: string): void {
    this.loadedTables.update(tables => {
      const updated = new Set(tables);
      updated.delete(tableName);
      return updated;
    });
  }


  private async syncTable(tableName: string, data: any[]): Promise<void> {
    try {
      const service = this.factory.getService(tableName);
      await service.save(data);
      this.invalidationRegistry.invalidate(tableName);
    } catch (error) {
      console.error(`Failed to sync ${tableName}:`, error);
    }
  }
}
