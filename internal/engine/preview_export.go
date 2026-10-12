package engine

import (
	"github.com/Jungley8/novel-studio/internal/domain"
)

// 预览透传导出（P2 预览拆分返工）：预览侧禁止手写第三份主角定位/清洗逻辑，
// 一律调用生产 helper（orchestrator_codex.go:226-251）的输出，逐字同源。
// 为避免触碰生产文件（sibling 并行改动 orchestrator_beats.go/workshop.go），
// 此处以同包透传包装形式导出，生产函数体零改动。

// PreviewProtNameLevel 透传 protNameLevel：nil → "未知"，否则 NameAndLevel()。
func PreviewProtNameLevel(e *domain.CodexEntry) string {
	return protNameLevel(e)
}

// PreviewProtInventory 透传 protInventory：nil/空 → "无"，否则原文。
func PreviewProtInventory(e *domain.CodexEntry) string {
	return protInventory(e)
}

// PreviewProtGoal 透传 protGoal：nil → "暂无"，否则 Want → Summary → "暂无"。
func PreviewProtGoal(e *domain.CodexEntry) string {
	return protGoal(e)
}
