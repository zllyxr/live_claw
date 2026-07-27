/**
 * 斗地主规则引擎
 *
 * 牌的内部表示：整数 0..53
 *   0..51  普通牌，rank = floor(i / 4)，suit = i % 4
 *          rank 0..12 对应 3 4 5 6 7 8 9 10 J Q K A 2
 *   52     小王（rank 13）
 *   53     大王（rank 14）
 *
 * 用 rank 而非牌面值排序，天然满足斗地主大小顺序（3 最小、2 次大、王最大）。
 * 顺子只允许 rank 0..11（3..A），不含 2 和王。
 */

export const RANK_NAMES = ['3', '4', '5', '6', '7', '8', '9', '10', 'J', 'Q', 'K', 'A', '2', '小王', '大王'];
export const SUIT_NAMES = ['♦', '♣', '♥', '♠'];

export const RANK_TWO = 12;      // 2
export const RANK_JOKER_S = 13;  // 小王
export const RANK_JOKER_B = 14;  // 大王
export const MAX_STRAIGHT_RANK = 11; // 顺子最高到 A

/** 牌型枚举 */
export const ComboType = {
  INVALID: 'invalid',
  SINGLE: 'single',            // 单张
  PAIR: 'pair',                // 对子
  TRIPLET: 'triplet',          // 三张
  TRIPLET_ONE: 'triplet_one',  // 三带一
  TRIPLET_PAIR: 'triplet_pair',// 三带二
  STRAIGHT: 'straight',        // 顺子（>=5 连）
  DOUBLE_STRAIGHT: 'double_straight', // 连对（>=3 连对）
  PLANE: 'plane',              // 飞机（>=2 连三张）
  PLANE_WINGS: 'plane_wings',  // 飞机带翅膀
  FOUR_TWO: 'four_two',        // 四带二（两单或两对）
  BOMB: 'bomb',                // 炸弹
  ROCKET: 'rocket'             // 王炸
};

export function cardRank(card) {
  if (card === 52) return RANK_JOKER_S;
  if (card === 53) return RANK_JOKER_B;
  return Math.floor(card / 4);
}

export function cardSuit(card) {
  return card < 52 ? card % 4 : -1;
}

export function cardLabel(card) {
  const rank = cardRank(card);
  if (rank >= RANK_JOKER_S) return RANK_NAMES[rank];
  return SUIT_NAMES[cardSuit(card)] + RANK_NAMES[rank];
}

/** 生成一副 54 张牌 */
export function freshDeck() {
  return Array.from({ length: 54 }, (_, i) => i);
}

/**
 * 洗牌。传入随机函数便于测试复现。
 * 默认使用 crypto 强随机，避免可预测的发牌。
 */
export function shuffle(cards, random = secureRandom) {
  const out = cards.slice();
  for (let i = out.length - 1; i > 0; i--) {
    const j = Math.floor(random() * (i + 1));
    [out[i], out[j]] = [out[j], out[i]];
  }
  return out;
}

let cryptoModule = null;
function secureRandom() {
  if (!cryptoModule) {
    // 延迟加载，浏览器端不会走到这里
    cryptoModule = globalThis.crypto ?? null;
  }
  if (cryptoModule?.getRandomValues) {
    const buf = new Uint32Array(1);
    cryptoModule.getRandomValues(buf);
    return buf[0] / 4294967296;
  }
  return Math.random();
}

/** 发牌：三家各 17 张，留 3 张底牌 */
export function deal(random = secureRandom) {
  const deck = shuffle(freshDeck(), random);
  return {
    hands: [
      sortCards(deck.slice(0, 17)),
      sortCards(deck.slice(17, 34)),
      sortCards(deck.slice(34, 51))
    ],
    bottom: sortCards(deck.slice(51, 54))
  };
}

/** 按 rank 升序排列，同 rank 按花色，保证展示稳定 */
export function sortCards(cards) {
  return cards.slice().sort((a, b) => {
    const diff = cardRank(a) - cardRank(b);
    return diff !== 0 ? diff : a - b;
  });
}

