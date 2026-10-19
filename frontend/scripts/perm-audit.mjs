#!/usr/bin/env node
/**
 * perm-audit — Kiểm tra permission Frontend (BMAD story fe-permission-audit / story-01).
 *
 * MỤC ĐÍCH: quét tĩnh toàn bộ `src/app`, đối chiếu cách dùng permission (4 lớp) với
 * nguồn định nghĩa (`PERMISSION_SELECT` + `PERMISSION_MATRIX` trong permission.define.ts),
 * phân loại bất thường theo 8 chiều D1–D8 và sinh báo cáo.
 *
 * 4 lớp usage:
 *   L1 route    — file .routes.ts   : requirePerm / requireAnyPerm / data:{permission}
 *   L2 menu     — file .menu.ts      : field permission: PERMISSION_SELECT.X
 *   L3 template — file .html         : hasPerm="permission.X" | hasPerm="'val'" | [...]
 *   L4 component— các .ts còn lại     : getPermission / hasPermission
 *
 * 8 chiều:
 *   D1 mismatch (heuristic)  D2 missing-guard  D3 orphaned  D4 undefined
 *   D5 matrix inconsistency  D6 CRUD coverage  D7 FE-only enforcement (report-only)
 *   D8 not-in-matrix (defined nhưng không gán được cho role thường)
 *
 * Usage:  node scripts/perm-audit.mjs [--json-only] [--quiet]
 * Output: Plan/perm-audit/report.md + report.json (so với repo root)
 * Exit:   0 nếu sạch · 1 nếu có D4>0, D5>0, D8(ngoài whitelist)>0 hoặc D3(ngoài whitelist)>0 --strict
 */
import { promises as fs } from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

import { parseSelect, parseMatrix } from './perm-define-parse.mjs';

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const FE_ROOT = path.resolve(__dirname, '..');            // frontend/
const REPO_ROOT = path.resolve(FE_ROOT, '..');            // repo root
const SRC = path.resolve(FE_ROOT, 'src/app');
const DEFINE = path.resolve(SRC, 'define/permission.define.ts');
// PERMISSION_MATRIX đã tách sang file riêng (2026-08-28) để không vào bundle eager.
const DEFINE_MATRIX = path.resolve(SRC, 'define/permission-matrix.define.ts');
const OUT_DIR = path.resolve(REPO_ROOT, 'Plan/perm-audit');
const ALLOW_FILE = path.resolve(__dirname, 'perm-audit.allow.json');

const rel = (p) => path.relative(REPO_ROOT, p);

// ── Map thư mục module → appModule (dùng cho heuristic D1 + gom nhóm) ────────────
const FOLDER_TO_APPMODULE = {
  crm: 'cxCrm', emr: 'cxEmr', care: 'cxCare', loyalty: 'cxLoyalty', sale: 'cxSale',
  marketing: 'cxMarketing', med: 'cxPharmacy', ai: 'cxAi', zalo: 'cxZalo',
  inventory: 'Kho & Vật tư', accountant: 'Kế toán', hr: 'Nhân sự', config: 'Cấu hình',
};

// ── FS walk ─────────────────────────────────────────────────────────────────────
async function walk(dir, out = []) {
  const entries = await fs.readdir(dir, { withFileTypes: true });
  for (const e of entries) {
    const full = path.join(dir, e.name);
    if (e.isDirectory()) {
      if (e.name === 'node_modules' || e.name.startsWith('.')) continue;
      await walk(full, out);
    } else if (e.isFile()) {
      out.push(full);
    }
  }
  return out;
}

