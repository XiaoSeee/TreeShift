import { toErrorMessage } from "./errors";

/**
 * describe 用于覆盖错误文案工具的主要分支。
 */
describe("errors", () => {
  /**
   * it 应优先返回 Error 实例中的 message。
   */
  it("会从 Error 实例中提取错误文案", () => {
    expect(toErrorMessage(new Error("权限不足"))).toBe("权限不足");
  });

  /**
   * it 应在收到未知值时回退到统一的默认提示。
   */
  it("会为未知异常返回兜底文案", () => {
    expect(toErrorMessage({ code: 500 })).toBe("发生了未知错误。");
  });
});
