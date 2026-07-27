import * as ddz from './ddz/game.js';
import * as mahjong from './mahjong/game.js';
import * as paodekuai from './paodekuai/game.js';
import * as zhajinhua from './zhajinhua/game.js';

const BOT_NAMES = Object.freeze([
  '星域小北',
  '星域阿哲',
  '星域小鹿',
  '星域青禾',
  '星域川川',
  '星域念安',
  '星域云帆',
  '星域初夏'
]);

function stableNameIndex(kind, tableNo, seat) {
  const source = `${kind}:${tableNo}:${seat}`;
  let hash = 0;
  for (const character of source) {
    hash = (hash * 31 + character.codePointAt(0)) >>> 0;
  }
  return hash % BOT_NAMES.length;
}

export function createBotPlayer({ kind, tableNo, seat, buyIn }) {
  return {
    uid: `bot:${kind}:${tableNo}:${seat}`,
    name: BOT_NAMES[stableNameIndex(kind, tableNo, seat)],
    table: tableNo,
    seat,
    socketId: null,
    balance: buyIn,
    isBot: true
  };
}

export function isBotPlayer(player) {
  return Boolean(player?.isBot);
}

function chooseZhajinhuaAction(game, seat) {
  if (!game.looked[seat] && game.round >= 2) {
    return { action: 'look' };
  }

  const required = game.highestBet - game.roundBets[seat];
  const strength = zhajinhua.evaluateHand(game.hands[seat]).category;
  const stack = game.stacks[seat];

  if (required > stack) {
    return { action: 'fold' };
  }
  if (required > 0) {
    if (strength >= 1 || (required <= 5 && game.round <= 4)) {
      return { action: 'call' };
    }
    return { action: 'fold' };
  }
  if (strength >= 3 && stack >= 5 && game.round >= 3) {
    return { action: 'bet', amount: 5 };
  }
  return { action: 'check' };
}

export function playBotTurn(kind, game, seat) {
  if (kind === 'ddz') {
    ddz.playBotTurn(game, seat);
    return { action: game.phase === 'bidding' ? 'bid' : 'turn' };
  }
  if (kind === 'mahjong' || kind === 'mahjong_red') {
    mahjong.playBotTurn(game, seat);
    return { action: 'turn' };
  }
  if (kind === 'paodekuai') {
    const view = paodekuai.viewFor(game, seat);
    if (view.hints.length > 0) {
      paodekuai.play(game, seat, view.hints[0]);
      return { action: 'play' };
    }
    paodekuai.pass(game, seat);
    return { action: 'pass' };
  }
  if (kind === 'zhajinhua') {
    const action = chooseZhajinhuaAction(game, seat);
    zhajinhua.act(game, seat, action);
    return action;
  }
  return null;
}
