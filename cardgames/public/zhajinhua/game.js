import { cardElement, backElement, sortCardsForDisplay, socketPathFor, launchPayload } from '../shared/cards.js';

const socket = io({ path: socketPathFor('zhajinhua'), transports: ['websocket','polling'] });
window.notifyGameExit = () => socket.connected && socket.emit('match:leave');
const $ = (id) => document.getElementById(id);
let state = null;
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
      <div class="seat-copy"><strong>${state.seatNames[seat]}${state.active[seat] ? '' : ' · 已弃牌'}</strong>
      <span>桌内 ${state.stacks[seat]} · 已下 ${state.totalBets[seat]}</span></div>`;
    const fan = document.createElement('div');
    fan.style.display = 'flex';
    for (let index=0; index<3; index++) fan.appendChild(backElement(true));
    box.appendChild(fan); root.appendChild(box);
  });
}
function renderHand() {
  const root = $('hand'); root.innerHTML = '';
  if (state.myHand.length) sortCardsForDisplay(state.myHand)
    .forEach((card) => root.appendChild(cardElement(card)));
  else for (let index=0; index<state.myHandCount; index++) root.appendChild(backElement());
}
function renderCenter() {
  const root = $('centerCards'); root.innerHTML = '';
  if (state.phase !== 'finished') return;
  state.revealedHands.forEach((cards, seat) => {
    const group = document.createElement('div');
    group.className = 'showdown';
    const name = document.createElement('strong');
    name.textContent = state.seatNames[seat];
    group.appendChild(name);
    sortCardsForDisplay(cards)
      .forEach((card) => group.appendChild(cardElement(card, { small: true })));
    root.appendChild(group);
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
  if (state.current !== state.seat) { message(`${state.seatNames[state.current]} 正在决策…`); return; }
  if (!state.looked[state.seat]) button('看牌', 'ghost', () => send('look'));
  if (state.canCheck) button('过牌', 'ghost', () => send('check'));
  else button(`跟注 ${state.requiredCall}`, '', () => send('call'));
  state.betLevels.forEach((amount) => button(`加 ${amount}`, '', () => send('bet', { amount })));
  state.compareTargets.forEach((target) => button(`比 ${state.seatNames[target]}`, 'ghost', () => send('compare', { target })));
  button('弃牌', 'danger', () => send('fold'));
  message(`桌内筹码 ${state.stacks[state.seat]} · 已投入 ${state.totalBets[state.seat]}`);
}
function showResult() {
  const result = state.result; if (!result) return;
  $('resultTitle').textContent = result.playerWon ? '你赢下底池' : `${result.winnerName} 赢下底池`;
  $('resultBody').textContent = `${result.reason} · ${result.handName}`;
  $('resultScore').textContent = `${result.score > 0 ? '+' : ''}${result.score}`;
  $('veil').classList.add('show');
}
function render() {
  $('tableNo').textContent = String(state.tableNo).padStart(4,'0');
  $('round').textContent = `${state.round}/${state.maxRounds}`;
  $('pot').textContent = state.pot;
  $('wallet').textContent = Number(state.walletBalance || 0).toLocaleString('zh-CN');
  updateTurnTimer(state);
  renderOpponents(); renderCenter(); renderHand(); renderActions();
  if (state.phase === 'finished') showResult();
}
$('again').onclick = () => {
  $('veil').classList.remove('show');
  socket.emit('match:ready', {}, (result) => message(result?.ok ? '已准备，等待同桌玩家' : result?.message || '结算中'));
};
socket.on('connect', () => socket.emit('match:join', launchPayload('zhajinhua'), (result) => {
  if (!result?.ok) return message(result?.message || '匹配失败');
  message(`已进入第 ${String(result.tableNo).padStart(4,'0')} 桌，匹配中 ${result.players.length}/${result.requiredPlayers}`);
}));
socket.on('match:state', (match) => message(match.status === 'starting' ? '玩家已齐，正在收取入桌金…' : `匹配中 ${match.players.length}/${match.requiredPlayers}`));
socket.on('match:error', (error) => message(error?.message || '暂时无法开局'));
socket.on('game:state', (next) => { state = next; render(); });
socket.on('disconnect', () => message('连接中断，正在重连…'));
