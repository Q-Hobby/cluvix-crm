# Module Config — Option CRM

## Mục lục

- Rule CRITICAL — option seed mới PHẢI match theo `name`, KHÔNG hardcode id
- Checklist thêm option mới

---


Màn `modules/config/config-system/option-crm/` render các row bảng BE `user_option`. **KHÔNG generic-render**: mỗi card khai báo tay, resolve theo `option.id` qua `switch(option.id)` trong `ngOnInit`.

## Rule CRITICAL — option seed mới PHẢI match theo `name`, KHÔNG hardcode id

- Seed BE chuẩn là `INSERT IGNORE ... (name, ...)` (idempotent, không id cố định) → **`id` KHÁC NHAU giữa các DB khách** (DB-per-customer).
- Hardcode/đoán id trong `CONFIG_OPTION_CRM_SELECT` từng gây collision thật (`OPTION_OMNICHAT_STALE_HOURS`).
- Pattern đúng (đã áp dụng `auto_ot_payroll`): match theo name NGAY TRƯỚC `switch(option.id)`, lưu id thật runtime để gọi update:

```typescript
// trong loop ngOnInit xử lý GetOptions()
if (option.name === 'auto_ot_payroll') {
    this.autoOtPayrollOptionId = option.id;          // id thật của DB này
    this.selectedAutoOtPayroll = option.options;
}
// switch(option.id) giữ nguyên cho các option cũ id cố định

// update: dùng id đã resolve
this.optionUpdate(this.autoOtPayrollOptionId!, value);
```

- Card bọc `@if (xxxOptionId !== null)` — ẩn an toàn khi môi trường chưa chạy migration seed.

## Checklist thêm option mới

1. BE: seed `INSERT IGNORE` theo `name` unique + const `define.UserOptionNameXxx` (repo cha `backend/pkg/define/`); BE đọc `Where("name = ?")`.
2. FE: field `xxxOptionId: number | null` + name-match như trên; KHÔNG thêm id vào `CONFIG_OPTION_CRM_SELECT`.
3. i18n 4 locale: `config.option_crm.xxx_title` / `_tooltip` / value labels.
4. Options cũ đang dùng id cố định (`switch`) — giữ nguyên, không migrate ngược.
