import {
  Directive,
  ElementRef,
  HostListener,
  inject,
  Input,
  OnInit,
  Renderer2,
} from '@angular/core';

import { NotificationService } from '@/share/services/notification.service';

@Directive({
  selector: '[appCopyText]',
  standalone: true,
})
export class CopyTextDirective implements OnInit {
  @Input('appCopyText') textToCopy: string | null | undefined;
  private el = inject(ElementRef);
  private renderer = inject(Renderer2);
  private noti = inject(NotificationService);

  ngOnInit() {
    const el = this.el.nativeElement;
    this.renderer.addClass(el, 'cursor-pointer');
    // Element gắn directive này thường là <span>/<td>/<b> (không phải <button>) — bù lại keyboard access.
    if (!el.hasAttribute('tabindex')) {
      this.renderer.setAttribute(el, 'tabindex', '0');
    }
    if (!el.hasAttribute('role')) {
      this.renderer.setAttribute(el, 'role', 'button');
    }
  }

  @HostListener('click')
  @HostListener('keydown.enter')
  @HostListener('keydown.space', ['$event'])
  copyToClipboard(event?: Event) {
    event?.preventDefault(); // chặn space cuộn trang khi kích hoạt bằng bàn phím
    if (!this.textToCopy) return;

    const text = this.textToCopy;
    navigator.clipboard
      .writeText(text)
      .then(() => {
        this.noti.success('Đã sao chép', text);
      })
      .catch((err: any) => {
        this.noti.error('Lỗi sao chép', err.message || 'Không thể sao chép');
      });
  }
}
