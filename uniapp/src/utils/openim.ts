import IMSDK, {
  GroupJoinSource,
  GroupMemberFilter,
  GroupMemberRole,
  GroupType,
  GroupVerificationType,
  IMEvents,
  IMMethods,
  LogLevel,
  MessageType,
  Platform,
  SessionType,
  type BlackUserItem,
  type ConversationItem,
  type GroupApplicationItem,
  type GroupItem,
  type GroupMemberItem,
  type MessageItem
} from "openim-uniapp-polyfill";
import { API_HOST, CORE_IM_BASE } from "@/constants/config";
import type {
  ChatGroup,
  ChatGroupApplication,
  ChatGroupMember,
  ChatMessage,
  Conversation
} from "@/types/api";
import { getSession } from "@/utils/session";
import { absolutizeUrl } from "@/utils/url";

type OpenIMSession = {
  userID: string;
  token: string;
  expireTimeSeconds: number;
  apiAddr: string;
  wsAddr: string;
};

type LiveSession = { session: OpenIMSession; groupID: string };
type LiveMessageResponse = { payload: Record<string, unknown>; serverMsgID: string };

const system = uni.getSystemInfoSync() as unknown as Record<string, unknown>;
const uniPlatform = String(system.uniPlatform || "web");
const isNativeApp = uniPlatform === "app";
const webSDK = IMSDK.nomalApi as Record<string, (...args: any[]) => Promise<any>>;
let currentUserID = "";
let sdkInitialized = false;
let loginTask: Promise<OpenIMSession> | undefined;

function platformID() {
  if (isNativeApp) {
    const os = String(system.osName || system.platform || "").toLowerCase();
    return os.includes("ios") ? Platform.iOS : Platform.Android;
  }
  return Platform.Web;
}

function absoluteAddress(address: string, websocket = false) {
  if (/^(https?|wss?):\/\//i.test(address)) {
    return address;
  }
  const locationLike = (globalThis as unknown as { location?: Location }).location;
  let origin = locationLike?.origin || API_HOST;
  if (websocket) {
    origin = origin.replace(/^http:/i, "ws:").replace(/^https:/i, "wss:");
  }
  return `${origin.replace(/\/$/, "")}/${address.replace(/^\//, "")}`;
}

function parseValue(value: unknown): any {
  if (typeof value === "string") {
    try {
      return JSON.parse(value);
    } catch {
      return value;
    }
  }
  return value;
}

function unwrap<T>(value: unknown): T {
  const parsed = parseValue(value) as Record<string, unknown>;
  if (parsed && typeof parsed === "object" && "data" in parsed) {
    return parseValue(parsed.data) as T;
  }
  return parsed as T;
}

function requestSession<T>(
  path: "session" | "prepare-users" | "live-session" | "live-message",
  extra: Record<string, unknown> = {}
) {
  const session = getSession();
  return new Promise<T>((resolve, reject) => {
    uni.request({
      url: `${CORE_IM_BASE}/${path}`,
      method: "POST",
      header: { "Content-Type": "application/json" },
      data: { uid: Number(session.uid), token: session.token, platformID: platformID(), ...extra },
      success: (response) => {
        const body = parseValue(response.data) as Record<string, unknown>;
        if (response.statusCode >= 200 && response.statusCode < 300 && Number(body.code || 0) === 0) {
          resolve(body.data as T);
          return;
        }
        reject(new Error(String(body.message || "IM 服务暂不可用")));
      },
      fail: (error) => reject(new Error(error.errMsg || "IM 服务连接失败"))
    });
  });
}

async function nativeCall<T>(method: IMMethods, ...args: unknown[]) {
  return unwrap<T>(await IMSDK.asyncApi(method, IMSDK.uuid(), ...args));
}

async function webCall<T>(method: IMMethods, ...args: unknown[]) {
  const fn = webSDK[method];
  if (typeof fn !== "function") {
    throw new Error(`OpenIM SDK method unavailable: ${method}`);
  }
  return unwrap<T>(await fn(...args));
}

async function payloadCall<T>(method: IMMethods, payload: unknown) {
  return isNativeApp
    ? nativeCall<T>(method, payload)
    : webCall<T>(method, payload);
}

