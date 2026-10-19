import { ChangeDetectionStrategy, Component, Input } from '@angular/core';
import { TranslatePipe } from '@ngx-translate/core';

@Component({
  changeDetection: ChangeDetectionStrategy.OnPush,
  selector: 'app-record-locked-banner',
  standalone: true,
  imports: [TranslatePipe],
  template: `
    <!-- LUÔN mount trong DOM (không @if) + transition grid-template-rows 0fr↔1fr
         (co giãn theo đúng chiều cao nội dung) — tránh content layout shift đột
         ngột khi \`show\` đổi lúc BS "Mở lại" bệnh án (xem frontend.md B.19).
         Alpha hex (không phải bg-amber-50 solid) để tự thích nghi dark mode — B.18. -->
    <div class="grid transition-[grid-template-rows] duration-200 ease-out"
         [style.grid-template-rows]="show ? '1fr' : '0fr'">
      <div class="overflow-hidden">
        <div class="mx-4 mt-4 px-4 py-3 flex items-start gap-3 rounded-lg border"
             style="background: #F59E0B1A; border-color: #F59E0B33">
          <i class="pi pi-lock text-lg mt-0.5" style="color: #B45309"></i>
          <div class="flex-1 min-w-0">
            <div class="text-sm font-semibold" style="color: #92400E">
              {{ titleKey | translate }}
            </div>
            <div class="text-xs mt-0.5" style="color: #B45309">
              {{ hintKey | translate }}
            </div>
          </div>
        </div>
      </div>
    </div>
  `,
})
export class RecordLockedBanner {
  @Input() show = false;
  @Input() titleKey = 'emr.common.record_locked_title';
  @Input() hintKey = 'emr.common.record_locked_hint';
}
