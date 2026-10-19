#!/usr/bin/env node
/**
 * breadcrumb-audit — gate chuẩn hoá breadcrumb (BMAD epic breadcrumb-standardization / story-01).
 *
 * Đọc cây route bằng TypeScript AST (KHÔNG regex — comment/`data` nhiều dòng/`null` làm regex đếm sai),
 * mô phỏng đúng thuật toán `addBreadcrumb` trong `src/app/layout/components/app.breadcrumb.ts`
 * + kế thừa `data` của router theo runtime đo được (con LUÔN merge `data` cha — architecture.md "Đính chính lần 2"),
 * rồi báo 7 loại lỗi:
 *
 *   B1  `breadcrumb` là chuỗi không phải i18n key (`a.b` chữ thường) hoặc key không có trong vi-VN.json
 *   B2  lá trong layout có breadcrumb mà crumb[0] không phải cấp module
 *   B3  lá có tham số `:x` mà chuỗi crumb < 3 mục (trừ lá `.../create`)
 *   B4  crumb không-cuối có URL (tự ghép hoặc `routerLink`) không khớp lá nào có component
 *   B5  route `path !== ''` khai `breadcrumb` bằng đúng giá trị (sau kế thừa) của cha ⇒ bị ẩn im lặng
 *   B6  lá có tham số (không phải tạo mới) mà file component không chứa `BreadcrumbLabelService`
 *   B7  crumb không-cuối có href lấy từ `routerLink` KẾ THỪA (route push mục đó không tự khai `routerLink`)
 *
 * Usage (chạy từ frontend/):
 *   node scripts/breadcrumb-audit.mjs                     bảng lỗi, exit 1 khi có lỗi
 *   node scripts/breadcrumb-audit.mjs --dump-map          `URL | component | guards | crumbs` (sort ổn định)
 *   node scripts/breadcrumb-audit.mjs --map-only-routing  `URL | component | guards` (bất biến cho diff baseline)
 *   node scripts/breadcrumb-audit.mjs --json              kết quả dạng JSON
 *   node scripts/breadcrumb-audit.mjs --root <dir>        trỏ cây khác (fixture) — <dir> đóng vai frontend/
 *   node scripts/breadcrumb-audit.mjs --allow <file>      allowlist khác (mặc định xem ALLOW_FILE bên dưới)
 *   node scripts/breadcrumb-audit.mjs --selftest          chạy fixture scripts/__fixtures__/breadcrumb-audit/{bad,clean}
 *
 * Fail-loud: cú pháp route không hiểu được / import không resolve được ⇒ in lỗi + exit 1, KHÔNG bỏ qua.
 */
import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

import ts from 'typescript';

const SCRIPT_DIR = path.dirname(fileURLToPath(import.meta.url));
const DEFAULT_ROOT = path.resolve(SCRIPT_DIR, '..');
const DEFAULT_ALLOW_FILE = path.join(SCRIPT_DIR, 'breadcrumb-audit.allow.json');
const FIXTURE_DIR = path.join(SCRIPT_DIR, '__fixtures__', 'breadcrumb-audit');

/** 15 route gốc (app.routes.ts) có layout render `app-breadcrumb`. Nhánh khác bỏ qua khi kiểm lỗi. */
const BREADCRUMB_ROOTS = new Set([
  'config', 'crm', 'emr', 'med', 'hr', 'accountant', 'inventory', 'care', 'loyalty',
  'cx-ai', 'cx-sale', 'cx-marketing', 'log', 'zalo', 'workspace',
]);
/** Module khai cấp module ở wrapper `''` trong `<module>.routes.ts` thay vì ở app.routes.ts (C1). */
const MODULE_WRAPPER_ROOTS = new Set(['config']);
const KEY_RE = /^[a-z0-9_]+(\.[a-z0-9_]+)+$/;
const LABEL_SERVICE = 'BreadcrumbLabelService';
const GUARD_KEYS = ['canMatch', 'canActivate', 'canActivateChild'];
const CODES = ['B1', 'B2', 'B3', 'B4', 'B5', 'B6', 'B7'];
const CODE_TITLES = {
  B1: 'breadcrumb không phải i18n key hợp lệ / key không có trong vi-VN.json',
  B2: 'crumb đầu tiên không phải cấp module',
  B3: 'lá có tham số nhưng chuỗi crumb < 3 mục',
  B4: 'crumb không-cuối trỏ URL không khớp lá nào có component',
  B5: 'breadcrumb trùng giá trị của cha ⇒ bị ẩn im lặng',
  B6: `lá có tham số nhưng component không dùng ${LABEL_SERVICE}`,
  B7: 'crumb không-cuối lấy href từ routerLink KẾ THỪA (route push mục không tự khai routerLink)',
};

class AuditError extends Error {}

const printer = ts.createPrinter({ removeComments: true });

// ── Source loading ────────────────────────────────────────────────────────────────
function createContext(root) {
  return { root, srcApp: path.join(root, 'src', 'app'), sources: new Map() };
}

