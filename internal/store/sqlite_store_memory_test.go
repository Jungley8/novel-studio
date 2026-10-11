package store_test

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Jungley8/novel-studio/internal/domain"
	"github.com/Jungley8/novel-studio/internal/store"
)

func TestSQLiteStore_MemoryFTS_AllScenarios(t *testing.T) {
	ctx := context.Background()
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test_memory_fts.db")

	s, err := store.NewSQLiteStore(dbPath)
	if err != nil {
		t.Fatalf("NewSQLiteStore failed: %v", err)
	}
	defer s.Close()

	// 1. 准备项目与源数据
	projA := &domain.Project{
		ID:             "proj-alpha",
		Title:          "修仙纪元",
		TargetPlatform: "fanqie",
	}
	if err := s.SaveProject(ctx, projA); err != nil {
		t.Fatalf("SaveProject A failed: %v", err)
	}

	projB := &domain.Project{
		ID:             "proj-beta",
		Title:          "都市之神",
		TargetPlatform: "qidian",
	}
	if err := s.SaveProject(ctx, projB); err != nil {
		t.Fatalf("SaveProject B failed: %v", err)
	}

	now := time.Now().Truncate(time.Second)

	// 1.1 Project A: 2 章摘要（含不同关键词）
	csA1 := &domain.ChapterSummary{
		ProjectID:    "proj-alpha",
		ChapterIndex: 1,
		Text:         "林凡在黑煞谷遭青云门截杀，反手祭出残剑斩敌首，坠入葬仙渊。",
		CreatedAt:    now,
	}
	csA2 := &domain.ChapterSummary{
		ProjectID:    "proj-alpha",
		ChapterIndex: 2,
		Text:         "林凡在深渊洞府偶获上古剑诀，强行炼化魔血恢复伤势，引动雷劫异象。",
		CreatedAt:    now.Add(time.Minute),
	}
	if err := s.SaveChapterSummary(ctx, csA1); err != nil {
		t.Fatalf("SaveChapterSummary csA1 failed: %v", err)
	}
	if err := s.SaveChapterSummary(ctx, csA2); err != nil {
		t.Fatalf("SaveChapterSummary csA2 failed: %v", err)
	}

	// 1.2 Project A: 2 钩子
	hA1 := &domain.PlotHook{
		ID:             "hook-alpha-1",
		ProjectID:      "proj-alpha",
		Title:          "青铜残片",
		Details:        "后山深坑捡到的生锈残片，刻有未知符文",
		CreatedChapter: 1,
		TargetChapter:  5,
		Status:         domain.HookStatusOpen,
		CreatedAt:      now,
	}
	hA2 := &domain.PlotHook{
		ID:             "hook-alpha-2",
		ProjectID:      "proj-alpha",
		Title:          "九幽冥火",
		Details:        "封印阵法深处跳动的黑色火种",
		CreatedChapter: 2,
		TargetChapter:  8,
		Status:         domain.HookStatusOpen,
		CreatedAt:      now,
	}
	if err := s.SavePlotHook(ctx, hA1); err != nil {
		t.Fatalf("SavePlotHook hA1 failed: %v", err)
	}
	if err := s.SavePlotHook(ctx, hA2); err != nil {
		t.Fatalf("SavePlotHook hA2 failed: %v", err)
	}

	// 1.3 Project A: 1 卷回顾
	vrA1 := &domain.VolumeRetrospective{
		ProjectID:   "proj-alpha",
		VolumeIndex: 1,
		Text:        "第一卷苍茫风云总结：少年破境逃亡，种下九幽伏笔，踏入修罗血海。",
		FromChapter: 1,
		ToChapter:   10,
		CreatedAt:   now,
	}
	if err := s.SaveVolumeRetrospective(ctx, vrA1); err != nil {
		t.Fatalf("SaveVolumeRetrospective vrA1 failed: %v", err)
	}

	// 1.4 Project B: 1 章摘要 + 1 钩子（用于隔离性验证）
	csB1 := &domain.ChapterSummary{
		ProjectID:    "proj-beta",
		ChapterIndex: 1,
		Text:         "楚天在顶级拍卖会豪掷千金买下青铜残片，震惊四方巨擘。",
		CreatedAt:    now,
	}
	hB1 := &domain.PlotHook{
		ID:             "hook-beta-1",
		ProjectID:      "proj-beta",
		Title:          "隐秘黑卡",
		Details:        "跨国财阀发放的黑卡",
		CreatedChapter: 1,
		TargetChapter:  3,
		Status:         domain.HookStatusOpen,
		CreatedAt:      now,
	}
	if err := s.SaveChapterSummary(ctx, csB1); err != nil {
		t.Fatalf("SaveChapterSummary csB1 failed: %v", err)
	}
	if err := s.SavePlotHook(ctx, hB1); err != nil {
		t.Fatalf("SavePlotHook hB1 failed: %v", err)
	}

	// 2. 执行 RebuildMemoryIndex
	if err := s.RebuildMemoryIndex(ctx, "proj-alpha"); err != nil {
		t.Fatalf("RebuildMemoryIndex proj-alpha failed: %v", err)
	}
	if err := s.RebuildMemoryIndex(ctx, "proj-beta"); err != nil {
		t.Fatalf("RebuildMemoryIndex proj-beta failed: %v", err)
	}

	// ------------------------------------------------------------------------
	// 验证项 1: 中文 2 字词分别命中对应 kind 与章号
	// ------------------------------------------------------------------------
	t.Run("HitKindAndChapterIndex", func(t *testing.T) {
		// 检索第 1 章关键词 "黑煞"
		hits1, err := s.SearchMemory(ctx, "proj-alpha", "黑煞", 10)
		if err != nil {
			t.Fatalf("SearchMemory 黑煞 failed: %v", err)
		}
		if len(hits1) == 0 {
			t.Fatalf("expected hit for 黑煞, got 0")
		}
		if hits1[0].Kind != "summary" || hits1[0].ChapterIndex != 1 || hits1[0].RefID != "chapter:1" {
			t.Errorf("unexpected hit1: kind=%s, chap=%d, ref=%s", hits1[0].Kind, hits1[0].ChapterIndex, hits1[0].RefID)
		}

		// 检索第 2 章关键词 "雷劫"
		hits2, err := s.SearchMemory(ctx, "proj-alpha", "雷劫", 10)
		if err != nil {
			t.Fatalf("SearchMemory 雷劫 failed: %v", err)
		}
		if len(hits2) == 0 {
			t.Fatalf("expected hit for 雷劫, got 0")
		}
		if hits2[0].Kind != "summary" || hits2[0].ChapterIndex != 2 || hits2[0].RefID != "chapter:2" {
			t.Errorf("unexpected hit2: kind=%s, chap=%d, ref=%s", hits2[0].Kind, hits2[0].ChapterIndex, hits2[0].RefID)
		}

		// 检索钩子 1 关键词 "生锈"
		hitsH1, err := s.SearchMemory(ctx, "proj-alpha", "生锈", 10)
		if err != nil {
			t.Fatalf("SearchMemory 生锈 failed: %v", err)
		}
		if len(hitsH1) == 0 {
			t.Fatalf("expected hit for 生锈, got 0")
		}
		if hitsH1[0].Kind != "hook" || hitsH1[0].ChapterIndex != 1 || hitsH1[0].RefID != "hook-alpha-1" {
			t.Errorf("unexpected hitH1: kind=%s, chap=%d, ref=%s", hitsH1[0].Kind, hitsH1[0].ChapterIndex, hitsH1[0].RefID)
		}

		// 检索钩子 2 关键词 "封印"
		hitsH2, err := s.SearchMemory(ctx, "proj-alpha", "封印", 10)
		if err != nil {
			t.Fatalf("SearchMemory 封印 failed: %v", err)
		}
		if len(hitsH2) == 0 {
			t.Fatalf("expected hit for 封印, got 0")
		}
		if hitsH2[0].Kind != "hook" || hitsH2[0].ChapterIndex != 2 || hitsH2[0].RefID != "hook-alpha-2" {
			t.Errorf("unexpected hitH2: kind=%s, chap=%d, ref=%s", hitsH2[0].Kind, hitsH2[0].ChapterIndex, hitsH2[0].RefID)
		}

		// 检索卷回顾关键词 "苍茫"
		hitsR1, err := s.SearchMemory(ctx, "proj-alpha", "苍茫", 10)
		if err != nil {
			t.Fatalf("SearchMemory 苍茫 failed: %v", err)
		}
		if len(hitsR1) == 0 {
			t.Fatalf("expected hit for 苍茫, got 0")
		}
		if hitsR1[0].Kind != "retro" || hitsR1[0].ChapterIndex != 0 || hitsR1[0].RefID != "volume:1" {
			t.Errorf("unexpected hitR1: kind=%s, chap=%d, ref=%s", hitsR1[0].Kind, hitsR1[0].ChapterIndex, hitsR1[0].RefID)
		}
	})

	// ------------------------------------------------------------------------
	// 验证项 2: 幂等性——连续 rebuild 两次，命中集合一致
	// ------------------------------------------------------------------------
	t.Run("Idempotence", func(t *testing.T) {
		hitsBefore, err := s.SearchMemory(ctx, "proj-alpha", "林凡", 10)
		if err != nil {
			t.Fatalf("SearchMemory before second rebuild failed: %v", err)
		}

		if err := s.RebuildMemoryIndex(ctx, "proj-alpha"); err != nil {
			t.Fatalf("second RebuildMemoryIndex failed: %v", err)
		}

		hitsAfter, err := s.SearchMemory(ctx, "proj-alpha", "林凡", 10)
		if err != nil {
			t.Fatalf("SearchMemory after second rebuild failed: %v", err)
		}

		if len(hitsBefore) != len(hitsAfter) {
			t.Fatalf("mismatched hit count: before=%d, after=%d", len(hitsBefore), len(hitsAfter))
		}
		for i := range hitsBefore {
			if hitsBefore[i].RefID != hitsAfter[i].RefID || hitsBefore[i].Kind != hitsAfter[i].Kind {
				t.Errorf("hit %d mismatch: before=%+v, after=%+v", i, hitsBefore[i], hitsAfter[i])
			}
		}
	})

	// ------------------------------------------------------------------------
	// 验证项 3: 项目隔离——第二个项目的数据不出现在第一个项目的检索结果
	// ------------------------------------------------------------------------
	t.Run("ProjectIsolation", func(t *testing.T) {
		// "拍卖" 仅在 Project B 中出现
		hitsA, err := s.SearchMemory(ctx, "proj-alpha", "拍卖", 10)
		if err != nil {
			t.Fatalf("SearchMemory in proj-alpha failed: %v", err)
		}
		if len(hitsA) != 0 {
			t.Errorf("expected 0 hits for proj-beta token in proj-alpha, got %d", len(hitsA))
		}

		hitsB, err := s.SearchMemory(ctx, "proj-beta", "拍卖", 10)
		if err != nil {
			t.Fatalf("SearchMemory in proj-beta failed: %v", err)
		}
		if len(hitsB) == 0 {
			t.Fatalf("expected hit for 拍卖 in proj-beta, got 0")
		}

		// "青铜" 在两个项目均有："proj-alpha" 为 hook-alpha-1，"proj-beta" 为 chapter:1
		hitsBronzeA, err := s.SearchMemory(ctx, "proj-alpha", "青铜", 10)
		if err != nil {
			t.Fatalf("SearchMemory 青铜 in proj-alpha failed: %v", err)
		}
		for _, h := range hitsBronzeA {
			if h.RefID != "hook-alpha-1" {
				t.Errorf("unexpected cross-project leak in proj-alpha: hit=%+v", h)
			}
		}

		hitsBronzeB, err := s.SearchMemory(ctx, "proj-beta", "青铜", 10)
		if err != nil {
			t.Fatalf("SearchMemory 青铜 in proj-beta failed: %v", err)
		}
		for _, h := range hitsBronzeB {
			if h.RefID != "chapter:1" {
				t.Errorf("unexpected cross-project leak in proj-beta: hit=%+v", h)
			}
		}
	})

	// ------------------------------------------------------------------------
	// 验证项 4: limit 生效；空查询返回空且无错
	// ------------------------------------------------------------------------
	t.Run("LimitAndEmptyQuery", func(t *testing.T) {
		// "林凡" 出现在 csA1 与 csA2 两章摘要中
		hitsLimit1, err := s.SearchMemory(ctx, "proj-alpha", "林凡", 1)
		if err != nil {
			t.Fatalf("SearchMemory limit 1 failed: %v", err)
		}
		if len(hitsLimit1) != 1 {
			t.Errorf("expected exactly 1 hit with limit=1, got %d", len(hitsLimit1))
		}

		hitsLimitAll, err := s.SearchMemory(ctx, "proj-alpha", "林凡", 20)
		if err != nil {
			t.Fatalf("SearchMemory limit 20 failed: %v", err)
		}
		if len(hitsLimitAll) < 2 {
			t.Errorf("expected at least 2 hits with limit=20, got %d", len(hitsLimitAll))
		}

		// 空串
		hitsEmpty, err := s.SearchMemory(ctx, "proj-alpha", "", 10)
		if err != nil {
			t.Fatalf("SearchMemory empty query returned error: %v", err)
		}
		if len(hitsEmpty) != 0 {
			t.Errorf("expected 0 hits for empty query, got %d", len(hitsEmpty))
		}

		// 纯标点空格
		hitsPunct, err := s.SearchMemory(ctx, "proj-alpha", " ，。？！ \t\n", 10)
		if err != nil {
			t.Fatalf("SearchMemory punctuation query returned error: %v", err)
		}
		if len(hitsPunct) != 0 {
			t.Errorf("expected 0 hits for punctuation query, got %d", len(hitsPunct))
		}
	})

	// ------------------------------------------------------------------------
	// 验证项 5: raw 字段为原文（非分词串）
	// ------------------------------------------------------------------------
	t.Run("RawFieldIsOriginalText", func(t *testing.T) {
		hits, err := s.SearchMemory(ctx, "proj-alpha", "黑煞", 10)
		if err != nil {
			t.Fatalf("SearchMemory 黑煞 failed: %v", err)
		}
		if len(hits) == 0 {
			t.Fatalf("expected hit for 黑煞")
		}

		raw := hits[0].Raw
		expectedSubstring := "林凡在黑煞谷遭青云门截杀"
		if !strings.Contains(raw, expectedSubstring) {
			t.Errorf("raw text %q does not contain expected original text %q", raw, expectedSubstring)
		}

		// 确认 raw 不是 2-gram 分词结果（不应含 "黑煞 煞谷" 之类）
		tokenizedArtifact := "黑煞 煞谷"
		if strings.Contains(raw, tokenizedArtifact) {
			t.Errorf("raw text %q appears to be tokenized string containing %q", raw, tokenizedArtifact)
		}
	})
}
