import { Directive, HostListener } from '@angular/core';

@Directive({
  standalone: true,
  selector: '[alphabetOnly]',
})
export class AlphabetOnlyDirective {
  private regex: RegExp = new RegExp(/^[a-zA-Z]+$/);

  @HostListener('keypress', ['$event'])
  onKeyPress(event: KeyboardEvent) {
    const inputChar = String.fromCharCode(event.charCode);
    if (!this.regex.test(inputChar)) {
      event.preventDefault();
    }
  }
}
