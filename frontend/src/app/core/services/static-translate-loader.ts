import { TranslateLoader } from '@ngx-translate/core';
import { from, Observable, of } from 'rxjs';

import viVN from '@assets/i18n/vi-VN.json';

// Sprint B: Static loader for prod build.
//
// vi-VN (default language, ~95% users) inline trong main.js → 0 HTTP fetch.
// en-US lazy chunk via dynamic import → fetched only khi user switch sang EN.
// Other langs (ja/fr/...) khi cần thì add vào lazyMap.
//
// Tradeoffs vs CachedTranslateLoader (HTTP-based):
// - Pro: 0 HTTP cho default lang. Build hash → cache forever. Bundle inline.
// - Con: Main.js +~20KB gzip (vi-VN). Update content cần rebuild + redeploy.
//
// Sprint B targets PROD build only. Dev tiếp tục dùng CachedTranslateLoader để
// translators có thể edit JSON + hot reload không cần rebuild.
const STATIC_TRANSLATIONS: Record<string, any> = {
  'vi-VN': viVN,
};

const LAZY_TRANSLATIONS: Record<string, () => Promise<any>> = {
  'en-US': () => import('@assets/i18n/en-US.json'),
};

export class StaticTranslateLoader implements TranslateLoader {
  getTranslation(lang: string): Observable<any> {
    const inline = STATIC_TRANSLATIONS[lang];
    if (inline) return of(inline);

    const lazy = LAZY_TRANSLATIONS[lang];
    if (lazy) {
      return from(lazy().then((m: any) => m?.default ?? m).catch(() => ({})));
    }

    return of({});
  }
}
