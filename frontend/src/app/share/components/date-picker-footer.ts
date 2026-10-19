import { CommonModule } from '@angular/common';
import {ChangeDetectionStrategy, Component, EventEmitter, inject, Output } from '@angular/core';
import { ButtonDirective } from 'primeng/button';
import { Divider } from 'primeng/divider';
import { TranslatePipe } from '@ngx-translate/core';

import { TimezoneService } from '@/core/services/timezone.service';

export type QuickRange = 'today' | 'tomorrow' | 'yesterday' | 'thisMonth' | 'lastMonth' | null;

@Component({
  changeDetection: ChangeDetectionStrategy.OnPush,
  selector: 'app-date-picker-footer',
  template: `
    <p-divider></p-divider>
    <div class="flex whitespace-nowrap">
      <button
        pButton
        type="button"
        (click)="selectYesterday()"
        size="small"
        variant="text"
        [outlined]="activeRange !== 'yesterday'"
        severity="secondary"
      ><span>{{ 'common.label.yesterday' | translate }}</span></button>
      <button
        pButton
        type="button"
        (click)="selectToday()"
        size="small"
        variant="text"
        [ngClass]="{ 'text-white': activeRange === 'today' }"
      ><span>{{ 'common.label.today' | translate }}</span></button>
      <button
        pButton
        type="button"
        (click)="selectTomorrow()"
        size="small"
        variant="text"
        severity="info"
      ><span>{{ 'common.label.tomorrow' | translate }}</span></button>
      <button
        pButton
        type="button"
        (click)="selectLastMonth()"
        size="small"
        [outlined]="activeRange !== 'lastMonth'"
        severity="warn"
        variant="text"
      ><span>{{ 'common.label.previous_month' | translate }}</span></button>
      <button
        pButton
        type="button"
        (click)="selectThisMonth()"
        size="small"
        [outlined]="activeRange !== 'thisMonth'"
        variant="text"
        severity="success"
      ><span>{{ 'common.label.this_month' | translate }}</span></button>
    </div>
  `,
  standalone: true,
  imports: [CommonModule, Divider, ButtonDirective, TranslatePipe],
})
export class DatePickerFooter {
  @Output() rangeChange = new EventEmitter<{ start: Date; end: Date }>();
  activeRange: QuickRange = null;
  private tz = inject(TimezoneService);

  selectRange(range: QuickRange, start: Date, end: Date) {
    this.activeRange = range;
    this.rangeChange.emit({ start, end });
  }

  selectTomorrow() {
    this.selectRange('tomorrow', this.tz.tomorrowStart(), this.tz.tomorrowEnd());
  }

  selectToday() {
    this.selectRange('today', this.tz.todayStart(), this.tz.todayEnd());
  }

  selectYesterday() {
    this.selectRange('yesterday', this.tz.yesterdayStart(), this.tz.yesterdayEnd());
  }

  selectThisMonth() {
    this.selectRange('thisMonth', this.tz.monthStart(), this.tz.monthEnd());
  }

  selectLastMonth() {
    this.selectRange('lastMonth', this.tz.lastMonthStart(), this.tz.lastMonthEnd());
  }
}
