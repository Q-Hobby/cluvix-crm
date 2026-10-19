import {
  ChangeDetectionStrategy, ChangeDetectorRef, Component,
  computed, DestroyRef, effect, inject, input, OnDestroy, signal,
} from '@angular/core';
import { CommonModule } from '@angular/common';
import { SkeletonModule } from 'primeng/skeleton';
import { ButtonModule } from 'primeng/button';
import { TooltipModule } from 'primeng/tooltip';
import { DialogModule } from 'primeng/dialog';
import { ProgressBarModule } from 'primeng/progressbar';
import { ConfirmPopup } from 'primeng/confirmpopup';
import { ConfirmationService } from 'primeng/api';
import { HttpErrorResponse, HttpParams } from '@angular/common/http';
import { TranslatePipe, TranslateService } from '@ngx-translate/core';
import { Subscription } from 'rxjs';

import { ApiService } from '@/share/services/api.service';
import { DialogFooter } from '@/share/components/dialog-footer';
import { NotificationService } from '@/share/services/notification.service';
import { AttachmentFileService, AttachmentMediaUrls } from '@/core/services/attachment-file.service';
import { branchActiveId } from '@/core/services/auth/auth.selectors';
import { TimezoneService } from '@/core/services/timezone.service';

interface AttachmentItem {
  id: number;
  file_name: string;
  download_path: string;
  file_type: string;
  file_size: number | null;
  note: string;
  created_at: number;
}

interface UploadEntry {
  file: File;
  name: string;
  size: number;
  error?: string;
  progress: number;
}

const MAX_MB = 20;
const MAX_BYTES = MAX_MB * 1024 * 1024;
const MAX_FILES = 5;
const PAGE_SIZE = 20;

