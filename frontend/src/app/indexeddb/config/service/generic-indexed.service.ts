import { computed, signal } from '@angular/core';
import { BaseIndexedDBService } from './base-indexed-db.service';

export interface IndexedEntity {
  id: number;
}

export class GenericIndexedService<T extends IndexedEntity> {
  private data = signal<T[]>([]);
  private loaded = signal<boolean>(false);
  private dataMap = computed(() => {
    const map = new Map<number, T>();
    this.data().forEach((item) => map.set(item.id, item));
    return map;
  });

  constructor(
    private storeName: string,
    private baseService: BaseIndexedDBService,
  ) {}

  async load(): Promise<void> {
    await this.baseService.initDB();
    const items = await this.baseService.getAll<T>(this.storeName);
    this.data.set(items);
    this.loaded.set(true);
  }

  async save(items: T[]): Promise<void> {
    await this.baseService.initDB();
    await this.baseService.saveAll(this.storeName, items);
    this.data.set(items);
    this.loaded.set(true);
  }

  getById(id: number): T | undefined {
    return this.dataMap().get(id);
  }

  getAll() {
    return this.data.asReadonly();
  }

  isLoaded(): boolean {
    return this.loaded();
  }
}
