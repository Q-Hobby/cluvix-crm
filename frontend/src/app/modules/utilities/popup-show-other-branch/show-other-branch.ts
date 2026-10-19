import { TranslatePipe } from '@ngx-translate/core';
import {ChangeDetectionStrategy, Component, inject, OnInit } from '@angular/core';
import { ButtonModule } from 'primeng/button';
import { DynamicDialogConfig, DynamicDialogRef } from 'primeng/dynamicdialog';
import { TooltipModule } from 'primeng/tooltip';

@Component({
  changeDetection: ChangeDetectionStrategy.OnPush,
  selector: 'popup-show-other-branch',
  templateUrl: './show-other-branch.html',
  styles: [],
  standalone: true,
  imports: [ButtonModule, TooltipModule, TranslatePipe],
})
export class ShowOtherBranch implements OnInit {
  dialogConfig = inject(DynamicDialogConfig);
  ref = inject(DynamicDialogRef);
  textTitle!: string;

  ngOnInit(): void {
    this.textTitle = this.dialogConfig?.data?.text;
  }
}
