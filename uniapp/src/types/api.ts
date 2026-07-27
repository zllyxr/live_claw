export type ApiInfo<T> = T[];

export interface ApiEnvelope<T = unknown> {
  code: number;
  msg: string;
  info: ApiInfo<T>;
}

export interface SessionState {
  uid: string;
  token: string;
  user?: UserProfile;
}

export interface UserProfile {
  id?: string;
  uid?: string;
  token?: string;
  user_nickname?: string;
  user_nicename?: string;
  userNiceName?: string;
  avatar?: string;
  avatar_thumb?: string;
  sex?: string | number;
  signature?: string;
  coin?: string;
  votes?: string;
  follows?: string | number;
  fans?: string | number;
  liang?: { name?: string };
  liang_name?: string;
  level?: string | number;
  list?: Array<{ list?: UserMenuItem[] }>;
  vip?: Record<string, unknown>;
  [key: string]: unknown;
}

export interface CountryCodeItem {
  name?: string;
  name_en?: string;
  tel?: string | number;
  index?: string;
  [key: string]: unknown;
}

export interface CountryCodeGroup {
  title?: string;
  lists?: CountryCodeItem[];
  [key: string]: unknown;
}

export interface WalletRule {
  id?: string | number;
  coin?: string | number;
  money?: string | number;
  give?: string | number;
  coin_paypal?: string | number;
  checked?: boolean;
  [key: string]: unknown;
}

export interface WalletPayMethod {
  id?: "ali" | "wx" | "paypal" | "usdt" | "balance" | string;
  name?: string;
  thumb?: string;
  href?: string;
  checked?: boolean;
  [key: string]: unknown;
}

export interface WalletBalance {
  coin?: string | number;
  score?: string | number;
  votes?: string | number;
  paylist?: WalletPayMethod[];
  rules?: WalletRule[];
  aliapp_partner?: string;
  aliapp_seller_id?: string;
  aliapp_key_android?: string;
  wx_appid?: string;
  [key: string]: unknown;
}

export interface CashAccount {
  id?: string | number;
  type?: string | number;
  account_bank?: string;
  account?: string;
  name?: string;
  addtime?: string | number;
  [key: string]: unknown;
}

export interface DailyTaskItem {
  id?: string | number;
  title?: string;
  tip?: string;
  tip_m?: string;
  reward?: string | number;
  state?: string | number;
  status?: string | number;
  [key: string]: unknown;
}

export interface DailyTaskBundle {
  tip_m?: string;
  list?: DailyTaskItem[];
  [key: string]: unknown;
}

export interface InviteCode {
  code?: string;
  href?: string;
  qr?: string;
  qrcode?: string;
  url?: string;
  link?: string;
  [key: string]: unknown;
}

export interface InviteAgentState {
  agent_switch?: string | number;
  agent_must?: string | number;
  has_agent?: string | number;
  openinstall_switch?: string | number;
  [key: string]: unknown;
}

export interface InviteBindResult {
  matched?: string | number;
  already_bound?: string | number;
  bound?: string | number;
  code?: string;
  inviter_uid?: string | number;
  click_id?: string;
  match_method?: string;
  confidence?: string | number;
  msg?: string;
  [key: string]: unknown;
}

export interface AuthSubmitPayload {
  realName: string;
  mobile: string;
  cardNo: string;
  frontView: string;
  backView: string;
  handsetView: string;
}

export interface UserMenuItem {
  id?: string | number;
  name?: string;
  thumb?: string;
  href?: string;
  [key: string]: unknown;
}

export interface LiveRoom {
  uid?: string;
  liveuid?: string;
  stream?: string;
  pull?: string;
  flvpull?: string;
  user_nicename?: string;
  user_nickname?: string;
  title?: string;
  avatar?: string;
  avatar_thumb?: string;
  thumb?: string;
  city?: string;
  nums?: string | number;
  hotvotes?: string | number;
  type?: string | number;
  type_val?: string;
  [key: string]: unknown;
}

