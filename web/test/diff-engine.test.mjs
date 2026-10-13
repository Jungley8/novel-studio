import test from 'node:test';
import assert from 'node:assert/strict';
import { computeWordDiff, computeLineDiff, computeDiffStats } from '../src/utils/diff.js';

test('Diff Engine - computeWordDiff: 细粒度词级与字符级差异计算', () => {
  const oldText = '林天傲冷笑着拔出长剑，指向林尘。';
  const newText = '林天傲面色惨白地拔出断剑，颤抖着指向林尘。';

  const tokens = computeWordDiff(oldText, newText);
  assert.ok(tokens.length > 0, 'tokens should not be empty');

  // Should have deletions (冷笑, 长剑) and additions (面色惨白, 断剑, 颤抖着)
  const delTokens = tokens.filter(t => t.type === 'del');
  const addTokens = tokens.filter(t => t.type === 'add');
  const eqTokens = tokens.filter(t => t.type === 'eq');

  assert.ok(delTokens.length > 0, 'should contain deleted tokens');
  assert.ok(addTokens.length > 0, 'should contain added tokens');
  assert.ok(eqTokens.length > 0, 'should contain equal tokens');

  // Rebuilding original from eq + del matches oldText
  const reconstructedOld = tokens.filter(t => t.type !== 'add').map(t => t.text).join('');
  assert.equal(reconstructedOld, oldText);

  // Rebuilding new from eq + add matches newText
  const reconstructedNew = tokens.filter(t => t.type !== 'del').map(t => t.text).join('');
  assert.equal(reconstructedNew, newText);
});

test('Diff Engine - computeLineDiff: 行级 LCS 与行号对齐', () => {
  const oldText = '第一行内容\n第二行被删除\n第三行未修改';
  const newText = '第一行内容\n第二行新插入\n第三行未修改\n第四行新追加';

  const lines = computeLineDiff(oldText, newText);
  assert.ok(lines.length >= 4);

  const hasAdd = lines.some(l => l.type === 'add');
  const hasDel = lines.some(l => l.type === 'del');
  const hasEq = lines.some(l => l.type === 'eq');

  assert.ok(hasAdd, 'should contain added line');
  assert.ok(hasDel, 'should contain deleted line');
  assert.ok(hasEq, 'should contain equal line');
});

test('Diff Engine - computeDiffStats: 字数与增减统计', () => {
  const oldText = '天地不仁，以万物为刍狗。';
  const newText = '天地不仁，以万物为刍狗。圣人不仁，以百姓为刍狗。';

  const stats = computeDiffStats(oldText, newText);
  assert.equal(stats.oldWordCount, 12);
  assert.equal(stats.newWordCount, 24);
  assert.equal(stats.netDiff, 12);
});
