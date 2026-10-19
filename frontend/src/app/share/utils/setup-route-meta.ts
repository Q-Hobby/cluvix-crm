import { DestroyRef, EnvironmentInjector, inject, runInInjectionContext } from '@angular/core';
import { takeUntilDestroyed } from '@angular/core/rxjs-interop';
import { Title } from '@angular/platform-browser';
import { ActivatedRouteSnapshot, NavigationEnd, Router } from '@angular/router';
import { TranslateService } from '@ngx-translate/core';
import { filter } from 'rxjs';

function findDeepestRoute(snapshot: ActivatedRouteSnapshot): ActivatedRouteSnapshot {
  let route = snapshot;
  while (route.firstChild) {
    route = route.firstChild;
  }
  return route;
}

function tryTranslate(translate: TranslateService, key: string): Promise<string> {
  return new Promise((resolve: any) => {
    translate.get(key).subscribe((translated: any) => {
      resolve(translated === key ? key : translated);
    });
  });
}

export async function setupRouteMeta(envInjector: EnvironmentInjector): Promise<void> {
  runInInjectionContext(envInjector, () => {
    const router = inject(Router);
    const titleService = inject(Title);
    const translate = inject(TranslateService);
    const destroyRef = inject(DestroyRef);

    /**
     * Chỉ path (bỏ query/fragment) mới quyết định title của route.
     * Lý do: màn detail điều hướng đổi query param để nhớ tab đang mở
     * (vd CustomerDetail `?tab=`); nếu NavigationEnd nào cũng set lại title theo
     * route data thì title do component tự đặt (tên khách hàng, tên CTV...) bị
     * ghi đè ngay khi user chuyển tab.
     */
    let lastPath: string | null = null;

    const handleTitle = async (url: string) => {
      const path = url.split(/[?#]/)[0];
      if (path === lastPath) return;
      lastPath = path;

      const route = findDeepestRoute(router.routerState.snapshot.root);
      const rawTitle = route.data?.['title'];
      if (rawTitle) {
        const translated = await tryTranslate(translate, rawTitle);
        titleService.setTitle(translated);
      }
    };

    // ✅ Xử lý ngay lần đầu khi vào app
    handleTitle(router.url).catch(console.error);

    // ✅ Và mỗi lần chuyển route
    router.events.pipe(filter((event: any) => event instanceof NavigationEnd), takeUntilDestroyed(destroyRef)).subscribe((event: NavigationEnd) => {
      handleTitle(event.urlAfterRedirects || event.url).catch(console.error);
    });
  });
}
