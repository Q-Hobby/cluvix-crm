#!/usr/bin/env node
/**
 * i18n mapping builder — Phase 2.
 *
 * Đọc i18n-dedup/report.json, sinh i18n-dedup/mapping.json — source of truth cho
 * codemod (Phase 4) + locale prune (Phase 5).
 *
 * Thuật toán (bảo thủ — KHÔNG bao giờ gộp nhầm 2 nghĩa khác nhau):
 *   1. Loại trừ PROTECTED prefix (key động, codemod không thấy được).
 *   2. Mỗi nhóm trùng value vi → sub-cluster theo "meaning signature" = EN chuẩn hoá
 *      (lowercase + singularize). Customer/Customers → 1 cluster; Payment/Billing → 2 cluster.
 *   3. Sub-cluster size>=2 = trùng thật → 1 canonical key. Size 1 = nghĩa riêng → để nguyên.
 *   4. Canonical key: ưu tiên reuse key common.* đã tồn tại trong cluster; else mint
 *      common.<bucket>.<slug>. Translation = bản phổ biến nhất mỗi locale.
 *
 * Output mapping.json: { stats, canonical: {key:{vi,en,ja,fil,isNew,bucket}}, remap: {old:{to,tier}} }
 */
import { promises as fs } from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const FE_ROOT = path.resolve(__dirname, '..');
const I18N_DIR = path.join(FE_ROOT, 'src/assets/i18n');
const OUT = path.join(FE_ROOT, 'i18n-dedup');

const PROTECTED_PREFIXES = ['define.emr.record_type.'];

const flat = (o, p = '', out = {}) => {
  for (const [k, v] of Object.entries(o ?? {})) {
    const np = p ? `${p}.${k}` : k;
    if (v && typeof v === 'object' && !Array.isArray(v)) flat(v, np, out);
    else out[np] = v;
  }
  return out;
};

// ASCII-fold Vietnamese + lower for slug/signature
const fold = (s) => s.normalize('NFD').replace(/[̀-ͯ]/g, '')
  .replace(/đ/g, 'd').replace(/Đ/g, 'D');

// Naive English singularize — chỉ để tạo signature gom số ít/nhiều, KHÔNG hiển thị.
function singular(word) {
  if (word.length <= 3) return word;
  if (/ies$/.test(word)) return word.slice(0, -3) + 'y';
  if (/(ses|xes|zes|ches|shes)$/.test(word)) return word.slice(0, -2);
  if (/ss$/.test(word)) return word;
  if (/s$/.test(word)) return word.slice(0, -1);
  return word;
}
// Meaning signature từ EN: lowercase, bỏ dấu câu, singularize từng từ.
function enSignature(en) {
  if (!en) return '∅';
  return fold(en).toLowerCase().replace(/[^a-z0-9\s]/g, ' ').trim()
    .split(/\s+/).map(singular).join(' ');
}

function slugFromEN(en) {
  return fold(en).toLowerCase().replace(/[^a-z0-9]+/g, '_').replace(/^_+|_+$/g, '').slice(0, 48) || 'item';
}

const ACTION_VERBS = new Set(['save','cancel','delete','update','create','add','edit','search','close','clear','export','import','print','view','reset','submit','confirm','remove','assign','clone','reject','restore','approve','send','copy','download','upload','filter','refresh','back','next','select','apply','duplicate','share','login','logout','register']);
const STATUS_WORDS = new Set(['active','inactive','pending','completed','cancelled','canceled','draft','approved','rejected','paid','unpaid','processing','new','done','expired','open','closed','enabled','disabled','success','failed','error','waiting','confirmed','arrived','absent','present']);

// Bucket suy từ NỘI DUNG EN (không dựa tên key — tên key như `payment_status`
// đánh lừa: nghĩa là label "Payment" chứ không phải 1 trạng thái).
function inferBucket(canonicalEN) {
  const en = (canonicalEN || '').toLowerCase().trim();
  const words = en.split(/\s+/);
  if (words.length === 1 && STATUS_WORDS.has(words[0])) return 'status';
  if (words.length === 1 && ACTION_VERBS.has(words[0])) return 'action';
  return 'label';
}

function mostFrequent(arr) {
  const m = new Map();
  for (const x of arr) if (x != null) m.set(x, (m.get(x) || 0) + 1);
  if (!m.size) return null;
  return [...m.entries()].sort((a, b) =>
    b[1] - a[1] || String(a[0]).length - String(b[0]).length || String(a[0]).localeCompare(String(b[0])))[0][0];
}

