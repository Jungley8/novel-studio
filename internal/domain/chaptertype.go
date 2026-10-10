package domain

import (
	"strings"
)

// ChapterType defines the intra-volume narrative rhythm positioning for chapters.
type ChapterType string

const (
	ChapterTypeEscalation ChapterType = "ESCALATION" // 蓄力
	ChapterTypeClimax     ChapterType = "CLIMAX"     // 爆发
	ChapterTypeAftermath  ChapterType = "AFTERMATH"  // 余波
	ChapterTypeSetup      ChapterType = "SETUP"      // 铺垫
	ChapterTypeInterlude  ChapterType = "INTERLUDE"  // 转场
)

// ChineseName returns the Chinese label for the chapter type.
func (c ChapterType) ChineseName() string {
	switch c {
	case ChapterTypeEscalation:
		return "蓄力"
	case ChapterTypeClimax:
		return "爆发"
	case ChapterTypeAftermath:
		return "余波"
	case ChapterTypeSetup:
		return "铺垫"
	case ChapterTypeInterlude:
		return "转场"
	default:
		return string(c)
	}
}

// clampTension limits tension to the valid range [1, 10].
func clampTension(t int) int {
	if t < 1 {
		return 1
	}
	if t > 10 {
		return 10
	}
	return t
}

// ClassifyChapterType evaluates the narrative beats of a chapter and deterministically
// derives its ChapterType without requiring schema storage.
//
// Classification rules:
//  1. 规则 1 (CLIMAX 爆发): 末拍张力 >= 8 且挂有钩子 (HookType != "")，或峰值张力 >= 9 且全章平均张力 >= 7.0。
//     爆发章是矛盾全面激化的高潮爆发点，末拍伴随强烈的情绪悬念或反转。
//  2. 规则 2 (AFTERMATH 余波): 末拍无钩子 (HookType == "")，且张力呈现降温回落趋势
//     （首拍张力高于末拍且末拍张力 <= 5，或全章峰值张力 >= 6 且末拍彻底降至 <= 4）。
//     余波章是大战/高潮后的创伤疗愈、战果清点与喘息收束。
//  3. 规则 3 (INTERLUDE 转场): 末拍无钩子 (HookType == "")，整体张力处于低位 (avg <= 4.0)，
//     且张力起伏平缓 (maxTension - minTension <= 1)。
//     转场章是纯粹的旅途行进、日常调剂与过渡连接，无强烈冲突。
//  4. 规则 4 (SETUP 铺垫): 全章平均张力偏低 (avg <= 5.0)，但末拍留有伏笔悬念 (HookType != "")，
//     或张力起伏明显 (maxTension - minTension > 1) 且末拍张力未明显跌落。
//     铺垫章重在展示信息差、暗线埋设与下一轮危机酝酿。
//  5. 规则 5 (ESCALATION 蓄力): 全章平均张力处于中高区间 (avg > 5.0)，未满足 CLIMAX 爆发条件。
//     蓄力章张力阶梯式爬坡或步步紧逼，推动主角与反派矛盾层层升级。
func ClassifyChapterType(beats []SceneBeat) ChapterType {
	if len(beats) == 0 {
		return ChapterTypeSetup
	}

	n := len(beats)
	tensions := make([]int, n)
	maxT := clampTension(beats[0].Tension)
	minT := maxT
	sumT := 0

	for i, b := range beats {
		t := clampTension(b.Tension)
		tensions[i] = t
		if t > maxT {
			maxT = t
		}
		if t < minT {
			minT = t
		}
		sumT += t
	}

	firstT := tensions[0]
	lastT := tensions[n-1]
	avgT := float64(sumT) / float64(n)
	hasHook := strings.TrimSpace(beats[n-1].HookType) != ""

	// 规则 1: CLIMAX 爆发章
	if (lastT >= 8 && hasHook) || (maxT >= 9 && avgT >= 7.0) {
		return ChapterTypeClimax
	}

	// 规则 2: AFTERMATH 余波章 (无末拍钩子，张力显著回落收束)
	if !hasHook && ((lastT < firstT && lastT <= 5) || (maxT >= 6 && lastT <= 4)) {
		return ChapterTypeAftermath
	}

	// 规则 3: INTERLUDE 转场章 (无末拍钩子，低张力且张力近乎平直)
	if !hasHook && (avgT <= 4.0 && (maxT-minT <= 1)) {
		return ChapterTypeInterlude
	}

	// 规则 4: SETUP 铺垫章 (低平均张力，但有钩子或起伏酝酿)
	if avgT <= 5.0 {
		return ChapterTypeSetup
	}

	// 规则 5: ESCALATION 蓄力章 (中高张力爬坡推进)
	return ChapterTypeEscalation
}

