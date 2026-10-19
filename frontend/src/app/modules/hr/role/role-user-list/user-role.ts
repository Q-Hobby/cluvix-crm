import {ChangeDetectionStrategy, Component, computed, HostListener, inject, OnInit, signal} from '@angular/core';
import { takeUntilDestroyed } from '@angular/core/rxjs-interop';
import { ButtonSeverity } from 'primeng/button';
// Import TRỰC TIẾP (không qua barrel `@/define`): matrix ~180 KB chỉ màn này dùng, đi qua barrel
// là kéo nó vào bundle eager. Xem đầu file permission-matrix.define.ts.
import { PERMISSION_MATRIX } from "@/define/permission-matrix.define";
import {finalize, lastValueFrom, take} from 'rxjs';

import {RoleModel} from '@/core/models/hr/role.model';
import {RoleForm} from '@/modules/hr/role/role-form/role-form';
import {BaseList} from '@/share/base/base-list';
import {ListShareImports} from '@/share/import/list-share-imports';
import {RadioButtonModule} from 'primeng/radiobutton';
import {RbacService} from '@/core/services/auth/rbac.service';
import { AccordionModule } from 'primeng/accordion';

/** ---- Types ---- */
export interface PermissionItem {
  value: string | null;
  label: string;
  type: number;
  parent: number;
}

export interface PermissionModule {
  id: number;
  name: string;
  nameKey?: string;
  appModule: string;
  typeModule: string;
  typeModuleKey?: string;
  viewMode?: 0 | 1;
  permissions: PermissionItem[];
}

export interface AppModuleGroup {
  appModule: string;
  label: string;
  labelKey?: string;
  modules: PermissionModule[];
}

export interface RoleOption {
  label: string;
  value: string;
  icon?: string;
}

interface ConfirmConfig {
  header: string;
  message: string;
  icon: string;
  iconBgClass: string;
  iconColorClass: string;
  acceptLabel: string;
  acceptSeverity: ButtonSeverity;
  onAccept: () => void;
}

// story-22: trim còn đúng 4 appModule thật sự xuất hiện trong PERMISSION_MATRIX (52 quyền catalog
// BE) — cxEmr/cxPharmacy/cxCare/cxLoyalty/cxSale/cxMarketing/cxAi/cxZalo/"Kho & Vật tư" đã hết module gán
// (danh sách này đã filter theo byKey.has nên trước đây chỉ là dead entry, không lỗi runtime).
const APP_MODULE_ORDER: { key: string; label: string; labelKey?: string }[] = [
  { key: 'cxCrm',   label: 'cxCrm — Vận hành KH & Lịch hẹn', labelKey: 'hr.role_page.app_module.cxCrm' },
  { key: 'Kế toán', label: 'Kế toán', labelKey: 'common.label.accounting' },
  { key: 'Nhân sự', label: 'Nhân sự', labelKey: 'common.label.human_resources' },
  { key: 'Cấu hình', label: 'Cấu hình', labelKey: 'common.label.configuration' },
];

export function getTableRole(): number {
  const width = window.innerWidth;

  if (width >= 3840) return 145;
  if (width >= 2560) return 145;
  if (width >= 1920) return 142;
  if (width >= 1512) return 125;
  if (width >= 1440) return 125;
  if (width >= 1024) return 125;
  if (width >= 768) return 115;
  if (width >= 640) return 115;

  return 115;
}

export function getMenuRole(): number {
  const width = window.innerWidth;

  if (width >= 3840) return 200;
  if (width >= 2560) return 200;
  if (width >= 1920) return 200;
  if (width >= 1512) return 185;
  if (width >= 1440) return 185;
  if (width >= 1024) return 184;
  if (width >= 768) return 184;
  if (width >= 640) return 184;

  return 184;
}

export function getTablePermission(): number {
  const width = window.innerWidth;

  if (width >= 3840) return 230;
  if (width >= 2560) return 230;
  if (width >= 1920) return 227;
  if (width >= 1512) return 210;
  if (width >= 1440) return 210;
  if (width >= 1024) return 210;
  if (width >= 768) return 200;
  if (width >= 640) return 200;

  return 200;
}

@Component({
  changeDetection: ChangeDetectionStrategy.OnPush,
  selector: 'app-users-user-role-list',
  templateUrl: './user-role.html',
  styleUrls: ['./user-role.scss'],
  standalone: true,
  imports: [...ListShareImports, RadioButtonModule, AccordionModule],
})
export class UserRole extends BaseList<RoleModel> implements OnInit {
  modelService = inject(RbacService);

