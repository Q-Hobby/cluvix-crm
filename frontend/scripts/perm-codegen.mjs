#!/usr/bin/env node
/**
 * perm-codegen — sinh artifact permission phẳng từ permission.define.ts (BMAD perm-codegen-seeder / story-01).
 *
 * MỤC ĐÍCH: nguồn = FE PERMISSION_MATRIX → artifact `[{name, description}]` cho BE startup seeder
 * (thay cơ chế Sync-from-FE thủ công). Khớp 100% logic `flattenMatrixToPermissions` hiện tại của
 * `rbac.service.ts` để chuyển đổi không làm loạn `rbac_auth_item`.
 *
 * Usage:
 *   node scripts/perm-codegen.mjs          # sinh/ghi artifact (FE + copy sang BE embed path)
 *   node scripts/perm-codegen.mjs --check   # freshness gate: exit≠0 nếu output khác file đã commit
 *
 * Output:
 *   frontend/scripts/permissions.gen.json                              (nguồn build FE)
 *   backend/internal/modules/webapp/auth/rbac/seed/permissions.gen.json (BE go:embed — story-03)
 */
import { promises as fs } from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

import { parseSelect, parseMatrix } from './perm-define-parse.mjs';

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const FE_ROOT = path.resolve(__dirname, '..');            // frontend/
const REPO_ROOT = path.resolve(FE_ROOT, '..');            // repo root
const DEFINE = path.resolve(FE_ROOT, 'src/app/define/permission.define.ts');
const DEFINE_MATRIX = path.resolve(FE_ROOT, 'src/app/define/permission-matrix.define.ts');
const OUT_FE = path.resolve(__dirname, 'permissions.gen.json');
const OUT_BE = path.resolve(REPO_ROOT, 'backend/internal/modules/webapp/auth/rbac/seed/permissions.gen.json');

const rel = (p) => path.relative(REPO_ROOT, p);

// Nguồn = TẤT CẢ quyền trong PERMISSION_SELECT (tập quyền FE định nghĩa — dùng cho gating),
// KHÔNG chỉ PERMISSION_MATRIX (tập con hiển thị ở UI gán role). Lý do: seeder PRUNE quyền không
// còn trong artifact; nếu nguồn chỉ là matrix thì các quyền defined-nhưng-không-ở-matrix (vd
// viewTool/configDevelop/extImage...) sẽ bị prune oan khỏi role. Xem story-07 pre-flight.
// Description: ưu tiên "<module> — <action>" từ matrix; fallback "<section> — <value>" từ SELECT.
function buildPermissions(selectMap, constToApp, matrix) {
  const desc = new Map(); // value → "module — label" (matrix, giữ lần đầu)
  for (const mod of matrix) {
    for (const action of mod.permissions ?? []) {
      const v = action.value;
      if (v && typeof v === 'string' && !v.startsWith('__BADREF__') && !desc.has(v)) {
        desc.set(v, `${mod.name} — ${action.label}`);
      }
    }
  }
  const out = [];
  const seen = new Set();
  for (const [constKey, value] of Object.entries(selectMap)) {
    if (seen.has(value)) continue;
    seen.add(value);
    out.push({ name: value, description: desc.get(value) ?? `${constToApp[constKey] || 'Khác'} — ${value}` });
  }
  return out.sort((a, b) => a.name.localeCompare(b.name));
}

async function main() {
  const check = process.argv.slice(2).includes('--check');
  const defineText = await fs.readFile(DEFINE, 'utf8');
  const { map: selectMap, constToApp } = parseSelect(defineText);
  const matrixText = await fs.readFile(DEFINE_MATRIX, 'utf8');
  const matrix = parseMatrix(matrixText, selectMap);
  const perms = buildPermissions(selectMap, constToApp, matrix);
  const json = JSON.stringify(perms, null, 2) + '\n';

  // OUT_BE chỉ áp dụng khi có `backend/` cạnh frontend (monorepo local dev).
  // CI FE-only (workflow auto-push) → backend/ vắng → chỉ dùng OUT_FE, workflow tự copy sang BE repo.
  let hasBackend = false;
  try { await fs.access(path.resolve(REPO_ROOT, 'backend')); hasBackend = true; } catch { /* FE-only */ }
  const targets = hasBackend ? [OUT_FE, OUT_BE] : [OUT_FE];

  if (check) {
    let stale = false;
    for (const out of targets) {
      let cur = null;
      try { cur = await fs.readFile(out, 'utf8'); } catch { /* missing */ }
      if (cur !== json) {
        stale = true;
        console.error(`❌ stale/thiếu: ${rel(out)} — chạy \`npm run perm:codegen\` để regen & commit.`);
      }
    }
    if (stale) { process.exit(1); }
    console.log(`✅ permissions.gen.json khớp nguồn (${perms.length} quyền, ${targets.length} target).`);
    process.exit(0);
  }

  for (const out of targets) {
    await fs.mkdir(path.dirname(out), { recursive: true });
    await fs.writeFile(out, json);
  }
  console.log(`perm-codegen: ${perms.length} quyền → ${targets.map(rel).join(' + ')}`);
  process.exit(0);
}

main().catch(err => { console.error(err); process.exit(2); });
