import {ChangeDetectionStrategy, Component, ElementRef, effect, inject, OnInit, signal, ViewChild} from '@angular/core';
import {RouterModule} from '@angular/router';

import {AppMenuitem} from './app.menuitem';

import {BaseApp} from '@/share/base/BaseApp';
import {filterMenuByPermission} from '@/share/utils/menu-permission.util';
import {userPermissions} from '@/core/services/auth/auth.state';
import {buildMenuTree} from '@/layout/menu.constant';

@Component({
  changeDetection: ChangeDetectionStrategy.OnPush,
  selector: 'app-menu, [app-menu]',
  standalone: true,
  imports: [AppMenuitem, RouterModule],
  template: `
    @if (!translationsReady()) {
      <ul class="layout-menu">
        @for (i of [1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14]; track i) {
          <li class="menu-skeleton">
            <div class="skeleton-icon"></div>
            <div class="skeleton-text"></div>
          </li>
        }
      </ul>
    } @else if (model.length > 0) {
      <ul class="layout-menu" #menuContainer>
        @for (item of model; track $index; let i = $index) {
          <li app-menuitem [item]="item" [index]="i" [root]="true" [parentKey]="''"></li>
        }
      </ul>
    }
  `,
  styles: [
    `
      .menu-skeleton {
        display: flex;
        align-items: center;
        gap: 12px;
        padding: 12px 16px;
        margin-bottom: 4px;
      }

      .skeleton-icon {
        width: 20px;
        height: 20px;
        border-radius: 4px;
        background: linear-gradient(90deg, #f0f0f0 25%, #e0e0e0 50%, #f0f0f0 75%);
        background-size: 200% 100%;
        animation: shimmer 1.5s infinite;
      }

      .skeleton-text {
        flex: 1;
        height: 16px;
        border-radius: 4px;
        background: linear-gradient(90deg, #f0f0f0 25%, #e0e0e0 50%, #f0f0f0 75%);
        background-size: 200% 100%;
        animation: shimmer 1.5s infinite;
      }

      @keyframes shimmer {
        0% {
          background-position: 200% 0;
        }
        100% {
          background-position: -200% 0;
        }
      }
    `,
  ],
})
export class AppMenu extends BaseApp implements OnInit {
  el: ElementRef = inject(ElementRef);
  model: any[] = [];
  translationsReady = signal(false);

  @ViewChild('menuContainer') menuContainer!: ElementRef;

  constructor() {
    super();
    effect(() => {
      if (!this.translationsReady()) return;
      this.buildMenu();
    });
  }

  ngOnInit() {
    // Wait for translations to load before showing menu
    this.translate.get('common.label.dashboard')?.subscribe(() => {
      this.translationsReady.set(true);
    });
  }

  private buildMenu(): void {
    this.model = buildMenuTree();

    const per = Array.from(userPermissions());
    this.model = filterMenuByPermission(this.model, per);
  }
}
