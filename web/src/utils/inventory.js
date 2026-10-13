/**
 * 随身底牌与道具 (Inventory) 解析与状态机推演工具
 */

const STOP_WORDS = new Set(['无', '暂无', '待定', '随身物品待起草', 'none', 'null', 'undefined']);

const ADD_PREFIXES = [
  '获得', '得到', '拾取', '买下', '购得', '炼成', '收服', '捡到', '缴获', '新增', '增加', '拿取', '执掌', '炼制'
];

const REMOVE_PREFIXES = [
  '消耗', '失去', '损毁', '碎裂', '丢弃', '销毁', '用尽', '破碎', '献祭', '交出', '扣除', '移除', '吞服', '服下', '耗尽', '上交'
];

/**
 * 解析库存字符串为结构化道具条目数组
 * @param {string} str 例如："劣质的定情玉佩, 洗髓丹x2, 辟毒珠"
 * @returns {Array<{ name: string, qty: number, raw: string }>}
 */
export function parseInventoryItems(str) {
  if (!str || typeof str !== 'string') return [];
  const trimmed = str.trim();
  if (!trimmed || STOP_WORDS.has(trimmed.toLowerCase())) return [];

  return trimmed
    .split(/[,，、;\n]+/)
    .map(s => s.trim())
    .filter(Boolean)
    .map(s => {
      // 匹配数量后缀：x2, X2, *2, x 3 等
      const m = s.match(/^(.*?)(?:x|X|\*)\s*(\d+)$/);
      if (m && m[1].trim()) {
        const name = m[1].trim();
        const qty = parseInt(m[2], 10) || 1;
        return { name, qty, raw: s };
      }
      return { name: s, qty: 1, raw: s };
    })
    .filter(i => i.name && !STOP_WORDS.has(i.name.toLowerCase()));
}

/**
 * 实时推演定稿后的随身底牌状态机
 * @param {string} currentInventoryText 当前主角随身道具文本
 * @param {string} deltaText 本章物品变动文本 (例如: "+洗髓丹x2, -辟毒珠")
 */
export function simulateInventoryDelta(currentInventoryText, deltaText) {
  const currentList = parseInventoryItems(currentInventoryText);
  const delta = (deltaText || '').trim();

  const itemMap = new Map();
  for (const it of currentList) {
    itemMap.set(it.name, (itemMap.get(it.name) || 0) + it.qty);
  }
  const initialMap = new Map(itemMap);

  if (!delta) {
    const list = currentList.map(it => ({ ...it, isNew: false, isIncreased: false }));
    return {
      currentList,
      predictedList: list,
      removedList: [],
      deltaSummary: '',
      predictedString: list.length > 0 ? list.map(i => i.qty > 1 ? `${i.name}x${i.qty}` : i.name).join(', ') : '无',
    };
  }

  const tokens = delta
    .replace(/\s+([+\-])/g, ',$1')
    .split(/[,，、;\n]+/)
    .map(t => t.trim())
    .filter(Boolean);

  const removedList = [];
  let addedCount = 0;
  let removedCount = 0;

  for (const rawToken of tokens) {
    let action = 'ADD';
    let token = rawToken;

    if (token.startsWith('+')) {
      action = 'ADD';
      token = token.slice(1).trim();
    } else if (token.startsWith('-')) {
      action = 'REMOVE';
      token = token.slice(1).trim();
    } else {
      let matched = false;
      for (const p of REMOVE_PREFIXES) {
        if (token.startsWith(p)) {
          action = 'REMOVE';
          token = token.slice(p.length).trim();
          matched = true;
          break;
        }
      }
      if (!matched) {
        for (const p of ADD_PREFIXES) {
          if (token.startsWith(p)) {
            action = 'ADD';
            token = token.slice(p.length).trim();
            matched = true;
            break;
          }
        }
      }
    }

    if (!token || STOP_WORDS.has(token.toLowerCase())) continue;

    let qty = 1;
    const qm = token.match(/^(.*?)(?:x|X|\*)\s*(\d+)$/);
    let itemName = token;
    if (qm && qm[1].trim()) {
      itemName = qm[1].trim();
      qty = parseInt(qm[2], 10) || 1;
    }

    let foundKey = null;
    if (itemMap.has(itemName)) {
      foundKey = itemName;
    } else {
      for (const [k] of itemMap) {
        if (k.includes(itemName) || itemName.includes(k)) {
          foundKey = k;
          break;
        }
      }
    }

    if (action === 'ADD') {
      const key = foundKey || itemName;
      itemMap.set(key, (itemMap.get(key) || 0) + qty);
      addedCount += qty;
    } else if (action === 'REMOVE') {
      removedCount += qty;
      if (foundKey) {
        const cur = itemMap.get(foundKey);
        if (cur > qty) {
          itemMap.set(foundKey, cur - qty);
        } else {
          itemMap.delete(foundKey);
        }
        removedList.push({ name: foundKey, qty });
      } else {
        // 尝试移除当前未持有的物品（如正文提到的旧物）
        removedList.push({ name: itemName, qty });
      }
    }
  }

  const predictedList = [];
  for (const [name, qty] of itemMap) {
    if (qty <= 0) continue;
    const origQty = initialMap.get(name) || 0;
    const isNew = origQty === 0;
    const isIncreased = origQty > 0 && qty > origQty;
    predictedList.push({ name, qty, isNew, isIncreased });
  }

  const summaryParts = [];
  if (addedCount > 0) summaryParts.push(`+${addedCount}件 获得`);
  if (removedCount > 0) summaryParts.push(`-${removedCount}件 消耗`);

  const predictedString = predictedList.length > 0 
    ? predictedList.map(i => i.qty > 1 ? `${i.name}x${i.qty}` : i.name).join(', ') 
    : '无';

  return {
    currentList,
    predictedList,
    removedList,
    deltaSummary: summaryParts.join(' / '),
    predictedString,
  };
}
