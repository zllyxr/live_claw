import {
  ComboType, identify, beats, isSubset, removeCards, sortCards,
  shuffle, findPlays, cardLabel
} from '../ddz/rules.js';

export const MAX_PLAYERS = 3;
export const BUY_IN = 100;
const FIRST_CARD = 3; // 黑桃 3

export function createGame(options = {}) {
  // 经典 48 张三人玩法：去掉四张 2 与大小王，每人 16 张。
  const deck = shuffle(Array.from({ length: 48 }, (_, index) => index));
  const hands = [
    sortCards(deck.slice(0, 16)),
    sortCards(deck.slice(16, 32)),
    sortCards(deck.slice(32, 48))
  ];
  const starter = hands.findIndex((hand) => hand.includes(FIRST_CARD));
  return {
    id: null,
    kind: 'paodekuai',
    phase: 'playing',
    hands,
    current: starter,
    starter,
    firstTurn: true,
    lastPlay: null,
    passCount: 0,
    winner: -1,
    result: null,
    history: [],
    seatNames: Array.isArray(options.seatNames) && options.seatNames.length === MAX_PLAYERS
      ? options.seatNames.map(String)
      : ['你', '左家', '右家'],
    createdAt: Date.now()
  };
}

function finish(game, winner) {
  game.phase = 'finished';
  game.winner = winner;
  const payouts = new Array(MAX_PLAYERS).fill(0);
  payouts[winner] = BUY_IN * MAX_PLAYERS;
  const scores = payouts.map((value) => value - BUY_IN);
  game.result = {
    winner,
    winnerName: game.seatNames[winner],
    payouts,
    scores,
    playerWon: winner === 0,
    score: scores[0]
  };
  game.history.push({ type: 'finish', winner });
}

export function play(game, seat, cards) {
  if (game.phase !== 'playing') throw new Error('本局已结束');
  if (game.current !== seat) throw new Error('还没轮到你');
  if (!Array.isArray(cards) || cards.length === 0) throw new Error('请选择要出的牌');
  if (!isSubset(cards, game.hands[seat])) throw new Error('出的牌不在你的手牌中');
  if (game.firstTurn && !cards.includes(FIRST_CARD)) throw new Error('首家第一手必须带黑桃3');
  const combo = identify(cards);
  if (combo.type === ComboType.INVALID || combo.type === ComboType.ROCKET) throw new Error('牌型不合法');
  const target = game.passCount >= 2 ? null : game.lastPlay?.combo ?? null;
  if (!beats(combo, target)) throw new Error('这手牌压不过上家');

  game.hands[seat] = removeCards(game.hands[seat], cards);
  game.lastPlay = { seat, cards: sortCards(cards), combo };
  game.passCount = 0;
  game.firstTurn = false;
  game.history.push({ type: 'play', seat, cards: sortCards(cards), comboType: combo.type });
  if (game.hands[seat].length === 0) {
    finish(game, seat);
  } else {
    game.current = (seat + 1) % MAX_PLAYERS;
  }
  return game;
}

export function pass(game, seat) {
  if (game.phase !== 'playing') throw new Error('本局已结束');
  if (game.current !== seat) throw new Error('还没轮到你');
  if (!game.lastPlay || game.passCount >= 2 || game.lastPlay.seat === seat) throw new Error('当前不能不要');
  game.passCount += 1;
  game.history.push({ type: 'pass', seat });
  game.current = (seat + 1) % MAX_PLAYERS;
  if (game.passCount >= 2) {
    game.current = game.lastPlay.seat;
    game.lastPlay = null;
    game.passCount = 0;
  }
  return game;
}

export function viewFor(game, seat) {
  const target = game.passCount >= 2 ? null : game.lastPlay?.combo ?? null;
  const mustCarryFirst = game.firstTurn && seat === game.starter;
  const hints = game.phase === 'playing' && game.current === seat
    ? findPlays(game.hands[seat], target)
      .filter((playItem) => !mustCarryFirst || playItem.cards.includes(FIRST_CARD))
      .slice(0, 50)
      .map((playItem) => playItem.cards)
    : [];
  return {
    id: game.id,
    kind: game.kind,
    phase: game.phase,
    seat,
    seatNames: game.seatNames,
    myHand: game.hands[seat],
    myHandLabels: game.hands[seat].map(cardLabel),
    handCounts: game.hands.map((hand) => hand.length),
    current: game.current,
    starter: game.starter,
    mustCarryFirst,
    lastPlay: game.lastPlay
      ? {
        seat: game.lastPlay.seat,
        cards: game.lastPlay.cards,
        comboType: game.lastPlay.combo.type
      }
      : null,
    canPass: Boolean(game.lastPlay) && game.lastPlay.seat !== seat,
    hints,
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