  /** UI sizes */
  plusTableRole = signal(getTableRole());
  plusMenuRole = signal(getMenuRole());
  plusTablePermissionRole = signal(getTablePermission());
  /** State (signals) */
  roles = signal<RoleOption[]>([]);
  activeRole = signal<RoleOption | null>(null);
  permissions = signal<PermissionModule[]>([]);
  // moduleId -> set permission values
  selectedByModule = signal<Record<number, Set<string>>>({});
  baselineByModule = signal<Record<number, Set<string>>>({});
  isReloading = signal(false);
  syncing = signal(false);
  isUpdating = signal(false);
  hasDiff = computed(() => this.diffChanges().length > 0);
  confirmVisible = signal(false);
  confirmState = signal<ConfirmConfig | null>(null);
  activeAccordionPanels = signal<string[]>(APP_MODULE_ORDER.map(g => g.key));
  
  // Loading states
  loadingRoles = signal(true);
  loadingPermissions = signal(false);

  /** Gom nhóm permissions theo appModule theo đúng thứ tự APP_MODULE_ORDER */
  appModuleGroups = computed<AppModuleGroup[]>(() => {
    const byKey = new Map<string, PermissionModule[]>();
    for (const m of this.permissions()) {
      const key = m.appModule ?? 'Khác';
      const list = byKey.get(key) ?? [];
      list.push(m);
      byKey.set(key, list);
    }
    return APP_MODULE_ORDER
      .filter(g => byKey.has(g.key))
      .map(g => ({ appModule: g.key, label: g.label, labelKey: g.labelKey, modules: byKey.get(g.key)! }));
  });

  /** Map appModule → group để tra O(1) trong các hàm check-all (thay .find() lặp mỗi CD) */
  private groupByAppModule = computed(() => {
    const map = new Map<string, AppModuleGroup>();
    for (const g of this.appModuleGroups()) map.set(g.appModule, g);
    return map;
  });

  /**
   * Precompute toàn bộ trạng thái checkbox "chọn tất cả" (global / nhóm / cột / hàng).
   * Template đọc O(1) từ đây thay vì gọi method lặp mỗi chu kỳ change detection.
   * Chỉ tính lại khi selectedByModule()/permissions() đổi (memo hóa qua computed).
   */
  readonly aggChecked = computed(() => {
    const state = this.selectedByModule();
    const rows: Record<number, boolean> = {};
    const byGroup: Record<string, boolean> = {};
    const byColumn: Record<string, Record<number, boolean>> = {};
    let global = this.permissions().length > 0;

    for (const g of this.appModuleGroups()) {
      let groupAll = true;
      // cols[t] = mọi module có quyền type t đều đã tick; colHasAny[t] = nhóm có tồn tại cột t
      const cols: Record<number, boolean> = { 1: true, 2: true, 3: true, 4: true, 5: true };
      const colHasAny: Record<number, boolean> = { 1: false, 2: false, 3: false, 4: false, 5: false };

      for (const mod of g.modules) {
        const set = state[mod.id] ?? new Set<string>();
        let rowAll = true;
        for (const p of mod.permissions) {
          if (!p.value) continue;
          const has = set.has(p.value);
          if (!has) {
            rowAll = false;
            groupAll = false;
            global = false;
          }
          colHasAny[p.type] = true;
          if (!has) cols[p.type] = false;
        }
        rows[mod.id] = rowAll;
      }

      byGroup[g.appModule] = groupAll;
      // columnAllChecked cũ: trả false nếu nhóm không có cột đó (hasAny=false)
      const finalCols: Record<number, boolean> = {};
      for (let t = 1; t <= 5; t++) finalCols[t] = colHasAny[t] && cols[t];
      byColumn[g.appModule] = finalCols;
    }

    return { global, rows, byGroup, byColumn };
  });

  private resizeTimeout?: number;

  constructor() {
    super();
  }

  @HostListener('window:resize')
  onResize() {
    if (this.resizeTimeout) clearTimeout(this.resizeTimeout);
    this.resizeTimeout = window.setTimeout(() => {
      this.plusTableRole.set(getTableRole());
      this.plusMenuRole.set(getMenuRole());
      this.plusTablePermissionRole.set(getTablePermission());
    }, 150);
  }

