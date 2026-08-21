import { API_HOST } from "@/constants/config";
import { t } from "@/i18n";
import type {
  ChatGroup,
  ChatGroupApplication,
  ChatGroupMember,
  ChatMessage,
  Conversation
} from "@/types/api";
import {
  cacheIMBlocks,
  cacheIMConversations,
  cacheIMGroup,
  cacheIMGroupApplications,
  cacheIMGroupMembers,
  cacheIMGroups,
  cacheIMMessages,
  markCachedIMMessageDeleted,
  readCachedIMBlocks,
  readCachedIMConversations,
  readCachedIMGroup,
  readCachedIMGroupApplications,
  readCachedIMGroupMembers,
  readCachedIMGroups,
  readCachedIMMessages,
  readDeletedIMMessageIDs,
  removeCachedIMConversation,
  removeCachedIMGroup,
  removeCachedIMMessage,
  setCachedIMBlock
} from "@/utils/imDatabase";
import { getSession, onSessionChange } from "@/utils/session";
import { absolutizeUrl } from "@/utils/url";

export type ChatKind = "single" | "group";
export type IMConnectionState = "idle" | "connecting" | "ready" | "offline";

type NativeMessage = {
  id?: string;
  conversation_id?: string;
  sequence?: number;
  client_message_id?: string;
  sender_user_id?: string | number;
  message_type?: number;
  text_content?: string;
  asset_id?: number;
  metadata?: Record<string, unknown>;
  sender_name?: string;
  sender_avatar?: string;
  created_at?: number;
};

type NativeConversation = {
  id: string;
  conversation_type: number;
  title?: string;
  message_seq?: number;
  last_read_seq?: number;
  unread_count?: number;
  updated_at?: number;
  peer_user_id?: string | number;
  peer_nickname?: string;
  peer_avatar?: string;
  latest_message?: NativeMessage;
};

type NativeGroup = {
  id: string;
  group_no?: string;
  title?: string;
  owner_user_id?: string | number;
  introduction?: string;
  announcement?: string;
  join_policy?: number;
  all_muted?: boolean;
  max_members?: number;
  member_count?: number;
  role?: number;
  created_at?: number;
};

type NativeMember = {
  user_id: string | number;
  nickname?: string;
  avatar?: string;
  role?: number;
  mute_until?: number;
  joined_at?: number;
};

type NativeApplication = {
  id: string;
  conversation_id: string;
  group_name?: string;
  user_id: string | number;
  nickname?: string;
  avatar?: string;
  request_message?: string;
  status?: number;
  created_at?: number;
};

type NativeBlock = {
  user_id: string | number;
  nickname?: string;
  avatar?: string;
  created_at?: number;
};

type NativeEnvelope<T> = {
  code?: number;
  message?: string;
  data?: T;
};

type SocketEnvelope = {
  type?: "ready" | "message" | "ack" | "error";
  code?: number;
  message?: string;
  data?: NativeMessage;
};

type MessageListener = {
  handler: (message: ChatMessage, raw: NativeMessage) => void;
  targetID: string;
  kind: ChatKind;
  conversationID?: string;
};

const conversationIDs = new Map<string, string>();
const sequenceByMessageID = new Map<string, number>();
const latestSequenceByConversation = new Map<string, number>();
const applicationIDs = new Map<string, string>();
const messageListeners = new Set<MessageListener>();
const connectionListeners = new Set<(state: IMConnectionState) => void>();

let socket: UniApp.SocketTask | undefined;
let socketUserID = "";
let socketReady = false;
let socketTask: Promise<void> | undefined;
let reconnectTimer: ReturnType<typeof setTimeout> | undefined;
let reconnectAttempt = 0;
let connectionState: IMConnectionState = "idle";
let activeIMOwner: string | undefined;

function baseURL() {
  return `${API_HOST.replace(/\/$/, "")}/api/v2/im`;
}

function websocketURL() {
  const origin = API_HOST.replace(/\/$/, "")
    .replace(/^https:/i, "wss:")
    .replace(/^http:/i, "ws:");
  return `${origin}/ws/im`;
}

function pathID(value: string) {
  return encodeURIComponent(value.trim());
}

function clipped(value: number, minimum: number, maximum: number) {
  return Math.max(minimum, Math.min(maximum, Math.floor(value || 0)));
}

