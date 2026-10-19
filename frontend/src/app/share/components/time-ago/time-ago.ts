import { ChangeDetectionStrategy, Component, Input, OnDestroy, OnInit, signal } from '@angular/core';
import { formatDistance } from 'date-fns';

@Component({
  changeDetection: ChangeDetectionStrategy.OnPush,
  selector: 'app-time-ago',
  standalone: true,
  imports: [],
  template: ` {{ displayData() }} `,
  styles: [],
})
export class TimeAgo implements OnInit, OnDestroy {
  @Input({ required: true }) data!: string | Date | number;
  @Input() refreshSecond: number = 60;
  displayData = signal('');
  private interval: any = null;

  ngOnInit() {
    this.displayData.set(formatDistance(new Date(this.data), new Date(), { addSuffix: true }));
    this.interval = setInterval(() => {
      this.displayData.set(formatDistance(new Date(this.data), new Date(), { addSuffix: true }));
    }, this.refreshSecond * 1000);
  }

  ngOnDestroy() {
    if (this.interval) {
      clearInterval(this.interval);
    }
  }
}
