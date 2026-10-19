import type { DashboardSize, DashboardWidgetDef, DashboardZone } from './dashboard-widgets';

/** Hình dạng `dashboard_layout.config` — hợp đồng với BE (`dashboard_layout/handler/config.go`). */
export const DASHBOARD_LAYOUT_VERSION = 1;
export const DASHBOARD_ZONES: readonly DashboardZone[] = ['top', 'main', 'side'];

export interface DashboardLayoutItem {
  key: string;
  zone: DashboardZone;
  visible: boolean;
  size: DashboardSize;
}

export interface DashboardLayoutConfig {
  version: number;
  items: DashboardLayoutItem[];
}

const isZone = (z: unknown): z is DashboardZone => DASHBOARD_ZONES.includes(z as DashboardZone);
const isSize = (s: unknown): s is DashboardSize => s === 'half' || s === 'full';

function byDefaultPosition(a: DashboardWidgetDef, b: DashboardWidgetDef): number {
  return DASHBOARD_ZONES.indexOf(a.defaultZone) - DASHBOARD_ZONES.indexOf(b.defaultZone) || a.defaultOrder - b.defaultOrder;
}

function defaultItem(w: DashboardWidgetDef): DashboardLayoutItem {
  return { key: w.key, zone: w.defaultZone, visible: w.defaultVisible, size: w.defaultSize };
}

/** Bố cục mặc định hệ thống: ĐỦ mọi khối (khối ẩn vẫn có mặt để giữ vị trí khi bật lại). */
export function systemDefaultLayout(registry: readonly DashboardWidgetDef[]): DashboardLayoutConfig {
  return { version: DASHBOARD_LAYOUT_VERSION, items: [...registry].sort(byDefaultPosition).map(defaultItem) };
}

/** Bố cục chuẩn "Hiện tất cả khối": đủ mọi khối của registry, vị trí/cỡ theo registry (đã sắp để đủ khối vẫn cân). */
export function fullLayout(registry: readonly DashboardWidgetDef[]): DashboardLayoutConfig {
  const base = systemDefaultLayout(registry);
  return { ...base, items: base.items.map((x) => ({ ...x, visible: true })) };
}

/**
 * Gộp bố cục đã lưu với registry hiện tại (PRD FR9):
 *  - `saved` null/sai hình dạng/khác phiên bản ⇒ mặc định hệ thống;
 *  - bỏ key không còn trong registry, bỏ key trùng (giữ lần đầu);
 *  - vùng/cỡ sai ⇒ lấy mặc định của khối;
 *  - khối mới (registry có, saved chưa có) ⇒ chèn vào CUỐI vùng mặc định của nó, theo `defaultVisible`.
 * Thứ tự/vùng/cỡ/ẩn-hiện của các khối đã lưu được giữ nguyên.
 */
export function mergeLayout(saved: unknown, registry: readonly DashboardWidgetDef[]): DashboardLayoutConfig {
  const cfg = saved as Partial<DashboardLayoutConfig> | null | undefined;
  if (!cfg || typeof cfg !== 'object' || cfg.version !== DASHBOARD_LAYOUT_VERSION || !Array.isArray(cfg.items)) {
    return systemDefaultLayout(registry);
  }
  const byKey = new Map(registry.map((w) => [w.key, w]));
  const seen = new Set<string>();
  const items: DashboardLayoutItem[] = [];
  for (const raw of cfg.items as unknown[]) {
    const it = raw as Partial<DashboardLayoutItem> | null;
    const w = it && typeof it.key === 'string' ? byKey.get(it.key) : undefined;
    if (!w || seen.has(w.key)) continue;
    seen.add(w.key);
    items.push({
      key: w.key,
      zone: isZone(it!.zone) ? it!.zone : w.defaultZone,
      visible: typeof it!.visible === 'boolean' ? it!.visible : w.defaultVisible,
      size: isSize(it!.size) ? it!.size : w.defaultSize,
    });
  }
  for (const w of [...registry].sort(byDefaultPosition)) {
    if (seen.has(w.key)) continue;
    // Chèn sau phần tử cuối cùng cùng vùng (giữ items gom theo thứ tự đã lưu).
    let at = -1;
    items.forEach((x, i) => {
      if (x.zone === w.defaultZone) at = i;
    });
    items.splice(at + 1 > 0 ? at + 1 : items.length, 0, defaultItem(w));
    seen.add(w.key);
  }
  return { version: DASHBOARD_LAYOUT_VERSION, items };
}

