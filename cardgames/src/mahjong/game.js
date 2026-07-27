/**
 * 麻将服务端权威对局状态机（推倒胡）。
 *
 * 座位 0 = 玩家，1/2/3 = AI，逆序摸打 0 → 1 → 2 → 3 → 0
 * 流程：发牌（庄 14 张，闲 13 张）→ 打牌/摸牌循环 → 有人胡或牌墙摸完
 *
 * 所有胡/碰/杠判定都在服务端做，客户端只提交动作。
 */
import {
  freshWall, shuffle, sortTiles, tileLabel, toCounts,
  canWin, canWinRedCenter, scoreWin, scoreRedCenterWin,
  canPeng, canGangFromDiscard, findConcealedGangs,
  waitingTiles, waitingTilesRedCenter, shantenEstimate, isWind, tileNumber,
  RED_CENTER_TILE
} from './rules.js';

const DEFAULT_SEAT_NAMES = ['你', '下家', '对家', '上家'];
const HAND_SIZE = 13;

export function createGame(options = {}) {
  const wall = shuffle(freshWall());
  const hands = [[], [], [], []];
  let cursor = 0;
  for (let seat = 0; seat < 4; seat++) {
    hands[seat] = sortTiles(wall.slice(cursor, cursor + HAND_SIZE));
    cursor += HAND_SIZE;
  }
  const dealer = 0; // 玩家坐庄，先摸一张
  const game = {
    id: null,
    phase: 'playing',
    wall: wall.slice(cursor),
    hands,
    melds: [[], [], [], []],   // 每家副露 [{type, tile}]
    discards: [[], [], [], []],
    dealer,
    current: dealer,
    drawnTile: null,           // 当前手上刚摸的牌（等待打出）
    pendingClaim: null,        // 待玩家决策：{ tile, from, canPeng, canGang, canWin }
    lastDiscard: null,
    winner: -1,
    result: null,
    deferBots: Boolean(options.deferBots),
    history: [],
    ruleset: options.ruleset === 'red-center' ? 'red-center' : 'classic',
    kind: options.kind || (options.ruleset === 'red-center' ? 'mahjong_red' : 'mahjong'),
    humanSeats: Array.isArray(options.humanSeats) ? [...new Set(options.humanSeats)] : [0],
    seatNames: Array.isArray(options.seatNames) && options.seatNames.length === 4
      ? options.seatNames.map(String)
      : DEFAULT_SEAT_NAMES.slice(),
    baseStake: Math.max(1, Math.floor(Number(options.baseStake) || 1)),
    createdAt: Date.now()
  };
  drawTile(game, dealer);
  return game;
}

function isHuman(game, seat) {
  return (game.humanSeats || [0]).includes(seat);
}

function isRedCenter(game) {
  return game.ruleset === 'red-center';
}

function canGameWin(game, tiles, meldCount) {
  return isRedCenter(game)
    ? canWinRedCenter(tiles, meldCount)
    : canWin(tiles, meldCount);
}

function waitingGameTiles(game, tiles, meldCount) {
  return isRedCenter(game)
    ? waitingTilesRedCenter(tiles, meldCount)
    : waitingTiles(tiles, meldCount);
}

function shantenGame(game, tiles, meldCount) {
  if (canGameWin(game, tiles, meldCount)) return 0;
  if (waitingGameTiles(game, tiles, meldCount).length > 0) return 1;
  return shantenEstimate(tiles, meldCount);
}

function scoreGameWin(game, tiles, melds, opts) {
  return isRedCenter(game)
    ? scoreRedCenterWin(tiles, melds, opts)
    : scoreWin(tiles, melds, opts);
}

function claimablePeng(game, hand, tile) {
  return !(isRedCenter(game) && tile === RED_CENTER_TILE) && canPeng(hand, tile);
}

function claimableGang(game, hand, tile) {
  return !(isRedCenter(game) && tile === RED_CENTER_TILE) && canGangFromDiscard(hand, tile);
}

function concealedGangsFor(game, hand) {
  return findConcealedGangs(hand)
    .filter((tile) => !(isRedCenter(game) && tile === RED_CENTER_TILE));
}

function drawTile(game, seat) {
  if (game.wall.length === 0) {
    endDraw(game);
    return null;
  }
  const tile = game.wall.shift();
  game.hands[seat] = sortTiles([...game.hands[seat], tile]);
  game.drawnTile = tile;
  game.current = seat;
  game.history.push({ type: 'draw', seat, tile, label: tileLabel(tile) });
  return tile;
}

