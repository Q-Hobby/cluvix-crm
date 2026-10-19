import { inject, Service} from '@angular/core';
import { INDEXEDDB_SELECT, STORAGE_KEY_SELECT } from '@/define';
import { TenantService } from '@/core/services/tenant/tenant.service';
import { BaseIndexedDBService } from '@/indexeddb/config/service/base-indexed-db.service';

@Service()
export class CacheManagerService {
  private indexedDB = inject(BaseIndexedDBService);

  constructor() {
    inject(TenantService).register(() => this.clearAllTenantData());
  }

  /**
   * Xóa toàn bộ IndexedDB database
   */
  async clearIndexedDB(): Promise<void> {
    try {
      await this.forceDeleteDatabase(INDEXEDDB_SELECT.INDEXEDDB_DB_NAME_CONFIG);
      await this.forceDeleteDatabase('CustomerLeadDB'); // Clear CustomerLead DB
    } catch (error) {
      console.warn('Error clearing IndexedDB:', error);
    }
  }

  /**
   * Xóa toàn bộ cache tenant-specific khi switch company.
   * Gọi trước location.reload() để tránh stale IndexedDB + dropdown leak.
   */
  async clearAllTenantData(): Promise<void> {
    await this.clearIndexedDB();
    localStorage.removeItem(STORAGE_KEY_SELECT.ConfigDataDropdown);
  }

  /**
   * Xóa CustomerLead IndexedDB riêng biệt
   */
  async clearCustomerLeadDB(): Promise<void> {
    try {
      await this.forceDeleteDatabase('CustomerLeadDB');
    } catch (error) {
      console.warn('Error clearing CustomerLead IndexedDB:', error);
    }
  }

  private async forceDeleteDatabase(dbName: string): Promise<void> {
    return new Promise<void>((resolve) => {
      const deleteReq = indexedDB.deleteDatabase(dbName);

      deleteReq.onsuccess = () => {
        resolve();
      };

      deleteReq.onerror = () => {
        resolve();
      };

      deleteReq.onblocked = () => {
        resolve();
      };

      setTimeout(() => {
        console.warn(`Timeout deleting IndexedDB '${dbName}'`);
        resolve();
      }, 1000);
    });
  }
}
