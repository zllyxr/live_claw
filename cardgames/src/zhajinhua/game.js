import { cardRank, cardSuit, shuffle, sortCards } from '../ddz/rules.js';

export const MAX_PLAYERS = 3;
export const BUY_IN = 100;
const ANTE = 5;
const BET_LEVELS = [5, 10, 20];
export const MAX_ROUNDS = 10;

function pokerRank(card) {
  const rank = cardRank(card);
  if (rank === 12) return 2;
  return rank + 3;
}

function straightHigh(ranks) {
  const unique = [...new Set(ranks)].sort((a, b) => a - b);
  if (unique.length !== 3) return 0;
  if (unique[0] + 1 === unique[1] && unique[1] + 1 === unique[2]) return unique[2];
  if (unique.join(',') === '2,3,14') return 3;
  return 0;
}

/** 炸金花牌型：豹子 > 同花顺 > 金花 > 顺子 > 对子 > 单张。 */
export function evaluateHand(cards) {
  if (!Array.isArray(cards) || cards.length !== 3) throw new Error('炸金花必须正好三张牌');
  const ranks = cards.map(pokerRank).sort((a, b) => b - a);
  const suits = cards.map(cardSuit);
  const counts = new Map();
  ranks.forEach((rank) => counts.set(rank, (counts.get(rank) || 0) + 1));
  const groups = [...counts.entries()].sort((a, b) => b[1] - a[1] || b[0] - a[0]);
  const flush = new Set(suits).size === 1;
  const high = straightHigh(ranks);
  let category = 0;
  let name = '单张';
  let tie = ranks;
  if (groups[0][1] === 3) {
    category = 5; name = '豹子'; tie = [groups[0][0]];
  } else if (flush && high) {
    category = 4; name = '同花顺'; tie = [high];
  } else if (flush) {
    category = 3; name = '金花';
  } else if (high) {
    category = 2; name = '顺子'; tie = [high];
  } else if (groups[0][1] === 2) {
    category = 1; name = '对子';
    tie = [groups[0][0], groups.find((group) => group[1] === 1)[0]];
  }
  return { category, name, tie, suitTie: Math.max(...suits) };
}

export function compareHands(left, right) {
  const a = evaluateHand(left);
  const b = evaluateHand(right);
  if (a.category !== b.category) return Math.sign(a.category - b.category);
  for (let index = 0; index < Math.max(a.tie.length, b.tie.length); index++) {
    const diff = Number(a.tie[index] || 0) - Number(b.tie[index] || 0);
    if (diff) return Math.sign(diff);
  }
  return Math.sign(a.suitTie - b.suitTie);
}

export function createGame(options = {}) {
  const deck = shuffle(Array.from({ length: 52 }, (_, index) => index));
  const hands = [sortCards(deck.slice(0, 3)), sortCards(deck.slice(3, 6)), sortCards(deck.slice(6, 9))];
  const stacks = new Array(MAX_PLAYERS).fill(BUY_IN - ANTE);
  return {
    id: null,
    kind: 'zhajinhua',
    phase: 'playing',
    hands,
    stacks,
    totalBets: new Array(MAX_PLAYERS).fill(ANTE),
    roundBets: new Array(MAX_PLAYERS).fill(0),
    active: new Array(MAX_PLAYERS).fill(true),
    looked: new Array(MAX_PLAYERS).fill(false),
    acted: new Set(),
    current: 0,
    round: 1,
    maxRounds: MAX_ROUNDS,
    highestBet: 0,
    pot: ANTE * MAX_PLAYERS,
    history: [],
    winner: -1,
    result: null,
    seatNames: Array.isArray(options.seatNames) && options.seatNames.length === MAX_PLAYERS
      ? options.seatNames.map(String)
      : ['你', '左家', '右家'],
    createdAt: Date.now()
  };
}

function nextActive(game, from) {
  for (let step = 1; step <= MAX_PLAYERS; step++) {
    const seat = (from + step) % MAX_PLAYERS;
    if (game.active[seat]) return seat;
  }
  return -1;
}

function pay(game, seat, amount) {
  const value = Math.max(0, Math.floor(Number(amount) || 0));
  if (value > game.stacks[seat]) throw new Error('本桌筹码不足');
  game.stacks[seat] -= value;
  game.roundBets[seat] += value;
  game.totalBets[seat] += value;
  game.pot += value;
  return value;
}

