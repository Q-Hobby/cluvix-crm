// 📁 share/utils/catch-api-error.ts
import { inject } from '@angular/core';
import { catchError, throwError } from 'rxjs';

import { ApiErrorHandlerService } from '../services/api-error-handler.service';

export function catchApiError(config: { silent?: boolean; redirect404?: string } = {}) {
  return catchError((error) => {
    const handler = inject(ApiErrorHandlerService);
    handler.handle(error, config); // ❌ không return — đã throw error trong handle
    return throwError(() => error);
  });
}
