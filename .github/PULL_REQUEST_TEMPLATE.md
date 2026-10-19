## What
<!-- 1-2 sentences: what changes / Đổi gì -->

## Why
<!-- 1-2 sentences: why it is needed / Tại sao cần đổi -->
<!-- Related issue: #… -->

## How
<!-- 2-3 bullets: how it was done / Làm thế nào -->
-

## Testing
<!-- How it was tested: gates, test cases, screenshots for UI changes… -->

---

## Checklist

- [ ] I have signed the CLA / Tôi đã ký CLA — comment `I have read the CLA Document and I hereby sign the CLA` on my first PR (see [`CLA.md`](../CLA.md))
- [ ] Backend changed → `cd backend && GOFLAGS=-p=4 go build ./... && go vet ./... && go test ./internal/... ./pkg/...` passes
- [ ] Frontend changed → `npx tsc --noEmit -p tsconfig.app.json`, `npx ng test --watch=false`, `npm run lint`, `npm run i18n:check` pass (in `frontend/`)
- [ ] Followed the Hard Rules in `CLAUDE.md` (tenant/branch scope, money as `decimal`, i18n, no `SELECT *`)
- [ ] Migrations (if any) are `.up.sql` only and idempotent
- [ ] No `.env`, secret, key, certificate or customer data committed (`git diff --cached --name-only`)
- [ ] New source files carry the SPDX header (`AGPL-3.0-only`, `Copyright 2026 Cluvix Solutions`)
