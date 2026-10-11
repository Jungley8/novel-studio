package engine_test

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"

	"github.com/Jungley8/novel-studio/internal/domain"
	"github.com/Jungley8/novel-studio/internal/engine"
)

type fakePipelineStore struct {
	mu        sync.Mutex
	projects  map[string]*domain.Project
	codex     map[string][]*domain.CodexEntry
	hooks     map[string][]*domain.PlotHook
	intents   map[string][]*domain.ChapterIntent
	relations map[string][]*domain.EntityRelation
}

func newFakePipelineStore() *fakePipelineStore {
	return &fakePipelineStore{
		projects:  make(map[string]*domain.Project),
		codex:     make(map[string][]*domain.CodexEntry),
		hooks:     make(map[string][]*domain.PlotHook),
		intents:   make(map[string][]*domain.ChapterIntent),
		relations: make(map[string][]*domain.EntityRelation),
	}
}

func (s *fakePipelineStore) GetProject(ctx context.Context, id string) (*domain.Project, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	p, ok := s.projects[id]
	if !ok {
		return nil, errors.New("project not found")
	}
	return p, nil
}

func (s *fakePipelineStore) SaveProject(ctx context.Context, p *domain.Project) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if p == nil || p.ID == "" {
		return errors.New("invalid project")
	}
	s.projects[p.ID] = p
	return nil
}

func (s *fakePipelineStore) SaveCodexEntry(ctx context.Context, e *domain.CodexEntry) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	list := s.codex[e.ProjectID]
	for idx, item := range list {
		if item.ID == e.ID {
			list[idx] = e
			return nil
		}
	}
	s.codex[e.ProjectID] = append(list, e)
	return nil
}

func (s *fakePipelineStore) SavePlotHook(ctx context.Context, h *domain.PlotHook) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	list := s.hooks[h.ProjectID]
	for idx, item := range list {
		if item.ID == h.ID {
			list[idx] = h
			return nil
		}
	}
	s.hooks[h.ProjectID] = append(list, h)
	return nil
}

func (s *fakePipelineStore) ListChapterIntents(ctx context.Context, projectID string, volumeIndex int) ([]*domain.ChapterIntent, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var res []*domain.ChapterIntent
	for _, it := range s.intents[projectID] {
		if volumeIndex <= 0 || it.VolumeIndex == volumeIndex {
			res = append(res, it)
		}
	}
	return res, nil
}

func (s *fakePipelineStore) SaveChapterIntent(ctx context.Context, i *domain.ChapterIntent) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	list := s.intents[i.ProjectID]
	for idx, item := range list {
		if item.ID == i.ID || item.ChapterIndex == i.ChapterIndex {
			list[idx] = i
			return nil
		}
	}
	s.intents[i.ProjectID] = append(list, i)
	return nil
}

func (s *fakePipelineStore) ListCodexEntries(ctx context.Context, projectID string, category domain.CodexCategory) ([]*domain.CodexEntry, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var res []*domain.CodexEntry
	for _, it := range s.codex[projectID] {
		if category == "" || it.Category == category {
			res = append(res, it)
		}
	}
	return res, nil
}

func (s *fakePipelineStore) SaveCodexRelation(ctx context.Context, projectID string, rel *domain.EntityRelation) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.relations[projectID] = append(s.relations[projectID], rel)
	return nil
}

type fakePipelineLLM struct {
	mu        sync.Mutex
	callCount int
}

func (f *fakePipelineLLM) ChatCompletion(ctx context.Context, model string, systemPrompt, userPrompt string, temp float64) (string, error) {
	resp, _, err := f.ChatCompletionWithUsage(ctx, model, systemPrompt, userPrompt, temp)
	return resp, err
}