// ── Route-graph: phân tích guard theo cây route (parent loadChildren áp cho con) ──
// Linear scan với stack frame per `{`; canActivate trên 1 object → áp cho object đó + con.
function scanRoutes(content) {
  const edges = [];   // { spec, guarded }  — loadChildren import
  const leaves = [];  // { guarded, path }  — loadComponent / component: (trang thực)
  const stack = [];   // frame { g:bool, path:string|null }
  let quote = null;
  for (let i = 0; i < content.length; i++) {
    const c = content[i];
    if (quote) { if (c === quote && content[i - 1] !== '\\') quote = null; continue; }
    if (c === '"' || c === "'" || c === '`') { quote = c; continue; }
    if (c === '{') { stack.push({ g: false, path: null }); continue; }
    if (c === '}') { stack.pop(); continue; }
    if (!stack.length) continue;
    if (content.startsWith('canActivate', i)) {
      // CHỈ tính permission-guard; authGuard/loginGuard (chỉ check đăng nhập) KHÔNG phải gate quyền
      if (/requirePerm|requireAnyPerm|permissionGuard/.test(content.slice(i, i + 200)))
        stack[stack.length - 1].g = true;
      i += 10; continue;
    }
    const pm = content.startsWith('path', i) && content.slice(i, i + 40).match(/^path\s*:\s*'([^']*)'/);
    if (pm) { stack[stack.length - 1].path = pm[1]; i += pm[0].length - 1; continue; }
    if (content.startsWith('loadChildren', i)) {
      const im = content.slice(i, i + 160).match(/import\(\s*['"]([^'"]+)['"]/);
      if (im) edges.push({ spec: im[1], frames: [...stack] });   // guarded tính sau (canActivate có thể nằm SAU loadChildren cùng object)
      continue;
    }
    if (content.startsWith('loadComponent', i)) {
      leaves.push({ frames: [...stack] });                        // path/guarded tính sau
      i += 12; continue;
    }
  }
  // Tính guarded/path theo TOÀN object (frame references) — không phụ thuộc thứ tự token trong object.
  for (const e of edges) e.guarded = e.frames.some(f => f.g);
  for (const l of leaves) { l.guarded = l.frames.some(f => f.g); l.path = [...l.frames].reverse().find(f => f.path)?.path ?? null; }
  return { edges, leaves };
}
function resolveSpec(fromFile, spec, srcDir) {
  let base = spec.startsWith('@/') ? path.join(srcDir, spec.slice(2)) : path.resolve(path.dirname(fromFile), spec);
  return base.endsWith('.ts') ? base : base + '.ts';
}

// ── Feature/action từ value string ────────────────────────────────────────────────
const ACTIONS = ['view', 'create', 'update', 'ext', 'delete'];
function featureOf(value) {
  const low = value.toLowerCase();
  for (const a of ACTIONS) {
    if (low.startsWith(a)) {
      const feature = value.slice(a.length);
      return { action: a[0].toUpperCase() + a.slice(1), feature: feature.charAt(0).toLowerCase() + feature.slice(1) };
    }
  }
  return { action: 'Other', feature: value };
}

