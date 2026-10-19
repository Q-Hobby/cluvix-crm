# Breadcrumb — cơ chế & gotcha

## Mục lục
1. Thành phần
2. Thuật toán dựng chuỗi
3. Nhãn động (tên bản ghi)
4. Gotcha
5. Quy ước bắt buộc (C1–C6)
6. Checklist thêm route mới
7. Gate `breadcrumb-audit`

---

## 1. Thành phần

| File | Vai trò |
|---|---|
| `src/app/layout/components/app.breadcrumb.ts` | Component `[app-breadcrumb]`, mount trong mọi `*.layout.ts` có sidebar (AppLayout/Emr/Care/Loyalty/Inventory/Sale/Config/Workspace/Accountant/Marketing…). Kiêm luôn cụm nút phải (gọi điện, SMS, đổi tiền tệ). |
| `src/app/share/services/breadcrumb-label.service.ts` | `BreadcrumbLabelService.set(name)` — màn detail thay nhãn CUỐI bằng tên bản ghi. |
| `src/app/share/utils/setup-route-meta.ts` | Đặt `document.title` từ `data.title` của route SÂU NHẤT — độc lập với breadcrumb. |

Nguồn dữ liệu DUY NHẤT là `route.data` trong các file `*.routes.ts`. KHÔNG có API set cả chuỗi từ component.

## 2. Thuật toán dựng chuỗi

Chạy lại ở mỗi `NavigationEnd` (+ 1 lần lúc khởi tạo cho F5/direct URL), đi `root → firstChild → …`:

- Route có `data.breadcrumb` (i18n key, hoặc chuỗi thô — `translate.get` trả lại nguyên key khi không có bản dịch) → push `{label, url}`.
- `url` = `data.routerLink` nếu có, không thì ghép `route.url` của mọi cấp tới đó.
- Bỏ qua khi `breadcrumb === parent.data.breadcrumb` — Angular 22.0.8 mặc định `paramsInheritanceStrategy: 'always'` (`node_modules/@angular/router/fesm2022/_router-chunk.mjs:1498` `DEFAULT_PARAMS_INHERITANCE_STRATEGY`, `src/app.config.ts` không cấu hình lại): route con LUÔN kế thừa (merge, con ghi đè) `data` của cha ở MỌI tầng, không so thì nhãn bị lặp.
- `data: { breadcrumb: null }` trên route con `path: ''` = chủ động ẩn (chặn kế thừa).
- Mục cuối luôn render `<span>` (không link). Icon home `/` luôn đứng đầu, không cấu hình được.
- Ẩn hoàn toàn khi `!isDesktop()`.

## 3. Nhãn động (tên bản ghi)

```ts
private readonly breadcrumbLabelService = inject(BreadcrumbLabelService);
// sau khi load xong model:
this.breadcrumbLabelService.set(model.fullname);
```

- Chỉ thay mục CUỐI; chuỗi rỗng → quay về không có mục nào được thay.
- Tự xoá ở `NavigationStart` khi **path** đổi (bỏ qua query/fragment) — đổi `?tab=` không làm mất tên. Xoá ở Start chứ không phải End vì component đích chạy `ngOnInit` trước `NavigationEnd`.
- Đang dùng: `crm/customer/customer-detail`, `emr/examination`, `loyalty/broker/detail`, `inventory/material-recall/detail/recall-detail` (hook `afterModelLoaded`), `accountant/fund/cashbook/fund-cashbook` (tên quỹ từ `FundHelperService` cache), `zalo/zns-campaign/edit/zns-campaign-edit`, `config/config-customer/consent-form/consent-form-print`.

## 4. Gotcha

- **Cấp cha chỉ hiện khi route CHA (tổ tiên) có `breadcrumb` — route anh em thì không.** Màn list khai `path: ''` và detail khai `path: 'detail/:id'` là 2 anh em trong cùng mảng ⇒ breadcrumb của detail KHÔNG có mục list. Từ epic `breadcrumb-standardization` (2026-09), pattern phẳng này đã được gom lại thành khuôn C3 ở toàn bộ module chuẩn hoá (CRM/sale/loyalty/EMR/inventory/accountant/config/zalo…) — vd `/crm/customer/detail/:id` nay ra `🏠 > CRM > Quản lý khách hàng > <tên khách>`, không còn nhảy thẳng module→bản ghi. Route mới/chưa chuẩn hoá vẫn có thể mắc lỗi này — dùng khuôn C3/C3' ở mục 5.
  Muốn có cấp cha: gom list + detail dưới 1 route cha có `breadcrumb`, route con `''` để `breadcrumb: null`.
