import { ACTIVE_TYPES, API_BASE, API_HOST, DEFAULT_LANGUAGE, SIGN_SALT, STORAGE_KEYS } from "@/constants/config";
import { firstInfo, infoList, request } from "@/api/client";
import { getSession, saveSession, saveUser } from "@/utils/session";
import { clearPendingInvite, getPendingInvite } from "@/utils/invite";
import { md5 } from "@/utils/md5";
import {
  openIMBlackList,
  openIMChangeGroupMute,
  openIMConversations,
  openIMCreateGroup,
  openIMDeleteLocalMessage,
  openIMDismissGroup,
  openIMGetGroup,
  openIMGroupApplications,
  openIMGroupMembers,
  openIMGroups,
  openIMHandleGroupApplication,
  openIMHistory,
  openIMInviteGroupMembers,
  openIMJoinGroup,
  openIMKickGroupMember,
  openIMMarkRead,
  openIMMuteGroupMember,
  openIMQuitGroup,
  openIMRemoveConversation,
  openIMRevokeMessage,
  openIMSendImage,
  openIMSendFile,
  openIMSendText,
  openIMSendVideo,
  openIMSendVoice,
  openIMSetBlack,
  openIMSetGroupInfo,
  openIMSetGroupMemberRole,
  openIMTransferGroupOwner,
  type ChatKind
} from "@/utils/openim";
import type {
  CountryCodeGroup,
  CountryCodeItem,
  AuthSubmitPayload,
  CashAccount,
  DailyTaskBundle,
  DynamicComment,
  DynamicCommentBundle,
  DynamicItem,
  FollowLiveHome,
  HomeDashboard,
  InviteAgentState,
  InviteBindResult,
  InviteCode,
  LiveHome,
  LiveGiftBundle,
  LiveRoom,
  LotteryHome,
  MiniGameBundle,
  MiniGameLaunch,
  SportsBetRecordBundle,
  SportsHome,
  SportsMarketBundle,
  UploadResult,
  RedPackItem,
  RedPackRobBundle,
  RechargeOrder,
  RechargeOrderBundle,
  VideoItem,
  WalletBalance,
  WalletPayMethod,
  WalletRule,
  UserProfile
} from "@/types/api";

function signParams(params: Record<string, string | number>) {
  const payload = Object.keys(params)
    .sort()
    .map((key) => `${key}=${params[key]}&`)
    .join("");
  return md5(`${payload}${SIGN_SALT}`);
}

function clientPlatform() {
  try {
    const info = uni.getSystemInfoSync() as unknown as Record<string, unknown>;
    return String(info.platform || "h5").toLowerCase();
  } catch {
    return "h5";
  }
}

function clientDeviceId() {
  try {
    const info = uni.getSystemInfoSync() as unknown as Record<string, unknown>;
    return String(info.deviceId || info.deviceModel || info.model || getSession().uid || "");
  } catch {
    return getSession().uid || "";
  }
}

function clientUserAgent() {
  return (globalThis as unknown as { navigator?: { userAgent?: string } }).navigator?.userAgent || "";
}

async function bindStoredInviteAttribution() {
  const pending = getPendingInvite();
  if (!pending || (!pending.code && !pending.ref && !pending.clickId)) {
    return;
  }
  try {
    await bindInviteAttribution(pending);
  } catch {
    // Login/register must not fail because a stored invite is expired or invalid.
  } finally {
    clearPendingInvite();
  }
}

export function getCachedConfig() {
  try {
    const value = uni.getStorageSync(STORAGE_KEYS.config);
    return value && typeof value === "object" ? (value as Record<string, unknown>) : undefined;
  } catch {
    return undefined;
  }
}

export async function getConfig() {
  const config = await firstInfo<Record<string, unknown>>("Home.getConfig", {}, { auth: false });
  if (config) {
    try {
      uni.setStorageSync(STORAGE_KEYS.config, config);
    } catch {
      // Config cache is optional; callers still receive the fresh value.
    }
  }
  return config;
}

export async function checkToken() {
  const session = getSession();
  return firstInfo<Record<string, unknown>>("User.ifToken", {
    uid: session.uid,
    token: session.token
  });
}

export async function getLoginInfo() {
  return firstInfo<Record<string, unknown>>("Home.getLogin", {}, { auth: false });
}

export function getCountryCodes(field = "") {
  return infoList<CountryCodeGroup | CountryCodeItem>(
    "Login.getCountrys",
    { field },
    { auth: false }
  );
}

