import test from 'node:test';
import assert from 'node:assert/strict';
import { parseInventoryItems, simulateInventoryDelta } from '../src/utils/inventory.js';

test('Inventory Engine - parseInventoryItems: 解析多道具与数量', () => {
  assert.deepEqual(parseInventoryItems(''), []);
  assert.deepEqual(parseInventoryItems('无'), []);
  assert.deepEqual(parseInventoryItems('暂无'), []);
  assert.deepEqual(parseInventoryItems('待定'), []);

  const items = parseInventoryItems('劣质的定情玉佩, 洗髓丹x2, 辟毒珠*3、断剑');
  assert.equal(items.length, 4);
  assert.deepEqual(items[0], { name: '劣质的定情玉佩', qty: 1, raw: '劣质的定情玉佩' });
  assert.deepEqual(items[1], { name: '洗髓丹', qty: 2, raw: '洗髓丹x2' });
  assert.deepEqual(items[2], { name: '辟毒珠', qty: 3, raw: '辟毒珠*3' });
  assert.deepEqual(items[3], { name: '断剑', qty: 1, raw: '断剑' });

  // 包含长实体名称、书名号与括号描述
  const complex = parseInventoryItems('林天傲画押的《弟子引资入局协议》霸王条款副本, 劣质的定情玉佩（作为物证提交天道销毁）x1');
  assert.equal(complex.length, 2);
  assert.equal(complex[0].name, '林天傲画押的《弟子引资入局协议》霸王条款副本');
  assert.equal(complex[0].qty, 1);
  assert.equal(complex[1].name, '劣质的定情玉佩（作为物证提交天道销毁）');
  assert.equal(complex[1].qty, 1);
});

test('Inventory Engine - simulateInventoryDelta: 增删推演与徽章标记', () => {
  const current = '劣质的定情玉佩, 洗髓丹x2';
  
  // 空变动
  const simNoDelta = simulateInventoryDelta(current, '');
  assert.equal(simNoDelta.currentList.length, 2);
  assert.equal(simNoDelta.predictedList.length, 2);
  assert.equal(simNoDelta.removedList.length, 0);
  assert.equal(simNoDelta.predictedString, '劣质的定情玉佩, 洗髓丹x2');

  // 获得新道具与消耗现有道具
  const simChange = simulateInventoryDelta(current, '+辟毒珠, -洗髓丹x1, 获得玄铁匕首');
  assert.equal(simChange.currentList.length, 2);
  
  // 预测列表：劣质的定情玉佩 (旧), 洗髓丹 (变为1), 辟毒珠 (新增), 玄铁匕首 (新增)
  const names = simChange.predictedList.map(i => i.name);
  assert.ok(names.includes('劣质的定情玉佩'));
  assert.ok(names.includes('洗髓丹'));
  assert.ok(names.includes('辟毒珠'));
  assert.ok(names.includes('玄铁匕首'));

  const biDuZhu = simChange.predictedList.find(i => i.name === '辟毒珠');
  assert.ok(biDuZhu.isNew);
  assert.equal(biDuZhu.qty, 1);

  const xiSuiDan = simChange.predictedList.find(i => i.name === '洗髓丹');
  assert.equal(xiSuiDan.isNew, false);
  assert.equal(xiSuiDan.qty, 1);

  assert.equal(simChange.removedList.length, 1);
  assert.equal(simChange.removedList[0].name, '洗髓丹');
  assert.equal(simChange.removedList[0].qty, 1);

  assert.ok(simChange.deltaSummary.includes('+2件 获得'));
  assert.ok(simChange.deltaSummary.includes('-1件 消耗'));
});

test('Inventory Engine - simulateInventoryDelta: 尝试移除未持有物品时记录在 removedList 中供告警提醒', () => {
  const current = '洗髓丹x2';
  const sim = simulateInventoryDelta(current, '-不存在的古铜镜');
  assert.equal(sim.removedList.length, 1);
  assert.equal(sim.removedList[0].name, '不存在的古铜镜');
});
