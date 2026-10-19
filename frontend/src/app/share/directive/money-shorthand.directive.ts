import { Directive, ElementRef, HostListener, inject } from '@angular/core';

@Directive({
  selector: '[appMoneyShorthand]',
})
export class MoneyShorthandDirective {
  private el = inject(ElementRef<HTMLInputElement>);

  @HostListener('blur')
  onBlur() {
    const raw = this.el.nativeElement.value.trim().toLowerCase();
    if (!raw) return;

    this.el.nativeElement.value = this.parseMoney(raw).toLocaleString('en-US');
  }

  @HostListener('keydown.enter', ['$event'])
  onEnter(event: KeyboardEvent) {
    this.onBlur();
  }

  private parseMoney(val: string): number {
    // Remove commas, spaces
    const cleaned = val.replace(/,/g, '').replace(/\s/g, '');

    if (cleaned.endsWith('k')) {
      return parseFloat(cleaned.replace('k', '')) * 1_000;
    }
    if (cleaned.endsWith('m')) {
      return parseFloat(cleaned.replace('m', '')) * 1_000_000;
    }
    if (cleaned.endsWith('b')) {
      return parseFloat(cleaned.replace('b', '')) * 1_000_000_000;
    }
    // Nếu chỉ là số thường
    return parseFloat(cleaned) || 0;
  }
}
