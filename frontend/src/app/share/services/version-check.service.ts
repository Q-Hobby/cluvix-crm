import { HttpClient } from '@angular/common/http';
import { inject, Service} from '@angular/core';



@Service()
export class VersionCheckService {
  public dialogVisible: boolean = false;
  public bc;
  private http = inject(HttpClient);
  // this will be replaced by actual hash post-build.js
  private version = '{{POST_BUILD_ENTERS_HASH_HERE}}';

  constructor() {
    this.bc = new BroadcastChannel('UpdateVersionChannel');
    // A handler that only logs the event to the console:
    this.bc.onmessage = (event: any) => {
      if (event.data === 'NewVersion') {
        location.reload();
      }
    };
  }

  /**
   * Checks in every set frequency the version of frontend application
   * @param {number} frequency - in milliseconds, defaults to 30 minutes
   */
  public initVersionCheck(frequency: any = 1000 * 60 * 30) {
    setInterval(() => {
      this.checkVersion();
    }, frequency);
  }

  private checkVersion() {
    // timestamp these requests to invalidate caches
    this.http.get(location.origin + '/version.json?t=' + Date.now()).subscribe({
      next: (response: any) => {
        const version = response.version;
        const versionChanged = this.hasHashChanged(this.version, version);

        // If new version, do something
        if (versionChanged) {
          if (!this.dialogVisible) {
            this.dialogVisible = true;
          }
        }
        // store the new hash, so we wouldn't trigger versionChange again
        // only necessary in case you did not force refresh
        this.version = version;
      },
      error: (err: any) => {
        console.error(err, 'Could not get version');
      },
    });
  }

  /**
   * Checks if Version has changed.
   * This file has the JS Version, if it is a different one than in the version.json
   * we are dealing with version change
   * @param currentVersion
   * @param newVersion
   * @returns {boolean}
   */
  private hasHashChanged(currentVersion: string, newVersion: string) {
    if (!currentVersion || currentVersion === '{{POST_BUILD_ENTERS_HASH_HERE}}') {
      return false;
    }

    return currentVersion !== newVersion;
  }
}
