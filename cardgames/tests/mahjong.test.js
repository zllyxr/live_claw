import test from 'node:test';
import assert from 'node:assert/strict';
import {
  freshWall, toCounts, tileLabel, tileNumber, tileSuit, isWind,
  isStandardWin, isSevenPairs, canWin, scoreWin,
  canPeng, canGangFromDiscard, findConcealedGangs, waitingTiles, shantenEstimate,
  canWinRedCenter, waitingTilesRedCenter, scoreRedCenterWin, RED_CENTER_TILE
} from '../src/mahjong/rules.js';
import { createGame, viewFor } from '../src/mahjong/game.js';

// 便捷构造：w=万(0..8) t=条(9..17) d=筒(18..26) f=风(27..33)
const w = (n) => n - 1;
const t = (n) => 9 + n - 1;
const d = (n) => 18 + n - 1;
const F = { 东: 27, 南: 28, 西: 29, 北: 30, 中: 31, 发: 32, 白: 33 };

test('牌墙：136 张，34 种各 4 张', () => {
  const wall = freshWall();
  assert.equal(wall.length, 136);
  const counts = toCounts(wall);
  assert.equal(counts.length, 34);
  assert.ok(counts.every((n) => n === 4), '每种牌应为 4 张');
});

test('牌面属性与名称', () => {
  assert.equal(tileNumber(w(5)), 5);
  assert.equal(tileSuit(w(5)), 0);
  assert.equal(tileSuit(t(5)), 1);
  assert.equal(tileSuit(d(5)), 2);
  assert.equal(isWind(F.东), true);
  assert.equal(tileNumber(F.东), 0, '风牌无数字');
  assert.equal(tileLabel(w(3)), '3万');
  assert.equal(tileLabel(t(7)), '7条');
  assert.equal(tileLabel(F.发), '发');
});

test('标准型胡牌：四顺子 + 将', () => {
  // 123万 456万 789万 123条 + 55筒
  const hand = [w(1), w(2), w(3), w(4), w(5), w(6), w(7), w(8), w(9),
    t(1), t(2), t(3), d(5), d(5)];
  assert.equal(hand.length, 14);
  assert.equal(isStandardWin(hand), true);
  assert.equal(canWin(hand), true);
});

test('标准型胡牌：刻子组合', () => {
  // 111万 222万 333条 444筒 + 东东
  const hand = [w(1), w(1), w(1), w(2), w(2), w(2), t(3), t(3), t(3),
    d(4), d(4), d(4), F.东, F.东];
  assert.equal(isStandardWin(hand), true);
});

test('未成胡的牌型应判否', () => {
  // 少一张搭不成
  const hand = [w(1), w(2), w(4), w(5), w(7), w(9), t(1), t(3), t(5),
    d(2), d(4), d(6), F.东, F.南];
  assert.equal(canWin(hand), false);
});

test('顺子不能跨花色边界', () => {
  // 8万9万 + 1条 不构成顺子：拿它当面子应失败
  const hand = [w(8), w(9), t(1), w(1), w(1), w(1), t(3), t(3), t(3),
    d(4), d(4), d(4), F.东, F.东];
  assert.equal(isStandardWin(hand), false, '8万9万1条不能算顺子');
});

test('七小对判定', () => {
  const hand = [w(1), w(1), w(3), w(3), t(2), t(2), t(5), t(5),
    d(4), d(4), d(7), d(7), F.中, F.中];
  assert.equal(hand.length, 14);
  assert.equal(isSevenPairs(hand), true);
  assert.equal(canWin(hand), true);

  // 四张同牌算两对，仍成立
  const withQuad = [w(1), w(1), w(1), w(1), w(3), w(3), t(2), t(2),
    t(5), t(5), d(4), d(4), F.中, F.中];
  assert.equal(isSevenPairs(withQuad), true);

  // 有三张的不成七对
  const triple = [w(1), w(1), w(1), w(3), w(3), t(2), t(2), t(5), t(5),
    d(4), d(4), d(7), d(7), F.中];
  assert.equal(isSevenPairs(triple), false);

  // 有副露不能七对
  assert.equal(isSevenPairs(hand, 1), false);
});

test('带副露的胡牌：每个副露顶掉一个面子', () => {
  // 已碰 1 组（东），手牌需 3 面子 + 将 = 11 张
  const hand = [w(1), w(2), w(3), w(4), w(5), w(6), t(7), t(8), t(9), d(5), d(5)];
  assert.equal(hand.length, 11);
  assert.equal(isStandardWin(hand, 1), true);
  // 张数不对应判否
  assert.equal(isStandardWin(hand, 0), false);
});

test('碰 / 明杠 / 暗杠 判定', () => {
  const hand = [w(5), w(5), t(3), t(3), t(3), d(9), d(9), d(9), d(9)];
  assert.equal(canPeng(hand, w(5)), true);
  assert.equal(canPeng(hand, w(6)), false);
  assert.equal(canGangFromDiscard(hand, t(3)), true, '手里三张可明杠');
  assert.equal(canGangFromDiscard(hand, w(5)), false, '手里两张不能杠');
  assert.deepEqual(findConcealedGangs(hand), [d(9)], '手里四张可暗杠');
});

