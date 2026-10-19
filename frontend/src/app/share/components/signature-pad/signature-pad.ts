import { AfterViewInit, ChangeDetectionStrategy, Component, ElementRef, input, ViewChild } from '@angular/core';
import { CommonModule } from '@angular/common';
import { TranslatePipe } from '@ngx-translate/core';
import { ButtonModule } from 'primeng/button';

/**
 * Canvas ký tay thuần (không thư viện ngoài) — vẽ + xuất PNG Blob. Dùng cho chữ ký điện tử
 * (emr-print-forms-digitalization story-06). Không có component ký-tay nào có sẵn trong repo
 * trước đó (đã kiểm tra — chỉ có `print-signature-block` là hiển thị TRẠNG THÁI đã ký số TT46,
 * không phải canvas nhập liệu).
 */
@Component({
  changeDetection: ChangeDetectionStrategy.OnPush,
  selector: 'app-signature-pad',
  standalone: true,
  imports: [CommonModule, TranslatePipe, ButtonModule],
  template: `
    <div class="border border-surface-300 dark:border-surface-600 rounded-lg overflow-hidden bg-white">
      <canvas
        #canvas
        class="w-full touch-none cursor-crosshair block"
        [style.height.px]="height()"
        (pointerdown)="onPointerDown($event)"
        (pointermove)="onPointerMove($event)"
        (pointerup)="onPointerUp()"
        (pointerleave)="onPointerUp()"
      ></canvas>
    </div>
    <div class="flex justify-end mt-2">
      <button type="button" pButton size="small" severity="secondary" [text]="true" (click)="clear()">
        <i class="pi pi-refresh"></i>
        <span>{{ 'common.action.clear' | translate }}</span>
      </button>
    </div>
  `,
})
export class SignaturePad implements AfterViewInit {
  @ViewChild('canvas') private canvasRef!: ElementRef<HTMLCanvasElement>;

  height = input(180);

  private ctx!: CanvasRenderingContext2D;
  private drawing = false;
  private hasContent = false;

  ngAfterViewInit(): void {
    const canvas = this.canvasRef.nativeElement;
    const rect = canvas.getBoundingClientRect();
    const dpr = window.devicePixelRatio || 1;
    canvas.width = Math.max(rect.width, 1) * dpr;
    canvas.height = this.height() * dpr;
    const ctx = canvas.getContext('2d');
    if (!ctx) return;
    ctx.scale(dpr, dpr);
    ctx.lineWidth = 2;
    ctx.lineCap = 'round';
    ctx.strokeStyle = '#1e293b';
    this.ctx = ctx;
  }

  onPointerDown(e: PointerEvent): void {
    if (!this.ctx) return;
    this.drawing = true;
    this.hasContent = true;
    const { x, y } = this.pos(e);
    this.ctx.beginPath();
    this.ctx.moveTo(x, y);
  }

  onPointerMove(e: PointerEvent): void {
    if (!this.drawing || !this.ctx) return;
    const { x, y } = this.pos(e);
    this.ctx.lineTo(x, y);
    this.ctx.stroke();
  }

  onPointerUp(): void {
    this.drawing = false;
  }

  private pos(e: PointerEvent): { x: number; y: number } {
    const rect = this.canvasRef.nativeElement.getBoundingClientRect();
    return { x: e.clientX - rect.left, y: e.clientY - rect.top };
  }

  clear(): void {
    const canvas = this.canvasRef.nativeElement;
    this.ctx?.clearRect(0, 0, canvas.width, canvas.height);
    this.hasContent = false;
  }

  isEmpty(): boolean {
    return !this.hasContent;
  }

  toBlob(): Promise<Blob | null> {
    return new Promise((resolve) => this.canvasRef.nativeElement.toBlob((b) => resolve(b), 'image/png'));
  }
}
