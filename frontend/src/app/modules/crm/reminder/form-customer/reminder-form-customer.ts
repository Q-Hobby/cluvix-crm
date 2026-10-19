import { ChangeDetectionStrategy, Component, inject } from '@angular/core';
import { finalize } from 'rxjs';
import { FormField, required, validate, requiredError, SchemaPathTree } from '@angular/forms/signals';

import { ReminderModel } from '@/core/models/crm/reminder.model';
import { userId } from '@/core/services/auth';
import { ReminderService } from '@/core/services/crm/reminder.service';
import { BaseSignalForm } from '@/share/base/base-signal-form';
import { REMINDER_TYPE_OPTIONS, REMINDER_STATUS_SELECT } from '@/define';
import { FormShareImports } from '@/share/import/form-share-imports';
import { translateOptions } from '@/share/utils/translate-options.util';

/** ĐÚNG 8 control của `buildForm()` bản Reactive Forms cũ — whitelist BẮT BUỘC, không phải cho gọn.
 * BE `UpdateReminder` (`crm/reminder/handler/upsert.go:116`) bind vào `map[string]interface{}` rồi
 * `.Table("reminder").Updates(body)` → **GORM lấy KEY của map làm TÊN CỘT**. Lọt bất kỳ key quan hệ nào
 * của `ReminderModel` (`customer`, `userCreated`...) là `Error 1054 Unknown column` → hỏng hẳn thao tác
 * lưu. `extractValue()` mặc định duyệt TOÀN BỘ key trên `model()` nên KHÔNG được dùng trần. */
const PAYLOAD_FIELDS = [
  'customer_id',
  'status',
  'type',
  'note',
  'start_time',
  'relatedUser',
  'updated_at',
] as const;

/** Bản parse THUẦN của `BaseForm.handleSelectedDate()` + `formatDate()` — 2 hàm này CHỈ có ở `BaseForm`,
 * KHÔNG có ở `BaseApp` (base thật của `BaseSignalForm`), và bản gốc còn tự `patchValue()` bên trong bằng
 * cách đọc attribute DOM `formControlName` — cơ chế không tồn tại ở Signal Forms (gotcha #22).
 * Giữ nguyên logic gốc: chỉ xử lý khi Enter; chuỗi không có `/` thì hiểu là `ddmmyyyy`.
 * Trả `Date` khi hợp lệ, `null` khi không (bản gốc trả `''`/`undefined` — cùng là falsy, nơi gọi chỉ
 * kiểm truthy nên hành vi không đổi). */
function parseTypedDate(event: KeyboardEvent, typed: unknown): Date | null {
  if ((event as unknown as { keyCode?: number }).keyCode !== 13) return null;
  let s = typeof typed === 'string' ? typed : '';
  if (!s) return null;
  if (!s.includes('/')) s = `${s.slice(0, 2)}/${s.slice(2, 4)}/${s.slice(4, 8)}`;
  const [day, month, year] = s.split('/');
  if (year && +month <= 12 && +day <= 31) return new Date(+year, +month - 1, +day);
  return null;
}

interface ReminderFormModel {
  id?: number;
  customer_id: number | null;
  status: number | null;
  type: number | null;
  note: string | null;
  start_time: Date | number | null;
  relatedUser: number[];
  updated_at: Date | number | null;
}

/** Seed đủ mọi field sẽ bind, không field nào `undefined` (gotcha #1).
 * `note` bind `<textarea pTextarea>` → seed `null` bình thường (gotcha #9 CHỈ áp cho
 * `<input pInputText type="text">`, `TEXTAREA` khác nhánh code trong `getNativeControlValue()`). */
function emptyReminderModel(): ReminderFormModel {
  return {
    customer_id: null,
    status: REMINDER_STATUS_SELECT.STATUS_PLANED,
    type: null,
    note: null,
    start_time: null,
    relatedUser: [],
    updated_at: null,
  };
}

@Component({
  changeDetection: ChangeDetectionStrategy.OnPush,
  selector: 'app-reminder-form-customer',
  templateUrl: './reminder-form-customer.html',
  styles: [],
  standalone: true,
  imports: [...FormShareImports, FormField],
})
export class ReminderFormCustomer extends BaseSignalForm<ReminderFormModel> {
  modelService = inject(ReminderService) as unknown as import('@/share/services/base-api.service').BaseApiService<ReminderFormModel> &
    Pick<ReminderService, 'createReminder' | 'updateReminder'>;
  minDate = this.timezoneService.todayStart();
  selectedDate!: any;
  protected readonly reminderTypeOptions = translateOptions(REMINDER_TYPE_OPTIONS);

