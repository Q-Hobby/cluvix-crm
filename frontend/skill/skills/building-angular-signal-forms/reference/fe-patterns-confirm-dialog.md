# AppConfirmDialog

## Mục lục

- KHÔNG dùng `ConfirmationService` cho action confirm mới
- Icon + iconBg theo loại action
- Ngoại lệ — GIỮ `ConfirmationService` với `target:`
- Import — KHÔNG thêm vào `imports[]`
- Gotcha — leftover `<p-confirmdialog>` gây hiện 2 dialog xác nhận

---


**File:** `src/app/share/components/app-confirm-dialog.ts`

Dùng thay thế `ConfirmationService` + `p-confirmDialog` cho mọi confirm action kiểu modal (KHÔNG phải popover). Mở qua `dialogWrapperService.open()` — sẵn trong `BaseApp`.

## KHÔNG dùng `ConfirmationService` cho action confirm mới

```typescript
// ❌ CŨ — KHÔNG dùng nữa
providers: [ConfirmationService],
private confirmService = inject(ConfirmationService);
// HTML: <p-confirmDialog ... />
this.confirmService.confirm({ message: '...', accept: () => { ... } });

// ✅ MỚI — AppConfirmDialog qua dialogWrapperService
const ref = this.dialogWrapperService.open(AppConfirmDialog, {
  data: {
    message: this.translate.instant('some.i18n.key'),
    icon: 'pi pi-check-circle',                        // optional, mặc định pi pi-exclamation-triangle
    iconBg: 'bg-...',                                  // optional, mặc định orange
    title: '...',                                      // optional header text
    acceptLabel: 'common.action.confirm',              // optional
    rejectLabel: 'common.action.cancel',               // optional
  },
  showHeader: false,
  styleClass: 'shadow-sm rounded-3xl',
  contentStyle: { padding: 0 },
  width: '40rem',
});
ref.onClose.pipe(takeUntilDestroyed(this.destroyRef)).subscribe((result: boolean) => {
  if (!result) return;
  // thực hiện action
});
```

## Icon + iconBg theo loại action

| Action | icon | iconBg |
|---|---|---|
| Mặc định | `pi pi-exclamation-triangle` | orange (mặc định) |
| Cancel/Hủy | `pi pi-times-circle` | `bg-red-100 dark:bg-red-400/30 text-red-500 dark:text-red-200 border-red-200 dark:border-red-400/20` |
| Delete/Xóa | `pi pi-trash` | red (cùng pattern cancel) |
| Approve/Duyệt | `pi pi-check-circle` | orange (mặc định) |
| Start/Bắt đầu | `pi pi-play` | orange (mặc định) |
| Complete/Hoàn tất | `pi pi-check` | orange (mặc định) |
| Ship/Xuất kho | `pi pi-send` | orange (mặc định) |
| Receive/Nhận | `pi pi-download` | orange (mặc định) |
| Lock/Đóng | `pi pi-lock` | orange (mặc định) |
| Convert/Chuyển | `pi pi-arrow-right-arrow-left` | orange (mặc định) |

## Ngoại lệ — GIỮ `ConfirmationService` với `target:`

`BaseList.deleteRecord()` và các popover confirm dùng `target: event.currentTarget` (hiện tại anchor gần button) — **GIỮ nguyên**, không migrate sang `AppConfirmDialog`.

```typescript
// ✅ GIỮ NGUYÊN — popover style (target = anchor)
this.confirmationService.confirm({
  target: event.currentTarget as EventTarget,
  message: '...',
  accept: () => { ... },
});
```

## Import — KHÔNG thêm vào `imports[]`

`AppConfirmDialog` là component được mở qua `dialogWrapperService.open()`, KHÔNG dùng trong template. Chỉ cần TypeScript import:

```typescript
import { AppConfirmDialog } from '@/share/components/app-confirm-dialog';
// KHÔNG thêm vào imports: [...] trong @Component
```

## Gotcha — leftover `<p-confirmdialog>` gây hiện 2 dialog xác nhận

`ConfirmationService.confirm()` là broadcast toàn cục — mọi `<p-confirmdialog>`/`<p-confirmpopup>` đang mounted đều phản ứng, không chỉ cái "gần nhất". Nếu 1 trang còn sót `<p-confirmdialog />` cũ (trước khi migrate sang AppConfirmDialog) bên cạnh `<p-confirmpopup>` global ở `app.component.ts`, bấm action trigger `confirmationService.confirm()` (vd `BaseForm.closeForm()`, `BaseList.deleteRecord()`) sẽ hiện **2 dialog cùng lúc**.
Fix: xóa `<p-confirmdialog>` local, chỉ dùng `<p-confirmpopup>` global.
