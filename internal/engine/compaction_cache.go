package engine

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/Jungley8/novel-studio/internal/domain"
)

// compactionCacheStore 是压缩缓存的可选存储能力（窄接口，SQLiteStore 实现）。
// 存储不支持时确定性跳过（纯优化，无正确性影响）。
type compactionCacheStore interface {
	GetCompactionCache(ctx context.Context, projectID, cacheKey string) (string, error)
	PutCompactionCache(ctx context.Context, projectID, cacheKey, compiledText string) error
}

// compactionCacheKey 以 (dropped 集合内容 + 剩余预算) 生成确定性指纹。
// 排序遍历（按 ID 稳定排序），字段用 \x00 分隔、项间 \x01，末尾附加 remaining。
// 限制：键不含模型标识（模型切换后旧编译产物仍命中——可接受：产物为内容摘要，不影响正确性）。
func compactionCacheKey(items []CompactionItem, remainingTokens int) string {
	sortedItems := make([]CompactionItem, len(items))
	copy(sortedItems, items)
	sort.SliceStable(sortedItems, func(i, j int) bool {
		if sortedItems[i].ID != sortedItems[j].ID {
			return sortedItems[i].ID < sortedItems[j].ID
		}
		if sortedItems[i].Kind != sortedItems[j].Kind {
			return sortedItems[i].Kind < sortedItems[j].Kind
		}
		return sortedItems[i].Text < sortedItems[j].Text
	})

	var sb strings.Builder
	for i, it := range sortedItems {
		if i > 0 {
			sb.WriteString("\x01")
		}
		sb.WriteString(it.ID)
		sb.WriteString("\x00")
		sb.WriteString(it.Kind)
		sb.WriteString("\x00")
		sb.WriteString(it.Text)
	}
	sb.WriteString("\x00")
	sb.WriteString(strconv.Itoa(remainingTokens))

	sum := sha256.Sum256([]byte(sb.String()))
	return hex.EncodeToString(sum[:])
}

// loadCompactionCache 读取缓存：存储不支持 → ("","")；读错误 → ("", note)；
// 命中但为空或超出 remaining → ("","")（视为未命中）。
func (c *CanonChronicle) loadCompactionCache(ctx context.Context, projectID, cacheKey string, remainingTokens int) (string, string) {
	if c.store == nil {
		return "", ""
	}
	cs, ok := c.store.(compactionCacheStore)
	if !ok || cs == nil {
		return "", ""
	}

	cached, err := cs.GetCompactionCache(ctx, projectID, cacheKey)
	if err != nil {
		return "", fmt.Sprintf("压缩缓存读取失败: %v", err)
	}

	if strings.TrimSpace(cached) == "" {
		return "", ""
	}
	if domain.EstimateTokens(cached) > remainingTokens {
		return "", ""
	}

	return cached, ""
}

// storeCompactionCache 回写缓存（best-effort）：存储不支持 → 无操作；写错误 → 返回 note。
func (c *CanonChronicle) storeCompactionCache(ctx context.Context, projectID, cacheKey, compiledText string) string {
	if c.store == nil {
		return ""
	}
	cs, ok := c.store.(compactionCacheStore)
	if !ok || cs == nil {
		return ""
	}

	if err := cs.PutCompactionCache(ctx, projectID, cacheKey, compiledText); err != nil {
		return fmt.Sprintf("压缩缓存写入失败: %v", err)
	}
	return ""
}
