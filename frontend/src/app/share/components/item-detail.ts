import {ChangeDetectionStrategy, Component, Input } from '@angular/core';

@Component({
  changeDetection: ChangeDetectionStrategy.OnPush,
  selector: 'app-item-detail',
  standalone: true,
  imports: [],
  template: ` <li
    class="flex items-center py-4 px-2 border-t border-surface-200 dark:border-surface-700 flex-wrap"
  >
    <div class="text-surface-500 dark:text-surface-300 w-6/12 md:w-2/12 font-medium">
      {{ label }}
    </div>
    <div class="text-surface-900 dark:text-surface-0 w-full md:w-8/12 md:order-none order-1">
      <ng-content></ng-content>
    </div>
  </li>`,
})
export class ItemDetail {
  @Input({ required: true }) label!: string;
}
