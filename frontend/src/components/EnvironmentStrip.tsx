import { useEffect, useState } from "react";

import type { EnvironmentStatus } from "../types";

/**
 * EnvironmentStripProps 定义环境诊断条组件的入参。
 */
interface EnvironmentStripProps {
  environment: EnvironmentStatus | null;
}

/**
 * EnvironmentStrip 展示启动阶段核心依赖的环境异常提示。
 */
export function EnvironmentStrip({ environment }: EnvironmentStripProps) {
  const [dismissed, setDismissed] = useState(false);
  const items = environment ? [environment.git, environment.terminal, ...environment.externalTools] : [];
  const issueItems = items.filter((item) => !item.available);
  const warningSummary = environment?.warnings.join(" | ") ?? "";
  const issueSignature = environment
    ? [
        ...issueItems.map((item) => `${item.name}:${item.message}`),
        ...environment.warnings,
      ].join("|")
    : "";

  useEffect(() => {
    setDismissed(false);
  }, [issueSignature]);

  if (!environment || dismissed || (issueItems.length === 0 && environment.warnings.length === 0)) {
    return null;
  }

  return (
    <section className="glass-panel border border-rose-200/70 bg-rose-50/55 px-3 py-2.5">
      <div className="flex items-center justify-between gap-2">
        <div className="flex min-w-0 items-center gap-2">
          <p className="shrink-0 font-display text-xs font-semibold uppercase tracking-[0.18em] text-rose-800">启动异常</p>
          {issueItems.map((item) => (
            <span
              key={`${item.name}-${item.executable}`}
              className="tag shrink-0 bg-rose-100 px-2 py-0.5 text-[10px] text-rose-700"
              title={`${item.name}：${item.message}${item.executable ? ` (${item.executable})` : ""}`}
            >
              {item.name}
            </span>
          ))}
          {environment.warnings.length > 0 ? (
            <span
              className="tag shrink-0 bg-amber-100 px-2 py-0.5 text-[10px] text-amber-900"
              title={warningSummary}
            >
              告警 {environment.warnings.length}
            </span>
          ) : null}
        </div>
        <button
          aria-label="关闭启动异常提示"
          className="ghost-button shrink-0 px-2 py-0.5 text-xs"
          onClick={() => setDismissed(true)}
          type="button"
        >
          关闭
        </button>
      </div>
    </section>
  );
}
