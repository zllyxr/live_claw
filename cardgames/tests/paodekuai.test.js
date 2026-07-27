import assert from 'node:assert/strict';
import test from 'node:test';
import {
  BUY_IN, createGame, pass, play, viewFor
} from '../src/paodekuai/game.js';

test('跑得快：48 张无重复、三家各 16 张、黑桃 3 玩家先手', () => {
  const game = createGame();
  const allCards = game.hands.flat();
  assert.equal(allCards.length, 48);
  assert.equal(new Set(allCards).size, 48);
  assert.deepEqual(game.hands.map((hand) => hand.length), [16, 16, 16]);
  assert.equal(game.hands[game.starter].includes(3), true);
  assert.equal(game.current, game.starter);
});

test('跑得快：首手必须带黑桃 3', () => {
  const game = createGame();
  const hand = game.hands[game.starter];
  const other = hand.find((card) => card !== 3);
  assert.throws(() => play(game, game.starter, [other]), /黑桃3/);
  play(game, game.starter, [3]);
  assert.equal(game.firstTurn, false);
});

test('跑得快：按服务端提示可完整打完，结算资金守恒', () => {
  const game = createGame({ seatNames: ['甲', '乙', '丙'] });
  let turns = 0;
  while (game.phase === 'playing' && turns < 600) {
    const seat = game.current;
    const view = viewFor(game, seat);
    if (view.hints.length > 0) play(game, seat, view.hints[0]);
    else pass(game, seat);
    turns++;
  }

  assert.equal(game.phase, 'finished', `应在有限回合内结束，实际执行 ${turns} 回合`);
  assert.equal(game.hands[game.winner].length, 0);
  assert.equal(game.result.payouts.reduce((sum, value) => sum + value, 0), BUY_IN * 3);
  assert.equal(game.result.scores.reduce((sum, value) => sum + value, 0), 0);
});
