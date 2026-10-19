import {ChangeDetectionStrategy, Component } from '@angular/core';
import { TranslatePipe } from '@ngx-translate/core';

@Component({
  changeDetection: ChangeDetectionStrategy.OnPush,
  selector: 'app-empty-message',
  templateUrl: './empty-message.html',
  styleUrls: ['./empty-message.scss'],
  standalone: true,
  imports: [TranslatePipe],
})
export class EmptyMessage {
  constructor() {}
}
