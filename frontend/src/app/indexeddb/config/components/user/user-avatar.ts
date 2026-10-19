import { ChangeDetectionStrategy, Component, computed, inject, Input, OnInit } from '@angular/core';

import { Avatar } from 'primeng/avatar';
import { UserCacheService } from './user-cache.service';
import { AvatarWordPipe } from '@/share/pipes/avatar-word/avatar-word.pipe';

@Component({
  changeDetection: ChangeDetectionStrategy.OnPush,
  selector: 'app-user-avatar',
  standalone: true,
  imports: [Avatar, AvatarWordPipe],
  template: `
    @if (user(); as userData) {
      @if (userData.avatar) {
        <!-- PrimeNG Avatar ưu tiên 'label' hơn 'image' (@if label() trước @else if image() trong
             chính component gốc) — truyền CẢ HAI cùng lúc thì label (chữ cái) luôn thắng, ảnh thật
             KHÔNG BAO GIỜ hiện dù có avatar (bug phát hiện 2026-08-29 qua teamchat). Phải tách 2
             nhánh, chỉ truyền đúng 1 input. -->
        <p-avatar
          [image]="userData.avatar"
          [size]="size"
          [shape]="shape"
          [style]="avatarStyle"
          [class]="avatarClass"
        >
        </p-avatar>
      } @else {
        <p-avatar
          [label]="((userData?.fullname ?? '') | avatarWord) ?? ''"
          [size]="size"
          [shape]="shape"
          [style]="avatarStyle"
          [class]="avatarClass"
        >
        </p-avatar>
      }
    } @else {
      <p-avatar
        [label]="'?'"
        [size]="size"
        [shape]="shape"
        [style]="avatarStyle"
        [class]="avatarClass"
      >
      </p-avatar>
    }
  `,
})
export class UserAvatar implements OnInit {
  @Input() userId!: number;
  @Input() size: 'normal' | 'large' | 'xlarge' = 'normal';
  @Input() shape: 'square' | 'circle' = 'circle';
  @Input() avatarStyle: any = {};
  @Input() avatarClass: string = '';

  private userCache = inject(UserCacheService);

  user = computed(() => {
    if (!this.userId) return null;
    return this.userCache.getById(this.userId);
  });

  async ngOnInit() {
    await this.userCache.loadUsers();
  }
}
