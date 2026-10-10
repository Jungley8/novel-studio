package domain_test

import (
	"testing"

	"github.com/Jungley8/novel-studio/internal/domain"
)

func TestClassifyChapterType(t *testing.T) {
	tests := []struct {
		name     string
		beats    []domain.SceneBeat
		expected domain.ChapterType
	}{
		// 1. CLIMAX 爆发章 (末拍 Tension >= 8 且带 HookType，或峰值 >= 9 且平均 >= 7.0)
		{
			name: "CLIMAX - group 1: high tension climb with cliffhanger hook",
			beats: []domain.SceneBeat{
				{Phase: "蓄力", Tension: 6, Action: "宗门大阵破裂"},
				{Phase: "激战", Tension: 8, Action: "主角拔出斩龙剑"},
				{Phase: "破局", Tension: 9, Action: "一剑斩断魔尊法身", HookType: "CLIFFHANGER"},
			},
			expected: domain.ChapterTypeClimax,
		},
		{
			name: "CLIMAX - group 2: explosive average tension and peak >= 9 without hook",
			beats: []domain.SceneBeat{
				{Phase: "高潮起", Tension: 8, Action: "天劫雷龙咆哮降临"},
				{Phase: "死战", Tension: 9, Action: "肉身硬抗九重玄雷"},
				{Phase: "逆天", Tension: 9, Action: "强行吞噬天劫本源"},
				{Phase: "落幕", Tension: 7, Action: "雷云翻涌未散"},
			},
			expected: domain.ChapterTypeClimax,
		},
		{
			name: "CLIMAX - boundary: last beat tension exactly 8 with hook",
			beats: []domain.SceneBeat{
				{Phase: "交锋", Tension: 6, Action: "双方法宝对轰"},
				{Phase: "绝杀", Tension: 8, Action: "暗棋发动，扣下杀招", HookType: "REVERSAL"},
			},
			expected: domain.ChapterTypeClimax,
		},
		{
			name: "CLIMAX - dirty data clamp: tension > 10 clamped to 10",
			beats: []domain.SceneBeat{
				{Phase: "暴走", Tension: 15, Action: "超神爆发", HookType: "POWER_UP"},
			},
			expected: domain.ChapterTypeClimax,
		},

		// 2. AFTERMATH 余波章 (无末拍钩子，张力显著回落或高位降温)
		{
			name: "AFTERMATH - group 1: descending tension after battle",
			beats: []domain.SceneBeat{
				{Phase: "战后", Tension: 7, Action: "硝烟渐散，强敌退走"},
				{Phase: "清点", Tension: 5, Action: "收殓同门遗体，包扎伤口"},
				{Phase: "沉淀", Tension: 3, Action: "主角盘膝调息，回顾战局"},
			},
			expected: domain.ChapterTypeAftermath,
		},
		{
			name: "AFTERMATH - group 2: peak >= 6 dropping to <= 4",
			beats: []domain.SceneBeat{
				{Phase: "残余冲突", Tension: 6, Action: "扑灭残余火势"},
				{Phase: "收拢", Tension: 4, Action: "清点战利品储物袋"},
				{Phase: "平静", Tension: 3, Action: "回到洞府掩上石门"},
			},
			expected: domain.ChapterTypeAftermath,
		},
		{
			name: "AFTERMATH - boundary: first 6, last 5, descending without hook",
			beats: []domain.SceneBeat{
				{Phase: "交接", Tension: 6, Action: "向堂主复命"},
				{Phase: "休整", Tension: 5, Action: "换下染血长衫"},
			},
			expected: domain.ChapterTypeAftermath,
		},

		// 3. INTERLUDE 转场章 (无末拍钩子，极低张力 avg <= 4.0 且波动平直 <= 1)
		{
			name: "INTERLUDE - group 1: flat low tension travel scene",
			beats: []domain.SceneBeat{
				{Phase: "启程", Tension: 3, Action: "登上去往中州的灵舟"},
				{Phase: "沿途", Tension: 3, Action: "俯瞰云海与万里河山"},
				{Phase: "见闻", Tension: 4, Action: "听邻座散修闲谈各域风貌"},
				{Phase: "小憩", Tension: 3, Action: "在舱房品灵茶体悟心境"},
			},
			expected: domain.ChapterTypeInterlude,
		},
		{
			name: "INTERLUDE - group 2: completely flat tension 2",
			beats: []domain.SceneBeat{
				{Phase: "日常", Tension: 2, Action: "巡视灵田药草生长"},
				{Phase: "浇灌", Tension: 2, Action: "引山泉施展小云雨决"},
				{Phase: "归居", Tension: 2, Action: "夕阳西下生火做饭"},
			},
			expected: domain.ChapterTypeInterlude,
		},
		{
			name: "INTERLUDE - boundary: flat tension at boundary 4",
			beats: []domain.SceneBeat{
				{Phase: "交易", Tension: 4, Action: "在坊市摊位询价"},
				{Phase: "采买", Tension: 4, Action: "换取基础朱砂符纸"},
			},
			expected: domain.ChapterTypeInterlude,
		},

		// 4. SETUP 铺垫章 (低平均张力 avg <= 5.0，但有钩子或起伏酝酿)
		{
			name: "SETUP - group 1: low tension with mystery hook",
			beats: []domain.SceneBeat{
				{Phase: "暗流", Tension: 3, Action: "宗门暗坊有人兜售神秘残图"},
				{Phase: "调查", Tension: 4, Action: "主角暗中尾随探查卖家身份"},
				{Phase: "留钩", Tension: 5, Action: "发现卖家竟戴着执法堂腰牌", HookType: "MYSTERY"},
			},
			expected: domain.ChapterTypeSetup,
		},
		{
			name: "SETUP - group 2: low tension rising curve with fluctuation without hook",
			beats: []domain.SceneBeat{
				{Phase: "异动", Tension: 2, Action: "后山灵兽不安低吼"},
				{Phase: "探查", Tension: 5, Action: "发现地底有魔气渗出痕迹"},
				{Phase: "起疑", Tension: 4, Action: "记录异常方位准备上报"},
			},
			expected: domain.ChapterTypeSetup,
		},
		{
			name: "SETUP - boundary: avg exactly 5.0 with cliffhanger hook",
			beats: []domain.SceneBeat{
				{Phase: "布局", Tension: 5, Action: "布置隐蔽警戒阵法"},
				{Phase: "候敌", Tension: 5, Action: "黑影破窗而入", HookType: "CLIFFHANGER"},
			},
			expected: domain.ChapterTypeSetup,
		},

		// 5. ESCALATION 蓄力章 (中高张力 avg > 5.0 爬坡推进)
		{
			name: "ESCALATION - group 1: classic climbing escalation with hook",
			beats: []domain.SceneBeat{
				{Phase: "施压", Tension: 5, Action: "同门当众设擂羞辱挑衅"},
				{Phase: "接战", Tension: 6, Action: "主角应战上台"},
				{Phase: "试探", Tension: 7, Action: "数招之内摸清对手套路"},
				{Phase: "反击", Tension: 7, Action: "剑锋抵住对方咽喉", HookType: "REVERSAL"},
			},
			expected: domain.ChapterTypeEscalation,
		},
		{
			name: "ESCALATION - group 2: mid-high tension without hook",
			beats: []domain.SceneBeat{
				{Phase: "追踪", Tension: 6, Action: "雪地追踪妖兽踪迹"},
				{Phase: "遭遇", Tension: 7, Action: "正面遭遇三阶雪狼首领"},
				{Phase: "缠斗", Tension: 6, Action: "各自负伤拉开距离"},
			},
			expected: domain.ChapterTypeEscalation,
		},
		{
			name: "ESCALATION - boundary: avg slightly above 5.0 (5.25)",
			beats: []domain.SceneBeat{
				{Phase: "蓄势", Tension: 5, Action: "打磨本命飞剑"},
				{Phase: "凝练", Tension: 5, Action: "注入地心灵液"},
				{Phase: "突破", Tension: 5, Action: "剑鸣轻颤"},
				{Phase: "功成", Tension: 6, Action: "锋芒初露"},
			},
			expected: domain.ChapterTypeEscalation,
		},

		// 边界与脏数据
		{
			name:     "Boundary: empty beats slice",
			beats:    []domain.SceneBeat{},
			expected: domain.ChapterTypeSetup,
		},
		{
			name: "Boundary: negative tension clamped to 1",
			beats: []domain.SceneBeat{
				{Phase: "极静", Tension: -5, Action: "空寂无声"},
				{Phase: "极静", Tension: 0, Action: "微风拂过"},
			},
			expected: domain.ChapterTypeInterlude,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := domain.ClassifyChapterType(tt.beats)
			if got != tt.expected {
				t.Errorf("ClassifyChapterType() = %v, expected %v", got, tt.expected)
			}
		})
	}
}

