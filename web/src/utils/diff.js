// web/src/utils/diff.js
// 工业级轻量文本 Diff 算法引擎，支持行级与字符级/词级细粒度比对 (VSCode 风格)

/**
 * 将文本拆分为字符/词元
 * 中文按字拆分，英文/数字按单词拆分，保留标点与空白
 */
export function tokenizeWords(text) {
  if (!text) return [];
  const tokens = [];
  const regex = /([\u4e00-\u9fa5]|[\w]+|[^\s\w\u4e00-\u9fa5]+|\s+)/g;
  let match;
  while ((match = regex.exec(text)) !== null) {
    tokens.push(match[0]);
  }
  return tokens;
}

/**
 * 字符/词级 LCS 细粒度 Diff
 * 返回数组: [{ kind: 'same' | 'add' | 'del', text: string }]
 */
export function computeWordDiff(oldText, newText) {
  if (oldText === newText) {
    return [{ kind: 'same', type: 'eq', text: oldText || '' }];
  }
  if (!oldText) {
    return [{ kind: 'add', type: 'add', text: newText || '' }];
  }
  if (!newText) {
    return [{ kind: 'del', type: 'del', text: oldText || '' }];
  }

  const a = tokenizeWords(oldText);
  const b = tokenizeWords(newText);

  // 公共前缀优化
  let pre = 0;
  while (pre < a.length && pre < b.length && a[pre] === b[pre]) pre++;

  // 公共后缀优化
  let suf = 0;
  while (suf < a.length - pre && suf < b.length - pre && a[a.length - 1 - suf] === b[b.length - 1 - suf]) suf++;

  const chunks = [];
  if (pre > 0) {
    chunks.push({ kind: 'same', text: a.slice(0, pre).join('') });
  }

  const ma = a.slice(pre, a.length - suf);
  const mb = b.slice(pre, b.length - suf);

  if (ma.length > 0 && mb.length === 0) {
    chunks.push({ kind: 'del', text: ma.join('') });
  } else if (ma.length === 0 && mb.length > 0) {
    chunks.push({ kind: 'add', text: mb.join('') });
  } else if (ma.length > 0 && mb.length > 0) {
    if (ma.length * mb.length > 25000) {
      // 保护降级：避免超长文本 O(N*M) 卡顿
      chunks.push({ kind: 'del', text: ma.join('') });
      chunks.push({ kind: 'add', text: mb.join('') });
    } else {
      const m = ma.length;
      const n = mb.length;
      const dp = Array.from({ length: m + 1 }, () => new Uint16Array(n + 1));
      for (let i = m - 1; i >= 0; i--) {
        for (let j = n - 1; j >= 0; j--) {
          dp[i][j] = ma[i] === mb[j] ? dp[i + 1][j + 1] + 1 : Math.max(dp[i + 1][j], dp[i][j + 1]);
        }
      }

      let i = 0;
      let j = 0;
      while (i < m && j < n) {
        if (ma[i] === mb[j]) {
          chunks.push({ kind: 'same', text: ma[i] });
          i++;
          j++;
        } else if (dp[i + 1][j] >= dp[i][j + 1]) {
          chunks.push({ kind: 'del', text: ma[i] });
          i++;
        } else {
          chunks.push({ kind: 'add', text: mb[j] });
          j++;
        }
      }
      while (i < m) chunks.push({ kind: 'del', text: ma[i++] });
      while (j < n) chunks.push({ kind: 'add', text: mb[j++] });
    }
  }

  if (suf > 0) {
    chunks.push({ kind: 'same', text: a.slice(a.length - suf).join('') });
  }

  // 合并相同 kind 的邻近碎片
  const merged = [];
  for (const c of chunks) {
    if (!c.text) continue;
    if (merged.length > 0 && merged[merged.length - 1].kind === c.kind) {
      merged[merged.length - 1].text += c.text;
    } else {
      merged.push({ ...c, type: c.kind === 'same' ? 'eq' : c.kind });
    }
  }
  return merged;
}

/**
 * 行级 LCS Diff (带有行号与单行内词级高亮标记)
 * 返回数组: [{
 *   kind: 'same' | 'add' | 'del' | 'mod',
 *   oldLineNum: number | null,
 *   newLineNum: number | null,
 *   oldText: string,
 *   newText: string,
 *   wordDiff: Array
 * }]
 */
