import { ChangeDetectionStrategy, Component } from '@angular/core';
import { BaseSelectIndexed } from '../base/base-select-indexed';
import { UserProfileModel } from '@/core/models/user-profile.model';

@Component({
  changeDetection: ChangeDetectionStrategy.OnPush,
  selector: 'app-user-select',
  standalone: true,
  imports: [BaseSelectIndexed],
  template: `
    <app-base-select-indexed
      tableName="users"
      optionLabel="fullname"
      [value]="value"
      (valueChange)="valueChange.emit($event)"
      [activeOnly]="activeOnly"
      [allowedStatuses]="allowedStatuses"
      [appendTo]="appendTo"
      [filter]="filter"
      [resetFilterOnHide]="resetFilterOnHide"
      [virtualScroll]="virtualScroll"
      [virtualScrollItemSize]="virtualScrollItemSize"
      [emptyFilterMessage]="emptyFilterMessage"
      [placeholder]="placeholder"
    >
    </app-base-select-indexed>
  `,
})
export class UserSelect extends BaseSelectIndexed<UserProfileModel> {
  override tableName = 'users'; // Fix: ngOnInit kế thừa từ base cần tableName thật, không phải giá trị mặc định undefined
  override allowedStatuses = [9, 10]; // User có nhiều status hơn
}
