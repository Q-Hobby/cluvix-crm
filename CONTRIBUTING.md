# Contributing to Cluvix CRM

Thank you for your interest in contributing! Cluvix CRM is maintained by **Cluvix Solutions**
and accepts contributions from the community under the terms below.

> 🇻🇳 **Tóm tắt**: ký CLA (comment trên PR đầu tiên) → fork → tạo nhánh → code theo Hard Rules
> trong `CLAUDE.md` (→ `skill/INDEX.md`) → chạy đủ gate backend/frontend → mở PR theo mẫu.
> Lỗ hổng bảo mật: KHÔNG mở issue công khai, xem `SECURITY.md`.

## 1. Contributor License Agreement (required)

Cluvix CRM is dual-licensed under the **GNU AGPL-3.0-only** and a **commercial license**
(see [`COMMERCIAL-LICENSE.md`](./COMMERCIAL-LICENSE.md)). To keep that possible, every
contributor must sign the [Contributor License Agreement](./CLA.md) before a pull request can
be merged. You keep the copyright in your work; the CLA grants Cluvix Solutions a license to
use and relicense it.

- **Individuals**: post this comment on your first pull request:
  `I have read the CLA Document and I hereby sign the CLA`
- **Contributing for an organization**: your organization signs Part B of the CLA first —
  see [`CLA.md`](./CLA.md).

By submitting a contribution you agree that it is licensed to the public under the
AGPL-3.0-only (with the section 7 additional permission in [`NOTICE`](./NOTICE)) and to Cluvix
Solutions under the CLA.

## 2. Before you start

1. Read [`CLAUDE.md`](./CLAUDE.md) (a symlink to [`skill/INDEX.md`](./skill/INDEX.md)) —
   the **Hard Rules** apply to humans and AI agents alike. Domain-specific rules live under
   `skill/skills/` (backend) and `frontend/skill/` (frontend).
2. For anything beyond a small fix (new feature, database/schema change, cross-module change),
   **open an issue first** to discuss the approach before writing code.
3. If the requirement is ambiguous, ask in the issue — do not guess.

## 3. Workflow

```bash
# 1. Fork the repository on GitHub, then clone your fork
git clone <your-fork-url> cluvix-crm
cd cluvix-crm

# 2. Create a branch from the default branch
git checkout -b feat/<short-description>     # or fix/…, docs/…, refactor/…

# 3. Make your change, run the gates (section 5), commit, push to your fork
git push origin feat/<short-description>

# 4. Open a pull request against the upstream default branch
```

Keep pull requests small and focused: one feature or fix per PR. Unrelated refactors or
"improvements" outside the scope of the PR will be asked to be split out.

## 4. Commit messages

Use [Conventional Commits](https://www.conventionalcommits.org/), first line under 72
characters:

```
feat(crm): add filter by source to customer list
fix(appointment): reject end time before start time
docs: clarify local TLS setup
```

Types: `feat`, `fix`, `refactor`, `perf`, `docs`, `test`, `chore`. One logical change per
commit.

## 5. Required gates

Every pull request must pass these locally before review.

**Backend** (`backend/`):

```bash
cd backend
GOFLAGS=-p=4 go build ./... && go vet ./... && go test ./internal/... ./pkg/...
```

Integration tests that need MySQL must point `TEST_MYSQL_DSN` at a **disposable** database
created only for the test run — never at a database that holds real data (the test helpers
drop all tables).

**Frontend** (`frontend/`):

```bash
cd frontend
npx tsc --noEmit -p tsconfig.app.json
npx ng test --watch=false
npm run lint
npm run i18n:check
```

## 6. Key rules (summary — the full list is in `CLAUDE.md`)

- **Never commit secrets**: no `.env`, keys, certificates, tokens, PrimeUI license keys or
  customer data. Check with `git diff --cached --name-only` before committing.
- **Migrations**: only `.up.sql` files (no `.down.sql`), 14-digit timestamp names, and every
  migration must be **idempotent** (check `information_schema` before adding/dropping
  columns, indexes or foreign keys). Timestamp columns are `BIGINT` Unix time; money is
  `decimal(20,2)`.
- **Tenant/branch scope**: reads and writes by id go through the `tenant` scope helpers;
  never query a record by raw id without scope. Parse path/query ids to numbers before
  querying.
- **Money**: `decimal.Decimal` with `.Add/.Sub/.Mul/.Div` — never `float64`.
- **Responses and messages**: use the `response.*` helpers and `i18n` keys; frontend strings
  go through i18n (add keys to both `vi-VN.json` and `en-US.json`).
- No `SELECT *`; select only the fields the frontend needs.
- Follow existing patterns; do not introduce new conventions without discussion.

## 7. License headers

New source files should start with an SPDX header:

```go
// SPDX-License-Identifier: AGPL-3.0-only
// Copyright 2026 Cluvix Solutions
```

(Use the comment syntax of the file's language.)

## 8. AI-assisted contributions

Code written with AI assistants is welcome. You must understand every line you submit, be able
to explain the design choices, and answer review questions about it. Pull requests that read
like unreviewed AI output (unrequested refactors, mass edits outside scope) will be closed.

## 9. Reporting security issues

**Do not** open a public issue for security vulnerabilities. See [`SECURITY.md`](./SECURITY.md).

## 10. Trademarks

Contributing does not grant rights to the "Cluvix" names or logos. See
[`TRADEMARK.md`](./TRADEMARK.md).