/** Các khối của 1 vùng theo đúng thứ tự (gồm cả khối ẩn — caller tự lọc `visible`). */
export function zoneItems(cfg: DashboardLayoutConfig, zone: DashboardZone): DashboardLayoutItem[] {
  return cfg.items.filter((x) => x.zone === zone);
}

/** Chữ ký ổn định để so 2 bố cục (dirty check, so bản tạm với API). */
export function layoutSignature(cfg: DashboardLayoutConfig): string {
  return JSON.stringify(cfg.items.map((x) => [x.key, x.zone, x.visible ? 1 : 0, x.size]));
}

// ── Reducer chế độ sửa (story-10). Thuần, KHÔNG mutate `cfg` — luôn trả object mới (OnPush). ──

/**
 * Đặt khối `key` vào `zone` tại `index` tính trên các khối HIỆN của vùng đích, không kể chính nó (= `currentIndex` của
 * CDK drop, cả cùng vùng lẫn khác vùng). `index` ≥ số khối hiện ⇒ cuối vùng. Khối được đặt luôn `visible`.
 * Khối ẩn cùng vùng giữ nguyên chỗ tương đối (bật lại thì về gần vị trí cũ).
 */
export function placeItem(cfg: DashboardLayoutConfig, key: string, zone: DashboardZone, index: number): DashboardLayoutConfig {
  const cur = cfg.items.find((x) => x.key === key);
  if (!cur) return cfg;
  const rest = cfg.items.filter((x) => x.key !== key);
  const targets = rest.filter((x) => x.zone === zone && x.visible);
  const i = Math.max(0, index);
  const at = i < targets.length ? rest.indexOf(targets[i]) : targets.length ? rest.indexOf(targets[targets.length - 1]) + 1 : rest.length;
  rest.splice(at, 0, { ...cur, zone, visible: true });
  return { ...cfg, items: rest };
}

/** Lên/Xuống 1 bậc trong vùng (menu di chuyển — thay kéo thả cho bàn phím/mobile). Ra ngoài biên ⇒ giữ nguyên. */
export function moveBy(cfg: DashboardLayoutConfig, key: string, delta: -1 | 1): DashboardLayoutConfig {
  const cur = cfg.items.find((x) => x.key === key);
  if (!cur || !cur.visible) return cfg;
  const visible = cfg.items.filter((x) => x.zone === cur.zone && x.visible);
  const to = visible.findIndex((x) => x.key === key) + delta;
  return to < 0 || to >= visible.length ? cfg : placeItem(cfg, key, cur.zone, to);
}

/** Bật khối (Kho khối "+"): về CUỐI vùng mặc định của nó (story-10 AC4). Đang hiện ⇒ giữ nguyên. */
export function showItem(cfg: DashboardLayoutConfig, key: string, registry: readonly DashboardWidgetDef[]): DashboardLayoutConfig {
  const w = registry.find((x) => x.key === key);
  const cur = cfg.items.find((x) => x.key === key);
  if (!w || !cur || cur.visible) return cfg;
  return placeItem(cfg, key, w.defaultZone, Number.MAX_SAFE_INTEGER);
}

export function hideItem(cfg: DashboardLayoutConfig, key: string): DashboardLayoutConfig {
  return patchItem(cfg, key, { visible: false });
}

export function setItemSize(cfg: DashboardLayoutConfig, key: string, size: DashboardSize): DashboardLayoutConfig {
  return patchItem(cfg, key, { size });
}

function patchItem(cfg: DashboardLayoutConfig, key: string, patch: Partial<DashboardLayoutItem>): DashboardLayoutConfig {
  const cur = cfg.items.find((x) => x.key === key);
  if (!cur || Object.entries(patch).every(([k, v]) => cur[k as keyof DashboardLayoutItem] === v)) return cfg;
  return { ...cfg, items: cfg.items.map((x) => (x.key === key ? { ...x, ...patch } : x)) };
}
