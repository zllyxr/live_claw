import test from 'node:test';
import assert from 'node:assert/strict';
import * as ddz from '../src/ddz/game.js';
import * as mahjong from '../src/mahjong/game.js';
import { identify, beats, findPlays, isSubset } from '../src/ddz/rules.js';
import { canWin } from '../src/mahjong/rules.js';

test('斗地主：开局发牌与视图不泄露 AI 手牌', () => {
  const game = ddz.createGame();
  const view = ddz.viewFor(game, 0);
  assert.equal(view.phase, 'bidding');
  assert.equal(view.myHand.length, 17);
  assert.deepEqual(view.handCounts, [17, 17, 17]);
  assert.equal(view.bottom.length, 0, '叫地主阶段不应看到底牌');
  // 视图里不应有其他人的手牌数组
  assert.equal(view.hands, undefined);
});

test('斗地主：叫地主后地主拿到底牌，且总牌数守恒', () => {
  const game = ddz.createGame();
  ddz.bid(game, 0, true);
  assert.equal(game.phase, 'playing');
  assert.ok(game.landlord >= 0);

  // 地主是玩家时还没人出牌，应恰好 20 张
  if (game.landlord === 0) {
    assert.equal(game.hands[0].length, 20, '地主应有 17+3=20 张');
  } else {
    // 地主是 AI 时引擎已自动出牌，手牌 <= 20
    assert.ok(game.hands[game.landlord].length <= 20);
    assert.ok(game.hands[game.landlord].length > 0);
  }

  // 手牌 + 已打出的牌 = 54，验证没有牌凭空产生或消失
  const played = game.history
    .filter((h) => h.type === 'play')
    .reduce((n, h) => n + h.cards.length, 0);
  const inHands = game.hands.reduce((n, h) => n + h.length, 0);
  assert.equal(inHands + played, 54, '总牌数守恒');
});

test('斗地主：非法出牌被拒绝', () => {
  const game = ddz.createGame();
  ddz.bid(game, 0, true);
  if (game.current !== 0) return; // 地主是 AI，跳过本用例
  // 出手里没有的牌
  const notMine = [...Array(54).keys()].find((c) => !game.hands[0].includes(c));
  assert.throws(() => ddz.play(game, 0, [notMine]), /不在你的手牌/);
  // 非法牌型（两张不同点数）
  const hand = game.hands[0];
  const bad = [hand[0], hand.find((c) => Math.floor(c / 4) !== Math.floor(hand[0] / 4))];
  if (bad[1] !== undefined) {
    assert.throws(() => ddz.play(game, 0, bad), /牌型不合法/);
  }
});

test('斗地主：完整对局能正常结束（用服务端提示自动出牌）', () => {
  // 多跑几局，覆盖地主是玩家/AI 的不同分支
  for (let round = 0; round < 12; round++) {
    const game = ddz.createGame();
    ddz.bid(game, 0, true);

    let guard = 0;
    while (game.phase === 'playing') {
      assert.ok(++guard < 300, '对局不应无限进行');
      if (game.current !== 0) {
        // AI 回合应已由引擎自动推进，走到这里说明卡住了
        assert.fail('AI 回合未自动推进');
      }
      const view = ddz.viewFor(game, 0);
      if (view.hints.length > 0) {
        ddz.play(game, 0, view.hints[0]);
      } else {
        assert.equal(view.canPass, true, '没有可出的牌时必须允许过');
        ddz.pass(game, 0);
      }
    }

    assert.equal(game.phase, 'finished');
    assert.ok(game.winner >= 0 && game.winner <= 2);
    assert.ok(game.result);
    assert.equal(typeof game.result.score, 'number');
    // 赢家手牌必须为空
    assert.equal(game.hands[game.winner].length, 0);
  }
});

test('斗地主：提示的每一手都合法且能压过上家', () => {
  const game = ddz.createGame();
  ddz.bid(game, 0, true);
  let guard = 0;
  while (game.phase === 'playing' && guard++ < 60) {
    if (game.current !== 0) break;
    const view = ddz.viewFor(game, 0);
    const target = game.passCount >= 2 ? null : game.lastPlay?.combo ?? null;
    for (const cards of view.hints) {
      assert.equal(isSubset(cards, game.hands[0]), true);
      const combo = identify(cards);
      assert.equal(beats(combo, target), true);
    }
    if (view.hints.length > 0) ddz.play(game, 0, view.hints[0]);
    else if (view.canPass) ddz.pass(game, 0);
    else break;
  }
});

