// app-avatar-lazy.ts
import {
  ChangeDetectionStrategy,
  Component,
  computed,
  Input,
  OnChanges,
  signal,
  SimpleChanges,
} from '@angular/core';
import { ImageLightbox } from '@/share/components/image-lightbox';

@Component({
  selector: 'app-avatar-lazy',
  standalone: true,
  imports: [ImageLightbox],
  changeDetection: ChangeDetectionStrategy.OnPush,
  styles: [
    `
      :host {
        display: block !important;
        line-height: 0 !important;
      }

      .wrap {
        position: relative !important;
        overflow: hidden !important;
        border-radius: 50% !important;
        background: var(--p-content-hover-background);
        width: var(--avatar-size) !important;
        height: var(--avatar-size) !important;
        min-width: var(--avatar-size) !important;
        min-height: var(--avatar-size) !important;
        max-width: var(--avatar-size) !important;
        max-height: var(--avatar-size) !important;
        flex-shrink: 0;
        display: flex !important;
        align-items: center !important;
        justify-content: center !important;
        box-shadow: 0 0 0 1px var(--p-content-border-color);
        line-height: 0 !important;
        vertical-align: middle !important;
      }

      .img {
        position: absolute !important;
        top: 0 !important;
        left: 0 !important;
        width: 100% !important;
        height: 100% !important;
        object-fit: cover !important;
        object-position: center !important;
        display: block !important;
        border-radius: 0 !important;
        transition: opacity 0.2s ease;
      }

      .skel {
        position: absolute !important;
        top: 0 !important;
        left: 0 !important;
        width: 100% !important;
        height: 100% !important;
        background: linear-gradient(
          90deg,
          var(--p-content-border-color) 25%,
          var(--p-content-hover-background) 50%,
          var(--p-content-border-color) 75%
        );
        background-size: 200% 100%;
        animation: shimmer 1.5s infinite;
        border-radius: 50%;
      }

      @keyframes shimmer {
        0% {
          background-position: -200% 0;
        }
        100% {
          background-position: 200% 0;
        }
      }

      .initials {
        background: linear-gradient(135deg, #7b3f9e 0%, #6a3589 100%);
        color: white;
        font-weight: 600;
        border-radius: 50%;
        width: 100%;
        height: 100%;
        display: flex;
        align-items: center;
        justify-content: center;
        font-size: 0.875rem;
        user-select: none;
      }
    `,
  ],
  template: `
    @if (shouldShowLightbox()) {
      <app-image-lightbox
        [src]="originalSrc || currentSrc()!"
        [alt]="altText"
        [size]="size"
        [rounded]="true"
        [ring]="true"
        [lightbox]="true"
      />
    } @else {
      <div class="wrap" [style.--avatar-size.px]="size">
        @if (currentSrc()) {
          <img
            [src]="currentSrc()!"
            loading="lazy"
            decoding="async"
            class="img"
            [class.opacity-0]="!loaded()"
            (load)="onImgLoad()"
            (error)="onImgError()"
            [attr.alt]="altText"
          />
          @if (!loaded()) {
            <div class="skel"></div>
          }
        } @else {
          <div class="initials">
            {{ initials() }}
          </div>
        }
      </div>
    }
  `,
})
export class AvatarLazy implements OnChanges {
  @Input() src: string | null | undefined = '';
  @Input() fallbackSrc = 'assets/overrides/images/no-avatar.svg';
  @Input() name: string | null | undefined = '';
  @Input() size = 36;
  @Input() lightbox = false;
  @Input() originalSrc?: string | null;
  @Input() rounded = true;

  loaded = signal(false);
  currentSrc = signal('');
  // Computed để check có nên show lightbox không
  shouldShowLightbox = computed(() => {
    const src = this.currentSrc();
    return this.lightbox && this.loaded() && src && src !== this.fallbackSrc;
  });
  // Computed cho initials - cached
  initials = computed(() => {
    const n = (this.name || '').trim();
    if (!n) return '?';

    const words = n.split(/\s+/).filter(Boolean);
    if (words.length === 0) return '?';
    if (words.length === 1) return words[0][0].toUpperCase();

    return (words[0][0] + words[words.length - 1][0]).toUpperCase();
  });
  private triedFallback = false;

  ngOnChanges(changes: SimpleChanges): void {
    if ('src' in changes) {
      const s = (this.src ?? '').trim();
      if (s !== this.currentSrc()) {
        this.loaded.set(false);
        this.triedFallback = false;
        this.currentSrc.set(s);
      }
    }
  }

  get altText() {
    return this.name ? `Avatar của ${this.name}` : 'Avatar';
  }

  onImgLoad() {
    this.loaded.set(true);
  }

  onImgError() {
    if (!this.triedFallback && this.fallbackSrc) {
      this.triedFallback = true;
      this.currentSrc.set(this.fallbackSrc);
      return;
    }

    // Fallback failed → show initials
    this.currentSrc.set('');
    this.loaded.set(true);
  }
}
