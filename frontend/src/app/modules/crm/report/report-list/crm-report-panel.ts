import {ChangeDetectionStrategy, Component, Input } from '@angular/core';
import { ListShareImports } from '@/share/import/list-share-imports';
import {RouterLinkActive} from "@angular/router";

@Component({
  changeDetection: ChangeDetectionStrategy.OnPush,
  selector: 'app-report-panel',
  standalone: true,
  templateUrl: './crm-report-panel.html',
  imports: [...ListShareImports, RouterLinkActive],
})
export class CrmReportPanel {
  @Input() title!: string;
  @Input() icon = 'pi pi-cog';
  @Input() items: {
    label: string;
    labelKey?: string;
    link: string;
    icon?: string;
    iconClass?: string;
    description?: string;
    descriptionKey?: string;
  }[] = [];
}
