import { DOCUMENT, inject, Service} from '@angular/core';
import { Observable, ReplaySubject } from 'rxjs';

@Service()
export class LazyLoadScriptService {
  loadedLibraries: { [url: string]: ReplaySubject<void> } = {};
  private readonly document = inject(DOCUMENT);

  loadScript(url: string): Observable<void> {
    if (this.loadedLibraries[url]) {
      return this.loadedLibraries[url].asObservable();
    }

    this.loadedLibraries[url] = new ReplaySubject();

    const script = this.document.createElement('script');
    script.type = 'text/javascript';
    script.src = url;
    script.onload = () => {
      this.loadedLibraries[url].next();
      this.loadedLibraries[url].complete();
    };

    this.document.body.appendChild(script);

    return this.loadedLibraries[url].asObservable();
  }
}
