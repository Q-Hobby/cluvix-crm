import { ChangeDetectionStrategy, Component, HostListener, Input, OnDestroy, OnInit, ViewChild } from '@angular/core';

@Component({
  changeDetection: ChangeDetectionStrategy.OnPush,
  selector: 'app-image-comparison',
  templateUrl: './image-comparison.html',
  standalone: true,
  styleUrls: ['./image-comparison.scss'],
})
export class ImageComparison implements OnInit, OnDestroy {
  public readonly FIRST_INDEX = 0;
  public readonly SECOND_INDEX = 1;

  public screenHeight!: number;
  public screenWidth!: number;
  @Input() public readonly images!: string[];
  isLoaded = 0;
  isInitComparison = false;
  @ViewChild('comparisonElement', { static: true }) private readonly comparisonElement;
  @Input() private maxHeight: string | number | null = null;
  @Input() private height: string | number | null = null;
  private cleanupFns: (() => void)[] = [];

  constructor() {}

  @HostListener('window:resize', ['$event'])
  getScreenSize() {
    this.screenHeight = window.innerHeight - 80;
    this.screenWidth = window.innerWidth;
    this.maxHeight = this.screenHeight;
    this.initComparison();
  }

  ngOnInit(): void {
    if (this.images.length !== 2) {
      return;
    }
  }

  ngOnDestroy(): void {
    this.cleanupFns.forEach((fn) => fn());
    this.cleanupFns = [];
  }

  dosomething() {
    this.isLoaded++;
    if (this.isLoaded === 2) {
      this.getScreenSize();
    }
  }

  private initComparison(): void {
    const comparisonElement = this.comparisonElement.nativeElement;

    const fixedImageElement = comparisonElement.querySelector('.fixed-image') as HTMLElement;
    const slideImageElement = comparisonElement.querySelector('.slide-image') as HTMLElement;
    const slider = comparisonElement.querySelector('.comparison__slider') as HTMLElement;

    if (this.maxHeight) {
      this.maxHeight = typeof this.maxHeight === 'number' ? `${this.maxHeight}px` : this.maxHeight;
      [].forEach.call(comparisonElement.querySelectorAll('img'), (img: any) => {
        img.style.maxHeight = this.maxHeight;
        img.style.height = this.maxHeight;
        img.classList.remove('h-0');
      });
    }

    let held = false;

    const getCursorPos = (event: any) => {
      const e = event || window.event;
      const a = fixedImageElement.getBoundingClientRect();
      const x = (e.pageX || e.touches[0].clientX) - a.left;
      return ((x - window.pageXOffset) / comparisonElement.clientWidth) * 100;
    };

    const slide = (x: any) => {
      slideImageElement.style.width = `${x}%`;
      slider.style.left = `${Math.min(Math.max(x, 0), 100)}%`;
    };

    const slideMove = (event: any) => {
      if (!held) {
        return false;
      }
      const position = getCursorPos(event);
      slide(position);
    };

    const onHoldSlider = (event: any) => {
      event.preventDefault();
      held = true;
    };
    const onRelease = () => (held = false);

    // Cleanup listeners từ lần init trước (tránh tích lũy khi resize)
    this.cleanupFns.forEach((fn) => fn());
    this.cleanupFns = [];

    window.addEventListener('mousemove', slideMove);
    window.addEventListener('touchmove', slideMove);
    window.addEventListener('mouseup', onRelease);
    window.addEventListener('touchend', onRelease);
    slider.addEventListener('mousedown', onHoldSlider);
    slider.addEventListener('touchstart', onHoldSlider);

    this.cleanupFns.push(
      () => window.removeEventListener('mousemove', slideMove),
      () => window.removeEventListener('touchmove', slideMove),
      () => window.removeEventListener('mouseup', onRelease),
      () => window.removeEventListener('touchend', onRelease),
      () => slider.removeEventListener('mousedown', onHoldSlider),
      () => slider.removeEventListener('touchstart', onHoldSlider),
    );

    slide(50);
    this.isInitComparison = true;
  }
}