// PlanVolumeRhythm deterministically plans the chapter type sequence for the next n chapters.
// Constraints:
//   - CLIMAX is sparse (at least 3 chapters between climaxes).
//   - AFTERMATH strictly follows CLIMAX.
//   - No consecutive identical chapter type exceeds 3.
//
// If volume is nil, returns nil.
func PlanVolumeRhythm(volume *VolumeArc, recent []ChapterType, n int) []ChapterType {
	if volume == nil {
		return nil
	}
	if n <= 0 {
		n = 8
	}

	history := make([]ChapterType, len(recent), len(recent)+n)
	copy(history, recent)
	planned := make([]ChapterType, 0, n)

	for step := 0; step < n; step++ {
		// 1. Check if previous was CLIMAX -> must follow with AFTERMATH
		if len(history) > 0 && history[len(history)-1] == ChapterTypeClimax {
			planned = append(planned, ChapterTypeAftermath)
			history = append(history, ChapterTypeAftermath)
			continue
		}

		// 2. Identify forbidden types due to consecutive repetition (>= 3 consecutive)
		var forbiddenConsec ChapterType
		if len(history) >= 3 {
			last1 := history[len(history)-1]
			last2 := history[len(history)-2]
			last3 := history[len(history)-3]
			if last1 == last2 && last2 == last3 {
				forbiddenConsec = last1
			}
		}

		// 3. Check distance from last CLIMAX to enforce sparsity (>= 3 chapters between climaxes)
		distSinceClimax := 999
		for j := len(history) - 1; j >= 0; j-- {
			if history[j] == ChapterTypeClimax {
				distSinceClimax = len(history) - 1 - j
				break
			}
		}
		canClimax := distSinceClimax >= 3

		// 4. Determine candidate progression
		var prev ChapterType
		if len(history) > 0 {
			prev = history[len(history)-1]
		}

		var candidates []ChapterType
		switch prev {
		case ChapterTypeAftermath:
			candidates = []ChapterType{ChapterTypeInterlude, ChapterTypeSetup, ChapterTypeEscalation}
		case ChapterTypeInterlude:
			candidates = []ChapterType{ChapterTypeSetup, ChapterTypeEscalation}
		case ChapterTypeSetup:
			candidates = []ChapterType{ChapterTypeEscalation, ChapterTypeInterlude}
		case ChapterTypeEscalation:
			escCount := 0
			for j := len(history) - 1; j >= 0; j-- {
				if history[j] == ChapterTypeEscalation {
					escCount++
				} else {
					break
				}
			}
			if escCount >= 2 && canClimax {
				candidates = []ChapterType{ChapterTypeClimax, ChapterTypeEscalation, ChapterTypeSetup}
			} else {
				candidates = []ChapterType{ChapterTypeEscalation, ChapterTypeSetup, ChapterTypeClimax}
			}
		default: // Empty history / start of volume
			candidates = []ChapterType{ChapterTypeSetup, ChapterTypeEscalation, ChapterTypeInterlude}
		}

		// If volume estimated chapters matches current chapter, encourage climax
		currCh := len(history) + 1
		if volume.EstimatedChapters > 0 && currCh == volume.EstimatedChapters && canClimax && forbiddenConsec != ChapterTypeClimax {
			candidates = append([]ChapterType{ChapterTypeClimax}, candidates...)
		}

		// Select the best candidate that respects all constraints
		selected := false
		var nextType ChapterType
		for _, cand := range candidates {
			if cand == forbiddenConsec {
				continue
			}
			if cand == ChapterTypeClimax && !canClimax {
				continue
			}
			nextType = cand
			selected = true
			break
		}

		if !selected {
			// Fallback: pick any valid type
			allTypes := []ChapterType{
				ChapterTypeSetup,
				ChapterTypeEscalation,
				ChapterTypeInterlude,
				ChapterTypeAftermath,
				ChapterTypeClimax,
			}
			for _, t := range allTypes {
				if t == forbiddenConsec {
					continue
				}
				if t == ChapterTypeClimax && !canClimax {
					continue
				}
				nextType = t
				selected = true
				break
			}
		}

		planned = append(planned, nextType)
		history = append(history, nextType)
	}

	return planned
}
