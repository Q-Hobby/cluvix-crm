import { ChangeDetectionStrategy, Component, computed, inject, OnInit, signal } from '@angular/core';
import { FormsModule } from '@angular/forms';
import { TranslatePipe, TranslateService } from '@ngx-translate/core';
import { toSignal } from '@angular/core/rxjs-interop';
import { ButtonModule } from 'primeng/button';
import { DatePicker } from 'primeng/datepicker';
import { Select } from 'primeng/select';
import { SelectButton } from 'primeng/selectbutton';

import { BranchCacheService } from '@/indexeddb/config/components/branch/branch-cache.service';

import { DashboardFilterStore } from './dashboard-filter.store';
import {
  createCustomPickerState, customRange, DASHBOARD_PRESETS, DashboardFilterState, DashboardPreset, datesFromRange, isRangeTooLong, previousRange,
} from './dashboard-filter.util';

/** i18n key nhãn preset — tái dùng common.* khi đã có (tra `npm run i18n:find`). */
const PRESET_LABEL: Record<DashboardPreset, string> = {
  today: 'common.label.today',
  last_7_days: 'common.label.last_7_days',
  this_month: 'common.label.this_month',
  last_month: 'common.label.previous_month',
  this_quarter: 'dashboard.crm.filter.this_quarter',
  this_year: 'common.label.this_year',
  custom: 'dashboard.crm.filter.custom',
};

/**
 * Thanh lọc dính đầu trang (PRD FR10/FR20): preset dạng nút phân đoạn (mobile cuộn ngang), "Tuỳ chọn" mở chọn
 * khoảng ngày (chỉ áp khi bấm Áp dụng; chặn > 366 ngày), chọn chi nhánh trong phạm vi user. Khe
 * `[dashboardActions]` cho nút "Thêm khối"/"Tuỳ chỉnh" (story-10).
 */
@Component({
  selector: 'app-dashboard-filter-bar',
  changeDetection: ChangeDetectionStrategy.OnPush,
  imports: [FormsModule, TranslatePipe, SelectButton, DatePicker, Select, ButtonModule],
  styleUrl: './dashboard-filter-bar.scss',
  template: `
    <div class="dash-sticky sticky z-50 -mx-1 px-1 py-2 bg-[var(--p-content-background)] border-b border-[var(--p-content-border-color)]">
      <div class="flex flex-wrap items-center gap-2">
        <div class="min-w-0 max-w-full overflow-x-auto">
          <p-selectbutton
            [options]="presetOptions"
            [ngModel]="shownPreset()"
            (ngModelChange)="onPreset($event)"
            optionLabel="label"
            optionValue="value"
            [allowEmpty]="false"
            size="small"
            [attr.aria-label]="'dashboard.crm.filter.period' | translate">
            <ng-template #item let-opt>
              <span class="whitespace-nowrap">{{ opt.label | translate }}</span>
            </ng-template>
          </p-selectbutton>
        </div>

        @if (customOpen()) {
          <div class="flex items-center gap-2">
            <p-datepicker
              [ngModel]="draft()"
              (ngModelChange)="draft.set($event)"
              selectionMode="range"
              [readonlyInput]="true"
              [showIcon]="true"
              size="small"
              dateFormat="dd/mm/yy"
              appendTo="body"
              inputId="dashboard_filter_range"
              [ariaLabel]="'dashboard.crm.filter.custom' | translate" />
            <!-- p-button: màu chữ theo token theme (text-white tự đặt trượt AA trên primary dark — QA P5). -->
            <p-button class="shrink-0" styleClass="whitespace-nowrap" size="small" [label]="'common.action.apply' | translate" [disabled]="!canApply()" (onClick)="applyCustom()" />
          </div>
        }

        <div class="w-full sm:w-56">
          <p-select
            [options]="branchOptions()"
            optionLabel="name"
            optionValue="id"
            [ngModel]="store.state().branchId"
            (ngModelChange)="store.setBranch($event)"
            [filter]="branchOptions().length > 8"
            size="small"
            filterBy="name"
            appendTo="body"
            inputId="dashboard_filter_branch"
            [ariaLabel]="'common.label.branch' | translate"
            styleClass="w-full" />
        </div>

        <div class="ml-auto flex items-center gap-2">
          <ng-content select="[dashboardActions]" />
        </div>
      </div>

      <!-- Hàng kỳ: luôn render (không CLS), nêu RÕ khoảng ngày đang xem và kỳ mà chip % đem ra so. -->
      <div class="mt-2 flex flex-wrap items-center gap-x-6 gap-y-1 text-base"
           aria-live="polite">
        <span class="inline-flex items-center gap-1.5 min-w-0">
          <i class="pi pi-calendar text-[var(--p-primary-color)]" aria-hidden="true"></i>
          <span class="text-[var(--p-text-color)]">{{ 'dashboard.crm.filter.period' | translate }}:</span>
          <span class="font-semibold text-[var(--p-primary-color)] tabular-nums whitespace-nowrap">{{ periodText() }}</span>
        </span>
        <span class="inline-flex items-center gap-1.5 min-w-0">
          <i class="pi pi-arrow-right-arrow-left text-[var(--p-primary-color)]" aria-hidden="true"></i>
          <span class="text-[var(--p-text-color)]">{{ 'dashboard.crm.filter.compare_hint' | translate }}:</span>
          <span class="font-semibold text-[var(--p-primary-color)] tabular-nums whitespace-nowrap">{{ previousText() }}</span>
        </span>
      </div>

      <div class="grid transition-[grid-template-rows] duration-200 ease-out" [style.grid-template-rows]="rangeTooLong() ? '1fr' : '0fr'"
           [attr.aria-hidden]="!rangeTooLong()">
        <div class="overflow-hidden">
          <p class="m-0 pt-1 text-xs text-[#C62828] dark:text-[#F87171]" [attr.role]="rangeTooLong() ? 'alert' : null">{{ 'dashboard.crm.filter.range_too_long' | translate }}</p>
        </div>
      </div>
    </div>
  `,
})
export class DashboardFilterBar implements OnInit {
  protected readonly store = inject(DashboardFilterStore);
  private readonly branchCache = inject(BranchCacheService);
  private readonly translate = inject(TranslateService);
  /** Chi nhánh trong phạm vi user (BranchCacheService lọc `branchListId`), đầu danh sách là "Tất cả" (QA P3). */
  private readonly branches = signal<{ id: number; name: string }[]>([]);
  private readonly allLabel = toSignal(this.translate.stream('common.label.all_branches'), { initialValue: '' });
  protected readonly branchOptions = computed(() => [{ id: null as number | null, name: this.allLabel() }, ...this.branches()]);
  protected readonly presetOptions = [...DASHBOARD_PRESETS, 'custom' as const].map((value) => ({ value, label: PRESET_LABEL[value] }));

