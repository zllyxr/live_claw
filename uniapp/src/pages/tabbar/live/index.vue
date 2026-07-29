<template>
  <view class="home-page">
    <view class="home-header">
      <view class="brand-lockup">
        <image class="brand-logo" src="/static/brand/icon-round.webp" mode="aspectFit" />
        <text class="brand-name">星域</text>
      </view>

      <view class="header-search" @tap="openSearch">
        <image src="/static/native/home_hot_search_dark.png" mode="aspectFit" />
        <text>搜索主播、赛事、游戏</text>
      </view>

      <view class="message-button" @tap="openMessages">
        <image src="/static/icons/nav-bell.svg" mode="aspectFit" />
      </view>

      <view class="wallet-chip" @tap="openWallet">
        <image src="/static/brand/icon-round.webp" mode="aspectFit" />
        <text>{{ balanceText }}</text>
      </view>
    </view>

    <swiper class="hero-swiper" autoplay circular :interval="4800" :duration="500" @change="onHeroChange">
      <swiper-item v-for="slide in heroSlides" :key="slide.image">
        <view class="hero-card" @tap="slide.action">
          <image class="hero-image" :src="slide.image" mode="aspectFill" />
          <view class="hero-mask" />
          <view class="hero-copy" :class="slide.copyClass">
            <text class="hero-kicker">{{ slide.kicker }}</text>
            <text class="hero-title">{{ slide.title }}</text>
            <text class="hero-subtitle">{{ slide.subtitle }}</text>
            <view class="hero-action">{{ slide.actionText }}</view>
          </view>
        </view>
      </swiper-item>
    </swiper>
    <view class="hero-dots">
      <view
        v-for="(_, index) in heroSlides"
        :key="index"
        class="hero-dot"
        :class="{ active: heroIndex === index }"
      />
    </view>

    <view class="quick-nav">
      <view
        v-for="entry in quickEntries"
        :key="entry.key"
        class="quick-entry"
        @tap="entry.action"
      >
        <image class="quick-icon" :src="entry.icon" mode="aspectFit" />
        <text class="quick-title">{{ entry.name }}</text>
        <text class="quick-subtitle">{{ entry.subtitle }}</text>
      </view>
    </view>

    <view class="section">
      <view class="section-heading">
        <view class="section-title-group">
          <image src="/static/native/hot_live.png" mode="aspectFit" />
          <text class="section-title">今日精选</text>
        </view>
        <text class="section-more" @tap="scrollToLive">更多推荐</text>
      </view>

      <scroll-view scroll-x class="featured-scroll" :show-scrollbar="false">
        <view class="horizontal-content">
          <view v-if="featuredRoom" class="featured-card media-card" @tap="openRoom(featuredRoom)">
            <SafeImage
              class="featured-media"
              :src="coverOf(featuredRoom)"
              :fallback="coverFallback(featuredRoom)"
              mode="aspectFill"
            />
            <view class="featured-badge">LIVE</view>
            <view class="featured-gradient">
              <text class="featured-title">{{ anchorName(featuredRoom) || "热门主播" }}</text>
              <text class="featured-copy">{{ featuredRoom.title || "精彩直播进行中" }}</text>
            </view>
          </view>

          <view v-if="featuredMatch" class="featured-card match-feature" @tap="openMatch(featuredMatch)">
            <view class="featured-card-head">
              <text class="featured-label">热门赛事</text>
              <text>{{ statusText(featuredMatch) }}</text>
            </view>
            <view class="featured-team-row">
              <SafeImage
                :src="teamLogo(featuredMatch, 'home')"
                fallback="/static/icons/league-default.svg"
                mode="aspectFit"
              />
              <text class="featured-score">{{ scoreText(featuredMatch) }}</text>
              <SafeImage
                :src="teamLogo(featuredMatch, 'away')"
                fallback="/static/icons/league-default.svg"
                mode="aspectFit"
              />
            </view>
            <text class="featured-title dark">{{ teamName(featuredMatch, "home") }}</text>
            <text class="featured-copy dark">对阵 {{ teamName(featuredMatch, "away") }}</text>
          </view>

          <view v-if="featuredLottery" class="featured-card lottery-feature-card" @tap="openLotteryGame(featuredLottery)">
            <view class="featured-card-head">
              <text class="featured-label lottery">开奖中</text>
              <text>{{ issueNumber(featuredLottery) }}</text>
            </view>
            <SafeImage
              class="featured-lottery-icon"
              :src="gameIcon(featuredLottery)"
              fallback="/static/art/category/lottery.webp"
              mode="aspectFit"
            />
            <text class="featured-title dark">{{ lotteryName(featuredLottery) }}</text>
            <text class="featured-countdown">{{ lotteryCountdown(featuredLottery) }}</text>
          </view>

          <view class="featured-card media-card" @tap="launchFish">
            <image
              class="featured-media"
              src="/static/art/home/home-fishing-banner.webp"
              mode="aspectFill"
            />
            <view class="featured-gradient">
              <text class="featured-title">{{ fishGame?.name || "深海猎手" }}</text>
              <text class="featured-copy">{{ fishGame?.players_text || "1-4人实时对战" }}</text>
            </view>
          </view>
        </view>
      </scroll-view>
    </view>

    <view class="section">
      <view class="section-heading">
        <view class="section-title-group">
          <image src="/static/art/home/home-entry-sports.webp" mode="aspectFit" />
          <text class="section-title">体育赛事</text>
        </view>
        <text class="section-more" @tap="openSports">更多赛事</text>
      </view>

      <scroll-view v-if="featuredMatches.length" scroll-x class="sports-scroll" :show-scrollbar="false">
        <view class="horizontal-content">
          <view
            v-for="match in featuredMatches"
            :key="matchKey(match)"
            class="sports-card"
            @tap="openMatch(match)"
          >
            <view class="sports-card-head">
              <text class="sports-league">{{ match.competition_type || match.league_name || "足球赛事" }}</text>
              <text class="sports-status" :class="{ live: isLiveMatch(match) }">{{ statusText(match) }}</text>
            </view>
            <view class="sports-teams">
              <view class="sports-team">
                <SafeImage
                  :src="teamLogo(match, 'home')"
                  fallback="/static/icons/league-default.svg"
                  mode="aspectFit"
                />
                <text>{{ teamName(match, "home") }}</text>
              </view>
              <view class="sports-score">
                <text>{{ scoreText(match) }}</text>
                <text>{{ match.kickoff_clock_text || match.kickoff_time_text || match.kickoff_text || "" }}</text>
              </view>
              <view class="sports-team">
                <SafeImage
                  :src="teamLogo(match, 'away')"
                  fallback="/static/icons/league-default.svg"
                  mode="aspectFit"
                />
                <text>{{ teamName(match, "away") }}</text>
              </view>
            </view>
            <view v-if="oddsFor(match).length" class="odds-row">
              <view v-for="option in oddsFor(match)" :key="String(option.id || option.option_code)" class="odds-chip">
                <text>{{ option.option_name || "赔率" }}</text>
                <text>{{ formatOdds(option.odds) }}</text>
              </view>
            </view>
            <view v-else class="odds-empty">点击查看赛事详情</view>
          </view>
        </view>
      </scroll-view>
      <view v-else class="section-state">{{ loading.sports ? "赛事同步中" : sportsError || "暂无可用赛事" }}</view>
    </view>

    <view class="section lottery-section">
      <view class="section-heading">
        <view class="section-title-group">
          <image src="/static/art/home/home-entry-lottery.webp" mode="aspectFit" />
          <text class="section-title">彩票中心</text>
        </view>
        <text class="section-more" @tap="openLotteryZone">更多玩法</text>
      </view>

      <view v-if="featuredLottery" class="lottery-panel">
        <view class="lottery-primary" @tap="openLotteryGame(featuredLottery)">
          <view class="lottery-primary-head">
            <view>
              <text class="lottery-primary-title">{{ lotteryName(featuredLottery) }}</text>
              <text class="lottery-issue">第 {{ issueNumber(featuredLottery) }} 期</text>
            </view>
            <SafeImage
              class="lottery-primary-icon"
              :src="gameIcon(featuredLottery)"
              fallback="/static/art/category/lottery.webp"
              mode="aspectFit"
            />
          </view>
          <text class="lottery-count-label">实时封盘倒计时</text>
          <text class="lottery-count-value">{{ lotteryCountdown(featuredLottery) }}</text>
          <view class="lottery-action">立即购彩</view>
        </view>

        <view class="lottery-secondary">
          <view
            v-for="game in secondaryLotteryGames"
            :key="String(game.id || game.game_code)"
            class="lottery-mini"
            @tap="openLotteryGame(game)"
          >
            <SafeImage
              :src="gameIcon(game)"
              fallback="/static/art/category/lottery.webp"
              mode="aspectFit"
            />
            <view>
              <text>{{ lotteryName(game) }}</text>
              <text>{{ issueNumber(game) }}期</text>
            </view>
          </view>
        </view>
      </view>
      <view v-else class="section-state">{{ loading.lottery ? "彩票数据同步中" : lotteryError || "暂无彩票数据" }}</view>
    </view>

    <view class="section">
      <view class="section-heading">
        <view class="section-title-group">
          <image src="/static/art/home/home-entry-fishing.webp" mode="aspectFit" />
          <text class="section-title">捕鱼达人</text>
        </view>
        <text class="section-more" @tap="openFishingZone">更多游戏</text>
      </view>

      <view class="fishing-banner" @tap="launchFish">
        <image src="/static/art/home/home-fishing-banner.webp" mode="aspectFill" />
        <view class="fishing-copy">
          <text class="fishing-kicker">多人实时 · 统一结算</text>
          <text class="fishing-title">{{ fishGame?.name || "深海猎手" }}</text>
          <text class="fishing-subtitle">{{ fishGame?.remark || "探索黄金海域，挑战深海巨兽" }}</text>
          <view class="fishing-action">{{ launchingFish ? "正在进入" : "快速开始" }}</view>
        </view>
      </view>
    </view>

    <view id="live-section" class="section live-section">
      <view class="section-heading">
        <view class="section-title-group">
          <image src="/static/native/hot_live.png" mode="aspectFit" />
          <text class="section-title">热门直播</text>
        </view>
        <text class="section-more" @tap="openFollow">我的关注</text>
      </view>

      <view v-if="visibleRooms.length" class="live-grid">
        <view
          v-for="room in visibleRooms"
          :key="String(room.uid || room.stream)"
          class="live-card"
          @tap="openRoom(room)"
        >
          <view class="live-cover">
            <SafeImage :src="coverOf(room)" :fallback="coverFallback(room)" mode="aspectFill" />
            <view class="live-status">直播中</view>
            <view class="live-count">{{ displayCount(room.nums || room.hotvotes) }}人</view>
            <view class="live-copy">
              <text>{{ room.title || "星域直播间" }}</text>
              <text>{{ anchorName(room) || "主播" }}</text>
            </view>
          </view>
        </view>
      </view>
      <EmptyState
        v-else-if="!loading.live"
        kind="live"
        title="暂无直播房间"
        :description="liveError || '下拉页面可重新刷新。'"
      />
      <view v-else class="section-state">直播房间同步中</view>

      <view v-if="rooms.length && (!liveExpanded || !liveFinished)" class="load-more" @tap="showMoreLive">
        {{ liveExpanded ? (loading.live ? "加载中" : "加载更多") : "查看更多直播" }}
      </view>
    </view>

    <FishingVenuePicker
      :visible="venuePickerVisible"
      :venues="fishingVenues"
      :balance="lotteryHome?.coin"
      @close="venuePickerVisible = false"
      @select="launchFishingVenue"
    />
  </view>
