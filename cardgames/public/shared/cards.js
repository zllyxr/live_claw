export const RANKS = ['3','4','5','6','7','8','9','10','J','Q','K','A','2'];
export const SUITS = ['♦','♣','♥','♠'];

export function cardRank(card) {
  const value = Number(card);
  if (value === 52) return 13;
  if (value === 53) return 14;
  return Math.floor(value / 4);
}

export function cardSuit(card) {
  const value = Number(card);
  return value < 52 ? value % 4 : -1;
}

/** 中国牌类游戏常用展示顺序：大王、小王、2、A…3；同点数黑红梅方。 */
export function sortCardsForDisplay(cards) {
  return [...cards].sort((left, right) => {
    const rankDiff = cardRank(right) - cardRank(left);
    return rankDiff || cardSuit(right) - cardSuit(left);
  });
}

export function cardElement(card, options = {}) {
  const element = document.createElement('img');
  element.className = `card${options.small ? ' small' : ''}`;
  element.src = `../assets/gpt/playing-cards-v2/deck/${Number(card)}.webp`;
  element.alt = `${SUITS[cardSuit(card)] || ''}${RANKS[cardRank(card)] || (Number(card) === 53 ? '大王' : '小王')}`;
  element.draggable = false;
  element.dataset.card = String(card);
  return element;
}

export function backElement(small = false) {
  const element = document.createElement('img');
  element.className = `card back${small ? ' small' : ''}`;
  element.src = '../assets/gpt/playing-cards-v2/deck/back.webp';
  element.alt = '牌背';
  element.draggable = false;
  return element;
}

export function socketPathFor(slug) {
  return location.pathname.replace(new RegExp(`/${slug}/?$`), '') + '/socket.io';
}

export function launchPayload(game) {
  const query = new URLSearchParams(location.search);
  return {
    game,
    uid: query.get('uid'),
    name: query.get('name'),
    ts: query.get('ts'),
    sig: query.get('sig'),
    table: query.get('table')
  };
}
