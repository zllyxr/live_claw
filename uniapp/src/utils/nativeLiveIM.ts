import { firstInfo } from "@/api/client";
import { API_HOST } from "@/constants/config";
import type { SessionState } from "@/types/api";
import { getSession, onSessionChange } from "@/utils/session";
import { t } from "@/i18n";

type JoinInfo = {
  conversation_id?: string;
  websocket?: string;
};

type NativeMessage = {
  id?: string;
  conversation_id?: string;
  sequence?: number;
  client_message_id?: string;
  sender_user_id?: string | number;
  message_type?: number;
  text_content?: string;
  metadata?: Record<string, unknown>;
  sender_name?: string;
  sender_avatar?: string;
  created_at?: number;
};

type ServerMessage = {
  type?: "ready" | "message" | "ack" | "error";
  code?: number;
  message?: string;
  client_message_id?: string;
  data?: NativeMessage;
};

type PendingAck = {
  resolve: (value: { payload: Record<string, unknown> }) => void;
  reject: (reason: Error) => void;
  timer: ReturnType<typeof setTimeout>;
};

export type NativeLiveConnectionState =
  | "idle"
  | "connecting"
  | "ready"
  | "reconnecting"
  | "offline";

export type NativeLiveConnectionSnapshot = {
  state: NativeLiveConnectionState;
  conversationID: string;
  reconnectAttempt: number;
};

type HistoryResponse = {
  items?: NativeMessage[];
};

const messageListeners = new Map<
  string,
  Set<(payload: Record<string, unknown>) => void>
>();
const connectionListeners = new Set<
  (snapshot: NativeLiveConnectionSnapshot) => void
>();
const pending = new Map<string, PendingAck>();
const seenIDs = new Map<string, Set<string>>();
const pendingPayloads = new Map<string, Record<string, unknown>[]>();
const latestSequences = new Map<string, number>();

let socket: UniApp.SocketTask | undefined;
let socketGeneration = 0;
let lifecycleGeneration = 0;
let activeConversationID = "";
let activeLiveUID = "";
let activeStream = "";
let activeWebsocketPath = "/ws/im";
let ownerAccountKey = sessionKey(getSession());
let connectionState: NativeLiveConnectionState = "idle";
let reconnectTimer: ReturnType<typeof setTimeout> | undefined;
let reconnectAttempt = 0;
let reconnectEnabled = false;
let ready = false;
let serial = 0;
let historySyncGeneration = 0;
let liveMessagesDuringHistory: ServerMessage[] = [];
let cancelActiveConnect: ((reason: Error) => void) | undefined;

function sessionKey(session: SessionState) {
  return `${String(session.uid || "")}:${String(session.token || "")}`;
}

function currentAccountMatches(expected = ownerAccountKey) {
  return sessionKey(getSession()) === expected;
}

function websocketURL(path = "/ws/im") {
  const rawPath = String(path || "/ws/im").trim();
  if (/^wss?:\/\//i.test(rawPath)) {
    return rawPath;
  }
  if (/^https?:\/\//i.test(rawPath)) {
    return rawPath.replace(/^http:/i, "ws:").replace(/^https:/i, "wss:");
  }
  const locationLike = (
    globalThis as unknown as { location?: { origin?: string } }
  ).location;
  const base = String(locationLike?.origin || API_HOST).replace(/\/$/, "");
  if (rawPath.startsWith("//")) {
    return `${base.startsWith("https:") ? "wss:" : "ws:"}${rawPath}`;
  }
  return `${base}/${rawPath.replace(/^\//, "")}`
    .replace(/^http:/i, "ws:")
    .replace(/^https:/i, "wss:");
}

function websocketOrigin(url: string) {
  const httpURL = String(url || "")
    .replace(/^ws:/i, "http:")
    .replace(/^wss:/i, "https:");
  const match = httpURL.match(/^https?:\/\/[^/]+/i);
  return match?.[0] || API_HOST.replace(/\/$/, "");
}

function imURL(path: string) {
  return `${API_HOST.replace(/\/$/, "")}/api/v2/im${path}`;
}

