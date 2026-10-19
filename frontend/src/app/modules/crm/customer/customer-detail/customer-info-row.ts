// customer-info-row.component.ts
import {ChangeDetectionStrategy, Component, computed, input } from '@angular/core';

import { ListShareImports } from '@/share/import/list-share-imports';
import { CustomDatePipe } from '@/share/pipes/custom-date/custom-date.pipe';

@Component({
  changeDetection: ChangeDetectionStrategy.OnPush,
  selector: 'app-customer-info-row',
  standalone: true,
  imports: [...ListShareImports],
  template: ` @if (value()) {
    <div class="mb-2">
      <span class="font-medium text-[var(--p-text-color)]">{{ label() | translate }}:</span>
      @if (linkHref()) {
        <!-- Field link (chat/facebook/zalo...): bấm thẳng vào giá trị là điều hướng NGAY TRONG TAB
             hiện tại — bỏ hành vi copy và bỏ icon "mở cửa sổ mới" trước đây. -->
        <a
          class="p-text-primary break-words ml-1 hover:underline"
          [href]="linkHref()"
          [pTooltip]="tooltip()"
          [innerText]="formattedValue() | translate"
        ></a>
      } @else {
        <span
          class="p-text-primary break-words ml-1"
          [appCopyText]="formattedValue()"
          [pTooltip]="tooltip()"
          [innerText]="formattedValue() | translate"
        >
        </span>
      }
    </div>
  }`,
})
export class CustomerInfoRow {
  label = input.required<string>();
  value = input.required<any>();
  tooltip = input<string>();
  isDate = input(false);
  isLink = input(false);

  private customDatePipe = new CustomDatePipe();

  /**
   * Href dùng để điều hướng, CHỈ khi giá trị là URL tuyệt đối http(s).
   * `chat_message`/`facebook`/`zalo`/`viber`/`instagram` là free-text do user nhập, BE không validate
   * URL (chỉ giới hạn độ dài) → giá trị có thể là username trần. Gắn `[href]` cho chuỗi đó thì bấm
   * vào sẽ điều hướng tương đối, đá user ra khỏi ERP tới trang không tồn tại. Không phải URL →
   * trả null để rơi về nhánh copy như các field thường.
   */
  linkHref = computed(() => {
    if (!this.isLink()) return null;
    const val = String(this.value() ?? '').trim();
    return /^https?:\/\//i.test(val) ? val : null;
  });

  formattedValue = computed(() => {
    const val = this.value();
    if (!val) return '';

    if (this.isDate()) {
      return this.customDatePipe.transform(val);
    }

    return val;
  });
}
