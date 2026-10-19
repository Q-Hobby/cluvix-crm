import { inject, Service} from '@angular/core';
import { BaseIndexedDBService } from '@/indexeddb/config/service/base-indexed-db.service';
import { UserModel } from '@/core/models/hr/user.model';

export interface UserAuthSession {
  userId: number;
  user: UserModel;
  permissions: string[];
  /** Unix timestamp từ BE (X-User-Auth-Version). 0 = mới login, chưa có version từ BE. */
  authVersion: number;
}

const STORE = 'userAuthSession';

@Service()
export class UserAuthSessionService {
  private idb = inject(BaseIndexedDBService);

  async save(session: UserAuthSession): Promise<void> {
    try {
      await this.idb.putItem<UserAuthSession>(STORE, session);
    } catch (err) {
      console.error('[UserAuthSession] save failed:', err);
    }
  }

  async load(userId: number): Promise<UserAuthSession | null> {
    try {
      return await this.idb.getItem<UserAuthSession>(STORE, userId);
    } catch (err) {
      console.error('[UserAuthSession] load failed:', err);
      return null;
    }
  }

  async updateAuthVersion(userId: number, authVersion: number): Promise<void> {
    try {
      const existing = await this.load(userId);
      if (!existing) return;
      await this.idb.putItem<UserAuthSession>(STORE, { ...existing, authVersion });
    } catch (err) {
      console.error('[UserAuthSession] updateAuthVersion failed:', err);
    }
  }

  async clearUser(userId: number): Promise<void> {
    try {
      await this.idb.deleteItem(STORE, userId);
    } catch (err) {
      console.error('[UserAuthSession] clearUser failed:', err);
    }
  }

  /**
   * Parse "userId:timestamp" từ X-User-Auth-Version header.
   * Trả true nếu header version mới hơn version đang lưu → cần re-fetch /api/auth/me.
   */
  async isStale(headerValue: string, currentUserId: number): Promise<boolean> {
    const parts = headerValue.split(':');
    if (parts.length !== 2) return false;

    const headerUserId = parseInt(parts[0], 10);
    const headerVersion = parseInt(parts[1], 10);

    if (isNaN(headerUserId) || isNaN(headerVersion)) return false;
    if (headerUserId !== currentUserId) return false;
    if (headerVersion === 0) return false; // BE chưa bump lần nào

    const session = await this.load(currentUserId);
    return !session || headerVersion > session.authVersion;
  }

}