- **Không có cấp module theo mặc định** — nay đã có: mọi route gốc trong `src/app.routes.ts` (`crm`, `emr`, `hr`…) khai `data.breadcrumb = <key module>` (C1). Module không có route con `''` có component (zalo, cx-ai, log) khai thêm `data.routerLink` trỏ trang đích (lấy theo `system-module-menu.ts`).
- `data.routerLink` ghi đè URL của mục đó — sai là link chết. Route con LUÔN kế thừa NGUYÊN `data` của cha ở MỌI tầng (`paramsInheritanceStrategy: 'always'`, không riêng cha-không-component), gồm cả `routerLink` — con dùng khuôn C3' (danh sách logic không phải tổ tiên URL, vd `/emr/examination/:id`) PHẢI tự khai lại `breadcrumb` + `title` của chính mình, `routerLink` kế thừa từ cha vô hại vì mục cuối không render link. Cha có `routerLink` mà quên khai tường minh ở đúng route mong muốn ⇒ dễ vô tình trỏ nhầm lên route tổ tiên xa hơn (vd rơi về wrapper `/config` thay vì `/config/print-template`, hoặc ca thật Zalo 2026-09-15: `zns-campaign`/`config` kế thừa `routerLink` của route gốc module `/zalo/zalo-user`) — mục breadcrumb không phải cuối mà có MỘT TỔ TIÊN đã khai `data.routerLink` (vd module C1 zalo/cx-ai/log, wrapper `config`, cha khuôn C3' có `routerLink`) PHẢI tự khai lại `routerLink` của chính route đó, không thì href kế thừa nhầm giá trị tổ tiên (gate B7). Route KHÔNG có tổ tiên nào khai `routerLink` thì KHÔNG cần khai — href tự ghép từ URL segment là đúng, gate không bắt (B7=0 với hàng trăm route dạng này trên cây thật).
- Nhãn thô tiếng Việt/Anh không phải i18n key vẫn hiển thị (translate trả nguyên chuỗi) ⇒ lỗi này không tự lộ ra khi đổi ngôn ngữ, phải grep hoặc chạy gate (B1 bắt được).
- Cùng component detail mount ở nhiều route (`CustomerDetail` ở `/crm/customer/detail/:id`, `/cx-sale/lead/detail/:id`, `/cx-sale/convert/detail/:id`) ⇒ breadcrumb cấp cha phải khai ở từng route, không khai trong component được.
- `data.title` (tab trình duyệt) và `data.breadcrumb` là 2 key riêng — thiếu cái nào thì mất cái đó, không fallback sang nhau.
- `permissionGuard` đọc `route.data.permission` trên CHÍNH route snapshot khớp — cha không component có nhiều con guard khác nhau (vd zalo `zns-campaign` view/create/update) thì PHẢI giữ `canActivate` + `data.permission` trên TỪNG con, cha không guard; chỉ dời `canActivate` lên cha khi guard của MỌI con giống hệt nhau.
- Nhãn bản ghi ở màn chi tiết (`BreadcrumbLabelService.set(...)`) lấy từ model/dữ liệu ĐÃ TẢI ở đúng màn đó — kể cả khi nguồn là helper cache async (vd `FundHelperService.getFundName()` sau `ensureFundsLoaded().then(...)`) thay vì response HTTP trực tiếp; không gọi thêm API riêng chỉ để lấy tên hiển thị breadcrumb.
- Route gốc module có `routerLink` (C1: zalo, cx-ai, log) ⇒ mọi route con kế thừa `routerLink` đó ⇒ mục crumb không phải cuối (không tự khai `routerLink` riêng) trỏ nhầm về trang đích của MODULE thay vì đích của chính nó (ca thật Zalo 2026-09-15: `zns-campaign`, `config` đều trỏ `/zalo/zalo-user`).
- Mục breadcrumb (và `data.routerLink` cấp module) trỏ tới route có guard **chặt hơn** trang đang đứng ⇒ user có quyền trang con nhưng thiếu quyền trang đích bấm vào bị đá `/auth/accessdenied`. Gate/dump-map chỉ bảo vệ guard theo URL, KHÔNG bắt lỗi này. Khi chọn đích link: đối chiếu guard trang đích với guard MỌI trang con dùng mục đó; đích chặt hơn ⇒ chọn đích khác mà các trang con cùng vào được, hoặc chấp nhận có ghi lý do (ca thật: `/zalo/zalo-user` cần `permissionViewZaloUser`, `/zalo/zns-campaign` cần `permissionViewZnsCampaign` — user chỉ có quyền tạo/sửa ZNS bị chặn).