  /** Override HOÀN TOÀN (không gọi `super.ngOnInit()`) — giữ nguyên 2 nhánh của bản cũ:
   * có `model` từ dialog → dùng thẳng; không có → seed từ `getInitialObject()` + `initialValue`.
   * Bản cũ KHÔNG gọi `GET(id)` ở nhánh nào (list truyền đủ `model`) → không thêm mới. */
  override ngOnInit(): void {
    const dialogModel = this.model();
    if (dialogModel && Object.keys(dialogModel).length > 0) {
      this._applyModel(dialogModel);
    } else {
      const { customer_id } = this.dialogConfig?.data ?? {};
      this.initialValue = {
        ...this.initialValue,
        customer_id,
        // `userId()` khai `number | undefined`; bản cũ nhét thẳng vào mảng (rfModel không có kiểu nên
        // lọt). Lọc `null/undefined` để không seed `[undefined]` — vừa đúng kiểu, vừa tránh gửi BE phần
        // tử rác trong `relatedUser` (BE `JSON_CONTAINS` so khớp mảng này để phân quyền xem nhắc nhở).
        relatedUser: [userId()].filter((v): v is number => v != null),
      };
      this._applyModel({
        ...this.modelService.getInitialObject(),
        ...this.initialValue,
      } as ReminderFormModel);
    }
  }

  /** Merge template đủ key trước khi `model.set()` (gotcha #1) — form override `ngOnInit()` hoàn toàn nên
   * KHÔNG được 2 lớp bảo vệ tại gốc của `BaseSignalForm` che (gotcha #13/#14), phải tự đảm bảo. */
  private _applyModel(model: Partial<ReminderFormModel>): void {
    this.model.set({
      ...emptyReminderModel(),
      ...model,
      // BE có thể trả `relatedUser` null; `p-multiselect` cần mảng thật, và `[]` giữ field bind được.
      relatedUser: (model.relatedUser ?? []) as number[],
    } as ReminderFormModel);
  }

  /** Xử lý format data date khi nhập tay vào calendar */
  inputDate(event: any) {
    this.selectedDate = event.target.value;
  }

  /** Khi nhấn Enter thì ngày nhập tay được fill vào datepicker */
  handleKeyPress(event: KeyboardEvent) {
    const parsed = parseTypedDate(event, this.selectedDate);
    if (!parsed) {
      this.noti.warnMsg(this.translate.instant('crm.reminder.noti_invalid_time'));
      return;
    }
    this.rootField.start_time!().value.set(parsed);
  }

  override onSubmit() {
    if (this.isSubmitting()) return;
    this.submitted.set(true);
    this.rootField().markAsTouched();

    if (this.rootField().invalid()) return;

    this.isSubmitting.set(true);
    const payload = this.getPayload();
    if (this.isEditMode()) {
      this.modelService
        .updateReminder(this.model().id!, payload, {})
        .pipe(finalize(() => this.isSubmitting.set(false)))
        .subscribe({
          next: (res: any) => {
            this.noti.successRes(res);
            this.dialogRef.close(res.data);
          },
        });
    } else {
      this.modelService
        .createReminder(payload, {})
        .pipe(finalize(() => this.isSubmitting.set(false)))
        .subscribe({
          next: (res: any) => {
            this.noti.successMsg(this.translate.instant('common.message.create_success'));
            this.dialogRef.close(res);
          },
        });
    }
  }

  protected override getPayload(): any {
    const raw = super.getPayload() as any;
    const payload: any = {};
    for (const key of PAYLOAD_FIELDS) payload[key] = raw[key];
    return payload;
  }

  protected override buildSchema(p: SchemaPathTree<ReminderFormModel>): void {
    required(p.type!);
    required(p.note!);
    required(p.start_time!);
    // `relatedUser` là MẢNG: `required()` của Signal Forms dùng `isEmpty()` nội bộ và KHÔNG coi `[]` là
    // rỗng (khác `Validators.required` gốc) → dùng thẳng sẽ ÂM THẦM cho submit dù chưa chọn ai (gotcha #10).
    validate(p.relatedUser!, ({ value }) => ((value()?.length ?? 0) === 0 ? requiredError() : null));
  }
}