function parseRecord(value: unknown) {
  if (value && typeof value === "object") {
    return value as Record<string, unknown>;
  }
  if (typeof value === "string") {
    try {
      const parsed = JSON.parse(value);
      return parsed && typeof parsed === "object" ? (parsed as Record<string, unknown>) : {};
    } catch {
      return {};
    }
  }
  return {};
}

function setConnectionState(state: NativeLiveConnectionState) {
  connectionState = state;
  const snapshot: NativeLiveConnectionSnapshot = {
    state,
    conversationID: activeConversationID,
    reconnectAttempt
  };
  connectionListeners.forEach((listener) => listener(snapshot));
}

function clearReconnectTimer() {
  if (reconnectTimer) {
    clearTimeout(reconnectTimer);
    reconnectTimer = undefined;
  }
}

function rejectPending(message: string) {
  for (const item of pending.values()) {
    clearTimeout(item.timer);
    item.reject(new Error(message));
  }
  pending.clear();
}

function clearConversationRuntime(conversationID: string) {
  if (!conversationID) {
    return;
  }
  seenIDs.delete(conversationID);
  pendingPayloads.delete(conversationID);
  latestSequences.delete(conversationID);
}

function invalidateSocket(message: string) {
  const cancelConnect = cancelActiveConnect;
  cancelActiveConnect = undefined;
  cancelConnect?.(new Error(message));
  ready = false;
  historySyncGeneration = 0;
  liveMessagesDuringHistory = [];
  socketGeneration += 1;
  const activeSocket = socket;
  socket = undefined;
  rejectPending(message);
  activeSocket?.close({});
}

function rememberIdentity(conversationID: string, ...identities: string[]) {
  const scoped = seenIDs.get(conversationID) || new Set<string>();
  const normalized = identities.map((value) => value.trim()).filter(Boolean);
  if (normalized.some((value) => scoped.has(value))) {
    return false;
  }
  normalized.forEach((value) => scoped.add(value));
  while (scoped.size > 1200) {
    const oldest = scoped.values().next().value;
    if (!oldest) {
      break;
    }
    scoped.delete(oldest);
  }
  seenIDs.set(conversationID, scoped);
  return true;
}

function advanceSequenceCursor(
  conversationID: string,
  messages: NativeMessage[]
) {
  const maximum = messages.reduce(
    (current, message) => Math.max(current, Number(message.sequence || 0)),
    latestSequences.get(conversationID) || 0
  );
  if (maximum > 0) {
    latestSequences.set(conversationID, maximum);
  }
}

function normalizePayload(message: NativeMessage) {
  const payload = parseRecord(message.text_content);
  const messageID = String(message.id || "");
  const eventID = String(payload.event_id || "");
  payload.message_id = messageID;
  payload.event_id = eventID || messageID;
  payload.sequence = Number(message.sequence || 0);
  payload.created_at = Number(message.created_at || 0);
  payload.client_message_id = String(message.client_message_id || "");
  payload.sender_user_id = String(message.sender_user_id || "");
  payload.sender_name = String(message.sender_name || "");
  payload.sender_avatar = String(message.sender_avatar || "");

  const legacyMessages = payload.msg;
  if (Array.isArray(legacyMessages) && legacyMessages.length) {
    const first = parseRecord(legacyMessages[0]);
    if (message.sender_user_id !== undefined && message.sender_user_id !== null) {
      first.uid = String(message.sender_user_id);
    }
    if (message.sender_name) {
      first.uname = message.sender_name;
    }
    if (message.sender_avatar) {
      first.uhead = message.sender_avatar;
    }
    legacyMessages[0] = first;
  }
  return payload;
}

function isReplayableHistoryMessage(message: NativeMessage) {
  const payload = parseRecord(message.text_content);
  const legacyMessages = payload.msg;
  if (!Array.isArray(legacyMessages) || !legacyMessages.length) {
    return false;
  }
  const first = parseRecord(legacyMessages[0]);
  const method = String(first._method_ || "");
  if (method === "SendMsg") {
    return String(first.msgtype || "") === "2";
  }
  if (method === "SendGift") {
    return true;
  }
  if (method === "SystemNot" || method === "warning") {
    return Boolean(first.ct || first.ct_zh || first.ct_en);
  }
  return false;
}

