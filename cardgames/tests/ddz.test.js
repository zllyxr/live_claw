import test from 'node:test';
import assert from 'node:assert/strict';
import {
  ComboType, identify, beats, cardRank, freshDeck, deal, sortCards,
  findPlays, isSubset, removeCards, handStrength
} from '../src/ddz/rules.js';

/** 构造指定 rank 的牌。rank 0..12 -> 3..2；13 小王；14 大王 */
function c(rank, nth = 0) {
  if (rank === 13) return 52;
  if (rank === 14) return 53;
  return rank * 4 + nth;
}
const cards = (...specs) => specs.map(([r, n = 0]) => c(r, n));

test('牌组构成正确：54 张、含双王', () => {
  const deck = freshDeck();
  assert.equal(deck.length, 54);
  assert.equal(new Set(deck).size, 54);
  assert.equal(cardRank(52), 13);
  assert.equal(cardRank(53), 14);
});

test('发牌：三家 17 张 + 3 张底牌，无重复', () => {
  const { hands, bottom } = deal();
  assert.equal(hands.length, 3);
  hands.forEach((h) => assert.equal(h.length, 17));
  assert.equal(bottom.length, 3);
  const all = [...hands.flat(), ...bottom];
  assert.equal(all.length, 54);
  assert.equal(new Set(all).size, 54, '发牌不能有重复');
});

test('识别：单张 / 对子 / 三张 / 炸弹 / 王炸', () => {
  assert.equal(identify(cards([5])).type, ComboType.SINGLE);
  assert.equal(identify(cards([5, 0], [5, 1])).type, ComboType.PAIR);
  assert.equal(identify(cards([5, 0], [5, 1], [5, 2])).type, ComboType.TRIPLET);
  assert.equal(identify(cards([5, 0], [5, 1], [5, 2], [5, 3])).type, ComboType.BOMB);
  assert.equal(identify([52, 53]).type, ComboType.ROCKET);
  // 两张不同点数不是对子
  assert.equal(identify(cards([5], [6])).type, ComboType.INVALID);
  // 单张王不是王炸
  assert.equal(identify([53]).type, ComboType.SINGLE);
});

test('识别：三带一 / 三带二', () => {
  assert.equal(identify(cards([5, 0], [5, 1], [5, 2], [9])).type, ComboType.TRIPLET_ONE);
  const tp = identify(cards([5, 0], [5, 1], [5, 2], [9, 0], [9, 1]));
  assert.equal(tp.type, ComboType.TRIPLET_PAIR);
  assert.equal(tp.mainRank, 5, 'mainRank 应为三张的点数');
  // 三带二必须是一对，不能是两张不同单牌
  assert.equal(identify(cards([5, 0], [5, 1], [5, 2], [9], [10])).type, ComboType.INVALID);
});

test('识别：顺子边界', () => {
  // 3..7 五连合法
  assert.equal(identify(cards([0], [1], [2], [3], [4])).type, ComboType.STRAIGHT);
  // 四连不合法
  assert.equal(identify(cards([0], [1], [2], [3])).type, ComboType.INVALID);
  // 到 A（rank 11）合法
  assert.equal(identify(cards([7], [8], [9], [10], [11])).type, ComboType.STRAIGHT);
  // 含 2（rank 12）不合法
  assert.equal(identify(cards([8], [9], [10], [11], [12])).type, ComboType.INVALID);
  // 含王不合法
  assert.equal(identify(cards([10], [11], [12], [13], [14])).type, ComboType.INVALID);
});

test('识别：连对与飞机', () => {
  const dbl = identify(cards([0, 0], [0, 1], [1, 0], [1, 1], [2, 0], [2, 1]));
  assert.equal(dbl.type, ComboType.DOUBLE_STRAIGHT);
  assert.equal(dbl.length, 3);
  // 两连对不合法
  assert.equal(identify(cards([0, 0], [0, 1], [1, 0], [1, 1])).type, ComboType.INVALID);

  const plane = identify(cards([3, 0], [3, 1], [3, 2], [4, 0], [4, 1], [4, 2]));
  assert.equal(plane.type, ComboType.PLANE);

  const wings = identify(cards([3, 0], [3, 1], [3, 2], [4, 0], [4, 1], [4, 2], [8], [9]));
  assert.equal(wings.type, ComboType.PLANE_WINGS);
  assert.equal(wings.wing, 'single');

  const wingPairs = identify(cards([3, 0], [3, 1], [3, 2], [4, 0], [4, 1], [4, 2],
    [8, 0], [8, 1], [9, 0], [9, 1]));
  assert.equal(wingPairs.type, ComboType.PLANE_WINGS);
  assert.equal(wingPairs.wing, 'pair');
});

