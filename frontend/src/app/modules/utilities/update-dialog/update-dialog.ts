import { ChangeDetectionStrategy, Component, inject } from '@angular/core';
import { environment } from '@env/environment';
import { ButtonDirective } from 'primeng/button';
import { Dialog } from 'primeng/dialog';

import { TranslatePipe } from '@ngx-translate/core';
import { VersionCheckService } from '@/share/services/version-check.service';

@Component({
  changeDetection: ChangeDetectionStrategy.OnPush,
  selector: 'app-update-dialog',
  templateUrl: './update-dialog.html',
  standalone: true,
  imports: [Dialog, ButtonDirective, TranslatePipe],
})
export class UpdateDialog {
  versionCheckService = inject(VersionCheckService);

  constructor() {
    /* Just do on production only */
    if (environment.prod) {
      this.versionCheckService.initVersionCheck(60 * 1000); // check every 600 seconds
    }
  }

  update() {
    window.location.reload();
    /* Do reload for all tabs */
    this.versionCheckService.bc.postMessage('NewVersion');
  }
}
