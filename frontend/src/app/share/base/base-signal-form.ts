import {
  computed,
  DestroyRef,
  Directive,
  effect,
  inject,
  Injector,
  Input,
  OnInit,
  signal,
  WritableSignal,
} from '@angular/core';
import { takeUntilDestroyed } from '@angular/core/rxjs-interop';
import { DynamicDialogConfig } from 'primeng/dynamicdialog';
import { catchError, finalize, forkJoin, map, of } from 'rxjs';
import { FieldTree, form, SchemaPathTree } from '@angular/forms/signals';
import { extractValue } from '@angular/forms/signals/compat';

import { BaseApp } from '@/share/base/BaseApp';
import { BaseApiService } from '@/share/services/base-api.service';
import { AppConfirmDialog } from '@/share/components/app-confirm-dialog';
import { UserCacheService } from '@/indexeddb/config/components/user';
import { EditorImageService } from '@/share/services/editor-image.service';

/**
 * Base cho form viết bằng Angular Signal Forms — sống độc lập cạnh `BaseForm` (Reactive Forms),
 * KHÔNG kế thừa chung 1 lớp cha. Xem `skill/bmad/signal-forms-poc/architecture.md`.
 *
 * Quyết định kiến trúc cốt lõi: `rootField` (FieldTree) dựng ĐÚNG 1 LẦN ở field initializer —
 * chạy trong lúc `super()` của subclass thực thi, TRƯỚC KHI field initializer của subclass chạy.
 * Hệ quả: `buildSchema()` chỉ được dùng import/hằng số module-level (vd `required`, `min`, các
 * SELECT constant) — KHÔNG được gọi `this.<injected service>` trực tiếp trong thân `buildSchema()`
 * (service subclass tự inject chưa sẵn sàng tại thời điểm này). Injected service CHỈ được dùng bên
 * trong closure `LogicFn`/`effect()` — những closure đó chạy sau, lúc field/effect thực sự evaluate,
 * không phải lúc đăng ký schema.
 *
 * Load lại data (edit mode, GET theo id) KHÔNG rebuild `form()` — chỉ `this.model.set(...)`.
 */
@Directive()
export abstract class BaseSignalForm<T extends { id?: number }> extends BaseApp implements OnInit {
  @Input() initialValue: Partial<T> = {};

  abstract modelService: BaseApiService<T>;

  readonly id = signal<number | null>(null);
  readonly model: WritableSignal<T> = signal<T>({} as T);
  readonly clone = signal<T>({} as T);
  readonly isEditMode = computed(() => !!this.model()?.id);
  readonly currentBranchId = signal<number | undefined>(undefined);
  protected submitted = signal(false);
  protected destroyRef = inject(DestroyRef);
  protected dialogConfig: DynamicDialogConfig | null = inject(DynamicDialogConfig, { optional: true });
  // `private` + tên riêng (KHÔNG phải `userCacheService` như `BaseForm`) — vài form con đã tự khai
  // `private userCacheService = inject(UserCacheService)` của riêng nó; đặt trùng tên ở base làm
  // TS2415/TS4114 cho chính các form đó. Helper `keepValid*Field` bên dưới dùng field này.
  private userCacheForScope = inject(UserCacheService);
  private injector = inject(Injector);
  protected editorImageService = inject(EditorImageService);

  /**
   * Form con implement — khai `required`/`min`/`max`/`applyEach`... lên `p`.
   * CHỈ chạy 1 lần lúc `form()` khởi tạo (bản chất `SchemaFn`) — KHÔNG được gọi lại.
   */
  protected abstract buildSchema(p: SchemaPathTree<T>): void;

  readonly rootField: FieldTree<T> = form(this.model, (p) => this.buildSchema(p), {
    injector: this.injector,
  });

  /**
   * `data.model` rỗng `{}` (vài list gọi `openForm({})` cho create-mode thay vì omit hẳn key `model`)
   * PHẢI coi như "không có model" — `{}` truthy nên check cũ để lọt, model kẹt `{}` vĩnh viễn, không
   * bao giờ được seed từ `getInitialObject()` (gotcha #1, phát hiện qua QA epic `config-signal-forms`
   * story-11 — `EmrExaminationCodeForm`/`EmrDmdcCatalogForm` vỡ hẳn nút "Tạo mới"). Áp dụng đồng bộ ở
   * cả đây (constructor) VÀ nhánh create-mode của `ngOnInit()` dưới — 2 chỗ đọc cùng nguồn dữ liệu.
   */
  constructor() {
    super();
    const idParam = this.dialogConfig?.data?.id ?? null;
    this.id.set(idParam);
    const modelParam = structuredClone(this.dialogConfig?.data?.model);
    if (modelParam && Object.keys(modelParam).length > 0) this.model.set(modelParam);
    const cloneParam = structuredClone(this.dialogConfig?.data?.clone);
    this.clone.set(cloneParam);
  }

