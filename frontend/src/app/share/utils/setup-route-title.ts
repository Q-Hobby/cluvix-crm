import { DestroyRef, EnvironmentInjector, inject, runInInjectionContext } from '@angular/core';
import { takeUntilDestroyed } from '@angular/core/rxjs-interop';
import { Title } from '@angular/platform-browser';
import { ActivatedRouteSnapshot, NavigationEnd, Router } from '@angular/router';
import { TranslateService } from '@ngx-translate/core';
import { filter } from 'rxjs/operators';

function findDeepestTitle(snapshot: ActivatedRouteSnapshot): string | undefined {
  let route = snapshot;
  while (route.firstChild) {
    route = route.firstChild;
  }
  return route.data?.['title'];
}

export function setupRouteTitle(envInjector: EnvironmentInjector): void {
  runInInjectionContext(envInjector, () => {
    const router = inject(Router);
    const title = inject(Title);
    const translate = inject(TranslateService);
    const destroyRef = inject(DestroyRef);

    router.events.pipe(filter((event: any) => event instanceof NavigationEnd), takeUntilDestroyed(destroyRef)).subscribe(() => {
      const titleKey = findDeepestTitle(router.routerState.snapshot.root);
      if (!titleKey) return;

      if (titleKey.startsWith('PAGE_TITLE.')) {
        translate.get(titleKey).subscribe((translated: any) => title.setTitle(translated));
      } else {
        title.setTitle(titleKey);
      }
    });
  });
}