export async function login(userLogin: string, password: string, countryCode = "86") {
  const info = await firstInfo<UserProfile>(
    "Login.userLogin",
    {
      user_login: userLogin,
      user_pass: password,
      country_code: countryCode
    },
    { auth: false }
  );
  if (info) {
    saveSession(info);
    void bindStoredInviteAttribution();
  }
  return info;
}

export async function getRegisterCode(mobile: string, email: string, countryCode = "86") {
  const sign = md5(`email=${email}&mobile=${mobile}&${SIGN_SALT}`);
  return request("Login.getCode", {
    mobile,
    email,
    country_code: countryCode,
    sign
  }, { auth: false });
}

export async function register(
  mobile: string,
  email: string,
  code: string,
  password: string,
  password2: string,
  countryCode = "86"
) {
  const info = await firstInfo<UserProfile>(
    "Login.userReg",
    {
      user_login: mobile,
      email,
      code,
      user_pass: password,
      user_pass2: password2,
      country_code: countryCode,
      source: "android"
    },
    { auth: false }
  );
  if (info) {
    saveSession(info);
    void bindStoredInviteAttribution();
  }
  return info;
}

export async function getForgotCode(mobile: string, email: string, countryCode = "86") {
  const sign = md5(`email=${email}&mobile=${mobile}&${SIGN_SALT}`);
  return request("Login.getForgetCode", {
    mobile,
    email,
    country_code: countryCode,
    sign
  }, { auth: false });
}

export function resetPassword(
  mobile: string,
  email: string,
  code: string,
  password: string,
  password2: string,
  countryCode = "86"
) {
  return request(
    "Login.userFindPass",
    {
      user_login: mobile,
      email,
      code,
      user_pass: password,
      user_pass2: password2,
      country_code: countryCode
    },
    { auth: false }
  );
}

export function getBaseInfo() {
  return firstInfo<UserProfile>("User.getBaseInfo").then((user) => {
    if (user) {
      saveUser(user);
    }
    return user;
  });
}

export function searchUsers(key: string, page = 1) {
  const { uid } = getSession();
  return infoList<UserProfile>(
    "Home.search",
    {
      uid,
      key,
      p: page
    },
    { auth: false }
  );
}

export async function getHotLive(page = 1) {
  const home = await firstInfo<LiveHome>("Home.getHot", { p: page }, { auth: false });
  return Array.isArray(home?.list) ? home.list : [];
}

export function getFollowLive(page = 1, liveType = 0) {
  return firstInfo<FollowLiveHome>("Home.getFollow", {
    live_type: liveType,
    p: page
  });
}

export function getLotteryHome() {
  return firstInfo<LotteryHome>("LotteryGame.home");
}

export function getHomeDashboard() {
  return firstInfo<HomeDashboard>("Home.dashboard");
}

export function getContentPage(key: "recharge_agreement") {
  return firstInfo<{ title: string; content: string }>("System.getPage", { key }, { auth: false });
}

export function getLotteryGameDetail(gameId: string | number, gameCode = "") {
  return firstInfo<Record<string, unknown>>("LotteryGame.detail", {
    game_id: gameId,
    game_code: gameCode
  });
}

export function getLotteryCurrentIssue(gameId: string | number, gameCode = "") {
  return firstInfo<Record<string, unknown>>("LotteryGame.currentIssue", {
    game_id: gameId,
    game_code: gameCode
  });
}

export function getLotteryIssueHistory(gameId: string | number, gameCode = "", page = 1) {
  return firstInfo<Record<string, unknown>>("LotteryGame.issueHistory", {
    game_id: gameId,
    game_code: gameCode,
    p: page
  });
}

export function submitLotteryBet(args: {
  gameId: string | number;
  issueId: string | number;
  optionId?: string | number;
  amount?: string | number;
  items?: Array<{ optionId: string | number; amount: string | number }>;
  gameCode?: string;
  clientTraceId?: string;
}) {
  const items =
    args.items?.map((item) => ({ option_id: Number(item.optionId), amount: Number(item.amount) })) ||
    (args.optionId !== undefined
      ? [{ option_id: Number(args.optionId), amount: Number(args.amount || 0) }]
      : []);
  const traceOption = items.map((item) => item.option_id).join("-").slice(0, 32) || "BET";
  return request("LotteryGame.bet", {
    game_id: args.gameId,
    game_code: args.gameCode || "",
    issue_id: args.issueId,
    client_trace_id: args.clientTraceId || `UNI_${Date.now()}_${traceOption}`,
    items: JSON.stringify(items)
  });
}