function deliverMessage(message: NativeMessage) {
  const conversationID = String(message.conversation_id || "");
  if (!conversationID) {
    return;
  }
  const payload = normalizePayload(message);
  const messageID = String(message.id || "");
  const eventID = String(payload.event_id || "");
  if (!rememberIdentity(conversationID, messageID, eventID)) {
    return;
  }
  const sequence = Number(message.sequence || 0);
  if (sequence > 0) {
    advanceSequenceCursor(conversationID, [message]);
  }
  const scoped = messageListeners.get(conversationID);
  if (scoped?.size) {
    scoped.forEach((listener) => listener(payload));
    return;
  }
  const backlog = pendingPayloads.get(conversationID) || [];
  backlog.push(payload);
  if (backlog.length > 120) {
    backlog.splice(0, backlog.length - 120);
  }
  pendingPayloads.set(conversationID, backlog);
}

function dispatch(message: ServerMessage) {
  if (message.type === "ack" && message.client_message_id) {
    const item = pending.get(message.client_message_id);
    if (!item) {
      return;
    }
    clearTimeout(item.timer);
    pending.delete(message.client_message_id);
    if (Number(message.code || 0) !== 0) {
      item.reject(new Error(message.message || t("core.liveMessageFailed")));
      return;
    }
    item.resolve({ payload: {} });
    return;
  }
  if (message.type === "message" && message.data?.conversation_id) {
    deliverMessage(message.data);
  }
}

function historyRequest(
  conversationID: string,
  beforeSequence: number,
  limit: number,
  expectedAccount: string,
  expectedLifecycle: number
) {
  const session = getSession();
  if (
    lifecycleGeneration !== expectedLifecycle ||
    sessionKey(session) !== expectedAccount ||
    !session.uid ||
    !session.token
  ) {
    return Promise.reject(new Error(t("core.accountChanged")));
  }
  const query =
    `limit=${encodeURIComponent(String(limit))}` +
    `&before_sequence=${encodeURIComponent(String(Math.max(0, beforeSequence)))}`;
  return new Promise<NativeMessage[]>((resolve, reject) => {
    uni.request({
      url: imURL(
        `/conversations/${encodeURIComponent(conversationID)}/messages?${query}`
      ),
      method: "GET",
      timeout: 12000,
      header: {
        "X-User-ID": String(session.uid),
        Authorization: `Bearer ${session.token}`
      },
      success: (response) => {
        if (
          lifecycleGeneration !== expectedLifecycle ||
          !currentAccountMatches(expectedAccount)
        ) {
          reject(new Error(t("core.accountChanged")));
          return;
        }
        const body = parseRecord(response.data);
        const data = parseRecord(body.data) as HistoryResponse;
        if (
          Number(response.statusCode || 0) < 200 ||
          Number(response.statusCode || 0) >= 300 ||
          Number(body.code || 0) !== 0
        ) {
          reject(new Error(String(body.message || t("core.liveHistoryFailed"))));
          return;
        }
        resolve(Array.isArray(data.items) ? data.items : []);
      },
      fail: (error) => reject(new Error(error.errMsg || t("core.liveHistoryFailed")))
    });
  });
}

async function loadHistory(
  conversationID: string,
  sinceSequence: number,
  expectedAccount: string,
  expectedLifecycle: number
) {
  const collected: NativeMessage[] = [];
  const limit = sinceSequence > 0 ? 100 : 50;
  const maxPages = sinceSequence > 0 ? 10 : 1;
  let beforeSequence = 0;
  for (let page = 0; page < maxPages; page += 1) {
    const items = await historyRequest(
      conversationID,
      beforeSequence,
      limit,
      expectedAccount,
      expectedLifecycle
    );
    collected.push(...items);
    if (sinceSequence <= 0 || items.length < limit) {
      break;
    }
    const sequences = items
      .map((item) => Number(item.sequence || 0))
      .filter((value) => value > 0);
    const minimum = sequences.length ? Math.min(...sequences) : 0;
    if (!minimum || minimum <= sinceSequence || minimum === beforeSequence) {
      break;
    }
    beforeSequence = minimum;
  }
  return collected
    .filter((item) => sinceSequence <= 0 || Number(item.sequence || 0) > sinceSequence)
    .sort((left, right) => {
      const sequenceDiff = Number(left.sequence || 0) - Number(right.sequence || 0);
      if (sequenceDiff) {
        return sequenceDiff;
      }
      return Number(left.created_at || 0) - Number(right.created_at || 0);
    });
}

