import { HttpInterceptorFn } from '@angular/common/http';
import { inject } from '@angular/core';
import { finalize } from 'rxjs/operators';

import { GlobalLoadingService } from '@/share/services/global-loading.service';
import { RouteProgressService } from '@/share/services/route-progress.service';

import { LOADING_LABEL_OVERRIDE, resolveLoadingLabelKey } from './loading-label.util';

// Danh sách URL không cần loading spinner (CUD) / progress bar (GET)
// ai-conversation/omnichat (omi/*) đã xoá khỏi BE + FE (module ai, omnichat không còn ở CRM
// standalone) — 2 nhóm URL cũ bỏ khỏi danh sách này (story-22, deferred-work story-54 QA-B).
const skipLoadingUrls = [
  '/version.json',
];

export const loadingInterceptor: HttpInterceptorFn = (req: any, next: any) => {
  const loadingService = inject(GlobalLoadingService);
  const routeProgress = inject(RouteProgressService);

  const shouldSkipLoading = skipLoadingUrls.some(url => req.url.includes(url));
  if (shouldSkipLoading) return next(req);

  // CUD (POST/PUT/PATCH/DELETE) → overlay tím chặn tương tác toàn trang.
  if (req.method !== 'GET') {
    const labelKey = req.context.get(LOADING_LABEL_OVERRIDE) ?? resolveLoadingLabelKey(req.method, req.url);
    return loadingService.track(req.url.split('?')[0], next(req), labelKey);
  }

  // GET → top progress bar, không chặn tương tác — feedback tức thời khi mạng chậm/nav.
  routeProgress.start();
  return next(req).pipe(finalize(() => routeProgress.stop()));
};