export function getLotteryBetRecords(game?: { id?: string | number; game_code?: string }, page = 1) {
  return firstInfo<Record<string, unknown>>("LotteryGame.orderList", {
    game_id: game?.id || "",
    game_code: game?.game_code || "",
    p: page
  });
}

export function getSportsHome(tab = "today", date = "", competitionType = "") {
  return firstInfo<SportsHome>("Sports.home", {
    tab,
    date,
    competition_type: competitionType
  });
}

export function getSportsMatchDetail(matchId: string | number) {
  return firstInfo<Record<string, unknown>>("Sports.matchDetail", {
    match_id: matchId
  });
}

export function getSportsBetMarkets(matchId: string | number) {
  return firstInfo<SportsMarketBundle>("SportsBet.matchMarkets", {
    match_id: matchId
  });
}

export function submitSportsBet(args: {
  matchId: string | number;
  optionId: string | number;
  amount: string | number;
}) {
  return request("SportsBet.bet", {
    match_id: args.matchId,
    client_trace_id: `UNI_SPORT_${Date.now()}_${args.optionId}`,
    items: JSON.stringify([{ option_id: Number(args.optionId), amount: Number(args.amount) }])
  });
}

export async function getSportsBetRecords(matchId = "", page = 1) {
  const params = {
    match_id: matchId,
    p: page
  };
  try {
    return await firstInfo<SportsBetRecordBundle>("SportsBet.recordList", params);
  } catch {
    return firstInfo<SportsBetRecordBundle>("SportsBet.orderList", params);
  }
}

export function getWalletBalance() {
  return firstInfo<WalletBalance>("User.getBalance", { type: 0 });
}

export function getWalletLedger(page = 1) {
  return firstInfo<Record<string, unknown>>("Wallet.ledger", { p: page });
}

export function getRechargeOrders(page = 1) {
  return firstInfo<RechargeOrderBundle>("Charge.orderList", { p: page });
}

export function getWithdrawalOrders(page = 1) {
  return firstInfo<Record<string, unknown>>("User.cashOrderList", { p: page });
}

export function getVerificationStatus() {
  return firstInfo<Record<string, unknown>>("Auth.getStatus");
}

function payServiceOf(payId?: string) {
  const map: Record<string, string> = {
    ali: "Charge.getAliOrder",
    wx: "Charge.getWxOrder",
    paypal: "Charge.getBraintreePaypalOrder",
    usdt: "Charge.getUsdtOrder",
    bank: "Charge.getBankOrder"
  };
  return map[String(payId || "")] || "";
}

export function createPaymentTraceId() {
  const uid = String(getSession().uid || "anonymous").replace(/[^0-9A-Za-z_-]/g, "").slice(0, 32);
  let random = "";
  try {
    random = globalThis.crypto?.randomUUID?.().replace(/-/g, "") || "";
  } catch {
    // Some older App WebViews do not expose Web Crypto.
  }
  if (!random) {
    random = `${Math.random().toString(36).slice(2)}${Math.random().toString(36).slice(2)}`;
  }
  return `PAY_${uid || "anonymous"}_${Date.now()}_${random.slice(0, 24)}`;
}

export function createCoinOrder(
  rule: WalletRule,
  pay: WalletPayMethod,
  clientTraceId = createPaymentTraceId()
) {
  const service = payServiceOf(String(pay.id || ""));
  if (!service) {
    throw new Error("请选择可用支付方式");
  }
  return firstInfo<RechargeOrder>(service, {
    money: rule.money || "",
    changeid: rule.id || "",
    coin: rule.coin || "",
    client_trace_id: clientTraceId
  });
}

export function getRechargeOrderStatus(orderNo: string) {
  const normalized = String(orderNo || "").trim();
  if (!normalized) {
    throw new Error("充值订单号无效");
  }
  return firstInfo<RechargeOrder>("Charge.orderStatus", { order_no: normalized });
}

