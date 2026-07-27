/**
 * 麻将规则引擎（推倒胡玩法）
 *
 * 牌的内部表示：整数 0..33
 *   0..8   万 1..9
 *   9..17  条 1..9
 *   18..26 筒 1..9
 *   27..33 风牌 东南西北中发白
 *
 * 采用「推倒胡」简化规则：
 *   - 108 张数牌 + 28 张风牌 = 136 张
 *   - 可碰、可明杠/暗杠，不吃（降低移动端操作复杂度）
 *   - 胡牌型：标准型（4 面子 + 1 将）或 七小对
 *   - 番型：平胡 / 碰碰胡 / 清一色 / 七小对 / 杠加番
 */

export const SUIT_WAN = 0;
export const SUIT_TIAO = 1;
export const SUIT_TONG = 2;
export const SUIT_WIND = 3;
export const RED_CENTER_TILE = 31;

const WIND_NAMES = ['东', '南', '西', '北', '中', '发', '白'];
const SUIT_LABELS = ['万', '条', '筒'];

export function tileSuit(tile) {
  if (tile >= 27) return SUIT_WIND;
  return Math.floor(tile / 9);
}

/** 数牌返回 1..9，风牌返回 0 */
export function tileNumber(tile) {
  if (tile >= 27) return 0;
  return (tile % 9) + 1;
}

export function tileLabel(tile) {
  if (tile >= 27) return WIND_NAMES[tile - 27];
  return tileNumber(tile) + SUIT_LABELS[tileSuit(tile)];
}

export function isWind(tile) {
  return tile >= 27;
}

/** 一副牌：34 种各 4 张 = 136 张 */
export function freshWall() {
  const wall = [];
  for (let tile = 0; tile < 34; tile++) {
    for (let i = 0; i < 4; i++) wall.push(tile);
  }
  return wall;
}

function secureRandom() {
  if (globalThis.crypto?.getRandomValues) {
    const buf = new Uint32Array(1);
    globalThis.crypto.getRandomValues(buf);
    return buf[0] / 4294967296;
  }
  return Math.random();
}

export function shuffle(tiles, random = secureRandom) {
  const out = tiles.slice();
  for (let i = out.length - 1; i > 0; i--) {
    const j = Math.floor(random() * (i + 1));
    [out[i], out[j]] = [out[j], out[i]];
  }
  return out;
}

/** 手牌转成 34 长度计数数组，便于递归判定 */
export function toCounts(tiles) {
  const counts = new Array(34).fill(0);
  for (const tile of tiles) counts[tile]++;
  return counts;
}

export function sortTiles(tiles) {
  return tiles.slice().sort((a, b) => a - b);
}

/**
 * 递归判断计数数组能否全部拆成面子（刻子或顺子）。
 * needSets 为剩余需要的面子数。
 */
function canFormSets(counts, needSets) {
  if (needSets === 0) return counts.every((n) => n === 0);

  // 找第一张还有的牌
  const i = counts.findIndex((n) => n > 0);
  if (i === -1) return false;

  // 试刻子
  if (counts[i] >= 3) {
    counts[i] -= 3;
    if (canFormSets(counts, needSets - 1)) {
      counts[i] += 3;
      return true;
    }
    counts[i] += 3;
  }

  // 试顺子（仅数牌，且不能跨花色边界）
  if (!isWind(i)) {
    const num = tileNumber(i);
    if (num <= 7 && counts[i + 1] > 0 && counts[i + 2] > 0) {
      counts[i]--; counts[i + 1]--; counts[i + 2]--;
      if (canFormSets(counts, needSets - 1)) {
        counts[i]++; counts[i + 1]++; counts[i + 2]++;
        return true;
      }
      counts[i]++; counts[i + 1]++; counts[i + 2]++;
    }
  }

  return false;
}

/**
 * 标准型判定：手牌（不含已碰杠的副露）能否拆成 needSets 个面子 + 1 个将。
 * @param tiles 手牌
 * @param meldCount 已有副露数（碰/杠），每个副露顶掉一个面子
 */
export function isStandardWin(tiles, meldCount = 0) {
  const needSets = 4 - meldCount;
  if (needSets < 0) return false;
  if (tiles.length !== needSets * 3 + 2) return false;

  const counts = toCounts(tiles);
  // 逐个尝试将牌
  for (let pair = 0; pair < 34; pair++) {
    if (counts[pair] < 2) continue;
    counts[pair] -= 2;
    const ok = canFormSets(counts, needSets);
    counts[pair] += 2;
    if (ok) return true;
  }
  return false;
}

/** 七小对：14 张手牌恰好 7 个对子（不能有副露） */
export function isSevenPairs(tiles, meldCount = 0) {
  if (meldCount > 0 || tiles.length !== 14) return false;
  const counts = toCounts(tiles);
  let pairs = 0;
  for (const n of counts) {
    if (n === 0) continue;
    if (n === 2) { pairs++; continue; }
    if (n === 4) { pairs += 2; continue; } // 四张算两对
    return false;
  }
  return pairs === 7;
}

