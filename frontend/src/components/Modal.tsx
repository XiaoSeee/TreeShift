import type { PropsWithChildren, ReactNode } from "react";

/**
 * ModalProps 定义通用弹窗容器的入参。
 */
interface ModalProps extends PropsWithChildren {
  open: boolean;
  title: string;
  description?: string;
  footer?: ReactNode;
  onClose: () => void;
  panelClassName?: string;
  bodyClassName?: string;
  footerClassName?: string;
}

/**
 * Modal 提供统一的遮罩层、标题和底部动作区。
 */
export function Modal({
  open,
  title,
  description,
  footer,
  onClose,
  children,
  panelClassName,
  bodyClassName,
  footerClassName,
}: ModalProps) {
  if (!open) {
    return null;
  }

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-stone-950/35 px-6 py-10 backdrop-blur-sm">
      <div className={`glass-panel w-full max-w-3xl overflow-hidden ${panelClassName ?? ""}`.trim()}>
        <div className="flex items-start justify-between border-b border-stone-200/80 px-5 py-4">
          <div className="space-y-2">
            <h2 className="font-display text-xl font-semibold text-stone-900">{title}</h2>
            {description ? <p className="max-w-2xl text-sm leading-6 text-stone-600">{description}</p> : null}
          </div>
          <button className="ghost-button px-2.5 py-1 text-xs" onClick={onClose} type="button">
            关闭
          </button>
        </div>
        <div className={`max-h-[70vh] overflow-y-auto px-5 py-5 ${bodyClassName ?? ""}`.trim()}>{children}</div>
        {footer ? <div className={`border-t border-stone-200/80 px-5 py-4 ${footerClassName ?? ""}`.trim()}>{footer}</div> : null}
      </div>
    </div>
  );
}