function endDraw(game) {
  game.phase = 'finished';
  game.winner = -1;
  game.result = { draw: true, message: '牌墙摸完，本局流局', score: 0 };
  game.history.push({ type: 'draw_end' });
}

function declareWin(game, seat, selfDraw, fromSeat = game.lastDiscard?.seat ?? -1) {
  const { fan, patterns } = scoreGameWin(game, game.hands[seat], game.melds[seat], { selfDraw });
  const unit = Math.max(1, fan) * game.baseStake;
  const scores = [0, 0, 0, 0];
  if (selfDraw) {
    for (let other = 0; other < 4; other++) {
      if (other === seat) continue;
      scores[other] = -unit;
      scores[seat] += unit;
    }
  } else {
    scores[seat] = unit * 3;
    scores[fromSeat] = -unit * 3;
  }
  game.phase = 'finished';
  game.winner = seat;
  game.result = {
    draw: false,
    winner: seat,
    winnerName: (game.seatNames || DEFAULT_SEAT_NAMES)[seat],
    selfDraw,
    fan,
    patterns,
    scores,
    playerWon: seat === 0,
    score: scores[0]
  };
  game.history.push({ type: 'win', seat, fan, patterns, selfDraw });
}

/** 玩家打出一张牌 */
export function discard(game, seat, tile) {
  if (game.phase !== 'playing') throw new Error('本局已结束');
  if (game.pendingClaim) throw new Error('请先处理碰/杠/胡的选择');
  if (game.current !== seat) throw new Error('还没轮到你');
  const counts = toCounts(game.hands[seat]);
  if (!counts[tile]) throw new Error('你手里没有这张牌');

  const idx = game.hands[seat].indexOf(tile);
  game.hands[seat].splice(idx, 1);
  game.discards[seat].push(tile);
  game.lastDiscard = { seat, tile };
  game.drawnTile = null;
  game.history.push({ type: 'discard', seat, tile, label: tileLabel(tile) });

  afterDiscard(game, seat, tile);
  return game;
}

/**
 * 一张牌被打出后：先看其他家能否胡（优先），再看能否碰/杠。
 * 玩家的可行动作挂到 pendingClaim 等前端决策；AI 直接决策。
 */
function afterDiscard(game, fromSeat, tile) {
  return processClaims(game, fromSeat, tile, 1);
}

function processClaims(game, fromSeat, tile, startStep) {
  // 依座位顺序检查其他三家
  for (let step = startStep; step <= 3; step++) {
    const seat = (fromSeat + step) % 4;
    const hand = game.hands[seat];
    const meldCount = game.melds[seat].length;
    const winnable = canGameWin(game, [...hand, tile], meldCount);
    const pengable = claimablePeng(game, hand, tile);
    const gangable = claimableGang(game, hand, tile);

    if (!winnable && !pengable && !gangable) continue;

    if (game.deferBots || isHuman(game, seat)) {
      game.pendingClaim = {
        tile, label: tileLabel(tile), from: fromSeat, seat, nextStep: step + 1,
        canWin: winnable, canPeng: pengable, canGang: gangable
      };
      return;
    }

    // AI 决策：能胡必胡；碰/杠看是否有助于成牌
    if (winnable) {
      game.hands[seat] = sortTiles([...hand, tile]);
      declareWin(game, seat, false, fromSeat);
      return;
    }
    if (gangable && aiWantsMeld(game, seat, tile, 'gang')) {
      applyMeld(game, seat, tile, 'gang', fromSeat);
      aiDiscard(game, seat);
      return;
    }
    if (pengable && aiWantsMeld(game, seat, tile, 'peng')) {
      applyMeld(game, seat, tile, 'peng', fromSeat);
      aiDiscard(game, seat);
      return;
    }
  }

  // 无人要牌 → 下家摸牌
  const next = (fromSeat + 1) % 4;
  const drawn = drawTile(game, next);
  if (drawn === null) return;

  if (game.deferBots || isHuman(game, next)) {
    game.pendingClaim = null;
    return;
  }
  aiTurn(game, next);
}

/** 玩家放弃碰/杠/胡 */
export function skipClaim(game, seat = 0) {
  if (!game.pendingClaim) throw new Error('当前没有待决策的操作');
  if (game.pendingClaim.seat !== undefined && game.pendingClaim.seat !== seat) {
    throw new Error('当前不是你的操作');
  }
  const { from, tile, nextStep = 4 } = game.pendingClaim;
  game.pendingClaim = null;
  game.history.push({ type: 'skip', seat });
  processClaims(game, from, tile, nextStep);
  return game;
}

