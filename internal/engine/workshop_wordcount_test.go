package engine

import (
	"context"
	"math"
	"strings"
	"testing"

	"github.com/Jungley8/novel-studio/internal/domain"
)

// mockWordcountLLMClient handles LLM simulation for word telemetry tests,
// including draft generation, review court auditing, and targeted rewrite flows.
type mockWordcountLLMClient struct {
	beatsJSON         string
	renderDraft       string
	reviewJSON        string
	settleJSON        string
	rewriteDraft      string
	rewriteReviewJSON string
}

func (m *mockWordcountLLMClient) ChatCompletion(ctx context.Context, model, systemPrompt, userPrompt string, temperature float64) (string, error) {
	if strings.Contains(systemPrompt, "状态对账") {
		return `{"consistent":true,"reconciliation_required":false,"observations":[]}`, nil
	}
	if strings.Contains(systemPrompt, "精算师") || strings.Contains(systemPrompt, "结算") || strings.Contains(systemPrompt, "Settle") {
		if m.settleJSON != "" {
			return m.settleJSON, nil
		}
		return `{"inventory_delta":"","power_delta":"","character_mutations":[],"evidence":[]}`, nil
	}
	if strings.Contains(systemPrompt, "白金级剧情架构师") || strings.Contains(systemPrompt, "高潮推演专家") {
		return "黄金冲突", nil
	}
	if strings.Contains(systemPrompt, "记忆压缩器") {
		return "", nil
	}
	if strings.Contains(systemPrompt, "总设计师") {
		if m.beatsJSON != "" {
			return m.beatsJSON, nil
		}
		return `{"beats":[{"phase":"推进","tension":7,"action":"行动","expectation_broken":"反转"}],"state_mutation":{}}`, nil
	}
	if strings.Contains(systemPrompt, "精修专家") {
		if m.rewriteDraft != "" {
			return m.rewriteDraft, nil
		}
		return m.renderDraft, nil
	}
	if strings.Contains(systemPrompt, "分庭总审编") || strings.Contains(systemPrompt, "总审编") || strings.Contains(systemPrompt, "总编审") || strings.Contains(systemPrompt, "ReviewCourt") {
		hasKW := strings.Contains(userPrompt, "REWRITE_KEYWORD")
		if m.rewriteDraft != "" && hasKW && m.rewriteReviewJSON != "" {
			return m.rewriteReviewJSON, nil
		}
		if m.reviewJSON != "" {
			return m.reviewJSON, nil
		}
		return `{"verdict":"ACCEPTED","score":90,"issues":[],"suggestions":"优秀"}`, nil
	}
	return m.renderDraft, nil
}

func (m *mockWordcountLLMClient) ChatCompletionWithUsage(ctx context.Context, model, systemPrompt, userPrompt string, temperature float64) (string, TokenUsage, error) {
	txt, err := m.ChatCompletion(ctx, model, systemPrompt, userPrompt, temperature)
	return txt, TokenUsage{PromptTokens: 50, CompletionTokens: 100, TotalTokens: 150}, err
}

func (m *mockWordcountLLMClient) ChatCompletionStream(ctx context.Context, model, systemPrompt, userPrompt string, temperature float64) (<-chan StreamChunk, error) {
	txt, err := m.ChatCompletion(ctx, model, systemPrompt, userPrompt, temperature)
	if err != nil {
		return nil, err
	}
	ch := make(chan StreamChunk, 2)
	ch <- StreamChunk{Delta: txt}
	ch <- StreamChunk{Done: true, Usage: &TokenUsage{PromptTokens: 50, CompletionTokens: 100, TotalTokens: 150}}
	close(ch)
	return ch, nil
}