test('识别：四带二', () => {
  const fourSingles = identify(cards([5, 0], [5, 1], [5, 2], [5, 3], [8], [9]));
  assert.equal(fourSingles.type, ComboType.FOUR_TWO);
  const fourPairs = identify(cards([5, 0], [5, 1], [5, 2], [5, 3], [8, 0], [8, 1], [9, 0], [9, 1]));
  assert.equal(fourPairs.type, ComboType.FOUR_TWO);
});

test('比较：同型比大小、长度必须一致', () => {
  const small = identify(cards([5]));
  const big = identify(cards([9]));
  assert.equal(beats(big, small), true);
  assert.equal(beats(small, big), false);
  // 自由出牌
  assert.equal(beats(small, null), true);

  // 顺子长度不同不能压
  const s5 = identify(cards([0], [1], [2], [3], [4]));
  const s6 = identify(cards([1], [2], [3], [4], [5], [6]));
  assert.equal(beats(s6, s5), false, '六连不能压五连');

  // 不同类型不能互压
  assert.equal(beats(identify(cards([9, 0], [9, 1])), small), false);
});

test('比较：炸弹与王炸的压制关系', () => {
  const single = identify(cards([12]));           // 单张 2
  const bomb5 = identify(cards([5, 0], [5, 1], [5, 2], [5, 3]));
  const bomb9 = identify(cards([9, 0], [9, 1], [9, 2], [9, 3]));
  const rocket = identify([52, 53]);

  assert.equal(beats(bomb5, single), true, '炸弹压任何普通牌');
  assert.equal(beats(bomb9, bomb5), true, '大炸弹压小炸弹');
  assert.equal(beats(bomb5, bomb9), false);
  assert.equal(beats(rocket, bomb9), true, '王炸压所有炸弹');
  assert.equal(beats(bomb9, rocket), false, '炸弹压不过王炸');
  assert.equal(beats(single, bomb5), false);
});

test('手牌子集校验与移除', () => {
  const hand = cards([5, 0], [5, 1], [9, 0]);
  assert.equal(isSubset(cards([5, 0], [5, 1]), hand), true);
  assert.equal(isSubset(cards([5, 0], [5, 2]), hand), false, '手里没有的牌不能出');
  assert.equal(removeCards(hand, cards([5, 0])).length, 2);
});

test('findPlays：能找出所有压过目标的出法且都合法', () => {
  const hand = sortCards(cards([5, 0], [5, 1], [9, 0], [9, 1], [12, 0], [13], [14]));
  const target = identify(cards([6, 0], [6, 1])); // 一对 8
  const plays = findPlays(hand, target);
  assert.ok(plays.length > 0, '应能找到压制出法');
  for (const play of plays) {
    assert.equal(beats(play.combo, target), true, '返回的出法必须能压过目标');
    assert.equal(isSubset(play.cards, hand), true, '返回的出法必须是手牌子集');
  }
  // 应包含一对 9（rank 9）
  assert.ok(plays.some((p) => p.combo.type === ComboType.PAIR && p.combo.mainRank === 9));
  // 应包含王炸
  assert.ok(plays.some((p) => p.combo.type === ComboType.ROCKET));
});

test('findPlays：自由出牌时不会返回非法牌型', () => {
  const hand = sortCards(cards([0], [1], [2], [3], [4], [5, 0], [5, 1]));
  const plays = findPlays(hand, null);
  assert.ok(plays.length > 0);
  for (const play of plays) {
    assert.notEqual(play.combo.type, ComboType.INVALID);
    assert.equal(isSubset(play.cards, hand), true);
  }
  // 应能找到 3..7 顺子
  assert.ok(plays.some((p) => p.combo.type === ComboType.STRAIGHT && p.combo.length === 5));
});

test('handStrength：王炸和炸弹显著加分', () => {
  const weak = cards([0], [1], [2]);
  const strong = cards([0], [1], [2], [13], [14]);
  assert.ok(handStrength(strong) > handStrength(weak));
  const bomb = cards([5, 0], [5, 1], [5, 2], [5, 3]);
  assert.ok(handStrength(bomb) > handStrength(weak));
});