export function submitBankPaymentProof(orderNo: string, filePath: string) {
  const normalizedOrderNo = String(orderNo || "").trim();
  if (!normalizedOrderNo || !filePath) {
    throw new Error("请选择付款凭证图片");
  }
  const session = getSession();
  const uploadUrl = proxyApiUrlForPreview(
    `${API_BASE}?service=Charge.submitBankPaymentProof`
  );
  return new Promise<RechargeOrder>((resolve, reject) => {
    uni.uploadFile({
      url: uploadUrl,
      filePath,
      name: "file",
      formData: {
        service: "Charge.submitBankPaymentProof",
        uid: session.uid,
        token: session.token,
        order_no: normalizedOrderNo,
        language: DEFAULT_LANGUAGE
      },
      success: (response) => {
        try {
          if (Number(response.statusCode || 0) < 200 || Number(response.statusCode || 0) >= 300) {
            throw new Error(`上传服务器响应异常（${response.statusCode || 0}）`);
          }
          const payload = typeof response.data === "string"
            ? JSON.parse(response.data.slice(Math.max(0, response.data.indexOf("{"))))
            : response.data;
          const inner = typeof (payload as any)?.data === "string"
            ? JSON.parse((payload as any).data)
            : (payload as any)?.data;
          if (Number(inner?.code ?? 0) !== 0) {
            throw new Error(String(inner?.msg || "付款凭证提交失败"));
          }
          const result = Array.isArray(inner?.info) ? inner.info[0] : undefined;
          if (!result) {
            throw new Error("付款凭证提交结果无效");
          }
          resolve(result as RechargeOrder);
        } catch (error) {
          reject(error);
        }
      },
      fail: (error) => reject(new Error(error.errMsg || "付款凭证提交失败"))
    });
  });
}

export function getProfit() {
  return firstInfo<Record<string, unknown>>("User.getProfit");
}

export function getCashAccounts() {
  return infoList<CashAccount>("User.getUserAccountList");
}

export function addCashAccount(args: {
  type: string | number;
  account: string;
  name?: string;
  accountBank?: string;
}) {
  return firstInfo<CashAccount>("User.setUserAccount", {
    type: args.type,
    account: args.account,
    name: args.name || "",
    account_bank: args.accountBank || ""
  });
}

export function deleteCashAccount(id: string | number) {
  return request("User.delUserAccount", { id });
}

export function submitCash(accountId: string | number, cashVote: string | number) {
  return request("User.setCash", {
    accountid: accountId,
    cashvote: cashVote
  });
}

export function getDailyTasks(liveUid = "0", isLive = 0) {
  return firstInfo<DailyTaskBundle>("User.seeDailyTasks", {
    liveuid: liveUid,
    islive: isLive
  });
}

export function receiveDailyTaskReward(taskId: string | number) {
  return request("User.receiveTaskReward", {
    taskid: taskId
  });
}

export function getDynamics(tab: "recommend" | "follow" | "newest", page = 1) {
  const serviceMap = {
    recommend: "Dynamic.getRecommendDynamics",
    follow: "Dynamic.getAttentionDynamic",
    newest: "Dynamic.getNewDynamic"
  } as const;
  const extra = tab === "newest" ? { lng: "", lat: "" } : {};
  return infoList<DynamicItem>(serviceMap[tab], { p: page, ...extra });
}

export function getUserDynamics(toUid: string, page = 1) {
  return infoList<DynamicItem>("Dynamic.getHomeDynamic", {
    touid: toUid,
    p: page
  });
}

export function getDynamicDetail(dynamicId: string | number) {
  return firstInfo<DynamicItem>("Dynamic.getDynamic", {
    dynamicid: dynamicId
  });
}

export function getDynamicComments(dynamicId: string | number, page = 1) {
  return firstInfo<DynamicCommentBundle>("Dynamic.getComments", {
    dynamicid: dynamicId,
    p: page
  });
}

export function getDynamicReplies(commentId: string | number, page = 1) {
  return infoList<DynamicComment>("Dynamic.getReplys", {
    commentid: commentId,
    p: page
  });
}

export function commentDynamic(args: {
  dynamicId: string | number;
  content: string;
  toUid?: string | number;
  commentId?: string | number;
  parentId?: string | number;
}) {
  return firstInfo<Record<string, unknown>>("Dynamic.setComment", {
    dynamicid: args.dynamicId,
    touid: args.toUid || "",
    commentid: args.commentId || "",
    parentid: args.parentId || "",
    content: args.content,
    type: 0,
    voice: "",
    length: ""
  });
}

export function deleteDynamic(dynamicId: string | number) {
  return request("Dynamic.del", { dynamicid: dynamicId });
}

export function reportDynamic(dynamicId: string | number, content: string) {
  return request("Dynamic.report", { dynamicid: dynamicId, content });
}

export function deleteDynamicComment(dynamicId: string | number, commentId: string | number, commentUid: string | number) {
  return request("Dynamic.delComments", {
    dynamicid: dynamicId,
    commentid: commentId,
    commentuid: commentUid
  });
}

