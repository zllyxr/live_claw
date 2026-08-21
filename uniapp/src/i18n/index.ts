import { computed, ref } from "vue";
import liveMessages from "./messages/live";
import socialMessages from "./messages/social";
import miscMessages from "./messages/misc";
import coreMessages from "./messages/core";
import commerceMessages from "./messages/commerce";

interface MessageTree { [key: string]: string | MessageTree }
type MessageBundle = Record<AppLocale, MessageTree>;

export const supportedLocales = ["zh-CN", "en", "ja", "ko"] as const;
export type AppLocale = (typeof supportedLocales)[number];

const STORAGE_KEY = "app_locale";

const baseMessages = {
  "zh-CN": {
    language: { title: "语言", system: "跟随系统", changed: "语言已切换", zh: "简体中文", en: "English", ja: "日本語", ko: "한국어" },
    tab: { home: "首页", game: "游戏", sports: "体育", dynamic: "动态", me: "我的" },
    settings: { title: "设置", profile: "我的资料", password: "修改密码", remote: "远程协助", cancel: "账号注销", invalid: "登录失效页", sessionRevalidate: "当前登录状态需要重新验证", logout: "退出登录", logoutConfirm: "确认退出当前账号？", loggedOut: "已退出登录" },
    me: { welcome: "欢迎来到星域", loginHint: "登录后查看我的资产与消息", login: "立即登录", noAccount: "还没有账号？", register: "注册账号", fans: "粉丝", following: "关注", favorites: "收藏", recharge: "充值", rechargeReward: "充值奖励", coin: "星币", details: "明细", viewDetails: "查看我的明细", verify: "认证", goVerify: "前去认证", myServices: "我的服务", moreServices: "更多服务", video: "视频", income: "收益", dailyTask: "每日任务", roomManage: "房间管理", inviteReward: "邀请奖励", winningRecord: "中奖记录", support: "在线客服", defaultUser: "星域用户", loadFailed: "资料加载失败" }
  },
  en: {
    language: { title: "Language", system: "Follow system", changed: "Language changed", zh: "简体中文", en: "English", ja: "日本語", ko: "한국어" },
    tab: { home: "Home", game: "Games", sports: "Sports", dynamic: "Feed", me: "Me" },
    settings: { title: "Settings", profile: "My profile", password: "Change password", remote: "Remote assistance", cancel: "Delete account", invalid: "Session expired page", sessionRevalidate: "Your session needs to be verified again", logout: "Log out", logoutConfirm: "Log out of this account?", loggedOut: "Logged out" },
    me: { welcome: "Welcome to Starfield", loginHint: "Sign in to view your assets and messages", login: "Sign in", noAccount: "No account yet?", register: "Create account", fans: "Fans", following: "Following", favorites: "Favorites", recharge: "Top up", rechargeReward: "Bonus", coin: "Coins", details: "Details", viewDetails: "View details", verify: "Verify", goVerify: "Verify now", myServices: "My services", moreServices: "More services", video: "Videos", income: "Earnings", dailyTask: "Daily tasks", roomManage: "Room management", inviteReward: "Referral rewards", winningRecord: "Winning records", support: "Support", defaultUser: "Starfield user", loadFailed: "Failed to load profile" }
  },
  ja: {
    language: { title: "言語", system: "システム設定に従う", changed: "言語を変更しました", zh: "简体中文", en: "English", ja: "日本語", ko: "한국어" },
    tab: { home: "ホーム", game: "ゲーム", sports: "スポーツ", dynamic: "フィード", me: "マイページ" },
    settings: { title: "設定", profile: "プロフィール", password: "パスワード変更", remote: "リモートサポート", cancel: "アカウント削除", invalid: "ログイン期限切れページ", sessionRevalidate: "ログイン状態を再確認する必要があります", logout: "ログアウト", logoutConfirm: "このアカウントからログアウトしますか？", loggedOut: "ログアウトしました" },
    me: { welcome: "星域へようこそ", loginHint: "ログインして資産とメッセージを確認", login: "ログイン", noAccount: "アカウントをお持ちでないですか？", register: "新規登録", fans: "ファン", following: "フォロー", favorites: "お気に入り", recharge: "チャージ", rechargeReward: "ボーナス", coin: "コイン", details: "明細", viewDetails: "明細を見る", verify: "認証", goVerify: "今すぐ認証", myServices: "マイサービス", moreServices: "その他のサービス", video: "動画", income: "収益", dailyTask: "デイリータスク", roomManage: "ルーム管理", inviteReward: "招待特典", winningRecord: "当選履歴", support: "カスタマーサポート", defaultUser: "星域ユーザー", loadFailed: "プロフィールを読み込めませんでした" }
  },
  ko: {
    language: { title: "언어", system: "시스템 설정 따르기", changed: "언어가 변경되었습니다", zh: "简体中文", en: "English", ja: "日本語", ko: "한국어" },
    tab: { home: "홈", game: "게임", sports: "스포츠", dynamic: "피드", me: "마이" },
    settings: { title: "설정", profile: "내 프로필", password: "비밀번호 변경", remote: "원격 지원", cancel: "계정 삭제", invalid: "로그인 만료 페이지", sessionRevalidate: "로그인 상태를 다시 확인해야 합니다", logout: "로그아웃", logoutConfirm: "현재 계정에서 로그아웃할까요?", loggedOut: "로그아웃되었습니다" },
    me: { welcome: "성역에 오신 것을 환영합니다", loginHint: "로그인하여 자산과 메시지를 확인하세요", login: "로그인", noAccount: "계정이 없으신가요?", register: "회원가입", fans: "팬", following: "팔로잉", favorites: "즐겨찾기", recharge: "충전", rechargeReward: "충전 보너스", coin: "코인", details: "내역", viewDetails: "내역 보기", verify: "인증", goVerify: "인증하기", myServices: "내 서비스", moreServices: "더 많은 서비스", video: "동영상", income: "수익", dailyTask: "일일 미션", roomManage: "방 관리", inviteReward: "초대 보상", winningRecord: "당첨 기록", support: "고객센터", defaultUser: "성역 사용자", loadFailed: "프로필을 불러오지 못했습니다" }
  }
} as const;

