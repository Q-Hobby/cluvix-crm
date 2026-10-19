import { ChangeDetectionStrategy, Component, input } from '@angular/core';
import { TranslatePipe } from '@ngx-translate/core';

/**
 * Một khối nội dung trong SlideOver: caption chữ nhỏ in hoa + thân dạng card.
 *
 * @example
 * ```html
 * <app-slide-over-section label="common.label.service">
 *   <span sectionActions class="text-sm font-medium">1.500.000 đ</span>
 *   ...nội dung...
 * </app-slide-over-section>
 * ```
 */
@Component({
  changeDetection: ChangeDetectionStrategy.OnPush,
  selector: 'app-slide-over-section',
  standalone: true,
  imports: [TranslatePipe],
  template: `
    <section class="flex flex-col gap-2">
      @if (label()) {
        <div class="flex items-center justify-between gap-2 px-1">
          <h3 class="m-0 text-xs font-semibold uppercase tracking-wide text-muted-color">
            {{ label() | translate }}
          </h3>
          <ng-content select="[sectionActions]" />
        </div>
      }

      @if (bare()) {
        <ng-content />
      } @else {
        <div class="flex flex-col gap-3 rounded-xl border border-surface p-4">
          <ng-content />
        </div>
      }
    </section>
  `,
})
export class SlideOverSection {
  /** Key i18n hoặc text thuần (TranslatePipe trả nguyên chuỗi khi không phải key). */
  readonly label = input('');
  /** true = bỏ khung card, tự dựng layout bên trong. */
  readonly bare = input(false);
}
