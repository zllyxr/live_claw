import type { LiveGift, UserProfile } from "@/types/api";
import { getSession } from "@/utils/session";
import { displayUrl } from "@/utils/url";
import {
  connectOpenIMLive,
  onOpenIMGroupMessage,
  sendOpenIMLiveMessage
} from "@/utils/openim";

type SocketMethod =
  | "SendMsg"
  | "SendGift"
  | "SendBarrage"
  | "SystemNot"
  | "KickUser"
  | "ShutUpUser"
  | "setAdmin"
  | "StartEndLive"
  | "disconnect"
  | "requestFans"
  | "BuyGuard"
  | "SendRed"
  | "warning";

export type LiveChatType = "normal" | "system" | "gift" | "enter" | "light" | "redpack";

export interface LiveSocketChat {
  id: string;
  uid?: string;
  name: string;
  content: string;
  type: LiveChatType;
  badge?: string;
  level?: number;
  vipType?: number;
  guardType?: number;
  liangName?: string;
  anchor?: boolean;
  manager?: boolean;
}

export interface LiveSocketGift {
  uid: string;
  name: string;
  avatar: string;
  giftName: string;
  giftIcon: string;
  giftCount: number;
  votes: string;
  raw: Record<string, unknown>;
  chat: LiveSocketChat;
}

export interface LiveSocketEnter {
  user: UserProfile;
  chat: LiveSocketChat;
}

interface LiveSocketOptions {
  liveUid: string;
  stream: string;
  userType?: number;
  guardType?: number;
  isAnchor?: boolean;
  onConnect?: (connected: boolean) => void;
  onChat?: (message: LiveSocketChat) => void;
  onGift?: (gift: LiveSocketGift) => void;
  onEnter?: (enter: LiveSocketEnter) => void;
  onLeave?: (uid: string) => void;
  onKick?: (uid: string) => void;
  onShutUp?: (uid: string, content: string) => void;
  onSetAdmin?: (uid: string, action: number) => void;
  onVotes?: (votes: string) => void;
  onLiveEnd?: (reason: string) => void;
  onFakeFans?: (users: UserProfile[]) => void;
  onError?: (message: string) => void;
}

function asRecord(value: unknown) {
  return value && typeof value === "object" ? (value as Record<string, unknown>) : {};
}

function asText(value: unknown, fallback = "") {
  return value === undefined || value === null ? fallback : String(value);
}

function asNumber(value: unknown, fallback = 0) {
  const next = Number(value);
  return Number.isFinite(next) ? next : fallback;
}

function parseRecord(value: unknown) {
  if (typeof value === "string") {
    try {
      return asRecord(JSON.parse(value));
    } catch {
      return {};
    }
  }
  return asRecord(value);
}

function firstMessage(payload: unknown) {
  const data = parseRecord(payload);
  const list = data.msg;
  if (!Array.isArray(list) || !list.length) {
    return undefined;
  }
  return asRecord(list[0]);
}

function ctText(message: Record<string, unknown>) {
  return asText(message.ct || message.ct_zh || message.ct_en);
}

function userNiceName(user?: UserProfile) {
  return asText(user?.user_nicename || user?.user_nickname || user?.userNiceName || "我");
}

function userVipType(user?: UserProfile) {
  const vip = asRecord(user?.vip);
  return asNumber(vip.type || user?.vip_type, 0);
}

function userLiangName(user?: UserProfile) {
  const liang = asRecord(user?.liang);
  return asText(liang.name || user?.liang_name || user?.liangname || "0");
}

function userAvatar(user?: UserProfile) {
  return displayUrl(asText(user?.avatar_thumb || user?.avatar), "/static/brand/icon-round.webp");
}

function createPayload(message: Record<string, unknown>) {
  return {
    retcode: "000000",
    retmsg: "ok",
    msg: [message]
  };
}

