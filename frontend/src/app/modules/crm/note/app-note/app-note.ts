import {ChangeDetectionStrategy, Component, inject, Input, OnChanges, signal, ViewChild } from '@angular/core';
import { Table } from 'primeng/table';
import { finalize } from 'rxjs';

import { NoteModel } from '@/core/models/crm/note.model';
import { NoteService } from '@/core/services/crm/note.service';
import { NoteForm } from '@/modules/crm/note/note-form/note-form';
import { BaseListV2 } from '@/share/base/base-list-v2';
import { FilterFieldDef } from '@/share/base/list-filter.model';
import { CONFIG_OPTION_CRM_SELECT, PERMISSION_SELECT } from '@/define';
import { WhitespaceControlDirective } from '@/share/directive/whitespace-control.directive';
import { ListShareImports } from '@/share/import/list-share-imports';
import { userId } from '@/core/services/auth';
import { UserOptionHelperService } from '@/indexeddb/config/components/user-option';

type NoteVM = NoteModel & { __pinLoading?: boolean };

@Component({
  changeDetection: ChangeDetectionStrategy.OnPush,
  selector: 'app-note',
  templateUrl: './app-note.html',
  standalone: true,
  imports: [NoteForm, ...ListShareImports, WhitespaceControlDirective],
})
/**
 * Dùng BaseListV2 để đồng bộ base toàn module CRM (user chốt 2026-08-03, deferred-work D6).
 *
 * LƯU Ý — component này KHÔNG hưởng được tính năng nào của V2: nó hiển thị bằng card list +
 * infinite scroll (`onContainerScroll`/`loadMore`), KHÔNG có `<p-table>` nên không có `onLazyLoad`.
 * Toàn bộ URL state / stale-while-revalidate / huỷ request cũ của V2 đều đi qua `loadRecords()`
 * mà `loadRecords` chỉ chạy khi bảng lazy bắn `onLazyLoad`. Vì vậy:
 *   - KHÔNG gọi `initListV2()` (gọi sẽ vô hại nhưng vô nghĩa: `scheduleFirstLoad` thoát ngay vì `dt` undefined).
 *   - Giữ nguyên luồng nạp dữ liệu riêng: `getList(reset)` → `fetchPage()` → `listNotes`.
 * Muốn component này thật sự dùng V2 thì phải đổi sang `<p-table [lazy]>`, tức viết lại UI — việc khác.
 */
export class AppNote extends BaseListV2<NoteModel> implements OnChanges {
  /** BaseListV2 khai `abstract dt` — component này không có bảng nên luôn undefined. */
  @ViewChild('dt') dt!: Table<NoteModel>;

  /** Không có filter panel. */
  protected filterFields(): FilterFieldDef[] {
    return [];
  }

  @Input({ required: true }) customer_id!: number;
  modelService = inject(NoteService);
  listNotes: NoteVM[] = [];
  optionsDeleteNote: boolean = false;
  skeletons = [1, 2, 3, 4, 5, 6, 7, 8];
  // 🔽 Phân trang & infinite scroll
  override page = signal(1);
  readonly pageSize = 20;
  total = signal<number | null>(null); // nếu BE có trả total thì set
  loadingMore = signal(false);
  private userOptionHelper = inject(UserOptionHelperService);
  private lastChunk = 0; // kích thước lô gần nhất để suy luận còn nữa không

  constructor() {
    super();
    this.userOptionHelper.ensureSettingLoaded().then(() => {
      this.optionsDeleteNote = !!this.userOptionHelper.getUserOptionInfo(
        CONFIG_OPTION_CRM_SELECT.OPTION_DELETE_NOTE,
      )?.options;
    });
  }

  ngOnChanges() {
    this.getList(true);
  }

  noteSaved(saved: any) {
    if (saved) {
      this.getList(true);
    }
  }

  override getList(reset: boolean = true) {
    if (!this.customer_id) return;

    if (reset) {
      this.listNotes = [];
      this.page.set(1);
      this.total.set(null);
      this.lastChunk = 0;
    }
    this.fetchPage(this.page(), reset);
  }

  // Sự kiện scroll trên container
  onContainerScroll(ev: Event) {
    const el = ev.target as HTMLElement;
    const nearBottom = el.scrollTop + el.clientHeight >= el.scrollHeight - 120; // buffer 120px
    if (nearBottom && !this.loading() && !this.loadingMore() && this.hasMore()) {
      this.page.update((p) => p + 1);
      this.fetchPage(this.page(), /*reset*/ false);
    }
  }