(async () => {
  const { groups } = JSON.parse(await fs.readFile(path.join(OUT, 'report.json'), 'utf8'));
  const vi = flat(JSON.parse(await fs.readFile(path.join(I18N_DIR, 'vi-VN.json'), 'utf8')));
  const en = flat(JSON.parse(await fs.readFile(path.join(I18N_DIR, 'en-US.json'), 'utf8')));

  const isProtected = (p) => PROTECTED_PREFIXES.some(pre => p.startsWith(pre));

  // Index existing common.* keys → meaning signature (để reuse / tránh đè nhầm).
  const existingCommon = new Map(); // key → enSignature(existing en)
  for (const p of Object.keys(vi)) if (p.startsWith('common.')) existingCommon.set(p, enSignature(en[p]));

  const canonical = {};      // key → {vi,en,ja,fil,isNew,bucket}
  const remap = {};          // oldPath → {to, tier}
  const usedSlugs = new Map(); // slug-key → meaning signature (collision guard)

  let safeRemap = 0, riskyRemap = 0, clustersSplit = 0, singletonsKept = 0, reusedExisting = 0, minted = 0;

  for (const g of groups) {
    const paths = g.perPath.filter(pp => !isProtected(pp.path));
    if (paths.length < 2) continue;

    // sub-cluster theo meaning signature
    const clusters = new Map();
    for (const pp of paths) {
      const sig = enSignature(pp.en);
      if (!clusters.has(sig)) clusters.set(sig, []);
      clusters.get(sig).push(pp);
    }
    if (clusters.size > 1) clustersSplit++;

    for (const [sig, members] of clusters) {
      if (members.length < 2) { singletonsKept++; continue; } // nghĩa riêng → để nguyên

      // chọn canonical key
      const existingInCluster = members.map(m => m.path).filter(p => p.startsWith('common.'))
        .sort((a, b) => a.length - b.length);
      const canEN = mostFrequent(members.map(m => m.en));
      const bucket = inferBucket(canEN);

      // slug "đã chiếm" nếu: (a) mint trong run này cho sig khác, HOẶC
      // (b) là key common.* sẵn có nhưng nghĩa khác. Cùng sig → reuse được.
      const taken = (slug) => {
        if (usedSlugs.has(slug) && usedSlugs.get(slug) !== sig) return true;
        if (existingCommon.has(slug) && existingCommon.get(slug) !== sig) return true;
        return false;
      };

      let key, reuseExistingValue = false;
      if (existingInCluster.length) {
        key = existingInCluster[0];           // 1 member đã là common.* → reuse luôn
        reuseExistingValue = true;
        reusedExisting++;
      } else {
        let base = `common.${bucket}.${slugFromEN(canEN)}`, slug = base, n = 1;
        while (taken(slug)) { n++; slug = `${base}_${n}`; }
        key = slug;
        if (existingCommon.has(slug) && existingCommon.get(slug) === sig) {
          reuseExistingValue = true;          // trùng key common.* cùng nghĩa → reuse, giữ value cũ
          reusedExisting++;
        } else {
          minted++;
        }
        usedSlugs.set(slug, sig);
      }

      // Reuse key sẵn có → KHÔNG ghi đè value cũ (giữ bản dịch hiện hành).
      if (!reuseExistingValue) {
        const cand = {
          vi: g.value,
          en: canEN,
          ja: mostFrequent(members.map(m => m.ja)),
          fil: mostFrequent(members.map(m => m.fil)),
          isNew: !existingCommon.has(key),
          bucket,
        };
        // Nhiều nhóm vi-value cùng EN-signature dồn về 1 key (vd "Thanh toán"/"thanh toán").
        // Groups sort desc theo size → ưu tiên giữ value nhóm LỚN; chỉ override khi
        // candidate có chữ-hoa-đầu mà bản hiện tại viết thường (sửa lỗi mất viết hoa).
        const cur = canonical[key];
        const upperFirst = (s) => { const c = (s || '')[0] || ''; return c !== c.toLowerCase() && c === c.toUpperCase(); };
        if (!cur || (upperFirst(cand.vi) && !upperFirst(cur.vi))) canonical[key] = cand;
      }

      const tier = g.safe ? 'safe' : 'risky';
      for (const m of members) {
        if (m.path === key) continue; // canonical chính nó
        remap[m.path] = { to: key, tier };
        if (tier === 'safe') safeRemap++; else riskyRemap++;
      }
    }
  }

  const stats = {
    totalRemap: Object.keys(remap).length,
    safeRemap, riskyRemap,
    canonicalKeys: Object.keys(canonical).length,
    reusedExisting, minted,
    clustersSplitByMeaning: clustersSplit,
    singletonMeaningsKept: singletonsKept,
    protectedPrefixes: PROTECTED_PREFIXES,
  };
  await fs.writeFile(path.join(OUT, 'mapping.json'), JSON.stringify({ stats, canonical, remap }, null, 2));
  console.log('=== mapping stats ===');
  console.log(JSON.stringify(stats, null, 2));
  console.log(`\nWrote ${OUT}/mapping.json`);
})().catch(e => { console.error(e); process.exit(1); });