func TestClassifyChapterType_OldBeatsBackwardCompatibility(t *testing.T) {
	// 旧章节 beats：4 拍、无章型字段、无 hook_type（或仅最后一拍简陋文本）
	oldBeats := []domain.SceneBeat{
		{Phase: "蓄力压迫", Tension: 5, Action: "反派拦路叫嚣", ExpectationBroken: "主角面色平静"},
		{Phase: "试探下套", Tension: 6, Action: "引诱反派进入预设阵法", ExpectationBroken: "反派毫无察觉"},
		{Phase: "绝地反转", Tension: 8, Action: "大阵发动反杀反派", ExpectationBroken: "反派骇然失色"},
		{Phase: "章末留钩", Tension: 8, Action: "暗处传来护法冷哼", ExpectationBroken: "主角警觉顿生", HookType: "CLIFFHANGER"},
	}

	// 必须正常运行不 panic，旧 4 拍有末拍 8 + CLIFFHANGER 应判定为 CLIMAX
	got := domain.ClassifyChapterType(oldBeats)
	if got != domain.ChapterTypeClimax {
		t.Errorf("expected CLIMAX for old beats, got %v", got)
	}

	// 旧章节无 HookType 拍
	oldBeatsNoHook := []domain.SceneBeat{
		{Phase: "第一拍", Tension: 5, Action: "动作一"},
		{Phase: "第二拍", Tension: 6, Action: "动作二"},
		{Phase: "第三拍", Tension: 6, Action: "动作三"},
		{Phase: "第四拍", Tension: 6, Action: "动作四"},
	}
	gotNoHook := domain.ClassifyChapterType(oldBeatsNoHook)
	if gotNoHook != domain.ChapterTypeEscalation {
		t.Errorf("expected ESCALATION for old beats without hook, got %v", gotNoHook)
	}
}

