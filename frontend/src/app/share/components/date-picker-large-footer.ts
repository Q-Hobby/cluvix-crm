import {ChangeDetectionStrategy, Component, EventEmitter, inject, Output } from '@angular/core';
import { ButtonDirective } from 'primeng/button';
import { Divider } from 'primeng/divider';
import { TranslatePipe } from '@ngx-translate/core';

import { TimezoneService } from '@/core/services/timezone.service';

export type QuickRange = 'thisMonth' | 'lastMonth' | 'thisYear' | 'lastYear' | null;

@Component({
  changeDetection: ChangeDetectionStrategy.OnPush,
  selector: 'app-date-picker-large-footer',
  template: `
    <p-divider></p-divider>
    <div class="flex gap-2 whitespace-nowrap">
      <button
        pButton
        type="button"
        (click)="selectThisMonth()"
        size="small"
        [raised]="true"
        [outlined]="activeRange !== 'thisMonth'"
        variant="text"
        severity="success"
      ><span>{{ 'common.label.this_month' | translate }}</span></button>
      <button
        pButton
        type="button"
        (click)="selectLastMonth()"
        size="small"
        [raised]="true"
        [outlined]="activeRange !== 'lastMonth'"
        variant="text"
        severity="warn"
      ><span>{{ 'common.label.previous_month' | translate }}</span></button>
      <button
        pButton
        type="button"
        (click)="selectThisYear()"
        size="small"
        severity="info"
        variant="text"
        [raised]="true"
        [outlined]="activeRange !== 'thisYear'"
      ><span>{{ 'common.label.this_year' | translate }}</span></button>
      <button
        pButton
        type="button"
        (click)="selectLastYear()"
        size="small"
        variant="text"
        [raised]="true"
        [outlined]="activeRange !== 'lastYear'"
      ><span>{{ 'common.label.previous_year' | translate }}</span></button>
    </div>
  `,
  standalone: true,
  imports: [Divider, ButtonDirective, TranslatePipe],
})
export class DatePickerLargeFooter {
  @Output() rangeChange = new EventEmitter<{ start: Date; end: Date }>();
  activeRange: QuickRange = null;
  private tz = inject(TimezoneService);

  selectRange(range: QuickRange, start: Date, end: Date) {
    this.activeRange = range;
    this.rangeChange.emit({ start, end });
  }

  selectThisMonth() {
    this.selectRange('thisMonth', this.tz.monthStart(), this.tz.monthEnd());
  }

  selectLastMonth() {
    this.selectRange('lastMonth', this.tz.lastMonthStart(), this.tz.lastMonthEnd());
  }

  selectThisYear() {
    this.selectRange('thisYear', this.tz.yearStart(), this.tz.yearEnd());
  }

  selectLastYear() {
    this.selectRange('lastYear', this.tz.lastYearStart(), this.tz.lastYearEnd());
  }
}
