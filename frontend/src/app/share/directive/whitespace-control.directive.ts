import { AfterViewInit, Directive, ElementRef, inject, Renderer2 } from '@angular/core';

@Directive({
  selector: '[appWhitespaceControl]',
  standalone: true,
})
// Chỉ áp dụng whitespace-pre-wrap cho html ngoài img
export class WhitespaceControlDirective implements AfterViewInit {
  private el = inject(ElementRef);
  private renderer = inject(Renderer2);

  ngAfterViewInit() {
    const hasImage = this.el.nativeElement.querySelector('img');
    if (!hasImage) {
      this.renderer.addClass(this.el.nativeElement, 'whitespace-pre-wrap');
    }
  }
}
