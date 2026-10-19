#!/usr/bin/env node
/**
 * Bench script — Customer Detail page performance.
 *
 * Mục đích: đo bundle size + chunk count cho trang `/crm/customer/detail/:id`
 * sau khi áp dụng optimizations (@defer 11 tabs, structuredClone, parallel cache load, ...).
 *
 * Usage:
 *   1. Build production: `npm run build` (sinh dist/ + stats.json)
 *   2. Chạy script: `node scripts/bench-customer-detail.mjs`
 *
 * Để có stats.json đầy đủ, sửa build script: `ng build --stats-json`.
 */

import { readdir, stat } from 'node:fs/promises';
import { join } from 'node:path';

const DIST_DIR = './dist/cluvix/browser';
const TARGETS = [
  // Các chunk dự kiến lazy-load qua @defer cho 11 tab customer-detail.
  'appointment-in-customer',
  'check-up-of-customer',
  'order-plan-in-customer',
  'payment-of-customer',
  'treatment-course',
  'labo-order-list-customer',
  'warranty-of-customer',
  'care-for-customer',
  'reminder-of-customer',
  'prescription-of-customer',
];

async function findFiles(dir) {
  const out = [];
  const entries = await readdir(dir, { withFileTypes: true });
  for (const e of entries) {
    const path = join(dir, e.name);
    if (e.isDirectory()) {
      out.push(...(await findFiles(path)));
    } else if (e.name.endsWith('.js') || e.name.endsWith('.css')) {
      const s = await stat(path);
      out.push({ path, size: s.size, name: e.name });
    }
  }
  return out;
}

function fmt(bytes) {
  if (bytes < 1024) return `${bytes} B`;
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`;
  return `${(bytes / 1024 / 1024).toFixed(2)} MB`;
}

(async () => {
  try {
    const files = await findFiles(DIST_DIR);
    files.sort((a, b) => b.size - a.size);

    console.log('=== Customer Detail — Bundle Analysis ===\n');

    const mainChunks = files.filter((f) => /^main(\.|-)/.test(f.name) || /^polyfills/.test(f.name));
    console.log('Main bundle (loaded eager):');
    for (const f of mainChunks) console.log(`  ${fmt(f.size).padStart(10)}  ${f.name}`);

    console.log('\nDeferred customer-detail chunks (target — should NOT be in main):');
    let foundCount = 0;
    for (const target of TARGETS) {
      const match = files.find((f) => f.name.toLowerCase().includes(target));
      if (match) {
        foundCount++;
        console.log(`  ${fmt(match.size).padStart(10)}  ${match.name}`);
      } else {
        console.log(`  ${'(missing)'.padStart(10)}  ${target}*  — could be inlined in main`);
      }
    }
    console.log(`\nDeferred chunks found: ${foundCount}/${TARGETS.length}`);

    const total = files.reduce((s, f) => s + f.size, 0);
    const mainTotal = mainChunks.reduce((s, f) => s + f.size, 0);
    console.log(`\nTotals:`);
    console.log(`  Main+polyfills: ${fmt(mainTotal)}`);
    console.log(`  All assets:     ${fmt(total)}`);
    console.log(`  Lazy ratio:     ${((1 - mainTotal / total) * 100).toFixed(1)}%`);

    console.log('\n=== Lighthouse manual run ===');
    console.log('Throttle: Slow 4G + 4x CPU. Target metrics:');
    console.log('  LCP  < 2.5s    — Largest Contentful Paint');
    console.log('  TBT  < 200ms   — Total Blocking Time');
    console.log('  FCP  < 1.8s    — First Contentful Paint');
    console.log('  TTI  < 3.8s    — Time to Interactive');
    console.log('\nĐiểm cần đo: /crm/customer/detail/{id} với customer có ≥3 orders.');
  } catch (err) {
    console.error('Bench failed:', err.message);
    console.error('Hint: chạy `npm run build` trước để sinh dist/');
    process.exit(1);
  }
})();
