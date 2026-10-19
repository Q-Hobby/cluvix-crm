import { inject, Service} from '@angular/core';
import { JobCacheService } from './job-cache.service';
import {JobModel} from "@/core/models/config/marketing/job.model";

@Service()
export class JobHelperService {
  private cache = inject(JobCacheService);

  async ensureJobLoaded(): Promise<void> {
    await this.cache.ensureJobLoaded();
  }

  getJobName(id: number | null | undefined): string {
    if (!id) return '';
    const item = this.cache.getById(id);
    return item?.name || '-';
  }

  getJobById(id: number): JobModel | undefined {
    return this.cache.getById(id);
  }
}