</template>

<script setup lang="ts">
import { computed, reactive, ref } from "vue";
import { onHide, onPullDownRefresh, onReachBottom, onShow, onUnload } from "@dcloudio/uni-app";
import EmptyState from "@/components/EmptyState.vue";
import FishingVenuePicker from "@/components/FishingVenuePicker.vue";
import SafeImage from "@/components/SafeImage.vue";
import {
  enterMiniGame,
  getHomeDashboard,
  getHotLive,
} from "@/api/services";
import type {
  HomeDashboard,
  HomeFishingVenue,
  FishingVenue,
  LiveRoom,
  LotteryGame,
  LotteryHome,
  MiniGameBundle,
  MiniGameItem,
  SportsHome,
  SportsMarketOption,
  SportsMatch
} from "@/types/api";
import { displayCount } from "@/utils/format";
import {
  openGameView,
  openGameZone,
  openSportsDetail
} from "@/utils/navigation";
import { isLoggedIn, requireLogin } from "@/utils/session";
import { absolutizeUrl, displayUrl, firstText } from "@/utils/url";

const rooms = ref<LiveRoom[]>([]);
const sportsHome = ref<SportsHome>();
const lotteryHome = ref<LotteryHome>();
const miniGames = ref<MiniGameBundle>();
const oddsByMatch = ref<Record<string, SportsMarketOption[]>>({});

