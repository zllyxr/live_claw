(function () {
  class FishingSocket {
    constructor(options = {}) {
      this.options = options;
      this.ws = null;
      this.connected = false;
      this.listeners = new Map();
      this.pending = new Map();
      this.sequence = 0;
      this.reconnectTimer = 0;
      this.manualClose = false;
    }

    on(event, listener) {
      const listeners = this.listeners.get(event) || [];
      listeners.push(listener);
      this.listeners.set(event, listeners);
      return this;
    }

    dispatch(event, payload) {
      for (const listener of this.listeners.get(event) || []) {
        try {
          listener(payload);
        } catch (error) {
          console.error(error);
        }
      }
    }

    connect() {
      if (this.connected || this.ws?.readyState === WebSocket.CONNECTING) return this;
      this.manualClose = false;
      const query = new URLSearchParams(location.search);
      const protocol = location.protocol === "https:" ? "wss:" : "ws:";
      const endpoint = new URL(`${protocol}//${location.host}/ws/game/fishing`);
      for (const key of ["session", "resume"]) {
        const value = query.get(key);
        if (value) endpoint.searchParams.set(key, value);
      }
      try {
        this.ws = new WebSocket(endpoint);
      } catch (error) {
        this.dispatch("connect_error", error);
        this.scheduleReconnect();
        return this;
      }
      this.ws.addEventListener("open", () => {
        this.connected = true;
        this.dispatch("connect");
      });
      this.ws.addEventListener("message", (message) => {
        let payload;
        try {
          payload = JSON.parse(message.data);
        } catch {
          return;
        }
        if (payload?.event === "ack" && payload.requestId) {
          const pending = this.pending.get(payload.requestId);
          if (!pending) return;
          this.pending.delete(payload.requestId);
          clearTimeout(pending.timer);
          pending.callback(null, payload.data);
          return;
        }
        if (payload?.event) this.dispatch(payload.event, payload.data);
      });
      this.ws.addEventListener("error", () => {
        if (!this.connected) this.dispatch("connect_error", new Error("捕鱼服务连接失败"));
      });
      this.ws.addEventListener("close", () => {
        const wasConnected = this.connected;
        this.connected = false;
        this.ws = null;
        if (wasConnected) this.dispatch("disconnect");
        if (!this.manualClose) this.scheduleReconnect();
      });
      return this;
    }

    scheduleReconnect() {
      if (this.manualClose || this.reconnectTimer) return;
      this.reconnectTimer = window.setTimeout(() => {
        this.reconnectTimer = 0;
        this.connect();
      }, 900);
    }

    disconnect() {
      this.manualClose = true;
      clearTimeout(this.reconnectTimer);
      this.reconnectTimer = 0;
      this.ws?.close();
    }

    send(event, data, callback, timeoutMs) {
      if (!this.connected || this.ws?.readyState !== WebSocket.OPEN) {
        if (callback) callback(new Error("捕鱼服务尚未连接"));
        return;
      }
      const requestId = callback ? `${Date.now()}-${++this.sequence}` : "";
      if (callback) {
        const timer = window.setTimeout(() => {
          const pending = this.pending.get(requestId);
          if (!pending) return;
          this.pending.delete(requestId);
          pending.callback(new Error("捕鱼服务响应超时"));
        }, timeoutMs || 5000);
        this.pending.set(requestId, { callback, timer });
      }
      this.ws.send(JSON.stringify({ event, requestId, data: data || {} }));
    }

    emit(event, data, callback) {
      this.send(event, data, callback, 5000);
      return this;
    }

    timeout(timeoutMs) {
      return {
        emit: (event, data, callback) => {
          this.send(event, data, callback, timeoutMs);
          return this;
        }
      };
    }
  }

  window.io = function io(options) {
    return new FishingSocket(options);
  };
})();