func makeChineseRunes(count int, prefix string) string {
	paragraphs := []string{
		"刀落。当年他拜入山门之时师尊曾在祖师堂前亲手传下一柄三尺青锋剑并郑重叮嘱凡天下修士皆当以除魔卫道为己任切莫堕入魔道。「别追了。」林凡的声音很轻。",
		"风起。黑衣修士按住腰间长刀冷冷注视前方那座经历千百年来风霜侵蚀依然矗立在天地之间巍峨不动的护宗大阵。「把东西交出来。」对方冷冷吐出一句话。",
		"剑啸。整座石桥在狂暴剑气的轰击之下彻底分崩离析化作漫天碎石暴雨般倾泻而下将深不见底的山谷完全掩埋。「你不是我对手。」林凡收剑入鞘。",
		"雨落。破庙在震耳欲聋的雷霆轰鸣声中剧烈摇晃无数残砖断瓦混杂着烟尘纷纷跌落将微弱火苗彻底吞噬。「天亮之前必须破阵。」楚长老沉声吩咐。",
		"雾散。千丈绝壁之巅盛开着一株通体晶莹剔透并流转着九色霞光的奇异仙草引来无数生灵在暗中窥伺。「小心守护异兽。」林凡提醒身侧同伴。",
		"步停。青石古道蜿蜒伸向云霄深处两旁数万株苍劲如龙的参天古木在山风吹拂之下发出海潮般轰鸣。「前方便是天剑宗山门。」引路执事止步回望。",
		"钟鸣。九座插云巨峰在万道朝阳晨辉映照之下紫气升腾霞光漫卷惊起林间数以万计的仙鹤灵禽盘旋飞舞。「今日大比正式开席。」宏大声音回荡四野。",
		"泉涌。古老石池底部浮现出一道道玄奥晦涩的金色阵纹爆发出令人目眩神迷的光辉激荡起滚滚热浪。「速速稳固心神入定。」林凡挥袖布下隔音禁制。",
		"街静。深秋寒风卷起漫天昏黄沙尘呼啸掠过街道两旁门窗紧闭的店铺远处隐约传来沉闷而急促的马蹄声响。「来者不善。」客栈掌柜悄悄掩上门板。",
		"掌出。庭院中数十棵需数人合抱的参天古槐应声爆碎漫天木屑伴随狂暴气浪将院墙整齐切断轰然倒塌。「你的玄天功果然不凡。」赵师兄擦去嘴角血痕。",
		"日暮。苍茫大漠尽头矗立着一座残破古城在残阳映照下显得愈发雄浑沧桑向过客无声诉说着昔日战火。「今夜在此宿营。」佣兵统领翻身下马下达号令。",
		"水流。幽深竹林尽头坐落着三间青竹茅舍微风拂过送来缕缕令人心旷神怡的奇异药香令人精神大振。「贵客远道而来所为何事？」老妪缓步推开竹门。",
		"霜降。子夜更鼓声穿透重重深宅大院沉闷悠长地回荡在空旷长街上更夫紧裹棉袍在刺骨寒风中瑟缩疾行。「外头天寒地冻快些关门。」老管家低声催促仆童。",
		"浪起。无边沧海在狂风肆虐下掀起十余丈高万钧巨浪以排山倒海之势猛烈撞击黑色玄武岩礁石激起漫天白沫。「抓紧桅杆千万莫松手。」老船工在暴风雨中大声呼喝。",
		"星垂。无垠荒原上升腾起袅袅炊烟数堆熊熊燃烧的营火映红了这群刚刚经历生死搏杀的年轻修士坚毅面庞。「明日便要进入凶兽山脉深处。」众人举起皮囊共饮浊酒。",
		"花落。落霞峰后山连绵数十里的桃林在微风吹拂下落英缤纷粉白花瓣铺满整条通往山巅的青石小径美不胜收。「练剑切忌心浮气躁。」白发传功长老负手立于溪边指点后辈。",
		"雪霁。巍峨雪山主峰在正午阳光照耀下反射出刺目银芒站在峰顶极目远眺只见群山如浪奔涌令人胸襟大开。「终于登顶这极北绝巅。」探险散修深深吸入冰凉空气。",
		"夜深。古渡口系舟老柳树下唯余几点孤寂渔火在苍茫水面上随波晃动江水拍打船舷发出单调而沉闷的回响。「渡船明日辰时方才启航。」白发艄公系好缆绳收起撑篙。",
		"丹成。药王阁顶层丹房内九转金炉爆发出清脆轰鸣漫天紫气氤氲升腾凝聚成一朵巨大灵芝状彩云久久不散。「这炉混元丹终于炼成了。」紫袍丹师长舒一口浊气露出微笑。",
		"阵列。峡谷两侧壁立千仞数十万身披玄黑重铠的精锐甲士肃然列阵手中锋锐长戈在惨淡日光下泛着幽冷寒芒。「放箭。」随着统帅一声令下漫天箭雨破空呼啸射向敌阵。",
		"暗夜。九天星河在无边夜色中缓缓流淌无声无息映照着苍茫大地之上无数修道之人在红尘与仙道之间的艰难求索。「道阻且长。」少年立于崖前轻声低语。",
		"古道。荒山野岭之间蔓延着一条不知通向何方的古旧驿道枯藤老树昏鸦在黄昏斜阳下拉出长长阴影。「前路茫茫亦当执剑前行。」独行剑客紧了紧背上剑匣。",
		"残月。冷清山岗上松柏傲然挺立不屈于严寒呼啸朔风中唯有一盏微弱油灯在荒废草庐之内摇曳生辉。「心若冰清天塌不惊。」闭目打坐的老道吐出一口浊气。",
		"青锋。林凡拔出佩剑剑尖斜指地面剑身清亮如秋水倒映出他波澜不惊的双眸以及天际滚滚而来的风云变幻。「出手吧。」他对面前强敌平静邀战。",
		"晨曦。破晓时分第一缕天光刺破深沉夜幕将温暖金芒洒向无边云海群峰在晨光中苏醒展露出雄奇壮美身姿。「新的一日又已到来。」巡山弟子迎风而立展颜欢笑。",
	}

	var sb strings.Builder
	if prefix != "" {
		sb.WriteString(prefix)
		sb.WriteString("\n")
	}

	idx := 0
	for len([]rune(sb.String())) < count {
		sb.WriteString(paragraphs[idx%len(paragraphs)])
		sb.WriteString("\n")
		idx++
	}

	runes := []rune(sb.String())
	if len(runes) > count {
		return string(runes[:count])
	}
	return sb.String()
}

