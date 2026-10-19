import { ChangeDetectionStrategy, Component, computed, inject, Input, OnInit } from '@angular/core';

import { UserCacheService } from './user-cache.service';

@Component({
  changeDetection: ChangeDetectionStrategy.OnPush,
  selector: 'app-user-name',
  standalone: true,
  imports: [],
  template: `
    @if (user(); as userData) {
      <span [title]="userData.fullname">{{ userData.fullname }}</span>
    } @else {
      <span class="text-gray-400">---</span>
    }
  `,
})
export class UserName implements OnInit {
  @Input() userId!: number;

  private userCache = inject(UserCacheService);

  user = computed(() => {
    if (!this.userId) return null;
    return this.userCache.getById(this.userId);
  });

  async ngOnInit() {
    await this.userCache.loadUsers();
  }
}