  /** Ô "Tuỳ chọn" (mở/đóng + nháp khoảng ngày) — bám KỲ thật, không reset khi đổi chi nhánh (QA P10/P13). */
  private readonly picker = createCustomPickerState(this.store.state, (st) => this.datesFrom(st));
  protected readonly customOpen = this.picker.open;
  /** Nút đang chọn: "Tuỳ chọn" ngay khi mở ô ngày (dù chưa Áp dụng), còn lại theo trạng thái thật. */
  protected readonly shownPreset = computed<DashboardPreset>(() => (this.customOpen() ? 'custom' : this.store.state().preset));
  /** Khoảng đang chọn dở trong date picker — CHỈ áp khi bấm Áp dụng (không bắn request khi chọn dở). */
  protected readonly draft = this.picker.draft;
  protected readonly draftRange = computed(() => {
    const d = this.draft();
    if (!d || d.length < 2 || !d[0] || !d[1]) return null;
    return customRange(d[0], d[1], this.store.wallToUnix);
  });
  protected readonly rangeTooLong = computed(() => {
    const r = this.draftRange();
    return !!r && isRangeTooLong(r.from, r.to);
  });
  /** Kỳ đang xem và kỳ đem ra so (cùng công thức BE `lastPeriod`) — dạng "dd/MM/yyyy – dd/MM/yyyy". */
  protected readonly periodText = computed(() => this.rangeText(this.store.range()));
  protected readonly previousText = computed(() => {
    const r = this.store.range();
    return this.rangeText(previousRange(r.from, r.to, this.store.unixToWall, this.store.wallToUnix));
  });
  protected readonly canApply = computed(() => !!this.draftRange() && !this.rangeTooLong());

  protected onPreset(value: DashboardPreset): void {
    if (value === 'custom') {
      this.customOpen.set(true);
      return;
    }
    this.customOpen.set(false);
    this.store.setPreset(value);
  }

  protected applyCustom(): void {
    const r = this.draftRange();
    if (!r || isRangeTooLong(r.from, r.to)) return;
    this.store.setCustom(r.from, r.to);
  }

  async ngOnInit(): Promise<void> {
    await this.branchCache.loadBranches();
    this.branches.set(
      this.branchCache
        .filterBranchesByUser({ activeOnly: true })
        .filter((b) => b.id)
        .map((b) => ({ id: b.id!, name: b.name ?? '' })),
    );
  }

  private rangeText(r: { from: number; to: number }): string {
    const [a, b] = datesFromRange(r.from, r.to, this.store.unixToWall);
    const fmt = (d: Date) => `${String(d.getDate()).padStart(2, '0')}/${String(d.getMonth() + 1).padStart(2, '0')}/${d.getFullYear()}`;
    return fmt(a) === fmt(b) ? fmt(a) : `${fmt(a)} – ${fmt(b)}`;
  }

  private datesFrom(s: DashboardFilterState): Date[] | null {
    if (s.preset !== 'custom' || s.from === null || s.to === null) return null;
    return datesFromRange(s.from, s.to, this.store.unixToWall);
  }
}