const loading = reactive({
  live: false,
  sports: false,
  lottery: false,
  games: false
});

const liveError = ref("");
const sportsError = ref("");
const lotteryError = ref("");
const gamesError = ref("");
const livePage = ref(1);
const liveFinished = ref(false);
const liveExpanded = ref(false);
const launchingFish = ref(false);
const venuePickerVisible = ref(false);
const fishingVenues = ref<FishingVenue[]>([]);
const heroIndex = ref(0);
const loggedIn = ref(isLoggedIn());
const nowSeconds = ref(Math.floor(Date.now() / 1000));
let loadedOnce = false;
let clockTimer: ReturnType<typeof setInterval> | undefined;

const COVERS = [
  "/static/art/cover/cover1.webp",
  "/static/art/cover/cover2.webp",
  "/static/art/cover/cover3.webp",
  "/static/art/cover/cover4.webp"
];

const heroSlides = [
  {
    image: "/static/art/home/home-hero-live.webp",
    kicker: "精彩赛事 · 热门直播",
    title: "每一场精彩 都在星域",
    subtitle: "实时赛事与人气主播一站直达",
    actionText: "查看赛事",
    copyClass: "hero-copy-center",
    action: openSports
  },
  {
    image: "/static/art/home/home-hero-games.webp",
    kicker: "彩票游戏 · 深海捕鱼",
    title: "多种玩法 随时开局",
    subtitle: "平台余额统一结算，过程清晰透明",
    actionText: "进入游戏",
    copyClass: "",
    action: openFishingZone
  }
];

const quickEntries = [
  {
    key: "live",
    name: "直播",
    subtitle: "热门主播",
    icon: "/static/art/home/home-entry-live.webp",
    action: scrollToLive
  },
  {
    key: "sports",
    name: "体育",
    subtitle: "实时赛事",
    icon: "/static/art/home/home-entry-sports.webp",
    action: openSports
  },
  {
    key: "lottery",
    name: "彩票",
    subtitle: "多种彩票",
    icon: "/static/art/home/home-entry-lottery.webp",
    action: openLotteryZone
  },
  {
    key: "fishing",
    name: "捕鱼",
    subtitle: "多人在线",
    icon: "/static/art/home/home-entry-fishing.webp",
    action: openFishingZone
  }
];

const featuredRoom = computed(() => rooms.value[0]);
const visibleRooms = computed(() => (liveExpanded.value ? rooms.value : rooms.value.slice(0, 6)));

const featuredMatches = computed(() => {
  const source = [...(sportsHome.value?.matches || []), ...(sportsHome.value?.upcoming || [])];
  const seen = new Set<string>();
  const ordered = source
    .filter((match) => {
      const key = matchKey(match);
      if (!key || seen.has(key)) {
        return false;
      }
      seen.add(key);
      return true;
    })
    .sort((a, b) => matchPriority(a) - matchPriority(b));
  const live = ordered.filter(isLiveMatch);
  const bettable = ordered.filter(
    (match) => !isLiveMatch(match) && String(match.bet_status || "") === "1"
  );
  const remaining = ordered.filter(
    (match) => !isLiveMatch(match) && String(match.bet_status || "") !== "1"
  );
  return [...live.slice(0, 1), ...bettable, ...live.slice(1), ...remaining].slice(0, 3);
});

const featuredMatch = computed(() => featuredMatches.value[0]);
const lotteryGames = computed(() => (lotteryHome.value?.games || []).slice(0, 4));
const featuredLottery = computed(() => lotteryGames.value[0]);
const secondaryLotteryGames = computed(() => lotteryGames.value.slice(1, 4));
const fishGame = computed<MiniGameItem | undefined>(() => {
  const games = miniGames.value?.games || [];
  return games.find((game) => String(game.code || "") === "deepsea_hunter") || games[0];
});

const balanceText = computed(() => {
  if (!loggedIn.value) {
    return "登录";
  }
  const raw = String(lotteryHome.value?.coin || "0");
  const value = Number(raw);
  if (Number.isFinite(value) && Math.abs(value) >= 10000) {
    return `${(value / 10000).toFixed(value >= 100000 ? 0 : 1)}万 星币`;
  }
  return `${raw} 星币`;
});

function onHeroChange(event: { detail?: { current?: number } }) {
  heroIndex.value = Number(event.detail?.current || 0);
}

function coverFallback(room: LiveRoom) {
  const seed = String(room.uid || room.stream || "0");
  let hash = 0;
  for (let i = 0; i < seed.length; i++) {
    hash = (hash * 31 + seed.charCodeAt(i)) % 997;
  }
  return COVERS[hash % COVERS.length] as string;
}

function coverOf(room: LiveRoom) {
  return absolutizeUrl(firstText(room.thumb, room.avatar_thumb, room.avatar));
}

function avatarOf(room: LiveRoom) {
  return absolutizeUrl(firstText(room.avatar_thumb, room.avatar)) || "/static/icons/avatar-anchor.svg";
}