  /**
   * Hook chạy ngay trước MỌI `model.set` trong `ngOnInit` base (3 nhánh). Mặc định no-op.
   * Form đã có `_applyModel`/`ngOnInit` riêng thì KHÔNG dùng hook này (tránh 2 nguồn).
   */
  protected prepareModel(obj: T): T {
    return obj;
  }

  /**
   * Mặc định: edit mode (có `id`, chưa có `model` từ dialog) → GET rồi `model.set()`; create mode
   * (không `id`, không `model` từ dialog) → seed từ `modelService.getInitialObject() + initialValue`.
   * Form con cần thêm logic bất đồng bộ trước khi seed default (vd load cache) → override `ngOnInit()`
   * HOÀN TOÀN (không gọi `super.ngOnInit()`), tự gọi `this.model.set(obj)` ở cuối — đúng pattern
   * `ServiceForm.ngOnInit()` đang làm với `_applyModel()`.
   *
   * `modelService.getInitialObject()` BẮT BUỘC trả object đủ key cho mọi field `buildSchema()` đụng
   * tới, giá trị rỗng dùng `null`/`[]` — KHÔNG được thiếu key hay `undefined` (Signal Forms coi field
   * giá trị `undefined` là "không tồn tại", `rootField.x` trả `undefined` chứ không phải FieldTree,
   * xem `skill/frontend/angular22.md` gotcha #1). Đường edit-mode GET dưới đây seed `model` bằng
   * `getInitialObject()` TRƯỚC khi có response — nếu không, khung hình render đầu tiên (OnPush vẫn
   * chạy CD ngay sau `ngOnInit()`) sẽ gặp `model` rỗng `{}`, mọi `rootField.x!()` trong template ném
   * TypeError vì field không tồn tại.
   */
  ngOnInit(): void {
    if (this.id()) {
      this.model.set(this.prepareModel({ ...this.modelService.getInitialObject() } as T));
      this.modelService
        .get(this.id()!, {})
        .pipe(takeUntilDestroyed(this.destroyRef))
        .subscribe((loaded: any) => {
          this.model.set(this.prepareModel({ ...this.modelService.getInitialObject(), ...loaded }));
        });
    } else if (!this.dialogConfig?.data?.model || Object.keys(this.dialogConfig.data.model).length === 0) {
      const obj = { ...this.modelService.getInitialObject(), ...this.initialValue } as T;
      this.model.set(this.prepareModel(obj));
    } else {
      // `data.model` có key (nhánh trên loại) NHƯNG có thể THIẾU key nếu BE dùng `omitempty` cho field
      // đang ở giá trị rỗng. Constructor không gọi được `getInitialObject()`
      // ở đây được (`modelService` là field subclass, chưa gán xong lúc `super()` chạy) nên phải backfill
      // ở `ngOnInit()`. Phát hiện qua QA epic `config-signal-forms` story-26 (`AiPromptForm`) — merge đè
      // giá trị model thật lên default, KHÔNG mất dữ liệu nếu model đã đủ key (an toàn cho mọi form khác).
      this.model.set(this.prepareModel({ ...this.modelService.getInitialObject(), ...this.model() }));
    }
  }

  onSubmit(): void {
    if (this.isSubmitting()) return;

    this.submitted.set(true);
    this.rootField().markAsTouched();
    if (this.rootField().invalid()) return;

    this.isSubmitting.set(true);

    this.submitWithRichTextImages(this.richTextImageFields(), () => this.submitAfterImageUpload());
  }