test('听牌检测：返回的每张牌都真的能胡', () => {
  // 123万 456万 789万 123条 + 5筒（听 5 筒成将）
  const hand = [w(1), w(2), w(3), w(4), w(5), w(6), w(7), w(8), w(9),
    t(1), t(2), t(3), d(5)];
  assert.equal(hand.length, 13);
  const waits = waitingTiles(hand);
  assert.ok(waits.length > 0, '应该听牌');
  assert.ok(waits.includes(d(5)), '应听 5 筒');
  for (const tile of waits) {
    assert.equal(canWin([...hand, tile]), true, `听的牌 ${tileLabel(tile)} 必须真能胡`);
  }
});

test('番型计算：七小对 / 清一色 / 碰碰胡 / 自摸', () => {
  const sevenPairs = [w(1), w(1), w(3), w(3), w(5), w(5), w(7), w(7),
    w(9), w(9), w(2), w(2), w(4), w(4)];
  const s1 = scoreWin(sevenPairs, [], {});
  assert.ok(s1.patterns.includes('七小对'));
  assert.ok(s1.patterns.includes('清一色'), '全万应判清一色');
  assert.ok(s1.fan >= 7, `七小对+清一色应有较高番数，实际 ${s1.fan}`);

  // 碰碰胡：全刻子
  const allTriplets = [w(1), w(1), w(1), w(3), w(3), w(3), t(5), t(5), t(5),
    d(7), d(7), d(7), F.中, F.中];
  const s2 = scoreWin(allTriplets, [], {});
  assert.ok(s2.patterns.includes('碰碰胡'));

  // 自摸加番
  const plain = [w(1), w(2), w(3), w(4), w(5), w(6), w(7), w(8), w(9),
    t(1), t(2), t(3), d(5), d(5)];
  const noDraw = scoreWin(plain, [], {});
  const selfDraw = scoreWin(plain, [], { selfDraw: true });
  assert.equal(selfDraw.fan, noDraw.fan + 1, '自摸应加 1 番');
  assert.ok(selfDraw.patterns.includes('自摸'));
});

test('番型：杠加番', () => {
  const hand = [w(1), w(2), w(3), w(4), w(5), w(6), t(7), t(8), t(9), d(5), d(5)];
  const withGang = scoreWin(hand, [{ type: 'gang', tile: F.东 }], {});
  const withPeng = scoreWin(hand, [{ type: 'peng', tile: F.东 }], {});
  assert.ok(withGang.fan > withPeng.fan, '杠应比碰番数高');
});

test('向听数：已胡为 0、听牌为 1、乱牌更大', () => {
  const won = [w(1), w(2), w(3), w(4), w(5), w(6), w(7), w(8), w(9),
    t(1), t(2), t(3), d(5), d(5)];
  assert.equal(shantenEstimate(won), 0);

  const ready = [w(1), w(2), w(3), w(4), w(5), w(6), w(7), w(8), w(9),
    t(1), t(2), t(3), d(5)];
  assert.equal(shantenEstimate(ready), 1);

  const messy = [w(1), w(4), w(7), t(2), t(5), t(8), d(3), d(6), d(9),
    F.东, F.南, F.西, F.北];
  assert.ok(shantenEstimate(messy) > 1, '散牌向听数应大于 1');
});

test('红中麻将：红中可补将、刻子与顺子', () => {
  const pairWildcard = [
    w(1), w(2), w(3), w(4), w(5), w(6), w(7), w(8), w(9),
    t(1), t(2), t(3), d(5), F.中
  ];
  assert.equal(canWin(pairWildcard), false, '普通麻将不能把红中当万能牌');
  assert.equal(canWinRedCenter(pairWildcard), true, '红中应可补 5 筒成将');

  const setWildcards = [
    w(1), w(1), w(1), w(3), w(4), F.中,
    t(6), t(6), F.中, d(7), d(8), d(9), F.东, F.东
  ];
  assert.equal(canWinRedCenter(setWildcards), true, '两张红中应分别补 345 万与 666 条');
});

test('红中麻将：听牌提示和番型标识使用红中规则', () => {
  const ready = [
    w(1), w(2), w(3), w(4), w(5), w(6), w(7), w(8), w(9),
    t(1), t(2), t(3), F.中
  ];
  const waits = waitingTilesRedCenter(ready);
  assert.ok(waits.includes(d(5)), '红中已有一张时，任意对子牌都应形成可胡牌型');

  const winning = [...ready, d(5)];
  const score = scoreRedCenterWin(winning, [], { selfDraw: true });
  assert.ok(score.patterns.includes('红中赖子'));
  assert.ok(score.patterns.includes('自摸'));
  assert.ok(score.fan >= 3);
  assert.equal(RED_CENTER_TILE, F.中);
});

test('红中麻将：对局视图明确返回红中规则集', () => {
  const game = createGame({ ruleset: 'red-center', kind: 'mahjong_red', humanSeats: [0, 1, 2, 3] });
  const state = viewFor(game, 0);
  assert.equal(state.kind, 'mahjong_red');
  assert.equal(state.ruleset, 'red-center');
  assert.equal(state.myHand.length, 14);
});