function anchorName(room: LiveRoom) {
  return firstText(room.user_nicename, room.user_nickname);
}

function streamSourceOf(room: LiveRoom) {
  return firstText(room.pull, room.flvpull, room.stream);
}

function matchKey(match: SportsMatch) {
  return String(match.match_id || match.public_match_id || match.id || "");
}

function isLiveMatch(match: SportsMatch) {
  const status = String(match.status_text || "").toLowerCase();
  return (
    status.includes("live") ||
    status.includes("进行") ||
    status.includes("first half") ||
    status.includes("second half") ||
    status.includes("上半") ||
    status.includes("下半")
  );
}

function isFinishedMatch(match: SportsMatch) {
  const status = String(match.status_text || "").toLowerCase();
  return status.includes("finished") || status.includes("结束") || status.includes("完场");
}

function matchPriority(match: SportsMatch) {
  if (isLiveMatch(match)) return 0;
  if (String(match.bet_status || "") === "1") return 1;
  if (!isFinishedMatch(match)) return 2;
  return 3;
}

function statusText(match: SportsMatch) {
  const value = String(match.status_text || match.bet_status_text || "");
  const lower = value.toLowerCase();
  if (isLiveMatch(match)) return "进行中";
  if (lower.includes("not started")) return "未开始";
  if (lower.includes("finished")) return "已结束";
  return value || "待开赛";
}

function teamName(match: SportsMatch, side: "home" | "away") {
  const direct = side === "home" ? match.home_name : match.away_name;
  const team = side === "home" ? match.home_team : match.away_team;
  if (direct) return String(direct);
  if (team && typeof team === "object") return String(team.name || "球队");
  return String(team || "球队");
}

function teamLogo(match: SportsMatch, side: "home" | "away") {
  const direct = side === "home" ? match.home_logo : match.away_logo;
  const team = side === "home" ? match.home_team : match.away_team;
  if (direct) return absolutizeUrl(String(direct));
  if (team && typeof team === "object") return absolutizeUrl(String(team.logo || ""));
  return "/static/icons/league-default.svg";
}

function scoreText(match: SportsMatch) {
  const home = String(match.home_score ?? "-");
  const away = String(match.away_score ?? "-");
  return home === "-" && away === "-" ? "VS" : `${home} - ${away}`;
}

function oddsFor(match: SportsMatch) {
  return oddsByMatch.value[matchKey(match)] || [];
}

function formatOdds(value?: string | number) {
  const parsed = Number(value || 0);
  return parsed > 0 ? parsed.toFixed(2) : "-";
}

function lotteryName(game: LotteryGame) {
  return String(game.game_name || game.game_name_en || "彩票游戏");
}

function gameIcon(game: LotteryGame) {
  const originalIcon = `/static/lotter/${String(game.game_code || "").toUpperCase()}.png`;
  return displayUrl(game.icon_url || game.icon || "", originalIcon);
}

function issueOf(game: LotteryGame) {
  return (game.current_issue || {}) as Record<string, unknown>;
}

function issueNumber(game: LotteryGame) {
  return String(issueOf(game).issue_num || "--");
}

function lotteryCountdown(game: LotteryGame) {
  const issue = issueOf(game);
  const deadline = Number(issue.seal_time || issue.open_time || 0);
  const fallback = Number(issue.seal_countdown || issue.open_countdown || 0);
  const seconds = deadline > 0 ? Math.max(0, deadline - nowSeconds.value) : Math.max(0, fallback);
  if (seconds <= 0) {
    return "等待开奖";
  }
  const hours = Math.floor(seconds / 3600);
  const minutes = Math.floor((seconds % 3600) / 60);
  const remain = seconds % 60;
  return [hours, minutes, remain].map((item) => String(item).padStart(2, "0")).join(":");
}

async function loadRooms(reset = false) {
  if (loading.live || (liveFinished.value && !reset)) {
    return;
  }
  loading.live = true;
  liveError.value = "";
  if (reset) {
    livePage.value = 1;
    liveFinished.value = false;
  }
  try {
    const list = await getHotLive(livePage.value);
    if (reset) {
      rooms.value = list;
    } else {
      const known = new Set(rooms.value.map((room) => String(room.uid || room.stream)));
      rooms.value = rooms.value.concat(
        list.filter((room) => !known.has(String(room.uid || room.stream)))
      );
    }
    if (!list.length) {
      liveFinished.value = true;
    } else {
      livePage.value += 1;
    }
  } catch {
    liveError.value = "直播数据暂时不可用，请下拉重试";
  } finally {
    loading.live = false;
  }
}

async function loadSportsOdds(matches: SportsMatch[]) {
  const next: Record<string, SportsMarketOption[]> = {};
  matches.forEach((match) => {
    const key = matchKey(match);
    const options = (match as SportsMatch & { options?: SportsMarketOption[] }).options || [];
    if (key && options.length) {
      next[key] = options.slice(0, 3);
    }
  });
  oddsByMatch.value = next;
}

function sectionError(section: { status?: string } | undefined, message: string) {
  return section?.status === "degraded" ? message : "";
}

function normalizeAggregateSports(payload: HomeDashboard) {
  const matches = (payload.sports?.items || []).map((item) => ({
    ...item,
    status_text: item.status || item.status_text || "",
    kickoff_ts: item.kickoff_at || item.kickoff_ts || 0,
    bet_close_ts: item.bet_close_at || item.bet_close_ts || 0
  }));
  sportsHome.value = {
    server_time: payload.server_time,
    matches,
    upcoming: []
  };
  loadSportsOdds(matches);
}

function normalizeAggregateLottery(payload: HomeDashboard) {
  const games = (payload.lottery?.items || []).map((game) => {
    const issue = (game.current_issue || {}) as Record<string, unknown>;
    return {
      ...game,
      current_issue: {
        ...issue,
        issue_num: issue.issue_num || issue.issue_number || "",
        seal_time: issue.seal_time || issue.close_at || 0,
        open_time: issue.open_time || issue.draw_at || 0
      }
    };
  });
  lotteryHome.value = {
    coin: String(payload.wallet?.coin || 0),
    games
  };
}

