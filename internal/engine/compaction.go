package engine

import (
	"context"
	"errors"
	"fmt"
	"strings"
)

// CompactionItem 一条被预算丢弃的可压缩记忆项。
type CompactionItem struct {
	ID   string
	Kind string
	Text string
}

// CompactionInput 语义压缩输入。
type CompactionInput struct {
	ChapterIndex int
	Dropped      []CompactionItem
	BudgetTokens int
}

// CompressibleCompactor 宿主注入的语义压缩器；nil = 不启用（保持确定性丢弃）。
type CompressibleCompactor func(ctx context.Context, in CompactionInput) (string, error)

const MinCompactionBudgetTokens = 100

// NewLLMCompactor 返回绑定 client+model 的压缩器闭包。
func NewLLMCompactor(client LLMClient, model string) CompressibleCompactor {
	return func(ctx context.Context, in CompactionInput) (string, error) {
		if client == nil {
			return "", errors.New("llm client is nil")
		}
		if len(in.Dropped) == 0 {
			return "", nil
		}

		systemPrompt := fmt.Sprintf("你是一名长篇小说的记忆压缩器。请将提供的一组因上下文预算受限而被丢弃的记忆项，在不超过 %d tokens（预估纯文本字数）的预算内压缩为一段纯文本记忆摘录。\n"+
			"要求：\n"+
			"1. 必须保留伏笔因果链、角色关系变化、未回收因果。\n"+
			"2. 按重要性排序提炼。\n"+
			"3. 只输出摘录正文，严禁包含任何前言、解释或 Markdown 格式包裹。", in.BudgetTokens)

		var sb strings.Builder
		sb.WriteString(fmt.Sprintf("当前目标章节：第 %d 章\n", in.ChapterIndex))
		sb.WriteString(fmt.Sprintf("最大预算 Tokens：%d\n\n", in.BudgetTokens))
		sb.WriteString("待压缩的记忆项列表：\n")
		for i, item := range in.Dropped {
			sb.WriteString(fmt.Sprintf("%d. [%s] (%s): %s\n", i+1, item.Kind, item.ID, item.Text))
		}
		sb.WriteString("\n请将上述记忆项压缩为一段纯文本记忆摘录：")
		userPrompt := sb.String()

		content, _, err := client.ChatCompletionWithUsage(ctx, model, systemPrompt, userPrompt, 0.2)
		if err != nil {
			return "", err
		}
		return strings.TrimSpace(content), nil
	}
}
