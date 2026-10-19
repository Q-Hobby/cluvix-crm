import { ChangeDetectionStrategy, Component, EventEmitter, inject, Input, Output, ViewChild } from '@angular/core';
import { Editor } from 'primeng/editor';
import { finalize } from 'rxjs';
import { FormField, SchemaPathTree } from '@angular/forms/signals';

import { NoteModel } from '@/core/models/crm/note.model';
import { NoteService } from '@/core/services/crm/note.service';
import { BaseApiService } from '@/share/services/base-api.service';
import { BaseSignalForm } from '@/share/base/base-signal-form';
import { PERMISSION_SELECT } from '@/define';
import { FormShareImports } from '@/share/import/form-share-imports';

interface NoteFormModel {
  id?: number;
  /** bind `p-editor` (HTML) — KHÔNG phải `<input pInputText>` nên seed `null` bình thường (gotcha #9). */
  note: string | null;
}

/** Seed đủ mọi field sẽ bind, không field nào `undefined` (gotcha #1). */
function emptyNoteModel(): NoteFormModel {
  return { note: null };
}

/**
 * ⚠ Component NHÚNG (không phải dialog) — render qua `<app-note-form>` trong `app-note.html`, nhận dữ
 * liệu bằng `@Input` và trả kết quả bằng `@Output saved`, KHÔNG dùng `dialogRef.close()`.
 *
 * Vẫn `extends BaseSignalForm` (thay vì `form()` trực tiếp như `skill/frontend/signal-forms-embedded.md`)
 * vì form này CÓ `modelService` và dùng `isSubmitting`/`noti`/`translate` của base — kế thừa là thay đổi
 * nhỏ nhất và đồng nhất với 21 form còn lại của epic.
 *
 * An toàn về DI: `BaseApp` inject `DynamicDialogRef` KHÔNG `optional`, nhưng **`src/app.config.ts` provide
 * `DynamicDialogRef` ở ROOT** (dòng ~144) → component nhúng nhận một instance "rỗng" ở root injector, còn
 * dialog thật nhận instance thật qua `DynamicDialogInjector` của PrimeNG. Form này không có nút Huỷ nên
 * `closeForm()` không bao giờ chạy; `BaseSignalForm` cũng không có `ngOnDestroy` đụng `dialogRef`.
 */
@Component({
  changeDetection: ChangeDetectionStrategy.OnPush,
  selector: 'app-note-form',
  templateUrl: './note-form.html',
  standalone: true,
  imports: [...FormShareImports, FormField],
})
export class NoteForm extends BaseSignalForm<NoteFormModel> {
  @Input() customer_id: number | null = null;
  @Output() saved: EventEmitter<boolean | NoteModel> = new EventEmitter<boolean | NoteModel>();
  @ViewChild('myEditor') myEditorComp!: Editor;
  modelService = inject(NoteService) as unknown as BaseApiService<NoteFormModel> &
    Pick<NoteService, 'createNote'>;
  formats = ['header', 'bold', 'italic', 'underline', 'link', 'image'];
  protected readonly permission = PERMISSION_SELECT;

  constructor() {
    super();
  }

  /** Override HOÀN TOÀN — bản cũ `buildForm()` dựng group 1 control `note` khởi tạo `null`, không đọc
   * model nào (component nhúng, luôn là "tạo mới"). Giữ nguyên. */
  override ngOnInit(): void {
    this.model.set(emptyNoteModel());
  }

