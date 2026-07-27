import assert from 'node:assert/strict';
import test from 'node:test';
import {
  BUY_IN, MAX_ROUNDS, compareHands, createGame, evaluateHand, act, viewFor
} from '../src/zhajinhua/game.js';

test('炸金花：完整识别六类牌型并按规则比较', () => {
  const high = [0, 17, 44];          // 3♦ 7♣ A♦
  const pair = [40, 41, 44];         // K♦ K♣ A♦
  const straight = [0, 5, 10];       // 3♦ 4♣ 5♥
  const flush = [0, 16, 44];         // 3♦ 7♦ A♦
  const straightFlush = [36, 40, 44];// Q♦ K♦ A♦
  const trips = [44, 45, 46];        // A♦ A♣ A♥

  assert.deepEqual(
    [high, pair, straight, flush, straightFlush, trips].map((hand) => evaluateHand(hand).name),
    ['单张', '对子', '顺子', '金花', '同花顺', '豹子']
  );
  assert.equal(compareHands(trips, straightFlush) > 0, true);
  assert.equal(compareHands(flush, straight) > 0, true);
  assert.equal(compareHands(pair, high) > 0, true);
});

test('炸金花：完成十轮后自动开牌，桌上资金守恒', () => {
  const game = createGame({ seatNames: ['甲', '乙', '丙'] });
  for (let round = 1; round <= MAX_ROUNDS; round++) {
    act(game, 0, { action: 'check' });
    act(game, 1, { action: 'check' });
    act(game, 2, { action: 'check' });
  }

  assert.equal(game.phase, 'finished');
  assert.ok(game.winner >= 0 && game.winner < 3);
  assert.equal(game.result.payouts.reduce((sum, value) => sum + value, 0), BUY_IN * 3);
  assert.equal(game.result.scores.reduce((sum, value) => sum + value, 0), 0);
  assert.equal(viewFor(game, 1).revealedHands.every((hand) => hand.length === 3), true);
});

test('炸金花：看牌不跳过当前玩家，弃牌后轮转到下一位', () => {
  const game = createGame();
  act(game, 0, { action: 'look' });
  assert.equal(game.current, 0);
  assert.equal(game.looked[0], true);
  act(game, 0, { action: 'fold' });
  assert.equal(game.current, 1);
  assert.equal(game.active[0], false);
});