  ngOnInit() {
    this.loadingRoles.set(true);
    this.permissions.set(PERMISSION_MATRIX as PermissionModule[]);
    this.loadRoles();
    this.initBaseList();
  }

  saveRole(name: string | null): void {
    this.dialogRef = this.dialogWrapperService.open(RoleForm, {
      width: '520px',
      data: {name},
      header: this.translate.instant(name == null ? 'hr.role_page.create_role_header' : 'common.action.update'),
    });

    this.dialogRef.onClose.pipe(take(1), takeUntilDestroyed(this.destroyRef)).subscribe((result: any) => {
      if (result) {
        this.noti.successMsg(this.translate.instant(name == null ? 'common.message.create_success' : 'common.message.update_success'));
        this.loadRoles(result?.value ?? this.activeRole()?.value);
      }
    });
  }

  deleteRoles(event: Event, name: string): void {
    this.openConfirm({
      header: this.translate.instant('hr.role_page.confirm_delete_header'),
      message: this.translate.instant('hr.role_page.confirm_delete_msg'),
      icon: 'pi pi-trash',
      iconBgClass: 'bg-red-100 dark:bg-red-400/30 border-red-200 dark:border-red-400/20',
      iconColorClass: 'text-red-500 dark:text-red-200',
      acceptLabel: this.translate.instant('common.action.delete'),
      acceptSeverity: 'danger',
      onAccept: () => {
        this.modelService.deleteRole(name).pipe(take(1), takeUntilDestroyed(this.destroyRef)).subscribe(() => {
          this.noti.successMsg(this.translate.instant('common.message.data_deleted'));
          this.loadRoles();
        });
      },
    });
  }

  onActiveItemChange(item: RoleOption): void {
    this.activeRole.set(item);
    this.loadingPermissions.set(true);

    this.modelService
      .getListPermissionByRole(item.value)
      .pipe(finalize(() => this.loadingPermissions.set(false)), takeUntilDestroyed(this.destroyRef))
      .subscribe((allowedValues: string[]) => {
        const allowed = new Set(allowedValues ?? []);
        const selected: Record<number, Set<string>> = {};
        const baseline: Record<number, Set<string>> = {};

        for (const m of this.permissions()) {
          const sel = new Set<string>();
          for (const p of m.permissions) {
            if (p.value && allowed.has(p.value)) sel.add(p.value);
          }
          selected[m.id] = sel;
          baseline[m.id] = new Set(sel);
        }

        this.selectedByModule.set(selected);
        this.baselineByModule.set(baseline);
      });
  }

  /** Toggle 1 permission */
  togglePermission(moduleId: number, perm: PermissionItem, checked: boolean): void {
    if (!perm.value) return;
    this.selectedByModule.update((state) => {
      const next = new Set(state[moduleId] ?? []);
      if (checked) {
        next.add(perm.value!);
      } else {
        next.delete(perm.value!);
      }
      return {...state, [moduleId]: next};
    });
  }

  /** Chọn tất cả theo hàng (1 module) */
  selectAllRow(moduleId: number, enable: boolean): void {
    const mod = this.permissions().find((m) => m.id === moduleId);
    if (!mod) return;

    const values = mod.permissions.map((p) => p.value).filter((v): v is string => !!v);

    this.selectedByModule.update((state) => {
      const set = new Set(state[moduleId] ?? []);
      for (const v of values) {
        if (enable) {
          set.add(v);
        } else {
          set.delete(v);
        }
      }
      return {...state, [moduleId]: set};
    });
  }

  /** Chọn tất cả theo nhóm app module */
  selectAllGroup(appModule: string, enable: boolean): void {
    const group = this.groupByAppModule().get(appModule);
    if (!group) return;
    this.selectedByModule.update((state) => {
      const next = {...state};
      for (const mod of group.modules) {
        const set = new Set(next[mod.id] ?? []);
        for (const p of mod.permissions) {
          if (p.value) enable ? set.add(p.value) : set.delete(p.value);
        }
        next[mod.id] = set;
      }
      return next;
    });
  }

  updatePermission(): void {
    const role = this.activeRole()?.value;
    if (!role || !this.hasDiff() || this.isUpdating()) return;

    this.isUpdating.set(true);
    this.modelService.updatePermissionWithRole(role, this.diffChanges())
      .pipe(finalize(() => this.isUpdating.set(false)), takeUntilDestroyed(this.destroyRef))
      .subscribe(() => {
        this.noti.successMsg(this.translate.instant('hr.role_page.update_perm_success'));
        const current = this.selectedByModule();
        const baseline: Record<number, Set<string>> = {};
        for (const [key, val] of Object.entries(current)) {
          baseline[+key] = new Set(val);
        }
        this.baselineByModule.set(baseline);
      });
  }