/** 统计 rank -> 数量 */
export function rankCounts(cards) {
  const counts = new Map();
  for (const card of cards) {
    const rank = cardRank(card);
    counts.set(rank, (counts.get(rank) || 0) + 1);
  }
  return counts;
}

/** 判断一组 rank 是否连续，且都不超过 A（顺子/连对/飞机通用） */
function isConsecutive(ranks) {
  if (ranks.some((r) => r > MAX_STRAIGHT_RANK)) return false;
  for (let i = 1; i < ranks.length; i++) {
    if (ranks[i] !== ranks[i - 1] + 1) return false;
  }
  return true;
}

/**
 * 识别牌型。
 * 返回 { type, mainRank, length }；无法识别返回 { type: INVALID }。
 *   mainRank 用于同型比较（顺子/连对/飞机取最小 rank，三带取三张的 rank）
 *   length 用于顺子类长度必须相同的校验
 */
export function identify(cards) {
  const invalid = { type: ComboType.INVALID };
  if (!Array.isArray(cards) || cards.length === 0) return invalid;

  const counts = rankCounts(cards);
  const ranks = [...counts.keys()].sort((a, b) => a - b);
  const size = cards.length;

  // 王炸
  if (size === 2 && counts.get(RANK_JOKER_S) === 1 && counts.get(RANK_JOKER_B) === 1) {
    return { type: ComboType.ROCKET, mainRank: RANK_JOKER_B, length: 1 };
  }
  if (size === 1) return { type: ComboType.SINGLE, mainRank: ranks[0], length: 1 };
  if (size === 2 && counts.get(ranks[0]) === 2) {
    return { type: ComboType.PAIR, mainRank: ranks[0], length: 1 };
  }
  if (size === 3 && counts.get(ranks[0]) === 3) {
    return { type: ComboType.TRIPLET, mainRank: ranks[0], length: 1 };
  }
  if (size === 4) {
    if (counts.get(ranks[0]) === 4) {
      return { type: ComboType.BOMB, mainRank: ranks[0], length: 1 };
    }
    // 三带一
    const triple = ranks.find((r) => counts.get(r) === 3);
    if (triple !== undefined) {
      return { type: ComboType.TRIPLET_ONE, mainRank: triple, length: 1 };
    }
    return invalid;
  }

  const triples = ranks.filter((r) => counts.get(r) === 3);
  const quads = ranks.filter((r) => counts.get(r) === 4);
  const pairs = ranks.filter((r) => counts.get(r) === 2);
  const singles = ranks.filter((r) => counts.get(r) === 1);

  // 三带二
  if (size === 5 && triples.length === 1 && pairs.length === 1) {
    return { type: ComboType.TRIPLET_PAIR, mainRank: triples[0], length: 1 };
  }

  // 顺子：全单张，>=5，连续
  if (size >= 5 && ranks.length === size && isConsecutive(ranks)) {
    return { type: ComboType.STRAIGHT, mainRank: ranks[0], length: size };
  }

  // 连对：全对子，>=3 对，连续
  if (size >= 6 && size % 2 === 0 && pairs.length === ranks.length && pairs.length >= 3 && isConsecutive(pairs)) {
    return { type: ComboType.DOUBLE_STRAIGHT, mainRank: pairs[0], length: pairs.length };
  }

  // 飞机（纯三张连续）
  if (size >= 6 && size % 3 === 0 && triples.length === ranks.length && triples.length >= 2 && isConsecutive(triples)) {
    return { type: ComboType.PLANE, mainRank: triples[0], length: triples.length };
  }

  // 飞机带翅膀：n 组连续三张 + n 张单牌 或 n 个对子
  if (triples.length >= 2 && isConsecutive(triples)) {
    const n = triples.length;
    const wingCards = size - n * 3;
    if (wingCards === n) {
      // 带单：其余牌不能含三张以上同点（四张当两单不合法）
      const rest = ranks.filter((r) => !triples.includes(r));
      const restTotal = rest.reduce((sum, r) => sum + counts.get(r), 0);
      if (restTotal === n && rest.every((r) => counts.get(r) <= 2)) {
        return { type: ComboType.PLANE_WINGS, mainRank: triples[0], length: n, wing: 'single' };
      }
    }
    if (wingCards === n * 2) {
      const rest = ranks.filter((r) => !triples.includes(r));
      if (rest.length === n && rest.every((r) => counts.get(r) === 2)) {
        return { type: ComboType.PLANE_WINGS, mainRank: triples[0], length: n, wing: 'pair' };
      }
    }
  }

  // 四带二：一个四张 + 两单 或 + 一对
  if (quads.length === 1) {
    if (size === 6) {
      const rest = ranks.filter((r) => r !== quads[0]);
      const restTotal = rest.reduce((sum, r) => sum + counts.get(r), 0);
      // 两张单牌，或一对
      if (restTotal === 2 && rest.every((r) => counts.get(r) <= 2)) {
        return { type: ComboType.FOUR_TWO, mainRank: quads[0], length: 1, wing: 'single' };
      }
    }
    if (size === 8) {
      const rest = ranks.filter((r) => r !== quads[0]);
      if (rest.length === 2 && rest.every((r) => counts.get(r) === 2)) {
        return { type: ComboType.FOUR_TWO, mainRank: quads[0], length: 1, wing: 'pair' };
      }
    }
  }

  return invalid;
}