// ── Phân loại file → layer ────────────────────────────────────────────────────────
function layerOf(file) {
  if (file.endsWith('.routes.ts')) return 'L1';
  if (file.endsWith('.menu.ts')) return 'L2';
  if (file.endsWith('.html')) return 'L3';
  if (file.endsWith('.ts')) return 'L4';
  return null;
}
function appModuleOfFile(file) {
  const r = rel(file); // frontend/src/app/modules/<mod>/...  hoặc .../layout/<mod>/...
  const m = r.match(/src\/app\/(?:modules|layout)\/([^/]+)\//);
  return m ? (FOLDER_TO_APPMODULE[m[1]] || m[1]) : null;
}

// ── Scan 1 file → danh sách usage {value, constKey, layer, line, raw} ──────────────
function scanFile(file, content, layer, selectMap, valueToConst) {
  const usages = [];
  const lines = content.split('\n');
  const push = (constKey, value, lineNo, raw) => usages.push({ constKey, value, layer, file, line: lineNo, raw });

  lines.forEach((line, idx) => {
    const lineNo = idx + 1;
    // (a) PERMISSION_SELECT.<constKey> — mọi layer
    let m;
    const selRe = /PERMISSION_SELECT\.(permission\w+)/g;
    while ((m = selRe.exec(line))) {
      const k = m[1];
      push(k, k in selectMap ? selectMap[k] : null, lineNo, m[0]);
    }
    // (a2) permission.<constKey> — mọi layer (component field `permission = PERMISSION_SELECT`
    //      dùng trong .html VÀ .ts như app.topbar.ts). Tên property permissionXxx là đặc trưng, an toàn.
    const dotRe = /\bpermission\.(permission\w+)/g;
    while ((m = dotRe.exec(line))) {
      const k = m[1];
      push(k, k in selectMap ? selectMap[k] : null, lineNo, m[0]);
    }
    if (layer === 'L3') {
      // (b) *hasPerm="..."  → trong expr tìm string literal 'value' (dạng array/inline)
      const hpRe = /hasPerm\s*=\s*"([^"]*)"/g;
      while ((m = hpRe.exec(line))) {
        const expr = m[1];
        let mm;
        const strRe = /'([a-zA-Z][\w]*)'/g;
        while ((mm = strRe.exec(expr))) {
          const v = mm[1];
          push(valueToConst[v] || null, v, lineNo, `'${v}'`);
        }
      }
    }
    if (layer === 'L4') {
      // (c) hasPermission('val') / getPermission('val') string literal
      const fnRe = /(?:hasPermission|getPermission)\(\s*['"]([a-zA-Z][\w]*)['"]\s*\)/g;
      while ((m = fnRe.exec(line))) {
        const v = m[1];
        push(valueToConst[v] || null, v, lineNo, m[0]);
      }
    }
  });
  return usages;
}

async function main() {
  const args = process.argv.slice(2);
  const jsonOnly = args.includes('--json-only');
  const quiet = args.includes('--quiet');

  const defineText = await fs.readFile(DEFINE, 'utf8');
  const { map: selectMap, valueToConst, constToApp } = parseSelect(defineText);
  const DEFINED = new Set(Object.values(selectMap));
  const matrixText = await fs.readFile(DEFINE_MATRIX, 'utf8');
  const matrix = parseMatrix(matrixText, selectMap);

  let allow = { orphaned: [], publicRoutes: [], notInMatrix: [] };
  try { allow = { ...allow, ...JSON.parse(await fs.readFile(ALLOW_FILE, 'utf8')) }; } catch { /* rỗng */ }
  const allowOrphan = new Set(allow.orphaned || []);
  const allowNotInMatrix = new Set(allow.notInMatrix || []);
  const allowPublic = new Set(allow.publicRoutes || []);

  // ── scan usage ──────────────────────────────────────────────────────────────
  const files = await walk(SRC);
  const usages = [];
  for (const file of files) {
    if (file === DEFINE || file === DEFINE_MATRIX) continue;  // KHÔNG tính define/matrix là usage
    if (file.endsWith('.spec.ts')) continue;     // bỏ test
    const layer = layerOf(file);
    if (!layer) continue;
    const content = await fs.readFile(file, 'utf8');
    if (!content.includes('PERMISSION_SELECT') && !content.includes('hasPerm') &&
        !content.includes('hasPermission') && !content.includes('getPermission')) continue;
    usages.push(...scanFile(file, content, layer, selectMap, valueToConst));
  }

  const usedValues = new Set(usages.filter(u => u.value && DEFINED.has(u.value)).map(u => u.value));

  // ── D3 orphaned ──────────────────────────────────────────────────────────────
  const D3 = [...DEFINED].filter(v => !usedValues.has(v)).map(v => ({
    value: v, constKey: valueToConst[v], appModule: constToApp[valueToConst[v]] || 'Unknown',
    whitelisted: allowOrphan.has(v),
  }));
  const D3active = D3.filter(x => !x.whitelisted);

  // ── D4 undefined (usage không map được về DEFINED) ────────────────────────────
  const D4 = usages.filter(u =>
    (u.value === null) ||                                 // const/typo không resolve
    (u.value && !DEFINED.has(u.value))                    // string literal lạ
  ).map(u => ({ token: u.raw, value: u.value, constKey: u.constKey, layer: u.layer, file: rel(u.file), line: u.line }));

  // ── D5 matrix inconsistency ───────────────────────────────────────────────────
  const D5 = [];
  const seenIds = new Map();
  for (const mod of matrix) {
    if (seenIds.has(mod.id)) D5.push({ kind: 'duplicate-id', id: mod.id, module: mod.name, note: `trùng id với "${seenIds.get(mod.id)}"` });
    else seenIds.set(mod.id, mod.name);
    for (const p of (mod.permissions || [])) {
      const ctx = `module "${mod.name}" (id ${mod.id}) · "${p.label}"`;
      if (typeof p.value === 'string' && p.value.startsWith('__BADREF__:'))
        D5.push({ kind: 'value-badref', id: mod.id, module: mod.name, note: `${ctx}: value tham chiếu const không tồn tại (${p.value.split(':')[1]})` });
      else if (p.value !== null && p.value !== undefined && !DEFINED.has(p.value))
        D5.push({ kind: 'value-undefined', id: mod.id, module: mod.name, note: `${ctx}: value "${p.value}" không có trong PERMISSION_SELECT` });
      if (!Number.isInteger(p.type) || p.type < 0 || p.type > 5)
        D5.push({ kind: 'type-range', id: mod.id, module: mod.name, note: `${ctx}: type=${p.type} ngoài {0..5}` });
      if (p.value != null && !String(p.value).startsWith('__BADREF__') && p.type === 0)
        D5.push({ kind: 'value-but-hidden', id: mod.id, module: mod.name, note: `${ctx}: có value nhưng type=0 (bị ẩn) → nên có type 1..5` });
      if ((p.value == null) && p.type !== 0)
        D5.push({ kind: 'null-but-active', id: mod.id, module: mod.name, note: `${ctx}: value=null nhưng type=${p.type}≠0` });
      if (p.parent !== mod.id)
        D5.push({ kind: 'parent-mismatch', id: mod.id, module: mod.name, note: `${ctx}: parent=${p.parent} ≠ id module ${mod.id}` });
    }
  }

  // ── D8 not-in-matrix: quyền defined nhưng KHÔNG gán được cho role thường ──────
  // PERMISSION_MATRIX là thứ duy nhất render ở màn Cấu hình → Phân quyền. Quyền chỉ nằm trong
  // PERMISSION_SELECT thì vào được catalog `rbac_auth_item` (super role tự có, vì super =
  // "mọi row type=2") nhưng KHÔNG có checkbox nào để gán cho role thường ⇒ tính năng khoá vĩnh
  // viễn với mọi user không phải admin, mà build/tsc/D1–D5 đều xanh.
  // Đây là nửa sau của sự cố mobile/plan/41 MOB-109 (nửa đầu — quên khai key hẳn — do
  // TestRoutePermMapPermsInCatalog bên BE chặn). Whitelist `notInMatrix` = baseline đã rà tay:
  // quyền standalone/dev-tool/report cố ý không cho gán qua UI.
  const inMatrixValues = new Set();
  for (const mod of matrix) {
    for (const p of (mod.permissions || [])) {
      if (typeof p.value === 'string' && p.value && !p.value.startsWith('__BADREF__')) inMatrixValues.add(p.value);
    }
  }
  const D8 = [...DEFINED].filter(v => !inMatrixValues.has(v)).map(v => ({
    value: v, constKey: valueToConst[v], appModule: constToApp[valueToConst[v]] || 'Unknown',
    whitelisted: allowNotInMatrix.has(v),
  }));
  const D8active = D8.filter(x => !x.whitelisted);

  // ── D2 missing-guard (parent-aware: guard ở route cha qua loadChildren áp cho con) ─
  const routeFiles = files.filter(f => f.endsWith('.routes.ts') || f.endsWith('.routers.ts'));
  // Shell gốc app.routes.ts nằm ở src/ (ngoài src/app) — nạp thêm để resolve cây route đầy đủ.
  const ROOT_ROUTES = path.resolve(SRC, '..', 'app.routes.ts');
  try { await fs.access(ROOT_ROUTES); if (!routeFiles.includes(ROOT_ROUTES)) routeFiles.push(ROOT_ROUTES); } catch { /* no-op */ }
  const parsedRoutes = {};                 // absFile -> { edges, leaves }
  const childToParents = {};               // childAbs -> [{ parent, guarded }]
  for (const f of routeFiles) {
    const content = await fs.readFile(f, 'utf8');
    parsedRoutes[f] = scanRoutes(content);
  }
  for (const f of routeFiles) {
    for (const e of parsedRoutes[f].edges) {
      const target = resolveSpec(f, e.spec, SRC);
      (childToParents[target] ||= []).push({ parent: f, guarded: e.guarded });
    }
  }
  const coveredMemo = new Map();
  function coveredByAncestor(f, seen = new Set()) {
    if (coveredMemo.has(f)) return coveredMemo.get(f);
    if (seen.has(f)) return false;
    seen.add(f);
    let res = false;
    for (const { parent, guarded } of (childToParents[f] || [])) {
      if (guarded || coveredByAncestor(parent, seen)) { res = true; break; }
    }
    coveredMemo.set(f, res);
    return res;
  }
  const D2 = [];
  for (const f of routeFiles) {
    const r = rel(f);
    if ([...allowPublic].some(p => r.includes(p))) continue;
    const leaves = parsedRoutes[f].leaves;
    if (!leaves.length) continue;                       // file thuần parent (chỉ loadChildren)
    if (coveredByAncestor(f)) continue;                 // đã gate ở tổ tiên
    // path '' / 'dashboard' = trang index/dashboard module — mở theo convention (vào bằng authGuard + menu), không tính gap.
    const LANDING = new Set(['', 'dashboard']);
    const openLeaves = leaves.filter(l => !l.guarded && !LANDING.has(l.path ?? ''));
    if (openLeaves.length) D2.push({ file: r, paths: [...new Set(openLeaves.map(l => l.path ?? "''"))] });
  }

  // ── D1 mismatch heuristic (usage feature thuộc appModule ≠ folder file) ────────
  const D1 = [];
  for (const u of usages) {
    if (!u.value || !DEFINED.has(u.value)) continue;
    const fileApp = appModuleOfFile(u.file);
    const keyApp = constToApp[u.constKey || valueToConst[u.value]] || null;
    if (!fileApp || !keyApp) continue;
    if (fileApp !== keyApp) {
      const { action } = featureOf(u.value);
      D1.push({ file: rel(u.file), line: u.line, value: u.value, layer: u.layer, fileApp, keyApp, action, mutating: action !== 'View' && action !== 'Other' });
    }
  }
  D1.sort((a, b) => (b.mutating - a.mutating));

  // ── D6 coverage: feature × action → nơi enforce ───────────────────────────────
  const featApp = {};        // feature -> appModule
  const featActions = {};    // feature -> action -> {defined:bool, layers:Set}
  for (const v of DEFINED) {
    const { action, feature } = featureOf(v);
    featApp[feature] = constToApp[valueToConst[v]] || 'Unknown';
    (featActions[feature] ||= {});
    (featActions[feature][action] ||= { defined: false, layers: new Set() });
    featActions[feature][action].defined = true;
  }
  for (const u of usages) {
    if (!u.value || !DEFINED.has(u.value)) continue;
    const { action, feature } = featureOf(u.value);
    if (!featActions[feature]?.[action]) continue;
    featActions[feature][action].layers.add(u.layer);
  }

  // ── report.json ───────────────────────────────────────────────────────────────
  const summary = {
    definedCount: DEFINED.size, matrixModules: matrix.length, usageCount: usages.length,
    D1: D1.length, D2: D2.length, D3_active: D3active.length, D3_whitelisted: D3.length - D3active.length,
    D4: D4.length, D5: D5.length,
    D8_active: D8active.length, D8_whitelisted: D8.length - D8active.length,
  };
  const report = { generatedFrom: rel(DEFINE), summary, D1, D2, D3, D4, D5, D8,
    D6: Object.fromEntries(Object.entries(featActions).map(([f, acts]) => [f, {
      appModule: featApp[f],
      actions: Object.fromEntries(Object.entries(acts).map(([a, o]) => [a, { defined: o.defined, layers: [...o.layers], covered: o.layers.size > 0 }])),
    }])),
  };
  await fs.mkdir(OUT_DIR, { recursive: true });
  await fs.writeFile(path.join(OUT_DIR, 'report.json'), JSON.stringify(report, null, 2));

  // ── report.md ─────────────────────────────────────────────────────────────────
  const md = buildMarkdown(report, { DEFINED, featActions, featApp });
  await fs.writeFile(path.join(OUT_DIR, 'report.md'), md);

  // ── console + exit code ───────────────────────────────────────────────────────
  // D4 (undefined/typo) + D5 (matrix broken) = LUÔN bug → block. D3 (orphaned) có thể đang làm dở
  // → chỉ WARN; block khi --strict. Pre-commit dùng mặc định (chỉ chặn bug thật).
  const strict = args.includes('--strict');
  // D8 (defined nhưng không gán được cho role) cũng LUÔN block: quyền mới mà quên đưa vào
  // PERMISSION_MATRIX = tính năng khoá với mọi non-admin, không lưới nào khác bắt được.
  const fail = D4.length > 0 || D5.length > 0 || D8active.length > 0 || (strict && D3active.length > 0);
  if (!jsonOnly && !quiet) {
    console.log('perm-audit — permission Frontend');
    console.log(`  defined=${summary.definedCount} matrixModules=${summary.matrixModules} usages=${summary.usageCount}`);
    console.log(`  D1 mismatch(heuristic) : ${summary.D1}`);
    console.log(`  D2 missing-guard       : ${summary.D2}`);
    console.log(`  D3 orphaned            : ${summary.D3_active} (+${summary.D3_whitelisted} whitelisted)`);
    console.log(`  D4 undefined           : ${summary.D4}`);
    console.log(`  D5 matrix              : ${summary.D5}`);
    console.log(`  D8 not-in-matrix       : ${summary.D8_active} (+${summary.D8_whitelisted} whitelisted)`);
    console.log(`  → ${rel(path.join(OUT_DIR, 'report.md'))}`);
    if (D3active.length && !strict) console.log(`  ⚠️  D3 orphaned ${D3active.length} (warn — dùng --strict để chặn)`);
    for (const x of D8active) {
      console.log(`  ❌ D8: "${x.value}" (${x.constKey}) có trong PERMISSION_SELECT nhưng KHÔNG có trong PERMISSION_MATRIX`);
      console.log('       → thêm module/entry vào PERMISSION_MATRIX để admin gán được cho role thường,');
      console.log('         hoặc khai vào "notInMatrix" của scripts/perm-audit.allow.json nếu CHỦ ĐÍCH không cho gán.');
    }
    console.log(fail ? '  ❌ FAIL (D4/D5/D8 > 0' + (strict ? ' hoặc D3 --strict' : '') + ')' : '  ✅ OK');
  }
  process.exit(fail ? 1 : 0);
}

function buildMarkdown(report, ctx) {
  const { summary, D1, D2, D3, D4, D5, D6, D8 } = report;
  const L = [];
  L.push('# Permission Audit Report — Frontend', '');
  L.push(`> Sinh tự động bởi \`frontend/scripts/perm-audit.mjs\` từ \`${report.generatedFrom}\`. KHÔNG sửa tay.`, '');
  L.push('## Tổng quan', '');
  L.push('| Chiều | Số lượng |', '|---|---|');
  L.push(`| Keys định nghĩa | ${summary.definedCount} |`);
  L.push(`| Module trong matrix | ${summary.matrixModules} |`);
  L.push(`| Điểm usage quét được | ${summary.usageCount} |`);
  L.push(`| **D1** mismatch (heuristic, cần review) | ${summary.D1} |`);
  L.push(`| **D2** missing-guard (ứng viên) | ${summary.D2} |`);
  L.push(`| **D3** orphaned (ngoài whitelist) | ${summary.D3_active} |`);
  L.push(`| D3 orphaned (đã whitelist) | ${summary.D3_whitelisted} |`);
  L.push(`| **D8** not-in-matrix (ngoài whitelist) | ${summary.D8_active} |`);
  L.push(`| D8 not-in-matrix (đã whitelist) | ${summary.D8_whitelisted} |`);
  L.push(`| **D4** undefined | ${summary.D4} |`);
  L.push(`| **D5** matrix inconsistency | ${summary.D5} |`);
  L.push('');

  L.push('## D5 — Matrix inconsistency', '');
  if (!D5.length) L.push('_Không có._', '');
  else { L.push('| id | module | lỗi |', '|---|---|---|'); for (const x of D5) L.push(`| ${x.id} | ${x.module} | ${x.kind}: ${x.note} |`); L.push(''); }

  L.push('## D4 — Undefined (usage không map được về PERMISSION_SELECT)', '');
  if (!D4.length) L.push('_Không có._', '');
  else { L.push('| file:line | layer | token | value |', '|---|---|---|---|'); for (const x of D4) L.push(`| ${x.file}:${x.line} | ${x.layer} | \`${x.token}\` | ${x.value ?? '(unresolved)'} |`); L.push(''); }

  L.push('## D3 — Orphaned (định nghĩa nhưng không dùng)', '');
  const d3a = D3.filter(x => !x.whitelisted);
  if (!d3a.length) L.push('_Không có (ngoài whitelist)._', '');
  else {
    L.push('| appModule | value | constKey |', '|---|---|---|');
    for (const x of d3a) L.push(`| ${x.appModule} | ${x.value} | ${x.constKey} |`);
    L.push('');
  }

  L.push('## D8 — Not-in-matrix (defined nhưng role thường không gán được)', '');
  const d8a = (D8 || []).filter(x => !x.whitelisted);
  if (!d8a.length) L.push('_Không có (ngoài whitelist)._', '');
  else {
    L.push('| appModule | value | constKey |', '|---|---|---|');
    for (const x of d8a) L.push(`| ${x.appModule} | ${x.value} | ${x.constKey} |`);
    L.push('');
  }

  L.push('## D2 — Missing guard (route file không có guard nào)', '');
  if (!D2.length) L.push('_Không có._', '');
  else { L.push('| route file | paths |', '|---|---|'); for (const x of D2) L.push(`| ${x.file} | ${x.paths.map(p => `\`${p || "''"}\``).join(', ')} |`); L.push(''); }

  L.push('## D1 — Mismatch heuristic (usage feature ≠ folder module)', '');
  L.push('> Heuristic — có thể là dùng chéo hợp lệ. **Review từng dòng**, ưu tiên `mutating=true`.', '');
  if (!D1.length) L.push('_Không có._', '');
  else {
    L.push('| file:line | layer | value | keyApp | fileApp | mutating |', '|---|---|---|---|---|---|');
    for (const x of D1) L.push(`| ${x.file}:${x.line} | ${x.layer} | ${x.value} | ${x.keyApp} | ${x.fileApp} | ${x.mutating ? '⚠️' : ''} |`);
    L.push('');
  }

  L.push('## D6 — CRUD coverage theo app-module', '');
  L.push('> `✅`=có enforce (kèm lớp), `❌`=key định nghĩa nhưng KHÔNG dùng ở đâu, `–`=không có key action đó.', '');
  const byApp = {};
  for (const [feat, o] of Object.entries(D6)) (byApp[o.appModule] ||= []).push([feat, o]);
  const order = ['View', 'Create', 'Update', 'Ext', 'Delete', 'Other'];
  for (const app of Object.keys(byApp).sort()) {
    L.push(`### ${app}`, '');
    L.push('| feature | View | Create | Update | Ext | Delete | Other |', '|---|---|---|---|---|---|---|');
    for (const [feat, o] of byApp[app].sort((a, b) => a[0].localeCompare(b[0]))) {
      const cell = (a) => { const x = o.actions[a]; if (!x || !x.defined) return '–'; return x.covered ? `✅ ${x.layers.join('/')}` : '❌'; };
      L.push(`| ${feat} | ${cell('View')} | ${cell('Create')} | ${cell('Update')} | ${cell('Ext')} | ${cell('Delete')} | ${cell('Other')} |`);
    }
    L.push('');
  }

  L.push('## D7 — FE-only enforcement (caveat, report-only)', '');
  L.push('Permission theo tính năng **chỉ enforce ở Frontend**. Backend (`pkg/define/rbac.go`) chỉ xử lý super-role + tenant scope, KHÔNG có middleware kiểm quyền per-endpoint. Do đó:', '');
  L.push('- Permission FE **không thay thế** kiểm soát dữ liệu ở BE (`ScopedDBNullBranch`, `GetCompanyActiveID` — Hard Rules `CLAUDE.md`).', '');
  L.push('- Người dùng có thể bỏ qua UI và gọi API trực tiếp → BE vẫn phải tự bảo vệ. Đây là caveat kiến trúc, KHÔNG nằm phạm vi fix đợt này (xem `Plan/fe-permission-audit.md`).', '');
  return L.join('\n');
}

main().catch(err => { console.error(err); process.exit(2); });
