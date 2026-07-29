(function () {
  window.returnToPlatform = async function returnToPlatform() {
    try {
      await window.leaveFishingSession?.();
    } catch {}
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
    if (history.length > 1) history.back();
    else location.replace('/h5/#/pages/minigame/index');
  };
})();