export function likeDynamic(dynamicId: string | number) {
  const { uid } = getSession();
  const sign = md5(`dynamicid=${dynamicId}&uid=${uid}&${SIGN_SALT}`);
  return firstInfo<Record<string, unknown>>("Dynamic.addLike", {
    dynamicid: dynamicId,
    sign
  });
}

export function publishDynamic(args: {
  type: number;
  text: string;
  images?: string;
  videoImage?: string;
  videoUrl?: string;
  voiceUrl?: string;
  voiceDuration?: number;
}) {
  const { uid } = getSession();
  const sign = md5(`type=${args.type}&uid=${uid}&${SIGN_SALT}`);
  return request("Dynamic.setDynamic", {
    type: args.type,
    title: args.text,
    thumb: args.images || "",
    video_thumb: args.videoImage || "",
    href: args.videoUrl || "",
    voice: args.voiceUrl || "",
    length: args.voiceDuration || 0,
    sign
  });
}

export function publishTextDynamic(text: string) {
  return publishDynamic({ type: ACTIVE_TYPES.text, text });
}

export function setAttention(toUid: string | number) {
  return firstInfo<{ isattent?: string | number }>("User.setAttent", {
    touid: toUid
  });
}

export function getUserHome(toUid: string | number) {
  return firstInfo<UserProfile>("User.getUserHome", {
    touid: toUid
  });
}

export function getFollowList(toUid: string | number, page = 1) {
  return infoList<UserProfile>("User.getFollowsList", {
    touid: toUid,
    p: page
  });
}

export function getFansList(toUid: string | number, page = 1) {
  return infoList<UserProfile>("User.getFansList", {
    touid: toUid,
    p: page
  });
}

export async function setBlack(toUid: string | number) {
  const result = await firstInfo<Record<string, unknown>>("User.setBlack", {
    touid: toUid
  });
  const blocked = Number(result?.isblack || 0) === 1;
  await openIMSetBlack(String(toUid), blocked);
  return result;
}

export function updateAvatar(avatar: string) {
  return request("User.updateAvatar", { avatar });
}

export function updateUserFields(fields: Record<string, string | number>) {
  return request("User.updateFields", {
    fields: JSON.stringify(fields)
  });
}

export function updateUserBg(img: string) {
  return request("User.updateBgImg", { img });
}

export function updatePassword(oldpass: string, pass: string, pass2: string) {
  return request("User.updatePass", { oldpass, pass, pass2 });
}

export function submitAuth(payload: AuthSubmitPayload) {
  return request("Auth.setAuth", {
    real_name: payload.realName,
    mobile: payload.mobile,
    cer_no: payload.cardNo,
    front_view: payload.frontView,
    back_view: payload.backView,
    handset_view: payload.handsetView
  });
}

export function getInviteCode() {
  return firstInfo<InviteCode>("Agent.getCode");
}

export function checkInviteAgent() {
  return firstInfo<InviteAgentState>("Agent.checkAgent");
}

export function bindInviteCode(code: string) {
  return firstInfo<{ msg?: string }>("User.setDistribut", { code });
}

export function bindInviteAttribution(params: { code?: string; ref?: string; clickId?: string }) {
  const code = String(params.code || params.ref || "");
  return firstInfo<InviteBindResult>("Invite.bind", {
    code,
    ref: String(params.ref || code),
    click_id: String(params.clickId || ""),
    platform: clientPlatform(),
    device_id: clientDeviceId(),
    android_id: clientDeviceId(),
    user_agent: clientUserAgent()
  });
}

export function getCancelCondition() {
  return firstInfo<Record<string, unknown>>("Login.getCancelCondition");
}

export function cancelAccount() {
  const { uid, token } = getSession();
  const time = String(Math.floor(Date.now() / 1000));
  const sign = md5(`time=${time}&token=${token}&uid=${uid}&${SIGN_SALT}`);
  return request("Login.cancelAccount", { time, sign });
}

export function getSystemMessages(page = 1) {
  return infoList<Record<string, unknown>>("Message.GetList", { p: page });
}

export function getNotifyMessages(type: "at" | "like" | "comment" | "fans", page = 1) {
  const map = {
    at: "Message.atLists",
    like: "Message.praiseLists",
    comment: "Message.commentLists",
    fans: "Message.fansLists"
  } as const;
  return infoList<Record<string, unknown>>(map[type], { p: page });
}

export type UnifiedNotificationType = "system" | "group" | "at" | "like" | "comment" | "fans";