  override onSubmit(): void {
    if (this.isSubmitting()) return;
    this.submitted.set(true);
    this.rootField().markAsTouched();
    if (this.rootField().invalid()) return;

    this.isSubmitting.set(true);

    const rawContent = this.rootField.note!().value() || '';

    this.editorImageService
      .processNoteHtml(rawContent)
      .pipe(finalize(() => this.isSubmitting.set(false)))
      .subscribe({
        next: ({ html, failed, total }) => {
          // 1) Nếu có ảnh lỗi → dừng và báo
          if (failed > 0) {
            this.noti.error(
              this.translate.instant('crm.note_form.upload_failed_title'),
              this.translate.instant('crm.note_form.upload_failed_msg', { failed, total }),
            );
            return;
          }
          // 2) Nếu (hi hữu) còn base64 → dừng
          if (/src="data:image\//.test(html)) {
            this.noti.error(
              this.translate.instant('crm.note_form.image_unprocessed_title'),
              this.translate.instant('crm.note_form.image_unprocessed_msg'),
            );
            return;
          }
          // 3) Nếu nội dung rỗng sau xử lý → dừng
          const cleaned = (html || '').trim();
          if (!cleaned) {
            this.noti.warn(
              this.translate.instant('crm.note_form.missing_content_title'),
              this.translate.instant('crm.note_form.missing_content_msg'),
            );
            return;
          }

          const createParam = {
            ...this.getPayload(),
            note: cleaned,
            customer_id: this.customer_id,
          };

          this.modelService.createNote(createParam).subscribe({
            next: () => {
              this.noti.successMsg(this.translate.instant('crm.note_form.create_success'));
              this.saved.emit(true);
              // Bản cũ `rfModel.reset()` → đưa control `note` về `null`. Signal Forms: seed lại model.
              this.model.set(emptyNoteModel());
            },
            // error đã có interceptor của bạn xử lý
          });
        },
        error: () => {
          // processNoteHtml văng lỗi “toàn cục” -> DỪNG hẳn
          this.noti.errorMsg(this.translate.instant('crm.note_form.process_failed'));
        },
      });
  }

  /** Bản cũ gửi `{...rfModel.value, note: cleaned, customer_id}` — `rfModel.value` chỉ có
   * đúng 1 key `note` (rồi bị `cleaned` ghi đè ngay sau). Giữ nguyên tập key gửi BE. */
  protected override getPayload(): any {
    const raw = super.getPayload() as any;
    return { note: raw.note };
  }

  onEditorInit(e: any) {
    const quill = e.editor;
    const tb = quill.getModule('toolbar');
    tb.addHandler('image', () => this.pickImages(quill)); // <-- override thành multiple
  }

  /** Bản gốc `buildForm()` KHÔNG khai validator nào — giữ nguyên (nút Lưu tự disable khi `note` rỗng,
   * và BE chặn bằng `response.RequireFields`). */
  protected override buildSchema(_p: SchemaPathTree<NoteFormModel>): void {
    // cố ý để trống — xem ghi chú trên
  }

  private async pickImages(quill: any) {
    const input = document.createElement('input');
    input.type = 'file';
    input.accept = 'image/*';
    input.multiple = true;

    input.onchange = async () => {
      const files = Array.from(input.files ?? []);
      if (!files.length) return;

      const range = quill.getSelection(true) ?? { index: quill.getLength(), length: 0 };
      let insertIndex = range.index;

      for (const file of files) {
        const dataUrl = await this.fileToDataUrl(file);

        // 1) chèn ảnh
        quill.insertEmbed(insertIndex, 'image', dataUrl, 'user');

        // 2) lấy ra DOM node của ảnh vừa chèn và set style
        const [leaf] = quill.getLeaf(insertIndex);
        const imgEl = (leaf?.domNode as HTMLImageElement) ?? null;
        if (imgEl && imgEl.tagName === 'IMG') {
          imgEl.style.width = '70px';
          imgEl.style.maxWidth = '100%';
          imgEl.style.height = 'auto';
          imgEl.setAttribute('data-normalized', '1'); // đánh dấu đã xử lý (phòng lặp)
        }

        insertIndex += 1;
        quill.insertText(insertIndex, '\n', 'user'); // tách ảnh
        insertIndex += 1;
      }

      quill.setSelection(insertIndex, 0);
    };

    input.click();
  }

  private fileToDataUrl(file: File): Promise<string> {
    return new Promise((resolve, reject) => {
      const r = new FileReader();
      r.onload = () => resolve(String(r.result));
      r.onerror = reject;
      r.readAsDataURL(file);
    });
  }
}
