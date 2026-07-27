(function () {
  function isBackMessage(payload) {
    const value = payload?.detail?.data ?? payload?.data ?? payload;
    const messages = Array.isArray(value) ? value : [value];
    return messages.some((item) => item?.type === 'claw:minigame-back' || item?.action === 'back');
  }

  window.returnToPlatform = function returnToPlatform() {
    if (!window.__clawLeavingGame) {
      window.__clawLeavingGame = true;
      try { window.notifyGameExit?.(); } catch {}
    }
    try {
      if (window.parent && window.parent !== window) {
        window.parent.postMessage({ type: 'claw:minigame-back', action: 'back' }, '*');
        return;
      }
    } catch {}

    try {
      if (window.uni?.postMessage) {
        window.uni.postMessage({ data: { type: 'claw:minigame-back', action: 'back' } });
        return;
      }
    } catch {}

    try {
      if (window.plus?.webview) {
        const current = window.plus.webview.currentWebview();
        const host = current?.parent?.() || current?.opener?.();
        if (host?.evalJS) {
          host.evalJS("uni.navigateBack({ delta: 1 })");
          return;
        }
      }
    } catch {}

    if (history.length > 1) {
      history.back();
      return;
    }
    location.replace('/h5/#/pages/minigame/index');
  };

  window.addEventListener('message', (event) => {
    if (isBackMessage(event)) window.returnToPlatform();
  });
})();
