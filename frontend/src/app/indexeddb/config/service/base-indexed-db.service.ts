import { inject, Service} from '@angular/core';
import { TABLE_REGISTRY } from '@/indexeddb/config/table-registry';
import { INDEXEDDB_SELECT } from '@/define';

/** Tên các DB cũ sẽ bị xóa lần đầu chạy v50 (đã gộp vào DB chính). */
const LEGACY_DB_NAMES = ['EmrConfigDB', 'CustomerLeadDB'];

@Service()
export class BaseIndexedDBService {
  protected dbName = INDEXEDDB_SELECT.INDEXEDDB_DB_NAME_CONFIG;
  protected dbVersion = INDEXEDDB_SELECT.INDEXEDDB_DB_VERSION;
  protected db: IDBDatabase | null = null;
  private tableConfigs = inject(TABLE_REGISTRY);
  private initPromise: Promise<void> | null = null;
  private legacyCleanupDone = false;

  async initDB(): Promise<void> {
    if (this.db) return;
    if (this.initPromise) return this.initPromise;

    this.initPromise = new Promise((resolve, reject) => {
      const request = indexedDB.open(this.dbName, this.dbVersion);

      // story-05 debt cleanup: reset `initPromise` khi open fail — nếu không, một lần reject
      // (điển hình `VersionError` sau khi bump version ở tab cũ) khoá vĩnh viễn MỌI thao tác
      // IndexedDB của tab đó tới khi F5, vì initDB() sau đó luôn trả lại promise ĐÃ reject này.
      request.onerror = () => {
        this.initPromise = null;
        reject(request.error);
      };
      request.onblocked = () => {
        this.initPromise = null;
        reject(new Error('IDB open blocked by another tab'));
      };
      request.onsuccess = () => {
        this.db = request.result;
        this.db.onclose = () => {
          this.db = null;
          this.initPromise = null;
        };
        this.db.onversionchange = () => {
          this.db?.close();
          this.db = null;
          this.initPromise = null;
        };
        // Async cleanup các DB cũ — fire-and-forget
        void this.cleanupLegacyDBs();
        resolve();
      };

      request.onupgradeneeded = (event) => {
        const db = (event.target as IDBOpenDBRequest).result;
        const tx = (event.target as IDBOpenDBRequest).transaction!;

        this.tableConfigs.forEach((config) => {
          let store: IDBObjectStore;
          if (!db.objectStoreNames.contains(config.name)) {
            store = db.createObjectStore(config.name, { keyPath: config.keyPath });
          } else {
            store = tx.objectStore(config.name);
          }

          // Tạo index nếu chưa có (idempotent qua nhiều lần upgrade)
          config.indexes?.forEach((idx) => {
            if (!store.indexNames.contains(idx.name)) {
              store.createIndex(idx.name, idx.field, { unique: !!idx.unique });
            }
          });
        });

        if (!db.objectStoreNames.contains('metadata')) {
          db.createObjectStore('metadata', { keyPath: 'key' });
        }
      };
    });

    return this.initPromise;
  }

  /** Xóa các DB legacy (EmrConfigDB, CustomerLeadDB) sau khi đã merge vào DB chính. */
  private async cleanupLegacyDBs(): Promise<void> {
    if (this.legacyCleanupDone) return;
    this.legacyCleanupDone = true;
    LEGACY_DB_NAMES.forEach((name) => {
      try {
        indexedDB.deleteDatabase(name);
      } catch {
        // ignore — best-effort cleanup
      }
    });
  }

  async getAll<T>(storeName: string): Promise<T[]> {
    if (!this.db) await this.initDB();

    return new Promise((resolve, reject) => {
      try {
        const transaction = this.db!.transaction([storeName], 'readonly');
        const store = transaction.objectStore(storeName);
        const request = store.getAll();

        request.onsuccess = () => resolve(request.result || []);
        request.onerror = () => reject(request.error);
        transaction.onerror = () => reject(transaction.error);
        transaction.onabort = () => reject(new Error(`IDB transaction aborted: ${storeName}`));
      } catch (error) {
        reject(error);
      }
    });
  }

  async saveAll<T>(storeName: string, items: T[]): Promise<void> {
    if (!this.db) await this.initDB();

    return new Promise((resolve, reject) => {
      try {
        const transaction = this.db!.transaction([storeName], 'readwrite');
        const store = transaction.objectStore(storeName);

        store.clear();
        items.forEach((item) => store.add(item));

        transaction.oncomplete = () => resolve();
        transaction.onerror = () => reject(transaction.error);
        transaction.onabort = () => reject(new Error(`IDB transaction aborted: ${storeName}`));
      } catch (error) {
        reject(error);
      }
    });
  }

