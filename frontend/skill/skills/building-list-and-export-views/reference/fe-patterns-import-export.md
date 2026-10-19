# Import/Export Excel

## Mục lục

- 1. Export — từ List component
  - BE trước
  - Logic id trong file Excel
  - ExportXxx trong `internal/modules/webapp/export/`
  - FE — service method
  - FE — trong List component
- 2. Import — XxxImportDialog
  - File mẫu (template)
  - Service methods
  - Dialog structure
  - Template HTML — file picker
  - Template HTML — summary sau import
  - TS class
  - Mở dialog từ List component
- 3. i18n keys
- 4. Complex import — dialog lỗi nhóm theo loại (khi BE trả nhiều lỗi 1 lần)
- 5. Checklist khi làm mới

---


**Use case:** List component có nút Export (tải dữ liệu ra file) và Import (upload file Excel để tạo/update hàng loạt).

Ví dụ chuẩn: `MaterialList` (export) + `MaterialImportDialog` (import), `LeadImportDialog` (import phức tạp có defaults form).

---

## 1. Export — từ List component

### BE trước

`ListXxx` handler **phải** check `_export=1` trước khi paginate, build query rồi gọi `exportHandlers.ExportXxx(c, q)`.
→ BE pattern đầy đủ: Xem `(repo cha) ../skill/skills/working-on-ops/reference/export.md` (§Cơ chế + §Pattern chuẩn).

**Gotcha:** ORDER BY / WHERE dùng table alias (`m.name`, `m.status`) vì `ExportXxx` sẽ gọi `q.Table("material m")` bên trong — nếu dùng bare column thì ambiguous error.

**Gotcha — export nhầm list:** FE service method + BE `ListXxx` dispatch `_export` phải CÙNG 1 list. Tái dùng `exportYyy()` của list khác cùng model (vd ticket-list gọi `exportLeadToExcel`) → xuất nhầm scope, vẫn compile. Symptom: file export giống hệt list kia. List mới quên `if c.Query("_export")=="1"` trong `ListXxx` → nút export ăn nhờ endpoint khác.

### Logic id trong file Excel

| Col `id` | Tìm thấy trong DB | Kết quả |
|----------|-------------------|---------|
| Trống | — | **Tạo mới** |
| Có giá trị | Có | **Update** bản ghi đó |
| Có giá trị | Không | **Tạo mới** (id bị bỏ qua) |

→ Khi export về rồi muốn thêm mới: thêm dòng, **để trống cột id**.

---

### ExportXxx trong `internal/modules/webapp/export/`

- Response: string URL trực tiếp, không wrap `{ data: url }`
- Column order phải **khớp hoàn toàn** với file mẫu import (thứ tự, kiểu dữ liệu)
- Header row phải giống header trong `mau_import_xxx.xlsx`
- Code mẫu `ExportXxx`: → Xem `(repo cha) ../skill/skills/working-on-ops/reference/export.md` §Pattern chuẩn

**Gotcha về tên cột:**
- Cột là reserved word MySQL (`desc`) → phải backtick: `` m.`desc` ``
- Cột lookup-by-name (Loại NVL, Đơn vị tính) → export tên chuỗi, import lookup ngược theo tên — phải khớp case
- JSON array field (`companyId`) → **không JOIN trực tiếp** `comp.id = m.companyId`; bỏ qua hoặc dùng subquery riêng

### FE — service method

```typescript
// trong XxxService extends BaseApiService
exportXxx(params: any = {}): Observable<string> {
    return this.apiService.get(`${this.path}?_export=1`, params);
}
```

`apiService.get` trả raw HTTP body — response là string URL đã được JSON-parse. **Không cần** `.pipe(map(res => res?.data))`.

### FE — trong List component

```typescript
exportMaterial(event: Event) {
    if (!this.permissionsService.getPermission(PERMISSION_SELECT.permissionManageConfig)) { ... return; }
    this.isSubmitting.set(true);
    this.confirmationService.confirm({
        target: event.target ?? undefined,
        accept: () => {
            this.loadingService.showLoading('exportMaterial');
            this.modelService.exportMaterial({ params: { ...this.searchParams() } })
                .pipe(finalize(() => { this.isSubmitting.set(false); this.loadingService.hideLoading('exportMaterial'); }))
                .subscribe({
                    next: (res: any) => {
                        this.noti.successMsg(this.translate.instant('common.message.export_success'));
                        window.location.href = res;   // res = string URL trực tiếp
                    },
                });
        },
        reject: () => { this.isSubmitting.set(false); },
    });
}
```

---

## 2. Import — XxxImportDialog

### File mẫu (template)