function scheduleReconnect(expectedLifecycle: number) {
  if (
    !reconnectEnabled ||
    lifecycleGeneration !== expectedLifecycle
  ) {
    return;
  }
  if (!activeLiveUID || !activeStream || !currentAccountMatches()) {
    setConnectionState("offline");
    return;
  }
  if (reconnectTimer) {
    return;
  }
  const sinceSequence = latestSequences.get(activeConversationID) || 0;
  const delay = Math.min(15000, 800 * 2 ** Math.min(reconnectAttempt, 4));
  reconnectAttempt += 1;
  setConnectionState("reconnecting");
  reconnectTimer = setTimeout(async () => {
    reconnectTimer = undefined;
    if (
      lifecycleGeneration !== expectedLifecycle ||
      !reconnectEnabled ||
      !currentAccountMatches()
    ) {
      return;
    }
    try {
      const info = await firstInfo<JoinInfo>("IM.joinLive", {
        liveuid: activeLiveUID,
        stream: activeStream
      });
      if (
        lifecycleGeneration !== expectedLifecycle ||
        !currentAccountMatches()
      ) {
        return;
      }
      const nextConversationID = String(info?.conversation_id || "");
      if (!nextConversationID) {
        throw new Error(t("core.liveGroupRestoreFailed"));
      }
      const previousConversationID = activeConversationID;
      if (
        previousConversationID &&
        previousConversationID !== nextConversationID
      ) {
        clearConversationRuntime(previousConversationID);
      }
      activeConversationID = nextConversationID;
      activeWebsocketPath = String(info?.websocket || "/ws/im");
      setConnectionState("reconnecting");
      await openSocket(
        expectedLifecycle,
        true,
        previousConversationID === nextConversationID ? sinceSequence : 0
      );
    } catch {
      if (!reconnectTimer && !socket) {
        scheduleReconnect(expectedLifecycle);
      }
    }
  }, delay);
}

