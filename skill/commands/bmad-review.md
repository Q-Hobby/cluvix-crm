Review sâu adversarial 3 lens (mượn BMAD-METHOD) — săn lỗi bằng subagent fresh-context, tổng hợp, để user lọc.

Tham số: `$ARGUMENTS` = mục tiêu review. Chấp nhận:
- story-id (`story-06`) → review diff của story đó trong `bmad/<slug>/stories/`.
- `changes` / rỗng → review working-tree diff hiện tại (`git diff`).
- đường dẫn file/thư mục → review phạm vi đó.

Dùng khi: cần soi kỹ 1 story trước khi Done, 1 diff nhạy cảm (tiền/tenant/transaction/migration), hoặc bất kỳ artifact nào muốn tránh "nhìn qua thấy ổn". Bổ trợ persona `skill/bmad/personas/qa.md` — QA có thể gọi lệnh này cho phần săn sâu.

---

## Nguyên tắc (đọc `skill/bmad/personas/qa.md` mục "Adversarial")
- Giả định CÓ lỗi, phải đi tìm; đánh giá artifact KHÔNG tin lời tự khai; 0 finding phải giải trình; con người lọc false-positive.
- **Fresh context = information asymmetry**: mỗi lens chạy trong 1 **subagent riêng** chỉ được đưa **diff + AC/yêu cầu**, KHÔNG đưa Dev Notes/lý lẽ của Dev → đánh giá cái LÀM, không phải cái NÓI.
- **Model tier = inherit** cho cả 3 lens (adversarial verify — sai âm thầm, KHÔNG hạ bậc; Subagent Model Policy trong `skill/INDEX.md`).

---

## Bước 0 — Dựng phạm vi
1. Xác định target từ `$ARGUMENTS` (story-id / changes / path). Không rõ → hỏi user.
2. Lấy **diff** (`git diff` working tree, hoặc diff commit liên quan). Lấy **AC/yêu cầu** (từ story nếu có; nếu không, tóm tắt 1-2 dòng ý định thay đổi).
3. Diff rỗng → dừng, báo "không có thay đổi để review".

## Bước 1 — Chạy 3 lens TRỰC GIAO (3 subagent song song, mỗi con fresh-context)
Mỗi subagent nhận: **diff + AC + Hard Rules Cluvix** (trích `skill/skills/backend-hard-rules/SKILL.md` phần liên quan). KHÔNG nhận Dev Notes/log.

> **Ràng buộc vận hành cho cả 3 lens** (3 con chạy SONG SONG — đây là chỗ dễ đụng tài nguyên nhất): KHÔNG con nào được `go build ./...` (3 full-build chồng nhau = sự cố RAM 30-40GB) — chỉ build scope theo path trong diff, hoặc đọc `backend/tmp/run.log`. KHÔNG bind port 8081, KHÔNG `git checkout/restore/stash` để dựng bản gốc (dùng `git show HEAD:<file>`). Lens là read-only: KHÔNG sửa code. Chi tiết: `skill/bmad/executor-contract.md`.

