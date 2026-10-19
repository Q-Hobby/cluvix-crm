import { ChangeDetectionStrategy, Component, inject, Input, OnInit } from '@angular/core';

import { UserAvatar } from './user-avatar';
import { UserName } from './user-name';
import { UserCacheService } from './user-cache.service';

@Component({
  changeDetection: ChangeDetectionStrategy.OnPush,
  selector: 'app-user-display',
  standalone: true,
  imports: [UserAvatar, UserName],
  template: `
    @if (showAvatar && showName) {
      <div class="flex items-center gap-2">
        <app-user-avatar [userId]="userId" [size]="avatarSize" [shape]="avatarShape">
        </app-user-avatar>
        <app-user-name [userId]="userId"></app-user-name>
      </div>
    } @else if (showAvatar) {
      <app-user-avatar [userId]="userId" [size]="avatarSize" [shape]="avatarShape">
      </app-user-avatar>
    } @else if (showName) {
      <app-user-name [userId]="userId"></app-user-name>
    }
  `,
})
export class UserDisplay implements OnInit {
  @Input() userId!: number;
  @Input() showAvatar: boolean = true;
  @Input() showName: boolean = true;
  @Input() avatarSize: 'normal' | 'large' | 'xlarge' = 'normal';
  @Input() avatarShape: 'square' | 'circle' = 'circle';

  private userCache = inject(UserCacheService);

  async ngOnInit() {
    await this.userCache.loadUsers();
  }
}