  /** Query qua object store index (đã tạo trong onupgradeneeded). Fallback scan + filter. */
  async getByIndex<T>(storeName: string, indexName: string, value: unknown): Promise<T[]> {
    if (!this.db) await this.initDB();

    return new Promise((resolve, reject) => {
      try {
        const transaction = this.db!.transaction([storeName], 'readonly');
        const store = transaction.objectStore(storeName);
        if (store.indexNames.contains(indexName)) {
          const request = store.index(indexName).getAll(value as IDBValidKey);
          request.onsuccess = () => resolve((request.result as T[]) || []);
          request.onerror = () => reject(request.error);
          return;
        }
        // Fallback: index không tồn tại → scan toàn bộ
        const request = store.getAll();
        request.onsuccess = () => {
          const filtered = (request.result || []).filter((it: any) => it?.[indexName] === value);
          resolve(filtered as T[]);
        };
        request.onerror = () => reject(request.error);
      } catch (error) {
        reject(error);
      }
    });
  }

  async getItem<T>(storeName: string, key: IDBValidKey): Promise<T | null> {
    if (!this.db) await this.initDB();

    return new Promise((resolve, reject) => {
      try {
        const transaction = this.db!.transaction([storeName], 'readonly');
        const store = transaction.objectStore(storeName);
        const request = store.get(key);
        request.onsuccess = () => resolve((request.result as T) ?? null);
        request.onerror = () => reject(request.error);
        transaction.onerror = () => reject(transaction.error);
      } catch (error) {
        reject(error);
      }
    });
  }

  async deleteItem(storeName: string, key: IDBValidKey): Promise<void> {
    if (!this.db) await this.initDB();

    return new Promise((resolve, reject) => {
      try {
        const transaction = this.db!.transaction([storeName], 'readwrite');
        const store = transaction.objectStore(storeName);
        const request = store.delete(key);
        request.onsuccess = () => resolve();
        request.onerror = () => reject(request.error);
        transaction.onerror = () => reject(transaction.error);
      } catch (error) {
        reject(error);
      }
    });
  }

  async putItem<T>(storeName: string, item: T): Promise<void> {
    if (!this.db) await this.initDB();

    return new Promise((resolve, reject) => {
      try {
        const transaction = this.db!.transaction([storeName], 'readwrite');
        const store = transaction.objectStore(storeName);
        const request = store.put(item);
        request.onsuccess = () => resolve();
        request.onerror = () => reject(request.error);
      } catch (error) {
        reject(error);
      }
    });
  }

  async getLocalVersion(): Promise<number> {
    if (!this.db) await this.initDB();

    return new Promise((resolve, reject) => {
      const transaction = this.db!.transaction(['metadata'], 'readonly');
      const store = transaction.objectStore('metadata');
      const request = store.get('versionDexie');

      request.onsuccess = () => resolve(request.result?.value || 0);
      request.onerror = () => reject(request.error);
    });
  }

  async setLocalVersion(version: number): Promise<void> {
    if (!this.db) await this.initDB();

    return new Promise((resolve, reject) => {
      const transaction = this.db!.transaction(['metadata'], 'readwrite');
      const store = transaction.objectStore('metadata');
      const request = store.put({ key: 'versionDexie', value: version });

      request.onsuccess = () => resolve();
      request.onerror = () => reject(request.error);
    });
  }

  /** Per-table versions để hỗ trợ Phương án B — sync chỉ table mismatch */
  async getTableVersions(): Promise<Record<string, number>> {
    if (!this.db) await this.initDB();

    return new Promise((resolve, reject) => {
      const transaction = this.db!.transaction(['metadata'], 'readonly');
      const store = transaction.objectStore('metadata');
      const request = store.get('dexieTableVersions');

      request.onsuccess = () => resolve(request.result?.value || {});
      request.onerror = () => reject(request.error);
    });
  }

  async setTableVersions(versions: Record<string, number>): Promise<void> {
    if (!this.db) await this.initDB();

    return new Promise((resolve, reject) => {
      const transaction = this.db!.transaction(['metadata'], 'readwrite');
      const store = transaction.objectStore('metadata');
      const request = store.put({ key: 'dexieTableVersions', value: versions });

      request.onsuccess = () => resolve();
      request.onerror = () => reject(request.error);
    });
  }

  /** Get/set generic metadata key — dùng cho versionEmr, customerLeadVersion, … */
  async getMeta<T = unknown>(key: string): Promise<T | undefined> {
    if (!this.db) await this.initDB();
    return new Promise((resolve, reject) => {
      const transaction = this.db!.transaction(['metadata'], 'readonly');
      const store = transaction.objectStore('metadata');
      const request = store.get(key);
      request.onsuccess = () => resolve(request.result?.value);
      request.onerror = () => reject(request.error);
    });
  }

  async setMeta<T>(key: string, value: T): Promise<void> {
    if (!this.db) await this.initDB();
    return new Promise((resolve, reject) => {
      const transaction = this.db!.transaction(['metadata'], 'readwrite');
      const store = transaction.objectStore('metadata');
      const request = store.put({ key, value });
      request.onsuccess = () => resolve();
      request.onerror = () => reject(request.error);
    });
  }
}