func newWordcountTestWorkshop(mockClient *mockWordcountLLMClient) (*ChapterWorkshop, *mockChronicleStore) {
	orch := NewOrchestrator(mockClient)
	qGate := NewQualityGate(mockClient, nil)

	mockStore := &mockChronicleStore{
		project: &domain.Project{
			ID:             "p-wordcount",
			Title:          "长篇修仙传",
			TargetPlatform: "番茄脑洞",
		},
		codexEntries: []*domain.CodexEntry{
			{
				ID:        "codex-1",
				ProjectID: "p-wordcount",
				Category:  domain.CategoryCharacter,
				Archetype: "PROTAGONIST",
				Name:      "林凡",
			},
		},
		chapters: []*domain.Chapter{},
	}
	chronicle := NewCanonChronicle(mockStore)
	ws := NewChapterWorkshop(orch, chronicle, qGate, mockStore)
	return ws, mockStore
}

// 1. TestChapterWorkshop_WordTelemetry_UnderTarget:
// 低于目标 80% 驱动定向扩写重写循环闭环管理。
func TestChapterWorkshop_WordTelemetry_UnderTarget(t *testing.T) {
	reviewResp := `{
		"verdict": "ACCEPTED",
		"score": 90,
		"issues": [],
		"suggestions": "合格"
	}`

	// 子用例 A「重写达标即停」：rewriteDraft 设为达标长度（如目标 2000 → ≥1600 字）
	// → 断言 res.RewriteLoops == 1、终稿达标（UnderTarget == false）、终稿 issues 无 【字数遥测】。
	t.Run("SubcaseA_RewriteReachesTarget_Stops", func(t *testing.T) {
		draft800 := makeChineseRunes(800, "第八百字初稿段落。")
		rewriteDraft1800 := makeChineseRunes(1800, "第一千八百字扩写达标段落。")

		mockClient := &mockWordcountLLMClient{
			renderDraft:  draft800,
			reviewJSON:   reviewResp,
			rewriteDraft: rewriteDraft1800,
		}

		ws, _ := newWordcountTestWorkshop(mockClient)

		req := WorkshopProduceRequest{
			ProjectID:       "p-wordcount",
			ChapterIndex:    1,
			CoreConflict:    "宗门反派夜袭",
			ReasoningModel:  "mock-r1",
			WriterModel:     "mock-writer",
			ReviewerModel:   "mock-reviewer",
			WordsTarget:     2000,
			AutoCommit:      false,
			MaxRewriteLoops: 2,
		}

		res, err := ws.ProduceChapter(context.Background(), req)
		if err != nil {
			t.Fatalf("ProduceChapter failed: %v", err)
		}

		if res.RewriteLoops != 1 {
			t.Errorf("expected RewriteLoops == 1, got %d", res.RewriteLoops)
		}
		if res.UnderTarget {
			t.Errorf("expected UnderTarget == false for final 1800 words draft, got true")
		}
		if res.ActualWords != len([]rune(rewriteDraft1800)) {
			t.Errorf("expected ActualWords %d, got %d", len([]rune(rewriteDraft1800)), res.ActualWords)
		}
		for _, issue := range res.Audit.Issues {
			if strings.Contains(issue, "【字数遥测】") {
				t.Errorf("expected no 【字数遥测】 in final passing issues, got: %s", issue)
			}
		}
	})

	// 子用例 B「重写仍不达标 → 循环封顶」：rewriteDraft 仍不足
	// → 断言 res.RewriteLoops == MaxRewriteLoops、UnderTarget == true、issues 含 【字数遥测】、且至少一轮 issues 文案含"已触发定向扩写"。
	t.Run("SubcaseB_RewriteStillUnderTarget_CapsAtMaxLoops", func(t *testing.T) {
		draft800 := makeChineseRunes(800, "第八百字初稿段落。")
		rewriteDraft800 := makeChineseRunes(800, "第八百字重写后仍不足段落。")

		mockClient := &mockWordcountLLMClient{
			renderDraft:  draft800,
			reviewJSON:   reviewResp,
			rewriteDraft: rewriteDraft800,
		}

		ws, _ := newWordcountTestWorkshop(mockClient)

		req := WorkshopProduceRequest{
			ProjectID:       "p-wordcount",
			ChapterIndex:    1,
			CoreConflict:    "宗门反派夜袭",
			ReasoningModel:  "mock-r1",
			WriterModel:     "mock-writer",
			ReviewerModel:   "mock-reviewer",
			WordsTarget:     2000,
			AutoCommit:      false,
			MaxRewriteLoops: 2,
		}

		res, err := ws.ProduceChapter(context.Background(), req)
		if err != nil {
			t.Fatalf("ProduceChapter failed: %v", err)
		}

		if res.RewriteLoops != req.MaxRewriteLoops {
			t.Errorf("expected RewriteLoops == %d, got %d", req.MaxRewriteLoops, res.RewriteLoops)
		}
		if !res.UnderTarget {
			t.Errorf("expected UnderTarget == true, got false")
		}

		hasTelemetryIssue := false
		hasTriggeredText := false
		for _, issue := range res.Audit.Issues {
			if strings.Contains(issue, "【字数遥测】") {
				hasTelemetryIssue = true
				if strings.Contains(issue, "已触发定向扩写") {
					hasTriggeredText = true
				}
			}
		}
		if !hasTelemetryIssue {
			t.Errorf("expected Audit.Issues to contain 【字数遥测】, got: %v", res.Audit.Issues)
		}
		if !hasTriggeredText {
			t.Errorf("expected Audit.Issues to contain '已触发定向扩写', got: %v", res.Audit.Issues)
		}
	})

	// 原 ThresholdSensitivity_70Percent 按新语义改写（70% 触发重写）：
	// 1400 / 2000 = 0.7 < 0.8 WordCountRevisionFloor → 触发定向重写（若门变异为 0.5 则 0.7 >= 0.5 不触发，变红）。
	t.Run("ThresholdSensitivity_70Percent", func(t *testing.T) {
		draft1400 := makeChineseRunes(1400, "第一千四百字敏感度测试段落。")
		rewriteDraft1800 := makeChineseRunes(1800, "第一千八百字敏感度扩写达标段落。")

		mockClient70 := &mockWordcountLLMClient{
			renderDraft:  draft1400,
			reviewJSON:   reviewResp,
			rewriteDraft: rewriteDraft1800,
		}

		ws70, _ := newWordcountTestWorkshop(mockClient70)
		req := WorkshopProduceRequest{
			ProjectID:       "p-wordcount",
			ChapterIndex:    1,
			CoreConflict:    "宗门反派夜袭",
			ReasoningModel:  "mock-r1",
			WriterModel:     "mock-writer",
			ReviewerModel:   "mock-reviewer",
			WordsTarget:     2000,
			AutoCommit:      false,
			MaxRewriteLoops: 1,
		}

		res70, err70 := ws70.ProduceChapter(context.Background(), req)
		if err70 != nil {
			t.Fatalf("ProduceChapter failed: %v", err70)
		}

		if res70.RewriteLoops != 1 {
			t.Errorf("expected RewriteLoops == 1 (70%% under 80%% threshold triggers rewrite), got %d", res70.RewriteLoops)
		}
		if res70.UnderTarget {
			t.Errorf("expected UnderTarget == false after rewrite to 1800 words, got true")
		}
	})
}