function finish(game, winner, reason) {
  game.phase = 'finished';
  game.winner = winner;
  const payouts = game.stacks.slice();
  payouts[winner] += game.pot;
  const scores = payouts.map((value) => value - BUY_IN);
  game.result = {
    winner,
    winnerName: game.seatNames[winner],
    reason,
    handName: evaluateHand(game.hands[winner]).name,
    payouts,
    scores,
    playerWon: winner === 0,
    score: scores[0]
  };
  game.history.push({ type: 'finish', winner, reason });
}

function maybeFinish(game) {
  const alive = game.active.map((active, seat) => active ? seat : -1).filter((seat) => seat >= 0);
  if (alive.length === 1) {
    finish(game, alive[0], '其余玩家已弃牌');
    return true;
  }
  const settled = alive.every((seat) => game.acted.has(seat) && game.roundBets[seat] === game.highestBet);
  if (!settled) return false;
  if (game.round < game.maxRounds && !alive.every((seat) => game.stacks[seat] === 0)) {
    game.round += 1;
    game.roundBets = new Array(MAX_PLAYERS).fill(0);
    game.highestBet = 0;
    game.acted.clear();
    game.history.push({ type: 'round', round: game.round });
    return false;
  }
  const winner = alive.slice().sort((left, right) => compareHands(game.hands[right], game.hands[left]))[0];
  finish(game, winner, `完成 ${game.round} 轮下注，自动开牌`);
  return true;
}

export function act(game, seat, input = {}) {
  if (game.phase !== 'playing') throw new Error('本局已结束');
  if (game.current !== seat || !game.active[seat]) throw new Error('还没轮到你');
  const action = String(input.action || '');
  const required = game.highestBet - game.roundBets[seat];

  if (action === 'look') {
    if (game.looked[seat]) throw new Error('已经看过牌了');
    game.looked[seat] = true;
    game.history.push({ type: 'look', seat });
    return game;
  }
  if (action === 'fold') {
    game.active[seat] = false;
    game.acted.add(seat);
    game.history.push({ type: 'fold', seat });
  } else if (action === 'check') {
    if (required !== 0) throw new Error('当前有下注，不能过牌');
    game.acted.add(seat);
    game.history.push({ type: 'check', seat });
  } else if (action === 'call') {
    if (required <= 0) throw new Error('当前无需跟注');
    pay(game, seat, required);
    game.acted.add(seat);
    game.history.push({ type: 'call', seat, amount: required });
  } else if (action === 'bet') {
    const raise = Math.floor(Number(input.amount));
    if (!BET_LEVELS.includes(raise)) throw new Error('下注档位无效');
    pay(game, seat, required + raise);
    game.highestBet = game.roundBets[seat];
    game.acted = new Set([seat]);
    game.history.push({ type: 'bet', seat, amount: required + raise });
  } else if (action === 'compare') {
    const target = Number(input.target);
    if (!game.active[target] || target === seat) throw new Error('比牌目标无效');
    pay(game, seat, Math.max(5, required));
    const loser = compareHands(game.hands[seat], game.hands[target]) >= 0 ? target : seat;
    game.active[loser] = false;
    game.acted.add(seat);
    game.history.push({ type: 'compare', seat, target, loser });
  } else {
    throw new Error('未知操作');
  }

  if (!maybeFinish(game)) game.current = nextActive(game, seat);
  return game;
}

export function viewFor(game, seat) {
  const required = game.highestBet - game.roundBets[seat];
  return {
    id: game.id,
    kind: game.kind,
    phase: game.phase,
    seat,
    seatNames: game.seatNames,
    current: game.current,
    round: game.round,
    maxRounds: game.maxRounds,
    pot: game.pot,
    stacks: game.stacks,
    totalBets: game.totalBets,
    active: game.active,
    looked: game.looked,
    myHand: game.looked[seat] || game.phase === 'finished' ? game.hands[seat] : [],
    myHandCount: 3,
    handCounts: [3, 3, 3],
    requiredCall: required,
    betLevels: BET_LEVELS.filter((amount) => amount + required <= game.stacks[seat]),
    canCheck: required === 0,
    compareTargets: game.active.map((active, target) => active && target !== seat ? target : -1).filter((target) => target >= 0),
    revealedHands: game.phase === 'finished'
      ? game.hands
      : game.hands.map((hand, target) => target === seat && game.looked[seat] ? hand : []),
    history: game.history.slice(-12),
    winner: game.winner,
    result: game.result
      ? {
        ...game.result,
        playerWon: game.result.scores[seat] > 0,
        score: game.result.scores[seat]
      }
      : null
  };
}