  /**
   * Field nào bind `p-editor` (Quill) cho phép dán ảnh → khai TÊN field ở đây. `onSubmit()` mặc định
   * sẽ upload ảnh base64 nhúng trong HTML lên S3 rồi mới gửi payload — cột DB (TEXT/MEDIUMTEXT) không
   * được phép nhận base64 nhúng thẳng (dễ vượt giới hạn cột hoặc phình payload vô ích). Mặc định rỗng
   * — form không có `p-editor` không tốn thêm 1 lượt xử lý nào.
   *
   * Form nào TỰ override `onSubmit()` hoàn toàn (upload file riêng, flow custom...) — KHÔNG khai field
   * ở đây (base `onSubmit()` không chạy) mà tự gọi `this.submitWithRichTextImages([...], () => {...})`
   * ngay đầu `onSubmit()` của mình (sau `isSubmitting.set(true)`), xem `NoteForm`/`FormAppointmentV2`
   * làm mẫu.
   */
  protected richTextImageFields(): (keyof T & string)[] {
    return [];
  }

  /**
   * Upload ảnh base64 nhúng trong các field HTML chỉ định lên S3 (GHI NGƯỢC url trả về vào chính field
   * đó — để `getPayload()`/`extractValue()` đọc lại đúng giá trị mới), rồi gọi `next()` nếu OK. Tự set
   * `isSubmitting=false` + toast lỗi và KHÔNG gọi `next()` khi: lỗi hạ tầng (network...) hoặc có ảnh bị
   * loại khỏi nội dung (upload lỗi/quá 10MB) — người gọi PHẢI đã `isSubmitting.set(true)` trước đó.
   */
  protected submitWithRichTextImages(fields: (keyof T & string)[], next: () => void): void {
    if (fields.length === 0) {
      next();
      return;
    }

    const tasks = fields.map((key) => {
      const field = (this.rootField as any)[key];
      const html = field ? (field().value() as string | null) : null;
      if (!html) return of({ key, html, failed: 0, total: 0 });
      return this.editorImageService
        .replaceBase64ImagesInHtml(html)
        .pipe(map((r) => ({ key, ...r })));
    });

    forkJoin(tasks)
      .pipe(
        map((results) => {
          results.forEach(({ key, html }) => {
            const field = (this.rootField as any)[key];
            if (field) field().value.set(html);
          });
          return {
            failed: results.reduce((sum, r) => sum + r.failed, 0),
            total: results.reduce((sum, r) => sum + r.total, 0),
          };
        }),
        catchError(() => of(null)),
      )
      .subscribe((result) => {
        if (!result) {
          this.isSubmitting.set(false);
          this.noti.errorMsg(this.translate.instant('common.message.image_process_failed'));
          return;
        }
        if (result.failed > 0) {
          this.isSubmitting.set(false);
          this.noti.error(
            this.translate.instant('common.message.image_upload_failed_title'),
            this.translate.instant('common.message.image_upload_failed_msg', {
              failed: result.failed,
              total: result.total,
            }),
          );
          return;
        }
        next();
      });
  }

  private submitAfterImageUpload(): void {
    let responseData: any = null;

    const handleSuccess = (res: any) => {
      responseData = res;
      this.noti.success(
        this.translate.instant('common.label.notification'),
        this.isEditMode()
          ? res.message || this.translate.instant('common.message.update_success')
          : this.translate.instant('common.message.create_success'),
      );
      if (res?.warning) this.noti.warnMsg(res.warning);
    };

    const handleFinalize = () => {
      this.isSubmitting.set(false);
      if (responseData) {
        this.dialogRef.close(this.isEditMode() ? responseData.data || responseData : responseData);
      }
    };

    const payload = this.getPayload();

    if (this.isEditMode() && !this.clone()) {
      this.modelService
        .update(this.model().id!, payload, {}, this.isToFormData())
        .pipe(finalize(handleFinalize))
        .subscribe({ next: handleSuccess });
    } else {
      this.modelService
        .create(payload, {}, this.isToFormData())
        .pipe(finalize(handleFinalize))
        .subscribe({ next: handleSuccess });
    }
  }

  closeForm(_event?: Event): void {
    if (this.rootField().dirty()) {
      const ref = this.dialogWrapperService.open(AppConfirmDialog, {
        data: { message: this.translate.instant('common.message.confirm_cancel') },
        showHeader: false,
        styleClass: 'shadow-sm rounded-3xl',
        contentStyle: { padding: 0 },
        width: '40rem',
      });
      ref.onClose.pipe(takeUntilDestroyed(this.destroyRef)).subscribe((result: boolean) => {
        if (!result) return;
        this.dialogRef.close(false);
      });
    } else {
      this.dialogRef.close(false);
    }
  }

