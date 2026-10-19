import {ChangeDetectionStrategy, Component, effect, inject, OnInit, signal, ViewChild,} from '@angular/core';
import {Table} from 'primeng/table';
import { FillHeightDirective } from '@/share/directive/fill-height.directive';
import {ListShareImports} from "@/share/import/list-share-imports";
import {JobModel} from "@/core/models/config/marketing/job.model";
import {BaseListV2} from "@/share/base/base-list-v2";
import {FilterFieldDef} from "@/share/base/list-filter.model";
import {JobService} from "@/core/services/config/marketing/job.service";
import {JobForm} from "@/modules/config/config-customer/job/job-form/job-form";
import {PERMISSION_SELECT} from "@/define";

@Component({
  changeDetection: ChangeDetectionStrategy.OnPush,
    selector: 'app-job-list',
    templateUrl: './job-list.html',
    styles: [],
    standalone: true,
    imports: [FillHeightDirective, ...ListShareImports],
})
export class JobList extends BaseListV2<JobModel> implements OnInit {
    @ViewChild('dt') dt!: Table<JobModel>;
    protected override URL_STATE_PREFIX = 'jo';
    modelService = inject(JobService);
    displayCols: any = [
        { label: 'ID', labelKey: 'common.label.id', value: 'id' },
        { label: 'Nghề nghiệp', labelKey: 'common.label.occupation', value: 'name' },
        { label: 'Mô tả', labelKey: 'common.label.description', value: 'desc' },
        { label: 'Trạng thái', labelKey: 'common.label.status', value: 'status' },
        { label: 'Vị trí', labelKey: 'common.label.sort_order', value: 'SortOrder' },
    ];
    override selectedColumns = signal(['id', 'name', 'desc', 'status', 'SortOrder']);

    override formComponent = JobForm;    override updatePermissionString = PERMISSION_SELECT.permissionManageConfig;
    override createPermissionString = PERMISSION_SELECT.permissionManageConfig;    constructor() {
        super();
        effect(() => {
            const len = this.selectedColumns()?.length ?? 0;
            this.padCols.set(len + 2);
        });
    }

    /**
     * Khai báo filter — list này chỉ có ô tìm kiếm theo từ khoá.
     */
    protected filterFields(): FilterFieldDef[] {
        return [{ name: '_keyword', kind: 'text' }];
    }

    ngOnInit() {
        this.initListV2();
    }

    checkUpdatePermission() {
        return !this.permissionsService.getPermission(PERMISSION_SELECT.permissionManageConfig);
    }

}
