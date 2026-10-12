package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"reflect"
	"strings"
	"time"

	"github.com/Jungley8/novel-studio/internal/activity"
	"github.com/Jungley8/novel-studio/internal/domain"
	"github.com/Jungley8/novel-studio/internal/engine"
)

func (s *Server) handleProjectFramework(w http.ResponseWriter, r *http.Request, projectID string, parts []string) {
	ctx := r.Context()
	p, err := s.store.GetProject(ctx, projectID)
	if err != nil {
		errorResponse(w, http.StatusNotFound, "project not found")
		return
	}

	// POST /api/projects/:id/framework/bootstrap
	if len(parts) >= 3 && parts[2] == "bootstrap" {
		if r.Method != http.MethodPost {
			errorResponse(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}
		var req struct {
			Concept        string `json:"concept"`
			ReasoningModel string `json:"reasoning_model,omitempty"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		fwRunID := activity.NewRunID()
		ctx = activity.WithRunID(activity.WithSource(ctx, activity.SourceBootstrap), fwRunID)
		concept := req.Concept
		if concept == "" {
			concept = p.WorldRules
		}
		model := req.ReasoningModel
		if model == "" {
			model = s.cfg.ReasoningModel
		}

		fw, _, bErr := s.orch.BootstrapFramework(ctx, model, engine.FrameworkBootstrapRequest{
			Title:          p.Title,
			TargetPlatform: p.TargetPlatform,
			CoreConcept:    concept,
		})
		if bErr != nil {
			errorResponse(w, http.StatusInternalServerError, "bootstrap framework failed: "+bErr.Error())
			return
		}

		p.Framework = fw
		if p.WorldRules == "" && len(fw.WorldAxioms) > 0 {
			var sb strings.Builder
			sb.WriteString("【世界底层规则与核心公理】\n")
			for i, ax := range fw.WorldAxioms {
				sb.WriteString(fmt.Sprintf("%d. %s\n", i+1, ax))
			}
			p.WorldRules = sb.String()
		}
		if err := s.store.SaveProject(ctx, p); err != nil {
			errorResponse(w, http.StatusInternalServerError, err.Error())
			return
		}

		// 同步主角至设定集 (Codex)
		initialRealm := "初入门槛"
		if len(fw.PowerLadder) > 0 && fw.PowerLadder[0].Realm != "" {
			initialRealm = fw.PowerLadder[0].Realm
		}
		protEntry, _ := s.store.GetProtagonistEntry(ctx, projectID)
		if protEntry == nil {
			protID := fmt.Sprintf("codex_%s_pro", projectID)
			protName := "主角"
			protEntry = &domain.CodexEntry{
				ID:              protID,
				ProjectID:       projectID,
				Category:        domain.CategoryCharacter,
				Archetype:       "PROTAGONIST",
				Name:            protName,
				Realm:           initialRealm,
				Aliases:         []string{"主角", fmt.Sprintf("%s (%s)", protName, initialRealm)},
				Summary:         fmt.Sprintf("全书核心主角 | 初始境界: %s | 核心长线动机: %s", initialRealm, fw.ThemePremise),
				DetailsMarkdown: fmt.Sprintf("【全书核心主角档案】\n- 初始境界：%s\n- 核心终极目标：%s\n- 随身底牌与物品：随身物品待起草\n- 当前状态：良好", initialRealm, fw.ThemePremise),
				Inventory:       "随身物品待起草",
				HealthStatus:    "良好",
				TrackingMode:    domain.TrackingModeAlwaysInject,
				CreatedAt:       time.Now(),
				UpdatedAt:       time.Now(),
			}
		} else {
			if protEntry.Realm == "" {
				protEntry.Realm = initialRealm
			}
			if protEntry.CoreMotivation == "" {
				protEntry.CoreMotivation = fw.ThemePremise
			}
			if protEntry.Archetype == "" {
				protEntry.Archetype = "PROTAGONIST"
			}
			protEntry.UpdatedAt = time.Now()
		}
		_ = s.store.SaveCodexEntry(ctx, protEntry)

		// 同步势力阵营至设定集 (Codex)
		for _, fac := range fw.Factions {
			if strings.TrimSpace(fac.Name) == "" {
				continue
			}
			entry := &domain.CodexEntry{
				ID:              fmt.Sprintf("codex_%s_fac_%d", projectID, time.Now().UnixNano()),
				ProjectID:       projectID,
				Category:        domain.CategoryFaction,
				Name:            fac.Name,
				Alignment:       fac.Alignment,
				ThreatLevel:     fac.ThreatLevel,
				Summary:         fmt.Sprintf("立场: %s | 威胁度: %s", fac.Alignment, fac.ThreatLevel),
				DetailsMarkdown: fac.Doctrine,
				TrackingMode:    domain.TrackingModeAutoMention,
				CreatedAt:       time.Now(),
				UpdatedAt:       time.Now(),
			}
			_ = s.store.SaveCodexEntry(ctx, entry)
		}

		// 同步关键角色谱系至设定集 (Codex)
		for _, kc := range fw.KeyCharacters {
			if strings.TrimSpace(kc.Name) == "" {
				continue
			}
			entry := &domain.CodexEntry{
				ID:              fmt.Sprintf("codex_%s_char_%d", projectID, time.Now().UnixNano()),
				ProjectID:       projectID,
				Category:        domain.CategoryCharacter,
				Name:            kc.Name,
				Summary:         fmt.Sprintf("定位: %s | 境界: %s | 动机: %s", kc.Role, kc.Realm, kc.Goal),
				DetailsMarkdown: fmt.Sprintf("宿命终局: %s", kc.FateArc),
				TrackingMode:    domain.TrackingModeAutoMention,
				CreatedAt:       time.Now(),
				UpdatedAt:       time.Now(),
			}
			_ = s.store.SaveCodexEntry(ctx, entry)
		}

		// Seed hooks
		for i, sh := range fw.SeedHooks {
			hook := &domain.PlotHook{
				ID:             fmt.Sprintf("hook_%s_fw_%d", projectID, i+1),
				ProjectID:      projectID,
				Title:          sh.Title,
				Details:        sh.Details,
				CreatedChapter: sh.CreatedChapter,
				TargetChapter:  sh.TargetChapter,
				Status:         domain.HookStatusOpen,
				CreatedAt:      time.Now(),
			}
			_ = s.store.SavePlotHook(ctx, hook)
		}

		// 全自动级联关系图谱推演 (Extract Codex Relations Cascade)
		codexEntries, _ := s.store.ListCodexEntries(ctx, projectID, "")
		if len(codexEntries) >= 2 && s.orch != nil {
			relations, _, relErr := s.orch.ExtractCodexRelations(
				ctx,
				s.cfg.ReasoningModel,
				p,
				codexEntries,
				p.WorldRules+"\n\n全书核心立意："+fw.ThemePremise,
			)
			if relErr == nil && len(relations) > 0 {
				for _, rel := range relations {
					_ = s.store.SaveCodexRelation(ctx, projectID, &rel)
				}
			}
		}

		// 全自动推演第 1 卷第 1 章开局黄金冲突并初始化草稿检查点
		if s.chronicle != nil && s.orch != nil {
			horizon, hErr := s.chronicle.AssembleHorizon(ctx, projectID, 1)
			if hErr == nil && horizon != nil {
				conflict1, _, cErr := s.orch.SuggestChapterConflict(ctx, s.cfg.ReasoningModel, horizon)
				if cErr == nil && strings.TrimSpace(conflict1) != "" {
					_ = s.store.SaveCheckpoint(ctx, &domain.ChapterCheckpoint{
						ProjectID:    projectID,
						ChapterIndex: 1,
						Phase:        domain.CheckpointPhaseInit,
						CoreConflict: conflict1,
						UpdatedAt:    time.Now(),
					})
				}
			}
		}

		// Ledger aggregate marker for the re-bootstrap run.
		_ = s.store.AppendEvent(ctx, &activity.ActivityEvent{
			ProjectID:  projectID,
			RunID:      fwRunID,
			EntityType: activity.EntityFramework,
			EntityID:   projectID,
			EventType:  activity.EventFrameworkBoot,
			Actor:      activity.ActorAIReasoner,
			Source:     activity.SourceBootstrap,
			Title:      fmt.Sprintf("创世总纲已重新推演《%s》", p.Title),
			Summary: fmt.Sprintf("%d 卷 · %d 势力 · %d 关键人物 · %d 种子伏笔",
				len(fw.VolumeArcs), len(fw.Factions), len(fw.KeyCharacters), len(fw.SeedHooks)),
			CreatedAt: time.Now(),
		})

		// Transient seeds have been persisted to codex_entries and plot_hooks.
		// Clean them from the macro framework to maintain a single source of truth.
		fw.Factions = nil
		fw.KeyCharacters = nil
		fw.SeedHooks = nil
		_ = s.store.SaveProject(ctx, p)

		jsonResponse(w, http.StatusOK, p.Framework)
		return
	}

	// GET or PUT /api/projects/:id/framework
	switch r.Method {
	case http.MethodGet:
		if p.Framework == nil {
			jsonResponse(w, http.StatusOK, map[string]any{"framework": nil, "message": "总纲尚未推演生成"})
			return
		}
		jsonResponse(w, http.StatusOK, p.Framework)
	case http.MethodPut:
		var fw domain.ProjectFramework
		if err := json.NewDecoder(r.Body).Decode(&fw); err != nil {
			errorResponse(w, http.StatusBadRequest, "invalid framework JSON: "+err.Error())
			return
		}

		// 保存前快照旧卷定义与框架级前提
		var (
			oldArcs    []domain.VolumeArc
			oldAxioms  []string
			oldLadder  []domain.PowerLadderTier
			oldPremise string
		)
		if p.Framework != nil {
			if len(p.Framework.VolumeArcs) > 0 {
				oldArcs = make([]domain.VolumeArc, len(p.Framework.VolumeArcs))
				copy(oldArcs, p.Framework.VolumeArcs)
			}
			if len(p.Framework.WorldAxioms) > 0 {
				oldAxioms = make([]string, len(p.Framework.WorldAxioms))
				copy(oldAxioms, p.Framework.WorldAxioms)
			}
			if len(p.Framework.PowerLadder) > 0 {
				oldLadder = make([]domain.PowerLadderTier, len(p.Framework.PowerLadder))
				copy(oldLadder, p.Framework.PowerLadder)
			}
			oldPremise = p.Framework.ThemePremise
		}

		p.Framework = &fw
		if len(fw.WorldAxioms) > 0 {
			var sb strings.Builder
			for i, ax := range fw.WorldAxioms {
				sb.WriteString(fmt.Sprintf("%d. %s\n", i+1, ax))
			}
			p.WorldRules = strings.TrimSpace(sb.String())
		}
		if err := s.store.SaveProject(ctx, p); err != nil {
			errorResponse(w, http.StatusInternalServerError, err.Error())
			return
		}

		// 卷变更联动：保存前读出旧 framework，保存后对两边先跑 engine.BackfillVolumeArcs 归一化，
		// 逐卷比较 (StartChapter, EndChapter)；凡发生变化的卷，调 store.MarkIntentsStale(projectID, volumeIndex)。
		// 架构依赖说明：server 包已导入 engine 与 store，调用 engine.BackfillVolumeArcs 不会形成循环依赖（engine 未导入 server）。
		normOld := engine.BackfillVolumeArcs(oldArcs)
		normNew := engine.BackfillVolumeArcs(fw.VolumeArcs)

		type chapterRange struct {
			start int
			end   int
		}
		oldMap := make(map[int]chapterRange)
		for _, arc := range normOld {
			oldMap[arc.VolumeIndex] = chapterRange{start: arc.StartChapter, end: arc.EndChapter}
		}
		newMap := make(map[int]chapterRange)
		for _, arc := range normNew {
			newMap[arc.VolumeIndex] = chapterRange{start: arc.StartChapter, end: arc.EndChapter}
		}

		checked := make(map[int]bool)
		for volIdx, oldR := range oldMap {
			checked[volIdx] = true
			newR, exists := newMap[volIdx]
			if !exists || oldR != newR {
				_, _ = s.store.MarkIntentsStale(ctx, projectID, volIdx)
			}
		}
		for volIdx := range newMap {
			if !checked[volIdx] {
				_, _ = s.store.MarkIntentsStale(ctx, projectID, volIdx)
			}
		}

		// 框架级前提变更 → 全卷 LOCKED 意图失效；卷界变更走上方逐卷块；仅 LOCKED→STALE，DRAFT 不受影响
		if !reflect.DeepEqual(oldAxioms, fw.WorldAxioms) || !reflect.DeepEqual(oldLadder, fw.PowerLadder) || oldPremise != fw.ThemePremise {
			intents, _ := s.store.ListChapterIntents(ctx, projectID, 0)
			vols := make(map[int]bool)
			for _, it := range intents {
				if it != nil {
					vols[it.VolumeIndex] = true
				}
			}
			for volIdx := range vols {
				_, _ = s.store.MarkIntentsStale(ctx, projectID, volIdx)
			}
		}

		jsonResponse(w, http.StatusOK, p.Framework)
	default:
		errorResponse(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func (s *Server) handleProjectCodex(w http.ResponseWriter, r *http.Request, projectID string, subParts []string) {
	ctx := r.Context()

	// 1. /api/projects/:id/codex (list or create)
	if len(subParts) == 0 {
		switch r.Method {
		case http.MethodGet:
			category := domain.CodexCategory(r.URL.Query().Get("category"))
			entries, err := s.store.ListCodexEntries(ctx, projectID, category)
			if err != nil {
				errorResponse(w, http.StatusInternalServerError, err.Error())
				return
			}
			if entries == nil {
				entries = []*domain.CodexEntry{}
			}
			jsonResponse(w, http.StatusOK, entries)
		case http.MethodPost:
			var entry domain.CodexEntry
			if err := json.NewDecoder(r.Body).Decode(&entry); err != nil {
				errorResponse(w, http.StatusBadRequest, "invalid json: "+err.Error())
				return
			}
			entry.ProjectID = projectID
			if err := s.store.SaveCodexEntry(ctx, &entry); err != nil {
				errorResponse(w, http.StatusBadRequest, err.Error())
				return
			}
			jsonResponse(w, http.StatusCreated, entry)
		default:
			errorResponse(w, http.StatusMethodNotAllowed, "method not allowed")
		}
		return
	}

	sub := subParts[0]

	// 2. /api/projects/:id/codex/ai-generate
	if sub == "ai-generate" {
		if r.Method != http.MethodPost {
			errorResponse(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}
		var req engine.CodexGenerateRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			errorResponse(w, http.StatusBadRequest, "invalid json: "+err.Error())
			return
		}
		proj, err := s.store.GetProject(ctx, projectID)
		if err != nil {
			errorResponse(w, http.StatusNotFound, "project not found")
			return
		}
		entry, _, err := s.orch.GenerateCodexEntry(ctx, s.cfg.ReasoningModel, proj, req)
		if err != nil {
			errorResponse(w, http.StatusInternalServerError, "ai generate codex entry failed: "+err.Error())
			return
		}
		if err := s.store.SaveCodexEntry(ctx, entry); err != nil {
			errorResponse(w, http.StatusInternalServerError, "save generated codex entry failed: "+err.Error())
			return
		}
		jsonResponse(w, http.StatusCreated, entry)
		return
	}

	// 3. /api/projects/:id/codex/scan
	if sub == "scan" {
		if r.Method != http.MethodPost {
			errorResponse(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}
		var req struct {
			Text string `json:"text"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			errorResponse(w, http.StatusBadRequest, "invalid json: "+err.Error())
			return
		}
		entries, err := s.store.ListCodexEntries(ctx, projectID, "")
		if err != nil {
			errorResponse(w, http.StatusInternalServerError, err.Error())
			return
		}
		scanner := engine.NewMentionScanner()
		matched, mentions := scanner.ScanText(req.Text, entries)
		if matched == nil {
			matched = []*domain.CodexEntry{}
		}
		if mentions == nil {
			mentions = []engine.MatchedMention{}
		}
		jsonResponse(w, http.StatusOK, map[string]interface{}{
			"matched_entries": matched,
			"mentions":        mentions,
		})
		return
	}

	// 4. /api/projects/:id/codex/relations (GET or POST) & /api/projects/:id/codex/relations/:relId (DELETE)
	if sub == "relations" {
		if len(subParts) > 1 {
			if subParts[1] == "ai-extract" {
				if r.Method != http.MethodPost {
					errorResponse(w, http.StatusMethodNotAllowed, "method not allowed")
					return
				}
				proj, err := s.store.GetProject(ctx, projectID)
				if err != nil {
					errorResponse(w, http.StatusNotFound, "project not found")
					return
				}
				entries, err := s.store.ListCodexEntries(ctx, projectID, "")
				if err != nil {
					errorResponse(w, http.StatusInternalServerError, err.Error())
					return
				}
				chapters, _ := s.store.ListChapters(ctx, projectID)
				var chapterText strings.Builder
				for _, ch := range chapters {
					chapterText.WriteString(fmt.Sprintf("第%d章 %s: %s\n", ch.ChapterIndex, ch.Title, ch.CoreConflict))
				}
				rels, _, err := s.orch.ExtractCodexRelations(ctx, s.cfg.ReasoningModel, proj, entries, chapterText.String())
				if err != nil {
					errorResponse(w, http.StatusInternalServerError, "ai extract relations failed: "+err.Error())
					return
				}
				for i := range rels {
					_ = s.store.SaveCodexRelation(ctx, projectID, &rels[i])
				}
				jsonResponse(w, http.StatusOK, rels)
				return
			}

			relID := subParts[1]
			if r.Method == http.MethodDelete {
				if err := s.store.DeleteCodexRelation(ctx, projectID, relID); err != nil {
					errorResponse(w, http.StatusInternalServerError, err.Error())
					return
				}
				jsonResponse(w, http.StatusOK, map[string]string{"status": "deleted"})
				return
			}
			errorResponse(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}

		switch r.Method {
		case http.MethodGet:
			entryID := r.URL.Query().Get("entry_id")
			rels, err := s.store.ListCodexRelations(ctx, projectID, entryID)
			if err != nil {
				errorResponse(w, http.StatusInternalServerError, err.Error())
				return
			}
			if rels == nil {
				rels = []domain.EntityRelation{}
			}
			jsonResponse(w, http.StatusOK, rels)
		case http.MethodPost:
			var rel domain.EntityRelation
			if err := json.NewDecoder(r.Body).Decode(&rel); err != nil {
				errorResponse(w, http.StatusBadRequest, "invalid json: "+err.Error())
				return
			}
			// Fail-Closed：关系两端的词条必须归属本作品，禁止跨作品引用。
			for _, eid := range []string{rel.SourceEntryID, rel.TargetEntryID} {
				if eid == "" {
					continue
				}
				owner, err := s.store.CodexEntryProjectID(ctx, eid)
				if err != nil {
					errorResponse(w, http.StatusInternalServerError, err.Error())
					return
				}
				if owner != projectID {
					errorResponse(w, http.StatusBadRequest, "relation references entries outside this project")
					return
				}
			}
			rel.ProjectID = projectID
			if err := s.store.SaveCodexRelation(ctx, projectID, &rel); err != nil {
				errorResponse(w, http.StatusBadRequest, err.Error())
				return
			}
			jsonResponse(w, http.StatusCreated, rel)
		default:
			errorResponse(w, http.StatusMethodNotAllowed, "method not allowed")
		}
		return
	}

	// 4. /api/projects/:id/codex/:entryId
	entryID := sub
	if len(subParts) == 1 {
		switch r.Method {
		case http.MethodGet:
			entry, err := s.store.GetCodexEntry(ctx, projectID, entryID)
			if err != nil {
				errorResponse(w, http.StatusInternalServerError, err.Error())
				return
			}
			if entry == nil {
				errorResponse(w, http.StatusNotFound, "codex entry not found")
				return
			}
			jsonResponse(w, http.StatusOK, entry)
		case http.MethodPut:
			var entry domain.CodexEntry
			if err := json.NewDecoder(r.Body).Decode(&entry); err != nil {
				errorResponse(w, http.StatusBadRequest, "invalid json: "+err.Error())
				return
			}
			// Fail-Closed：词条必须归属本作品，严禁用其它作品的词条 ID 覆盖其数据。
			ownerProjectID, err := s.store.CodexEntryProjectID(ctx, entryID)
			if err != nil {
				errorResponse(w, http.StatusInternalServerError, err.Error())
				return
			}
			if ownerProjectID != "" && ownerProjectID != projectID {
				errorResponse(w, http.StatusConflict, "codex entry belongs to another project")
				return
			}
			entry.ID = entryID
			entry.ProjectID = projectID
			if err := s.store.SaveCodexEntry(ctx, &entry); err != nil {
				errorResponse(w, http.StatusBadRequest, err.Error())
				return
			}
			jsonResponse(w, http.StatusOK, entry)
		case http.MethodDelete:
			if err := s.store.DeleteCodexEntry(ctx, projectID, entryID); err != nil {
				errorResponse(w, http.StatusInternalServerError, err.Error())
				return
			}
			jsonResponse(w, http.StatusOK, map[string]string{"status": "deleted"})
		default:
			errorResponse(w, http.StatusMethodNotAllowed, "method not allowed")
		}
		return
	}

	// 5. /api/projects/:id/codex/:entryId/progressions
	if len(subParts) == 2 && subParts[1] == "progressions" {
		// Fail-Closed：词条必须归属本作品，禁止借道写入其它作品的词条演进。
		owner, err := s.store.CodexEntryProjectID(ctx, entryID)
		if err != nil {
			errorResponse(w, http.StatusInternalServerError, err.Error())
			return
		}
		if owner != projectID {
			errorResponse(w, http.StatusNotFound, "codex entry not found")
			return
		}
		switch r.Method {
		case http.MethodGet:
			progs, err := s.store.ListCodexProgressions(ctx, entryID)
			if err != nil {
				errorResponse(w, http.StatusInternalServerError, err.Error())
				return
			}
			if progs == nil {
				progs = []domain.Progression{}
			}
			jsonResponse(w, http.StatusOK, progs)
		case http.MethodPost:
			var prog domain.Progression
			if err := json.NewDecoder(r.Body).Decode(&prog); err != nil {
				errorResponse(w, http.StatusBadRequest, "invalid json: "+err.Error())
				return
			}
			if err := s.store.SaveCodexProgression(ctx, entryID, &prog); err != nil {
				errorResponse(w, http.StatusBadRequest, err.Error())
				return
			}
			jsonResponse(w, http.StatusCreated, prog)
		default:
			errorResponse(w, http.StatusMethodNotAllowed, "method not allowed")
		}
		return
	}

	errorResponse(w, http.StatusNotFound, "route not found")
}

func appendIntentConstraints(prompt string, intent *domain.ChapterIntent) string {
	if intent == nil {
		return prompt
	}
	var constraints []string
	if strings.TrimSpace(intent.POV) != "" {
		constraints = append(constraints, fmt.Sprintf("视角人物(POV): %s", strings.TrimSpace(intent.POV)))
	}
	if len(intent.EmotionCurve) > 0 {
		constraints = append(constraints, fmt.Sprintf("情绪曲线: %s", strings.Join(intent.EmotionCurve, " -> ")))
	}
	if intent.TensionTarget > 0 {
		constraints = append(constraints, fmt.Sprintf("张力目标: %d/10", intent.TensionTarget))
	}
	if len(intent.HookPlants) > 0 {
		constraints = append(constraints, fmt.Sprintf("埋设伏笔/钩子: %s", strings.Join(intent.HookPlants, "、")))
	}
	if len(intent.HookPayoffs) > 0 {
		constraints = append(constraints, fmt.Sprintf("回收伏笔/钩子: %s", strings.Join(intent.HookPayoffs, "、")))
	}
	if len(constraints) == 0 {
		return prompt
	}
	block := "【章节意图约束】\n" + strings.Join(constraints, "\n")
	if strings.TrimSpace(prompt) == "" {
		return block
	}
	return strings.TrimSpace(prompt) + "\n\n" + block
}

func (s *Server) handleProjectMatrix(w http.ResponseWriter, r *http.Request, projectID string, subParts []string) {
	ctx := r.Context()
	if len(subParts) > 0 && subParts[0] == "ai-generate" {
		if r.Method != http.MethodPost {
			errorResponse(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}
		var req engine.MatrixSceneGenerateRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			errorResponse(w, http.StatusBadRequest, "invalid json: "+err.Error())
			return
		}
		proj, err := s.store.GetProject(ctx, projectID)
		if err != nil || proj == nil {
			errorResponse(w, http.StatusNotFound, "project not found")
			return
		}

		intent, _ := s.store.GetChapterIntent(ctx, projectID, req.ChapterIndex)
		if intent != nil {
			if strings.TrimSpace(req.CoreConflict) == "" && strings.TrimSpace(intent.Conflict) != "" {
				req.CoreConflict = intent.Conflict
			}
			req.Prompt = appendIntentConstraints(req.Prompt, intent)
		}

		if s.orch == nil {
			errorResponse(w, http.StatusInternalServerError, "orchestrator not initialized")
			return
		}

		reasoningModel := ""
		if s.cfg != nil {
			reasoningModel = s.cfg.ReasoningModel
		}

		scenes, _, err := s.orch.GenerateMatrixScenes(ctx, reasoningModel, proj, req)
		if err != nil {
			errorResponse(w, http.StatusInternalServerError, "generate matrix scenes failed: "+err.Error())
			return
		}
		chapterID := ""
		chapters, _ := s.store.ListChapters(ctx, projectID)
		for _, ch := range chapters {
			if ch.ChapterIndex == req.ChapterIndex {
				chapterID = ch.ID
				break
			}
		}

		intentID := ""
		if intent != nil {
			if strings.TrimSpace(intent.ID) != "" {
				intentID = intent.ID
			} else {
				intentID = fmt.Sprintf("intent_%s_%d", projectID, req.ChapterIndex)
			}
		}

		for i := range scenes {
			if chapterID != "" {
				scenes[i].ChapterID = chapterID
			} else {
				scenes[i].ChapterID = fmt.Sprintf("ch_%d", req.ChapterIndex)
			}
			scenes[i].ChapterIntentID = intentID
			_ = s.store.SaveScene(ctx, &scenes[i])
		}
		jsonResponse(w, http.StatusOK, scenes)
		return
	}

	if len(subParts) > 0 && subParts[0] == "generate-from-intents" {
		if r.Method != http.MethodPost {
			errorResponse(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}
		var req struct {
			VolumeIndex int `json:"volume_index"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			errorResponse(w, http.StatusBadRequest, "invalid json: "+err.Error())
			return
		}
		if req.VolumeIndex <= 0 {
			errorResponse(w, http.StatusBadRequest, "volume_index must be positive")
			return
		}
		proj, err := s.store.GetProject(ctx, projectID)
		if err != nil || proj == nil {
			errorResponse(w, http.StatusNotFound, "project not found")
			return
		}

		intents, err := s.store.ListChapterIntents(ctx, projectID, req.VolumeIndex)
		if err != nil {
			errorResponse(w, http.StatusInternalServerError, err.Error())
			return
		}
		if len(intents) == 0 {
			errorResponse(w, http.StatusBadRequest, "该卷暂无意图卡，请先推演")
			return
		}

		chapters, _ := s.store.ListChapters(ctx, projectID)
		chapterIDMap := make(map[int]string)
		for _, ch := range chapters {
			if ch != nil {
				chapterIDMap[ch.ChapterIndex] = ch.ID
			}
		}

		type resultItem struct {
			ChapterIndex  int    `json:"chapter_index"`
			OK            bool   `json:"ok"`
			ScenesCreated int    `json:"scenes_created"`
			Error         string `json:"error"`
		}

		results := make([]resultItem, 0, len(intents))
		totalScenes := 0

		reasoningModel := ""
		if s.cfg != nil {
			reasoningModel = s.cfg.ReasoningModel
		}

		for _, intent := range intents {
			if intent == nil || strings.TrimSpace(intent.Conflict) == "" {
				continue
			}

			prompt := appendIntentConstraints("", intent)
			sceneReq := engine.MatrixSceneGenerateRequest{
				VolumeIndex:  req.VolumeIndex,
				ChapterIndex: intent.ChapterIndex,
				ChapterTitle: fmt.Sprintf("第 %d 章", intent.ChapterIndex),
				CoreConflict: intent.Conflict,
				Prompt:       prompt,
			}

			if s.orch == nil {
				results = append(results, resultItem{
					ChapterIndex:  intent.ChapterIndex,
					OK:            false,
					ScenesCreated: 0,
					Error:         "orchestrator not initialized",
				})
				continue
			}

			scenes, _, genErr := s.orch.GenerateMatrixScenes(ctx, reasoningModel, proj, sceneReq)
			if genErr != nil {
				results = append(results, resultItem{
					ChapterIndex:  intent.ChapterIndex,
					OK:            false,
					ScenesCreated: 0,
					Error:         genErr.Error(),
				})
				continue
			}

			chID := chapterIDMap[intent.ChapterIndex]
			if chID == "" {
				chID = fmt.Sprintf("ch_%d", intent.ChapterIndex)
			}

			intentID := intent.ID
			if strings.TrimSpace(intentID) == "" {
				intentID = fmt.Sprintf("intent_%s_%d", projectID, intent.ChapterIndex)
			}

			for i := range scenes {
				scenes[i].ChapterID = chID
				scenes[i].ChapterIntentID = intentID
				_ = s.store.SaveScene(ctx, &scenes[i])
			}

			totalScenes += len(scenes)
			results = append(results, resultItem{
				ChapterIndex:  intent.ChapterIndex,
				OK:            true,
				ScenesCreated: len(scenes),
				Error:         "",
			})
		}

		jsonResponse(w, http.StatusOK, map[string]any{
			"volume_index": req.VolumeIndex,
			"total_scenes": totalScenes,
			"results":      results,
		})
		return
	}

	if len(subParts) > 0 {
		errorResponse(w, http.StatusNotFound, "route not found")
		return
	}

	if r.Method != http.MethodGet {
		errorResponse(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	overview, err := s.store.GetMatrixOverview(ctx, projectID)
	if err != nil {
		errorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	jsonResponse(w, http.StatusOK, overview)
}

func (s *Server) handleProjectScenes(w http.ResponseWriter, r *http.Request, projectID string, subParts []string) {
	ctx := r.Context()

	// 1. /api/projects/:id/scenes (list or create)
	if len(subParts) == 0 {
		switch r.Method {
		case http.MethodGet:
			chapterID := r.URL.Query().Get("chapter_id")
			var scenes []*domain.Scene
			if chapterID != "" {
				// Fail-Closed：按章节列举场次时，章节若已归档必须归属本作品。
				owner, err := s.store.ChapterProjectID(ctx, chapterID)
				if err != nil {
					errorResponse(w, http.StatusInternalServerError, err.Error())
					return
				}
				if owner != "" && owner != projectID {
					errorResponse(w, http.StatusNotFound, "chapter not found")
					return
				}
				scenes, err = s.store.ListScenes(ctx, chapterID)
				if err != nil {
					errorResponse(w, http.StatusInternalServerError, err.Error())
					return
				}
			} else {
				chapters, err := s.store.ListChapters(ctx, projectID)
				if err != nil {
					errorResponse(w, http.StatusInternalServerError, err.Error())
					return
				}
				for _, ch := range chapters {
					scs, err := s.store.ListScenes(ctx, ch.ID)
					if err == nil {
						scenes = append(scenes, scs...)
					}
				}
			}
			if scenes == nil {
				scenes = []*domain.Scene{}
			}
			jsonResponse(w, http.StatusOK, scenes)
		case http.MethodPost:
			var sc domain.Scene
			if err := json.NewDecoder(r.Body).Decode(&sc); err != nil {
				errorResponse(w, http.StatusBadRequest, "invalid json: "+err.Error())
				return
			}
			sc.ProjectID = projectID
			// Fail-Closed：场次挂载的章节若已归档，必须归属本作品（纯规划章节允许占位挂载）。
			if sc.ChapterID != "" {
				owner, err := s.store.ChapterProjectID(ctx, sc.ChapterID)
				if err != nil {
					errorResponse(w, http.StatusInternalServerError, err.Error())
					return
				}
				if owner != "" && owner != projectID {
					errorResponse(w, http.StatusBadRequest, "scene chapter does not belong to this project")
					return
				}
			}
			if sc.ID == "" {
				sc.ID = fmt.Sprintf("sc-%d", time.Now().UnixNano())
			}
			if err := sc.Validate(); err != nil {
				errorResponse(w, http.StatusBadRequest, err.Error())
				return
			}
			if err := s.store.SaveScene(ctx, &sc); err != nil {
				errorResponse(w, http.StatusBadRequest, err.Error())
				return
			}
			jsonResponse(w, http.StatusCreated, sc)
		default:
			errorResponse(w, http.StatusMethodNotAllowed, "method not allowed")
		}
		return
	}

	// 2. /api/projects/:id/scenes/:sceneId
	sceneID := subParts[0]
	if len(subParts) == 1 {
		switch r.Method {
		case http.MethodGet:
			sc, err := s.store.GetScene(ctx, sceneID)
			if err != nil {
				errorResponse(w, http.StatusInternalServerError, err.Error())
				return
			}
			if sc == nil || sc.ProjectID != projectID {
				errorResponse(w, http.StatusNotFound, "scene not found")
				return
			}
			jsonResponse(w, http.StatusOK, sc)
		case http.MethodPut:
			existing, err := s.store.GetScene(ctx, sceneID)
			if err != nil {
				errorResponse(w, http.StatusInternalServerError, err.Error())
				return
			}
			if existing == nil || existing.ProjectID != projectID {
				errorResponse(w, http.StatusNotFound, "scene not found")
				return
			}
			var sc domain.Scene
			if err := json.NewDecoder(r.Body).Decode(&sc); err != nil {
				errorResponse(w, http.StatusBadRequest, "invalid json: "+err.Error())
				return
			}
			sc.ID = sceneID
			sc.ProjectID = projectID
			if sc.ChapterID != "" {
				owner, err := s.store.ChapterProjectID(ctx, sc.ChapterID)
				if err != nil {
					errorResponse(w, http.StatusInternalServerError, err.Error())
					return
				}
				if owner != "" && owner != projectID {
					errorResponse(w, http.StatusBadRequest, "scene chapter does not belong to this project")
					return
				}
			}
			if err := sc.Validate(); err != nil {
				errorResponse(w, http.StatusBadRequest, err.Error())
				return
			}
			if err := s.store.SaveScene(ctx, &sc); err != nil {
				errorResponse(w, http.StatusBadRequest, err.Error())
				return
			}
			jsonResponse(w, http.StatusOK, sc)
		case http.MethodDelete:
			existing, err := s.store.GetScene(ctx, sceneID)
			if err != nil {
				errorResponse(w, http.StatusInternalServerError, err.Error())
				return
			}
			if existing == nil || existing.ProjectID != projectID {
				errorResponse(w, http.StatusNotFound, "scene not found")
				return
			}
			if err := s.store.DeleteScene(ctx, sceneID); err != nil {
				errorResponse(w, http.StatusInternalServerError, err.Error())
				return
			}
			jsonResponse(w, http.StatusOK, map[string]string{"status": "deleted"})
		default:
			errorResponse(w, http.StatusMethodNotAllowed, "method not allowed")
		}
		return
	}

	// 3. /api/projects/:id/scenes/:sceneId/markers
	if len(subParts) >= 2 && subParts[1] == "markers" {
		if len(subParts) == 2 {
			switch r.Method {
			case http.MethodGet:
				markers, err := s.store.ListSceneMarkers(ctx, sceneID)
				if err != nil {
					errorResponse(w, http.StatusInternalServerError, err.Error())
					return
				}
				if markers == nil {
					markers = []domain.SceneMarker{}
				}
				jsonResponse(w, http.StatusOK, markers)
			case http.MethodPost:
				var m domain.SceneMarker
				if err := json.NewDecoder(r.Body).Decode(&m); err != nil {
					errorResponse(w, http.StatusBadRequest, "invalid json: "+err.Error())
					return
				}
				m.SceneID = sceneID
				if m.ID == "" {
					m.ID = fmt.Sprintf("marker-%d", time.Now().UnixNano())
				}
				if err := s.store.SaveSceneMarker(ctx, &m); err != nil {
					errorResponse(w, http.StatusBadRequest, err.Error())
					return
				}
				jsonResponse(w, http.StatusCreated, m)
			default:
				errorResponse(w, http.StatusMethodNotAllowed, "method not allowed")
			}
			return
		}

		if len(subParts) == 3 && r.Method == http.MethodDelete {
			markerID := subParts[2]
			if err := s.store.DeleteSceneMarker(ctx, markerID); err != nil {
				errorResponse(w, http.StatusInternalServerError, err.Error())
				return
			}
			jsonResponse(w, http.StatusOK, map[string]string{"status": "deleted"})
			return
		}
	}

	errorResponse(w, http.StatusNotFound, "route not found")
}

func (s *Server) handleSceneRoutes(w http.ResponseWriter, r *http.Request) {
	r = r.WithContext(activity.WithSource(r.Context(), activity.SourceUIManual))
	ctx := r.Context()
	rawPath := strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/scenes/"), "/")
	if rawPath == "" {
		errorResponse(w, http.StatusBadRequest, "missing scene id")
		return
	}
	parts := strings.Split(rawPath, "/")
	sceneID := parts[0]

	if len(parts) == 1 {
		switch r.Method {
		case http.MethodGet:
			sc, err := s.store.GetScene(ctx, sceneID)
			if err != nil {
				errorResponse(w, http.StatusInternalServerError, err.Error())
				return
			}
			if sc == nil {
				errorResponse(w, http.StatusNotFound, "scene not found")
				return
			}
			jsonResponse(w, http.StatusOK, sc)
		case http.MethodPut:
			var sc domain.Scene
			if err := json.NewDecoder(r.Body).Decode(&sc); err != nil {
				errorResponse(w, http.StatusBadRequest, "invalid json: "+err.Error())
				return
			}
			sc.ID = sceneID
			if err := sc.Validate(); err != nil {
				errorResponse(w, http.StatusBadRequest, err.Error())
				return
			}
			if err := s.store.SaveScene(ctx, &sc); err != nil {
				errorResponse(w, http.StatusBadRequest, err.Error())
				return
			}
			jsonResponse(w, http.StatusOK, sc)
		case http.MethodDelete:
			if err := s.store.DeleteScene(ctx, sceneID); err != nil {
				errorResponse(w, http.StatusInternalServerError, err.Error())
				return
			}
			jsonResponse(w, http.StatusOK, map[string]string{"status": "deleted"})
		default:
			errorResponse(w, http.StatusMethodNotAllowed, "method not allowed")
		}
		return
	}

	if len(parts) >= 2 && parts[1] == "markers" {
		if len(parts) == 2 {
			switch r.Method {
			case http.MethodGet:
				markers, err := s.store.ListSceneMarkers(ctx, sceneID)
				if err != nil {
					errorResponse(w, http.StatusInternalServerError, err.Error())
					return
				}
				if markers == nil {
					markers = []domain.SceneMarker{}
				}
				jsonResponse(w, http.StatusOK, markers)
			case http.MethodPost:
				var m domain.SceneMarker
				if err := json.NewDecoder(r.Body).Decode(&m); err != nil {
					errorResponse(w, http.StatusBadRequest, "invalid json: "+err.Error())
					return
				}
				m.SceneID = sceneID
				if m.ID == "" {
					m.ID = fmt.Sprintf("marker-%d", time.Now().UnixNano())
				}
				if err := s.store.SaveSceneMarker(ctx, &m); err != nil {
					errorResponse(w, http.StatusBadRequest, err.Error())
					return
				}
				jsonResponse(w, http.StatusCreated, m)
			default:
				errorResponse(w, http.StatusMethodNotAllowed, "method not allowed")
			}
			return
		}
		if len(parts) == 3 && r.Method == http.MethodDelete {
			markerID := parts[2]
			if err := s.store.DeleteSceneMarker(ctx, markerID); err != nil {
				errorResponse(w, http.StatusInternalServerError, err.Error())
				return
			}
			jsonResponse(w, http.StatusOK, map[string]string{"status": "deleted"})
			return
		}
	}

	errorResponse(w, http.StatusNotFound, "route not found")
}

func (s *Server) handleAnalytics(w http.ResponseWriter, r *http.Request, projectID string, subParts []string) {
	if r.Method != http.MethodGet {
		errorResponse(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	if len(subParts) == 0 {
		errorResponse(w, http.StatusBadRequest, "missing analytics type")
		return
	}

	ctx := r.Context()
	action := subParts[0]

	switch action {
	case "heatmap":
		entries, err := s.store.ListCodexEntries(ctx, projectID, "")
		if err != nil {
			errorResponse(w, http.StatusInternalServerError, err.Error())
			return
		}
		chapters, err := s.store.ListChapters(ctx, projectID)
		if err != nil {
			errorResponse(w, http.StatusInternalServerError, err.Error())
			return
		}

		type ChapterMeta struct {
			ChapterIndex int    `json:"chapter_index"`
			Title        string `json:"title"`
			WordCount    int    `json:"word_count"`
		}
		type EntityHeatmapItem struct {
			EntryID       string               `json:"entry_id"`
			Name          string               `json:"name"`
			Category      domain.CodexCategory `json:"category"`
			ColorTag      string               `json:"color_tag"`
			TotalMentions int                  `json:"total_mentions"`
			ChapterCounts map[int]int          `json:"chapter_counts"`
		}

		var chapMetas []ChapterMeta
		for _, ch := range chapters {
			chapMetas = append(chapMetas, ChapterMeta{
				ChapterIndex: ch.ChapterIndex,
				Title:        ch.Title,
				WordCount:    ch.WordCount,
			})
		}

		scanner := engine.NewMentionScanner()
		itemsMap := make(map[string]*EntityHeatmapItem)
		for _, ent := range entries {
			itemsMap[ent.ID] = &EntityHeatmapItem{
				EntryID:       ent.ID,
				Name:          ent.Name,
				Category:      ent.Category,
				ColorTag:      ent.ColorTag,
				ChapterCounts: make(map[int]int),
			}
		}

		for _, ch := range chapters {
			_, mentions := scanner.ScanText(ch.Content, entries)
			for _, m := range mentions {
				if item, ok := itemsMap[m.Entry.ID]; ok {
					item.ChapterCounts[ch.ChapterIndex]++
					item.TotalMentions++
				}
			}
		}

		var heatmapItems []EntityHeatmapItem
		for _, ent := range entries {
			if item, ok := itemsMap[ent.ID]; ok {
				heatmapItems = append(heatmapItems, *item)
			}
		}

		jsonResponse(w, http.StatusOK, map[string]interface{}{
			"chapters": chapMetas,
			"entities": heatmapItems,
		})

	case "tension":
		chapters, err := s.store.ListChapters(ctx, projectID)
		if err != nil {
			errorResponse(w, http.StatusInternalServerError, err.Error())
			return
		}

		type TensionPoint struct {
			ChapterIndex int    `json:"chapter_index"`
			ChapterTitle string `json:"chapter_title"`
			SceneIndex   int    `json:"scene_index"`
			SceneTitle   string `json:"scene_title"`
			TensionLevel int    `json:"tension_level"`
			DramaticGoal string `json:"dramatic_goal"`
			WordCount    int    `json:"word_count"`
		}

		var points []TensionPoint
		totalTension := 0

		for _, ch := range chapters {
			scenes, err := s.store.ListScenes(ctx, ch.ID)
			if err == nil && len(scenes) > 0 {
				for _, sc := range scenes {
					points = append(points, TensionPoint{
						ChapterIndex: ch.ChapterIndex,
						ChapterTitle: ch.Title,
						SceneIndex:   sc.SceneIndex,
						SceneTitle:   sc.Title,
						TensionLevel: sc.TensionLevel,
						DramaticGoal: sc.DramaticGoal,
						WordCount:    sc.WordCount,
					})
					totalTension += sc.TensionLevel
				}
			} else {
				// Fallback to chapter row if no atomic scenes yet
				points = append(points, TensionPoint{
					ChapterIndex: ch.ChapterIndex,
					ChapterTitle: ch.Title,
					SceneIndex:   1,
					SceneTitle:   ch.Title,
					TensionLevel: 5,
					DramaticGoal: "推进剧情",
					WordCount:    ch.WordCount,
				})
				totalTension += 5
			}
		}

		avgTension := 0.0
		if len(points) > 0 {
			avgTension = float64(totalTension) / float64(len(points))
		}

		jsonResponse(w, http.StatusOK, map[string]interface{}{
			"points":      points,
			"avg_tension": avgTension,
		})

	default:
		errorResponse(w, http.StatusBadRequest, "unsupported analytics type")
	}
}