export interface LiveHome {
  slide?: Record<string, unknown>[];
  list?: LiveRoom[];
  [key: string]: unknown;
}

export interface FollowLiveHome {
  title?: string;
  des?: string;
  list?: LiveRoom[];
  [key: string]: unknown;
}

export interface LotteryCategory {
  id?: string | number;
  name?: string;
  name_en?: string;
  icon?: string;
  icon_url?: string;
  [key: string]: unknown;
}

export interface LotteryGame {
  id?: string | number;
  category_id?: string | number;
  game_code?: string;
  game_name?: string;
  game_name_en?: string;
  icon?: string;
  icon_url?: string;
  current_issue?: Record<string, unknown>;
  [key: string]: unknown;
}

export interface LotteryHome {
  coin?: string;
  categories?: LotteryCategory[];
  games?: LotteryGame[];
  [key: string]: unknown;
}

export interface SportsMatch {
  id?: string | number;
  match_id?: string | number;
  public_match_id?: string | number;
  home_name?: string;
  away_name?: string;
  home_team?: string | Record<string, unknown>;
  away_team?: string | Record<string, unknown>;
  home_logo?: string;
  away_logo?: string;
  home_score?: string | number;
  away_score?: string | number;
  competition_type?: string;
  league_name?: string;
  status_text?: string;
  bet_status?: string | number;
  bet_status_text?: string;
  kickoff_ts?: string | number;
  kickoff_time?: string | number;
  bet_close_ts?: string | number;
  bet_close_time?: string | number;
  kickoff_text?: string;
  kickoff_date_text?: string;
  kickoff_clock_text?: string;
  kickoff_time_text?: string;
  trend?: string;
  prediction?: Array<Record<string, unknown>>;
  match_time?: string;
  [key: string]: unknown;
}

export interface SportsHome {
  selected_tab?: string;
  selected_date?: string;
  selected_competition_type?: string;
  updated_at?: string;
  server_time?: string | number;
  timezone?: string;
  timezone_offset?: string | number;
  tabs?: Array<Record<string, unknown>>;
  matches?: SportsMatch[];
  upcoming?: SportsMatch[];
  top_leagues?: Array<Record<string, unknown>>;
  competitions?: Array<Record<string, unknown>>;
  quick_stats?: Array<Record<string, unknown>>;
  analysis?: Array<Record<string, unknown>>;
  matches_title?: string;
  quick_stats_title?: string;
  [key: string]: unknown;
}

export interface SportsBetRecord {
  id?: string | number;
  orderid?: string | number;
  match_id?: string | number;
  match_name?: string;
  home_name?: string;
  away_name?: string;
  bet_name?: string;
  odds?: string | number;
  money?: string | number;
  bet_money?: string | number;
  win_money?: string | number;
  status_text?: string;
  addtime?: string;
  [key: string]: unknown;
}

export interface SportsBetRecordBundle {
  total_bet?: string | number;
  total_payout?: string | number;
  profit_loss?: string | number;
  net_amount?: string | number;
  list?: SportsBetRecord[];
  items?: SportsBetRecord[];
  orders?: SportsBetRecord[];
  [key: string]: unknown;
}

export interface DynamicItem {
  id?: string | number;
  dynamicid?: string | number;
  uid?: string | number;
  type?: string | number;
  title?: string;
  thumb?: string;
  href?: string;
  voice?: string;
  length?: string | number;
  datetime?: string;
  likes?: string | number;
  comments?: string | number;
  islike?: string | number;
  userinfo?: UserProfile;
  [key: string]: unknown;
}

export interface DynamicComment {
  id?: string | number;
  commentid?: string | number;
  uid?: string | number;
  touid?: string | number;
  content?: string;
  datetime?: string;
  likes?: string | number;
  islike?: string | number;
  replys?: string | number;
  replylist?: DynamicComment[];
  userinfo?: UserProfile;
  touserinfo?: UserProfile;
  [key: string]: unknown;
}

export interface DynamicCommentBundle {
  comments?: string | number;
  commentlist?: DynamicComment[];
  [key: string]: unknown;
}

