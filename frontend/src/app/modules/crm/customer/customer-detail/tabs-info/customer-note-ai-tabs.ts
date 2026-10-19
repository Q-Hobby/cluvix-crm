import { ChangeDetectionStrategy, Component, computed, input, output, signal } from '@angular/core';
import { CommonModule } from '@angular/common';
import { FormsModule } from '@angular/forms';
import { TranslatePipe } from '@ngx-translate/core';
import { SkeletonModule } from 'primeng/skeleton';
import { ButtonModule } from 'primeng/button';
import { TooltipModule } from 'primeng/tooltip';
import { FloatLabel } from 'primeng/floatlabel';

import { AppNote } from '@/modules/crm/note/app-note/app-note';
import { AppointmentNotBookedReasonsCascadeFormControl } from '@/indexeddb/config/components/appointment-not-booked-reasons';
import { CustomerModel } from '@/core/models/crm/customer.model';
import { hasPermission } from '@/core/services/auth/auth.selectors';
import { PERMISSION_SELECT } from '@/define';
import { CustomerAttachmentFile } from './customer-attachment-file';
import { CustomerAttachmentAudio } from './customer-attachment-audio';
import { CustomerAttachmentVideo } from './customer-attachment-video';

interface Tab {
  labelKey: string;
  icon: string;
}

@Component({
  changeDetection: ChangeDetectionStrategy.OnPush,
  selector: 'app-customer-note-ai-tabs',
  standalone: true,
  imports: [
    CommonModule,
    FormsModule,
    TranslatePipe,
    SkeletonModule,
    ButtonModule,
    TooltipModule,
    FloatLabel,
    AppNote,
    AppointmentNotBookedReasonsCascadeFormControl,
    CustomerAttachmentFile,
    CustomerAttachmentAudio,
    CustomerAttachmentVideo,
  ],
  template: `
    <div class="col-span-12 p-0 border-l border-r border-[var(--p-content-border-color)]">
      @if (loading()) {
        <!-- Skeleton: tab bar + content -->
        <div class="bg-surface-0 dark:bg-surface-950 px-2 py-3 flex items-center gap-4 border-b">
          @for (_ of skeletonTabs; track $index) {
            <p-skeleton width="90px" height="1.5rem" />
          }
        </div>
        <div class="p-3 flex flex-col gap-2">
          <p-skeleton width="100%" height="2rem" />
          <p-skeleton width="100%" height="8rem" />
        </div>
      } @else {
      <!-- Tabs -->
      <ul role="tablist" class="bg-surface-0 dark:bg-surface-950 px-2 m-0 list-none flex items-center overflow-x-auto select-none gap-4 border-b">
        @for (item of tabs(); track item.labelKey; let index = $index) {
          <li role="presentation">
            <!-- <button> chứ KHÔNG phải <a> không href: anchor thiếu href không nằm
                 trong tab order nên 6 tab này sẽ không thao tác được bằng bàn phím. -->
            <button
              type="button"
              role="tab"
              [attr.aria-selected]="activeTab() === index"
              [attr.tabindex]="activeTab() === index ? 0 : -1"
              class="cursor-pointer p-3 flex items-center justify-center border-b-2 transition-colors duration-150 whitespace-nowrap"
              [ngClass]="{
                'border-primary text-primary dark:text-primary-400 hover:border-primary-emphasis font-semibold': activeTab() === index,
                'text-surface-500 dark:text-surface-400 border-transparent hover:border-surface-300 dark:hover:border-surface-600': activeTab() !== index,
              }"
              (click)="activeTab.set(index)"
            >
              {{ item.labelKey | translate }}
            </button>
          </li>
          @if (index < tabs().length - 1) {
            <div class="w-px h-6 bg-surface-200 dark:bg-surface-700 shrink-0"></div>
          }
        }
      </ul>

      <!-- Tab Content — mỗi tab dựng 1 lần rồi GIỮ trong DOM, ẩn bằng [hidden]
           (đồng bộ với tab ngoài: p-tabpanel cũng ẩn bằng host attribute hidden).
           @if trần sẽ huỷ component khi rời tab → quay lại phải gọi API lại, mất
           vị trí cuộn và mất nội dung đang soạn dở ở AI Chat. @defer lo phần
           "chỉ dựng khi mở lần đầu" nên vẫn không tốn gì cho tab chưa đụng tới. -->
      <div class="h-full">
        <div [hidden]="activeTab() !== 0" class="h-full">
        @defer (when activeTab() === 0) {
          @let cust = customer();
          @if (cust.id) {
            <!-- R-4: danh sách ghi chú cần viewNote (GET note/list-advise enforce ở BE) — thiếu quyền thì
                 KHÔNG dựng app-note (không gọi API nhận 403), chỉ giữ ô lý do chưa đặt lịch. -->
            @if (canViewNote()) {
              <app-note [customer_id]="cust.id" class="h-full">
                <ng-container [ngTemplateOutlet]="reasonTpl" />
              </app-note>
            } @else {
              <div class="p-3"><ng-container [ngTemplateOutlet]="reasonTpl" /></div>
            }
          }
          <ng-template #reasonTpl>
            <div class="flex gap-2 pt-1">
              <p-floatlabel variant="on">
                <app-appointment-not-booked-reasons-cascade-form-control
                  id="appointment_reason"
                  [value]="customer().appointment_not_booked_reasons_id ?? null"
                  [style]="{ width: '200px' }"
                  (valueChange)="reasonChange.emit($event)"
                  size="small"
                />
                <label for="appointment_reason" class="text-[var(--p-text-muted-color)] font-medium">{{ 'crm.customer_note_ai_tabs.reason_label' | translate }}</label>
              </p-floatlabel>
            </div>
          </ng-template>
        }
        </div>

        <div [hidden]="activeTab() !== 1" class="h-full">
        @defer (when activeTab() === 1) {
          @if (customer().id) {
            <app-customer-attachment-file [customerId]="customer().id!" />
          }
        }
        </div>

        <div [hidden]="activeTab() !== 2" class="h-full">
        @defer (when activeTab() === 2) {
          @if (customer().id) {
            <app-customer-attachment-audio [customerId]="customer().id!" />
          }
        }
        </div>

        <div [hidden]="activeTab() !== 3" class="h-full">
        @defer (when activeTab() === 3) {
          @if (customer().id) {
            <app-customer-attachment-video [customerId]="customer().id!" />
          }
        }
        </div>
      </div>
      }
    </div>
  `,
})
export class CustomerNoteAiTabs {
  customer = input.required<CustomerModel>();
  loading = input<boolean>(false);

  reasonChange = output<any>();

  activeTab = signal(0);

  /** R-4: quyền xem danh sách ghi chú — khớp BE `GET /api/crm/note/list-advise` (viewNote). */
  readonly canViewNote = computed(() => hasPermission(PERMISSION_SELECT.permissionViewNote));
  readonly skeletonTabs = [1, 2, 3, 4]; // 4 tab thật (đã bỏ tab AI Chat — story-21 QA-B)

  tabs = signal<Tab[]>([
    { labelKey: 'crm.customer_note_ai_tabs.note',        icon: 'pi pi-file-edit' },
    { labelKey: 'crm.customer_note_ai_tabs.att_file',     icon: 'pi pi-file' },
    { labelKey: 'crm.customer_note_ai_tabs.att_audio',    icon: 'pi pi-volume-up' },
    { labelKey: 'crm.customer_note_ai_tabs.att_video',    icon: 'pi pi-video' },
  ]);
}
