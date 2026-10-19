import { AsyncPipe } from '@angular/common';
import { ChangeDetectionStrategy, Component, inject, OnInit } from '@angular/core';
import { Router } from '@angular/router';
import { ButtonDirective } from 'primeng/button';
import { Dialog } from 'primeng/dialog';

import { TranslatePipe } from '@ngx-translate/core';
import { TimezoneCheckService } from '@/share/services/timezone-check.service';

@Component({
  changeDetection: ChangeDetectionStrategy.OnPush,
  selector: 'app-timezone-dialog',
  templateUrl: './timezone-dialog.html',
  standalone: true,
  imports: [Dialog, ButtonDirective, AsyncPipe, TranslatePipe],
})
export class TimezoneDialog implements OnInit {
  timezoneCheckService = inject(TimezoneCheckService);
  router = inject(Router);

  ngOnInit() {
    void this.timezoneCheckService.check();
  }

  close() {
    this.timezoneCheckService.close();
  }
}
