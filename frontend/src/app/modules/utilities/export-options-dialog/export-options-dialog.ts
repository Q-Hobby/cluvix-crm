import { ChangeDetectionStrategy, Component, inject, signal } from '@angular/core';
import { FormsModule } from '@angular/forms';
import { TranslatePipe } from '@ngx-translate/core';
import { ButtonModule } from 'primeng/button';
import { RadioButton } from 'primeng/radiobutton';
import { DynamicDialogConfig, DynamicDialogRef } from 'primeng/dynamicdialog';

export type ExportMode = 'all' | 'selected';

export interface ExportOptionsData {
  /** Number of currently visible columns */
  selectedCount: number;
  /** Total number of available columns */
  totalCount: number;
}

@Component({
  changeDetection: ChangeDetectionStrategy.OnPush,
  selector: 'app-export-options-dialog',
  templateUrl: './export-options-dialog.html',
  standalone: true,
  imports: [ButtonModule, RadioButton, FormsModule, TranslatePipe],
})
export class ExportOptionsDialog {
  ref = inject(DynamicDialogRef);
  data: ExportOptionsData = inject(DynamicDialogConfig).data;

  mode = signal<ExportMode>('selected');

  confirm() {
    this.ref.close(this.mode());
  }

  cancel() {
    this.ref.close(null);
  }
}