function createChatMessage(message: Record<string, unknown>, type: LiveChatType, content?: string): LiveSocketChat {
  const uid = asText(message.uid || message.id);
  const userType = asNumber(message.usertype, 30);
  const name = asText(message.uname || message.user_nickname || message.user_nicename || "星域用户");
  return {
    id: `${type}-${uid || "system"}-${Date.now()}-${Math.random().toString(16).slice(2)}`,
    uid,
    name: type === "system" ? "系统" : name,
    content: content ?? ctText(message),
    type,
    badge: type === "system" ? "系统" : userType === 40 ? "房管" : asNumber(message.isAnchor, 0) === 1 ? "主播" : undefined,
    level: asNumber(message.level, 0),
    vipType: asNumber(message.vip_type, 0),
    guardType: asNumber(message.guard_type, 0),
    liangName: asText(message.liangname),
    anchor: asNumber(message.isAnchor, 0) === 1 || userType === 50,
    manager: userType === 40
  };
}

function createEnterUser(ct: Record<string, unknown>) {
  return {
    id: asText(ct.id),
    uid: asText(ct.id),
    user_nicename: asText(ct.user_nickname || ct.user_nicename || "星域用户"),
    user_nickname: asText(ct.user_nickname || ct.user_nicename || "星域用户"),
    avatar: asText(ct.avatar),
    avatar_thumb: asText(ct.avatar_thumb || ct.avatar),
    level: asText(ct.level),
    vip: { type: asText(ct.vip_type || "0") },
    liang: { name: asText(ct.liangname || "0") },
    guard_type: asText(ct.guard_type || "0"),
    usertype: asText(ct.usertype || "30")
  } as UserProfile;
}

export class LiveSocketClient {
  private connected = false;
  private options: LiveSocketOptions;
  private groupID = "";
  private stopMessages?: () => void;
  private seenEventIDs = new Set<string>();
  private leaveTask?: Promise<void>;

  constructor(options: LiveSocketOptions) {
    this.options = options;
  }

  async connect() {
    this.disconnect();
    try {
      this.groupID = await connectOpenIMLive(this.options.liveUid, this.options.stream, this.options.stream);
      this.stopMessages = onOpenIMGroupMessage(this.groupID, (payload) => this.handleBroadcasting(payload));
      this.connected = true;
      this.options.onConnect?.(true);
      this.emitConn();
    } catch (error: any) {
      this.connected = false;
      this.options.onConnect?.(false);
      this.options.onError?.(error?.message || "OpenIM 直播群组连接失败");
    }
  }

  disconnect() {
    this.stopMessages?.();
    this.stopMessages = undefined;
    this.groupID = "";
    this.connected = false;
    this.seenEventIDs.clear();
  }

  isConnected() {
    return this.connected;
  }

  leave() {
    if (this.leaveTask) {
      return this.leaveTask;
    }
    if (!this.connected) {
      this.disconnect();
      return Promise.resolve();
    }
    const session = getSession();
    const payload = createPayload({
      _method_: "disconnect" satisfies SocketMethod,
      action: "0",
      msgtype: "0",
      uid: session.uid,
      ct: { id: session.uid }
    });
    this.leaveTask = this.emitBroadcast(payload)
      .catch(() => undefined)
      .then(() => {
        this.disconnect();
        this.leaveTask = undefined;
      });
    return this.leaveTask;
  }

  sendChat(content: string) {
    const user = getSession().user;
    const message = {
      _method_: "SendMsg" satisfies SocketMethod,
      action: "0",
      msgtype: "2",
      usertype: String(this.options.userType || 30),
      isAnchor: this.options.isAnchor ? "1" : "0",
      level: asText(user?.level || "0"),
      uname: userNiceName(user),
      uid: getSession().uid,
      liangname: userLiangName(user),
      vip_type: String(userVipType(user)),
      guard_type: String(this.options.guardType || 0),
      ct: content
    };
    this.emitBroadcast(createPayload(message));
  }