  // Fallback nút "Tải thêm"
  loadMore() {
    if (!this.hasMore() || this.loading() || this.loadingMore()) return;
    this.page.update((p) => p + 1);
    this.fetchPage(this.page(), /*reset*/ false);
  }

  hasMore(): boolean {
    const total = this.total();
    if (typeof total === 'number') return this.listNotes.length < total;
    return this.lastChunk === this.pageSize; // nếu lô trước đủ 20 thì nhiều khả năng còn nữa
  }

  checkDeleteNote(createdAt: string | number | Date, createdBy: number) {
    const createdAtDate = createdAt instanceof Date ? createdAt : new Date(Number(createdAt));
    return (
      this.permissionsService.getPermission(PERMISSION_SELECT.permissionDeleteNote) ||
      (this.permissionsService.getPermission(PERMISSION_SELECT.permissionCreateNote) &&
        this.optionsDeleteNote &&
        userId() === createdBy &&
        this.timezoneService.now().toDateString() === createdAtDate.toDateString())
    );
  }

  deleteNote(event: Event, model: NoteModel): void {
    this.confirmationService.confirm({
      target: (event.target ?? undefined),
      message: this.translate.instant('crm.note_app.confirm_delete'),
      icon: 'pi pi-exclamation-triangle',
      acceptButtonProps: {severity: 'danger', size: 'small'},
      accept: () => {
        this.modelService.deleteNote(model.id!).subscribe({
          next: (res: any) => {
            this.noti.successRes(res);
            this.getList(true);
          },
          error: () => {},
        });
      },
    });
  }

  onTogglePin(ev: Event, n: NoteVM) {
    ev.stopPropagation();
    if (n.__pinLoading) return;

    const target = !n.pinned;
    const prevPinned = !!n.pinned;
    const prevPinnedAt = n.pinned_at ?? undefined;

    // optimistic (epoch giây)
    n.__pinLoading = true;
    n.pinned = target;
    n.pinned_at = target ? Math.floor(Date.now() / 1000) : undefined;

    this.modelService
      .togglePin(n.id!, target)
      .pipe(finalize(() => (n.__pinLoading = false)))
      .subscribe({
        next: (updated) => {
          Object.assign(n, this.normalizeNotes([updated])[0]);
          // Reload từ đầu để đảm bảo thứ tự ghim/không ghim đúng với BE
          this.getList(true);
        },
        error: () => {
          // rollback
          n.pinned = prevPinned;
          n.pinned_at = prevPinnedAt;
          this.noti.warnMsg(this.translate.instant('crm.note_app.pin_failed'));
        },
      });
  }

  private fetchPage(page: number, reset: boolean) {
    const params = {
      params: {
        'per-page': this.pageSize,
        page,
        id: this.customer_id,
      },
    };

    (reset ? this.loading : this.loadingMore).set(true);
    this.modelService
      .listNoteAdvise(params)
      .pipe(finalize(() => (reset ? this.loading : this.loadingMore).set(false)))
      .subscribe(async (res: any) => {
        try {
          const pound = await this.modelService.addLabels(res.items ?? []);
          const items = this.normalizeNotes(pound as NoteVM[]) ?? [];

          // Append + dedupe theo id, giữ thứ tự hiện có
          const existingIds = new Set(this.listNotes.map((n) => n.id));
          const toAppend = items.filter((it) => !existingIds.has(it.id));
          this.listNotes = [...this.listNotes, ...toAppend];

          // Lưu kích thước lô gần nhất để suy luận còn trang sau nếu BE không trả total
          this.lastChunk = items.length;

          this.total.set(res?._meta?.totalCount ?? 0);
        } catch (err) {
          console.error('[AppNote] listNoteAdvise async error:', err);
        }
      });
  }

  // Chuẩn hoá kiểu trả về từ BE (tránh case '0'/'1' là string)
  private normalizeNotes(items: NoteVM[]): NoteVM[] {
    return (items ?? []).map((n) => ({
      ...n,
      pinned: n.pinned === true || n.pinned === 1 || n.pinned === '1',
      pinned_at: n.pinned_at != null ? +n.pinned_at : undefined,
      created_at: n.created_at != null ? +n.created_at : 0,
    }));
  }
}