func (f *fakePipelineLLM) ChatCompletionWithUsage(ctx context.Context, model string, systemPrompt, userPrompt string, temp float64) (string, engine.TokenUsage, error) {
	f.mu.Lock()
	f.callCount++
	f.mu.Unlock()

	usage := engine.TokenUsage{PromptTokens: 100, CompletionTokens: 150, TotalTokens: 250}

	// 1. Framework bootstrap分流
	if strings.Contains(systemPrompt, "创世总纲") || strings.Contains(userPrompt, "创世总纲") {
		frameworkJSON := `{
			"theme_premise": "万古长夜，一人一剑逆伐诸仙",
			"world_axioms": ["天道崩塌", "灵气含毒"],
			"power_ladder": [
				{"realm": "凡胎境", "description": "凡俗武夫", "bottleneck": "开辟气海", "drawback": "寿不过百"},
				{"realm": "凝气境", "description": "引气入体", "bottleneck": "凝练九重", "drawback": "丹毒淤积"}
			],
			"factions": [
				{"name": "天衍圣宗", "alignment": "守序正统", "doctrine": "替天行道", "threat_level": "极高"}
			],
			"key_characters": [
				{"name": "楚掌柜", "role": "引路导师", "realm": "凝气九重", "goal": "培育新秀", "fate_arc": "云游四海"}
			],
			"volume_arcs": [
				{
					"volume_index": 1,
					"title": "第一卷：边荒觉醒",
					"theme": "求存与复仇",
					"core_goal": "冲出边荒杀阵",
					"climax": "斩杀血煞老祖",
					"estimated_chapters": 2,
					"key_payoffs": ["觉醒不灭印"]
				},
				{
					"volume_index": 2,
					"title": "第二卷：宗门风云",
					"theme": "争锋与立足",
					"core_goal": "入主内门大比夺魁",
					"climax": "力压各峰首徒",
					"estimated_chapters": 2,
					"key_payoffs": ["掌控天衍剑阵"]
				}
			],
			"seed_hooks": [
				{
					"title": "胸前残印之谜",
					"details": "遇血发热的神秘铁印",
					"created_chapter": 1,
					"target_chapter": 10,
					"status": "OPEN"
				}
			]
		}`
		return frameworkJSON, usage, nil
	}

	// 2. 实体关系抽取分流
	if strings.Contains(systemPrompt, "关系网络分析") || strings.Contains(systemPrompt, "实体之间") {
		relationsJSON := `[
			{
				"source_name": "顾渊",
				"target_name": "聚仙楼",
				"relation_type": "LOCATED_IN",
				"description": "主角驻扎身处"
			}
		]`
		return relationsJSON, usage, nil
	}

	// 3. 章节意图推演分流
	if strings.Contains(systemPrompt, "剧情规划师") || strings.Contains(systemPrompt, "章节意图卡") {
		var targetChapters []int
		idx := strings.Index(userPrompt, "请为以下目标章号逐章推演意图卡：")
		if idx != -1 {
			chPart := strings.TrimSpace(userPrompt[idx+len("请为以下目标章号逐章推演意图卡："):])
			parts := strings.Split(chPart, ",")
			for _, p := range parts {
				p = strings.TrimSpace(p)
				var c int
				if _, err := fmt.Sscanf(p, "%d", &c); err == nil && c > 0 {
					targetChapters = append(targetChapters, c)
				}
			}
		}
		if len(targetChapters) == 0 {
			targetChapters = []int{1, 2}
		}

		var items []string
		for _, ch := range targetChapters {
			item := fmt.Sprintf(`{
				"chapter_index": %d,
				"conflict": "第 %d 章生死冲突阻碍",
				"pov": "顾渊",
				"emotion_curve": ["压抑", "反转", "扬眉吐气"],
				"hook_plants": ["偶得古剑残片"],
				"hook_payoffs": [],
				"tension_target": 7,
				"expected_mutation": {
					"inventory_delta": "获得聚气散",
					"power_delta": "突破凝气一重"
				}
			}`, ch, ch)
			items = append(items, item)
		}
		return "[" + strings.Join(items, ",") + "]", usage, nil
	}

	return "{}", usage, nil
}

func (f *fakePipelineLLM) ChatCompletionStream(ctx context.Context, model string, systemPrompt, userPrompt string, temp float64) (<-chan engine.StreamChunk, error) {
	resp, usage, err := f.ChatCompletionWithUsage(ctx, model, systemPrompt, userPrompt, temp)
	if err != nil {
		return nil, err
	}
	ch := make(chan engine.StreamChunk, 2)
	ch <- engine.StreamChunk{Delta: resp}
	ch <- engine.StreamChunk{Done: true, Usage: &usage}
	close(ch)
	return ch, nil
}

type fakeVolumeProducer struct {
	mu          sync.Mutex
	calls       []engine.VolumeProduceRequest
	produceFunc func(ctx context.Context, req engine.VolumeProduceRequest) (*engine.VolumeProduceResult, error)
}

