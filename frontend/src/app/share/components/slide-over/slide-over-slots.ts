import { Directive } from '@angular/core';

/**
 * Slot cho vùng nút hành động trên header của SlideOver.
 * Dùng directive (thay vì chỉ ng-content select) để SlideOver query được sự tồn tại
 * của slot mà không cần input cờ thủ công.
 *
 * <button slideOverActions pButton ...>
 */
@Directive({ selector: '[slideOverActions]', standalone: true })
export class SlideOverActionsDirective {}

/**
 * Slot cho footer cố định đáy panel. Không khai slot này thì SlideOver không render
 * vùng footer (không chiếm chỗ, không kẻ viền thừa).
 *
 * <div slideOverFooter> ... </div>
 */
@Directive({ selector: '[slideOverFooter]', standalone: true })
export class SlideOverFooterDirective {}
