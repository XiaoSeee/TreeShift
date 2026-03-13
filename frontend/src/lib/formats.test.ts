import {
  argsToText,
  buildSuggestedPath,
  sanitizePathSegment,
  textToArgs,
} from "./formats";

/**
 * describe 路径与参数格式工具测试。
 */
describe("formats", () => {
  /**
   * it 应把分支名转换为可用目录名。
   */
  it("会把分支名清洗为安全路径片段", () => {
    expect(sanitizePathSegment("feature/login fix")).toBe("feature-login-fix");
  });

  /**
   * it 应基于根目录和分支名生成建议路径。
   */
  it("会生成建议 worktree 路径", () => {
    expect(buildSuggestedPath("D:\\Worktrees", "feature/login")).toBe("D:\\Worktrees\\feature-login");
  });

  /**
   * it 应在文本与数组之间稳定转换工具参数。
   */
  it("会在参数文本与数组之间相互转换", () => {
    const rawText = "--model\n gpt-5 \n\n--approval-mode\nnever";
    expect(textToArgs(rawText)).toEqual(["--model", "gpt-5", "--approval-mode", "never"]);
    expect(argsToText(["--model", "gpt-5"])).toBe("--model\ngpt-5");
  });
});
