// related-user-avatars.ts
import { ChangeDetectionStrategy, Component, computed, inject, Input } from '@angular/core';
import { AvatarModule } from 'primeng/avatar';
import { AvatarGroupModule } from 'primeng/avatargroup';
import { TooltipModule } from 'primeng/tooltip';
import { UserCacheService } from '@/indexeddb/config/components/user';

type AvatarVM = { id: number; name: string; avatar?: string | null };

@Component({
  changeDetection: ChangeDetectionStrategy.OnPush,
  selector: 'app-related-user-avatars',
  standalone: true,
  imports: [AvatarModule, AvatarGroupModule, TooltipModule],
  template: `
    <p-avatargroup class="flex items-center">
      @for (u of visible(); track u.id) {
        <p-avatar
          [image]="u.avatar || undefined"
          [label]="!u.avatar ? initials(u.name) : undefined"
          [size]="size"
          shape="circle"
          pTooltip="{{ u.name }}"
          tooltipPosition="top"
        >
        </p-avatar>
      }

      @if (overflowCount() > 0) {
        <p-avatar label="+{{ overflowCount() }}" [size]="size" shape="circle"></p-avatar>
      }
    </p-avatargroup>
  `,
})
export class RelatedUserAvatarsComponent {
  /** Danh sách id user (ví dụ: [2,3,4]) */
  @Input({ required: true }) ids: number[] | null = [];
  /** Số avatar hiển thị tối đa, phần dư gộp +N */
  @Input() max = 4;
  /** Kích cỡ avatar: 'normal' | 'large' | 'xlarge' */
  @Input() size: 'normal' | 'large' | 'xlarge' = 'large';
  private userCache: UserCacheService = inject(UserCacheService);

  // Lấy toàn bộ options user từ SelectService (LS-first).
  // Cho phép nhiều schema: {value,label,avatar} hoặc {id,fullname,image}...
  private options = computed(() => this.resolveOptions());

  // Chuyển ids -> AvatarVM[] đã map tên & avatar
  private users = computed<AvatarVM[]>(() => {
    const ids = this.ids ?? [];
    if (!ids.length) return [];
    const map = this.options();
    const out: AvatarVM[] = [];
    for (const rawId of ids) {
      const id = Number(rawId);
      const o = map.get(id);
      if (!o) continue;
      const name = o.label ?? o.fullname ?? o.name ?? o.nickname ?? `User #${id}`;
      const avatar = o.avatar ?? o.image ?? o.photo ?? o.picture ?? null;
      out.push({ id, name, avatar });
    }
    return out;
  });

  visible = computed(() => this.users().slice(0, this.max));
  overflowCount = computed(() => Math.max(this.users().length - this.max, 0));

  initials(name: string) {
    return name
      .split(' ')
      .filter(Boolean)
      .map((n) => n[0]!)
      .join('')
      .slice(0, 2)
      .toUpperCase();
  }

  /** Gom options user về Map<id, option> để tra cứu siêu nhanh */
  private resolveOptions(): Map<number, any> {
    try {
      const users = this.userCache.getAll();
      const map = new Map<number, any>();

      for (const user of users) {
        map.set(user.id!, {
          id: user.id,
          fullname: user.fullname,
          avatar: user.avatar,
        });
      }
      return map;
    } catch {
      return new Map();
    }
  }
}