function openSocket(
  expectedLifecycle: number,
  reconnecting: boolean,
  sinceSequence: number
) {
  const expectedAccount = ownerAccountKey;
  const conversationID = activeConversationID;
  const session = getSession();
  const cancelPreviousConnect = cancelActiveConnect;
  cancelActiveConnect = undefined;
  cancelPreviousConnect?.(new Error(t("core.liveChatReplaced")));
  const previous = socket;
  socket = undefined;
  previous?.close({});
  socketGeneration += 1;
  const expectedSocketGeneration = socketGeneration;
  const socketAddress = websocketURL(activeWebsocketPath);
  const task = uni.connectSocket({
    url: socketAddress,
    // HBuilderX's Android WebSocket adapter otherwise injects
    // `Origin: http://localhost`, which the production same-origin gate
    // correctly rejects. App 2.9.6+ forwards this explicit header.
    header: { Origin: websocketOrigin(socketAddress) },
    complete: () => undefined
  });
  socket = task;
  ready = false;
  setConnectionState(reconnecting ? "reconnecting" : "connecting");

  return new Promise<void>((resolve, reject) => {
    let settled = false;
    let completingReady = false;
    let cancelThisConnect: ((reason: Error) => void) | undefined;
    const isCurrent = () =>
      socket === task &&
      socketGeneration === expectedSocketGeneration &&
      lifecycleGeneration === expectedLifecycle &&
      currentAccountMatches(expectedAccount);
    const releaseConnectWaiter = () => {
      if (cancelActiveConnect === cancelThisConnect) {
        cancelActiveConnect = undefined;
      }
    };
    const timer = setTimeout(() => {
      if (!isCurrent()) {
        return;
      }
      socket = undefined;
      ready = false;
      historySyncGeneration = 0;
      liveMessagesDuringHistory = [];
      task.close({});
      rejectPending(t("core.liveChatDisconnected"));
      setConnectionState("offline");
      cancelThisConnect?.(new Error(t("core.liveChatTimeout")));
      scheduleReconnect(expectedLifecycle);
    }, 12000);
    cancelThisConnect = (reason: Error) => {
      clearTimeout(timer);
      releaseConnectWaiter();
      if (!settled) {
        settled = true;
        reject(reason);
      }
    };
    cancelActiveConnect = cancelThisConnect;

    const fail = (message: string) => {
      if (!isCurrent()) {
        return;
      }
      clearTimeout(timer);
      socket = undefined;
      ready = false;
      historySyncGeneration = 0;
      liveMessagesDuringHistory = [];
      rejectPending(t("core.liveChatDisconnected"));
      cancelThisConnect?.(new Error(message));
      setConnectionState("offline");
      scheduleReconnect(expectedLifecycle);
    };

    task.onOpen(() => {
      if (!isCurrent()) {
        task.close({});
        return;
      }
      task.send({
        data: JSON.stringify({
          type: "auth",
          uid: session.uid,
          token: session.token
        })
      });
    });

    task.onMessage((event) => {
      if (!isCurrent()) {
        return;
      }
      let message: ServerMessage;
      try {
        message = JSON.parse(String(event.data || "{}")) as ServerMessage;
      } catch {
        return;
      }
      if (message.type === "ready" && !completingReady) {
        completingReady = true;
        historySyncGeneration = expectedSocketGeneration;
        liveMessagesDuringHistory = [];
        void loadHistory(
          conversationID,
          reconnecting ? sinceSequence : 0,
          expectedAccount,
          expectedLifecycle
        )
          .catch(() => [] as NativeMessage[])
          .then((history) => {
            if (!isCurrent()) {
              return;
            }
            const queued = liveMessagesDuringHistory
              .map((item) => item.data)
              .filter((item): item is NativeMessage => Boolean(item));
            historySyncGeneration = 0;
            liveMessagesDuringHistory = [];
            advanceSequenceCursor(conversationID, [...history, ...queued]);
            [
              ...history.filter(isReplayableHistoryMessage),
              ...queued
            ]
              .sort((left, right) => {
                const sequenceDiff =
                  Number(left.sequence || 0) - Number(right.sequence || 0);
                if (sequenceDiff) {
                  return sequenceDiff;
                }
                return Number(left.created_at || 0) - Number(right.created_at || 0);
              })
              .forEach(deliverMessage);
            ready = true;
            reconnectAttempt = 0;
            clearTimeout(timer);
            releaseConnectWaiter();
            setConnectionState("ready");
            if (!settled) {
              settled = true;
              resolve();
            }
          });
        return;
      }
      if (message.type === "error" && !ready) {
        fail(message.message || t("core.liveChatAuthFailed"));
        return;
      }
      if (
        message.type === "message" &&
        historySyncGeneration === expectedSocketGeneration
      ) {
        liveMessagesDuringHistory.push(message);
        return;
      }
      dispatch(message);
    });

    task.onError(() => fail(t("core.liveChatConnectFailed")));
    task.onClose(() => fail(t("core.liveChatDisconnected")));
  });
}