const toPosix = (p) => p.split(path.sep).join('/');
const relOf = (ctx, file) => toPosix(path.relative(ctx.root, file));
const lineOf = (sf, node) => sf.getLineAndCharacterOfPosition(node.getStart(sf)).line + 1;
const where = (ctx, sf, node) => `${relOf(ctx, sf.fileName)}:${lineOf(sf, node)}`;

/** Chuẩn hoá text biểu thức: bỏ comment + gộp khoảng trắng ⇒ đổi format/xuống dòng không làm lệch dump. */
function exprText(sf, node) {
  return printer.printNode(ts.EmitHint.Expression, node, sf).replace(/\s+/g, ' ').trim();
}

function loadSource(ctx, file) {
  let src = ctx.sources.get(file);
  if (src) return src;
  if (!isFile(file)) throw new AuditError(`Không tìm thấy file: ${relOf(ctx, file)}`);
  const text = fs.readFileSync(file, 'utf8');
  const sf = ts.createSourceFile(file, text, ts.ScriptTarget.Latest, true, ts.ScriptKind.TS);
  if (sf.parseDiagnostics?.length) {
    const d = sf.parseDiagnostics[0];
    throw new AuditError(`Lỗi cú pháp ${relOf(ctx, file)}: ${ts.flattenDiagnosticMessageText(d.messageText, '\n')}`);
  }
  const imports = new Map(); // localName -> { spec, exported }
  const namedArrays = new Map(); // exportName/localName -> initializer
  let defaultExpr = null;
  for (const st of sf.statements) {
    if (ts.isImportDeclaration(st) && ts.isStringLiteral(st.moduleSpecifier) && st.importClause) {
      const spec = st.moduleSpecifier.text;
      const clause = st.importClause;
      if (clause.name) imports.set(clause.name.text, { spec, exported: 'default' });
      if (clause.namedBindings && ts.isNamedImports(clause.namedBindings)) {
        for (const el of clause.namedBindings.elements) {
          imports.set(el.name.text, { spec, exported: (el.propertyName ?? el.name).text });
        }
      }
    } else if (ts.isExportAssignment(st) && !st.isExportEquals) {
      defaultExpr = st.expression;
    } else if (ts.isVariableStatement(st)) {
      for (const decl of st.declarationList.declarations) {
        if (ts.isIdentifier(decl.name) && decl.initializer) namedArrays.set(decl.name.text, decl.initializer);
      }
    }
  }
  src = { file, sf, text, imports, namedArrays, defaultExpr };
  ctx.sources.set(file, src);
  return src;
}

function isFile(p) {
  try {
    return fs.statSync(p).isFile();
  } catch {
    return false;
  }
}

/** Alias `@/` = `src/app/`; tương đối theo file hiện tại; thử nguyên văn, `.ts`, `/index.ts`. */
function resolveSpec(ctx, fromFile, spec) {
  let base;
  if (spec.startsWith('@/')) base = path.join(ctx.srcApp, spec.slice(2));
  else if (spec.startsWith('.')) base = path.resolve(path.dirname(fromFile), spec);
  else return null;
  for (const cand of [base + '.ts', base, path.join(base, 'index.ts')]) {
    if (isFile(cand)) return cand;
  }
  return null;
}

function unwrap(expr) {
  let e = expr;
  while (
    e &&
    (ts.isAsExpression(e) || ts.isSatisfiesExpression(e) || ts.isParenthesizedExpression(e) || ts.isTypeAssertionExpression(e))
  ) {
    e = e.expression;
  }
  return e;
}

/** Tìm `import('…')` + tên export trong `.then((m) => m.X)` (không có `.then` ⇒ 'default'). */
function readLazyImport(ctx, sf, node, key) {
  let spec = null;
  let exported = 'default';
  const visit = (n) => {
    if (ts.isCallExpression(n)) {
      if (n.expression.kind === ts.SyntaxKind.ImportKeyword) {
        const arg = n.arguments[0];
        if (!arg || !ts.isStringLiteralLike(arg)) {
          throw new AuditError(`${where(ctx, sf, n)}: ${key} dùng import() không phải chuỗi literal`);
        }
        spec = arg.text;
      } else if (ts.isPropertyAccessExpression(n.expression) && n.expression.name.text === 'then') {
        const fn = n.arguments[0];
        if (fn && (ts.isArrowFunction(fn) || ts.isFunctionExpression(fn))) {
          let body = fn.body;
          if (ts.isBlock(body)) {
            const ret = body.statements.find(ts.isReturnStatement);
            body = ret?.expression;
          }
          body = unwrap(body);
          if (body && ts.isPropertyAccessExpression(body)) exported = body.name.text;
          else throw new AuditError(`${where(ctx, sf, n)}: ${key} .then(...) không đọc được tên export`);
        }
      }
    }
    ts.forEachChild(n, visit);
  };
  visit(node);
  if (!spec) throw new AuditError(`${where(ctx, sf, node)}: ${key} không có import('…')`);
  const file = resolveSpec(ctx, sf.fileName, spec);
  if (!file) throw new AuditError(`${where(ctx, sf, node)}: ${key} không resolve được import('${spec}')`);
  return { file, exported };
}