  selectAllColumn(appModule: string, permType: number, enable: boolean): void {
    const group = this.groupByAppModule().get(appModule);
    if (!group) return;
    this.selectedByModule.update((state) => {
      const next = { ...state };
      for (const mod of group.modules) {
        const perm = mod.permissions.find(p => p.type === permType && p.value);
        if (!perm?.value) continue;
        const set = new Set(next[mod.id] ?? []);
        enable ? set.add(perm.value) : set.delete(perm.value);
        next[mod.id] = set;
      }
      return next;
    });
  }

  // Kiểm tra 1 quyền đã được tick cho module chưa
  hasPermission(moduleId: number, value: string | null): boolean {
    if (!value) return false;
    const set = this.selectedByModule()[moduleId];
    return set ? set.has(value) : false;
  }

  permissionsList(m: PermissionModule) {
    return m.permissions;
  }

  /** Chọn/Bỏ tất cả quyền toàn bảng */
  selectAllGlobal(enable: boolean): void {
    this.selectedByModule.update((state) => {
      const next: Record<number, Set<string>> = {};
      for (const m of this.permissions()) {
        next[m.id] = new Set(state[m.id] ?? []);
        for (const p of m.permissions) {
          if (p.value) {
            enable ? next[m.id].add(p.value) : next[m.id].delete(p.value);
          }
        }
      }
      return {...state, ...next};
    });
  }

  override async resetPage(hard = false): Promise<void> {
    if (hard) {
      window.location.reload();
      return;
    }

    this.isReloading.set(true);
    this.loadingRoles.set(true);
    this.loadingPermissions.set(true);
    
    try {
      const keepRole = this.activeRole()?.value ?? null;

      const response = await lastValueFrom(this.modelService.getChildRoleForDropdown());

      const roles = Array.isArray(response) ? response : (response as any)?.data ?? [];
      this.permissions.set(PERMISSION_MATRIX as PermissionModule[]);
      this.roles.set(roles);
      this.loadingRoles.set(false);

      const next = (keepRole && roles.find((r: RoleOption) => r.value === keepRole)) ?? roles[0];
      if (!next) {
        // Không còn role nào → về empty state, tránh onActiveItemChange(undefined) crash
        this.activeRole.set(null);
        this.selectedByModule.set({});
        this.baselineByModule.set({});
        this.loadingPermissions.set(false);
        return;
      }
      this.onActiveItemChange(next);
      this.activeAccordionPanels.set(APP_MODULE_ORDER.map(g => g.key));
    } finally {
      this.isReloading.set(false);
    }
  }

  openConfirm(config: ConfirmConfig): void {
    this.confirmState.set(config);
    this.confirmVisible.set(true);
  }

  closeConfirm(): void {
    this.confirmVisible.set(false);
    this.confirmState.set(null);
  }

  private loadRoles(selectValue?: string): void {
    this.loadingRoles.set(true);
    this.modelService.getChildRoleForDropdown()
      .pipe(finalize(() => this.loadingRoles.set(false)), takeUntilDestroyed(this.destroyRef))
      .subscribe((response: any) => {
        const res = Array.isArray(response) ? response : response?.data || [];
        if (!res.length) return;
        this.roles.set(res);

        const next =
          (selectValue && res.find((r: RoleOption) => r.value === selectValue)) ??
          (this.activeRole() && res.find((r: RoleOption) => r.value === this.activeRole()!.value)) ??
          res[0];

        this.onActiveItemChange(next);
      });
  }

  /** Diff để cập nhật DB */
  private diffChanges(): { oldPer: string | null; newPer: string | null }[] {
    const before = this.baselineByModule();
    const after = this.selectedByModule();
    const changes: { oldPer: string | null; newPer: string | null }[] = [];

    for (const m of this.permissions()) {
      const b = before[m.id] ?? new Set<string>();
      const a = after[m.id] ?? new Set<string>();

      for (const v of a) if (!b.has(v)) changes.push({oldPer: null, newPer: v});
      for (const v of b) if (!a.has(v)) changes.push({oldPer: v, newPer: null});
    }
    return changes;
  }
}