export async function connectNativeLive(liveUid: string, stream: string) {
  disconnectNativeLive();
  activeLiveUID = String(liveUid || "").trim();
  activeStream = String(stream || "").trim();
  ownerAccountKey = sessionKey(getSession());
  reconnectEnabled = true;
  lifecycleGeneration += 1;
  const expectedLifecycle = lifecycleGeneration;
  setConnectionState("connecting");
  try {
    const info = await firstInfo<JoinInfo>("IM.joinLive", {
      liveuid: activeLiveUID,
      stream: activeStream
    });
    if (
      lifecycleGeneration !== expectedLifecycle ||
      !currentAccountMatches()
    ) {
      throw new Error(t("core.accountChanged"));
    }
    const conversationID = String(info?.conversation_id || "");
    if (!conversationID) {
      throw new Error(t("core.liveGroupCreateFailed"));
    }
    activeConversationID = conversationID;
    activeWebsocketPath = String(info?.websocket || "/ws/im");
    setConnectionState("connecting");
    await openSocket(expectedLifecycle, false, 0);
    return conversationID;
  } catch (error) {
    if (
      lifecycleGeneration === expectedLifecycle &&
      reconnectEnabled &&
      currentAccountMatches()
    ) {
      scheduleReconnect(expectedLifecycle);
    }
    throw error;
  }
}

export function onNativeLiveConnection(
  listener: (snapshot: NativeLiveConnectionSnapshot) => void
) {
  connectionListeners.add(listener);
  listener({
    state: connectionState,
    conversationID: activeConversationID,
    reconnectAttempt
  });
  return () => connectionListeners.delete(listener);
}

export function onNativeLiveMessage(
  conversationID: string,
  listener: (payload: Record<string, unknown>) => void
) {
  const scoped = messageListeners.get(conversationID) || new Set();
  scoped.add(listener);
  messageListeners.set(conversationID, scoped);
  const backlog = pendingPayloads.get(conversationID) || [];
  backlog.forEach(listener);
  pendingPayloads.delete(conversationID);
  return () => {
    scoped.delete(listener);
    if (!scoped.size) {
      messageListeners.delete(conversationID);
    }
  };
}

export function sendNativeLiveMessage(payload: Record<string, unknown>) {
  if (
    !socket ||
    !ready ||
    !activeConversationID ||
    !currentAccountMatches()
  ) {
    return Promise.reject(new Error(t("core.liveChatNotConnected")));
  }
  const activeSocket = socket;
  serial += 1;
  const clientMessageID = `live_${Date.now()}_${serial}`;
  return new Promise<{ payload: Record<string, unknown> }>((resolve, reject) => {
    const timer = setTimeout(() => {
      pending.delete(clientMessageID);
      reject(new Error(t("core.liveMessageTimeout")));
    }, 10000);
    pending.set(clientMessageID, { resolve, reject, timer });
    activeSocket.send({
      data: JSON.stringify({
        type: "send",
        conversation_id: activeConversationID,
        client_message_id: clientMessageID,
        message_type: 1,
        text_content: JSON.stringify(payload),
        metadata: { kind: "live" }
      }),
      fail: () => {
        const item = pending.get(clientMessageID);
        if (item) {
          clearTimeout(item.timer);
          pending.delete(clientMessageID);
          item.reject(new Error(t("core.liveMessageFailed")));
        }
      }
    });
  });
}

export function disconnectNativeLive() {
  const previousConversationID = activeConversationID;
  reconnectEnabled = false;
  lifecycleGeneration += 1;
  clearReconnectTimer();
  invalidateSocket(t("core.liveChatDisconnected"));
  activeConversationID = "";
  activeLiveUID = "";
  activeStream = "";
  activeWebsocketPath = "/ws/im";
  reconnectAttempt = 0;
  clearConversationRuntime(previousConversationID);
  setConnectionState("idle");
}

onSessionChange((session) => {
  const nextAccountKey = sessionKey(session);
  if (nextAccountKey === ownerAccountKey) {
    return;
  }
  ownerAccountKey = nextAccountKey;
  const previousConversationID = activeConversationID;
  reconnectEnabled = false;
  lifecycleGeneration += 1;
  clearReconnectTimer();
  invalidateSocket(t("core.accountChangedLiveDisconnected"));
  activeConversationID = "";
  activeLiveUID = "";
  activeStream = "";
  activeWebsocketPath = "/ws/im";
  reconnectAttempt = 0;
  messageListeners.clear();
  clearConversationRuntime(previousConversationID);
  setConnectionState("idle");
  connectionListeners.clear();
});
