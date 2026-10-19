import {ChangeDetectionStrategy, Component, effect, inject, OnInit, signal, ViewChild,} from '@angular/core';
import {Table} from 'primeng/table';
import { FillHeightDirective } from '@/share/directive/fill-height.directive';
import {ListShareImports} from "@/share/import/list-share-imports";
import {BaseListV2} from "@/share/base/base-list-v2";
import {FilterFieldDef} from "@/share/base/list-filter.model";
import {PERMISSION_SELECT} from "@/define";
import {EthnicityModel} from "@/core/models/config/marketing/ethnicity.model";
import {EthnicityService} from "@/core/services/config/marketing/ethnicity.service";
import {EthnicityForm} from "@/modules/config/config-customer/ethnicity/ethnicity-form/ethnicity-form";

@Component({
  changeDetection: ChangeDetectionStrategy.OnPush,
    selector: 'app-ethnicity-list',
    templateUrl: './ethnicity-list.html',
    styles: [],
    standalone: true,
    imports: [FillHeightDirective, ...ListShareImports],
})
export class EthnicityList extends BaseListV2<EthnicityModel> implements OnInit {
    @ViewChild('dt') dt!: Table<EthnicityModel>;
    protected override URL_STATE_PREFIX = 'et';
    modelService = inject(EthnicityService);
    displayCols: any = [
        { label: 'ID', labelKey: 'common.label.id', value: 'id' },
        { label: 'Dân tộc', labelKey: 'common.label.ethnicity', value: 'name' },
        { label: 'Mã dân tộc', labelKey: 'config.ethnicity.code', value: 'codeEthnicity' },
        { label: 'Trạng thái', labelKey: 'common.label.status', value: 'status' },
        { label: 'Vị trí', labelKey: 'common.label.sort_order', value: 'SortOrder' },
    ];
    override selectedColumns = signal(['id', 'name', 'codeEthnicity', 'status', 'SortOrder']);

    override formComponent = EthnicityForm;    override updatePermissionString = PERMISSION_SELECT.permissionManageConfig;
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