function aggregateFishingGame(venues: HomeFishingVenue[]): MiniGameItem | undefined {
  const venue = venues[0];
  if (!venue) {
    return undefined;
  }
  return {
    id: String(venue.game_id || ""),
    code: venue.game_code || "deepsea_hunter",
    name: venue.game_name || "深海猎手",
    category: "fishing",
    entry_type: "internal",
    players_text: `1-${venue.seats_per_table || 4}人`,
    play_mode: "match",
    need_login: "1",
    use_wallet: "1",
    orientation: "landscape",
    remark: `${venues.length || 3}个倍率场，随机分配桌位`
  };
}

function applyHomeDashboard(payload: HomeDashboard) {
  rooms.value = payload.live?.items || [];
  livePage.value = rooms.value.length ? 2 : 1;
  liveFinished.value = rooms.value.length < 6;
  normalizeAggregateSports(payload);
  normalizeAggregateLottery(payload);
  const game = aggregateFishingGame(payload.fishing?.items || []);
  fishingVenues.value = payload.fishing?.items || [];
  miniGames.value = {
    total: game ? "1" : "0",
    games: game ? [game] : [],
    categories: []
  };
  liveError.value = sectionError(payload.live, "直播数据暂时不可用，请下拉重试");
  sportsError.value = sectionError(payload.sports, "赛事数据暂时不可用，请下拉重试");
  lotteryError.value = sectionError(payload.lottery, "彩票数据暂时不可用，请下拉重试");
  gamesError.value = sectionError(payload.fishing, "游戏数据暂时不可用，请下拉重试");
}

async function loadDashboard() {
  loading.live = true;
  loading.sports = true;
  loading.lottery = true;
  loading.games = true;
  try {
    const payload = await getHomeDashboard();
    if (!payload) {
      throw new Error("首页数据为空");
    }
    applyHomeDashboard(payload);
  } catch {
    liveError.value = "直播数据暂时不可用，请下拉重试";
    sportsError.value = "赛事数据暂时不可用，请下拉重试";
    lotteryError.value = "彩票数据暂时不可用，请下拉重试";
    gamesError.value = "游戏数据暂时不可用，请下拉重试";
  } finally {
    loading.live = false;
    loading.sports = false;
    loading.lottery = false;
    loading.games = false;
    uni.stopPullDownRefresh();
  }
}

function openRoom(room: LiveRoom) {
  const src = streamSourceOf(room);
  const liveUid = firstText(room.liveuid, room.uid);
  const stream = firstText(room.stream);
  uni.navigateTo({
    url:
      `/pages/live/player?title=${encodeURIComponent(String(room.title || anchorName(room) || "直播间"))}` +
      `&src=${encodeURIComponent(src)}&cover=${encodeURIComponent(coverOf(room))}` +
      `&liveuid=${encodeURIComponent(liveUid)}&stream=${encodeURIComponent(stream)}` +
      `&avatar=${encodeURIComponent(avatarOf(room))}&anchor=${encodeURIComponent(String(anchorName(room) || "主播"))}` +
      `&nums=${encodeURIComponent(String(room.nums || 0))}&votes=${encodeURIComponent(String(room.hotvotes || 0))}`
  });
}

function openSearch() {
  uni.navigateTo({ url: "/pages/search/index" });
}

function openMessages() {
  if (requireLogin()) {
    uni.navigateTo({ url: "/pages/message/index" });
  }
}

function openWallet() {
  if (requireLogin()) {
    uni.navigateTo({ url: "/pages/wallet/recharge" });
  }
}

function openFollow() {
  if (requireLogin()) {
    uni.navigateTo({ url: "/pages/live/follow" });
  }
}

function openSports() {
  uni.switchTab({ url: "/pages/tabbar/sports/index" });
}

function openLotteryZone() {
  openGameZone("lottery");
}

function openFishingZone() {
  openGameZone("fishing");
}

function scrollToLive() {
  uni.pageScrollTo({ selector: "#live-section", duration: 360 });
}

function openMatch(match: SportsMatch) {
  if (!requireLogin()) {
    return;
  }
  openSportsDetail(match);
}

function openLotteryGame(game: LotteryGame) {
  if (!requireLogin()) {
    return;
  }
  const query = [
    `game_id=${encodeURIComponent(String(game.id || ""))}`,
    `game_code=${encodeURIComponent(String(game.game_code || ""))}`,
    `title=${encodeURIComponent(lotteryName(game))}`
  ].join("&");
  uni.navigateTo({ url: `/pages/game/bet?${query}` });
}

async function launchFish() {
  const game = fishGame.value;
  if (!game || launchingFish.value) {
    if (!game && gamesError.value) {
      uni.showToast({ title: gamesError.value, icon: "none" });
    }
    return;
  }
  if (game.need_login === "1" && !requireLogin()) {
    return;
  }
  venuePickerVisible.value = true;
}

async function launchFishingVenue(venue: FishingVenue) {
  const game = fishGame.value;
  if (!game || launchingFish.value) return;
  venuePickerVisible.value = false;
  launchingFish.value = true;
  uni.showLoading({ title: "进入游戏", mask: true });
  try {
    const info = await enterMiniGame(
      String(game.code || "deepsea_hunter"),
      String(venue.venue_code || "novice")
    );
    const url = String(info?.launch_url || "");
    if (!url) {
      throw new Error("游戏地址无效");
    }
    openGameView(absolutizeUrl(url) || url);
  } catch (error: any) {
    uni.showToast({ title: error?.message || "进入游戏失败", icon: "none" });
  } finally {
    uni.hideLoading();
    launchingFish.value = false;
  }
}

function showMoreLive() {
  if (!liveExpanded.value) {
    liveExpanded.value = true;
    return;
  }
  void loadRooms(false);
}

function startClock() {
  if (clockTimer) {
    clearInterval(clockTimer);
  }
  nowSeconds.value = Math.floor(Date.now() / 1000);
  clockTimer = setInterval(() => {
    nowSeconds.value = Math.floor(Date.now() / 1000);
  }, 1000);
}