function nativeDataDir() {
  if (!isNativeApp) {
    return Promise.resolve("");
  }
  const runtime = (globalThis as unknown as { plus?: any }).plus;
  if (!runtime?.io) {
    return Promise.resolve("");
  }
  return new Promise<string>((resolve, reject) => {
    runtime.io.requestFileSystem(runtime.io.PRIVATE_DOC, (fileSystem: any) => {
      fileSystem.root.getDirectory("openim", { create: true }, (entry: any) => resolve(entry.fullPath), reject);
    }, reject);
  });
}

async function loginWithSession(session: OpenIMSession) {
  const apiAddr = absoluteAddress(session.apiAddr);
  const wsAddr = absoluteAddress(session.wsAddr, true);
  if (currentUserID === session.userID) {
    return session;
  }
  if (currentUserID && currentUserID !== session.userID) {
    await (isNativeApp ? nativeCall(IMMethods.Logout) : webCall(IMMethods.Logout));
    currentUserID = "";
  }
  if (isNativeApp) {
    if (!sdkInitialized) {
      const dataDir = await nativeDataDir();
      await nativeCall(IMMethods.InitSDK, {
        platformID: platformID(), apiAddr, wsAddr, dataDir, logLevel: LogLevel.Warn,
        isLogStandardOutput: false, logFilePath: dataDir, isExternalExtensions: false
      });
      sdkInitialized = true;
    }
    await nativeCall(IMMethods.Login, { userID: session.userID, token: session.token });
  } else {
    await webCall(IMMethods.Login, {
      userID: session.userID,
      token: session.token,
      apiAddr,
      wsAddr,
      platformID: platformID(),
      logLevel: LogLevel.Warn
    });
  }
  currentUserID = session.userID;
  return session;
}

export async function ensureOpenIM() {
  const uid = getSession().uid;
  if (uid && currentUserID === uid) {
    return { userID: uid } as OpenIMSession;
  }
  if (!loginTask) {
    loginTask = requestSession<OpenIMSession>("session")
      .then(loginWithSession)
      .catch((error) => {
        currentUserID = "";
        throw error;
      })
      .finally(() => {
        loginTask = undefined;
      });
  }
  return loginTask;
}

export async function connectOpenIMLive(liveID: string, stream: string, liveName = "") {
  const response = await requestSession<LiveSession>("live-session", { liveID, stream, liveName });
  await loginWithSession(response.session);
  return response.groupID;
}

export function sendOpenIMLiveMessage(liveID: string, stream: string, payload: Record<string, unknown>) {
  return requestSession<LiveMessageResponse>("live-message", { liveID, stream, payload });
}

export type ChatKind = "single" | "group";

function chatSessionType(kind: ChatKind) {
  return kind === "group" ? SessionType.Group : SessionType.Single;
}

function latestMessage(value: string) {
  const message = parseValue(value) as MessageItem;
  if (!message || typeof message !== "object") {
    return "";
  }
  if (message.contentType === MessageType.TextMessage || message.contentType === MessageType.AtTextMessage) {
    return message.textElem?.content || message.atTextElem?.text || "";
  }
  if (message.contentType === MessageType.PictureMessage) {
    return "[图片]";
  }
  if (message.contentType === MessageType.VoiceMessage) {
    return "[语音]";
  }
  if (message.contentType === MessageType.VideoMessage) {
    return "[视频]";
  }
  if (message.contentType === MessageType.FileMessage) {
    return `[文件] ${message.fileElem?.fileName || ""}`.trim();
  }
  if (message.contentType >= 1000) {
    return "[群通知]";
  }
  return "[新消息]";
}

function mapConversation(item: ConversationItem): Conversation {
  const kind: ChatKind = item.conversationType === SessionType.Group ? "group" : "single";
  return {
    ...item,
    id: item.conversationID,
    conversation_type: kind,
    uid: item.userID,
    touid: item.userID,
    group_id: item.groupID,
    group_name: kind === "group" ? item.showName : "",
    peer_uid: item.userID,
    peer_nickname: item.showName,
    peer_avatar: item.faceURL,
    unread_count: item.unreadCount,
    last_msg: latestMessage(item.latestMsg),
    addtime: String(item.latestMsgSendTime)
  } as unknown as Conversation;
}

