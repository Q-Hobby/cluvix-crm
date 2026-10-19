import { ChangeDetectionStrategy, Component, inject } from '@angular/core';
import { Router } from '@angular/router';

import { FormShareImports } from '@/share/import/form-share-imports';

@Component({
  changeDetection: ChangeDetectionStrategy.OnPush,
  selector: 'app-print-template-page',
  templateUrl: './print-template-page.html',
  imports: [...FormShareImports],
  standalone: true,
})
export class PrintTemplatePage {
  private router = inject(Router);

  goBack() {
    this.router.navigate(['/config/print-template']);
  }
}