export interface LiveGift {
  id?: string | number;
  giftname?: string;
  name?: string;
  gifticon?: string;
  needcoin?: string | number;
  [key: string]: unknown;
}

export interface LiveGiftBundle {
  giftlist?: LiveGift[];
  proplist?: LiveGift[];
  coin?: string | number;
  [key: string]: unknown;
}

export interface RedPackItem {
  id?: string | number;
  uid?: string | number;
  type?: string | number;
  type_grant?: string | number;
  coin?: string | number;
  nums?: string | number;
  des?: string;
  second?: string | number;
  isrob?: string | number;
  avatar?: string;
  avatar_thumb?: string;
  user_nickname?: string;
  user_nicename?: string;
  [key: string]: unknown;
}

export interface RedPackRobBundle {
  redinfo?: RedPackItem;
  list?: Record<string, unknown>[];
  win?: string | number;
  msg?: string;
  [key: string]: unknown;
}

export interface Conversation {
  id?: string;
  conversationID?: string;
  conversation_type?: "single" | "group";
  uid?: string;
  touid?: string;
  groupID?: string;
  group_id?: string;
  group_name?: string;
  user_nicename?: string;
  avatar?: string;
  last_msg?: string;
  content?: string;
  addtime?: string;
  unread?: string | number;
  [key: string]: unknown;
}

export interface ChatMessage {
  id?: string | number;
  client_msg_id?: string;
  server_msg_id?: string;
  uid?: string;
  from_uid?: string;
  touid?: string;
  content?: string;
  image?: string;
  voice?: string;
  voice_duration?: number;
  video?: string;
  video_cover?: string;
  file?: string;
  file_name?: string;
  file_size?: number;
  sender_name?: string;
  sender_avatar?: string;
  group_id?: string;
  content_type?: number;
  system?: boolean;
  type?: string | number;
  addtime?: string;
  is_self?: boolean;
  [key: string]: unknown;
}

export interface ChatGroup {
  groupID: string;
  groupName: string;
  notification?: string;
  introduction?: string;
  faceURL?: string;
  ownerUserID?: string;
  memberCount?: number;
  status?: number;
  groupType?: number;
  needVerification?: number;
  createTime?: number;
  [key: string]: unknown;
}

export interface ChatGroupMember {
  groupID: string;
  userID: string;
  nickname?: string;
  faceURL?: string;
  roleLevel?: number;
  muteEndTime?: number;
  joinTime?: number;
  [key: string]: unknown;
}

export interface ChatGroupApplication {
  groupID: string;
  groupName?: string;
  userID: string;
  nickname?: string;
  userFaceURL?: string;
  reqMsg?: string;
  handleResult?: number;
  reqTime?: number;
  [key: string]: unknown;
}

export interface UploadResult {
  file?: string;
  file_name?: string;
  filepath?: string;
  url?: string;
}

export interface VideoItem {
  id?: string | number;
  videoid?: string | number;
  uid?: string | number;
  title?: string;
  thumb?: string;
  href?: string;
  video_url?: string;
  video_thumb?: string;
  datetime?: string;
  likes?: string | number;
  comments?: string | number;
  userinfo?: UserProfile;
  [key: string]: unknown;
}

/* ---------- 小游戏 ---------- */

export interface MiniGameItem {
  id?: string;
  code?: string;
  name?: string;
  name_en?: string;
  category?: string;
  cover?: string;
  entry_type?: string;
  players_text?: string;
  play_mode?: string;
  need_login?: string;
  use_wallet?: string;
  orientation?: string;
  remark?: string;
  is_hot?: string;
  is_new?: string;
}

export interface MiniGameCategory {
  key?: string;
  name?: string;
  count?: string;
  games?: MiniGameItem[];
}

export interface MiniGameBundle {
  total?: string;
  games?: MiniGameItem[];
  categories?: MiniGameCategory[];
}

export interface MiniGameLaunch extends MiniGameItem {
  launch_url?: string;
  nickname?: string;
}