export async function openIMConversations() {
  await ensureOpenIM();
  const list = await payloadCall<ConversationItem[]>(IMMethods.GetConversationListSplit, { offset: 0, count: 100 });
  return (Array.isArray(list) ? list : [])
    .filter(
      (item) =>
        (item.conversationType === SessionType.Single || item.conversationType === SessionType.Group) &&
        !String(item.groupID || "").startsWith("claw_live_")
    )
    .map(mapConversation);
}

async function oneConversation(targetID: string, kind: ChatKind) {
  const conversation = await payloadCall<ConversationItem>(IMMethods.GetOneConversation, {
    sourceID: targetID,
    sessionType: chatSessionType(kind)
  });
  return conversation;
}

async function oneConversationID(targetID: string, kind: ChatKind) {
  return (await oneConversation(targetID, kind)).conversationID;
}

export async function prepareOpenIMUsers(userIDs: string[]) {
  const unique = [...new Set(userIDs.map((userID) => Number(userID)).filter((userID) => Number.isInteger(userID) && userID > 0))];
  if (!unique.length) {
    return { prepared: 0 };
  }
  return requestSession<{ prepared: number }>("prepare-users", { userIDs: unique });
}

export async function pickOpenIMLocalFile() {
  const path = await IMSDK.pickFile();
  return String(path || "");
}

function notificationText(message: MessageItem) {
  const raw = parseValue(message.notificationElem?.detail || "");
  if (raw && typeof raw === "object") {
    return String(
      raw.defaultTips ||
        raw.default_tips ||
        raw.groupName ||
        raw.group_name ||
        raw.nickname ||
        "群聊信息已更新"
    );
  }
  return typeof raw === "string" && raw ? raw : "群聊信息已更新";
}

export function mapOpenIMMessage(message: MessageItem): ChatMessage {
  const image = message.pictureElem?.sourcePicture?.url || message.pictureElem?.bigPicture?.url || "";
  const isSystem = Number(message.contentType || 0) >= 1000;
  return {
    id: message.clientMsgID,
    client_msg_id: message.clientMsgID,
    server_msg_id: message.serverMsgID,
    uid: message.sendID,
    from_uid: message.sendID,
    touid: message.recvID,
    group_id: message.groupID,
    content:
      message.textElem?.content ||
      message.atTextElem?.text ||
      message.quoteElem?.text ||
      (isSystem ? notificationText(message) : ""),
    image,
    voice: message.soundElem?.sourceUrl || "",
    voice_duration: Number(message.soundElem?.duration || 0),
    video: message.videoElem?.videoUrl || "",
    video_cover: message.videoElem?.snapshotUrl || "",
    file: message.fileElem?.sourceUrl || "",
    file_name: message.fileElem?.fileName || "",
    file_size: Number(message.fileElem?.fileSize || 0),
    sender_name: message.senderNickname,
    sender_avatar: message.senderFaceUrl,
    avatar: message.senderFaceUrl,
    avatar_thumb: message.senderFaceUrl,
    content_type: message.contentType,
    system: isSystem,
    is_self: message.sendID === currentUserID,
    addtime: String(message.sendTime || message.createTime)
  } as ChatMessage;
}

export async function openIMHistory(
  targetID: string,
  kind: ChatKind = "single",
  startClientMsgID = "",
  count = 30
) {
  await ensureOpenIM();
  if (kind === "single") {
    await prepareOpenIMUsers([targetID]);
  }
  const conversationID = await oneConversationID(targetID, kind);
  const result = await payloadCall<{ messageList?: MessageItem[]; isEnd?: boolean }>(IMMethods.GetAdvancedHistoryMessageList, {
    conversationID,
    startClientMsgID,
    count: Math.max(20, Math.min(100, count))
  });
  return {
    messages: (result.messageList || []).map(mapOpenIMMessage),
    isEnd: Boolean(result.isEnd) || !(result.messageList || []).length
  };
}

async function sendCreatedMessage(
  targetID: string,
  kind: ChatKind,
  message: MessageItem,
  method = IMMethods.SendMessage
) {
  const result = await payloadCall<MessageItem>(method, {
    recvID: kind === "single" ? targetID : "",
    groupID: kind === "group" ? targetID : "",
    message,
    offlinePushInfo: {
      title: kind === "group" ? "群聊消息" : "新消息",
      desc: kind === "group" ? "群聊中有新消息" : "您收到一条新消息",
      ex: ""
    }
  });
  return mapOpenIMMessage(result);
}