@Component({
  changeDetection: ChangeDetectionStrategy.OnPush,
  selector: 'app-customer-attachment-audio',
  standalone: true,
  imports: [CommonModule, SkeletonModule, ButtonModule, TooltipModule, DialogModule, ProgressBarModule, ConfirmPopup, DialogFooter, TranslatePipe],
  template: `
    <div class="flex flex-col" style="max-height: calc(100vh - 260px)">
      <div
        class="flex items-center justify-between px-3 py-2 border-b border-surface-200 dark:border-surface-700 shrink-0">
        <span class="text-xs text-surface-400">{{ 'crm.customer_attachment.header_audio' | translate: { size: MAX_MB } }}</span>
        <button type="button" pButton size="small" severity="secondary" (click)="openDialog()"><i class="pi pi-upload"></i><span>{{ 'common.action.upload' | translate }}</span></button>
      </div>

      <div class="overflow-y-auto flex-1" (scroll)="onScroll($event)">
        @if (loading() && items.length === 0) {
          <div class="p-3 flex flex-col gap-3">
            @for (_ of skeletons; track $index) {
              <div class="flex flex-col gap-2 p-3 border-b border-surface-100">
                <div class="flex items-center gap-3">
                  <p-skeleton shape="circle" size="36px"/>
                  <div class="flex-1">
                    <p-skeleton height="14px" class="mb-1"/>
                    <p-skeleton height="12px" width="50%"/>
                  </div>
                </div>
                <p-skeleton height="36px"/>
              </div>
            }
          </div>
        }
        @if (!loading() && fetchError()) {
          <div class="flex flex-col items-center justify-center gap-2 py-12 text-red-400">
            <i class="pi pi-exclamation-circle text-3xl"></i>
            <span class="text-sm">{{ 'common.message.load_failed' | translate }}</span>
            <button type="button" pButton size="small" severity="secondary" (click)="reload()"><span>{{ 'common.action.retry' | translate }}</span></button>
          </div>
        }
        @if (!loading() && !fetchError() && items.length === 0) {
          <div class="flex flex-col items-center justify-center gap-2 py-12 text-surface-400">
            <i class="pi pi-volume-up text-3xl"></i>
            <span class="text-sm">{{ 'crm.customer_attachment.empty_audio' | translate }}</span>
          </div>
        }
        <div class="divide-y divide-surface-100 dark:divide-surface-800">
          @for (item of items; track item.id) {
            <div
              class="flex flex-col gap-2 px-3 py-3 hover:bg-surface-50 dark:hover:bg-surface-800/50 transition-colors">
              <div class="flex items-center gap-3">
                <div
                  class="shrink-0 w-9 h-9 rounded-lg flex items-center justify-center text-white text-xs font-bold bg-[#8b5cf6]">
                  AUD
                </div>
                <div class="flex-1 min-w-0">
                  <div class="text-sm font-medium truncate" [title]="item.file_name">{{ item.file_name }}</div>
                  <div class="flex items-center gap-2 text-xs text-surface-400">
                    @if (item.file_size) {
                      <span>{{ formatSize(item.file_size) }}</span><span>·</span>
                    }
                    <span>{{ formatDate(item.created_at) }}</span>
                  </div>
                </div>
                <button type="button" pButton [iconOnly]="true" [rounded]="true" [text]="true" size="small"
                          severity="danger" [pTooltip]="'common.action.delete' | translate" tooltipPosition="left" (click)="deleteItem(item, $event)"><i class="pi pi-trash"></i></button>
              </div>
              @if (media.urls()[item.id]; as src) {
                <audio #player controls autoplay preload="metadata" [src]="src"
                       (error)="media.onMediaError(item.id, player)" (loadedmetadata)="media.onMediaLoaded(item.id, player)"
                       class="w-full" style="height:36px"></audio>
              } @else {
                <button type="button" pButton size="small" severity="secondary" [text]="true"
                        [disabled]="media.loadingIds().has(item.id)" (click)="play(item)">
                  <i class="pi" [class]="media.loadingIds().has(item.id) ? 'pi-spin pi-spinner' : 'pi-play'"></i><span>{{ 'crm.customer_attachment.listen' | translate }}</span>
                </button>
              }
            </div>
          }
          @if (loadingMore()) {
            <div class="px-3 py-2">
              <p-skeleton height="80px"/>
            </div>
          }
          @if (!loading() && !loadingMore() && !hasMore() && items.length > 0) {
            <div class="text-center py-2 text-xs text-surface-400">{{ 'common.page.loaded_all' | translate }}</div>
          }
        </div>
      </div>
    </div>

    <p-dialog [(visible)]="dialogVisible" [modal]="true" [closable]="!uploading()"
              [header]="'crm.customer_attachment.upload_audio_title' | translate" [style]="{ width: '480px' }" (onHide)="queue.set([])">
      <div class="border-2 border-dashed rounded-lg p-6 text-center cursor-pointer transition-colors"
           [class]="dragOver() ? 'border-primary bg-primary/5' : 'border-surface-300 dark:border-surface-600 hover:border-primary'"
           (click)="fileInput.click()"
           (dragover)="$event.preventDefault(); dragOver.set(true)"
           (dragleave)="dragOver.set(false)"
           (drop)="onDrop($event)">
        <i class="pi pi-volume-up text-3xl text-surface-400 mb-2 block"></i>
        <p class="text-sm text-surface-600 dark:text-surface-300 mb-1">{{ 'crm.customer_attachment.drop_hint' | translate }} <span
          class="text-primary font-medium">{{ 'common.label.choose_file' | translate }}</span></p>
        <p class="text-xs text-surface-400">MP3, WAV, OGG, AAC, FLAC · {{ 'crm.customer_attachment.limits' | translate: { size: MAX_MB, count: MAX_FILES } }}</p>
        <input #fileInput type="file" class="hidden" multiple
               accept="audio/mpeg,audio/wav,audio/ogg,audio/aac,audio/flac,.mp3,.wav,.ogg,.aac,.flac"
               (change)="onFilesSelected($event)"/>
      </div>
      @if (queue().length > 0) {
        <div class="mt-2 flex items-center justify-between px-1">
          <span class="text-xs text-surface-400">{{ 'crm.customer_attachment.valid_count' | translate: { valid: validCount(), max: MAX_FILES } }}</span>
          @if (validCount() >= MAX_FILES) {
            <span class="text-xs text-amber-500">{{ 'crm.customer_attachment.limit_reached' | translate }}</span>
          }
        </div>
        <div class="mt-1 flex flex-col gap-2 max-h-52 overflow-y-auto">
          @for (entry of queue(); track entry.name; let i = $index) {
            <div class="flex items-center gap-3 p-2 rounded-lg bg-surface-50 dark:bg-surface-800">
              <i class="pi pi-volume-up text-[#8b5cf6] shrink-0"></i>
              <div class="flex-1 min-w-0">
                <div class="text-sm truncate" [class.text-red-500]="entry.error">{{ entry.name }}</div>
                @if (entry.error) {
                  <div class="text-xs text-red-400">{{ entry.error }}</div>
                } @else if (entry.progress > 0 && entry.progress < 100) {
                  <p-progressbar [value]="entry.progress" [showValue]="false" class="h-1 mt-1"></p-progressbar>
                } @else if (entry.progress === 100) {
                  <div class="text-xs text-green-500">{{ 'crm.customer_attachment.uploaded' | translate }}</div>
                } @else {
                  <div class="text-xs text-surface-400">{{ formatSize(entry.size) }}</div>
                }
              </div>
              @if (entry.progress === 0 && !entry.error) {
                <button type="button" pButton [iconOnly]="true" [rounded]="true" [text]="true" size="small" severity="secondary"
                          (click)="removeFromQueue(i)"><i class="pi pi-times"></i></button>
              }
            </div>
          }
        </div>
      }
      <app-dialog-footer
        saveLabel="common.action.upload"
        [isSubmitting]="uploading()"
        (cancelForm)="closeDialog()"
        (saveForm)="startUpload()"
      />
    </p-dialog>
    <p-confirmpopup />
  `,
})
export class CustomerAttachmentAudio implements OnDestroy {
  customerId = input.required<number>();