- **Lens 1 — Adversarial correctness/rule** (bắt buộc săn, tìm cả cái thiếu): bug logic + vi phạm Hard Rules (tenant scope `ScopedDBNullBranch`/`GetBranchActiveID` — company đã xoá D15, KHÔNG `GetCompanyActiveID`, tiền `decimal`, response+i18n, migration idempotent+FK thật, entity `dao/entity.go`, không `SELECT *`, timezone, constants, `bumpDexieVersion`, cross-tenant leak, transaction/lock/idempotency, **cấm test/migrate chạm `DB_NAME`**). Săn thật; 0 finding phải giải trình đã săn gì.
- **Lens 2 — Edge-case path enumeration** (method-driven, KHÔNG bàn tốt/xấu, KHÔNG tự gán severity — tổng hợp mới xếp): đi CƠ HỌC mọi nhánh reachable từ dòng đổi — control flow (if/loop/early-return/error) + domain boundary (nil/rỗng/off-by-one/overflow/coercion/race/timeout) + **nhánh ngầm của tập hữu hạn** (enum/status/sentinel/flag: diff xử vài case của tập cố định thì phần còn lại là nhánh ngầm chưa xử). **Scope kỷ luật**: diff + cái diff gọi trực tiếp — KHÔNG lan sang phần codebase không được tham chiếu. Đi xong PHẢI **tự soát vòng 2** theo từng edge-class (else/default thiếu · input không guard · off-by-one · overflow · coercion · race · timeout) trước khi chốt. Báo CHỈ nhánh chưa có guard — nhánh đã handle bỏ qua im lặng; mỗi finding: điều kiện kích hoạt + **phác guard tối thiểu** + hệ quả. Diff xoá code → thêm bước check hồi quy do xoá.
- **Lens 3 — Verification-gap** (đọc test THẬT): hỏi đúng 1 câu — *"hành vi mà thay đổi này tạo ra, nếu hỏng ở chỗ nó được dùng, có kiểm chứng nào fail không?"* Không săn bug correctness (đó là Lens 1), nhưng vấn đề thật thấy trên đường trace → ghi mục **Other findings** riêng, không đi săn thêm.
  - **Sàng trước**: diff thuần phi-hành-vi (format/comment/rename thuần/type-only không đổi runtime) → dừng lens, báo sạch — không đào caller/test thêm. Đổi dependency/build/config/data-file = CÓ hành vi dù không dòng nào "trông quan trọng".
  - 3 dạng gap: (a) **regression-gap** — chỗ dùng code đổi regress mà không test nào fail; (b) **missing-adoption** — chỗ khác lẽ ra phải dùng logic mới mà không; CHỈ tính khi có **supersession signal** (intent PR/naming/site anh em đã thay/logic trùng bị xoá) + cùng contract quan sát được — thiếu 1 trong 2 thì chỉ là gợi ý refactor, không phải finding; (c) **broken-verification** — test có vẻ cover nhưng skip/flaky/không chạy trong verify bình thường/assert quá yếu.
  - **Trace**: hành vi đổi → caller trực tiếp/route đăng ký/consumer contract (FE, mobile, ZMA client, DB reader, event) — ưu tiên boundary quan sát được GẦN NHẤT (1–3 hop); >5 consumer giống nhau → gom nhóm, check đại diện, chỉ mở rộng khi consumer quan sát hành vi khác nhau.
  - **Demonstration bắt buộc**: mỗi finding nêu được regression NHỎ NHẤT thực tế (đảo nhánh, bỏ default, trả error code cũ, bỏ field, skip call...) mà không test nào bắt — code downstream không test SUÔNG không phải finding.
  - **Test chỉ được tính khi**: chạy trong verify bình thường + assertion quan sát đúng output/nhánh/contract đổi. KHÔNG tính: chỉ check không-throw/snapshot-only/đếm mock-call, e2e đi xuyên qua không assert output đổi, fixture/assertion cũ stale.
  - Pattern hay gặp: caller-path gap (test helper cover nhánh, nhưng giá trị caller thật không đi vào) · contract drift (đổi payload/schema/response phải verify ở consumer) · **migration compatibility (test chỉ tạo row format mới/schema sạch — không chạy trên data cũ/dirty state)** · phantom exception (nhánh partial-failure có handle nhưng 0 test) · removed verification (xoá test/nới assertion làm hành vi mất neo).
  - **KHÔNG báo**: cái compiler/type-check đã chặn; hành vi đã có integration/contract/e2e test thật cover; "coverage thấp"/thiếu file test suông; legacy không test mà diff không đụng.
  - **Quy tắc bằng chứng**: đọc test trước khi kết luận nó cover gì; search cả repo theo symbol + import trước khi nói "không có test"; finding phải nói rõ đã check tới đâu ("các test tôi đã đọc là X, Y — không cái nào assert Z"); không claim điều chưa verify → drop finding không nền được.

## Bước 2 — Tổng hợp + lọc
1. Gom finding 3 lens, **dedup** theo file:dòng.
2. **Lọc false-positive** (adversarial sinh nhiễu): bỏ nitpick/hiểu nhầm ý/hallucination; giữ lỗi có thể tái hiện. Nêu rõ finding nào là "cần user quyết".
3. Xếp theo mức nghiêm trọng (Critical: sai tiền/leak tenant/mất data/transaction lệch → High → Medium → Low). Mỗi finding: **file:dòng + điều kiện tái hiện + hệ quả + cách sửa** (actionable).

## Bước 3 — (tuỳ chọn) Pass 2
Nếu Pass 1 tìm được lỗi thật đáng kể và đã sửa → cân nhắc chạy lại 1 lần (thường bắt thêm). Pass 3 hiếm khi cần — diminishing returns (chỉ còn nitpick). Ghi rõ nếu dừng vì hội tụ.

## Bước 4 — Báo cáo
- Danh sách finding đã lọc, xếp hạng, actionable.
- **Other findings** (vấn đề thật Lens 3 nhặt được trên đường trace nhưng không phải verification-gap) → liệt kê mục riêng, không xếp hạng.
- Nếu là review story BMAD: gợi ý QA cập nhật Status (`Done` nếu sạch sau săn + AC pass; `In Progress` kèm findings nếu có lỗi) — KHÔNG tự sửa code trừ khi user yêu cầu.
- Nếu 0 finding thật sau khi săn: **giải trình** đã enumerate path nào, kiểm test nào, rule nào — không im lặng "sạch".