- Đặt tại `frontend/src/assets/download/mau_import_xxx.xlsx`
- Sinh bằng script Go: `backend/scripts/gen_xxx_template/main.go`
- Chạy: `go run scripts/gen_xxx_template/main.go` từ `backend/`
- **Column order phải giống hệt BE ImportXxxFromExcel** (index 0, 1, 2, ...)

### Service methods

```typescript
downloadImportTemplate() {
    const a = document.createElement('a');
    a.href = '/assets/download/mau_import_xxx.xlsx';
    a.download = 'mau_import_xxx.xlsx';
    a.click();
}

importXxxFromExcel(file: File): Observable<any> {
    const formData = new FormData();
    formData.append('file', file, file.name);
    return this.apiService.post(`${this.path}/import-xxx-from-excel`, formData);
}
```

Nếu import cần truyền defaults (như LeadImportDialog):

```typescript
importXxxFromExcel(file: File, defaults: Record<string, any> = {}) {
    const formData = new FormData();
    formData.append('file', file, file.name);
    Object.entries(defaults).forEach(([key, val]) => {
        if (val !== null && val !== undefined) formData.append(key, String(val));
    });
    return this.apiService.post(`${this.path}/import-xxx-from-excel`, formData);
}
```

### Dialog structure

**Simple** (không có defaults form — như `MaterialImportDialog`):

```
Step 1: Tải file mẫu (p-button download)
Step 2: Chọn file + nút upload (native <input type="file" class="hidden">)
→ sau submit: hiện summary { created, updated, skipped }
→ close(true) nếu đã import, close(false) nếu cancel
```

**Complex** (có defaults form — như `LeadImportDialog`):

```
Step 1: Fill defaults form (company, branch, source, v.v.)
Divider
Step 2: Tải file mẫu + Chọn file
→ sau submit: hiện summary
```

### Template HTML — file picker

```html
<input #fileInput type="file" accept=".xlsx,.xls,.csv" class="hidden" (change)="onFileChange($event)" />

<div class="p-4 border border-dashed border-gray-300 rounded-lg bg-gray-50 flex items-center justify-between gap-3">
  <div class="flex items-center gap-3 min-w-0">
    <i class="pi pi-file-excel shrink-0 text-xl" [class]="selectedFile() ? 'text-green-600' : 'text-[#7B3F9E]'"></i>
    @if (selectedFile()) {
      <span class="text-sm font-medium text-gray-800 truncate">{{ selectedFile()!.name }}</span>
      <i class="pi pi-times shrink-0 text-gray-400 cursor-pointer hover:text-red-500"
         (click)="selectedFile.set(null); fileInput.value = ''"></i>
    } @else {
      <span class="text-sm text-gray-400">{{ 'xxx.import_no_file' | translate }}</span>
    }
  </div>
  <p-button icon="pi pi-upload"
    [label]="(selectedFile() ? 'xxx.import_change_file' : 'common.label.choose_file') | translate"
    severity="secondary" [outlined]="true" size="small"
    (click)="fileInput.click()" />
</div>
```

### Template HTML — summary sau import

```html
@if (importResult()) {
  <div class="flex flex-col gap-4 p-6">
    <div class="flex items-center gap-3">
      <i class="pi pi-check-circle text-3xl text-green-500"></i>
      <span class="text-lg font-semibold">{{ 'xxx.import_done_title' | translate }}</span>
    </div>
    <div class="grid grid-cols-3 gap-3">
      <div class="flex flex-col items-center gap-1 p-4 bg-green-50 border border-green-200 rounded-xl">
        <span class="text-2xl font-bold text-green-600">{{ importResult()!.created }}</span>
        <span class="text-xs text-green-700">{{ 'xxx.import_created' | translate }}</span>
      </div>
      <div class="flex flex-col items-center gap-1 p-4 bg-blue-50 border border-blue-200 rounded-xl">
        <span class="text-2xl font-bold text-blue-600">{{ importResult()!.updated }}</span>
        <span class="text-xs text-blue-700">{{ 'common.label.updated' | translate }}</span>
      </div>
      <div class="flex flex-col items-center gap-1 p-4 bg-gray-50 border border-gray-200 rounded-xl">
        <span class="text-2xl font-bold text-gray-500">{{ importResult()!.skipped }}</span>
        <span class="text-xs text-gray-500">{{ 'xxx.import_skipped' | translate }}</span>
      </div>
    </div>
  </div>
  <app-dialog-footer [showSave]="false" [cancelLabel]="'common.label.close'" (cancelForm)="cancel()" />
}
```

### TS class