func (p *fakeVolumeProducer) ProduceVolume(ctx context.Context, req engine.VolumeProduceRequest) (*engine.VolumeProduceResult, error) {
	p.mu.Lock()
	p.calls = append(p.calls, req)
	fn := p.produceFunc
	p.mu.Unlock()

	if fn != nil {
		return fn(ctx, req)
	}

	return &engine.VolumeProduceResult{
		VolumeIndex:  req.VolumeIndex,
		StartChapter: 1,
		EndChapter:   2,
		Chapters: []engine.VolumeChapterSummary{
			{ChapterIndex: 1, Committed: true, Score: 85},
			{ChapterIndex: 2, Committed: true, Score: 90},
		},
		TotalUsage: engine.TokenUsage{PromptTokens: 500, CompletionTokens: 1000, TotalTokens: 1500},
	}, nil
}

func TestStudioPipeline_FullAutoRun(t *testing.T) {
	ctx := context.Background()
	llm := &fakePipelineLLM{}
	orch := engine.NewOrchestrator(llm)
	producer := &fakeVolumeProducer{}
	store := newFakePipelineStore()

	pipeline := engine.NewStudioPipeline(orch, producer, store)

	var events []engine.PipelineEvent
	req := engine.PipelineRequest{
		Title:           "逆苍穹",
		TargetPlatform:  "起点仙侠",
		Concept:         "凡人修真弑神",
		ReasoningModel:  "deepseek-reasoner",
		Volumes:         1,
		AutoLockIntents: true,
		WordsTarget:     2500,
		ChapterFrom:     1,
		OnProgress: func(ev engine.PipelineEvent) {
			events = append(events, ev)
		},
	}

	res, err := pipeline.Run(ctx, req)
	if err != nil {
		t.Fatalf("pipeline run failed: %v", err)
	}

	if res.Halted {
		t.Fatalf("expected pipeline not halted, but got halted: %s", res.HaltReason)
	}

	if len(res.Volumes) != 1 {
		t.Fatalf("expected 1 volume result, got %d", len(res.Volumes))
	}

	// 1. 项目落库断言
	savedProj, err := store.GetProject(ctx, res.ProjectID)
	if err != nil || savedProj == nil {
		t.Fatalf("project was not saved in store: %v", err)
	}
	if savedProj.Title != "逆苍穹" {
		t.Errorf("expected project title 逆苍穹, got %s", savedProj.Title)
	}

	// 2. 意图全部 LOCKED 断言
	intents, err := store.ListChapterIntents(ctx, res.ProjectID, 1)
	if err != nil || len(intents) == 0 {
		t.Fatalf("expected chapter intents in store, got %v", intents)
	}
	for _, it := range intents {
		if it.Status != domain.IntentStatusLocked {
			t.Errorf("expected intent status %s for ch%d, got %s", domain.IntentStatusLocked, it.ChapterIndex, it.Status)
		}
	}

	// 3. producer 被调 1 次（参数：VolumeIndex/ChapterFrom/WordsTarget 正确）
	producer.mu.Lock()
	callCount := len(producer.calls)
	var firstCall engine.VolumeProduceRequest
	if callCount > 0 {
		firstCall = producer.calls[0]
	}
	producer.mu.Unlock()

	if callCount != 1 {
		t.Fatalf("expected producer to be called 1 time, got %d", callCount)
	}
	if firstCall.VolumeIndex != 1 {
		t.Errorf("expected producer call VolumeIndex 1, got %d", firstCall.VolumeIndex)
	}
	if firstCall.ChapterFrom != 1 {
		t.Errorf("expected producer call ChapterFrom 1, got %d", firstCall.ChapterFrom)
	}
	if firstCall.WordsTarget != 2500 {
		t.Errorf("expected producer call WordsTarget 2500, got %d", firstCall.WordsTarget)
	}

	// 4. TotalUsage > 0, Cost > 0
	if res.TotalUsage.TotalTokens <= 0 {
		t.Errorf("expected TotalUsage > 0, got %d", res.TotalUsage.TotalTokens)
	}
	if res.EstimatedCostUSD <= 0 {
		t.Errorf("expected EstimatedCostUSD > 0, got %f", res.EstimatedCostUSD)
	}

	// 5. 校验事件
	if len(events) == 0 {
		t.Errorf("expected progress events to be emitted")
	}
}

