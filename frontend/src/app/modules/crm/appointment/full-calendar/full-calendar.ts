// 📁 app/components/appointment-calendar/appointment-calendar.component.ts
import { CommonModule } from '@angular/common';
import {ChangeDetectionStrategy, Component, effect, Input, signal } from '@angular/core';
import { FullCalendarModule } from '@fullcalendar/angular';
import { CalendarOptions, EventInput } from '@fullcalendar/core';
import dayGridPlugin from '@fullcalendar/daygrid';
import interactionPlugin from '@fullcalendar/interaction';
import timeGridPlugin from '@fullcalendar/timegrid';

@Component({
  changeDetection: ChangeDetectionStrategy.OnPush,
  selector: 'app-appointment-full-calendar',
  standalone: true,
  imports: [CommonModule, FullCalendarModule],
  template: `
    <full-calendar [options]="calendarOptions()" style="min-height: 600px"></full-calendar>
  `,
})
export class FullCalendar {
  @Input({ required: true }) appointments: any[] = [];
  events = signal<EventInput[]>([]);

  calendarOptions = signal<CalendarOptions>({
    plugins: [dayGridPlugin, timeGridPlugin, interactionPlugin],
    initialView: 'timeGridWeek',
    headerToolbar: {
      left: 'prev,next today',
      center: 'title',
      right: 'dayGridMonth,timeGridWeek,timeGridDay',
    },
    allDaySlot: false,
    slotDuration: '00:30:00',
    editable: false,
    selectable: true,
    events: () => this.events(), // ✅ dùng hàm để reactive
  });

  constructor() {
    effect(() => {
      this.events.set(this.appointments); // ✅ an toàn
    });
  }
}