export async function openIMSendText(targetID: string, content: string, kind: ChatKind = "single") {
  await ensureOpenIM();
  if (kind === "single") {
    await prepareOpenIMUsers([targetID]);
  }
  const message = isNativeApp
    ? await nativeCall<MessageItem>(IMMethods.CreateTextMessage, content)
    : await webCall<MessageItem>(IMMethods.CreateTextMessage, content);
  return sendCreatedMessage(targetID, kind, message);
}

export async function openIMSendImage(targetID: string, sourceURL: string, kind: ChatKind = "single") {
  await ensureOpenIM();
  if (kind === "single") {
    await prepareOpenIMUsers([targetID]);
  }
  const url = absolutizeUrl(sourceURL);
  const picture = { uuid: IMSDK.uuid(), type: "image", size: 0, width: 0, height: 0, url };
  const params = { sourcePicture: picture, bigPicture: picture, snapshotPicture: picture, sourcePath: url };
  const message = await payloadCall<MessageItem>(IMMethods.CreateImageMessageByURL, params);
  return sendCreatedMessage(targetID, kind, message, IMMethods.SendMessageNotOss);
}

export async function openIMSendVideo(
  targetID: string,
  sourceURL: string,
  coverURL = "",
  duration = 0,
  size = 0,
  kind: ChatKind = "single"
) {
  await ensureOpenIM();
  if (kind === "single") {
    await prepareOpenIMUsers([targetID]);
  }
  const videoURL = absolutizeUrl(sourceURL);
  const snapshotURL = absolutizeUrl(coverURL);
  const message = await payloadCall<MessageItem>(IMMethods.CreateVideoMessageByURL, {
    videoPath: videoURL,
    duration,
    videoType: "video/mp4",
    snapshotPath: snapshotURL,
    videoUUID: IMSDK.uuid(),
    videoUrl: videoURL,
    videoSize: size,
    snapshotUUID: IMSDK.uuid(),
    snapshotSize: 0,
    snapshotUrl: snapshotURL,
    snapshotWidth: 0,
    snapshotHeight: 0
  });
  return sendCreatedMessage(targetID, kind, message, IMMethods.SendMessageNotOss);
}

export async function openIMSendVoice(
  targetID: string,
  sourceURL: string,
  duration = 0,
  size = 0,
  kind: ChatKind = "single"
) {
  await ensureOpenIM();
  if (kind === "single") {
    await prepareOpenIMUsers([targetID]);
  }
  const voiceURL = absolutizeUrl(sourceURL);
  const message = await payloadCall<MessageItem>(IMMethods.CreateSoundMessageByURL, {
    uuid: IMSDK.uuid(),
    soundPath: voiceURL,
    sourceUrl: voiceURL,
    dataSize: size,
    duration
  });
  return sendCreatedMessage(targetID, kind, message, IMMethods.SendMessageNotOss);
}

export async function openIMSendFile(
  targetID: string,
  sourceURL: string,
  fileName: string,
  fileSize = 0,
  kind: ChatKind = "single"
) {
  await ensureOpenIM();
  if (kind === "single") {
    await prepareOpenIMUsers([targetID]);
  }
  const fileURL = absolutizeUrl(sourceURL);
  const message = await payloadCall<MessageItem>(IMMethods.CreateFileMessageByURL, {
    filePath: fileURL,
    fileName,
    uuid: IMSDK.uuid(),
    sourceUrl: fileURL,
    fileSize
  });
  return sendCreatedMessage(targetID, kind, message, IMMethods.SendMessageNotOss);
}

export async function openIMMarkRead(targetID: string, kind: ChatKind = "single") {
  await ensureOpenIM();
  const conversationID = await oneConversationID(targetID, kind);
  return payloadCall<unknown>(IMMethods.MarkConversationMessageAsRead, conversationID);
}

export async function openIMRemoveConversation(targetID: string, kind: ChatKind = "single") {
  await ensureOpenIM();
  const conversationID = await oneConversationID(targetID, kind);
  return payloadCall<unknown>(IMMethods.DeleteConversationAndDeleteAllMsg, conversationID);
}

export async function openIMRevokeMessage(targetID: string, clientMsgID: string, kind: ChatKind = "single") {
  await ensureOpenIM();
  const conversationID = await oneConversationID(targetID, kind);
  return payloadCall<unknown>(IMMethods.RevokeMessage, { conversationID, clientMsgID });
}