/** 是否可胡 */
export function canWin(tiles, meldCount = 0) {
  return isSevenPairs(tiles, meldCount) || isStandardWin(tiles, meldCount);
}

function canFormSetsWithWildcards(counts, needSets, wilds, memo) {
  const key = `${needSets}|${wilds}|${counts.join(',')}`;
  if (memo.has(key)) return memo.get(key);
  if (needSets === 0) {
    const ok = wilds === 0 && counts.every((n) => n === 0);
    memo.set(key, ok);
    return ok;
  }

  const tile = counts.findIndex((n) => n > 0);
  if (tile === -1) {
    const ok = wilds === needSets * 3;
    memo.set(key, ok);
    return ok;
  }

  // 刻子：同一种牌可以用 1~3 张实体牌，再由红中补齐。
  for (let real = Math.min(3, counts[tile]); real >= 1; real--) {
    const missing = 3 - real;
    if (missing > wilds) continue;
    counts[tile] -= real;
    if (canFormSetsWithWildcards(counts, needSets - 1, wilds - missing, memo)) {
      counts[tile] += real;
      memo.set(key, true);
      return true;
    }
    counts[tile] += real;
  }

  // 顺子：红中可以补顺子里的任一位置，所以需要检查包含当前牌的三个起点。
  if (!isWind(tile)) {
    const suitStart = Math.floor(tile / 9) * 9;
    const firstStart = Math.max(suitStart, tile - 2);
    const lastStart = Math.min(tile, suitStart + 6);
    for (let start = firstStart; start <= lastStart; start++) {
      const used = [];
      let missing = 0;
      for (let offset = 0; offset < 3; offset++) {
        const candidate = start + offset;
        if (counts[candidate] > 0) {
          counts[candidate]--;
          used.push(candidate);
        } else {
          missing++;
        }
      }
      if (used.includes(tile) && missing <= wilds
        && canFormSetsWithWildcards(counts, needSets - 1, wilds - missing, memo)) {
        used.forEach((candidate) => { counts[candidate]++; });
        memo.set(key, true);
        return true;
      }
      used.forEach((candidate) => { counts[candidate]++; });
    }
  }

  memo.set(key, false);
  return false;
}

function isSevenPairsWithWildcards(tiles, meldCount = 0, wildTile = RED_CENTER_TILE) {
  if (meldCount > 0 || tiles.length !== 14) return false;
  const counts = toCounts(tiles);
  let wilds = counts[wildTile];
  counts[wildTile] = 0;
  let pairs = 0;
  let singles = 0;
  for (const count of counts) {
    pairs += Math.floor(count / 2);
    singles += count % 2;
  }
  if (wilds < singles) return false;
  wilds -= singles;
  pairs += singles;
  return wilds % 2 === 0 && pairs + wilds / 2 === 7;
}

/**
 * 红中麻将：四张红中作为万能牌，可替代任意将牌、刻子或顺子中的牌。
 * 副露仍按普通碰/杠计算，红中本身不参与副露。
 */
export function canWinRedCenter(tiles, meldCount = 0, wildTile = RED_CENTER_TILE) {
  const needSets = 4 - meldCount;
  if (needSets < 0 || tiles.length !== needSets * 3 + 2) return false;
  if (isSevenPairsWithWildcards(tiles, meldCount, wildTile)) return true;

  const counts = toCounts(tiles);
  const wilds = counts[wildTile];
  counts[wildTile] = 0;
  const pairChoices = [];
  for (let tile = 0; tile < 34; tile++) {
    if (tile === wildTile) continue;
    if (counts[tile] >= 2) pairChoices.push({ tile, real: 2, wild: 0 });
    if (counts[tile] >= 1 && wilds >= 1) pairChoices.push({ tile, real: 1, wild: 1 });
  }
  if (wilds >= 2) pairChoices.push({ tile: -1, real: 0, wild: 2 });

  for (const pair of pairChoices) {
    if (pair.tile >= 0) counts[pair.tile] -= pair.real;
    const ok = canFormSetsWithWildcards(
      counts,
      needSets,
      wilds - pair.wild,
      new Map()
    );
    if (pair.tile >= 0) counts[pair.tile] += pair.real;
    if (ok) return true;
  }
  return false;
}

export function waitingTilesRedCenter(tiles, meldCount = 0, wildTile = RED_CENTER_TILE) {
  const out = [];
  const counts = toCounts(tiles);
  for (let tile = 0; tile < 34; tile++) {
    if (counts[tile] >= 4) continue;
    if (canWinRedCenter([...tiles, tile], meldCount, wildTile)) out.push(tile);
  }
  return out;
}