const unifiedNotificationSources: Array<{
  type: UnifiedNotificationType;
  label: string;
  load: (page: number) => Promise<Record<string, unknown>[]>;
}> = [
  { type: "system", label: "平台通知", load: getSystemMessages },
  {
    type: "group",
    label: "群聊申请",
    load: async (page) => {
      const applications = await openIMGroupApplications((page - 1) * 20, 20);
      return applications
        .filter((application) => Number(application.handleResult || 0) === 0)
        .map((application) => ({
          ...application,
          title: `${application.nickname || `用户${application.userID}`}申请加入${application.groupName || "群聊"}`,
          content: application.reqMsg || "申请加入群聊",
          addtime: application.reqTime || 0,
          uid: application.userID,
          group_id: application.groupID
        }));
    }
  },
  { type: "at", label: "提及通知", load: (page) => getNotifyMessages("at", page) },
  { type: "like", label: "点赞通知", load: (page) => getNotifyMessages("like", page) },
  { type: "comment", label: "评论通知", load: (page) => getNotifyMessages("comment", page) },
  { type: "fans", label: "关注通知", load: (page) => getNotifyMessages("fans", page) }
];

function notificationTimestamp(item: Record<string, unknown>) {
  const raw =
    item.addtime ??
    item.create_time ??
    item.created_at ??
    item.datetime ??
    item.time ??
    item.timestamp ??
    0;
  const numeric = Number(raw);
  if (Number.isFinite(numeric) && numeric > 0) {
    return numeric < 1_000_000_000_000 ? numeric * 1000 : numeric;
  }
  const parsed = Date.parse(String(raw || ""));
  return Number.isFinite(parsed) ? parsed : 0;
}

export async function getUnifiedNotifications(page = 1) {
  const results = await Promise.allSettled(
    unifiedNotificationSources.map(async (source) => ({
      source,
      items: await source.load(page)
    }))
  );
  const fulfilled = results.filter((result) => result.status === "fulfilled");
  if (!fulfilled.length) {
    const failed = results.find((result) => result.status === "rejected");
    throw failed && failed.status === "rejected" ? failed.reason : new Error("系统通知加载失败");
  }

  return results
    .flatMap((result) => {
      if (result.status !== "fulfilled") {
        return [];
      }
      const { source, items } = result.value;
      return items.map((item) => ({
        ...item,
        _notice_type: source.type,
        _notice_label: source.label
      }));
    })
    .sort((left, right) => notificationTimestamp(right) - notificationTimestamp(left));
}

export function getConversations() {
  return openIMConversations();
}

export function getChatHistory(targetID: string, kind: ChatKind = "single", startClientMsgID = "", count = 30) {
  return openIMHistory(targetID, kind, startClientMsgID, count);
}

export function sendTextMessage(targetID: string, content: string, kind: ChatKind = "single") {
  return openIMSendText(targetID, content, kind);
}

export function sendImageMessage(targetID: string, image: string, kind: ChatKind = "single") {
  return openIMSendImage(targetID, image, kind);
}

export function sendVideoMessage(
  targetID: string,
  video: string,
  cover = "",
  duration = 0,
  size = 0,
  kind: ChatKind = "single"
) {
  return openIMSendVideo(targetID, video, cover, duration, size, kind);
}

export function sendVoiceMessage(
  targetID: string,
  voice: string,
  duration = 0,
  size = 0,
  kind: ChatKind = "single"
) {
  return openIMSendVoice(targetID, voice, duration, size, kind);
}

export function sendFileMessage(
  targetID: string,
  file: string,
  fileName: string,
  fileSize = 0,
  kind: ChatKind = "single"
) {
  return openIMSendFile(targetID, file, fileName, fileSize, kind);
}

export function markConversationRead(targetID: string, kind: ChatKind = "single") {
  return openIMMarkRead(targetID, kind);
}

export function removeConversation(targetID: string, kind: ChatKind = "single") {
  return openIMRemoveConversation(targetID, kind);
}

export function revokeChatMessage(targetID: string, clientMsgID: string, kind: ChatKind = "single") {
  return openIMRevokeMessage(targetID, clientMsgID, kind);
}

export function deleteLocalChatMessage(targetID: string, clientMsgID: string, kind: ChatKind = "single") {
  return openIMDeleteLocalMessage(targetID, clientMsgID, kind);
}

