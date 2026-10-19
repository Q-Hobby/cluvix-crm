import { Service} from '@angular/core';

@Service()
export class ReloadService {
  private channel!: BroadcastChannel;

  constructor() {
    this.channel = new BroadcastChannel('reload_channel');
  }

  sendMessage(message: string): void {
    this.channel.postMessage(message);
  }
}