export function computeLineDiff(oldText, newText) {
  const aLines = (oldText || '').split('\n');
  const bLines = (newText || '').split('\n');

  let pre = 0;
  while (pre < aLines.length && pre < bLines.length && aLines[pre] === bLines[pre]) pre++;

  let suf = 0;
  while (suf < aLines.length - pre && suf < bLines.length - pre && aLines[aLines.length - 1 - suf] === bLines[bLines.length - 1 - suf]) suf++;

  const rows = [];
  let oldNum = 1;
  let newNum = 1;

  for (let i = 0; i < pre; i++) {
    rows.push({
      kind: 'same',
      oldLineNum: oldNum++,
      newLineNum: newNum++,
      text: aLines[i],
      oldText: aLines[i],
      newText: aLines[i],
    });
  }

  const ma = aLines.slice(pre, aLines.length - suf);
  const mb = bLines.slice(pre, bLines.length - suf);

  if (ma.length * mb.length > 80000) {
    for (const t of ma) {
      rows.push({ kind: 'del', oldLineNum: oldNum++, newLineNum: null, text: t, oldText: t, newText: '' });
    }
    for (const t of mb) {
      rows.push({ kind: 'add', oldLineNum: null, newLineNum: newNum++, text: t, oldText: '', newText: t });
    }
  } else {
    const m = ma.length;
    const n = mb.length;
    const dp = Array.from({ length: m + 1 }, () => new Int32Array(n + 1));
    for (let i = m - 1; i >= 0; i--) {
      for (let j = n - 1; j >= 0; j--) {
        dp[i][j] = ma[i] === mb[j] ? dp[i + 1][j + 1] + 1 : Math.max(dp[i + 1][j], dp[i][j + 1]);
      }
    }

    let i = 0;
    let j = 0;
    while (i < m && j < n) {
      if (ma[i] === mb[j]) {
        rows.push({
          kind: 'same',
          oldLineNum: oldNum++,
          newLineNum: newNum++,
          text: ma[i],
          oldText: ma[i],
          newText: ma[i],
        });
        i++;
        j++;
      } else if (dp[i + 1][j] >= dp[i][j + 1]) {
        rows.push({
          kind: 'del',
          oldLineNum: oldNum++,
          newLineNum: null,
          text: ma[i],
          oldText: ma[i],
          newText: '',
        });
        i++;
      } else {
        rows.push({
          kind: 'add',
          oldLineNum: null,
          newLineNum: newNum++,
          text: mb[j],
          oldText: '',
          newText: mb[j],
        });
        j++;
      }
    }
    while (i < m) {
      rows.push({
        kind: 'del',
        oldLineNum: oldNum++,
        newLineNum: null,
        text: ma[i++],
        oldText: ma[i - 1],
        newText: '',
      });
    }
    while (j < n) {
      rows.push({
        kind: 'add',
        oldLineNum: null,
        newLineNum: newNum++,
        text: mb[j++],
        oldText: '',
        newText: mb[j - 1],
      });
    }
  }

  for (let k = aLines.length - suf; k < aLines.length; k++) {
    rows.push({
      kind: 'same',
      oldLineNum: oldNum++,
      newLineNum: newNum++,
      text: aLines[k],
      oldText: aLines[k],
      newText: aLines[k],
    });
  }

  // 二次优化：将相邻紧挨的 del 与 add 行标记为配对 mod 行，并执行单行内词级对比
  for (let idx = 0; idx < rows.length; idx++) {
    if (rows[idx].kind === 'del' && idx + 1 < rows.length && rows[idx + 1].kind === 'add') {
      const delRow = rows[idx];
      const addRow = rows[idx + 1];
      const wordDiff = computeWordDiff(delRow.text, addRow.text);
      delRow.wordDiff = wordDiff.filter(w => w.kind !== 'add');
      addRow.wordDiff = wordDiff.filter(w => w.kind !== 'del');
    }
  }

  for (const r of rows) {
    r.type = r.kind === 'same' ? 'eq' : r.kind;
  }

  return rows;
}

/**
 * 计算 Diff 统计数据
 */
export function computeDiffStats(oldText, newText, rows) {
  const oldWords = (oldText || '').replace(/\s+/g, '').length;
  const newWords = (newText || '').replace(/\s+/g, '').length;
  const diffRows = rows || computeLineDiff(oldText, newText);

  let addedLines = 0;
  let deletedLines = 0;
  let modifiedChunks = 0;

  for (const r of diffRows) {
    if (r.kind === 'add' || r.type === 'add') addedLines++;
    else if (r.kind === 'del' || r.type === 'del') deletedLines++;
  }

  return {
    oldWords,
    oldWordCount: oldWords,
    newWords,
    newWordCount: newWords,
    deltaWords: newWords - oldWords,
    netDiff: newWords - oldWords,
    addedLines,
    deletedLines,
    changedLines: addedLines + deletedLines,
    modifiedChunks,
  };
}
