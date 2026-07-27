/**
 * 斗地主服务端权威对局状态机。
 *
 * 座位：0 = 玩家，1/2 = AI（逆时针出牌 0 → 1 → 2 → 0）
 * 阶段：bidding 叫地主 → playing 出牌 → finished 结算
 *
 * 所有判定都在服务端完成，客户端只提交「出哪几张牌」，
 * 服务端校验是否为手牌子集、牌型是否合法、能否压过上家。
 */
import {
  ComboType, identify, beats, isSubset, removeCards, sortCards,
  deal, findPlays, handStrength, cardLabel
} from './rules.js';

const DEFAULT_SEAT_NAMES = ['你', '左家', '右家'];

export function createGame(options = {}) {
  const { hands, bottom } = deal();
  return {
    id: null,
    phase: 'bidding',
    hands,
    bottom,
    landlord: -1,
    bids: [],            // 叫分记录 [{seat, call}]
    bidTurn: 0,
    current: 0,
    lastPlay: null,      // { seat, cards, combo }
    passCount: 0,
    multiplier: 1,
    history: [],
    winner: -1,
    result: null,
    deferBots: Boolean(options.deferBots),
    humanSeats: Array.isArray(options.humanSeats) ? [...new Set(options.humanSeats)] : [0],
    seatNames: Array.isArray(options.seatNames) && options.seatNames.length === 3
      ? options.seatNames.map(String)
      : DEFAULT_SEAT_NAMES.slice(),
    baseStake: Math.max(1, Math.floor(Number(options.baseStake) || 1)),
    createdAt: Date.now()
  };
}

function isHuman(game, seat) {
  return (game.humanSeats || [0]).includes(seat);
}

/** 玩家叫/不叫地主；随后推进 AI 叫分直到定地主 */
export function bid(game, seat, call) {
  if (game.phase !== 'bidding') throw new Error('当前不是叫地主阶段');
  if (game.bidTurn !== seat) throw new Error('还没轮到你叫分');

  game.bids.push({ seat, call: Boolean(call) });
  game.bidTurn = (game.bidTurn + 1) % 3;
  advanceBidding(game);
  return game;
}

function advanceBidding(game) {
  // AI 依次决策，直到轮到任意真人或一轮结束
  while (!game.deferBots && game.phase === 'bidding' && !isHuman(game, game.bidTurn)) {
    const seat = game.bidTurn;
    const willCall = handStrength(game.hands[seat]) >= 14;
    game.bids.push({ seat, call: willCall });
    game.bidTurn = (game.bidTurn + 1) % 3;
    if (game.bids.length >= 3) break;
  }

  if (game.bids.length < 3) {
    // 还没问完一圈，若回到玩家则等待玩家操作
    return game;
  }

  const callers = game.bids.filter((b) => b.call);
  if (callers.length === 0) {
    // 无人叫地主：重新发牌
    const fresh = createGame({
      humanSeats: game.humanSeats,
      seatNames: game.seatNames,
      baseStake: game.baseStake,
      deferBots: game.deferBots
    });
    Object.assign(game, fresh, { id: game.id, history: [], multiplier: 1 });
    return game;
  }

  // 取叫分者中手牌最强的当地主（简化：无抢地主多轮）
  const landlord = callers
    .map((b) => ({ seat: b.seat, power: handStrength(game.hands[b.seat]) }))
    .sort((a, b) => b.power - a.power)[0].seat;

  setLandlord(game, landlord);
  return game;
}

function setLandlord(game, seat) {
  game.landlord = seat;
  game.hands[seat] = sortCards([...game.hands[seat], ...game.bottom]);
  game.phase = 'playing';
  game.current = seat;
  game.lastPlay = null;
  game.passCount = 0;
  game.history.push({ type: 'landlord', seat, bottom: game.bottom.slice() });
  if (!game.deferBots && !isHuman(game, seat)) runAiTurns(game);
}

