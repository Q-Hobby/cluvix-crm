import { Service} from '@angular/core';

@Service()
export class KeeperService {
  keeper: Promise<any> | null = null;
}
