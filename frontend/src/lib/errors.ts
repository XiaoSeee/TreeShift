/**
 * toErrorMessage 把未知异常统一转换为可展示的错误文案。
 *
 * 该方法优先保留 Error 实例或字符串中的原始信息，
 * 以便界面提示尽量接近后端或运行时返回的真实原因。
 *
 * @param error 任意来源的异常对象、字符串或其他值。
 * @returns 适合直接显示给用户的简体中文错误文案。
 */
export function toErrorMessage(error: unknown): string {
  if (error instanceof Error) {
    return error.message;
  }

  if (typeof error === "string") {
    return error;
  }

  return "发生了未知错误。";
}