/** 玩家碰 */
export function peng(game, seat = 0) {
  const claim = game.pendingClaim;
  if (!claim || (claim.seat !== undefined && claim.seat !== seat) || !claim.canPeng) throw new Error('当前不能碰');
  applyMeld(game, seat, claim.tile, 'peng', claim.from);
  game.pendingClaim = null;
  game.current = seat; // 碰后由自己打出一张
  return game;
}

/** 玩家杠（明杠） */
export function gang(game, seat = 0) {
  const claim = game.pendingClaim;
  if (!claim || (claim.seat !== undefined && claim.seat !== seat) || !claim.canGang) throw new Error('当前不能杠');
  applyMeld(game, seat, claim.tile, 'gang', claim.from);
  game.pendingClaim = null;
  // 杠后补摸一张
  drawTile(game, seat);
  return game;
}

/** 玩家暗杠 */
export function concealedGang(game, seat, tile) {
  if (game.current !== seat) throw new Error('还没轮到你');
  if (!concealedGangsFor(game, game.hands[seat]).includes(tile)) throw new Error('这张牌不能暗杠');
  game.hands[seat] = game.hands[seat].filter((t) => t !== tile);
  game.melds[seat].push({ type: 'gang', tile, concealed: true });
  game.history.push({ type: 'gang', seat, tile, label: tileLabel(tile), concealed: true });
  drawTile(game, seat);
  return game;
}

/** 玩家胡（点炮胡或自摸） */
export function declare(game, seat = 0) {
  const claim = game.pendingClaim;
  if (claim && (claim.seat === undefined || claim.seat === seat) && claim.canWin) {
    game.hands[seat] = sortTiles([...game.hands[seat], claim.tile]);
    game.pendingClaim = null;
    declareWin(game, seat, false, claim.from);
    return game;
  }
  // 自摸
  if (canGameWin(game, game.hands[seat], game.melds[seat].length)) {
    declareWin(game, seat, true);
    return game;
  }
  throw new Error('当前牌型不能胡');
}

function applyMeld(game, seat, tile, type, fromSeat) {
  const need = type === 'gang' ? 3 : 2;
  let removed = 0;
  game.hands[seat] = game.hands[seat].filter((t) => {
    if (t === tile && removed < need) { removed++; return false; }
    return true;
  });
  game.melds[seat].push({ type, tile, from: fromSeat });
  game.history.push({ type, seat, tile, label: tileLabel(tile), from: fromSeat });
  // 被碰/杠的牌从对方牌河移除，语义上归入副露
  const pool = game.discards[fromSeat];
  const idx = pool.lastIndexOf(tile);
  if (idx !== -1) pool.splice(idx, 1);
}

/**
 * AI 是否愿意碰/杠：只有能让向听数变好（或不变差）才要，
 * 避免 AI 乱碰把手牌打散。
 */
function aiWantsMeld(game, seat, tile, type) {
  const hand = game.hands[seat];
  const meldCount = game.melds[seat].length;
  const need = type === 'gang' ? 3 : 2;

  // 副露后剩下的手牌
  let removed = 0;
  const after = hand.filter((t) => {
    if (t === tile && removed < need) { removed++; return false; }
    return true;
  });

  const before = shantenGame(game, hand, meldCount);
  const afterShanten = shantenGame(game, after, meldCount + 1);

  // 杠白得一番且能补牌，条件放宽一点
  if (type === 'gang') return afterShanten <= before;
  return afterShanten < before;
}

/** AI 摸牌后的完整回合 */
function aiTurn(game, seat) {
  if (game.phase !== 'playing') return;
  const meldCount = game.melds[seat].length;

  // 自摸
  if (canGameWin(game, game.hands[seat], meldCount)) {
    declareWin(game, seat, true);
    return;
  }
  // 暗杠（有四张且不破坏听牌时才杠）
  const gangs = concealedGangsFor(game, game.hands[seat]);
  if (gangs.length > 0) {
    const tile = gangs[0];
    const before = shantenGame(game, game.hands[seat], meldCount);
    const after = shantenGame(game, game.hands[seat].filter((t) => t !== tile), meldCount + 1);
    if (after <= before) {
      concealedGangInternal(game, seat, tile);
      return;
    }
  }
  aiDiscard(game, seat);
}