// 新增「MaxRewriteLoops=0 仅遥测」：under 且 Max=0 → RewriteLoops == 0、UnderTarget == true、issues 含遥测文案。
func TestChapterWorkshop_WordTelemetry_MaxRewriteLoopsZero(t *testing.T) {
	draft800 := makeChineseRunes(800, "第八百字初稿段落。")

	reviewResp := `{
		"verdict": "ACCEPTED",
		"score": 90,
		"issues": [],
		"suggestions": "合格"
	}`

	mockClient := &mockWordcountLLMClient{
		renderDraft: draft800,
		reviewJSON:  reviewResp,
	}

	ws, _ := newWordcountTestWorkshop(mockClient)

	req := WorkshopProduceRequest{
		ProjectID:       "p-wordcount",
		ChapterIndex:    1,
		CoreConflict:    "宗门反派夜袭",
		ReasoningModel:  "mock-r1",
		WriterModel:     "mock-writer",
		ReviewerModel:   "mock-reviewer",
		WordsTarget:     2000,
		AutoCommit:      false,
		MaxRewriteLoops: 0,
	}

	res, err := ws.ProduceChapter(context.Background(), req)
	if err != nil {
		t.Fatalf("ProduceChapter failed: %v", err)
	}

	if res.RewriteLoops != 0 {
		t.Errorf("expected RewriteLoops == 0 when MaxRewriteLoops is 0, got %d", res.RewriteLoops)
	}
	if !res.UnderTarget {
		t.Errorf("expected UnderTarget == true, got false")
	}

	hasTelemetryIssue := false
	for _, issue := range res.Audit.Issues {
		if strings.Contains(issue, "【字数遥测】") {
			hasTelemetryIssue = true
			break
		}
	}
	if !hasTelemetryIssue {
		t.Errorf("expected Audit.Issues to contain 【字数遥测】, got: %v", res.Audit.Issues)
	}
}

