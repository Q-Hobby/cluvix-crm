# Security Policy

Cluvix CRM handles real customer data (contact details, appointments, orders, payments and
debts, staff accounts). We take every vulnerability report seriously.

> 🇻🇳 **Tóm tắt**: phát hiện lỗ hổng → KHÔNG mở issue công khai; báo qua GitHub Private
> Vulnerability Reporting (tab Security) hoặc kênh liên hệ bên dưới. Xác nhận trong 48h.

## Supported versions

Security fixes are made on the latest version of the default branch. Older snapshots are not
maintained; self-hosted deployments should update to the latest version.

## Scope

**In scope** — please report:

- **IDOR / tenant and branch scope**: reading or modifying a record (customer, appointment,
  order, payment, attachment, staff account…) that belongs to another branch you are not
  allowed to access, or by guessing/altering ids in the URL or request body.
- **RBAC bypass**: calling any protected `/api/*` route without the permission mapped to it in
  `backend/internal/modules/webapp/auth/rbac/seed/route_perm_map.json`, or escalating your own
  role or permissions.
- **Token / session issues**: a token issued for one deployment (customer database) or one
  user being accepted for another; refresh-token or 2FA bypass; device-limit bypass;
  session fixation.
- **Export / download**: accessing export files (`GET /api/download/export/:name`) or uploaded
  attachments without authentication or belonging to someone else.
- **Injection and client-side issues**: SQL injection, XSS (stored or reflected), path
  traversal, SSRF, unsafe file upload, CSRF on state-changing endpoints.
- **Secret exposure**: JWT keys, database or Redis credentials, S3 keys, third-party API keys
  leaked through logs, API responses, error messages, the frontend bundle or the git history.
- Public webhook/callback endpoints that accept forged or unauthenticated requests.

**Out of scope**:

- Vulnerabilities in third-party dependencies (Go modules, npm packages) without a working
  exploit against Cluvix CRM — report those to the upstream maintainer.
- Findings that require a compromised server, database or administrator account.
- Social engineering, physical attacks, and denial of service without a specific
  vulnerability (volumetric DoS/DDoS).
- Missing security headers or best-practice suggestions without demonstrated impact.
- Issues in a self-hosted deployment caused by its own misconfiguration.

## How to report

**Do not open a public GitHub issue, discussion or pull request** for a vulnerability —
disclosing it before a fix is available puts every deployment at risk.

Report through one of:

1. **GitHub Private Vulnerability Reporting** (preferred): the repository's **Security** tab →
   **Report a vulnerability**.
2. **Direct contact** with Cluvix Solutions: **<contact — to be announced>**.

Please include:

- Type of issue (IDOR, injection, secret exposure…)
- Affected endpoint(s) or file path(s)
- Step-by-step reproduction
- Estimated impact (whose data can be read or changed, what can be done)
- A proof of concept if available — **use your own test deployment and test data; do not
  access, modify or exfiltrate real customer data.**

## What to expect

| Step | Target time |
|---|---|
| Acknowledgement of your report | within 48 hours |
| Initial assessment and severity | within 7 days |
| Fix for critical issues (customer data exposure, authentication bypass) | target 72 hours after confirmation |
| Fix for other issues | in a regular release cycle |

We will keep you informed of progress, coordinate a disclosure date with you, and credit you
in the release notes if you wish.

## Safe harbor

We will not pursue legal action against researchers who act in good faith: who follow this
policy, avoid privacy violations and service disruption, only test against their own
deployments or accounts, and give us reasonable time to fix the issue before any disclosure.

## Recommendations for deployers and contributors

- Never commit `.env`, certificates, keys or PrimeUI license keys (already in `.gitignore`;
  always check `git status` before committing).
- Do not disable permission checks through configuration; the system is designed to fail
  closed.
- Do not serve the storage/export directory directly from the web server — export files
  contain customer personal data and must only be served by the authenticated download
  endpoint.
- Pull requests that touch authentication, permissions, tenant/branch scope or export code
  receive extra security review.