function stopClock() {
  if (clockTimer) {
    clearInterval(clockTimer);
    clockTimer = undefined;
  }
}

onShow(() => {
  loggedIn.value = isLoggedIn();
  startClock();
  if (!loadedOnce) {
    loadedOnce = true;
    void loadDashboard();
  } else {
    void loadDashboard();
  }
});

onHide(stopClock);
onUnload(stopClock);

onPullDownRefresh(() => {
  loggedIn.value = isLoggedIn();
  void loadDashboard();
});

onReachBottom(() => {
  if (liveExpanded.value) {
    void loadRooms(false);
  }
});
</script>

<style scoped>
.home-page {
  min-height: 100vh;
  overflow-x: hidden;
  padding: calc(22rpx + var(--status-bar-height)) 24rpx calc(144rpx + env(safe-area-inset-bottom));
  color: var(--ink);
  background: var(--bg);
}

.home-header {
  display: flex;
  min-width: 0;
  align-items: center;
  gap: 12rpx;
  margin-bottom: 20rpx;
}

.brand-lockup {
  display: flex;
  flex: 0 0 auto;
  align-items: center;
  gap: 10rpx;
}

.brand-logo {
  width: 52rpx;
  height: 52rpx;
  border-radius: 14rpx;
  box-shadow: 0 8rpx 20rpx rgba(255, 77, 110, 0.18);
}

.brand-name {
  color: var(--ink);
  font-size: 32rpx;
  font-weight: 900;
  letter-spacing: 2rpx;
}

.header-search {
  display: flex;
  min-width: 0;
  height: 64rpx;
  flex: 1;
  align-items: center;
  gap: 10rpx;
  padding: 0 18rpx;
  border: 2rpx solid var(--line);
  border-radius: 32rpx;
  background: var(--surface);
}

.header-search image {
  width: 28rpx;
  height: 28rpx;
  flex: 0 0 28rpx;
}