  // Generic <V> thay vì `FieldTree<any>` — TS conditional type trong định nghĩa `FieldTree` distribute
  // trên `any` (`[any] extends [X] ? A : B` → `A | B`), khiến `FieldTree<any>` trở thành union type bị
  // lỗi khi nhận `FieldTree<number,...>`/`FieldTree<string,...>` thật (NG8002/TS2345 lúc `ng serve`,
  // "ng build" AOT cache không lộ ra lỗi này). Generic tránh hẳn vấn đề vì không có literal `any`.
  protected isFieldInvalid<V>(field: FieldTree<V>): boolean {
    return field().invalid();
  }

  protected getError<V>(field: FieldTree<V>): string | null {
    const state = field();
    if (!state.invalid()) return null;
    const err = state.errors()[0] as any;
    if (!err) return null;
    switch (err.kind) {
      case 'required':
        return this.translate.instant('common.validation.required');
      case 'email':
        return this.translate.instant('common.validation.email');
      case 'minLength':
        return this.translate.instant('common.validation.minlength', { min: err.minLength });
      case 'maxLength':
        return this.translate.instant('common.validation.maxlength', { max: err.maxLength });
      case 'pattern':
        return this.translate.instant('common.validation.pattern');
      default:
        return this.translate.instant('common.validation.invalid');
    }
  }

  /**
   * Override để transform payload trước khi submit — tương đương `BaseForm.getPayload()`.
   * `isToFormData()`: true → gửi FormData (upload file); mặc định gửi raw value.
   */
  protected getPayload(): any {
    return extractValue(this.rootField);
  }

  protected isToFormData(): boolean {
    return false;
  }

  /**
   * Bản `FieldTree` của `BaseForm.keepValidSingleUser()` — giữ lựa chọn cũ (bác sĩ/KTV single-select)
   * nếu người đó vẫn thuộc branch/department mới, chỉ clear khi không còn hợp lệ. Gọi từ trong
   * `effectOnChange(() => rootField.branch_id!().value(), ...)` và bọc `untracked()` khi ghi.
   *
   * Nhận `FieldTree` (không phải tên field) vì nơi gọi luôn nằm trong closure
   * effect — model đã seed xong, `rootField.x` chắc chắn tồn tại. Field nested (vd
   * `rootField.appointmentNext.appointmentDoctor`) chỉ truyền được theo đường này.
   */
  protected keepValidSingleUserField<V>(
    field: FieldTree<V> | undefined,
    branchId: number | null | undefined,
    department: number | undefined,
  ): void {
    if (!field) return;
    const state = field();
    const currentId = state.value() as number | null | undefined;
    if (!currentId) return;

    if (!this.isValidUserId(currentId, branchId, department)) {
      // `state.value` là WritableSignal có kiểu điều kiện (`V extends AbstractControl ? ... : never`)
      // nên không nhận thẳng `null as V` — cast qua `unknown` là cách duy nhất, giống `$any()` bên template.
      (state.value as unknown as WritableSignal<unknown>).set(null);
    }
  }

  /** Bản multiselect — giữ các id vẫn hợp lệ, chỉ lọc bỏ id không còn thuộc branch/department mới. */
  protected keepValidUserListField(
    field: FieldTree<number[]> | undefined,
    branchId: number | null | undefined,
    department: number | undefined,
  ): void {
    if (!field) return;
    const state = field();
    const current = state.value();
    if (!current || current.length === 0) return;

    const validIds = this.validUserIds(branchId, department);
    const filtered = current.filter((id) => validIds.has(id));
    if (filtered.length !== current.length) {
      state.value.set(filtered);
    }
  }

  private isValidUserId(
    userId: number,
    branchId: number | null | undefined,
    department: number | undefined,
  ): boolean {
    return this.validUserIds(branchId, department).has(userId);
  }

  private validUserIds(
    branchId: number | null | undefined,
    department: number | undefined,
  ): Set<number | undefined> {
    return new Set(
      this.userCacheForScope
        .filterUsers({
          department,
          branchId: branchId ?? undefined,
          activeOnly: !this.isEditMode(),
        })
        .map((u) => u.id),
    );
  }

  protected trackScopeFields(branchKey?: keyof T & string): void {
    if (branchKey) {
      effect(() => {
        const field = (this.rootField as any)[branchKey];
        this.currentBranchId.set(field ? (field().value() ?? undefined) : undefined);
      });
    }
  }
}