  sendGift(gift: LiveGift, giftToken: string, liveName: string) {
    const user = getSession().user;
    const message = {
      _method_: "SendGift" satisfies SocketMethod,
      action: "0",
      msgtype: "1",
      level: asText(user?.level || "0"),
      uname: userNiceName(user),
      uid: getSession().uid,
      uhead: userAvatar(user),
      evensend: asText(gift.type || "0"),
      liangname: userLiangName(user),
      vip_type: String(userVipType(user)),
      guard_type: String(this.options.guardType || 0),
      ct: giftToken,
      roomnum: this.options.liveUid,
      livename: liveName,
      paintedPath: [],
      paintedWidth: "0",
      paintedHeight: "0"
    };
    this.emitBroadcast(createPayload(message));
  }

  sendKick(toUid: string, toName: string) {
    const user = getSession().user;
    this.emitBroadcast(createPayload({
      _method_: "KickUser" satisfies SocketMethod,
      action: "2",
      msgtype: "4",
      level: asText(user?.level || "0"),
      uname: userNiceName(user),
      uid: getSession().uid,
      touid: toUid,
      toname: toName,
      ct: `${toName}被踢出房间`,
      ct_en: `${toName} was kicked out of the room`
    }));
  }

  sendShutUp(toUid: string, toName: string, type = 1) {
    const user = getSession().user;
    this.emitBroadcast(createPayload({
      _method_: "ShutUpUser" satisfies SocketMethod,
      action: "1",
      msgtype: "4",
      level: asText(user?.level || "0"),
      uname: userNiceName(user),
      uid: getSession().uid,
      touid: toUid,
      toname: toName,
      ct: `${toName}${type === 0 ? "被永久禁言" : "被本场禁言"}`,
      ct_en: `${toName}${type === 0 ? " is permanently banned" : " has been banned from this site"}`
    }));
  }

  sendSetAdmin(action: number, toUid: string, toName: string) {
    const user = getSession().user;
    this.emitBroadcast(createPayload({
      _method_: "setAdmin" satisfies SocketMethod,
      action: String(action),
      msgtype: "1",
      uname: userNiceName(user),
      uid: getSession().uid,
      touid: toUid,
      toname: toName,
      ct: `${toName}${action === 1 ? "被设为管理员" : "被取消管理员"}`,
      ct_en: `${toName}${action === 1 ? " is set as administrator" : " was removed as administrator"}`
    }));
  }

  private emitConn() {
    const session = getSession();
    const user = session.user;
    this.emitBroadcast(createPayload({
      _method_: "SendMsg" satisfies SocketMethod,
      action: "0",
      msgtype: "0",
      uid: session.uid,
      ct: {
        id: session.uid,
        user_nickname: userNiceName(user),
        avatar: userAvatar(user),
        avatar_thumb: userAvatar(user),
        level: asText(user?.level || "0"),
        vip_type: String(userVipType(user)),
        liangname: userLiangName(user),
        guard_type: String(this.options.guardType || 0),
        usertype: String(this.options.userType || 30)
      }
    }));
  }

  private async emitBroadcast(payload: Record<string, unknown>) {
    if (!this.groupID || !this.connected) {
      this.options.onError?.("聊天服务器未连接，请稍后重试");
      return;
    }
    try {
      const result = await sendOpenIMLiveMessage(this.options.liveUid, this.options.stream, payload);
      this.handlePayload(result.payload);
    } catch (error: any) {
      this.options.onError?.(error?.message || "直播消息发送失败");
    }
  }

  private sendRequestFans() {
    // 在线成员由 OpenIM 群组成员列表维护，不再注入虚假观众。
  }

  private handleBroadcasting(payload: unknown) {
    const list = Array.isArray(payload) ? payload : [payload];
    list.forEach((item) => {
      if (item === "stopplay") {
        this.options.onLiveEnd?.("直播间已被关闭");
        return;
      }
      this.handlePayload(item);
    });
  }

