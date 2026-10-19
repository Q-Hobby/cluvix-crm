import { Pipe, PipeTransform } from '@angular/core';

@Pipe({
  name: 'replacePageBreak',
  standalone: true,
})
export class ReplacePageBreakPipe implements PipeTransform {
  transform(
    value: string | null,
    pageBreakSeparator: string = '<!-- my page break -->',
    replaceValue: any = `
            </div>
        </div>
            <!--<div style="page-break-before: always; clear:both"/></div>-->
        <div class="sheet">
            <div class="sheet_body">`,
  ): string {
    if (!value) {
      return '';
    }
    const startHtml = `<div class="sheet">
                        <div class="sheet_body">`;
    const endHtml = `</div>
                    </div>`;
    const repValue = startHtml + value.split(pageBreakSeparator).join(replaceValue) + endHtml;
    return repValue;
  }
}
