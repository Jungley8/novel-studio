package engine

import (
	"strings"
	"unicode/utf8"

	"github.com/Jungley8/novel-studio/internal/domain"
)

const (
	RejectionMissingHookID        = "missing-hook-id"
	RejectionUnknownHookID        = "unknown-hook-id"
	RejectionInsufficientEvidence = "insufficient-evidence"
	RejectionEvidenceNotInDraft   = "evidence-not-in-draft"
	RejectionDuplicateClaim       = "duplicate-claim"
)

// HookClaimRejection 记录被证据门拒绝的伏笔回收声明。
type HookClaimRejection struct {
	Claim  domain.HookResolutionClaim
	Reason string
}

// FilterResolvedHookClaims 独立校验 reviewer 的伏笔回收声明（确定性、无 LLM 调用）：
// (a) ID 必须与 activeHooks 中真实 ID 精确匹配；
// (b) quote 去首尾空白后 rune 数 ≥8 且为正文子串（与结算证据门同口径）；
// (c) 同一 ID 重复声明仅首个通过。
// 返回通过者 ID（保序去重）与拒绝记录。
func FilterResolvedHookClaims(claims []domain.HookResolutionClaim, activeHooks []*domain.PlotHook, draftText string) (validIDs []string, rejected []HookClaimRejection) {
	if len(claims) == 0 {
		return nil, nil
	}

	if strings.TrimSpace(draftText) == "" {
		for _, c := range claims {
			rejected = append(rejected, HookClaimRejection{
				Claim:  c,
				Reason: RejectionInsufficientEvidence,
			})
		}
		return nil, rejected
	}

	activeSet := make(map[string]bool)
	for _, h := range activeHooks {
		if h != nil && h.ID != "" {
			activeSet[h.ID] = true
		}
	}

	passed := make(map[string]bool)
	for _, claim := range claims {
		if strings.TrimSpace(claim.ID) == "" {
			rejected = append(rejected, HookClaimRejection{
				Claim:  claim,
				Reason: RejectionMissingHookID,
			})
			continue
		}

		if !activeSet[claim.ID] {
			rejected = append(rejected, HookClaimRejection{
				Claim:  claim,
				Reason: RejectionUnknownHookID,
			})
			continue
		}

		if passed[claim.ID] {
			rejected = append(rejected, HookClaimRejection{
				Claim:  claim,
				Reason: RejectionDuplicateClaim,
			})
			continue
		}

		quote := strings.TrimSpace(claim.Quote)
		if utf8.RuneCountInString(quote) < 8 {
			rejected = append(rejected, HookClaimRejection{
				Claim:  claim,
				Reason: RejectionInsufficientEvidence,
			})
			continue
		}

		if !strings.Contains(draftText, quote) {
			rejected = append(rejected, HookClaimRejection{
				Claim:  claim,
				Reason: RejectionEvidenceNotInDraft,
			})
			continue
		}

		passed[claim.ID] = true
		validIDs = append(validIDs, claim.ID)
	}

	return validIDs, rejected
}