// 2. TestChapterWorkshop_WordTelemetry_Passing:
// renderDraft 达标（≥80%）→ UnderTarget == false、无 【字数遥测】 issue、事件含实际/目标数字。
func TestChapterWorkshop_WordTelemetry_Passing(t *testing.T) {
	draft1800 := makeChineseRunes(1800, "第一千八百字达标段落。")
	if len([]rune(draft1800)) != 1800 {
		t.Fatalf("setup failed: expected 1800 runes, got %d", len([]rune(draft1800)))
	}

	reviewResp := `{
		"verdict": "ACCEPTED",
		"score": 92,
		"issues": [],
		"suggestions": "合格"
	}`

	mockClient := &mockWordcountLLMClient{
		renderDraft: draft1800,
		reviewJSON:  reviewResp,
	}

	ws, _ := newWordcountTestWorkshop(mockClient)

	var events []WorkshopEvent
	req := WorkshopProduceRequest{
		ProjectID:       "p-wordcount",
		ChapterIndex:    1,
		CoreConflict:    "宗门反派夜袭",
		ReasoningModel:  "mock-r1",
		WriterModel:     "mock-writer",
		ReviewerModel:   "mock-reviewer",
		WordsTarget:     2000,
		AutoCommit:      false,
		MaxRewriteLoops: 1,
		OnProgress: func(ev WorkshopEvent) {
			events = append(events, ev)
		},
	}

	res, err := ws.ProduceChapter(context.Background(), req)
	if err != nil {
		t.Fatalf("ProduceChapter failed: %v", err)
	}

	if res.ActualWords != len([]rune(draft1800)) {
		t.Errorf("expected ActualWords %d, got %d", len([]rune(draft1800)), res.ActualWords)
	}
	if res.WordsTarget != 2000 {
		t.Errorf("expected WordsTarget 2000, got %d", res.WordsTarget)
	}
	if res.UnderTarget {
		t.Errorf("expected UnderTarget == false, got true")
	}
	if math.Abs(res.WordQuotaRatio-0.9) > 0.0001 {
		t.Errorf("expected WordQuotaRatio ≈ 0.9, got %f", res.WordQuotaRatio)
	}

	// Audit.Issues must NOT contain 【字数遥测】
	for _, issue := range res.Audit.Issues {
		if strings.Contains(issue, "【字数遥测】") {
			t.Errorf("expected no 【字数遥测】 issue for passing draft, got: %s", issue)
		}
	}

	// Events must contain PhaseDrafted event with 字数遥测 and actual 1800 / target 2000
	hasTelemetryEvent := false
	for _, ev := range events {
		if ev.Phase == PhaseDrafted && strings.Contains(ev.Message, "字数遥测") && strings.Contains(ev.Message, "1800") && strings.Contains(ev.Message, "2000") {
			hasTelemetryEvent = true
			break
		}
	}
	if !hasTelemetryEvent {
		t.Errorf("expected progress event with 字数遥测 (1800/2000), got events: %+v", events)
	}
}