export const getChatGroups = openIMGroups;
export const createChatGroup = openIMCreateGroup;
export const getChatGroup = openIMGetGroup;
export const getChatGroupMembers = openIMGroupMembers;
export const inviteChatGroupMembers = openIMInviteGroupMembers;
export const joinChatGroup = openIMJoinGroup;
export const updateChatGroup = openIMSetGroupInfo;
export const setChatGroupMemberRole = openIMSetGroupMemberRole;
export const muteChatGroupMember = openIMMuteGroupMember;
export const muteChatGroup = openIMChangeGroupMute;
export const kickChatGroupMember = openIMKickGroupMember;
export const transferChatGroupOwner = openIMTransferGroupOwner;
export const quitChatGroup = openIMQuitGroup;
export const dismissChatGroup = openIMDismissGroup;
export const getChatGroupApplications = openIMGroupApplications;
export const handleChatGroupApplication = openIMHandleGroupApplication;
export const getChatBlackList = openIMBlackList;
export const setChatBlack = openIMSetBlack;

export async function getUploadInfo() {
  return firstInfo<{
    cloudtype?: string;
    storageInfo?: { upload_url?: string; field?: string };
    localInfo?: { upload_url?: string };
  }>("Upload.getCosInfo");
}

function parseUploadResponse(data: unknown) {
  const payload = typeof data === "string" ? JSON.parse(data.slice(Math.max(0, data.indexOf("{")))) : data;
  const rawData = (payload as any)?.data;
  const inner = typeof rawData === "string" ? JSON.parse(rawData) : rawData;
  const info = Array.isArray(inner?.info) ? inner.info : [];
  if (Number(inner?.code ?? 0) !== 0) {
    throw new Error(String(inner?.msg || "上传失败"));
  }
  const result = info[0] as UploadResult | undefined;
  if (!result) {
    throw new Error("上传服务器未返回文件信息");
  }
  return result;
}