function propName(ctx, sf, prop) {
  if (ts.isIdentifier(prop.name) || ts.isStringLiteral(prop.name)) return prop.name.text;
  throw new AuditError(`${where(ctx, sf, prop)}: tên thuộc tính route không đọc được`);
}

// ── Route parsing ─────────────────────────────────────────────────────────────────
function getRoutesArray(ctx, file, exported, stack) {
  const key = `${file}#${exported}`;
  if (stack.includes(key)) throw new AuditError(`Vòng lặp loadChildren: ${relOf(ctx, file)}#${exported}`);
  const src = loadSource(ctx, file);
  let arr = unwrap(exported === 'default' ? src.defaultExpr : src.namedArrays.get(exported));
  // `const xRoutes: Routes = [...]; export default xRoutes;` (vd teamchat.routes.ts)
  if (arr && ts.isIdentifier(arr)) {
    const name = arr.text;
    const imp = src.imports.get(name);
    if (imp) {
      const target = resolveSpec(ctx, file, imp.spec);
      if (!target) throw new AuditError(`${relOf(ctx, file)}: không resolve được import '${imp.spec}' cho ${name}`);
      return getRoutesArray(ctx, target, imp.exported, [...stack, key]);
    }
    arr = unwrap(src.namedArrays.get(name));
  }
  if (!arr || !ts.isArrayLiteralExpression(arr)) {
    throw new AuditError(`${relOf(ctx, file)}: không tìm thấy mảng Routes cho export '${exported}'`);
  }
  return parseRoutesArray(ctx, src, arr, [...stack, key]);
}

function parseRoutesArray(ctx, src, arr, stack) {
  const routes = [];
  for (const el of arr.elements) {
    if (ts.isObjectLiteralExpression(el)) {
      routes.push(parseRoute(ctx, src, el, stack));
    } else if (ts.isSpreadElement(el) && ts.isIdentifier(unwrap(el.expression))) {
      const name = unwrap(el.expression).text;
      const imp = src.imports.get(name);
      if (imp) {
        const file = resolveSpec(ctx, src.file, imp.spec);
        if (!file) throw new AuditError(`${where(ctx, src.sf, el)}: không resolve được import '${imp.spec}' cho ...${name}`);
        routes.push(...getRoutesArray(ctx, file, imp.exported, stack));
      } else if (src.namedArrays.has(name)) {
        routes.push(...getRoutesArray(ctx, src.file, name, stack));
      } else {
        throw new AuditError(`${where(ctx, src.sf, el)}: spread ...${name} không tìm thấy nguồn`);
      }
    } else {
      throw new AuditError(`${where(ctx, src.sf, el)}: phần tử Routes không hỗ trợ (${ts.SyntaxKind[el.kind]})`);
    }
  }
  return routes;
}

function readLiteral(ctx, sf, node, key, allowExpr) {
  const v = unwrap(node);
  if (ts.isStringLiteralLike(v)) return v.text;
  if (v.kind === ts.SyntaxKind.NullKeyword) return null;
  if (v.kind === ts.SyntaxKind.TrueKeyword) return true;
  if (v.kind === ts.SyntaxKind.FalseKeyword) return false;
  if (ts.isNumericLiteral(v)) return Number(v.text);
  if (allowExpr) return { expr: exprText(sf, v) };
  throw new AuditError(`${where(ctx, sf, node)}: '${key}' phải là chuỗi literal hoặc null`);
}