func TestPlanVolumeRhythm(t *testing.T) {
	vol := &domain.VolumeArc{
		VolumeIndex:       1,
		Title:             "青云风云",
		EstimatedChapters: 12,
	}

	t.Run("nil volume returns nil", func(t *testing.T) {
		res := domain.PlanVolumeRhythm(nil, nil, 8)
		if res != nil {
			t.Errorf("expected nil for nil volume, got %v", res)
		}
	})

	t.Run("default n is 8 when n <= 0", func(t *testing.T) {
		res := domain.PlanVolumeRhythm(vol, nil, 0)
		if len(res) != 8 {
			t.Errorf("expected 8 chapters by default, got %d", len(res))
		}
	})

	t.Run("rules validation on long sequence", func(t *testing.T) {
		// 生成 30 章序列验证核心铁律
		plan := domain.PlanVolumeRhythm(vol, nil, 30)
		if len(plan) != 30 {
			t.Fatalf("expected 30 chapters, got %d", len(plan))
		}

		consecCount := 1
		lastClimaxIdx := -999

		for i, ct := range plan {
			// 铁律 1: 不连续同型超 3 (consecutive <= 3)
			if i > 0 {
				if ct == plan[i-1] {
					consecCount++
					if consecCount > 3 {
						t.Errorf("chapter %d: consecutive same type %s exceeded 3 (got %d)", i+1, ct, consecCount)
					}
				} else {
					consecCount = 1
				}
			}

			// 铁律 2: CLIMAX 稀疏 (距离上一个 CLIMAX 至少 3 章)
			if ct == domain.ChapterTypeClimax {
				if lastClimaxIdx >= 0 {
					dist := i - lastClimaxIdx
					if dist < 3 {
						t.Errorf("chapter %d: CLIMAX is too dense, distance to previous CLIMAX at %d is %d (< 3)", i+1, lastClimaxIdx+1, dist)
					}
				}
				lastClimaxIdx = i

				// 铁律 3: 余波跟爆发 (CLIMAX 紧随其后必须是 AFTERMATH)
				if i+1 < len(plan) {
					next := plan[i+1]
					if next != domain.ChapterTypeAftermath {
						t.Errorf("chapter %d: AFTERMATH must immediately follow CLIMAX, got %s", i+2, next)
					}
				}
			}
		}
	})

	t.Run("recent history boundary constraints", func(t *testing.T) {
		// 场景 A: recent 包含连续 3 章 ESCALATION -> 第一章绝不能是 ESCALATION
		recent3Esc := []domain.ChapterType{
			domain.ChapterTypeEscalation,
			domain.ChapterTypeEscalation,
			domain.ChapterTypeEscalation,
		}
		planA := domain.PlanVolumeRhythm(vol, recent3Esc, 5)
		if planA[0] == domain.ChapterTypeEscalation {
			t.Errorf("planA[0] should not be ESCALATION when recent has 3 consecutive ESCALATION")
		}

		// 场景 B: recent 最后一章是 CLIMAX -> 第一章必须是 AFTERMATH
		recentClimax := []domain.ChapterType{
			domain.ChapterTypeEscalation,
			domain.ChapterTypeClimax,
		}
		planB := domain.PlanVolumeRhythm(vol, recentClimax, 5)
		if planB[0] != domain.ChapterTypeAftermath {
			t.Errorf("planB[0] must be AFTERMATH when recent ends with CLIMAX, got %s", planB[0])
		}

		// 场景 C: recent 紧邻上一章爆发加余波 -> 第一章不能马上大爆发
		recentAftermath := []domain.ChapterType{
			domain.ChapterTypeClimax,
			domain.ChapterTypeAftermath,
		}
		planC := domain.PlanVolumeRhythm(vol, recentAftermath, 5)
		if planC[0] == domain.ChapterTypeClimax {
			t.Errorf("planC[0] should not be CLIMAX right after AFTERMATH (sparsity rule)")
		}
	})
}