export async function openIMDeleteLocalMessage(targetID: string, clientMsgID: string, kind: ChatKind = "single") {
  await ensureOpenIM();
  const conversationID = await oneConversationID(targetID, kind);
  return payloadCall<unknown>(IMMethods.DeleteMessageFromLocalStorage, { conversationID, clientMsgID });
}

export function onOpenIMMessage(
  handler: (message: ChatMessage, raw: MessageItem) => void,
  targetID = "",
  kind: ChatKind = "single"
) {
  const listener = (event: Record<string, unknown>) => {
    const raw = unwrap<MessageItem>(event);
    if (!raw || raw.sessionType !== chatSessionType(kind)) {
      return;
    }
    if (targetID) {
      const matches =
        kind === "group"
          ? raw.groupID === targetID
          : raw.sendID === targetID || raw.recvID === targetID;
      if (!matches) {
        return;
      }
    }
    handler(mapOpenIMMessage(raw), raw);
  };
  IMSDK.subscribe(IMEvents.OnRecvNewMessage, listener);
  return () => IMSDK.unsubscribe(IMEvents.OnRecvNewMessage, listener as () => void);
}

function asChatGroup(item: GroupItem) {
  return item as unknown as ChatGroup;
}

function asChatGroupMember(item: GroupMemberItem) {
  return item as unknown as ChatGroupMember;
}

export async function openIMGroups(offset = 0, count = 100) {
  await ensureOpenIM();
  const groups = await payloadCall<GroupItem[]>(IMMethods.GetJoinedGroupListPage, { offset, count });
  return (Array.isArray(groups) ? groups : [])
    .filter((group) => !String(group.groupID || "").startsWith("claw_live_"))
    .map(asChatGroup);
}

export async function openIMCreateGroup(groupName: string, memberUserIDs: string[]) {
  await ensureOpenIM();
  await prepareOpenIMUsers(memberUserIDs);
  const session = getSession();
  const group = await payloadCall<GroupItem>(IMMethods.CreateGroup, {
    ownerUserID: session.uid,
    memberUserIDs: [...new Set(memberUserIDs.filter((userID) => userID !== session.uid))],
    groupInfo: {
      groupName: groupName.trim(),
      groupType: GroupType.Group,
      introduction: "",
      notification: "",
      faceURL: "",
      needVerification: GroupVerificationType.AllNeed,
      lookMemberInfo: 0,
      applyMemberFriend: 0,
      ex: JSON.stringify({ kind: "claw_chat" })
    }
  });
  return asChatGroup(group);
}

export async function openIMGetGroup(groupID: string) {
  await ensureOpenIM();
  const groups = await payloadCall<GroupItem[]>(IMMethods.GetSpecifiedGroupsInfo, [groupID]);
  const group = (groups || [])[0];
  if (!group) {
    throw new Error("群聊不存在或已解散");
  }
  return asChatGroup(group);
}

export async function openIMGroupMembers(groupID: string, offset = 0, count = 100) {
  await ensureOpenIM();
  const members = await payloadCall<GroupMemberItem[]>(IMMethods.GetGroupMemberList, {
    groupID,
    filter: GroupMemberFilter.All,
    offset,
    count
  });
  return (Array.isArray(members) ? members : []).map(asChatGroupMember);
}

export async function openIMInviteGroupMembers(groupID: string, userIDs: string[]) {
  await ensureOpenIM();
  await prepareOpenIMUsers(userIDs);
  return payloadCall<unknown>(IMMethods.InviteUserToGroup, {
    groupID,
    reason: "邀请加入群聊",
    userIDList: [...new Set(userIDs)]
  });
}

export async function openIMJoinGroup(groupID: string, message = "申请加入群聊") {
  await ensureOpenIM();
  return payloadCall<unknown>(IMMethods.JoinGroup, {
    groupID: groupID.trim(),
    reqMsg: message,
    joinSource: GroupJoinSource.Search,
    ex: ""
  });
}

export async function openIMSetGroupInfo(groupID: string, changes: Partial<ChatGroup>) {
  await ensureOpenIM();
  return payloadCall<unknown>(IMMethods.SetGroupInfo, { groupID, ...changes });
}