/** 炸弹权重：王炸 > 炸弹 > 其他 */
function bombPower(combo) {
  if (combo.type === ComboType.ROCKET) return 2;
  if (combo.type === ComboType.BOMB) return 1;
  return 0;
}

/**
 * 判断 candidate 能否压过 target。
 * target 为 null 表示自由出牌，任何合法牌型都可以。
 */
export function beats(candidate, target) {
  if (!candidate || candidate.type === ComboType.INVALID) return false;
  if (!target || target.type === ComboType.INVALID) return true;

  const candPower = bombPower(candidate);
  const targetPower = bombPower(target);
  if (candPower !== targetPower) return candPower > targetPower;
  if (candPower > 0) {
    // 同为炸弹（王炸只有一种，不会走到比较）
    return candidate.mainRank > target.mainRank;
  }
  // 普通牌型必须同类型、同长度
  if (candidate.type !== target.type) return false;
  if (candidate.length !== target.length) return false;
  return candidate.mainRank > target.mainRank;
}

/** 校验出牌是否为手牌子集 */
export function isSubset(cards, hand) {
  const pool = hand.slice();
  for (const card of cards) {
    const idx = pool.indexOf(card);
    if (idx === -1) return false;
    pool.splice(idx, 1);
  }
  return true;
}

/** 从手牌移除指定牌 */
export function removeCards(hand, cards) {
  const out = hand.slice();
  for (const card of cards) {
    const idx = out.indexOf(card);
    if (idx !== -1) out.splice(idx, 1);
  }
  return out;
}

/**
 * 枚举手牌中所有能压过 target 的出法（用于 AI 与提示）。
 * 为控制计算量，带牌的翅膀只取最小若干张的组合。
 */
