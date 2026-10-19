import { Component, inject, ChangeDetectionStrategy } from '@angular/core';

import { RouteProgressService } from '@/share/services/route-progress.service';

@Component({
  selector: 'app-route-progress-bar',
  standalone: true,
  imports: [],
  template: `
    <div class="route-progress" [class.route-progress-visible]="isVisible()">
      <div class="route-progress-bar"></div>
    </div>
  `,
  changeDetection: ChangeDetectionStrategy.Eager,
  styles: [
    `
      .route-progress {
        position: fixed;
        top: 0;
        left: 0;
        right: 0;
        height: 3px;
        z-index: 10001;
        overflow: hidden;
        opacity: 0;
        pointer-events: none;
        transition: opacity 200ms ease;
      }

      .route-progress-visible {
        opacity: 1;
      }

      .route-progress-bar {
        position: absolute;
        top: 0;
        left: -35%;
        height: 100%;
        width: 35%;
        background: linear-gradient(90deg, transparent, #7b3f9e, transparent);
        animation: route-progress-slide 1s ease-in-out infinite;
      }

      @keyframes route-progress-slide {
        0% {
          left: -35%;
        }
        100% {
          left: 100%;
        }
      }
    `,
  ],
})
export class RouteProgressBar {
  private routeProgress = inject(RouteProgressService);

  // ✅ Dùng computed để lấy trạng thái hiện tại
  isVisible = this.routeProgress.visible;
}