export function scoreRedCenterWin(tiles, melds = [], opts = {}) {
  const usedWildcard = tiles.includes(RED_CENTER_TILE);
  const base = canWin(tiles, melds.length)
    ? scoreWin(tiles, melds, opts)
    : {
      fan: opts.selfDraw ? 2 : 1,
      patterns: opts.selfDraw ? ['平胡', '自摸'] : ['平胡']
    };
  if (!usedWildcard) return base;
  return {
    fan: base.fan + 1,
    patterns: [...base.patterns.filter((pattern) => pattern !== '平胡'), '红中赖子']
  };
}

/**
 * 计算番数与番型。
 * @param tiles 手牌（含刚摸/刚点的那张）
 * @param melds 副露数组 [{type:'peng'|'gang', tile}]
 * @param opts { selfDraw 自摸 }
 */
export function scoreWin(tiles, melds = [], opts = {}) {
  const patterns = [];
  let fan = 1; // 平胡底 1 番

  const meldTiles = melds.flatMap((m) => Array(m.type === 'gang' ? 4 : 3).fill(m.tile));
  const allTiles = [...tiles, ...meldTiles];

  if (isSevenPairs(tiles, melds.length)) {
    patterns.push('七小对');
    fan += 3;
  }

  // 碰碰胡：所有面子都是刻子（无顺子）
  if (!isSevenPairs(tiles, melds.length) && isAllTriplets(tiles, melds)) {
    patterns.push('碰碰胡');
    fan += 2;
  }

  // 清一色：全部同一花色且无风牌
  const suits = new Set(allTiles.map(tileSuit));
  if (suits.size === 1 && !suits.has(SUIT_WIND)) {
    patterns.push('清一色');
    fan += 3;
  }

  const gangCount = melds.filter((m) => m.type === 'gang').length;
  if (gangCount > 0) {
    patterns.push(`杠×${gangCount}`);
    fan += gangCount;
  }

  if (opts.selfDraw) {
    patterns.push('自摸');
    fan += 1;
  }

  if (patterns.length === 0) patterns.push('平胡');
  return { fan, patterns };
}

/** 判断是否碰碰胡（全刻子 + 将） */
function isAllTriplets(tiles, melds) {
  // 副露里有杠/碰都是刻子；手牌部分必须是若干刻子 + 一将
  const counts = toCounts(tiles);
  let pairFound = false;
  for (let i = 0; i < 34; i++) {
    const n = counts[i];
    if (n === 0) continue;
    if (n === 3) continue;
    if (n === 2 && !pairFound) { pairFound = true; continue; }
    return false;
  }
  return pairFound && melds.every((m) => m.type === 'peng' || m.type === 'gang');
}

/** 能否碰：手里有两张一样的 */
export function canPeng(tiles, tile) {
  return toCounts(tiles)[tile] >= 2;
}

/** 能否明杠（别人打出）：手里有三张 */
export function canGangFromDiscard(tiles, tile) {
  return toCounts(tiles)[tile] >= 3;
}

/** 能否暗杠：手里有四张，返回可杠的牌列表 */
export function findConcealedGangs(tiles) {
  const counts = toCounts(tiles);
  const out = [];
  for (let i = 0; i < 34; i++) if (counts[i] === 4) out.push(i);
  return out;
}

/**
 * 听牌检测：返回摸到哪些牌可以胡。
 * 用于「听牌」提示与 AI 决策。
 */
export function waitingTiles(tiles, meldCount = 0) {
  const out = [];
  const counts = toCounts(tiles);
  for (let tile = 0; tile < 34; tile++) {
    if (counts[tile] >= 4) continue; // 四张都在自己手里，不可能再摸到
    if (canWin([...tiles, tile], meldCount)) out.push(tile);
  }
  return out;
}

/**
 * 向听数估算（还差几张成胡），用于 AI 打牌取舍。
 * 简化算法：枚举丢一张后是否听牌，能听牌返回 1；否则用面子/搭子数粗算。
 */
export function shantenEstimate(tiles, meldCount = 0) {
  if (canWin(tiles, meldCount)) return 0;
  // 摸一张能胡 → 听牌
  if (waitingTiles(tiles, meldCount).length > 0) return 1;

  const counts = toCounts(tiles);
  let sets = meldCount;
  let partial = 0;
  const work = counts.slice();

  // 先提刻子
  for (let i = 0; i < 34; i++) {
    while (work[i] >= 3) { work[i] -= 3; sets++; }
  }
  // 再提顺子
  for (let i = 0; i < 34; i++) {
    if (isWind(i)) continue;
    while (tileNumber(i) <= 7 && work[i] > 0 && work[i + 1] > 0 && work[i + 2] > 0) {
      work[i]--; work[i + 1]--; work[i + 2]--; sets++;
    }
  }
  // 数搭子（对子或临张）
  for (let i = 0; i < 34; i++) {
    if (work[i] >= 2) { work[i] -= 2; partial++; continue; }
    if (!isWind(i) && tileNumber(i) <= 8 && work[i] > 0 && work[i + 1] > 0) {
      work[i]--; work[i + 1]--; partial++;
    }
  }
  const need = Math.max(0, 4 - sets);
  return Math.max(1, need * 2 - Math.min(partial, need));
}