export function findPlays(hand, target) {
  const counts = rankCounts(hand);
  const byRank = new Map();
  for (const card of sortCards(hand)) {
    const rank = cardRank(card);
    if (!byRank.has(rank)) byRank.set(rank, []);
    byRank.get(rank).push(card);
  }
  const ranks = [...byRank.keys()].sort((a, b) => a - b);
  const results = [];

  const push = (cards) => {
    const combo = identify(cards);
    if (combo.type !== ComboType.INVALID && beats(combo, target)) {
      results.push({ cards: sortCards(cards), combo });
    }
  };

  const takeN = (rank, n) => byRank.get(rank).slice(0, n);

  // 单/对/三/炸
  for (const rank of ranks) {
    const n = counts.get(rank);
    push(takeN(rank, 1));
    if (n >= 2) push(takeN(rank, 2));
    if (n >= 3) push(takeN(rank, 3));
    if (n >= 4) push(takeN(rank, 4));
  }

  // 王炸
  if (counts.get(RANK_JOKER_S) && counts.get(RANK_JOKER_B)) {
    push([52, 53]);
  }

  // 三带一 / 三带二
  for (const rank of ranks) {
    if (counts.get(rank) < 3) continue;
    const triple = takeN(rank, 3);
    for (const other of ranks) {
      if (other === rank) continue;
      push([...triple, ...takeN(other, 1)]);
      if (counts.get(other) >= 2) push([...triple, ...takeN(other, 2)]);
    }
  }

  // 顺子
  const straightRanks = ranks.filter((r) => r <= MAX_STRAIGHT_RANK);
  for (let i = 0; i < straightRanks.length; i++) {
    const seq = [straightRanks[i]];
    for (let j = i + 1; j < straightRanks.length; j++) {
      if (straightRanks[j] !== seq[seq.length - 1] + 1) break;
      seq.push(straightRanks[j]);
      if (seq.length >= 5) push(seq.map((r) => byRank.get(r)[0]));
    }
  }

  // 连对
  const pairRanks = ranks.filter((r) => counts.get(r) >= 2 && r <= MAX_STRAIGHT_RANK);
  for (let i = 0; i < pairRanks.length; i++) {
    const seq = [pairRanks[i]];
    for (let j = i + 1; j < pairRanks.length; j++) {
      if (pairRanks[j] !== seq[seq.length - 1] + 1) break;
      seq.push(pairRanks[j]);
      if (seq.length >= 3) push(seq.flatMap((r) => takeN(r, 2)));
    }
  }

  // 飞机（纯 + 带单 + 带对）
  const tripleRanks = ranks.filter((r) => counts.get(r) >= 3 && r <= MAX_STRAIGHT_RANK);
  for (let i = 0; i < tripleRanks.length; i++) {
    const seq = [tripleRanks[i]];
    for (let j = i + 1; j < tripleRanks.length; j++) {
      if (tripleRanks[j] !== seq[seq.length - 1] + 1) break;
      seq.push(tripleRanks[j]);
      if (seq.length < 2) continue;
      const body = seq.flatMap((r) => takeN(r, 3));
      push(body);
      const n = seq.length;
      // 翅膀取剩余最小的牌
      const restSingles = ranks.filter((r) => !seq.includes(r)).flatMap((r) => byRank.get(r));
      if (restSingles.length >= n) push([...body, ...restSingles.slice(0, n)]);
      const restPairs = ranks.filter((r) => !seq.includes(r) && counts.get(r) >= 2);
      if (restPairs.length >= n) {
        push([...body, ...restPairs.slice(0, n).flatMap((r) => takeN(r, 2))]);
      }
    }
  }

  // 四带二
  for (const rank of ranks) {
    if (counts.get(rank) < 4) continue;
    const quad = takeN(rank, 4);
    const others = ranks.filter((r) => r !== rank);
    const singles = others.flatMap((r) => byRank.get(r));
    if (singles.length >= 2) push([...quad, ...singles.slice(0, 2)]);
    const pairs = others.filter((r) => counts.get(r) >= 2);
    if (pairs.length >= 2) push([...quad, ...pairs.slice(0, 2).flatMap((r) => takeN(r, 2))]);
  }

  // 去重（同一组牌可能被多路径生成）
  const seen = new Set();
  return results.filter((item) => {
    const key = item.cards.join(',');
    if (seen.has(key)) return false;
    seen.add(key);
    return true;
  });
}

/** 手牌评估分：用于叫地主决策，越大越强 */
export function handStrength(hand) {
  const counts = rankCounts(hand);
  let score = 0;
  if (counts.get(RANK_JOKER_B)) score += 8;
  if (counts.get(RANK_JOKER_S)) score += 6;
  if (counts.get(RANK_JOKER_S) && counts.get(RANK_JOKER_B)) score += 6; // 王炸
  for (const [rank, n] of counts) {
    if (n === 4) score += 12;
    if (rank === RANK_TWO) score += n * 3;
    if (rank === 11) score += n * 1.5; // A
  }
  return score;
}
