package main

import (
	"bytes"
	"strings"
	"testing"

	"github.com/Jungley8/novel-studio/internal/domain"
	"github.com/Jungley8/novel-studio/internal/engine"
)

func TestVersion(t *testing.T) {
	if version == "" {
		t.Fatal("version should not be empty")
	}
}

func TestPrintProduceSummary_NeedsHumanExit2(t *testing.T) {
	var stdout, stderr bytes.Buffer
	res := &engine.WorkshopProduceResult{
		RewriteLoops: 3,
		Committed:    false,
		Audit: &domain.AuditReport{
			Verdict:    domain.ReviewVerdictRevision,
			Score:      55,
			NeedsHuman: true,
			Issues: []string{
				"第1点：战力崩塌，主角越级过大",
				"第2点：人设跳戏，反派态度突变",
				"第3点：节奏拖沓，对话占比过高",
				"第4点：伏笔回收证据不足",
			},
		},
	}

	code := printProduceSummary(&stdout, &stderr, res)
	if code != 2 {
		t.Fatalf("expected exit code 2 on NeedsHuman, got %d", code)
	}

	if stdout.Len() != 0 {
		t.Errorf("expected empty stdout on halt, got: %s", stdout.String())
	}

	errStr := stderr.String()
	lines := strings.Split(strings.TrimSpace(errStr), "\n")
	// Expected 5 lines: 1 for verdict/score, 1 for loops, 3 for top 3 issues
	if len(lines) != 5 {
		t.Errorf("expected 5 lines in stderr, got %d:\n%s", len(lines), errStr)
	}

	if !strings.Contains(lines[0], "终审裁决: REVISION_NEEDED (综合评分: 55/100)") {
		t.Errorf("line 0 should contain verdict and score, got: %s", lines[0])
	}
	if !strings.Contains(lines[1], "返工轮次: 3 次") {
		t.Errorf("line 1 should contain rewrite loops, got: %s", lines[1])
	}
	if !strings.Contains(lines[2], "第1点：战力崩塌") || !strings.Contains(lines[3], "第2点：人设跳戏") || !strings.Contains(lines[4], "第3点：节奏拖沓") {
		t.Errorf("expected top 3 issues in lines 2..4, got:\n%s", errStr)
	}
	if strings.Contains(errStr, "第4点") {
		t.Errorf("should only print top 3 issues, got: %s", errStr)
	}
}

func TestPrintProduceSummary_SuccessExit0(t *testing.T) {
	var stdout, stderr bytes.Buffer
	res := &engine.WorkshopProduceResult{
		Content:      "这是成功的章节正文内容，已顺利通过主编审核。",
		RewriteLoops: 1,
		Committed:    true,
		Audit: &domain.AuditReport{
			Verdict:         domain.ReviewVerdictAccepted,
			Score:           92,
			BurstinessScore: 78,
			NeedsHuman:      false,
			Issues:          []string{"微小标点瑕疵"},
			ResolvedHookIDs: []string{"hook-1"},
		},
	}

	code := printProduceSummary(&stdout, &stderr, res)
	if code != 0 {
		t.Fatalf("expected exit code 0 on success, got %d", code)
	}
	if stderr.Len() != 0 {
		t.Errorf("expected empty stderr on success, got: %s", stderr.String())
	}

	outStr := stdout.String()
	if !strings.Contains(outStr, "✓ 生产完成！") {
		t.Errorf("expected success header in stdout, got: %s", outStr)
	}
	if !strings.Contains(outStr, "已原子归档写入 SQLite") {
		t.Errorf("expected committed text in stdout, got: %s", outStr)
	}
}

func TestPrintProduceSummary_ManualUncommittedExit0(t *testing.T) {
	var stdout, stderr bytes.Buffer
	res := &engine.WorkshopProduceResult{
		Content:      "章节正文内容，未开启自动归档。",
		RewriteLoops: 0,
		Committed:    false,
		Audit: &domain.AuditReport{
			Verdict:    domain.ReviewVerdictAccepted,
			Score:      88,
			NeedsHuman: false,
		},
	}

	code := printProduceSummary(&stdout, &stderr, res)
	if code != 0 {
		t.Fatalf("expected exit code 0 on manual uncommitted, got %d", code)
	}
	if stderr.Len() != 0 {
		t.Errorf("expected empty stderr, got: %s", stderr.String())
	}

	outStr := stdout.String()
	if !strings.Contains(outStr, "未归档 (可手动审核后归档)") {
		t.Errorf("expected uncommitted notice in stdout, got: %s", outStr)
	}
}

func TestPrintProduceSummary_NilAuditDefensive(t *testing.T) {
	var stdout, stderr bytes.Buffer
	res := &engine.WorkshopProduceResult{
		Content:      "测试空指针安全性正文",
		RewriteLoops: 0,
		Committed:    false,
		Audit:        nil,
	}

	// Defensive check: res.Audit == nil must not panic and must return 0
	code := printProduceSummary(&stdout, &stderr, res)
	if code != 0 {
		t.Fatalf("expected exit code 0 on nil audit, got %d", code)
	}
	if stderr.Len() != 0 {
		t.Errorf("expected empty stderr, got: %s", stderr.String())
	}
	outStr := stdout.String()
	if !strings.Contains(outStr, "✓ 生产完成！") {
		t.Errorf("expected success header in stdout, got: %s", outStr)
	}
}

func TestResolveServerPort(t *testing.T) {
	tests := []struct {
		name     string
		portFlag int
		cfgPort  int
		isDev    bool
		expected int
	}{
		{
			name:     "CLI flag explicitly specified in prod",
			portFlag: 30000,
			cfgPort:  28980,
			isDev:    false,
			expected: 30000,
		},
		{
			name:     "CLI flag explicitly specified in dev",
			portFlag: 30000,
			cfgPort:  28980,
			isDev:    true,
			expected: 30000,
		},
		{
			name:     "Dev mode with default cfgPort (28980) uses isolated dev port 28981",
			portFlag: 0,
			cfgPort:  28980,
			isDev:    true,
			expected: 28981,
		},
		{
			name:     "Dev mode with empty cfgPort (0) uses isolated dev port 28981",
			portFlag: 0,
			cfgPort:  0,
			isDev:    true,
			expected: 28981,
		},
		{
			name:     "Dev mode with custom cfgPort (29999) respects custom port",
			portFlag: 0,
			cfgPort:  29999,
			isDev:    true,
			expected: 29999,
		},
		{
			name:     "Prod mode with default cfgPort (28980) uses 28980",
			portFlag: 0,
			cfgPort:  28980,
			isDev:    false,
			expected: 28980,
		},
		{
			name:     "Prod mode with custom cfgPort (30001) uses 30001",
			portFlag: 0,
			cfgPort:  30001,
			isDev:    false,
			expected: 30001,
		},
		{
			name:     "Prod mode with empty cfgPort (0) falls back to default 28980",
			portFlag: 0,
			cfgPort:  0,
			isDev:    false,
			expected: 28980,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actual := resolveServerPort(tt.portFlag, tt.cfgPort, tt.isDev)
			if actual != tt.expected {
				t.Errorf("resolveServerPort(%d, %d, %v) = %d, expected %d",
					tt.portFlag, tt.cfgPort, tt.isDev, actual, tt.expected)
			}
		})
	}
}
