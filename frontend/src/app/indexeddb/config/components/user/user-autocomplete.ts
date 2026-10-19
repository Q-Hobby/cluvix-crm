import { ChangeDetectionStrategy, Component, EventEmitter, Input, OnInit, Output } from '@angular/core';
import { BaseAutoCompleteIndexed } from '@/indexeddb/config/components/base/base-autocomplete-indexed';
import { UserCacheService } from './user-cache.service';

@Component({
  changeDetection: ChangeDetectionStrategy.OnPush,
  selector: 'app-user-autocomplete',
  standalone: true,
  imports: [BaseAutoCompleteIndexed],
  template: `
    <app-base-autocomplete-indexed
      [tableName]="'users'"
      [optionLabel]="'fullname'"
      [activeOnly]="activeOnly"
      [allowedStatuses]="allowedStatuses"
      [appendTo]="appendTo"
      [emptyFilterMessage]="emptyFilterMessage"
      [value]="value"
      (valueChange)="valueChange.emit($event)"
    >
    </app-base-autocomplete-indexed>
  `,
})
export class UserAutoComplete implements OnInit {
  @Input() value: number | null = null;
  @Input() activeOnly: boolean = false;
  @Input() allowedStatuses: number[] = [9, 10];
  @Input() appendTo: any = 'body';
  @Input() emptyFilterMessage: string = '';
  @Output() valueChange = new EventEmitter<number | null>();

  constructor(private userCache: UserCacheService) {}

  async ngOnInit() {
    await this.userCache.loadUsers();
  }
}
