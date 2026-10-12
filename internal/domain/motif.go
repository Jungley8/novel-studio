package domain

import (
	"fmt"
	"strings"
)

// MaxProjectMotifs 项目级母题上限：过多会稀释每一条母题在提示词中的注意力权重。
const MaxProjectMotifs = 8

// SanitizeMotifs 作者编辑入口的确定性清洗（纯函数，不修改入参）：
// 逐条 trim、丢弃空条目、按出现顺序去重；清洗后超过上限 → fail-closed 报错。
func SanitizeMotifs(motifs []string) ([]string, error) {
	seen := make(map[string]bool, len(motifs))
	cleaned := make([]string, 0, len(motifs))
	for _, m := range motifs {
		trimmed := strings.TrimSpace(m)
		if trimmed == "" || seen[trimmed] {
			continue
		}
		seen[trimmed] = true
		cleaned = append(cleaned, trimmed)
	}
	if len(cleaned) > MaxProjectMotifs {
		return nil, fmt.Errorf("too many motifs: %d (max %d)", len(cleaned), MaxProjectMotifs)
	}
	return cleaned, nil
}
