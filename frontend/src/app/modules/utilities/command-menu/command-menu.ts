import { ChangeDetectionStrategy, Component, computed, inject } from '@angular/core';
import { toSignal } from '@angular/core/rxjs-interop';
import { startWith } from 'rxjs/operators';
import { MenuItem } from 'primeng/api';
import { CommandMenu as PCommandMenu } from 'primeng/commandmenu';
import { Dialog } from 'primeng/dialog';
import { TranslatePipe } from '@ngx-translate/core';

import { BaseApp } from '@/share/base/BaseApp';
import { buildMenuTree } from '@/layout/menu.constant';
import { filterMenuByPermission, flattenMenuToCommands } from '@/share/utils/menu-permission.util';
import { userPermissions } from '@/core/services/auth/auth.state';
import { CommandMenuService } from '@/modules/utilities/command-menu/command-menu.service';

@Component({
  changeDetection: ChangeDetectionStrategy.OnPush,
  selector: 'app-utilities-command-menu',
  standalone: true,
  imports: [Dialog, PCommandMenu, TranslatePipe],
  templateUrl: './command-menu.html',
})
export class CommandMenu extends BaseApp {
  commandMenuService = inject(CommandMenuService);

  private readonly langTick = toSignal(this.translate.onLangChange.pipe(startWith(null)), {
    requireSync: true,
  });

  readonly commands = computed<MenuItem[]>(() => {
    this.langTick();
    const per = Array.from(userPermissions());
    const tree = filterMenuByPermission(buildMenuTree(), per);
    const flat = flattenMenuToCommands(tree);

    return flat.map((item) => ({
      label: this.translate.instant(item.labelKey || item.label),
      icon: item.icon,
      command: () => this.selectCommand(item),
    }));
  });

  selectCommand(item: any) {
    void this.route.navigate(item.routerLink, { queryParams: item.queryParams });
    this.commandMenuService.close();
  }

  onHide() {}
}