function concealedGangInternal(game, seat, tile) {
  game.hands[seat] = game.hands[seat].filter((t) => t !== tile);
  game.melds[seat].push({ type: 'gang', tile, concealed: true });
  game.history.push({ type: 'gang', seat, tile, label: tileLabel(tile), concealed: true });
  const drawn = drawTile(game, seat);
  if (drawn === null) return;
  if (!game.deferBots && !isHuman(game, seat)) aiTurn(game, seat);
}

/** 延迟调度模式下一次只执行一个机器人动作，由牌桌服务器决定等待时间。 */
export function playBotTurn(game, seat) {
  if (game.phase !== 'playing') return game;
  const claim = game.pendingClaim;
  if (claim && claim.seat === seat) {
    if (claim.canWin) return declare(game, seat);
    if (claim.canGang && aiWantsMeld(game, seat, claim.tile, 'gang')) return gang(game, seat);
    if (claim.canPeng && aiWantsMeld(game, seat, claim.tile, 'peng')) return peng(game, seat);
    return skipClaim(game, seat);
  }
  if (game.current === seat) aiTurn(game, seat);
  return game;
}

/**
 * AI 打牌：选让向听数最小的那张打出。
 * 同分时优先打孤张风牌、再打边张（1/9），保留中间牌。
 */
function aiDiscard(game, seat) {
  const hand = game.hands[seat];
  const meldCount = game.melds[seat].length;
  const candidates = [...new Set(hand)];

  let best = candidates[0];
  let bestScore = Infinity;
  for (const tile of candidates) {
    const trimmed = removeOne(hand, tile);
    const shanten = shantenGame(game, trimmed, meldCount);
    const waits = waitingGameTiles(game, trimmed, meldCount).length;
    const counts = toCounts(hand);
    // 打分：向听数为主，听口数为辅，风牌与边张略微加权丢弃
    let score = shanten * 100 - waits;
    if (isWind(tile) && counts[tile] === 1) score -= 12;
    const num = tileNumber(tile);
    if (!isWind(tile) && (num === 1 || num === 9) && counts[tile] === 1) score -= 4;
    if (score < bestScore) { bestScore = score; best = tile; }
  }

  const idx = game.hands[seat].indexOf(best);
  game.hands[seat].splice(idx, 1);
  game.discards[seat].push(best);
  game.lastDiscard = { seat, tile: best };
  game.drawnTile = null;
  game.history.push({ type: 'discard', seat, tile: best, label: tileLabel(best) });
  afterDiscard(game, seat, best);
}

function removeOne(tiles, tile) {
  const out = tiles.slice();
  const idx = out.indexOf(tile);
  if (idx !== -1) out.splice(idx, 1);
  return out;
}

/** 客户端视图：隐藏他人手牌 */
export function viewFor(game, seat = 0) {
  const hand = game.hands[seat];
  const meldCount = game.melds[seat].length;
  const myTurn = game.current === seat && !game.pendingClaim && game.phase === 'playing';
  return {
    id: game.id,
    kind: game.kind,
    ruleset: game.ruleset,
    seat,
    phase: game.phase,
    myHand: hand,
    myHandLabels: hand.map(tileLabel),
    handCounts: game.hands.map((h) => h.length),
    melds: game.melds.map((list) => list.map((m) => ({ ...m, label: tileLabel(m.tile) }))),
    discards: game.discards.map((list) => list.map((t) => ({ tile: t, label: tileLabel(t) }))),
    seatNames: game.seatNames || DEFAULT_SEAT_NAMES,
    current: game.current,
    wallLeft: game.wall.length,
    drawnTile: seat === game.current ? game.drawnTile : null,
    pendingClaim: game.pendingClaim && game.pendingClaim.seat === seat
      ? game.pendingClaim
      : null,
    myTurn,
    canSelfWin: myTurn && canGameWin(game, hand, meldCount),
    concealedGangs: myTurn ? concealedGangsFor(game, hand).map((t) => ({ tile: t, label: tileLabel(t) })) : [],
    // 听牌提示：服务端算，客户端不实现规则
    waiting: waitingGameTiles(game, hand, meldCount).map((t) => ({ tile: t, label: tileLabel(t) })),
    shanten: shantenGame(game, hand, meldCount),
    history: game.history.slice(-14),
    winner: game.winner,
    result: game.result
      ? {
        ...game.result,
        winnerName: game.result.draw
          ? ''
          : (game.seatNames || DEFAULT_SEAT_NAMES)[game.result.winner],
        playerWon: !game.result.draw && game.result.scores[seat] > 0,
        score: game.result.draw ? 0 : game.result.scores[seat]
      }
      : null
  };
}