export async function openIMSetGroupMemberRole(groupID: string, userID: string, roleLevel: number) {
  await ensureOpenIM();
  return payloadCall<unknown>(IMMethods.SetGroupMemberInfo, {
    groupID,
    userID,
    roleLevel: roleLevel === GroupMemberRole.Admin ? GroupMemberRole.Admin : GroupMemberRole.Normal
  });
}

export async function openIMMuteGroupMember(groupID: string, userID: string, mutedSeconds: number) {
  await ensureOpenIM();
  return payloadCall<unknown>(IMMethods.ChangeGroupMemberMute, { groupID, userID, mutedSeconds });
}

export async function openIMChangeGroupMute(groupID: string, isMute: boolean) {
  await ensureOpenIM();
  return payloadCall<unknown>(IMMethods.ChangeGroupMute, { groupID, isMute });
}

export async function openIMKickGroupMember(groupID: string, userID: string) {
  await ensureOpenIM();
  return payloadCall<unknown>(IMMethods.KickGroupMember, {
    groupID,
    reason: "由群管理员移出群聊",
    userIDList: [userID]
  });
}

export async function openIMTransferGroupOwner(groupID: string, newOwnerUserID: string) {
  await ensureOpenIM();
  return payloadCall<unknown>(IMMethods.TransferGroupOwner, { groupID, newOwnerUserID });
}

export async function openIMQuitGroup(groupID: string) {
  await ensureOpenIM();
  return payloadCall<unknown>(IMMethods.QuitGroup, groupID);
}

export async function openIMDismissGroup(groupID: string) {
  await ensureOpenIM();
  return payloadCall<unknown>(IMMethods.DismissGroup, groupID);
}

export async function openIMGroupApplications(offset = 0, count = 100) {
  await ensureOpenIM();
  const applications = await payloadCall<GroupApplicationItem[]>(
    IMMethods.GetGroupApplicationListAsRecipient,
    { offset, count }
  );
  return (Array.isArray(applications) ? applications : []) as unknown as ChatGroupApplication[];
}

export async function openIMHandleGroupApplication(
  groupID: string,
  fromUserID: string,
  accept: boolean,
  handleMsg = ""
) {
  await ensureOpenIM();
  return payloadCall<unknown>(
    accept ? IMMethods.AcceptGroupApplication : IMMethods.RefuseGroupApplication,
    { groupID, fromUserID, handleMsg }
  );
}

export async function openIMBlackList(offset = 0, count = 100) {
  await ensureOpenIM();
  return payloadCall<BlackUserItem[]>(IMMethods.GetBlackList, { offset, count });
}

export async function openIMSetBlack(userID: string, blocked: boolean) {
  await ensureOpenIM();
  await prepareOpenIMUsers([userID]);
  return blocked
    ? payloadCall<unknown>(IMMethods.AddBlack, { toUserID: userID, ex: "" })
    : payloadCall<unknown>(IMMethods.RemoveBlack, userID);
}

export async function createOpenIMCustomMessage(data: unknown) {
  const params = { data: JSON.stringify(data), extension: "claw.live.v1", description: "Claw live event" };
  return payloadCall<MessageItem>(IMMethods.CreateCustomMessage, params);
}

export async function sendOpenIMGroupMessage(groupID: string, message: MessageItem) {
  return payloadCall<MessageItem>(IMMethods.SendMessage, {
    recvID: "", groupID, message, isOnlineOnly: true,
    offlinePushInfo: { title: "直播消息", desc: "直播间有新消息", ex: "" }
  });
}

export function onOpenIMGroupMessage(groupID: string, handler: (data: unknown) => void) {
  const listener = (event: Record<string, unknown>) => {
    const raw = unwrap<MessageItem>(event);
    if (raw?.groupID !== groupID || raw.contentType !== MessageType.CustomMessage) {
      return;
    }
    handler(parseValue(raw.customElem?.data));
  };
  IMSDK.subscribe(IMEvents.OnRecvNewMessage, listener);
  IMSDK.subscribe(IMEvents.OnRecvOnlineOnlyMessage, listener);
  return () => {
    IMSDK.unsubscribe(IMEvents.OnRecvNewMessage, listener as () => void);
    IMSDK.unsubscribe(IMEvents.OnRecvOnlineOnlyMessage, listener as () => void);
  };
}
