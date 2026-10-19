import {ChangeDetectionStrategy, Component, inject, Input } from '@angular/core';
import { FormControl, ReactiveFormsModule } from '@angular/forms';
import { TranslatePipe } from '@ngx-translate/core';
import { ButtonDirective } from 'primeng/button';
import { DividerModule } from 'primeng/divider';

import { TimezoneService } from '@/core/services/timezone.service';

type ButtonSeverity =
  | 'primary'
  | 'secondary'
  | 'success'
  | 'info'
  | 'warn'
  | 'danger'
  | 'help'
  | null;

@Component({
  changeDetection: ChangeDetectionStrategy.OnPush,
  selector: 'date-picker-single-footer',
  standalone: true,
  imports: [ReactiveFormsModule, ButtonDirective, DividerModule, TranslatePipe],
  template: `
    <p-divider></p-divider>
    <div class="flex justify-center gap-2 px-2">
      @for (item of offsetOptions; track item.days) {
        <button
          pButton
          type="button"
          (click)="setOffset(item.days)"
          [severity]="item.severity"
          size="small"
          class="whitespace-nowrap"
          [text]="true"
          [raised]="true"
          [outlined]="activeOffset !== item.days"
        ><span>{{ (item.labelKey || item.label) | translate }}</span></button>
      }
    </div>
  `,
})
export class DatePickerSingleFooter {
  @Input({ required: true }) control!: FormControl<Date | null>;
  activeOffset: number | null = null;

  offsetOptions: { label: string; labelKey?: string; days: number; severity: ButtonSeverity }[] = [
    { label: '1 tháng', labelKey: 'share.date_picker.m1', days: 30, severity: 'success' },
    { label: '6 tháng', labelKey: 'share.date_picker.m6', days: 180, severity: 'info' },
    { label: '1 năm', labelKey: 'share.date_picker.y1', days: 365, severity: 'warn' },
    { label: '2 năm', labelKey: 'share.date_picker.y2', days: 730, severity: 'help' },
    { label: '10 năm', labelKey: 'share.date_picker.y10', days: 3650, severity: 'danger' },
  ];

  private tz = inject(TimezoneService);

  getEndOfOffsetDay(offset: number): Date {
    const d = this.tz.now();
    d.setDate(d.getDate() + offset);
    d.setHours(23, 59, 59, 999);
    return d;
  }

  setOffset(days: number) {
    this.control.setValue(this.getEndOfOffsetDay(days));
    this.activeOffset = days;
  }
}
