import { cardElement, backElement, sortCardsForDisplay, socketPathFor, launchPayload } from '../shared/cards.js';

const socket = io({ path: socketPathFor('paodekuai'), transports: ['websocket','polling'] });
window.notifyGameExit = () => socket.connected && socket.emit('match:leave');
const $ = (id) => document.getElementById(id);
let state = null;
let selected = new Set();

function message(text) { $('message').textContent = text; }
function send(action, payload = {}) {
  socket.timeout(8000).emit('game:action', { action, ...payload }, (error, result) => {
    if (error || !result?.ok) message(result?.message || '操作失败，请重试');
  });
}
function renderOpponents() {
  const root = $('opponents'); root.innerHTML = '';
  [0,1,2].filter((seat) => seat !== state.seat).forEach((seat) => {
    const box = document.createElement('div');
    box.className = `seat${state.current === seat ? ' active' : ''}`;
    box.innerHTML = `<div class="avatar">${String(state.seatNames[seat]).slice(0,1)}</div>
      <div class="seat-copy"><strong>${state.seatNames[seat]}</strong><span>${state.handCounts[seat]} 张牌</span></div>`;
    root.appendChild(box);
  });
}
function renderPlayed() {
  const root = $('played'); root.innerHTML = '';
  if (!state.lastPlay) {
    $('tableLabel').textContent = state.mustCarryFirst ? '首手必须带黑桃 3' : '新一轮 · 自由出牌';
    return;
  }
  sortCardsForDisplay(state.lastPlay.cards)
    .forEach((card) => root.appendChild(cardElement(card, { small: true })));
  $('tableLabel').textContent = `${state.seatNames[state.lastPlay.seat]} 刚刚出牌`;
}
function renderHand() {
  const root = $('hand'); root.innerHTML = '';
  sortCardsForDisplay(state.myHand).forEach((card) => {
    const element = cardElement(card);
    if (selected.has(card)) element.classList.add('selected');
    element.onclick = () => {
      selected.has(card) ? selected.delete(card) : selected.add(card);
      renderHand(); renderActions();
    };
    root.appendChild(element);
  });
}
function button(text, kind, handler, disabled = false) {
  const element = document.createElement('button');
  element.textContent = text; element.className = kind; element.disabled = disabled; element.onclick = handler;
  $('actions').appendChild(element);
}
function renderActions() {
  $('actions').innerHTML = '';
  if (state.phase === 'finished') return;
  if (state.current !== state.seat) { message(`${state.seatNames[state.current]} 正在出牌…`); return; }
  button('出牌', '', () => {
    if (!selected.size) return message('请先选择要出的牌');
    send('play', { cards: [...selected] });
  }, !selected.size);
  button('不要', 'ghost', () => send('pass'), !state.canPass);
  button('提示', 'ghost', () => {
    if (!state.hints.length) return message('没有能出的牌');
    selected = new Set(state.hints[0]); renderHand(); renderActions();
  }, !state.hints.length);
  message(state.mustCarryFirst ? '你持有黑桃 3，第一手必须带出' : '轮到你出牌');
}
function showResult() {
  if (!state.result) return;
  $('resultTitle').textContent = state.result.playerWon ? '你跑第一！' : `${state.result.winnerName} 获胜`;
  $('resultBody').textContent = '本桌 300 星币奖池已由平台钱包结算';
  $('resultScore').textContent = `${state.result.score > 0 ? '+' : ''}${state.result.score}`;
  $('veil').classList.add('show');
}
function render() {
  $('tableNo').textContent = String(state.tableNo).padStart(4,'0');
  $('wallet').textContent = Number(state.walletBalance || 0).toLocaleString('zh-CN');
  updateTurnTimer(state);
  renderOpponents(); renderPlayed(); renderHand(); renderActions();
  if (state.phase === 'finished') showResult();
}
$('again').onclick = () => {
  $('veil').classList.remove('show');
  socket.emit('match:ready', {}, (result) => message(result?.ok ? '已准备，等待同桌玩家' : result?.message || '结算中'));
};
socket.on('connect', () => socket.emit('match:join', launchPayload('paodekuai'), (result) => {
  if (!result?.ok) return message(result?.message || '匹配失败');
  message(`已进入第 ${String(result.tableNo).padStart(4,'0')} 桌，匹配中 ${result.players.length}/${result.requiredPlayers}`);
}));
socket.on('match:state', (match) => message(match.status === 'starting' ? '玩家已齐，正在收取入桌金…' : `匹配中 ${match.players.length}/${match.requiredPlayers}`));
socket.on('match:error', (error) => message(error?.message || '暂时无法开局'));
socket.on('game:state', (next) => { state = next; selected.clear(); render(); });
socket.on('disconnect', () => message('连接中断，正在重连…'));