/** 玩家出牌 */
export function play(game, seat, cards) {
  if (game.phase !== 'playing') throw new Error('当前不是出牌阶段');
  if (game.current !== seat) throw new Error('还没轮到你出牌');
  if (!Array.isArray(cards) || cards.length === 0) throw new Error('请选择要出的牌');
  if (!isSubset(cards, game.hands[seat])) throw new Error('出的牌不在你的手牌中');

  const combo = identify(cards);
  if (combo.type === ComboType.INVALID) throw new Error('牌型不合法');

  const target = game.passCount >= 2 ? null : game.lastPlay?.combo ?? null;
  if (!beats(combo, target)) throw new Error('这手牌压不过上家');

  applyPlay(game, seat, cards, combo);
  if (!game.deferBots && game.phase === 'playing') runAiTurns(game);
  return game;
}

/** 玩家过牌 */
export function pass(game, seat) {
  if (game.phase !== 'playing') throw new Error('当前不是出牌阶段');
  if (game.current !== seat) throw new Error('还没轮到你');
  if (!game.lastPlay || game.passCount >= 2) throw new Error('你是自由出牌，不能过');

  game.history.push({ type: 'pass', seat });
  game.passCount++;
  game.current = (seat + 1) % 3;
  if (!game.deferBots) runAiTurns(game);
  return game;
}

/** 延迟调度模式下一次只执行一个机器人动作，由牌桌服务器决定等待时间。 */
export function playBotTurn(game, seat) {
  if (game.phase === 'bidding') {
    return bid(game, seat, handStrength(game.hands[seat]) >= 14);
  }
  if (game.phase !== 'playing' || game.current !== seat) return game;
  const target = game.passCount >= 2 ? null : game.lastPlay?.combo ?? null;
  const decision = decideAi(game, seat, target);
  if (decision) {
    applyPlay(game, seat, decision.cards, decision.combo);
  } else {
    game.history.push({ type: 'pass', seat });
    game.passCount++;
    game.current = (seat + 1) % 3;
  }
  return game;
}

function applyPlay(game, seat, cards, combo) {
  game.hands[seat] = removeCards(game.hands[seat], cards);
  game.lastPlay = { seat, cards: sortCards(cards), combo };
  game.passCount = 0;
  game.history.push({
    type: 'play', seat, cards: sortCards(cards),
    comboType: combo.type, labels: sortCards(cards).map(cardLabel)
  });

  // 炸弹/王炸翻倍
  if (combo.type === ComboType.BOMB || combo.type === ComboType.ROCKET) {
    // 平台钱包桌封顶 32 倍，保证最坏输赢不超过已收取的 100 星币入桌金。
    game.multiplier = Math.min(32, game.multiplier * 2);
  }

  if (game.hands[seat].length === 0) {
    finish(game, seat);
    return;
  }
  game.current = (seat + 1) % 3;
}

function finish(game, winnerSeat) {
  game.phase = 'finished';
  game.winner = winnerSeat;
  const landlordWon = winnerSeat === game.landlord;
  const stake = game.baseStake * game.multiplier;
  const scores = [0, 0, 0];
  for (let seat = 0; seat < 3; seat++) {
    const isLandlord = seat === game.landlord;
    const won = isLandlord ? landlordWon : !landlordWon;
    scores[seat] = (isLandlord ? 2 : 1) * stake * (won ? 1 : -1);
  }
  game.result = {
    winner: winnerSeat,
    winnerName: (game.seatNames || DEFAULT_SEAT_NAMES)[winnerSeat],
    landlordWon,
    multiplier: game.multiplier,
    scores,
    playerWon: scores[0] > 0,
    score: scores[0]
  };
  game.history.push({ type: 'finish', seat: winnerSeat, result: game.result });
}