const messageBundles: Array<Record<string, MessageTree>> = [
  baseMessages as unknown as Record<string, MessageTree>,
  liveMessages as unknown as Record<string, MessageTree>,
  socialMessages as unknown as Record<string, MessageTree>,
  miscMessages as unknown as Record<string, MessageTree>,
  coreMessages as unknown as Record<string, MessageTree>,
  commerceMessages as unknown as Record<string, MessageTree>
];

function systemLocale(): AppLocale {
  const raw = (uni.getSystemInfoSync().language || "zh-CN").toLowerCase();
  if (raw.startsWith("ja")) return "ja";
  if (raw.startsWith("ko")) return "ko";
  if (raw.startsWith("en")) return "en";
  return "zh-CN";
}

function initialLocale(): AppLocale {
  const saved = uni.getStorageSync(STORAGE_KEY);
  return supportedLocales.includes(saved as AppLocale) ? saved as AppLocale : systemLocale();
}

const locale = ref<AppLocale>(initialLocale());

function uniLocale(value: AppLocale) {
  return value === "zh-CN" ? "zh-Hans" : value;
}

function lookup(bundle: Record<string, MessageTree>, language: AppLocale, key: string) {
  return key.split(".").reduce<any>((node, part) => node?.[part], bundle[language]);
}

export function t(key: string, params?: Record<string, string | number>): string {
  let result: unknown;
  for (const bundle of messageBundles) {
    result = lookup(bundle, locale.value, key);
    if (typeof result === "string") break;
  }
  if (typeof result !== "string") {
    for (const bundle of messageBundles) {
      result = lookup(bundle, "zh-CN", key);
      if (typeof result === "string") break;
    }
  }
  if (typeof result !== "string") return key;
  return params
    ? result.replace(/\{(\w+)\}/g, (match, name) => params[name] === undefined ? match : String(params[name]))
    : result;
}

export function getApiLanguage() {
  return locale.value === "zh-CN" ? "zh-cn" : locale.value;
}

export function setLocale(next: AppLocale) {
  locale.value = next;
  uni.setStorageSync(STORAGE_KEY, next);
  uni.setLocale(uniLocale(next));
  applyTabBarLocale();
}

export function applyTabBarLocale() {
  ["home", "game", "sports", "dynamic", "me"].forEach((key, index) => {
    uni.setTabBarItem({ index, text: t(`tab.${key}`) });
  });
}

export function useI18n() {
  return { locale: computed(() => locale.value), t, setLocale };
}