function nativeRequest<T>(
  path: string,
  method: "GET" | "POST" = "GET",
  data?: Record<string, unknown>
) {
  const activeUserID = ensureIMAccountContext();
  const session = getSession();
  if (
    !session.uid ||
    !session.token ||
    session.uid === "-9999" ||
    session.token === "-9999" ||
    session.uid !== activeUserID
  ) {
    return Promise.reject(new Error(t("core.sessionExpired")));
  }
  return new Promise<T>((resolve, reject) => {
    uni.request({
      url: `${baseURL()}${path}`,
      method,
      data,
      timeout: 15_000,
      header: {
        "Content-Type": "application/json",
        "X-User-ID": session.uid,
        Authorization: `Bearer ${session.token}`
      },
      success: (response) => {
        if (String(getSession().uid || "") !== activeUserID) {
          reject(new Error(t("core.accountChangedRetry")));
          return;
        }
        const body = response.data as NativeEnvelope<T>;
        const status = Number(response.statusCode || 0);
        if (status >= 200 && status < 300 && Number(body?.code || 0) === 0) {
          resolve(body.data as T);
          return;
        }
        reject(new Error(String(body?.message || t("core.imUnavailable"))));
      },
      fail: (error) => {
        if (String(getSession().uid || "") !== activeUserID) {
          reject(new Error(t("core.accountChangedRetry")));
          return;
        }
        reject(new Error(error.errMsg || t("core.imConnectFailed")));
      }
    });
  });
}

function setConnectionState(state: IMConnectionState) {
  if (connectionState === state) {
    return;
  }
  connectionState = state;
  connectionListeners.forEach((listener) => listener(state));
}

function clearReconnectTimer() {
  if (reconnectTimer) {
    clearTimeout(reconnectTimer);
    reconnectTimer = undefined;
  }
}

function scheduleReconnect() {
  if (!messageListeners.size || reconnectTimer) {
    return;
  }
  const delay = Math.min(15_000, 800 * 2 ** Math.min(reconnectAttempt, 4));
  reconnectAttempt += 1;
  reconnectTimer = setTimeout(() => {
    reconnectTimer = undefined;
    void connectSocket().catch(() => scheduleReconnect());
  }, delay);
}

function resetSocket(close = false) {
  clearReconnectTimer();
  socketReady = false;
  socketTask = undefined;
  const activeSocket = socket;
  socket = undefined;
  if (close) {
    activeSocket?.close({});
  }
}

function ensureIMAccountContext() {
  const currentUserID = String(getSession().uid || "");
  if (activeIMOwner === undefined) {
    activeIMOwner = currentUserID;
    return currentUserID;
  }
  if (activeIMOwner === currentUserID) {
    return currentUserID;
  }
  activeIMOwner = currentUserID;
  clearReconnectTimer();
  resetSocket(true);
  socketUserID = "";
  reconnectAttempt = 0;
  conversationIDs.clear();
  sequenceByMessageID.clear();
  latestSequenceByConversation.clear();
  applicationIDs.clear();
  messageListeners.clear();
  setConnectionState("idle");
  return currentUserID;
}

onSessionChange(() => {
  ensureIMAccountContext();
});

function dispatchMessage(message: NativeMessage) {
  const currentUserID = ensureIMAccountContext();
  if (!currentUserID || currentUserID !== socketUserID) {
    return;
  }
  const conversationID = String(message.conversation_id || "");
  const messageID = String(message.id || "");
  if (!conversationID || !messageID) {
    return;
  }
  if (Number(message.sequence || 0) > 0) {
    const sequence = Number(message.sequence);
    sequenceByMessageID.set(messageID, sequence);
    latestSequenceByConversation.set(
      conversationID,
      Math.max(latestSequenceByConversation.get(conversationID) || 0, sequence)
    );
  }
  const mapped = mapOpenIMMessage(message);
  void cacheIMMessages(conversationID, [mapped]);
  messageListeners.forEach((listener) => {
    if (listener.targetID && listener.conversationID !== conversationID) {
      return;
    }
    listener.handler(mapped, message);
  });
}

