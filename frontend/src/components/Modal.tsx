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
  bodyScrollable?: boolean;
  panelClassName?: string;
  headerClassName?: string;
  headerContentClassName?: string;
  titleClassName?: string;
  descriptionClassName?: string;
  closeButtonClassName?: string;
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
  bodyScrollable = true,
  children,
  panelClassName,
  headerClassName,
  headerContentClassName,
  titleClassName,
  descriptionClassName,
  closeButtonClassName,
  bodyClassName,
  footerClassName,
}: ModalProps) {
  if (!open) {
    return null;
  }

  const bodyLayoutClass = bodyScrollable ? "min-h-0 flex-1 overflow-y-auto" : "";

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-stone-950/35 px-8 py-12 backdrop-blur-sm">
      <div className={`glass-panel flex max-h-[calc(100vh-6rem)] w-full max-w-3xl flex-col overflow-hidden ${panelClassName ?? ""}`.trim()}>
        <div
          className={`flex items-start justify-between border-b border-stone-200/80 px-5 py-4 ${headerClassName ?? ""}`.trim()}
        >
          <div className={`space-y-2 ${headerContentClassName ?? ""}`.trim()}>
            <h2 className={`font-display text-xl font-semibold text-stone-900 ${titleClassName ?? ""}`.trim()}>{title}</h2>
            {description ? (
              <p className={`max-w-2xl text-sm leading-6 text-stone-600 ${descriptionClassName ?? ""}`.trim()}>
                {description}
              </p>
            ) : null}
          </div>
          <button
            className={`ghost-button px-2.5 py-1 text-xs ${closeButtonClassName ?? ""}`.trim()}
            onClick={onClose}
            type="button"
          >
            关闭
          </button>
        </div>
        <div className={`${bodyLayoutClass} px-5 py-5 ${bodyClassName ?? ""}`.trim()}>{children}</div>
        {footer ? <div className={`border-t border-stone-200/80 px-5 py-4 ${footerClassName ?? ""}`.trim()}>{footer}</div> : null}
      </div>
    </div>
  );
}