function parseRoute(ctx, src, obj, stack) {
  const { sf } = src;
  const r = {
    file: relOf(ctx, src.file),
    line: lineOf(sf, obj),
    path: undefined,
    component: null, // "<file rel>#<export>"
    componentFile: null,
    hasComponent: false,
    guards: [],
    data: {},
    dataLines: {},
    redirect: false,
    children: null,
    lazyChildren: false,
  };
  for (const prop of obj.properties) {
    if (!ts.isPropertyAssignment(prop)) {
      throw new AuditError(`${where(ctx, sf, prop)}: thuộc tính route không hỗ trợ (${ts.SyntaxKind[prop.kind]})`);
    }
    const name = propName(ctx, sf, prop);
    const init = prop.initializer;
    switch (name) {
      case 'path': {
        const v = unwrap(init);
        if (!ts.isStringLiteralLike(v)) throw new AuditError(`${where(ctx, sf, prop)}: 'path' phải là chuỗi literal`);
        r.path = v.text;
        break;
      }
      case 'component': {
        const v = unwrap(init);
        const imp = ts.isIdentifier(v) ? src.imports.get(v.text) : null;
        const file = imp ? resolveSpec(ctx, src.file, imp.spec) : null;
        if (!file) throw new AuditError(`${where(ctx, sf, prop)}: 'component' không resolve được về file import`);
        r.component = `${relOf(ctx, file)}#${imp.exported}`;
        r.componentFile = file;
        r.hasComponent = true;
        break;
      }
      case 'loadComponent': {
        const { file, exported } = readLazyImport(ctx, sf, init, 'loadComponent');
        r.component = `${relOf(ctx, file)}#${exported}`;
        r.componentFile = file;
        r.hasComponent = true;
        break;
      }
      case 'loadChildren': {
        const { file, exported } = readLazyImport(ctx, sf, init, 'loadChildren');
        r.children = getRoutesArray(ctx, file, exported, stack);
        r.lazyChildren = true;
        break;
      }
      case 'children': {
        const v = unwrap(init);
        if (!ts.isArrayLiteralExpression(v)) throw new AuditError(`${where(ctx, sf, prop)}: 'children' phải là mảng literal`);
        r.children = parseRoutesArray(ctx, src, v, stack);
        break;
      }
      case 'redirectTo':
        r.redirect = true;
        break;
      case 'data': {
        const v = unwrap(init);
        if (!ts.isObjectLiteralExpression(v)) throw new AuditError(`${where(ctx, sf, prop)}: 'data' phải là object literal`);
        for (const dp of v.properties) {
          if (!ts.isPropertyAssignment(dp)) {
            throw new AuditError(`${where(ctx, sf, dp)}: thuộc tính data không hỗ trợ (${ts.SyntaxKind[dp.kind]})`);
          }
          const dk = propName(ctx, sf, dp);
          const strict = dk === 'breadcrumb' || dk === 'routerLink';
          r.data[dk] = readLiteral(ctx, sf, dp.initializer, dk, !strict);
          r.dataLines[dk] = lineOf(sf, dp);
        }
        break;
      }
      default:
        if (GUARD_KEYS.includes(name)) {
          const v = unwrap(init);
          if (!ts.isArrayLiteralExpression(v)) throw new AuditError(`${where(ctx, sf, prop)}: '${name}' phải là mảng literal`);
          const prefix = name === 'canActivate' ? '' : `${name}:`;
          for (const g of v.elements) r.guards.push(prefix + exprText(sf, g));
        }
      // pathMatch, title, providers… không ảnh hưởng breadcrumb/guard ⇒ bỏ qua
    }
  }
  if (r.path === undefined) throw new AuditError(`${r.file}:${r.line}: route thiếu 'path'`);
  return r;
}

// ── Mô phỏng router + breadcrumb ────────────────────────────────────────────────────
/**
 * Kế thừa data — theo runtime đo được (architecture.md "Đính chính lần 2"): con LUÔN merge `data` của cha
 * (con ghi đè) ở mọi tầng. Root data = {} ⇒ route gốc không nhận gì thêm.
 */
function walkTree(roots) {
  const nodes = []; // mọi route (kể cả không phải lá)
  const leaves = [];
  const rootCtx = {
    data: {}, routerLinkSrc: null,
    urlSegs: [], crumbs: [], guards: [], depth: 0, branch: null,
  };
  const visit = (route, parent) => {
    // Quy tắc kế thừa đo từ RUNTIME (router snapshot thật trên dev server 2026-09-15 — architecture.md
    // "Đính chính lần 2"): con LUÔN kế thừa (merge, con ghi đè) `data` cha ở MỌI tầng, bất kể path/component/
    // loadChildren — Angular 22.0.8 mặc định `paramsInheritanceStrategy: 'always'`
    // (`node_modules/@angular/router/fesm2022/_router-chunk.mjs:1498`), app không cấu hình lại. KHÔNG chắp vế theo từng ca.
    const data = { ...parent.data, ...route.data };
    const ownRouterLink = Object.prototype.hasOwnProperty.call(route.data, 'routerLink');
    const routerLinkSrc = ownRouterLink
      ? { file: route.file, line: route.dataLines.routerLink ?? route.line }
      : parent.routerLinkSrc;
    const urlSegs = parent.urlSegs.concat(route.path === '' ? [] : route.path.split('/'));
    const depth = parent.depth + 1;
    const branch = depth === 1 ? (BREADCRUMB_ROOTS.has(route.path) ? route.path : null) : parent.branch;
    const crumbs = parent.crumbs.slice();
    const bc = data.breadcrumb;
    if (bc && bc !== parent.data.breadcrumb) {
      crumbs.push({
        key: bc,
        url: data.routerLink || '/' + urlSegs.join('/'),
        routerLink: data.routerLink || null,
        // href đến từ routerLink kế thừa (route này không tự khai) ⇒ nguồn B7
        inheritedRouterLinkSrc: data.routerLink && !ownRouterLink ? routerLinkSrc : null,
        nodeUrl: '/' + urlSegs.join('/'),
        file: route.file,
        line: route.dataLines.breadcrumb ?? route.line,
        depth,
        isWrapper: route.path === '' && !!route.children && !route.lazyChildren,
      });
    }
    // Guard đọc `route.data` (permissionGuard) nhận data của CHÍNH route khai guard ⇒ gắn permission hiệu lực
    // tại route đó (không ghi tầng: gom C3 hợp lệ dời guard lên cha vẫn ra cùng text; dời guard lên cha mà
    // bỏ permission ở lá ⇒ `{permission=-}` ⇒ diff baseline lộ ra).
    const guards = parent.guards.concat(route.guards.map((g) => annotateGuard(g, data)));
    const url = '/' + urlSegs.join('/');
    const ctx = {
      data, routerLinkSrc,
      urlSegs, crumbs, guards, depth, branch,
    };
    const node = { route, parentData: parent.data, data, url, urlSegs, crumbs, guards, depth, branch };
    nodes.push(node);
    if (route.hasComponent && !route.children && !route.redirect) leaves.push(node);
    for (const child of route.children ?? []) visit(child, ctx);
  };
  for (const r of roots) visit(r, rootCtx);
  return { nodes, leaves };
}