function connectSocket() {
  const activeUserID = ensureIMAccountContext();
  const session = getSession();
  if (
    !session.uid ||
    !session.token ||
    session.uid === "-9999" ||
    session.token === "-9999" ||
    session.uid !== activeUserID
  ) {
    return Promise.reject(new Error(t("core.sessionExpired")));
  }
  if (socketReady && socket && socketUserID === session.uid) {
    return Promise.resolve();
  }
  if (socketTask && socketUserID === session.uid) {
    return socketTask;
  }
  resetSocket(true);
  socketUserID = session.uid;
  setConnectionState("connecting");
  socketTask = new Promise<void>((resolve, reject) => {
    const task = uni.connectSocket({
      url: websocketURL(),
      complete: () => undefined
    });
    socket = task;
    let settled = false;
    const timer = setTimeout(() => {
      if (settled || socket !== task) {
        return;
      }
      settled = true;
      resetSocket(true);
      setConnectionState("offline");
      reject(new Error(t("core.imTimeout")));
    }, 10_000);

    task.onOpen(() => {
      if (socket !== task || String(getSession().uid || "") !== activeUserID) {
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
      if (socket !== task || String(getSession().uid || "") !== activeUserID) {
        return;
      }
      let envelope: SocketEnvelope;
      try {
        envelope = JSON.parse(String(event.data || "{}")) as SocketEnvelope;
      } catch {
        return;
      }
      if (envelope.type === "ready") {
        socketReady = true;
        reconnectAttempt = 0;
        setConnectionState("ready");
        if (!settled) {
          settled = true;
          clearTimeout(timer);
          resolve();
        }
        return;
      }
      if (envelope.type === "message" && envelope.data) {
        dispatchMessage(envelope.data);
        return;
      }
      if (envelope.type === "error" && !settled) {
        settled = true;
        clearTimeout(timer);
        setConnectionState("offline");
        reject(new Error(envelope.message || t("core.imAuthFailed")));
      }
    });

    task.onError(() => {
      if (socket !== task) {
        return;
      }
      if (!settled) {
        settled = true;
        clearTimeout(timer);
        reject(new Error(t("core.imConnectFailed")));
      }
      resetSocket();
      setConnectionState("offline");
      scheduleReconnect();
    });

    task.onClose(() => {
      if (socket !== task) {
        return;
      }
      clearTimeout(timer);
      if (!settled) {
        settled = true;
        reject(new Error(t("core.imDisconnected")));
      }
      resetSocket();
      setConnectionState("offline");
      scheduleReconnect();
    });
  });
  return socketTask;
}

async function directConversation(peerUserID: string) {
  const normalized = peerUserID.trim();
  const key = `single:${normalized}`;
  const cached = conversationIDs.get(key);
  if (cached) {
    return cached;
  }
  const localConversation = (await readCachedIMConversations()).find(
    (item) =>
      item.conversation_type === "single" &&
      String(item.peer_uid || item.touid || item.uid || "") === normalized
  );
  const localConversationID = String(
    localConversation?.conversationID || localConversation?.id || ""
  );
  if (localConversationID) {
    conversationIDs.set(key, localConversationID);
    return localConversationID;
  }
  const conversation = await nativeRequest<NativeConversation>("/direct", "POST", {
    peer_user_id: normalized
  });
  const mapped = mapConversation(conversation);
  const conversations = await readCachedIMConversations();
  await cacheIMConversations(
    conversations
      .filter((item) => String(item.conversationID || item.id || "") !== conversation.id)
      .concat(mapped)
  );
  return conversation.id;
}

async function oneConversationID(targetID: string, kind: ChatKind) {
  const normalized = targetID.trim();
  if (!normalized) {
    throw new Error(kind === "group" ? t("core.invalidGroupId") : t("core.invalidUserId"));
  }
  if (kind === "group") {
    conversationIDs.set(`group:${normalized}`, normalized);
    return normalized;
  }
  return directConversation(normalized);
}

function latestText(message?: NativeMessage) {
  if (!message?.id) {
    return t("core.noMessagesPreview");
  }
  switch (Number(message.message_type || 0)) {
    case 1:
      return message.text_content || t("core.textMessage");
    case 2:
      return t("core.imageMessage");
    case 3:
      return t("core.voiceMessage");
    case 4:
      return t("core.videoMessage");
    case 5:
      return `${t("core.fileMessage")} ${String(message.metadata?.file_name || "")}`.trim();
    case 100:
      return t("core.groupNoticeMessage");
    default:
      return t("core.newMessage");
  }
}

function mapGroup(item: NativeGroup): ChatGroup {
  return {
    groupID: item.id,
    groupNo: item.group_no || "",
    groupName: item.title || "",
    notification: item.announcement || "",
    introduction: item.introduction || "",
    faceURL: "",
    ownerUserID: String(item.owner_user_id || ""),
    memberCount: Number(item.member_count || 0),
    maxMemberCount: Number(item.max_members || 0),
    status: item.all_muted ? 3 : 0,
    allMuted: Boolean(item.all_muted),
    groupType: 2,
    needVerification: Number(item.join_policy || 1),
    createTime: Number(item.created_at || 0),
    roleLevel: Number(item.role || 0)
  };
}

function mapGroupMember(groupID: string, member: NativeMember): ChatGroupMember {
  return {
    groupID,
    userID: String(member.user_id),
    nickname: member.nickname || "",
    faceURL: member.avatar || "",
    roleLevel: Number(member.role || 10),
    muteEndTime: Number(member.mute_until || 0),
    joinTime: Number(member.joined_at || 0)
  };
}

function mapGroupApplication(application: NativeApplication): ChatGroupApplication {
  applicationIDs.set(`${application.conversation_id}:${application.user_id}`, application.id);
  return {
    groupID: application.conversation_id,
    groupName: application.group_name || "",
    userID: String(application.user_id),
    nickname: application.nickname || "",
    userFaceURL: application.avatar || "",
    reqMsg: application.request_message || "",
    handleResult: Number(application.status || 0),
    reqTime: Number(application.created_at || 0),
    applicationID: application.id
  };
}

function mapBlock(item: NativeBlock) {
  return {
    userID: String(item.user_id),
    nickname: item.nickname || "",
    faceURL: item.avatar || "",
    createTime: Number(item.created_at || 0)
  };
}

export function mapOpenIMMessage(message: NativeMessage): ChatMessage {
  const metadata = message.metadata || {};
  const sourceURL = String(metadata.source_url || "");
  const messageType = Number(message.message_type || 0);
  const currentUserID = getSession().uid;
  return {
    id: message.id,
    client_msg_id: message.id,
    request_id: message.client_message_id,
    server_msg_id: message.id,
    conversation_id: message.conversation_id,
    uid: String(message.sender_user_id || ""),
    from_uid: String(message.sender_user_id || ""),
    content: message.text_content || "",
    image: messageType === 2 ? sourceURL : "",
    voice: messageType === 3 ? sourceURL : "",
    voice_duration: Number(metadata.duration || 0),
    video: messageType === 4 ? sourceURL : "",
    video_cover: String(metadata.cover_url || ""),
    file: messageType === 5 ? sourceURL : "",
    file_name: String(metadata.file_name || ""),
    file_size: Number(metadata.file_size || 0),
    sender_name: message.sender_name || "",
    sender_avatar: message.sender_avatar || "",
    avatar: message.sender_avatar || "",
    avatar_thumb: message.sender_avatar || "",
    group_id: message.conversation_id,
    content_type: messageType,
    system: messageType === 100,
    is_self: String(message.sender_user_id || "") === currentUserID,
    addtime: String(message.created_at || 0),
    sequence: Number(message.sequence || 0),
    metadata
  };
}

export async function ensureOpenIM() {
  await connectSocket();
  return { userID: getSession().uid };
}

export function closeOpenIM() {
  clearReconnectTimer();
  messageListeners.clear();
  resetSocket(true);
  socketUserID = "";
  reconnectAttempt = 0;
  setConnectionState("idle");
}

export function onIMConnectionState(listener: (state: IMConnectionState) => void) {
  connectionListeners.add(listener);
  listener(connectionState);
  return () => connectionListeners.delete(listener);
}

function mapConversation(item: NativeConversation) {
  const kind: ChatKind = item.conversation_type === 2 ? "group" : "single";
  const targetID = kind === "group" ? item.id : String(item.peer_user_id || "");
  conversationIDs.set(`${kind}:${targetID}`, item.id);
  latestSequenceByConversation.set(item.id, Number(item.message_seq || 0));
  return {
    id: item.id,
    conversationID: item.id,
    conversation_type: kind,
    uid: kind === "single" ? targetID : "",
    touid: kind === "single" ? targetID : "",
    groupID: kind === "group" ? item.id : "",
    group_id: kind === "group" ? item.id : "",
    group_name: kind === "group" ? item.title || "" : "",
    user_nicename: kind === "single" ? item.peer_nickname || "" : item.title || "",
    avatar: item.peer_avatar || "",
    peer_uid: targetID,
    peer_nickname: item.peer_nickname || item.title || "",
    peer_avatar: item.peer_avatar || "",
    title: item.title || item.peer_nickname || "",
    unread: Number(item.unread_count || 0),
    unread_count: Number(item.unread_count || 0),
    last_msg: latestText(item.latest_message),
    content: latestText(item.latest_message),
    latest_message_type: Number(item.latest_message?.message_type || 0),
    latest_sender_id: String(item.latest_message?.sender_user_id || ""),
    message_seq: Number(item.message_seq || 0),
    last_read_seq: Number(item.last_read_seq || 0),
    updated_at: Number(item.updated_at || 0),
    addtime: String(item.latest_message?.created_at || item.updated_at || 0)
  } as Conversation;
}

function registerConversation(item: Conversation) {
  const kind: ChatKind = item.conversation_type === "group" ? "group" : "single";
  const targetID =
    kind === "group"
      ? String(item.groupID || item.group_id || item.conversationID || item.id || "")
      : String(item.peer_uid || item.touid || item.uid || "");
  const id = String(item.conversationID || item.id || "");
  if (targetID && id) {
    conversationIDs.set(`${kind}:${targetID}`, id);
    latestSequenceByConversation.set(id, Number(item.message_seq || 0));
  }
  return item;
}

function cachedMessageID(message: ChatMessage) {
  return String(message.server_msg_id || message.client_msg_id || message.id || "");
}

export async function openIMConversations() {
  void connectSocket().catch(() => undefined);
  try {
    const response = await nativeRequest<{ items: NativeConversation[] }>("/conversations");
    const items = (response.items || [])
      .filter((item) => item.conversation_type === 1 || item.conversation_type === 2)
      .map(mapConversation);
    await cacheIMConversations(items);
    return items;
  } catch (error) {
    const cached = (await readCachedIMConversations()).map(registerConversation);
    if (cached.length) {
      return cached;
    }
    throw error;
  }
}

export async function openIMHistory(
  targetID: string,
  kind: ChatKind = "single",
  startClientMsgID = "",
  count = 30
) {
  const conversationID = await oneConversationID(targetID, kind);
  const limit = clipped(count, 20, 100);
  const beforeSequence = startClientMsgID
    ? Number(sequenceByMessageID.get(startClientMsgID) || 0)
    : 0;
  try {
    const response = await nativeRequest<{ items: NativeMessage[] }>(
      `/conversations/${pathID(conversationID)}/messages` +
        `?before_sequence=${beforeSequence}&limit=${limit}`
    );
    const items = response.items || [];
    items.forEach((message) => {
      const messageID = String(message.id || "");
      const sequence = Number(message.sequence || 0);
      if (messageID && sequence > 0) {
        sequenceByMessageID.set(messageID, sequence);
        latestSequenceByConversation.set(
          conversationID,
          Math.max(latestSequenceByConversation.get(conversationID) || 0, sequence)
        );
      }
    });
    const deleted = await readDeletedIMMessageIDs(conversationID);
    const mapped = items
      .filter((message) => !deleted.has(String(message.id || "")))
      .map(mapOpenIMMessage);
    await cacheIMMessages(conversationID, mapped);
    return {
      messages: mapped,
      isEnd: items.length < limit
    };
  } catch (error) {
    const cached = await readCachedIMMessages(conversationID, beforeSequence, limit);
    if (cached.length) {
      cached.forEach((message) => {
        const id = cachedMessageID(message);
        const sequence = Number(message.sequence || 0);
        if (id && sequence) sequenceByMessageID.set(id, sequence);
      });
      return {
        messages: cached,
        isEnd: cached.length < limit
      };
    }
    throw error;
  }
}

async function sendMessage(
  targetID: string,
  kind: ChatKind,
  messageType: number,
  textContent: string,
  metadata: Record<string, unknown> = {}
) {
  const conversationID = await oneConversationID(targetID, kind);
  const message = await nativeRequest<NativeMessage>(
    `/conversations/${pathID(conversationID)}/messages`,
    "POST",
    {
      client_message_id: `uni_${Date.now()}_${Math.random().toString(36).slice(2, 10)}`,
      message_type: messageType,
      text_content: textContent,
      asset_id: 0,
      metadata
    }
  );
  dispatchMessage(message);
  return mapOpenIMMessage(message);
}

export function openIMSendText(targetID: string, content: string, kind: ChatKind = "single") {
  return sendMessage(targetID, kind, 1, content.trim());
}

export function openIMSendImage(targetID: string, sourceURL: string, kind: ChatKind = "single") {
  return sendMessage(targetID, kind, 2, "", { source_url: absolutizeUrl(sourceURL) });
}

export function openIMSendVideo(
  targetID: string,
  sourceURL: string,
  coverURL = "",
  duration = 0,
  size = 0,
  kind: ChatKind = "single"
) {
  return sendMessage(targetID, kind, 4, "", {
    source_url: absolutizeUrl(sourceURL),
    cover_url: absolutizeUrl(coverURL),
    duration: Math.max(0, Math.round(duration)),
    file_size: Math.max(0, Math.round(size))
  });
}

export function openIMSendVoice(
  targetID: string,
  sourceURL: string,
  duration = 0,
  size = 0,
  kind: ChatKind = "single"
) {
  return sendMessage(targetID, kind, 3, "", {
    source_url: absolutizeUrl(sourceURL),
    duration: Math.max(1, Math.round(duration)),
    file_size: Math.max(0, Math.round(size))
  });
}

export function openIMSendFile(
  targetID: string,
  sourceURL: string,
  fileName: string,
  fileSize = 0,
  kind: ChatKind = "single"
) {
  return sendMessage(targetID, kind, 5, "", {
    source_url: absolutizeUrl(sourceURL),
    file_name: fileName.trim() || t("core.chatFile"),
    file_size: Math.max(0, Math.round(fileSize))
  });
}

export async function openIMMarkRead(
  targetID: string,
  kind: ChatKind = "single",
  sequence = 0
) {
  const conversationID = await oneConversationID(targetID, kind);
  const readSequence = Math.max(
    0,
    Math.round(sequence || latestSequenceByConversation.get(conversationID) || 0)
  );
  const result = await nativeRequest(`/conversations/${pathID(conversationID)}/read`, "POST", {
    sequence: readSequence
  });
  const conversations = await readCachedIMConversations();
  await cacheIMConversations(
    conversations.map((item) =>
      String(item.conversationID || item.id || "") === conversationID
        ? { ...item, unread: 0, unread_count: 0, last_read_seq: readSequence }
        : item
    )
  );
  return result;
}

export async function openIMRemoveConversation(targetID: string, kind: ChatKind = "single") {
  const conversationID = await oneConversationID(targetID, kind);
  const result = await nativeRequest(`/conversations/${pathID(conversationID)}/hide`, "POST", {});
  await removeCachedIMConversation(conversationID);
  conversationIDs.delete(`${kind}:${targetID.trim()}`);
  return result;
}

export async function openIMRevokeMessage(
  targetID: string,
  messageID: string,
  kind: ChatKind = "single"
) {
  const conversationID = await oneConversationID(targetID, kind);
  const result = await nativeRequest(
    `/conversations/${pathID(conversationID)}/messages/${pathID(messageID)}/revoke`,
    "POST",
    {}
  );
  await removeCachedIMMessage(conversationID, messageID);
  return result;
}

export async function openIMDeleteLocalMessage(
  targetID: string,
  messageID: string,
  kind: ChatKind = "single"
) {
  const conversationID = await oneConversationID(targetID, kind);
  await markCachedIMMessageDeleted(conversationID, messageID);
  return { deleted: true };
}

export function onOpenIMMessage(
  handler: (message: ChatMessage, raw: NativeMessage) => void,
  targetID = "",
  kind: ChatKind = "single"
) {
  const listener: MessageListener = { handler, targetID: targetID.trim(), kind };
  messageListeners.add(listener);
  if (listener.targetID) {
    void oneConversationID(listener.targetID, kind)
      .then((conversationID) => {
        listener.conversationID = conversationID;
      })
      .catch(() => undefined);
  }
  void connectSocket().catch(() => undefined);
  return () => {
    messageListeners.delete(listener);
    if (!messageListeners.size) {
      clearReconnectTimer();
    }
  };
}

export async function openIMGroups(offset = 0, count = 100) {
  const requested = clipped(count, 1, 500);
  const start = Math.max(0, Math.floor(offset || 0));
  try {
    const items: NativeGroup[] = [];
    let cursor = start;
    while (items.length < requested) {
      const limit = Math.min(200, requested - items.length);
      const response = await nativeRequest<{ items: NativeGroup[] }>(
        `/groups?offset=${cursor}&limit=${limit}`
      );
      const page = response.items || [];
      items.push(...page);
      if (page.length < limit) {
        break;
      }
      cursor += page.length;
    }
    const mapped = items.map(mapGroup);
    if (start === 0) {
      await cacheIMGroups(mapped);
    } else {
      const merged = new Map((await readCachedIMGroups()).map((item) => [item.groupID, item]));
      mapped.forEach((item) => merged.set(item.groupID, item));
      await cacheIMGroups([...merged.values()]);
    }
    return mapped;
  } catch (error) {
    const cached = (await readCachedIMGroups()).slice(start, start + requested);
    if (cached.length) {
      return cached;
    }
    throw error;
  }
}

export async function openIMCreateGroup(groupName: string, memberUserIDs: string[]) {
  const memberIDs = [
    ...new Set(
      memberUserIDs
        .map((id) => id.trim())
        .filter((id) => id && id !== String(getSession().uid))
    )
  ];
  const group = await nativeRequest<NativeConversation>("/groups", "POST", {
    title: groupName.trim(),
    max_members: 500,
    member_ids: memberIDs
  });
  conversationIDs.set(`group:${group.id}`, group.id);
  const mapped = mapGroup({
    id: group.id,
    title: group.title || groupName,
    owner_user_id: getSession().uid,
    member_count: memberIDs.length + 1,
    max_members: 500,
    role: 100
  });
  await cacheIMGroup(mapped);
  return mapped;
}

export async function openIMGetGroup(groupID: string) {
  try {
    const group = mapGroup(await nativeRequest<NativeGroup>(`/groups/${pathID(groupID)}`));
    await cacheIMGroup(group);
    return group;
  } catch (error) {
    const cached = await readCachedIMGroup(groupID);
    if (cached) {
      return cached;
    }
    throw error;
  }
}

export async function openIMGroupMembers(groupID: string, offset = 0, count = 100) {
  const requested = clipped(count, 1, 500);
  const start = Math.max(0, Math.floor(offset || 0));
  try {
    const items: NativeMember[] = [];
    let cursor = start;
    while (items.length < requested) {
      const limit = Math.min(200, requested - items.length);
      const response = await nativeRequest<{ items: NativeMember[] }>(
        `/groups/${pathID(groupID)}/members?offset=${cursor}&limit=${limit}`
      );
      const page = response.items || [];
      items.push(...page);
      if (page.length < limit) {
        break;
      }
      cursor += page.length;
    }
    const mapped = items.map((member) => mapGroupMember(groupID, member));
    if (start === 0) {
      await cacheIMGroupMembers(groupID, mapped);
    } else {
      const merged = new Map(
        (await readCachedIMGroupMembers(groupID)).map((item) => [item.userID, item])
      );
      mapped.forEach((item) => merged.set(item.userID, item));
      await cacheIMGroupMembers(groupID, [...merged.values()]);
    }
    return mapped;
  } catch (error) {
    const cached = (await readCachedIMGroupMembers(groupID)).slice(start, start + requested);
    if (cached.length) {
      return cached;
    }
    throw error;
  }
}

export async function openIMInviteGroupMembers(groupID: string, userIDs: string[]) {
  const unique = [
    ...new Set(
      userIDs
        .map((id) => id.trim())
        .filter((id) => id && id !== String(getSession().uid))
    )
  ];
  let invited = 0;
  for (const userID of unique) {
    try {
      await nativeRequest(`/groups/${pathID(groupID)}/members`, "POST", {
        user_id: userID
      });
      invited += 1;
    } catch (error) {
      if (!invited) {
        throw error;
      }
      throw new Error(t("core.partialInviteFailed", { count: invited }));
    }
  }
  return { invited };
}

export function openIMJoinGroup(groupID: string, message = t("core.requestJoinGroup")) {
  return nativeRequest<{ id?: string; status: number; joined: boolean }>(
    `/groups/${pathID(groupID)}/join`,
    "POST",
    { message: message.trim() }
  );
}

export async function openIMSetGroupInfo(groupID: string, changes: Partial<ChatGroup>) {
  const current = await nativeRequest<NativeGroup>(`/groups/${pathID(groupID)}`);
  const result = await nativeRequest(`/groups/${pathID(groupID)}`, "POST", {
    title: changes.groupName ?? current.title ?? "",
    introduction: changes.introduction ?? current.introduction ?? "",
    announcement: changes.notification ?? current.announcement ?? "",
    join_policy: changes.needVerification ?? current.join_policy ?? 1
  });
  await cacheIMGroup(
    mapGroup({
      ...current,
      title: changes.groupName ?? current.title,
      introduction: changes.introduction ?? current.introduction,
      announcement: changes.notification ?? current.announcement,
      join_policy: changes.needVerification ?? current.join_policy
    })
  );
  return result;
}

export async function openIMSetGroupMemberRole(
  groupID: string,
  userID: string,
  roleLevel: number
) {
  const result = await nativeRequest(
    `/groups/${pathID(groupID)}/members/${pathID(userID)}/role`,
    "POST",
    { role: roleLevel === 60 ? 60 : 10 }
  );
  const members = await readCachedIMGroupMembers(groupID);
  await cacheIMGroupMembers(
    groupID,
    members.map((member) =>
      member.userID === userID ? { ...member, roleLevel: roleLevel === 60 ? 60 : 10 } : member
    )
  );
  return result;
}

export async function openIMMuteGroupMember(
  groupID: string,
  userID: string,
  mutedSeconds: number
) {
  const duration = Math.max(0, Math.round(mutedSeconds));
  const result = await nativeRequest(
    `/groups/${pathID(groupID)}/members/${pathID(userID)}/mute`,
    "POST",
    { duration_seconds: duration }
  );
  const members = await readCachedIMGroupMembers(groupID);
  await cacheIMGroupMembers(
    groupID,
    members.map((member) =>
      member.userID === userID
        ? { ...member, muteEndTime: duration > 0 ? Math.floor(Date.now() / 1000) + duration : 0 }
        : member
    )
  );
  return result;
}

export async function openIMChangeGroupMute(groupID: string, isMute: boolean) {
  const result = await nativeRequest(`/groups/${pathID(groupID)}/all-mute`, "POST", {
    muted: isMute
  });
  const group = await readCachedIMGroup(groupID);
  if (group) {
    await cacheIMGroup({ ...group, allMuted: isMute, status: isMute ? 3 : 0 });
  }
  return result;
}

export async function openIMKickGroupMember(groupID: string, userID: string) {
  const result = await nativeRequest(
    `/groups/${pathID(groupID)}/members/${pathID(userID)}/remove`,
    "POST",
    {}
  );
  const members = await readCachedIMGroupMembers(groupID);
  await cacheIMGroupMembers(
    groupID,
    members.filter((member) => member.userID !== userID)
  );
  const group = await readCachedIMGroup(groupID);
  if (group) {
    await cacheIMGroup({ ...group, memberCount: Math.max(0, Number(group.memberCount || 0) - 1) });
  }
  return result;
}

export async function openIMTransferGroupOwner(groupID: string, newOwnerUserID: string) {
  const result = await nativeRequest(`/groups/${pathID(groupID)}/transfer`, "POST", {
    user_id: newOwnerUserID
  });
  const currentUserID = String(getSession().uid || "");
  const members = await readCachedIMGroupMembers(groupID);
  await cacheIMGroupMembers(
    groupID,
    members.map((member) => {
      if (member.userID === newOwnerUserID) return { ...member, roleLevel: 100 };
      if (member.userID === currentUserID) return { ...member, roleLevel: 60 };
      return member;
    })
  );
  const group = await readCachedIMGroup(groupID);
  if (group) {
    await cacheIMGroup({ ...group, ownerUserID: newOwnerUserID, roleLevel: 60 });
  }
  return result;
}

export async function openIMQuitGroup(groupID: string) {
  const result = await nativeRequest(`/groups/${pathID(groupID)}/leave`, "POST", {});
  await Promise.all([removeCachedIMGroup(groupID), removeCachedIMConversation(groupID)]);
  conversationIDs.delete(`group:${groupID}`);
  return result;
}

export async function openIMDismissGroup(groupID: string) {
  const result = await nativeRequest(`/groups/${pathID(groupID)}/dissolve`, "POST", {});
  await Promise.all([removeCachedIMGroup(groupID), removeCachedIMConversation(groupID)]);
  conversationIDs.delete(`group:${groupID}`);
  return result;
}

export async function openIMGroupApplications(offset = 0, count = 100) {
  const requested = clipped(count, 1, 500);
  const start = Math.max(0, Math.floor(offset || 0));
  try {
    const items: NativeApplication[] = [];
    let cursor = start;
    while (items.length < requested) {
      const limit = Math.min(200, requested - items.length);
      const response = await nativeRequest<{ items: NativeApplication[] }>(
        `/group-applications?offset=${cursor}&limit=${limit}`
      );
      const page = response.items || [];
      items.push(...page);
      if (page.length < limit) {
        break;
      }
      cursor += page.length;
    }
    const mapped = items.map(mapGroupApplication);
    if (start === 0) {
      await cacheIMGroupApplications(mapped);
    } else {
      const merged = new Map(
        (await readCachedIMGroupApplications()).map((item) => [
          String(item.applicationID || `${item.groupID}:${item.userID}`),
          item
        ])
      );
      mapped.forEach((item) =>
        merged.set(String(item.applicationID || `${item.groupID}:${item.userID}`), item)
      );
      await cacheIMGroupApplications([...merged.values()]);
    }
    return mapped;
  } catch (error) {
    const cached = (await readCachedIMGroupApplications()).slice(start, start + requested);
    cached.forEach((item) => {
      if (item.applicationID) {
        applicationIDs.set(`${item.groupID}:${item.userID}`, item.applicationID);
      }
    });
    if (cached.length) {
      return cached;
    }
    throw error;
  }
}

export async function openIMHandleGroupApplication(
  groupID: string,
  fromUserID: string,
  accept: boolean,
  handleMsg = ""
) {
  const applicationID = applicationIDs.get(`${groupID}:${fromUserID}`);
  if (!applicationID) {
    throw new Error(t("core.groupApplicationExpired"));
  }
  const result = await nativeRequest(`/group-applications/${pathID(applicationID)}`, "POST", {
    accept,
    message: handleMsg.trim()
  });
  const applications = await readCachedIMGroupApplications();
  await cacheIMGroupApplications(
    applications.map((item) =>
      item.applicationID === applicationID ? { ...item, handleResult: accept ? 1 : 2 } : item
    )
  );
  return result;
}

export async function openIMBlackList(offset = 0, count = 100) {
  const limit = clipped(count, 1, 200);
  const start = Math.max(0, Math.floor(offset || 0));
  try {
    const response = await nativeRequest<{ items: NativeBlock[] }>(
      `/blocks?offset=${start}&limit=${limit}`
    );
    const mapped = (response.items || []).map(mapBlock);
    if (start === 0) {
      await cacheIMBlocks(mapped);
    } else {
      const merged = new Map((await readCachedIMBlocks()).map((item) => [item.userID, item]));
      mapped.forEach((item) => merged.set(item.userID, item));
      await cacheIMBlocks([...merged.values()]);
    }
    return mapped;
  } catch (error) {
    const cached = (await readCachedIMBlocks()).slice(start, start + limit);
    if (cached.length) {
      return cached;
    }
    throw error;
  }
}

export async function openIMSetBlack(userID: string, blocked: boolean) {
  const result = await nativeRequest(`/blocks/${pathID(userID)}`, "POST", { blocked });
  await setCachedIMBlock(userID, blocked);
  return result;
}

export function createOpenIMCustomMessage(data: unknown) {
  return Promise.resolve({ data });
}

export function sendOpenIMGroupMessage(groupID: string, message: { data?: unknown }) {
  return sendMessage(groupID, "group", 100, JSON.stringify(message.data ?? {}), {
    kind: "custom"
  });
}

export function onOpenIMGroupMessage(groupID: string, handler: (data: unknown) => void) {
  return onOpenIMMessage((message) => {
    if (!message.system) {
      return;
    }
    try {
      handler(JSON.parse(String(message.content || "{}")));
    } catch {
      handler(message.content);
    }
  }, groupID, "group");
}

export async function prepareOpenIMUsers(userIDs: string[]) {
  return {
    prepared: new Set(userIDs.map((id) => id.trim()).filter(Boolean)).size
  };
}

export function pickOpenIMLocalFile() {
  return Promise.resolve("");
}

export async function connectOpenIMLive(liveID: string, stream: string) {
  const response = await nativeRequest<{ conversation_id: string }>("/live/join", "POST", {
    live_user_id: liveID,
    stream
  });
  void connectSocket().catch(() => undefined);
  return response.conversation_id;
}

export async function sendOpenIMLiveMessage(
  _liveID: string,
  _stream: string,
  payload: Record<string, unknown>
) {
  return { payload, serverMsgID: "" };
}
