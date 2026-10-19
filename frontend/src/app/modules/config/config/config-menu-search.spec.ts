import { CONFIG_MENU, ConfigMenuGroup, ConfigMenuItem } from './config-menu.registry';
import {
  buildMenuIndex,
  isItemVisible,
  normalizeAligned,
  PermissionContext,
  searchMenu,
} from './config-menu-search';

/** Dịch giả lập: trả lại chính key → phần nhãn tiếng Việt gốc vẫn phải đủ để tìm ra. */
const echoKey = (key: string) => key;

const SUPER: PermissionContext = { isSuperUser: true, granted: new Set<string>() };
const NOBODY: PermissionContext = { isSuperUser: false, granted: new Set<string>() };

function links(ctx: PermissionContext, query: string, t: (k: string) => string = echoKey): string[] {
  return searchMenu(buildMenuIndex(CONFIG_MENU, ctx, t), query).map((h) => h.item.link);
}

describe('normalizeAligned', () => {
  it('giữ nguyên độ dài để offset highlight khớp nhãn CÓ dấu', () => {
    const label = 'Nguồn khách hàng';
    expect(normalizeAligned(label)).toBe('nguon khach hang');
    expect(Array.from(normalizeAligned(label)).length).toBe(Array.from(label).length);
  });

  it('bỏ dấu cả chữ đ', () => {
    expect(normalizeAligned('Đơn vị tính')).toBe('don vi tinh');
  });
});

describe('searchMenu', () => {
  it('gõ KHÔNG dấu vẫn ra mục có dấu', () => {
    expect(links(SUPER, 'nguon')).toContain('/config/customer-source');
  });

  it('gõ có dấu — "Nguồn" ra cả nguồn và nhóm nguồn', () => {
    const found = links(SUPER, 'nguồn');
    expect(found).toContain('/config/customer-source');
    expect(found).toContain('/config/group-source');
  });

  it('nhiều token là AND, không phải OR', () => {
    const found = links(SUPER, 'nhom nguon');
    expect(found).toContain('/config/group-source');
    expect(found).not.toContain('/config/customer-source');
  });

  it('khớp đầu nhãn xếp trên khớp giữa nhãn', () => {
    const found = links(SUPER, 'nguon');
    expect(found.indexOf('/config/customer-source')).toBeLessThan(found.indexOf('/config/group-source'));
  });

  it('gõ tên NHÓM ra mọi mục trong nhóm đó', () => {
    // story-20: nhóm 'zalo' đã xoá khỏi registry (module Zalo bỏ khỏi CRM standalone) — đổi sang
    // nhóm 'location' (vẫn tồn tại), giữ nguyên ý định test: gõ tên nhóm ra mọi mục của nhóm đó.
    const location = CONFIG_MENU.find((g) => g.key === 'location')!;
    const found = links(SUPER, 'location');
    location.items.forEach((item) => expect(found).toContain(item.link));
  });

  it('khớp được cả đường dẫn route', () => {
    expect(links(SUPER, 'security-audit')).toContain('/config/security-audit');
  });

  it('query rỗng/khoảng trắng không trả gì (không đổ cả 131 mục ra)', () => {
    expect(links(SUPER, '')).toEqual([]);
    expect(links(SUPER, '   ')).toEqual([]);
  });

  it('không khớp thì rỗng', () => {
    expect(links(SUPER, 'khongcogitoday')).toEqual([]);
  });

  it('vị trí highlight cắt đúng chữ trên nhãn CÓ dấu', () => {
    const t = (k: string) => (k === 'common.label.source' ? 'Nguồn khách hàng' : k);
    const hits = searchMenu(buildMenuIndex(CONFIG_MENU, SUPER, t), 'khach');
    const hit = hits.find((h) => h.item.link === '/config/customer-source')!;
    expect(hit.label.slice(hit.match!.start, hit.match!.end)).toBe('khách');
  });

  it('UI tiếng Anh + gõ tiếng Việt: vẫn tìm ra, chỉ không có đoạn tô', () => {
    // Nhãn VN gốc luôn nằm trong chỉ số nên vẫn khớp; highlight bám nhãn ĐANG HIỂN THỊ
    // (tiếng Anh) nên không có đoạn nào để tô — CÓ CHỦ ĐÍCH, không phải thiếu sót.
    const t = (k: string) => (k === 'common.label.source' ? 'Customer source' : k);
    const hits = searchMenu(buildMenuIndex(CONFIG_MENU, SUPER, t), 'nguon');
    const hit = hits.find((h) => h.item.link === '/config/customer-source')!;
    expect(hit).toBeDefined();
    expect(hit.match).toBeNull();
  });
});

describe('lọc quyền', () => {
  it('user KHÔNG quyền gì thì search không trả về mục nào', () => {
    expect(links(NOBODY, 'nguon')).toEqual([]);
  });

  it('mục superUserOnly ẩn với user thường, hiện với super-user', () => {
    const granted = new Set(CONFIG_MENU.map((g) => g.permission as string));
    const normal: PermissionContext = { isSuperUser: false, granted };
    expect(links(normal, 'nhat ky bao mat')).toEqual([]);
    expect(links(SUPER, 'nhat ky bao mat')).toContain('/config/security-audit');
  });

  it('item có quyền riêng bị ẩn dù mở được panel cha', () => {
    // story-20: nhóm 'omniChannel' (item '/config/omni-channel/assignment' có permission riêng)
    // đã xoá khỏi registry cùng module Omnichannel — dựng fixture tổng hợp để giữ nguyên ý định
    // test isItemVisible() (hàm THUẦN, không phụ thuộc registry thật).
    const group: ConfigMenuGroup = {
      key: 'fixture',
      title: 'Fixture',
      titleKey: 'fixture',
      icon: 'pi pi-cog',
      permission: 'fixture.parent',
      column: 1,
      items: [],
    };
    const restrictedItem: ConfigMenuItem = {
      label: 'Fixture riêng',
      link: '/config/fixture-restricted',
      permission: 'fixture.child',
    };
    const ctx: PermissionContext = { isSuperUser: false, granted: new Set([group.permission as string]) };

    expect(isItemVisible(group, restrictedItem, ctx)).toBe(false);
  });
});

describe('registry', () => {
  it('không có link trùng trong cùng một nhóm (track @for dựa vào link)', () => {
    CONFIG_MENU.forEach((g) => {
      const seen = g.items.map((i) => i.link + JSON.stringify(i.queryParams ?? {}));
      expect(new Set(seen).size).toBe(seen.length);
    });
  });

  it('mọi mục đều có link bắt đầu bằng /config', () => {
    CONFIG_MENU.flatMap((g) => g.items).forEach((i) => expect(i.link.startsWith('/config')).toBe(true));
  });
});
