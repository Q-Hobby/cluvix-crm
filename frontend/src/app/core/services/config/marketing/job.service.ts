import {Service} from '@angular/core';
import {BaseApiService} from "@/share/services/base-api.service";
import {JobModel} from "@/core/models/config/marketing/job.model";

@Service()
export class JobService extends BaseApiService<JobModel> {
  protected override apiModule = 'config';
    protected override endpoint = 'job';
    protected override dateFields = [];
    protected override unixTimesTampFields = ['created_at'];

    override getInitialObject() {
        return {
            name: '',
            desc: null,
            status: 1,
        };
    }
}