## 5. Quy ước bắt buộc (C1–C6)

Bảng dưới là quy ước chính thức (chốt từ epic `breadcrumb-standardization`, 2026-09) — tự đủ, không cần tra tài liệu epic. Máy cưỡng chế bằng gate ở mục 7.

| # | Cấp | Khai ở đâu | Quy tắc |
|---|---|---|---|
| C1 | Module | route gốc trong `src/app.routes.ts` (`crm`, `emr`…) | `data.breadcrumb = <key module>`. Module KHÔNG có route con `''` có component (zalo, cx-ai, log) → thêm `data.routerLink = <trang đích>` lấy theo `system-module-menu.ts`. Riêng `config`: giữ nguyên wrapper `''` trong `config.routes.ts` (đã đúng C1), không khai thêm ở `app.routes.ts`. |
| C2 | Trang tổng quan module | route `''` đầu mỗi `<module>.routes.ts` | `breadcrumb: null` (giữ `title`). |
| C3 | Danh sách | route CHA của resource | Khuôn C3 bên dưới. List là con `''` **không khai `breadcrumb`** (kế thừa cha → bị khử trùng); detail/sửa/tạo là con khác. |
| C4 | Bản ghi | component màn chi tiết/sửa | Route con khai `breadcrumb` tĩnh (fallback: `<resource>.detail` / `common.action.update` / `common.label.detail`), component gọi `BreadcrumbLabelService.set(tên\|mã)` sau khi có model. Màn tạo mới: `breadcrumb: 'common.action.create'`, không gọi service. |
| C5 | Nhãn | mọi `data.breadcrumb` | Chỉ i18n key có trong `src/assets/i18n/vi-VN.json`; `null` chỉ để chặn kế thừa. |
| C6 | Link | `data.routerLink` | Chỉ dùng khi URL tự ghép không phải trang (module không có `''`, hoặc danh sách logic không phải tổ tiên URL). Phải resolve tới route có component. Mục breadcrumb không phải cuối có MỘT TỔ TIÊN đã khai `routerLink` (module C1 zalo/cx-ai/log, wrapper `config`, cha C3' có `routerLink`) PHẢI tự khai lại `routerLink` của chính route đó, KHÔNG dựa vào kế thừa (gate B7). Route không có tổ tiên nào khai `routerLink` thì KHÔNG cần khai — href tự ghép URL segment là đúng, gate không bắt. |

### Khuôn C3 — gom list/detail không đổi URL

```ts
// TRƯỚC (anh em phẳng — detail mất cấp danh sách)
{ path: 'broker', canActivate: [g], loadComponent: List, data: { breadcrumb: 'x.list', title: 'x.list' } },
{ path: 'broker/detail/:id', canActivate: [g], loadComponent: Detail, data: { breadcrumb: 'x.detail', title: 'x.detail', preload: true } },

// SAU (cha không component, URL giữ nguyên)
{ path: 'broker', canActivate: [g] /* chỉ khi guard list == guard detail */, data: { breadcrumb: 'x.list' },
  children: [
    { path: '', loadComponent: List, data: { title: 'x.list' } },
    { path: 'detail/:id', loadComponent: Detail, data: { breadcrumb: 'x.detail', title: 'x.detail', preload: true } },
  ] },
```

### Khuôn C3' — danh sách logic không phải tổ tiên URL (không đổi URL)

```ts
// /emr/examination/:id — danh sách là /emr/reception
{ path: 'examination', data: { breadcrumb: 'common.label.reception_examination', routerLink: '/emr/reception' },
  children: [{ path: ':id', canActivate: [g], loadComponent: Exam, data: { breadcrumb: 'common.label.examination', title: '…' } }] },
```

Gotcha khuôn này: con LUÔN kế thừa `data` của cha ở mọi tầng (`paramsInheritanceStrategy: 'always'`), gồm cả `routerLink` và `title`. Con PHẢI tự khai `breadcrumb` + `title` của mình.

Nhãn con `:id` khi danh sách chỉ là bộ lọc (không phải trang quản lý resource riêng) → dùng `common.label.detail` (tránh B5: nhãn con trùng hệt nhãn cha bị khử âm thầm).

## 6. Checklist thêm route mới

- [ ] Route module mới → có `data.breadcrumb` (C1) ở `app.routes.ts`, hoặc `data.routerLink` nếu module không có `''` component.
- [ ] Trang tổng quan module (`''`) → `breadcrumb: null`, giữ `title`.
- [ ] List + detail cùng resource → gom theo khuôn C3 (không tạo anh em phẳng mới).
- [ ] Danh sách logic không phải tổ tiên URL → khuôn C3', con TỰ khai lại `breadcrumb`+`title`.
- [ ] `breadcrumb` luôn là i18n key có thật trong `vi-VN.json` (`npm run i18n:find -- "<value>"` trước khi thêm key mới).
- [ ] Route lá có `:id`, không phải tạo mới → component gọi `BreadcrumbLabelService.set(...)` sau khi có model.
- [ ] Cha không component có nhiều con guard KHÁC nhau → giữ `canActivate`+`data.permission` trên từng con, không dời lên cha.
- [ ] Mục breadcrumb KHÔNG phải cuối, có TỔ TIÊN đã khai `routerLink` (module C1 zalo/cx-ai/log, wrapper `config`, cha C3' có `routerLink`) → tự khai lại `routerLink` của chính route đó, không dựa kế thừa (B7). Không có tổ tiên nào khai `routerLink` → không cần khai.
- [ ] Chạy `npm run breadcrumb:audit` trước khi commit (đã gắn `.husky/pre-commit`).

## 7. Gate `breadcrumb-audit`

- `npm run breadcrumb:audit` — quét toàn bộ cây route từ `src/app.routes.ts`, in bảng lỗi (B1–B7) + exit 1 khi có lỗi chưa allowlist.
- `node scripts/breadcrumb-audit.mjs --dump-map` — in `URL pattern → component import path + guards + crumb` (cột crumb dạng `key→href`), dùng để đối chiếu tay khi thêm/sửa route lớn. `--map-only-routing` chỉ in URL+guard (không crumb) — dùng để diff bất biến routing giữa 2 lần chạy (vd trước/sau khi gom route).
- `npm run breadcrumb:audit:selftest` — tự kiểm script gate bằng fixture (7 loại lỗi B1–B7 phải đỏ đúng số, fixture sạch phải xanh). Chạy khi sửa `scripts/breadcrumb-audit.mjs`.
- Allowlist `scripts/breadcrumb-audit.allow.json`: `[{ "code": "B6", "route": "/cx-sale/opportunity/macro/:id", "reason": "..." }]` — chỉ dùng cho ngoại lệ có lý do thật (placeholder chưa có model, danh sách lọc theo id không phải bản ghi độc lập). Mục không còn khớp route thật ⇒ gate cảnh báo "allowlist không còn khớp" (tránh allowlist rác tích tụ).
- **B7** (2026-09-15, story-01 reopen ×2): mục breadcrumb KHÔNG phải cuối có href lấy từ `routerLink` KẾ THỪA (route đó không tự khai `routerLink`) — bắt đúng lớp lỗi Zalo thật (`zns-campaign`/`config` trỏ nhầm `/zalo/zalo-user`); mục cuối miễn (không render link).

### Giới hạn gate đã biết

- Gate chỉ mô phỏng `permissionGuard` đọc `route.data.permission` — không mô phỏng hành vi runtime khác của guard (redirect đích, thứ tự nhiều guard).
- `data.redirectTo` không được gate theo dõi/tính vào chuỗi crumb.
