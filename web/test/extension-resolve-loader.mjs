// Resolve hook: 允许 Node 原生测试直接导入 Vite 风格的“无扩展名相对路径”模块。
export async function resolve(specifier, context, nextResolve) {
  if (specifier.startsWith('.') && !/\.[cm]?[jt]s$/i.test(specifier)) {
    try {
      return await nextResolve(`${specifier}.js`, context);
    } catch (_) {
      // 回退到原始解析（例如目录形式的导入）
    }
  }
  return nextResolve(specifier, context);
}
