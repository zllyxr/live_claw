(function installTurnTimer(global) {
  let currentState = null;
  let elementId = 'countdown';

  function render() {
    const element = document.getElementById(elementId);
    if (!element) return;
    const deadline = Number(currentState?.turnDeadline || 0);
    if (!deadline || currentState?.phase === 'finished') {
      element.textContent = '—';
      element.classList.remove('urgent');
      return;
    }
    const seconds = Math.max(0, Math.ceil((deadline - Date.now()) / 1000));
    element.textContent = `${seconds}秒`;
    element.classList.toggle(
      'urgent',
      currentState?.turnSeat === currentState?.seat && seconds <= 5
    );
    const actor = currentState?.seatNames?.[currentState.turnSeat] || '当前玩家';
    element.title = currentState?.turnSeat === currentState?.seat
      ? '您的操作时间，超时将自动托管'
      : `${actor}正在操作`;
  }

  global.updateTurnTimer = function updateTurnTimer(state, targetId = 'countdown') {
    currentState = state;
    elementId = targetId;
    render();
  };

  setInterval(render, 250);
})(window);
