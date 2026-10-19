import { ChangeDetectionStrategy, Component, Input } from '@angular/core';

/**
 * Loader tím thương hiệu Cluvix — box 9 ô morph, dùng chung cho:
 * app-loader.ts (global overlay), syncing.ts, index.html (bản HTML thuần, không dùng component này được vì chạy trước khi Angular bootstrap).
 */
@Component({
  changeDetection: ChangeDetectionStrategy.OnPush,
  selector: 'app-brand-loader',
  standalone: true,
  imports: [],
  template: `
    <div class="brand-loader-box">
      <div class="brand-loader" [style.--brand-loader-color]="color"></div>
    </div>
    @if (text) {
      <div class="brand-loader-text" [style.color]="textColor">{{ text }}</div>
    }
  `,
  styles: [
    `
      :host {
        display: flex;
        flex-direction: column;
        align-items: center;
        gap: 16px;
      }

      .brand-loader-box {
        width: 65px;
        height: 65px;
        display: flex;
        align-items: center;
        justify-content: center;
      }

      .brand-loader {
        --c: no-repeat linear-gradient(var(--brand-loader-color), var(--brand-loader-color));
        background:
          var(--c), var(--c), var(--c), var(--c), var(--c), var(--c), var(--c), var(--c), var(--c);
        background-size: 16px 16px;
        animation:
          brand-loader-1 1s infinite,
          brand-loader-2 1s infinite;
        will-change: width, height, background-position;
      }

      .brand-loader-text {
        font-size: 1.2308rem;
        font-weight: 500;
      }

      @keyframes brand-loader-1 {
        0%,
        100% {
          width: 45px;
          height: 45px;
        }
        35%,
        65% {
          width: 65px;
          height: 65px;
        }
      }

      @keyframes brand-loader-2 {
        0%,
        40% {
          background-position:
            0 0,
            0 50%,
            0 100%,
            50% 100%,
            100% 100%,
            100% 50%,
            100% 0,
            50% 0,
            50% 50%;
        }
        60%,
        100% {
          background-position:
            0 50%,
            0 100%,
            50% 100%,
            100% 100%,
            100% 50%,
            100% 0,
            50% 0,
            0 0,
            50% 50%;
        }
      }
    `,
  ],
})
export class BrandLoader {
  @Input() color = '#7B3F9E';
  @Input() textColor = '#fff';
  @Input() text?: string;
}