  private readonly api = inject(ApiService);
  private readonly cdr = inject(ChangeDetectorRef);
  private readonly confirm = inject(ConfirmationService);
  private readonly noti = inject(NotificationService);
  private readonly files = inject(AttachmentFileService);
  private readonly translate = inject(TranslateService);
  private readonly tz = inject(TimezoneService);
  // URL ký ngắn hạn phát thẳng (tua được, không blob). DestroyRef ⇒ tự huỷ request đang xin khi component huỷ.
  readonly media = new AttachmentMediaUrls(this.files, inject(DestroyRef));

  items: AttachmentItem[] = [];
  readonly loading = signal(false);
  readonly loadingMore = signal(false);
  readonly uploading = signal(false);
  readonly dragOver = signal(false);
  readonly fetchError = signal(false);
  readonly queue = signal<UploadEntry[]>([]);
  dialogVisible = false;
  readonly skeletons = [1, 2, 3];

  private page = 1;
  private lastChunk = 0;
  private sub?: Subscription;

  constructor() {
    effect(() => { const id = this.customerId(); if (id) this.reset(); });
  }

  ngOnDestroy() { this.sub?.unsubscribe(); }

  play(item: AttachmentItem) { this.media.load(item); }

  private reset() {
    this.sub?.unsubscribe();
    // Đổi khách: huỷ request URL đang xin + bỏ URL phát của khách cũ.
    this.media.reset();
    this.items = [];
    this.page = 1;
    this.lastChunk = 0;
    this.fetchError.set(false);
    this.fetch(true);
  }

  reload() { this.reset(); }

  private fetch(initial = false) {
    initial ? this.loading.set(true) : this.loadingMore.set(true);
    const params = new HttpParams()
      .set('page', String(this.page))
      .set('per-page', String(PAGE_SIZE))
      .set('file_type', 'audio');
    this.sub = this.api.get<any>(`crm/customer/${this.customerId()}/attachments`, { params }).subscribe({
      next: (res) => {
        const chunk: AttachmentItem[] = res?.items ?? [];
        this.lastChunk = chunk.length;
        if (initial) { this.items = chunk; this.loading.set(false); }
        else { this.items = [...this.items, ...chunk]; this.loadingMore.set(false); }
        this.cdr.markForCheck();
      },
      error: () => {
        if (initial) this.fetchError.set(true);
        this.loading.set(false); this.loadingMore.set(false); this.cdr.markForCheck();
      },
    });
  }

  onScroll(ev: Event) {
    const el = ev.target as HTMLElement;
    if (el.scrollTop + el.clientHeight >= el.scrollHeight - 120 && !this.loading() && !this.loadingMore() && this.hasMore()) {
      this.page++;
      this.fetch(false);
    }
  }

  hasMore() { return this.lastChunk === PAGE_SIZE; }

