import { inject, Service} from '@angular/core';
import { GenericIndexedService, IndexedEntity } from './generic-indexed.service';
import { BaseIndexedDBService } from './base-indexed-db.service';

@Service()
export class TableServiceFactory {
  private baseService = inject(BaseIndexedDBService);
  private services = new Map<string, GenericIndexedService<any>>();

  getService<T extends IndexedEntity>(tableName: string): GenericIndexedService<T> {
    if (!this.services.has(tableName)) {
      this.services.set(tableName, new GenericIndexedService<T>(tableName, this.baseService));
    }
    return this.services.get(tableName)!;
  }
}
