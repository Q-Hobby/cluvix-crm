import { ChangeDetectionStrategy, Component, input } from '@angular/core';
import { TranslatePipe } from '@ngx-translate/core';

/**
 * Một dòng nhãn — giá trị bên trong `app-slide-over-section`.
 * Giá trị đưa vào bằng ng-content để nhúng được tag/badge/link, không chỉ text.
 *
 * @example
 * ```html
 * <app-slide-over-row label="common.label.status">
 *   <p-tag severity="info" [value]="statusLabel()" />
 * </app-slide-over-row>
 * ```
 */
@Component({
  changeDetection: ChangeDetectionStrategy.OnPush,
  selector: 'app-slide-over-row',
  standalone: true,
  imports: [TranslatePipe],
  template: `
    @if (vertical()) {
      <div class="flex flex-col gap-1">
        <span class="text-sm text-muted-color">{{ label() | translate }}</span>
        <div class="text-sm font-medium text-color break-words"><ng-content /></div>
      </div>
    } @else {
      <div class="flex items-start justify-between gap-3">
        <span class="text-sm text-muted-color shrink-0">{{ label() | translate }}</span>
        <div class="text-sm font-medium text-color text-right break-words min-w-0">
          <ng-content />
        </div>
      </div>
    }
  `,
})
export class SlideOverRow {
  readonly label = input('');
  /** true = nhãn nằm trên, giá trị xuống dòng (hợp với giá trị dài: địa chỉ, ghi chú). */
  readonly vertical = input(false);
}
