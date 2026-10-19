import { inject, Service} from '@angular/core';
import { EthnicityCacheService } from './ethnicity-cache.service';
import {EthnicityModel} from "@/core/models/config/marketing/ethnicity.model";

@Service()
export class EthnicityHelperService {
  private cache = inject(EthnicityCacheService);

  async ensureEthnicityLoaded(): Promise<void> {
    await this.cache.ensureEthnicityLoaded();
  }

  getEthnicityName(id: number | null | undefined): string {
    if (!id) return '';
    const item = this.cache.getById(id);
    return item?.name || '-';
  }

  getEthnicityInfo(id: number | null | undefined): EthnicityModel | undefined {
    if (!id) return undefined;
    const item = this.cache.getById(id);
    return {
      codeEthnicity: item?.codeEthnicity || '',
      name: item?.name || '',
    }
  }

  getEthnicityById(id: number): EthnicityModel | undefined {
    return this.cache.getById(id);
  }
}