/** Guard đọc `route.data.permission` của route khai nó (`src/app/core/guards/permissionGuard.ts`). */
const DATA_PERMISSION_GUARD_RE = /^permissionGuard\b/;

function annotateGuard(guardText, data) {
  if (!DATA_PERMISSION_GUARD_RE.test(guardText)) return guardText;
  const perm = data.permission;
  const text = perm === undefined ? '-' : typeof perm === 'object' && perm ? perm.expr : JSON.stringify(perm);
  return `${guardText}{permission=${text}}`;
}

function i18nHas(dict, key) {
  let cur = dict;
  for (const part of key.split('.')) {
    if (cur === null || typeof cur !== 'object' || !Object.prototype.hasOwnProperty.call(cur, part)) return false;
    cur = cur[part];
  }
  return typeof cur === 'string';
}

const hasParam = (segs) => segs.some((s) => s.startsWith(':'));
const isCreate = (segs) => segs[segs.length - 1] === 'create';

/** Khớp URL crumb với pattern lá: `:x` ở lá khớp mọi segment; `:x` ở crumb chỉ khớp `:y` ở lá. */
function urlMatchesLeaf(crumbUrl, leafSegs) {
  const segs = crumbUrl.split(/[?#]/)[0].split('/').filter(Boolean);
  if (segs.length !== leafSegs.length) return false;
  return segs.every((s, i) => leafSegs[i].startsWith(':') || (!s.startsWith(':') && s === leafSegs[i]));
}

const chainText = (crumbs) => (crumbs.length ? crumbs.map((c) => c.key).join(' > ') : '-');

function checkTree(ctx, tree, dict) {
  const issues = [];
  const seen = new Set();
  const add = (code, file, line, url, crumbs, detail) => {
    const k = `${code}|${file}:${line}|${url}`;
    if (seen.has(k)) return;
    seen.add(k);
    issues.push({ code, location: `${file}:${line}`, route: url, chain: chainText(crumbs), detail });
  };
  const matchable = tree.leaves.filter((l) => !l.urlSegs.includes('**'));

  for (const n of tree.nodes) {
    if (!n.branch) continue;
    const r = n.route;
    const own = r.data.breadcrumb;
    const bLine = r.dataLines.breadcrumb ?? r.line;
    if (typeof own === 'string') {
      if (!KEY_RE.test(own)) add('B1', r.file, bLine, n.url, n.crumbs, `'${own}' không phải i18n key`);
      else if (!i18nHas(dict, own)) add('B1', r.file, bLine, n.url, n.crumbs, `'${own}' không có trong vi-VN.json`);
      if (r.path !== '' && own && own === n.parentData.breadcrumb) {
        add('B5', r.file, bLine, n.url, n.crumbs, `'${own}' trùng breadcrumb của cha ⇒ không hiển thị`);
      }
    }
  }

  for (const l of tree.leaves) {
    if (!l.branch) continue;
    const r = l.route;
    const first = l.crumbs[0];
    const moduleLevel =
      first && (first.depth === 1 || (MODULE_WRAPPER_ROOTS.has(l.branch) && first.depth === 2 && first.isWrapper));
    if (!moduleLevel) {
      add('B2', r.file, r.line, l.url, l.crumbs, first ? `crumb đầu '${first.key}' (${first.file}:${first.line}) không phải cấp module` : 'không có crumb nào');
    }
    const param = hasParam(l.urlSegs) && !isCreate(l.urlSegs);
    if (param && l.crumbs.length < 3) add('B3', r.file, r.line, l.url, l.crumbs, `${l.crumbs.length} crumb`);
    l.crumbs.slice(0, -1).forEach((c) => {
      if (!matchable.some((m) => urlMatchesLeaf(c.url, m.urlSegs))) {
        const k = `B4|${c.file}:${c.line}|${c.url}`;
        if (seen.has(k)) return;
        seen.add(k);
        issues.push({
          code: 'B4',
          location: `${c.file}:${c.line}`,
          route: l.url,
          chain: chainText(l.crumbs),
          detail: `crumb '${c.key}' → ${c.url}${c.routerLink ? ' (routerLink)' : ''} không khớp lá nào`,
        });
      }
    });
    // B7: mục không-cuối render link ⇒ href kế thừa trỏ nhầm trang đích của tổ tiên (B4 không bắt vì vẫn là lá hợp lệ)
    l.crumbs.slice(0, -1).forEach((c) => {
      if (!c.inheritedRouterLinkSrc) return;
      const k = `B7|${c.file}:${c.line}|${c.nodeUrl}`;
      if (seen.has(k)) return;
      seen.add(k);
      const src = c.inheritedRouterLinkSrc;
      issues.push({
        code: 'B7',
        location: `${c.file}:${c.line}`,
        route: c.nodeUrl,
        chain: chainText(l.crumbs),
        detail: `crumb '${c.key}' href ${c.url} kế thừa routerLink khai ở ${src.file}:${src.line} (lá ${l.url}) — cần tự khai routerLink`,
      });
    });
    if (param) {
      const text = fs.readFileSync(r.componentFile, 'utf8');
      if (!text.includes(LABEL_SERVICE)) {
        add('B6', r.file, r.line, l.url, l.crumbs, `${r.component} không dùng ${LABEL_SERVICE}`);
      }
    }
  }
  issues.sort((a, b) => cmp(a.code, b.code) || cmp(a.location, b.location) || cmp(a.route, b.route));
  return issues;
}

const cmp = (a, b) => (a < b ? -1 : a > b ? 1 : 0);

function dumpLines(tree, withCrumbs) {
  return tree.leaves
    .map((l) => {
      const guards = l.guards.length ? l.guards.join(', ') : '-';
      const base = `${l.url} | ${l.route.component} | ${guards}`;
      // cột crumb in `key→href` từng mục (soát link không cần mô phỏng tay); --map-only-routing không đổi
      const crumbText = l.crumbs.length ? l.crumbs.map((c) => `${c.key}→${c.url}`).join(' > ') : '-';
      return withCrumbs ? `${base} | ${crumbText}` : base;
    })
    .sort(cmp);
}

// ── Audit API ───────────────────────────────────────────────────────────────────────
function readAllow(file, required) {
  if (!isFile(file)) {
    if (required) throw new AuditError(`Không tìm thấy allowlist: ${file}`);
    return [];
  }
  let list;
  try {
    list = JSON.parse(fs.readFileSync(file, 'utf8'));
  } catch (e) {
    throw new AuditError(`Allowlist JSON lỗi (${file}): ${e.message}`);
  }
  if (!Array.isArray(list) || list.some((x) => !x || !CODES.includes(x.code) || typeof x.route !== 'string' || typeof x.reason !== 'string')) {
    throw new AuditError(`Allowlist sai định dạng (${file}): cần mảng {code: ${CODES[0]}..${CODES[CODES.length - 1]}, route, reason}`);
  }
  return list;
}

export function audit({ root = DEFAULT_ROOT, allowFile = null } = {}) {
  const ctx = createContext(root);
  const appRoutesFile = path.join(root, 'src', 'app.routes.ts');
  const roots = getRoutesArray(ctx, appRoutesFile, 'appRoutes', []);
  const dictFile = path.join(root, 'src', 'assets', 'i18n', 'vi-VN.json');
  if (!isFile(dictFile)) throw new AuditError(`Không tìm thấy ${relOf(ctx, dictFile)}`);
  const dict = JSON.parse(fs.readFileSync(dictFile, 'utf8'));
  const tree = walkTree(roots);
  const all = checkTree(ctx, tree, dict);

  const allowPath = allowFile ?? (root === DEFAULT_ROOT ? DEFAULT_ALLOW_FILE : path.join(root, 'breadcrumb-audit.allow.json'));
  const allow = readAllow(allowPath, root === DEFAULT_ROOT && !allowFile);
  const used = new Set();
  const errors = [];
  const allowed = [];
  for (const is of all) {
    const idx = allow.findIndex((a) => a.code === is.code && a.route === is.route);
    if (idx >= 0) {
      used.add(idx);
      allowed.push({ ...is, reason: allow[idx].reason });
    } else errors.push(is);
  }
  const warnings = allow
    .map((a, i) => ({ ...a, i }))
    .filter((a) => !used.has(a.i))
    .map((a) => `allowlist không còn khớp: ${a.code} ${a.route} (${a.reason})`);
  const counts = Object.fromEntries(CODES.map((c) => [c, errors.filter((e) => e.code === c).length]));
  return { tree, errors, allowed, warnings, counts, stats: { files: ctx.sources.size, routes: tree.nodes.length, leaves: tree.leaves.length } };
}

// ── CLI ─────────────────────────────────────────────────────────────────────────────
function parseArgs(argv) {
  const opts = { root: DEFAULT_ROOT, allowFile: null, dump: false, routingOnly: false, json: false, selftest: false };
  for (let i = 0; i < argv.length; i++) {
    const a = argv[i];
    if (a === '--root') opts.root = path.resolve(argv[++i] ?? '');
    else if (a === '--allow') opts.allowFile = path.resolve(argv[++i] ?? '');
    else if (a === '--dump-map') opts.dump = true;
    else if (a === '--map-only-routing') opts.dump = opts.routingOnly = true;
    else if (a === '--json') opts.json = true;
    else if (a === '--selftest') opts.selftest = true;
    else throw new AuditError(`Tham số không hỗ trợ: ${a}`);
  }
  if (opts.selftest && (opts.dump || opts.json || opts.root !== DEFAULT_ROOT)) {
    throw new AuditError('--selftest không đi kèm --root/--dump-map/--json');
  }
  return opts;
}

function printReport(res, ms) {
  const { errors, allowed, warnings, counts, stats } = res;
  console.log(`breadcrumb-audit: ${stats.files} file route, ${stats.routes} route, ${stats.leaves} lá (${ms}ms)`);
  for (const code of CODES) {
    const list = errors.filter((e) => e.code === code);
    if (!list.length) continue;
    console.log(`\n${code} — ${CODE_TITLES[code]} (${list.length})`);
    for (const e of list) console.log(`  ${e.location}  ${e.route}  [${e.chain}]  ${e.detail}`);
  }
  if (allowed.length) {
    console.log(`\nAllowlist (${allowed.length}):`);
    for (const a of allowed) console.log(`  ${a.code} ${a.location}  ${a.route}  — ${a.reason}`);
  }
  for (const w of warnings) console.warn(`\n⚠ ${w}`);
  const total = errors.length;
  console.log(`\nTổng lỗi: ${total} — ${CODES.map((c) => `${c}=${counts[c]}`).join(' ')}`);
  console.log(total ? '✗ breadcrumb-audit FAIL' : '✓ breadcrumb-audit sạch');
}

function selftest() {
  const failures = [];
  const check = (name, ok, info) => {
    console.log(`${ok ? '✓' : '✗'} ${name}${ok ? '' : ` — ${info}`}`);
    if (!ok) failures.push(name);
  };

  const bad = audit({ root: path.join(FIXTURE_DIR, 'bad') });
  // B5=2: ca `plist/mid/g` chỉ ra B5 khi mô phỏng nhánh kế thừa "cha không component" (F2 QA Pass 1)
  // B7=1: `zalo/camp` kế thừa routerLink của root zalo (cha component+loadChildren) — story-01 reopen 2026-09-15
  const expected = { B1: 3, B2: 1, B3: 1, B4: 1, B5: 2, B6: 1, B7: 1 };
  check('bad: đỏ đúng 7 mã với số lượng mong đợi', CODES.every((c) => bad.counts[c] === expected[c]),
    `thực tế ${JSON.stringify(bad.counts)} / mong đợi ${JSON.stringify(expected)}\n${bad.errors.map((e) => `    ${e.code} ${e.location} ${e.route} [${e.chain}] ${e.detail}`).join('\n')}`);
  check('bad: B1 không đếm dòng bị comment', !bad.errors.some((e) => e.detail.includes('Commented')), 'dòng comment bị đếm');
  check('bad: B5 qua cha không component + mid path khác rỗng (/crm/plist/mid/g)',
    bad.errors.some((e) => e.code === 'B5' && e.route === '/crm/plist/mid/g'), 'không ra B5 ⇒ mô phỏng kế thừa data sai');
  check('bad: B7 mục /zalo/camp không tự khai routerLink, href kế thừa /zalo/user (ca b)',
    bad.errors.some((e) => e.code === 'B7' && e.route === '/zalo/camp' && e.detail.includes('href /zalo/user')),
    'không ra B7 ⇒ mô phỏng kế thừa `always` (mọi tầng, kể cả loadChildren) sai hoặc check B7 tắt');

  const clean = audit({ root: path.join(FIXTURE_DIR, 'clean') });
  check('clean: 0 lỗi', clean.errors.length === 0,
    clean.errors.map((e) => `${e.code} ${e.location} ${e.route} [${e.chain}] ${e.detail}`).join('; '));
  check('clean: allowlist áp 1 mục B6', clean.allowed.length === 1 && clean.allowed[0].code === 'B6', JSON.stringify(clean.allowed));
  check('clean: mục allowlist lỗi thời ⇒ 1 cảnh báo (không phải lỗi)', clean.warnings.length === 1, JSON.stringify(clean.warnings));

  const map = dumpLines(clean.tree, true);
  // Cột crumb dump-map = `key→href` từng mục.
  const expectChain = {
    '/crm/customer': 'module.crm→/crm > x.customer_list→/crm/customer', // cha không component ⇒ con '' kế thừa ⇒ khử trùng
    '/crm/customer/detail/:id': 'module.crm→/crm > x.customer_list→/crm/customer > x.customer_detail→/crm/customer/detail/:id',
    // `mid`: cha `group` CÓ component + children inline, KHÔNG loadChildren, path khác rỗng ⇒ VẪN kế thừa
    // (runtime, Đính chính lần 2) ⇒ lá nhận routerLink '/crm/group' của group qua `mid`
    '/crm/group/mid/leaf': 'module.crm→/crm > x.group→/crm/group > x.exam→/crm/group',
    '/crm/exam/:id': 'module.crm→/crm > x.reception→/crm/customer > x.exam→/crm/customer', // C3' routerLink (mục cuối kế thừa, không render link)
    '/config/sms': 'module.config→/config > x.sms→/config', // wrapper '' + '' loadChildren kế thừa ⇒ khử trùng
    '/crm': 'module.crm→/crm', // breadcrumb: null chặn kế thừa
    // ca (a): `perm` là con path khác rỗng của root zalo (component+loadChildren) ⇒ kế thừa ⇒ khử trùng
    '/zalo/perm': 'module.zalo→/zalo/user',
    // ca (c): `camp` tự khai routerLink ⇒ không B7
    '/zalo/camp/create': 'module.zalo→/zalo/user > x.camp→/zalo/camp > common.create→/zalo/camp',
  };
  for (const [url, chain] of Object.entries(expectChain)) {
    const line = map.find((l) => l.startsWith(`${url} | `));
    check(`clean: chuỗi crumb ${url} = ${chain}`, !!line && line.endsWith(` | ${chain}`), line ?? 'không có lá');
  }
  const guardLine = map.find((l) => l.startsWith('/crm/income | '));
  check('clean: guards gom tổ tiên + permission tại route khai permissionGuard',
    !!guardLine && guardLine.includes('| authGuard, permissionGuard{permission=P.viewIncome} |'), guardLine ?? 'không có lá');
  // F1 QA Pass 1 — bất biến dump khi dời permissionGuard: so phần guards giữa các biến thể cùng ngữ nghĩa runtime
  const guardsOf = (url) => {
    const line = map.find((l) => l.startsWith(`${url} | `));
    return line ? line.split(' | ')[2] : null;
  };
  // Đính chính lần 2: cha component + children inline (KHÔNG loadChildren), con component thiếu key ⇒ vẫn nhận key cha
  check('clean: /crm/pgroup/child kế thừa permission qua cha component KHÔNG loadChildren (luôn kế thừa)',
    guardsOf('/crm/pgroup/child') === 'authGuard, permissionGuard{permission=P.viewGroup}', String(guardsOf('/crm/pgroup/child')));
  // ca (a) runtime: con path khác rỗng của cha component+loadChildren kế thừa `data.permission` của cha
  check('clean: /zalo/perm kế thừa permission qua cha component+loadChildren (ca a)',
    guardsOf('/zalo/perm') === 'authGuard, permissionGuard{permission=P.viewZalo}', String(guardsOf('/zalo/perm')));
  const flat = guardsOf('/crm/stock-flat');
  const okGuards = 'authGuard, permissionGuard{permission=P.viewStock}';
  check('clean: permissionGuard ở từng lá (phẳng) = baseline mong đợi', flat === okGuards, String(flat));
  for (const url of ['/crm/stock-c3', '/crm/stock-c3/:id', '/crm/stock-leafguard', '/crm/stock-leafguard/:id']) {
    check(`clean: dời hợp lệ ⇒ guards ${url} giống bản phẳng`, guardsOf(url) === okGuards, String(guardsOf(url)));
  }
  for (const url of ['/crm/stock-wrong', '/crm/stock-wrong/:id']) {
    const g = guardsOf(url);
    check(`clean: guard ở cha, permission ở lá ⇒ guards ${url} KHÁC bản phẳng`,
      g !== null && g !== okGuards && g.includes('permissionGuard{permission=-}'), String(g));
  }
  const again = dumpLines(audit({ root: path.join(FIXTURE_DIR, 'clean') }).tree, true);
  check('clean: dump-map ổn định giữa 2 lần chạy', JSON.stringify(map) === JSON.stringify(again), 'khác nhau');

  console.log(failures.length ? `\n✗ selftest FAIL (${failures.length})` : '\n✓ selftest PASS');
  return failures.length ? 1 : 0;
}

function main() {
  const started = Date.now();
  try {
    const opts = parseArgs(process.argv.slice(2));
    if (opts.selftest) return selftest();
    const res = audit({ root: opts.root, allowFile: opts.allowFile });
    if (opts.dump) {
      process.stdout.write(dumpLines(res.tree, !opts.routingOnly).join('\n') + '\n');
      return 0;
    }
    if (opts.json) {
      const { errors, allowed, warnings, counts, stats } = res;
      process.stdout.write(JSON.stringify({ counts, errors, allowed, warnings, stats }, null, 2) + '\n');
      return errors.length ? 1 : 0;
    }
    printReport(res, Date.now() - started);
    return res.errors.length ? 1 : 0;
  } catch (e) {
    if (e instanceof AuditError) {
      console.error(`✗ breadcrumb-audit: ${e.message}`);
      return 1;
    }
    throw e;
  }
}

if (process.argv[1] && path.resolve(process.argv[1]) === fileURLToPath(import.meta.url)) {
  process.exitCode = main();
}