.header-search text {
  min-width: 0;
  color: var(--ink-3);
  font-size: 21rpx;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.message-button {
  display: flex;
  width: 58rpx;
  height: 58rpx;
  flex: 0 0 58rpx;
  align-items: center;
  justify-content: center;
  border-radius: 50%;
  background: var(--brand);
  box-shadow: 0 8rpx 18rpx rgba(255, 77, 110, 0.24);
}

.message-button image {
  width: 32rpx;
  height: 32rpx;
}

.wallet-chip {
  display: flex;
  min-width: 126rpx;
  max-width: 188rpx;
  height: 58rpx;
  flex: 0 1 auto;
  align-items: center;
  gap: 8rpx;
  padding: 0 16rpx;
  border-radius: 29rpx;
  background: #fff2e8;
}

.wallet-chip image {
  width: 32rpx;
  height: 32rpx;
  flex: 0 0 32rpx;
  border-radius: 50%;
}

.wallet-chip text {
  min-width: 0;
  color: #b86518;
  font-size: 21rpx;
  font-weight: 800;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.hero-swiper {
  width: 100%;
  height: 284rpx;
}

.hero-card {
  position: relative;
  width: 100%;
  height: 100%;
  overflow: hidden;
  border-radius: 28rpx;
  background: #10234c;
  box-shadow: var(--shadow-card);
}

.hero-image,
.hero-mask {
  position: absolute;
  inset: 0;
  width: 100%;
  height: 100%;
}

.hero-mask {
  background: linear-gradient(90deg, rgba(8, 19, 49, 0.88), rgba(8, 19, 49, 0.34) 62%, rgba(8, 19, 49, 0.06));
}

.hero-copy {
  position: relative;
  z-index: 1;
  display: flex;
  width: 64%;
  height: 100%;
  padding: 32rpx 28rpx;
  flex-direction: column;
  align-items: flex-start;
  justify-content: center;
}

.hero-copy-center {
  width: 53%;
}

.hero-kicker {
  color: #ffd66f;
  font-size: 20rpx;
  font-weight: 800;
  letter-spacing: 1rpx;
}

.hero-title {
  margin-top: 8rpx;
  color: #fff;
  font-size: 37rpx;
  font-weight: 900;
  line-height: 1.16;
  text-shadow: 0 4rpx 14rpx rgba(0, 0, 0, 0.36);
}

.hero-subtitle {
  margin-top: 8rpx;
  color: rgba(255, 255, 255, 0.76);
  font-size: 20rpx;
  line-height: 1.35;
}

.hero-action {
  display: flex;
  height: 48rpx;
  align-items: center;
  justify-content: center;
  margin-top: 17rpx;
  padding: 0 22rpx;
  border-radius: 24rpx;
  color: #fff;
  font-size: 20rpx;
  font-weight: 800;
  background: var(--grad-brand);
  box-shadow: 0 8rpx 18rpx rgba(255, 77, 110, 0.26);
}

.hero-dots {
  display: flex;
  height: 30rpx;
  align-items: center;
  justify-content: center;
  gap: 10rpx;
}

.hero-dot {
  width: 10rpx;
  height: 10rpx;
  border-radius: 5rpx;
  background: #d6d9e2;
  transition: width 0.2s ease;
}

.hero-dot.active {
  width: 30rpx;
  background: var(--brand);
}

.quick-nav {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  overflow: hidden;
  padding: 18rpx 8rpx 16rpx;
  border-radius: var(--radius);
  background: var(--surface);
  box-shadow: var(--shadow-soft);
}

.quick-entry {
  position: relative;
  display: flex;
  min-width: 0;
  align-items: center;
  flex-direction: column;
}

.quick-entry + .quick-entry {
  border-left: 2rpx solid var(--line);
}

.quick-icon {
  width: 82rpx;
  height: 82rpx;
}

.quick-title {
  margin-top: 4rpx;
  color: var(--ink);
  font-size: 24rpx;
  font-weight: 800;
}

.quick-subtitle {
  margin-top: 3rpx;
  color: var(--ink-3);
  font-size: 17rpx;
  line-height: 1.2;
}

.section {
  margin-top: 28rpx;
}

.section-heading {
  display: flex;
  height: 54rpx;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 12rpx;
}

.section-title-group {
  display: flex;
  min-width: 0;
  align-items: center;
  gap: 10rpx;
}

.section-title-group image {
  width: 38rpx;
  height: 38rpx;
  flex: 0 0 38rpx;
}

.section-title {
  color: var(--ink);
  font-size: 30rpx;
  font-weight: 900;
  letter-spacing: 0.5rpx;
}

.section-more {
  color: var(--ink-3);
  font-size: 21rpx;
}

.featured-scroll,
.sports-scroll {
  width: calc(100% + 24rpx);
  margin-right: -24rpx;
}

.horizontal-content {
  display: inline-flex;
  gap: 16rpx;
  padding-right: 24rpx;
  white-space: nowrap;
}

.featured-card {
  position: relative;
  width: 258rpx;
  height: 310rpx;
  flex: 0 0 258rpx;
  overflow: hidden;
  border-radius: 22rpx;
  background: var(--surface);
  box-shadow: var(--shadow-soft);
}

.media-card {
  background: #162551;
}

.featured-media {
  width: 100%;
  height: 100%;
}

.featured-badge {
  position: absolute;
  top: 14rpx;
  left: 14rpx;
  display: flex;
  height: 38rpx;
  align-items: center;
  padding: 0 14rpx;
  border-radius: 19rpx;
  color: #fff;
  font-size: 18rpx;
  font-weight: 900;
  background: var(--brand);
}

.featured-gradient {
  position: absolute;
  right: 0;
  bottom: 0;
  left: 0;
  padding: 72rpx 16rpx 16rpx;
  background: linear-gradient(180deg, transparent, rgba(9, 13, 27, 0.82));
}

.featured-title {
  display: block;
  max-width: 100%;
  color: #fff;
  font-size: 25rpx;
  font-weight: 900;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.featured-copy {
  display: block;
  max-width: 100%;
  margin-top: 5rpx;
  color: rgba(255, 255, 255, 0.72);
  font-size: 18rpx;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.featured-title.dark,
.featured-copy.dark {
  margin-right: 16rpx;
  margin-left: 16rpx;
  color: var(--ink);
}

.featured-copy.dark {
  margin-top: 6rpx;
  color: var(--ink-3);
}

.featured-card-head {
  display: flex;
  height: 52rpx;
  align-items: center;
  justify-content: space-between;
  padding: 0 14rpx;
  color: var(--ink-3);
  font-size: 17rpx;
}

.featured-label {
  color: #2489db;
  font-weight: 800;
}

.featured-label.lottery {
  color: var(--brand);
}

.featured-team-row {
  display: flex;
  height: 142rpx;
  align-items: center;
  justify-content: center;
  gap: 14rpx;
  background: #f7f9fd;
}

.featured-team-row :deep(.safe-image) {
  width: 62rpx;
  height: 62rpx;
  border-radius: 50%;
  background: #fff;
}

.featured-score {
  color: var(--ink);
  font-size: 31rpx;
  font-weight: 900;
}

.featured-lottery-icon {
  width: 126rpx;
  height: 126rpx;
  margin: 4rpx auto 0;
}

.featured-countdown {
  display: block;
  margin: 10rpx 16rpx 0;
  color: var(--brand);
  font-size: 24rpx;
  font-weight: 900;
}

.sports-card {
  width: 552rpx;
  min-height: 298rpx;
  flex: 0 0 552rpx;
  padding: 18rpx;
  border-radius: 24rpx;
  background: var(--surface);
  box-shadow: var(--shadow-soft);
}

.sports-card-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
}

.sports-league {
  max-width: 340rpx;
  color: var(--ink-2);
  font-size: 20rpx;
  font-weight: 700;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.sports-status {
  display: flex;
  height: 36rpx;
  align-items: center;
  padding: 0 14rpx;
  border-radius: 18rpx;
  color: var(--ink-3);
  font-size: 17rpx;
  background: var(--bg);
}

.sports-status.live {
  color: #fff;
  background: var(--brand);
}

.sports-teams {
  display: grid;
  grid-template-columns: minmax(0, 1fr) 116rpx minmax(0, 1fr);
  align-items: center;
  gap: 12rpx;
  margin-top: 18rpx;
}

.sports-team {
  display: flex;
  min-width: 0;
  align-items: center;
  flex-direction: column;
  gap: 8rpx;
}

.sports-team :deep(.safe-image) {
  width: 64rpx;
  height: 64rpx;
  border-radius: 50%;
  background: #fff;
}

.sports-team text {
  width: 100%;
  color: var(--ink-2);
  font-size: 19rpx;
  font-weight: 700;
  text-align: center;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.sports-score {
  display: flex;
  align-items: center;
  flex-direction: column;
}

.sports-score text:first-child {
  color: var(--ink);
  font-size: 34rpx;
  font-weight: 900;
}

.sports-score text:last-child {
  max-width: 116rpx;
  margin-top: 4rpx;
  color: var(--ink-3);
  font-size: 16rpx;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.odds-row {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 8rpx;
  margin-top: 18rpx;
}

.odds-chip {
  display: flex;
  height: 55rpx;
  align-items: center;
  justify-content: center;
  gap: 7rpx;
  border-radius: 14rpx;
  background: var(--bg);
}

.odds-chip text {
  color: var(--ink-2);
  font-size: 17rpx;
}

.odds-chip text:last-child {
  color: #be7d16;
  font-weight: 900;
}

.odds-empty {
  display: flex;
  height: 55rpx;
  align-items: center;
  justify-content: center;
  margin-top: 18rpx;
  border-radius: 14rpx;
  color: var(--ink-3);
  font-size: 18rpx;
  background: var(--bg);
}

.section-state {
  display: flex;
  min-height: 132rpx;
  align-items: center;
  justify-content: center;
  border-radius: var(--radius);
  color: var(--ink-3);
  font-size: 22rpx;
  background: var(--surface);
  box-shadow: var(--shadow-soft);
}

.lottery-panel {
  display: grid;
  grid-template-columns: 1.18fr 0.82fr;
  overflow: hidden;
  border-radius: var(--radius);
  background: var(--surface);
  box-shadow: var(--shadow-soft);
}

.lottery-primary {
  min-width: 0;
  padding: 22rpx;
  border-right: 2rpx solid var(--line);
}

.lottery-primary-head {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 14rpx;
}

.lottery-primary-title {
  display: block;
  color: var(--ink);
  font-size: 29rpx;
  font-weight: 900;
}

.lottery-issue {
  display: block;
  max-width: 240rpx;
  margin-top: 7rpx;
  color: var(--ink-3);
  font-size: 17rpx;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.lottery-primary-icon {
  width: 72rpx;
  height: 72rpx;
  flex: 0 0 72rpx;
}

.lottery-count-label {
  display: block;
  margin-top: 24rpx;
  color: var(--ink-3);
  font-size: 18rpx;
}

.lottery-count-value {
  display: block;
  margin-top: 5rpx;
  color: var(--brand);
  font-family: "SFMono-Regular", Consolas, monospace;
  font-size: 34rpx;
  font-weight: 900;
  letter-spacing: 1rpx;
}

.lottery-action {
  display: flex;
  width: 170rpx;
  height: 50rpx;
  align-items: center;
  justify-content: center;
  margin-top: 18rpx;
  border-radius: 25rpx;
  color: #fff;
  font-size: 20rpx;
  font-weight: 800;
  background: var(--grad-brand);
}

.lottery-secondary {
  display: flex;
  flex-direction: column;
}

.lottery-mini {
  display: flex;
  min-height: 100rpx;
  flex: 1;
  align-items: center;
  gap: 10rpx;
  padding: 12rpx;
}

.lottery-mini + .lottery-mini {
  border-top: 2rpx solid var(--line);
}

.lottery-mini :deep(.safe-image) {
  width: 50rpx;
  height: 50rpx;
  flex: 0 0 50rpx;
}

.lottery-mini > view {
  min-width: 0;
}

.lottery-mini text {
  display: block;
  max-width: 100%;
  color: var(--ink-2);
  font-size: 19rpx;
  font-weight: 700;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.lottery-mini text:last-child {
  margin-top: 4rpx;
  color: var(--ink-3);
  font-size: 15rpx;
  font-weight: 500;
}

.fishing-banner {
  position: relative;
  height: 304rpx;
  overflow: hidden;
  border-radius: var(--radius-lg);
  background: #07376c;
  box-shadow: 0 14rpx 34rpx rgba(9, 56, 109, 0.18);
}

.fishing-banner > image {
  position: absolute;
  inset: 0;
  width: 100%;
  height: 100%;
}

.fishing-copy {
  position: relative;
  z-index: 1;
  display: flex;
  width: 47%;
  height: 100%;
  padding: 30rpx 0 26rpx 26rpx;
  flex-direction: column;
  align-items: flex-start;
  justify-content: center;
}

.fishing-kicker {
  color: #74e6ff;
  font-size: 18rpx;
  font-weight: 800;
}

.fishing-title {
  margin-top: 8rpx;
  color: #fff;
  font-size: 37rpx;
  font-weight: 900;
}

.fishing-subtitle {
  display: -webkit-box;
  margin-top: 8rpx;
  color: rgba(255, 255, 255, 0.72);
  font-size: 18rpx;
  line-height: 1.35;
  overflow: hidden;
  -webkit-box-orient: vertical;
  -webkit-line-clamp: 2;
}

.fishing-action {
  display: flex;
  height: 50rpx;
  align-items: center;
  justify-content: center;
  margin-top: 18rpx;
  padding: 0 22rpx;
  border-radius: 25rpx;
  color: #6d3e00;
  font-size: 20rpx;
  font-weight: 900;
  background: #ffd06c;
}

.live-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 16rpx;
}

.live-card {
  min-width: 0;
  overflow: hidden;
  border-radius: 22rpx;
  background: var(--surface);
  box-shadow: var(--shadow-soft);
}

.live-cover {
  position: relative;
  width: 100%;
  aspect-ratio: 4 / 5;
  overflow: hidden;
  border-radius: 22rpx;
}

.live-cover :deep(.safe-image) {
  width: 100%;
  height: 100%;
}

.live-status,
.live-count {
  position: absolute;
  top: 12rpx;
  display: flex;
  height: 36rpx;
  align-items: center;
  padding: 0 12rpx;
  border-radius: 18rpx;
  color: #fff;
  font-size: 17rpx;
  font-weight: 800;
  backdrop-filter: blur(6px);
}

.live-status {
  left: 12rpx;
  background: rgba(255, 77, 110, 0.94);
}

.live-count {
  right: 12rpx;
  background: rgba(15, 17, 26, 0.44);
}

.live-copy {
  position: absolute;
  right: 0;
  bottom: 0;
  left: 0;
  padding: 72rpx 14rpx 14rpx;
  background: linear-gradient(180deg, transparent, rgba(10, 12, 20, 0.78));
}

.live-copy text {
  display: block;
  color: #fff;
  font-size: 22rpx;
  font-weight: 800;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.live-copy text:last-child {
  margin-top: 4rpx;
  color: rgba(255, 255, 255, 0.7);
  font-size: 17rpx;
  font-weight: 500;
}

.load-more {
  display: flex;
  height: 70rpx;
  align-items: center;
  justify-content: center;
  margin-top: 18rpx;
  border-radius: 35rpx;
  color: var(--brand);
  font-size: 23rpx;
  font-weight: 800;
  background: var(--brand-soft);
}

@media screen and (max-width: 360px) {
  .brand-name {
    display: none;
  }

  .wallet-chip {
    min-width: 108rpx;
    max-width: 150rpx;
  }

  .quick-subtitle {
    display: none;
  }

  .hero-copy {
    width: 70%;
  }

  .hero-title {
    font-size: 34rpx;
  }
}
</style>
