// ✅ Reusable Column Toggler Component for Angular + PrimeNG (Safe with Signals)

import {ChangeDetectionStrategy, Component, EventEmitter, Input, Output, Signal } from '@angular/core';
import { FormsModule } from '@angular/forms';
import { TranslatePipe } from '@ngx-translate/core';
import { CheckboxModule } from 'primeng/checkbox';

@Component({
  changeDetection: ChangeDetectionStrategy.OnPush,
  selector: 'app-column-toggle',
  standalone: true,
  imports: [CheckboxModule, FormsModule, TranslatePipe],
  template: `
    <div class="grid w-3xs max-h-96 overflow-auto">
      @for (col of columns; track col.value) {
        <div class="flex items-center gap-2 mb-1">
          <p-checkbox
            name="displayCols"
            [value]="col.value"
            [ngModel]="selectedCols()"
            (ngModelChange)="onChangeToggle($event)"
          ></p-checkbox>
          <label>{{ (col.labelKey || col.label) | translate }}</label>
        </div>
      }
    </div>
  `,
})
export class AppColumnToggle {
  @Input({ required: true }) selectedCols!: Signal<string[]>;
  @Input({ required: true }) columns!: { label: string; labelKey?: string; value: string }[];
  @Output() changeToggle = new EventEmitter<string[]>();

  onChangeToggle(updated: string[]) {
    this.changeToggle.emit(updated);
  }
}