// 3. TestChapterWorkshop_WordTelemetry_OnFinalDraftAfterRewrite:
// 构造首稿短 + 改写稿达标（照既有 rewrite mock 模式）→ 遥测取改写后 final（ActualWords 为改写稿字数、无 under issue）。
func TestChapterWorkshop_WordTelemetry_OnFinalDraftAfterRewrite(t *testing.T) {
	draft600 := makeChineseRunes(600, "第六百字短篇初稿。")
	rewriteDraft1800 := makeChineseRunes(1800, "REWRITE_KEYWORD：第一千八百字改写扩写达标终稿。")

	initialReviewResp := "```json\n" + `{
		"verdict": "REVISION_NEEDED",
		"score": 70,
		"issues": ["篇幅偏短，需要深度展开"],
		"suggestions": "请扩充并包含 REWRITE_KEYWORD 场景"
	}` + "\n```"

	rewriteReviewResp := "```json\n" + `{
		"verdict": "ACCEPTED",
		"score": 95,
		"issues": [],
		"suggestions": "改写达标，结构完整"
	}` + "\n```"

	mockClient := &mockWordcountLLMClient{
		renderDraft:       draft600,
		reviewJSON:        initialReviewResp,
		rewriteDraft:      rewriteDraft1800,
		rewriteReviewJSON: rewriteReviewResp,
	}

	ws, _ := newWordcountTestWorkshop(mockClient)

	var events []WorkshopEvent
	req := WorkshopProduceRequest{
		ProjectID:       "p-wordcount",
		ChapterIndex:    1,
		CoreConflict:    "宗门反派夜袭",
		ReasoningModel:  "mock-r1",
		WriterModel:     "mock-writer",
		ReviewerModel:   "mock-reviewer",
		WordsTarget:     2000,
		AutoCommit:      false,
		MaxRewriteLoops: 2,
		OnProgress: func(ev WorkshopEvent) {
			events = append(events, ev)
		},
	}

	res, err := ws.ProduceChapter(context.Background(), req)
	if err != nil {
		t.Fatalf("ProduceChapter failed: %v", err)
	}

	if res.RewriteLoops != 1 {
		t.Errorf("expected RewriteLoops == 1, got %d", res.RewriteLoops)
	}
	if res.Content != rewriteDraft1800 {
		t.Errorf("expected Content to be rewriteDraft1800, got: %s", res.Content)
	}
	if res.ActualWords != len([]rune(rewriteDraft1800)) {
		t.Errorf("expected ActualWords to match final rewrite draft (%d), got %d", len([]rune(rewriteDraft1800)), res.ActualWords)
	}
	if res.WordsTarget != 2000 {
		t.Errorf("expected WordsTarget 2000, got %d", res.WordsTarget)
	}
	if res.UnderTarget {
		t.Errorf("expected UnderTarget == false on final draft after rewrite, got true")
	}
	if math.Abs(res.WordQuotaRatio-0.9) > 0.0001 {
		t.Errorf("expected WordQuotaRatio ≈ 0.9 on final draft, got %f", res.WordQuotaRatio)
	}

	// Must NOT contain 【字数遥测】 issue
	for _, issue := range res.Audit.Issues {
		if strings.Contains(issue, "【字数遥测】") {
			t.Errorf("expected no 【字数遥测】 issue on final passing draft after rewrite, got: %s", issue)
		}
	}

	// Events must contain the final telemetry event reflecting 1800 words
	hasFinalTelemetryEvent := false
	for _, ev := range events {
		if ev.Phase == PhaseDrafted && strings.Contains(ev.Message, "字数遥测") && strings.Contains(ev.Message, "1800") && strings.Contains(ev.Message, "2000") {
			hasFinalTelemetryEvent = true
			break
		}
	}
	if !hasFinalTelemetryEvent {
		t.Errorf("expected final progress event reflecting 1800 words telemetry, got events: %+v", events)
	}
}