```typescript
export class XxxImportDialog extends BaseApp {
    private modelService = inject(XxxService);
    private dialogConfig = inject(DynamicDialogConfig, { optional: true });
    protected readonly selectedFile = signal<File | null>(null);
    protected readonly importResult = signal<{created: number; updated: number; skipped: number} | null>(null);

    onFileChange(event: Event) {
        const input = event.target as HTMLInputElement;
        this.selectedFile.set(input.files?.[0] ?? null);
    }

    submit() {
        const file = this.selectedFile();
        if (!file) { this.noti.warnMsg(...); return; }
        this.isSubmitting.set(true);
        this.loadingService.showLoading('xxxImport');
        this.modelService.importXxxFromExcel(file)
            .pipe(finalize(() => { this.isSubmitting.set(false); this.loadingService.hideLoading('xxxImport'); }))
            .subscribe({ next: (res: any) => { this.importResult.set(res?.data ?? { created: 0, updated: 0, skipped: 0 }); } });
    }

    downloadTemplate() { this.modelService.downloadImportTemplate(); }

    cancel() { this.dialogRef.close(!!this.importResult()); }  // true nếu đã import
}
```

### Mở dialog từ List component

```typescript
openImportDialog() {
    this.dialogWrapperService.open(XxxImportDialog, {
        width: this.screenService.getDialogWidthNormal(),
        header: this.translate.instant('xxx.import_dialog_title'),
        data: { onImported: () => this.getList() },  // reload ngay khi import xong
    });
}
```

---

## 3. i18n keys

**Chỉ thêm 2 key mới** (module-specific):
```
xxx.import_dialog_title      (tiêu đề dialog)
xxx.import_step2_title       (Bước 2: Chọn file để import)
```

**Dùng lại key đã có** (không tạo trùng):
| Vị trí trong dialog | Key dùng |
|----------------------|----------|
| Badge Step 1 label | `common.label.download_template` |
| Placeholder chưa chọn file | `sale.lead.import_no_file` |
| Nút đổi file | `sale.lead.import_change_file` |
| Tiêu đề summary | `sale.lead.import_done_title` |
| Số bản ghi tạo mới | `sale.lead.import_created` |
| Số bản ghi bỏ qua | `sale.lead.import_skipped` |
| Số bản ghi cập nhật | `common.label.updated` |

---

## 4. Complex import — dialog lỗi nhóm theo loại (khi BE trả nhiều lỗi 1 lần)

Dùng khi BE là dạng bulk-import với FK-by-name + accumulate-all-errors (xem
`(repo cha) ../skill/skills/debugging-backend-queries/reference/patterns-bulk-import.md`) — trả 422 `{ message, details: ImportErrorGroup[] }`
thay vì 1 message đơn. Chuẩn: `StockEntryImportDialog` + `ImportErrorDetailDialog`.

**Service method** — set context để interceptor không tự toast (component tự xử lý):

```typescript
import { HttpContext } from '@angular/common/http';
import { SKIP_ERROR_NOTIFICATION } from '@/core/interceptors/response-handler.interceptor';

importXxxFromExcel(file: File): Observable<any> {
    const formData = new FormData();
    formData.append('file', file, file.name);
    return this.apiService.post(`${this.path}/import-xxx`, formData, {
        context: new HttpContext().set(SKIP_ERROR_NOTIFICATION, true),
    });
}
```

**Component** — `error:` callback tự mở dialog nếu có `details`, fallback toast nếu không:

```typescript
import { ImportErrorDetailDialog } from '@/share/components/import-error-detail-dialog';

.subscribe({
    next: (res: any) => { this.noti.successRes(res); this.dialogRef.close(true); },
    error: (err: any) => {
        const payload = err?.error;
        if (payload && Array.isArray(payload.details) && payload.details.length > 0) {
            this.dialogWrapperService.open(ImportErrorDetailDialog, {
                header: this.translate.instant('xxx.import_error_title'),
                width: '600px',
                data: { message: payload.message, details: payload.details },
            });
        } else {
            this.noti.errorMsg(payload?.message ?? this.translate.instant('common.message.error'));
        }
    },
});
```

`ImportErrorDetailDialog` (`share/components/import-error-detail-dialog.ts`) là dialog dùng chung — KHÔNG
viết dialog riêng cho từng module, chỉ truyền `{ message, details }` qua `data`.

---

## 5. Checklist khi làm mới

- [ ] BE: `ListXxx` check `_export=1` trước paginate, pass `q` với alias đúng
- [ ] BE: `ExportXxx` column order = import template order; headers giống hệt template
- [ ] BE: `ImportXxxFromExcel` — comment rõ `// Cột: 0:id, 1:..., 2:...*`
- [ ] Template file `mau_import_xxx.xlsx` — gen bằng script, đặt vào `assets/download/`
- [ ] Service: `exportXxx`, `downloadImportTemplate`, `importXxxFromExcel`
- [ ] Dialog: `XxxImportDialog` — step UI + file picker + summary
- [ ] List: `openImportDialog()` + `exportXxx()` trong component
- [ ] i18n: thêm đủ 4 locale
- [ ] Verify: `go build ./...` + `npx tsc --noEmit`