/** 推进所有 AI 回合，直到轮到玩家或对局结束 */
function runAiTurns(game) {
  let guard = 0;
  while (game.phase === 'playing' && !isHuman(game, game.current)) {
    if (++guard > 50) break; // 防御：避免任何意外死循环
    const seat = game.current;
    const target = game.passCount >= 2 ? null : game.lastPlay?.combo ?? null;
    const decision = decideAi(game, seat, target);

    if (decision) {
      applyPlay(game, seat, decision.cards, decision.combo);
    } else {
      game.history.push({ type: 'pass', seat });
      game.passCount++;
      game.current = (seat + 1) % 3;
    }
  }
  return game;
}

/**
 * AI 策略（够用且不犯蠢）：
 *  - 自由出牌：优先出最小的非炸弹牌型，尽量拆出长牌型减少手牌
 *  - 跟牌：出能压过的最小牌；对手只剩 1-2 张时才动炸弹
 *  - 队友（同为闲家）领先时不压自己人
 */
function decideAi(game, seat, target) {
  const hand = game.hands[seat];
  const plays = findPlays(hand, target);
  if (plays.length === 0) return null;

  const isLandlord = seat === game.landlord;
  const lastSeat = game.lastPlay?.seat ?? -1;
  const sameTeam = !isLandlord && lastSeat !== -1 && lastSeat !== game.landlord && lastSeat !== seat;

  // 队友刚出牌且自己是闲家 → 不压队友
  if (target && sameTeam) return null;

  const nonBomb = plays.filter(
    (p) => p.combo.type !== ComboType.BOMB && p.combo.type !== ComboType.ROCKET
  );

  // 对手快走完（<=2 张）时允许用炸弹拦
  const danger = game.hands.some((h, i) => i !== seat && h.length <= 2
    && (isLandlord ? i !== game.landlord : i === game.landlord));

  const pool = nonBomb.length > 0 && !danger ? nonBomb : plays;

  if (!target) {
    // 自由出牌：先挑张数多的牌型消耗手牌，同张数取最小
    return pool.slice().sort((a, b) => {
      const sizeDiff = b.cards.length - a.cards.length;
      if (sizeDiff !== 0) return sizeDiff;
      return a.combo.mainRank - b.combo.mainRank;
    })[0];
  }

  // 跟牌：取能压过的最小一手，张数少优先
  return pool.slice().sort((a, b) => {
    const rankDiff = a.combo.mainRank - b.combo.mainRank;
    if (rankDiff !== 0) return rankDiff;
    return a.cards.length - b.cards.length;
  })[0];
}

/** 输出给客户端的视图：隐藏 AI 手牌，只给张数 */
export function viewFor(game, seat = 0) {
  const target = game.passCount >= 2 ? null : game.lastPlay?.combo ?? null;
  const myHand = game.hands[seat];
  return {
    id: game.id,
    seat,
    phase: game.phase,
    myHand,
    myHandLabels: myHand.map(cardLabel),
    handCounts: game.hands.map((h) => h.length),
    seatNames: game.seatNames || DEFAULT_SEAT_NAMES,
    landlord: game.landlord,
    bottom: game.phase === 'bidding' ? [] : game.bottom,
    current: game.current,
    bidTurn: game.bidTurn,
    bids: game.bids,
    lastPlay: game.lastPlay
      ? {
        seat: game.lastPlay.seat,
        cards: game.lastPlay.cards,
        labels: game.lastPlay.cards.map(cardLabel),
        comboType: game.lastPlay.combo.type
      }
      : null,
    canPass: Boolean(game.lastPlay) && game.passCount < 2 && game.current === seat,
    multiplier: game.multiplier,
    // 提示：当前能出的牌（服务端算，避免客户端实现一套规则）
    hints: game.phase === 'playing' && game.current === seat
      ? findPlays(myHand, target).slice(0, 40).map((p) => p.cards)
      : [],
    history: game.history.slice(-12),
    winner: game.winner,
    result: game.result
      ? {
        ...game.result,
        winnerName: (game.seatNames || DEFAULT_SEAT_NAMES)[game.result.winner],
        playerWon: game.result.scores[seat] > 0,
        score: game.result.scores[seat]
      }
      : null
  };
}
