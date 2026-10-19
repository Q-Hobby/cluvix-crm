import {
  HttpErrorResponse,
  HttpHandlerFn,
  HttpInterceptorFn,
  HttpRequest,
} from '@angular/common/http';
import { inject } from '@angular/core';
import { TranslateService } from '@ngx-translate/core';
import { throwError } from 'rxjs';

import { NotificationService } from '@/share/services/notification.service';

export const requestLengthInterceptor: HttpInterceptorFn = (
  req: HttpRequest<any>,
  next: HttpHandlerFn,
) => {
  const noti: NotificationService = inject(NotificationService);
  const translate = inject(TranslateService);
  const MAX_URL_LENGTH = 1000;

  if (req.urlWithParams.length > MAX_URL_LENGTH) {
    noti.warnMsg(translate.instant('common.message.too_many_selected'));

    return throwError(() => new HttpErrorResponse({
      status: 414,
      statusText: 'Request URI Too Long',
      url: req.urlWithParams,
      error: 'Request URL is too long',
    }));
  }

  return next(req);
};