  private handlePayload(payload: unknown) {
    const root = parseRecord(payload);
    const eventID = asText(root.event_id);
    if (eventID) {
      if (this.seenEventIDs.has(eventID)) {
        return;
      }
      this.seenEventIDs.add(eventID);
      if (this.seenEventIDs.size > 300) {
        const oldest = this.seenEventIDs.values().next().value;
        if (oldest) {
          this.seenEventIDs.delete(oldest);
        }
      }
    }
    const message = firstMessage(root);
    if (!message) {
      return;
    }
    const method = asText(message._method_) as SocketMethod;
    if (method === "SystemNot" || method === "warning") {
      this.options.onChat?.(createChatMessage(message, "system"));
      return;
    }
    if (method === "KickUser") {
      const content = ctText(message);
      this.options.onChat?.(createChatMessage(message, "system", content));
      this.options.onKick?.(asText(message.touid));
      return;
    }
    if (method === "ShutUpUser") {
      const content = ctText(message);
      this.options.onChat?.(createChatMessage(message, "system", content));
      this.options.onShutUp?.(asText(message.touid), content);
      return;
    }
    if (method === "setAdmin") {
      const content = ctText(message);
      this.options.onChat?.(createChatMessage(message, "system", content));
      this.options.onSetAdmin?.(asText(message.touid), asNumber(message.action, 0));
      return;
    }
    if (method === "StartEndLive") {
      this.options.onLiveEnd?.(ctText(message) || "直播已结束");
      return;
    }
    if (method === "disconnect") {
      const ct = parseRecord(message.ct);
      this.options.onLeave?.(asText(ct.id || message.uid));
      return;
    }
    if (method === "requestFans") {
      this.handleFakeFans(message);
      return;
    }
    if (method === "BuyGuard") {
      this.options.onChat?.(createChatMessage(message, "system", `${asText(message.uname)} 开通了守护`));
      this.options.onVotes?.(asText(message.votestotal));
      return;
    }
    if (method === "SendRed") {
      this.options.onChat?.(createChatMessage(message, "redpack", `${asText(message.uname || "主播")}${ctText(message)}`));
      return;
    }
    if (method === "SendMsg") {
      this.handleSendMsg(root, message);
      return;
    }
    if (method === "SendGift") {
      this.handleSendGift(message);
    }
  }

  private handleSendMsg(root: Record<string, unknown>, message: Record<string, unknown>) {
    const retcode = asText(root.retcode);
    if (retcode === "409002") {
      this.options.onError?.("你已被禁言");
      return;
    }
    const msgtype = asText(message.msgtype);
    if (msgtype === "0") {
      const ct = parseRecord(message.ct);
      const user = createEnterUser(ct);
      const chat = createChatMessage({
        ...ct,
        uid: ct.id,
        uname: ct.user_nickname,
        isAnchor: "0"
      }, "enter", "进入直播间");
      this.options.onEnter?.({ user, chat });
      return;
    }
    if (msgtype === "2") {
      const heart = asNumber(message.heart, 0);
      this.options.onChat?.(createChatMessage(message, heart > 0 ? "light" : "normal"));
    }
  }

  private handleSendGift(message: Record<string, unknown>) {
    const gift = parseRecord(message.ct);
    const giftCount = asNumber(gift.giftcount, 1);
    const giftName = asText(gift.giftname || "礼物");
    const name = asText(message.uname || "星域用户");
    const chat = createChatMessage(message, "gift", `送出 ${giftCount} 个 ${giftName}`);
    const event: LiveSocketGift = {
      uid: asText(message.uid),
      name,
      avatar: displayUrl(asText(message.uhead), "/static/brand/icon-round.webp"),
      giftName,
      giftIcon: displayUrl(asText(gift.gifticon), "/static/brand/icon-round.webp"),
      giftCount,
      votes: asText(gift.votestotal || gift.votes),
      raw: gift,
      chat
    };
    this.options.onGift?.(event);
  }

  private handleFakeFans(message: Record<string, unknown>) {
    const ct = parseRecord(message.ct);
    const data = parseRecord(ct.data);
    const info = Array.isArray(data.info) ? data.info : [];
    const first = parseRecord(info[0]);
    const listRaw = first.list;
    let list: unknown[] = [];
    if (typeof listRaw === "string") {
      try {
        list = JSON.parse(listRaw);
      } catch {
        list = [];
      }
    } else if (Array.isArray(listRaw)) {
      list = listRaw;
    }
    if (list.length) {
      this.options.onFakeFans?.(list as UserProfile[]);
    }
  }
}