test('麻将：开局庄家 14 张、闲家 13 张，牌数守恒', () => {
  const game = mahjong.createGame();
  assert.equal(game.hands[0].length, 14, '庄家摸牌后 14 张');
  for (let seat = 1; seat < 4; seat++) {
    assert.equal(game.hands[seat].length, 13);
  }
  const inHands = game.hands.reduce((n, h) => n + h.length, 0);
  assert.equal(inHands + game.wall.length, 136, '手牌 + 牌墙 = 136');
});

test('麻将：视图不泄露他人手牌', () => {
  const game = mahjong.createGame();
  const view = mahjong.viewFor(game, 0);
  assert.equal(view.myHand.length, 14);
  assert.deepEqual(view.handCounts, [14, 13, 13, 13]);
  assert.equal(view.hands, undefined);
  assert.equal(view.wall, undefined, '不应暴露牌墙内容');
});

test('麻将：打出手里没有的牌被拒绝', () => {
  const game = mahjong.createGame();
  const notMine = [...Array(34).keys()].find((t) => !game.hands[0].includes(t));
  assert.throws(() => mahjong.discard(game, 0, notMine), /没有这张牌/);
});

test('麻将：完整对局能结束（流局或有人胡）', () => {
  for (let round = 0; round < 8; round++) {
    const game = mahjong.createGame();
    let guard = 0;

    while (game.phase === 'playing') {
      assert.ok(++guard < 400, '对局不应无限进行');

      if (game.pendingClaim) {
        // 能胡就胡，否则放弃（保证流程能继续）
        if (game.pendingClaim.canWin) mahjong.declare(game, 0);
        else mahjong.skipClaim(game);
        continue;
      }

      if (game.current !== 0) {
        assert.fail('AI 回合未自动推进');
      }

      const view = mahjong.viewFor(game, 0);
      if (view.canSelfWin) {
        mahjong.declare(game, 0);
        continue;
      }
      // 打出第一张（测试只关心流程能走完）
      mahjong.discard(game, 0, view.myHand[0]);
    }

    assert.equal(game.phase, 'finished');
    assert.ok(game.result, '结束时必须有结果');
    if (!game.result.draw) {
      assert.ok(game.result.fan >= 1, '胡牌番数至少 1');
      assert.equal(canWin(game.hands[game.winner], game.melds[game.winner].length), true,
        '宣布胡牌的手牌必须真的能胡');
    }
  }
});

test('麻将：碰牌后副露与手牌数一致', () => {
  // 构造一个必然可碰的局面
  const game = mahjong.createGame();
  // 手动布置：玩家手里两张 5万(索引4)，AI 下家打出 5万
  game.hands[0] = [4, 4, 0, 1, 2, 9, 10, 11, 18, 19, 20, 27, 27];
  game.hands[1] = [4, 5, 6, 7, 8, 12, 13, 14, 15, 16, 17, 21, 22];
  game.current = 1;
  game.drawnTile = null;
  // 让 AI 打出 5万：直接走 discard 内部逻辑
  game.hands[1] = [4, ...game.hands[1].filter((t) => t !== 4)];
  const handBefore = game.hands[0].length;

  // 模拟 1 号打出 4（5万）
  game.hands[1].splice(game.hands[1].indexOf(4), 1);
  game.discards[1].push(4);
  game.lastDiscard = { seat: 1, tile: 4 };
  game.pendingClaim = { tile: 4, label: '5万', from: 1, canWin: false, canPeng: true, canGang: false };

  mahjong.peng(game, 0);
  assert.equal(game.melds[0].length, 1, '应有一组副露');
  assert.equal(game.melds[0][0].type, 'peng');
  assert.equal(game.hands[0].length, handBefore - 2, '碰掉手里两张');
  assert.equal(game.current, 0, '碰后应由自己出牌');
});
