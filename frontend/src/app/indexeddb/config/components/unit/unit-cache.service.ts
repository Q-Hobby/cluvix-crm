import { Service} from '@angular/core';
import type { BaseCacheFilterOptions } from '@/indexeddb/config/components/base';
import { BaseCacheService } from '@/indexeddb/config/components/base';
import { UnitModel } from '@/core/models/config/system/unit.model';

export interface UnitFilterOptions extends BaseCacheFilterOptions {
  category?: number;
  includeUnitId?: number;
  includeUnitIds?: number[];
}

@Service()
export class UnitCacheService extends BaseCacheService<UnitModel> {
  protected tableName = 'units';
  protected activeStatus = 1;
  protected allowedStatuses = [1];

  async ensureUnitLoaded(): Promise<void> {
    return this.loadItems();
  }

  async loadUnits() {
    return this.loadItems();
  }

  getAllUnits(): UnitModel[] {
    return this.getAllItems()
      .filter((item) => item.status === 1)
      .sort((a, b) => (b.SortOrder || 0) - (a.SortOrder || 0));
  }

  filterUnits(options: UnitFilterOptions = {}): UnitModel[] {
    const baseOptions = {
      ...options,
      includeItemId: options.includeUnitId,
      includeItemIds: options.includeUnitIds,
    };

    return this.filterItems(baseOptions, (units, opts) =>
      this.customUnitFilter(units, opts as UnitFilterOptions),
    );
  }

  protected override generateCacheKey(options: UnitFilterOptions): string {
    return `${options.activeOnly}_${options.category || ''}_${options.includeUnitId || ''}_${(options.includeUnitIds || []).join(',')}`;
  }

  private customUnitFilter(units: UnitModel[], options: UnitFilterOptions): UnitModel[] {
    if (options.activeOnly) {
      units = units.filter((unit) => unit.status === this.activeStatus);
    } else {
      units = units.filter((unit) => this.allowedStatuses.includes(unit.status!));
    }

    if (options.category) {
      units = units.filter((unit) => {
        const catId = unit.categoryId ?? (unit as any).category_id;
        if (Array.isArray(catId)) return catId.some((c) => +c === options.category!);
        return +catId === options.category;
      });
    }

    return units;
  }
}
