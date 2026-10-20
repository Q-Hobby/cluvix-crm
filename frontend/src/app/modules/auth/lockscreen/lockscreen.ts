import { ChangeDetectionStrategy, Component } from '@angular/core';
import { RouterModule } from '@angular/router';
import { TranslatePipe } from '@ngx-translate/core';
import { ButtonModule } from 'primeng/button';
import { RippleModule } from 'primeng/ripple';

@Component({
  changeDetection: ChangeDetectionStrategy.OnPush,
  selector: 'app-lockscreen',
  standalone: true,
  imports: [RouterModule, ButtonModule, RippleModule, TranslatePipe],
  template: `
    <main class="min-h-screen flex items-center justify-center bg-surface-0 dark:bg-surface-950 px-6 py-16">
      <section class="flex flex-col items-center gap-8 text-center max-w-md w-full mx-auto">
        <span
          class="flex items-center justify-center w-24 h-24 rounded-full bg-surface-100 dark:bg-surface-800 text-surface-700 dark:text-surface-0"
          aria-hidden="true"
        >
          <i class="pi pi-lock text-5xl"></i>
        </span>
        <div class="flex flex-col gap-3">
          <h1 class="text-4xl font-bold leading-tight text-surface-900 dark:text-surface-0">
            {{ 'auth.lockscreen.title' | translate }}
          </h1>
          <p class="text-lg leading-normal text-surface-600 dark:text-surface-300 break-words">
            {{ 'auth.lockscreen.message' | translate }}
          </p>
        </div>
        <button pButton pRipple type="button" [routerLink]="['/login']">
          <i pButtonIcon class="pi pi-sign-in"></i>
          <span pButtonLabel>{{ 'auth.login.title' | translate }}</span>
        </button>
      </section>
    </main>
  `,
})
export class LockScreen {}
