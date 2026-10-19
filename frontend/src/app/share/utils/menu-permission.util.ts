/**
 * Lọc menu theo danh sách permission đang có
 * @param menu - Mảng model gốc
 * @param userPermissions - Danh sách quyền của user (array<string>)
 */
export function filterMenuByPermission(menu: any[], userPermissions: string[]): any[] {
  const hasPermission = (permission: string | string[] | undefined): boolean => {
    if (!permission) return true; // public menu nếu không có permission

    const perms = Array.isArray(permission) ? permission : [permission];
    return perms.some((p: any) => typeof p === 'string' && userPermissions.includes(p));
  };

  const deepFilter = (items: any[]): any[] => {
    return items
      .map((item: any) => {
        const newItem = { ...item };

        if (newItem.items) {
          newItem.items = deepFilter(newItem.items);
        }

        return newItem;
      })
      .filter((item: any) => {
        const allow = hasPermission(item.permission);
        const hasVisibleChildren = Array.isArray(item.items) && item.items.length > 0;
        // Nhóm (có `items`, không routerLink riêng) mà con bị lọc hết → bỏ luôn, không để tiêu đề rỗng
        // trên sidebar (từ khi gộp Nhân sự/Lịch sử/Cấu hình vào AppLayout, user thiếu quyền vẫn thấy nhóm).
        if (Array.isArray(item.items) && !hasVisibleChildren && !item.routerLink) return false;
        return allow || hasVisibleChildren;
      });
  };

  return deepFilter(menu);
}

/**
 * Làm phẳng menu tree thành danh sách lệnh (leaf item có routerLink) — dùng cho CommandMenu.
 * Node chỉ có `items` con (không có routerLink riêng) bị bỏ qua, chỉ giữ leaf thật sự điều hướng được.
 * @param menu - Mảng model gốc (nên truyền menu ĐÃ filter theo permission)
 */
export function flattenMenuToCommands(menu: any[]): any[] {
  const commands: any[] = [];

  const walk = (items: any[]): void => {
    for (const item of items) {
      if (item.routerLink) {
        commands.push(item);
      }
      if (Array.isArray(item.items) && item.items.length > 0) {
        walk(item.items);
      }
    }
  };

  walk(menu);
  return commands;
}
