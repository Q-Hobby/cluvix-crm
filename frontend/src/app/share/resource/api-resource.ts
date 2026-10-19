import { httpResource, HttpResourceRef, HttpResourceRequest } from '@angular/common/http';
import { HttpParams } from '@angular/common/http';
import { environment } from '@env/environment';

const BASE_URL = environment.apiUrl.replace(/\/+$/, '');

export interface ApiResourceRequest {
  /** Path relative to apiUrl, e.g. 'crm/patient/123' */
  path: string;
  /** Query params */
  params?: Record<string, string | number | boolean | ReadonlyArray<string | number | boolean>>;
}

/**
 * Signal-based GET wrapper using httpResource.
 * Handles baseUrl + withCredentials automatically.
 *
 * Usage:
 *   readonly patient = apiResource<PatientModel>(
 *     () => ({ path: `crm/patient/${this.id()}` }),
 *     { parse: raw => raw as PatientModel }
 *   );
 *   // Template: patient.value() | patient.isLoading() | patient.error() | patient.reload()
 *
 * Return undefined from factory to suspend (e.g. when id not ready):
 *   () => this.id() ? { path: `crm/patient/${this.id()}` } : undefined
 */
export function apiResource<T>(
  requestFn: () => ApiResourceRequest | string | null | undefined,
  options?: {
    /** Transform raw JSON response — replaces afterLoad() */
    parse?: (raw: unknown) => T;
    /** Value while loading or idle (default: undefined) */
    defaultValue?: T;
  },
): HttpResourceRef<T | undefined> {
  return httpResource<T>(
    () => {
      const req = requestFn();
      if (req == null) return undefined;

      const path = typeof req === 'string' ? req : req.path;
      const params = typeof req === 'string' ? undefined : req.params;

      const httpReq: HttpResourceRequest = {
        url: `${BASE_URL}/${path.replace(/^\/+/, '')}`,
        withCredentials: true,
      };

      if (params) httpReq.params = params;

      return httpReq;
    },
    {
      parse: options?.parse,
      defaultValue: options?.defaultValue,
    },
  );
}
