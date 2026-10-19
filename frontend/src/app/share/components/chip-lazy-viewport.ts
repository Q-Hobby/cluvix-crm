// chip-lazy-viewport.ts
import { NgOptimizedImage, TitleCasePipe } from '@angular/common';
import { ChangeDetectionStrategy, Component, EventEmitter, Input, Output } from '@angular/core';
import { ChipModule } from 'primeng/chip';

type MaybeStr = string | null | undefined;

@Component({
  selector: 'app-chip-lazy',
  standalone: true,
  imports: [ChipModule, TitleCasePipe, NgOptimizedImage],
  changeDetection: ChangeDetectionStrategy.OnPush,
  styles: [
    `
      :host ::ng-deep .app-chip {
        display: inline-flex;
        align-items: center;
        border-radius: 9999px;
        height: 32px;
        padding: 0 0.75rem 0 0.375rem;
        gap: 0.5rem;
      }

      :host ::ng-deep .app-chip .p-chip-text {
        line-height: 1;
      }

      .wrap {
        position: relative; /* cần cho fill */
        border-radius: 9999px;
        background: #fff;
        box-shadow: 0 0 0 1px rgba(0, 0, 0, 0.08);
        display: flex;
        align-items: center;
        justify-content: center;
        overflow: hidden;
      }

      .img {
        border-radius: 9999px;
        object-fit: cover; /* crop vuông đẹp */
        display: block;
      }

      .skel {
        border-radius: 9999px;
        background: #e5e7eb;
      }
    `,
  ],
  template: `
    <p-chip class="app-chip">
      @defer (on viewport) {
        @if (currentSrc) {
          <span class="wrap" [style.width.px]="avatarSize" [style.height.px]="avatarSize">
            <img
              [ngSrc]="currentSrc"
              fill
              loading="lazy"
              decoding="async"
              fetchpriority="low"
              class="img"
              (error)="onImgError($event)"
              [attr.alt]="alt || ''"
            />
          </span>
        } @else {
          <span class="wrap" [style.width.px]="avatarSize" [style.height.px]="avatarSize">
            <span class="skel" [style.width.px]="avatarSize" [style.height.px]="avatarSize"></span>
          </span>
        }
      } @placeholder {
        <span class="skel" [style.width.px]="avatarSize" [style.height.px]="avatarSize"></span>
      } @loading {
        <span class="skel" [style.width.px]="avatarSize" [style.height.px]="avatarSize"></span>
      }

      <span
        class="p-chip-text font-medium truncate min-w-0"
        [title]="label"
        [style.max-width.px]="labelMax"
      >
        {{ label | titlecase }}
      </span>
    </p-chip>
  `,
})
export class ChipLazyViewport {
  @Input() fallbackSrc = 'assets/overrides/images/no-avatar.svg';
  @Input() label: string = '';
  @Input() alt?: string;
  @Input() size: 28 | 32 | 36 = 32;
  @Input() labelMax = 160;
  @Output() imageError = new EventEmitter<Event>();
  currentSrc = '';
  private triedFallback = false;

  private _src?: MaybeStr;

  get src(): MaybeStr {
    return this._src;
  }

  @Input() set src(v: MaybeStr) {
    this._src = v;
    const s = (v ?? '').trim();
    this.triedFallback = false;
    this.currentSrc = s || '';
  }

  get avatarSize() {
    return this.size === 36 ? 28 : this.size === 28 ? 20 : 24;
  }

  onImgError(ev: Event) {
    if (!this.triedFallback) {
      this.triedFallback = true;
      this.currentSrc = this.fallbackSrc;
    }
    this.imageError.emit(ev);
  }
}