function proxyApiUrlForPreview(url: string) {
  const raw = String(url || "").trim();
  const appApiMatch = raw.match(
    /^(?:https?:\/\/[^/]+)?\/appapi\/?(\?[^#]*)?$/i
  );
  if (appApiMatch) {
    const base = API_BASE.endsWith("/") ? API_BASE : `${API_BASE}/`;
    return `${base}${appApiMatch[1] || ""}`;
  }
  if (raw.startsWith("/")) {
    return `${API_HOST.replace(/\/$/, "")}${raw}`;
  }
  return raw;
}

export async function uploadOne(filePath: string) {
  const uploadInfo = await getUploadInfo();
  const uploadUrl = proxyApiUrlForPreview(uploadInfo?.storageInfo?.upload_url || `${API_BASE}?service=Upload.uploadFile`);
  const field = uploadInfo?.storageInfo?.field || "file";
  const session = getSession();
  return new Promise<UploadResult>((resolve, reject) => {
    uni.uploadFile({
      url: uploadUrl,
      filePath,
      name: field,
      formData: {
        language: DEFAULT_LANGUAGE,
        uid: session.uid,
        token: session.token
      },
      success: (res) => {
        try {
          if (Number(res.statusCode || 0) < 200 || Number(res.statusCode || 0) >= 300) {
            throw new Error(`上传服务器响应异常（${res.statusCode || 0}）`);
          }
          resolve(parseUploadResponse(res.data));
        } catch (error) {
          reject(error);
        }
      },
      fail: (error) => reject(new Error(error.errMsg || "上传失败"))
    });
  });
}

export function enterLiveRoom(liveUid: string, stream: string) {
  return firstInfo<Record<string, unknown>>("Live.enterRoom", {
    city: "",
    liveuid: liveUid,
    mobileid: "",
    stream
  });
}

export interface DirectLiveSource {
  url?: string;
  format?: string;
  height?: string | number;
  resolution?: string;
  provider?: string;
  room_id?: string;
  room_page?: string;
  cache_seconds?: string | number;
  delivery?: string;
}

export function resolveLiveSource(liveUid: string, stream: string, refresh = false) {
  return firstInfo<DirectLiveSource>(
    "Live.resolveSource",
    {
      liveuid: liveUid,
      stream,
      refresh: refresh ? 1 : 0
    },
    {
      auth: false,
      timeout: 65000,
      retry: 0
    }
  );
}

export function signOutWatchLive(liveUid: string, stream: string) {
  return request("Live.signOutWatchLive", {
    liveuid: liveUid,
    stream
  });
}

export interface LiveUserListInfo {
  userlist?: UserProfile[];
  nums?: string | number;
  votestotal?: string | number;
  [key: string]: unknown;
}

export function getLiveUserListInfo(liveUid: string, stream: string, page = 1) {
  return firstInfo<LiveUserListInfo>(
    "Live.getUserLists",
    {
      liveuid: liveUid,
      stream,
      p: page
    },
    { auth: false }
  );
}

export async function getLiveUsers(liveUid: string, stream: string, page = 1) {
  const data = await getLiveUserListInfo(liveUid, stream, page);
  return Array.isArray(data?.userlist) ? data.userlist : [];
}

export function getLiveGiftList(liveType = 0) {
  return firstInfo<LiveGiftBundle>("Live.getGiftList", {
    live_type: liveType
  });
}

export function getRedPacks(stream: string) {
  return infoList<RedPackItem>("Red.getRedList", {
    stream,
    sign: signParams({ stream })
  });
}

export function sendRedPack(args: {
  stream: string;
  type: string | number;
  typeGrant: string | number;
  coin: string | number;
  nums: string | number;
  des?: string;
}) {
  return firstInfo<Record<string, unknown>>("Red.sendRed", {
    stream: args.stream,
    type: args.type,
    type_grant: args.typeGrant,
    coin: args.coin,
    nums: args.nums,
    des: args.des || "恭喜发财，大吉大利"
  });
}

export function robRedPack(stream: string, redId: string | number) {
  const { uid } = getSession();
  return firstInfo<{ win?: string | number; msg?: string }>("Red.robRed", {
    stream,
    redid: redId,
    sign: signParams({ redid: redId, stream, uid })
  });
}

export function getRedPackRobList(stream: string, redId: string | number) {
  return firstInfo<RedPackRobBundle>("Red.getRedRobList", {
    stream,
    redid: redId,
    sign: signParams({ redid: redId, stream })
  });
}

export function sendLiveGift(args: {
  liveUid: string;
  stream: string;
  giftId: string | number;
  giftCount?: string | number;
  clientRequestId: string;
  ispack?: number;
  is_sticker?: number;
}) {
  const giftCount = Math.min(
    999,
    Math.max(1, Math.floor(Number(args.giftCount) || 1))
  );
  return firstInfo<Record<string, unknown>>("Live.sendGift", {
    liveuid: args.liveUid,
    stream: args.stream,
    touids: args.liveUid,
    giftid: args.giftId,
    giftcount: giftCount,
    client_request_id: args.clientRequestId,
    ispack: args.ispack || 0,
    is_sticker: args.is_sticker || 0
  });
}

export function getLiveAdminList(liveUid: string) {
  return firstInfo<{ list?: UserProfile[]; nums?: string | number; total?: string | number }>("Live.getAdminList", {
    liveuid: liveUid
  });
}

export function setLiveAdmin(liveUid: string, toUid: string | number) {
  return firstInfo<Record<string, unknown>>("Live.setAdmin", {
    liveuid: liveUid,
    touid: toUid
  });
}

export function reportLiveUser(toUid: string | number, content: string) {
  return request("Live.setReport", {
    touid: toUid,
    content
  });
}

export function shutUpLiveUser(liveUid: string, stream: string, toUid: string | number, type = 1) {
  return request("Live.setShutUp", {
    liveuid: liveUid,
    stream,
    touid: toUid,
    type
  });
}

export function kickLiveUser(liveUid: string, toUid: string | number) {
  return request("Live.kicking", {
    liveuid: liveUid,
    touid: toUid
  });
}

export function getGuardList(liveUid: string, page = 1) {
  return infoList<Record<string, unknown>>("Guard.GetGuardList", {
    liveuid: liveUid,
    p: page
  });
}

export function getLiveUserRank(liveUid: string, stream: string) {
  return infoList<Record<string, unknown>>("Live.getUserRank", {
    liveuid: liveUid,
    stream
  });
}

export function getMyVideos(page = 1) {
  return infoList<VideoItem>("Video.getMyVideo", { p: page });
}

export function getLikeVideos(page = 1) {
  return infoList<VideoItem>("Video.getLikeVideos", { p: page });
}

export function getVideoDetail(videoId: string | number) {
  return firstInfo<VideoItem>("Video.getVideo", {
    videoid: videoId
  });
}

/* ---------- 小游戏 ---------- */

export function getMiniGames(category = "") {
  return firstInfo<MiniGameBundle>("MiniGame.list", { category }, { auth: false });
}

export function enterMiniGame(code: string, room = "") {
  return firstInfo<MiniGameLaunch>("MiniGame.enter", { code, room });
}