  readonly MAX_FILES = MAX_FILES;
  readonly MAX_MB = MAX_MB;
  readonly validCount = computed(() => this.queue().filter((e) => !e.error && e.progress === 0).length);

  openDialog() { this.dialogVisible = true; this.cdr.markForCheck(); }
  closeDialog() { this.dialogVisible = false; this.queue.set([]); this.cdr.markForCheck(); }

  onFilesSelected(e: Event) {
    this.addToQueue(Array.from((e.target as HTMLInputElement).files ?? []));
    (e.target as HTMLInputElement).value = '';
  }
  onDrop(e: DragEvent) {
    e.preventDefault(); this.dragOver.set(false);
    this.addToQueue(Array.from(e.dataTransfer?.files ?? []));
  }
  private addToQueue(files: File[]) {
    const currentValid = this.queue().filter((e) => !e.error && e.progress === 0).length;
    const remaining = MAX_FILES - currentValid;
    this.queue.update((q) => [...q, ...files.map((f, i) => ({
      file: f, name: f.name, size: f.size, progress: 0,
      error: i >= remaining ? this.translate.instant('crm.customer_attachment.max_files_error', { count: MAX_FILES })
           : f.size > MAX_BYTES ? this.translate.instant('crm.customer_attachment.size_exceeded', { size: MAX_MB })
           : undefined,
    }))]);
    this.cdr.markForCheck();
  }
  removeFromQueue(i: number) { this.queue.update((q) => q.filter((_, idx) => idx !== i)); }

  startUpload() {
    const valid = this.queue().filter((e) => !e.error && e.progress === 0);
    if (!valid.length) return;
    this.uploading.set(true);
    let done = 0;
    valid.forEach((entry) => {
      const fd = new FormData();
      fd.append('file', entry.file);
      if (branchActiveId()) fd.append('branch_id', String(branchActiveId()));
      this.updateEntry(entry.name, { progress: 10 });
      this.api.post<any>(`crm/customer/${this.customerId()}/attachments`, fd as any).subscribe({
        next: (res) => {
          const item = res?.data ?? res;
          this.updateEntry(entry.name, { progress: 100 });
          if (item?.id) this.items = [item, ...this.items];
          if (++done === valid.length) { this.uploading.set(false); setTimeout(() => this.closeDialog(), 800); }
          this.cdr.markForCheck();
        },
        error: (err: HttpErrorResponse) => {
          // 413: vượt trần dung lượng phía máy chủ — báo rõ thay vì "thất bại" chung chung.
          const error = err?.status === 413
            ? this.translate.instant('common.message.file_too_large')
            : this.translate.instant('common.message.upload_failed');
          this.updateEntry(entry.name, { progress: 0, error });
          if (++done === valid.length) this.uploading.set(false);
          this.cdr.markForCheck();
        },
      });
    });
  }
  private updateEntry(name: string, patch: Partial<UploadEntry>) {
    this.queue.update((q) => q.map((e) => e.name === name ? { ...e, ...patch } : e));
  }

  deleteItem(item: AttachmentItem, event: Event) {
    this.confirm.confirm({
      target: event.target as EventTarget,
      message: this.translate.instant('crm.customer_attachment.confirm_delete_audio'),
      icon: 'pi pi-exclamation-triangle',
      acceptButtonProps: {severity: 'danger', size: 'small'},
      accept: () => {
        this.api.delete(`crm/attachment/${item.id}`).subscribe({
          next: () => { this.media.forget(item.id); this.items = this.items.filter((i) => i.id !== item.id); this.noti.successMsg(this.translate.instant('common.deleted')); this.cdr.markForCheck(); },
          error: () => this.noti.errorMsg(this.translate.instant('common.message.delete_failed')),
        });
      },
    });
  }

  formatSize(b: number): string {
    if (b < 1024) return `${b} B`;
    if (b < 1048576) return `${(b / 1024).toFixed(1)} KB`;
    return `${(b / 1048576).toFixed(1)} MB`;
  }
  /** Ngày theo timezone + locale của thương hiệu (TimezoneService) — không dùng giờ máy trình duyệt. */
  formatDate(unix: number): string {
    if (!unix) return '';
    return this.tz.format(unix, { day: '2-digit', month: '2-digit', year: 'numeric' });
  }
}