func TestStudioPipeline_NoAutoLock_HaltsBeforeProduction(t *testing.T) {
	ctx := context.Background()
	llm := &fakePipelineLLM{}
	orch := engine.NewOrchestrator(llm)
	producer := &fakeVolumeProducer{}
	store := newFakePipelineStore()

	pipeline := engine.NewStudioPipeline(orch, producer, store)

	req := engine.PipelineRequest{
		Title:           "逆苍穹",
		TargetPlatform:  "起点仙侠",
		Concept:         "凡人修真弑神",
		ReasoningModel:  "deepseek-reasoner",
		Volumes:         1,
		AutoLockIntents: false,
	}

	res, err := pipeline.Run(ctx, req)
	if err != nil {
		t.Fatalf("pipeline run returned error: %v", err)
	}

	// 断言：result.Halted=true、HaltReason 含"意图"、producer 零调用
	if !res.Halted {
		t.Fatalf("expected pipeline to be halted when AutoLockIntents=false")
	}
	if !strings.Contains(res.HaltReason, "意图") {
		t.Errorf("expected HaltReason to contain '意图', got %q", res.HaltReason)
	}

	producer.mu.Lock()
	callCount := len(producer.calls)
	producer.mu.Unlock()

	if callCount != 0 {
		t.Fatalf("expected producer to have 0 calls, got %d", callCount)
	}
}

func TestStudioPipeline_ProducerStoppedPropagates(t *testing.T) {
	ctx := context.Background()
	llm := &fakePipelineLLM{}
	orch := engine.NewOrchestrator(llm)
	producer := &fakeVolumeProducer{
		produceFunc: func(ctx context.Context, req engine.VolumeProduceRequest) (*engine.VolumeProduceResult, error) {
			if req.VolumeIndex == 1 {
				return &engine.VolumeProduceResult{
					VolumeIndex:      1,
					StartChapter:     1,
					EndChapter:       2,
					Stopped:          true,
					StoppedAtChapter: 2,
					StopReason:       "HALTED_ON_QUALITY_GATE",
					TotalUsage:       engine.TokenUsage{PromptTokens: 200, CompletionTokens: 200, TotalTokens: 400},
				}, nil
			}
			return &engine.VolumeProduceResult{
				VolumeIndex: 2,
			}, nil
		},
	}
	store := newFakePipelineStore()

	pipeline := engine.NewStudioPipeline(orch, producer, store)

	req := engine.PipelineRequest{
		Title:           "逆苍穹",
		TargetPlatform:  "起点仙侠",
		Concept:         "凡人修真弑神",
		ReasoningModel:  "deepseek-reasoner",
		Volumes:         2,
		AutoLockIntents: true,
	}

	res, err := pipeline.Run(ctx, req)
	if err != nil {
		t.Fatalf("pipeline run failed: %v", err)
	}

	// 2 卷、producer 第 1 卷返回 Stopped → result.Halted=true、Volumes 长度 1、producer 只调 1 次
	if !res.Halted {
		t.Fatalf("expected result.Halted=true when producer stopped")
	}
	if res.HaltReason != "HALTED_ON_QUALITY_GATE" {
		t.Errorf("expected HaltReason 'HALTED_ON_QUALITY_GATE', got %q", res.HaltReason)
	}
	if len(res.Volumes) != 1 {
		t.Fatalf("expected 1 volume result, got %d", len(res.Volumes))
	}

	producer.mu.Lock()
	callCount := len(producer.calls)
	producer.mu.Unlock()

	if callCount != 1 {
		t.Fatalf("expected producer to be called only 1 time, got %d", callCount)
	}
}

func TestStudioPipeline_Validation(t *testing.T) {
	ctx := context.Background()
	llm := &fakePipelineLLM{}
	orch := engine.NewOrchestrator(llm)
	producer := &fakeVolumeProducer{}
	store := newFakePipelineStore()

	pipeline := engine.NewStudioPipeline(orch, producer, store)

	// 1. 空 Title
	_, err := pipeline.Run(ctx, engine.PipelineRequest{
		Title:   "",
		Concept: "有效概念",
	})
	if err == nil {
		t.Errorf("expected error on empty title, got nil")
	}

	// 2. 空 Concept
	_, err = pipeline.Run(ctx, engine.PipelineRequest{
		Title:   "有效书名",
		Concept: "",
	})
	if err == nil {
		t.Errorf("expected error on empty concept, got nil")
	}

	// 3. Volumes < 0
	_, err = pipeline.Run(ctx, engine.PipelineRequest{
		Title:   "有效书名",
		Concept: "有效概念",
		Volumes: -1,
	})
	if err == nil {
		t.Errorf("expected error on negative volumes, got nil")
	}

	// 4. Volumes 超卷数 (mock framework 只有 2 卷，请求 10 卷)
	_, err = pipeline.Run(ctx, engine.PipelineRequest{
		Title:   "有效书名",
		Concept: "有效概念",
		Volumes: 10,
	})
	if err == nil {
		t.Errorf("expected error when requested volumes exceeds planned volumes, got nil")
	}
}
