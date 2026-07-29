<template>
  <view class="sports-page">
    <view class="sports-hero">
      <view class="pitch-lines" />
      <view class="hero-glow" />
      <text class="sports-title">体育中心</text>
      <text class="sports-sub">足球 · 赛程 · 即时比分 · 下注</text>
      <view class="sports-tags">
        <text>比分</text>
        <text>分析</text>
        <text>下注</text>
      </view>
      <view class="hero-bottom">
        <view class="clock-chip">
          <view class="clock-dot" />
          <text>实时 {{ currentTime }}</text>
        </view>
        <view class="history-button" @tap="openHistory">投注历史</view>
      </view>
    </view>

    <view class="date-tabs">
      <view
        v-for="tab in tabs"
        :key="tab.key"
        class="date-tab"
        :class="{ active: tab.key === selectedTab }"
        @tap="selectTab(tab.key)"
      >
        {{ tab.name }}
      </view>
    </view>

    <view class="section-row">
      <view class="section-dot" />
      <text class="section-title">{{ matchesTitle }}</text>
    </view>
    <scroll-view scroll-x class="league-strip" :show-scrollbar="false">
      <view v-for="league in leagueCards" :key="leagueKey(league)" class="league-card">
        <SafeImage class="league-icon" :src="leagueIcon(league)" fallback="/static/icons/league-default.svg" mode="aspectFit" />
        <view class="league-main">
          <text class="league-name">{{ leagueName(league) }}</text>
          <text class="league-count">{{ league.count || 0 }} 场</text>
        </view>
      </view>
    </scroll-view>

    <view v-if="matches.length" class="match-list">
      <view v-for="match in matches" :key="String(match.match_id || match.id)" class="match-card" @tap="openMatch(match)">
        <view class="match-head">
          <view class="sport-pill">足</view>
          <text class="league-title">{{ match.competition_type || match.league_name || "足球赛事" }}</text>
          <text class="half-pill" :class="{ live: isLiveText(match.status_text) }">{{ match.status_text || "进行中" }}</text>
        </view>

        <view class="score-wrap">
          <view class="team-block">
            <view class="team-logo-ring">
              <SafeImage class="team-logo" :src="teamLogo(match, 'home')" fallback="/static/icons/league-default.svg" mode="aspectFit" />
            </view>
            <text class="team-name">{{ teamName(match, "home") }}</text>
          </view>
          <view class="score-main">
            <text class="score" :class="{ upcoming: !matchHasStarted(match) }">{{ scoreText(match) }}</text>
            <text class="score-sub">{{ match.kickoff_text || match.kickoff_time_text || match.match_time || "" }}</text>
          </view>
          <view class="team-block">
            <view class="team-logo-ring">
              <SafeImage class="team-logo" :src="teamLogo(match, 'away')" fallback="/static/icons/league-default.svg" mode="aspectFit" />
            </view>
            <text class="team-name">{{ teamName(match, "away") }}</text>
          </view>
        </view>

        <view class="prediction-row">
          <view v-for="item in predictionCards(match)" :key="item.name" class="prediction-card">
            <text>{{ item.name }}</text>
            <text>{{ item.value }}</text>
          </view>
        </view>
      </view>
    </view>
    <view v-else class="match-card skeleton-card">
      <text class="skeleton-title">赛事正在同步</text>
      <view class="score-placeholder">VS</view>
      <text class="skeleton-sub">下拉刷新可重新获取赛程。</text>
    </view>

    <view class="section-row data-title">
      <view class="section-dot" />
      <text class="section-title">{{ home?.quick_stats_title || "今日数据" }}</text>
    </view>
    <view class="stats-grid">
      <view v-for="(stat, index) in statCards" :key="String(stat.name || index)" class="stat-card">
        <text class="stat-value">{{ stat.value }}</text>
        <text class="stat-name">{{ stat.name }}</text>
        <text class="stat-desc">{{ stat.desc }}</text>
      </view>
    </view>
  </view>
</template>

<script setup lang="ts">
import { computed, ref } from "vue";
import { onHide, onPullDownRefresh, onShow, onUnload } from "@dcloudio/uni-app";
import SafeImage from "@/components/SafeImage.vue";
import { getSportsHome } from "@/api/services";
import type { SportsHome, SportsMatch } from "@/types/api";
import { absolutizeUrl } from "@/utils/url";
import { openSportsDetail } from "@/utils/navigation";
import { requireLogin } from "@/utils/session";

const defaultTabs = [
  { key: "yesterday", name: "昨日" },
  { key: "today", name: "今日" },
  { key: "tomorrow", name: "明日" },
  { key: "fixtures", name: "赛程" }
];

const defaultStats = [
  { name: "比赛总数", value: "0", desc: "当前列表" },
  { name: "进行中", value: "0", desc: "实时比分" },
  { name: "进球均值", value: "0", desc: "已出比分场次" }
];

const defaultLeagues = [
  { key: "world-cup", name: "FIFA World Cup", count: "0", icon_url: "/static/icons/league-default.svg" },
  { key: "premier-league", name: "Premier League", count: "0", icon_url: "/static/icons/league-default.svg" }
];

const tabs = ref(defaultTabs);
const selectedTab = ref("today");
const home = ref<SportsHome>();
const loading = ref(false);
const currentTime = ref("");
const serverOffsetSeconds = ref(0);
let loadedOnce = false;
let clockTimer: ReturnType<typeof setInterval> | undefined;

const matches = computed(() => home.value?.matches || []);
const matchesTitle = computed(() => home.value?.matches_title || `即时赛况（${matches.value.length}场）`);
const leagueCards = computed(() => (home.value?.top_leagues?.length ? home.value.top_leagues : defaultLeagues));
const statCards = computed(() => (home.value?.quick_stats?.length ? home.value.quick_stats : defaultStats));

function updateClock() {
  const timestamp = Math.floor(Date.now() / 1000) + serverOffsetSeconds.value;
  const date = new Date(timestamp * 1000);
  currentTime.value = new Intl.DateTimeFormat("zh-CN", {
    timeZone: home.value?.timezone || "Asia/Shanghai",
    hour: "2-digit",
    minute: "2-digit",
    second: "2-digit",
    hourCycle: "h23"
  }).format(date);
}

function syncServerClock() {
  const serverTime = Number(home.value?.server_time || 0);
  if (serverTime > 0) {
    serverOffsetSeconds.value = serverTime - Math.floor(Date.now() / 1000);
  }
  updateClock();
}

function startClock() {
  if (clockTimer) clearInterval(clockTimer);
  updateClock();
  clockTimer = setInterval(updateClock, 1000);
}

function stopClock() {
  if (clockTimer) {
    clearInterval(clockTimer);
    clockTimer = undefined;
  }
}

function isLiveText(text?: string) {
  const value = String(text || "");
  return value.includes("进行") || value.includes("上半") || value.includes("下半") || value.toUpperCase().includes("LIVE");
}

function leagueName(league: Record<string, unknown>) {
  return String(league.name_cn || league.name || league.name_en || "赛事");
}

function leagueKey(league: Record<string, unknown>) {
  return String(league.key || league.value || league.name || league.name_en || leagueName(league));
}

function leagueIcon(league: Record<string, unknown>) {
  return absolutizeUrl(String(league.icon_url || league.icon || "")) || "/static/icons/league-default.svg";
}

function teamName(match: SportsMatch, side: "home" | "away") {
  const direct = side === "home" ? match.home_name : match.away_name;
  const team = side === "home" ? match.home_team : match.away_team;
  if (direct) {
    return String(direct);
  }
  if (team && typeof team === "object") {
    return String(team.name || "球队");
  }
  return String(team || "球队");
}

function teamLogo(match: SportsMatch, side: "home" | "away") {
  const direct = side === "home" ? match.home_logo : match.away_logo;
  const team = side === "home" ? match.home_team : match.away_team;
  if (direct) {
    return absolutizeUrl(String(direct));
  }
  if (team && typeof team === "object") {
    return absolutizeUrl(String(team.logo || ""));
  }
  return "/static/icons/league-default.svg";
}

function predictionCards(match: SportsMatch) {
  const markets = Array.isArray(match.markets) ? match.markets : [];
  const market = markets.find((item) => item.market_code === "MATCH_RESULT") || markets[0];
  const options = Array.isArray(market?.options) ? market.options.slice(0, 3) : [];
  const cards = options.map((option) => ({
    name: String(option.option_name || option.option_code || "投注项"),
    value: `赔率 ${String(option.odds || "-")}`
  }));
  const marketCount = Number(match.market_count || markets.length || 0);
  if (marketCount > 0) {
    cards.push({
      name: market?.market_name || "赛事盘口",
      value: marketCount > 1 ? `共 ${marketCount} 种` : "查看详情"
    });
  }
  return cards.length ? cards : [{ name: "暂无盘口", value: "查看详情" }];
}

function matchHasStarted(match: SportsMatch) {
  const explicit = match.has_started;
  if (explicit !== undefined) {
    return explicit === 1 || explicit === "1";
  }
  return isLiveText(String(match.match_status || match.status_text || ""));
}

function scoreText(match: SportsMatch) {
  if (!matchHasStarted(match)) {
    return "VS";
  }
  return String(match.score_text || `${match.home_score ?? "-"} : ${match.away_score ?? "-"}`);
}

async function load() {
  loading.value = true;
  try {
    home.value = await getSportsHome(selectedTab.value);
    const apiTabs = (home.value?.tabs || [])
      .map((item) => ({ key: String(item.key || ""), name: String(item.name || "") }))
      .filter((item) => item.key && item.name);
    if (apiTabs.length) {
      tabs.value = apiTabs;
    }
    selectedTab.value = home.value?.selected_tab || selectedTab.value;
    syncServerClock();
  } catch (error: any) {
    uni.showToast({ title: error?.message || "体育加载失败", icon: "none" });
  } finally {
    loading.value = false;
    uni.stopPullDownRefresh();
  }
}

function selectTab(tab: string) {
  if (selectedTab.value === tab) {
    return;
  }
  selectedTab.value = tab;
  void load();
}

function openMatch(match: SportsMatch) {
  if (!requireLogin()) {
    return;
  }
  openSportsDetail(match);
}

function openHistory() {
  if (!requireLogin()) {
    return;
  }
  uni.navigateTo({ url: "/pages/sports/record" });
}

onShow(() => {
  startClock();
  if (!loadedOnce) {
    loadedOnce = true;
    void load();
  }
});

onHide(stopClock);
onUnload(stopClock);

onPullDownRefresh(() => {
  updateClock();
  void load();
});
</script>

<style scoped>
.sports-page {
  min-height: 100vh;
  overflow-x: hidden;
  padding: calc(30rpx + var(--status-bar-height)) 28rpx calc(128rpx + env(safe-area-inset-bottom));
  color: var(--ink);
  background: var(--bg);
}

.sports-hero {
  position: relative;
  overflow: hidden;
  margin-bottom: 26rpx;
  padding: 40rpx 30rpx 30rpx;
  border-radius: var(--radius-lg);
  color: #fff;
  background: linear-gradient(134deg, #063b2c 0%, #0b7a52 58%, #12a06b 100%);
  box-shadow: 0 16rpx 40rpx rgba(10, 96, 66, 0.24);
}

.pitch-lines {
  position: absolute;
  inset: 0;
  pointer-events: none;
  background: repeating-linear-gradient(
    96deg,
    rgba(255, 255, 255, 0.045) 0,
    rgba(255, 255, 255, 0.045) 70rpx,
    transparent 70rpx,
    transparent 140rpx
  );
}

.hero-glow {
  position: absolute;
  top: -140rpx;
  right: -90rpx;
  width: 360rpx;
  height: 360rpx;
  border-radius: 50%;
  pointer-events: none;
  background: radial-gradient(circle at 40% 40%, rgba(130, 255, 200, 0.4), rgba(130, 255, 200, 0) 70%);
}

.sports-title,
.sports-sub,
.sports-tags,
.hero-bottom {
  position: relative;
  z-index: 1;
}

.sports-title {
  display: block;
  font-size: 42rpx;
  font-weight: 800;
  line-height: 1.1;
  letter-spacing: 2rpx;
}

.sports-sub {
  display: block;
  margin-top: 14rpx;
  color: rgba(255, 255, 255, 0.78);
  font-size: 23rpx;
  letter-spacing: 1rpx;
}

.sports-tags {
  display: flex;
  gap: 14rpx;
  margin-top: 30rpx;
}

.sports-tags text {
  display: flex;
  align-items: center;
  justify-content: center;
  height: 48rpx;
  padding: 0 26rpx;
  border: 2rpx solid rgba(255, 255, 255, 0.24);
  border-radius: 999rpx;
  color: #fff;
  font-size: 23rpx;
  font-weight: 600;
  background: rgba(255, 255, 255, 0.12);
}

.hero-bottom {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-top: 34rpx;
}

.clock-chip {
  display: flex;
  align-items: center;
  gap: 10rpx;
  color: rgba(255, 255, 255, 0.85);
  font-size: 22rpx;
  font-variant-numeric: tabular-nums;
}

.clock-dot {
  width: 12rpx;
  height: 12rpx;
  border-radius: 50%;
  background: #7dffc8;
  animation: blink 1.6s ease-in-out infinite;
}

@keyframes blink {
  0%,
  100% {
    opacity: 1;
  }
  50% {
    opacity: 0.35;
  }
}

.history-button {
  display: flex;
  align-items: center;
  justify-content: center;
  min-width: 152rpx;
  height: 56rpx;
  border-radius: 999rpx;
  color: #0b6647;
  font-size: 23rpx;
  font-weight: 800;
  background: #fff;
  box-shadow: 0 8rpx 20rpx rgba(4, 42, 29, 0.24);
}

.date-tabs {
  display: flex;
  flex-wrap: wrap;
  gap: 12rpx;
  margin-bottom: 30rpx;
  padding: 10rpx;
  border-radius: 999rpx;
  background: var(--surface);
  box-shadow: var(--shadow-soft);
}

.date-tab {
  display: flex;
  flex: 1;
  min-width: 96rpx;
  align-items: center;
  justify-content: center;
  height: 64rpx;
  border-radius: 999rpx;
  color: var(--ink-2);
  font-size: 25rpx;
  font-weight: 600;
  transition: all 0.18s ease;
}

.date-tab.active {
  color: #fff;
  font-weight: 800;
  background: linear-gradient(135deg, #0d9861, #16bd7f);
  box-shadow: 0 8rpx 18rpx rgba(16, 164, 106, 0.28);
}

.section-row {
  display: flex;
  align-items: center;
  gap: 14rpx;
  margin-bottom: 20rpx;
}

.section-dot {
  width: 10rpx;
  height: 30rpx;
  border-radius: 6rpx;
  background: linear-gradient(180deg, #12a06b, #7dffc8);
}

.section-title {
  color: var(--ink);
  font-size: 30rpx;
  font-weight: 800;
}

.league-strip {
  width: 100%;
  margin-bottom: 26rpx;
  overflow: hidden;
  white-space: nowrap;
}

.league-card {
  display: inline-flex;
  width: 288rpx;
  height: 112rpx;
  align-items: center;
  margin-right: 16rpx;
  padding: 0 24rpx;
  border-radius: var(--radius);
  vertical-align: top;
  background: var(--surface);
  box-shadow: var(--shadow-soft);
}

.league-icon {
  width: 56rpx;
  height: 56rpx;
  margin-right: 20rpx;
  border-radius: 50%;
  background: #f0f6f3;
}

.league-main {
  min-width: 0;
}

.league-name {
  display: block;
  color: var(--ink);
  font-size: 24rpx;
  font-weight: 700;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.league-count {
  display: block;
  margin-top: 10rpx;
  color: var(--ink-3);
  font-size: 21rpx;
}

.match-card {
  position: relative;
  margin-bottom: 22rpx;
  padding: 26rpx 26rpx 24rpx;
  border-radius: var(--radius);
  background: var(--surface);
  box-shadow: var(--shadow-soft);
  transition: transform 0.15s ease;
}

.match-card:active {
  transform: scale(0.98);
}

.match-head {
  display: flex;
  align-items: center;
  gap: 14rpx;
}

.sport-pill {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 44rpx;
  height: 44rpx;
  border-radius: 14rpx;
  color: #0b8a5c;
  font-size: 22rpx;
  font-weight: 800;
  background: #e2f8ee;
}

.league-title {
  flex: 1;
  min-width: 0;
  color: var(--ink-2);
  font-size: 24rpx;
  font-weight: 600;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.half-pill {
  display: flex;
  align-items: center;
  justify-content: center;
  height: 44rpx;
  padding: 0 20rpx;
  border-radius: 999rpx;
  color: var(--ink-3);
  font-size: 21rpx;
  font-weight: 700;
  background: var(--bg);
}

.half-pill.live {
  color: #0b8a5c;
  background: #e2f8ee;
}

.score-wrap {
  display: grid;
  grid-template-columns: 150rpx minmax(0, 1fr) 150rpx;
  align-items: center;
  gap: 16rpx;
  margin-top: 30rpx;
}

.team-block {
  display: flex;
  min-width: 0;
  flex-direction: column;
  align-items: center;
}

.team-logo-ring {
  display: flex;
  width: 104rpx;
  height: 104rpx;
  align-items: center;
  justify-content: center;
  border-radius: 50%;
  background: linear-gradient(160deg, #f2f5f9, #fbfcfe);
  box-shadow: inset 0 0 0 2rpx var(--line);
}

.team-logo {
  width: 68rpx;
  height: 68rpx;
}

.team-name {
  width: 100%;
  margin-top: 14rpx;
  color: var(--ink);
  font-size: 22rpx;
  font-weight: 700;
  text-align: center;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.score-main {
  min-width: 0;
  text-align: center;
}

.score {
  display: block;
  color: var(--ink);
  font-size: 56rpx;
  font-weight: 800;
  line-height: 1;
  font-variant-numeric: tabular-nums;
}

.score.upcoming {
  color: #b8bfca;
  font-size: 48rpx;
  letter-spacing: 4rpx;
}

.score-sub {
  display: block;
  margin-top: 14rpx;
  color: var(--ink-3);
  font-size: 21rpx;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.prediction-row {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(120rpx, 1fr));
  gap: 12rpx;
  margin-top: 28rpx;
  padding-top: 24rpx;
  border-top: 2rpx solid var(--line);
}

.prediction-card {
  min-height: 84rpx;
  padding: 12rpx 8rpx;
  border-radius: 16rpx;
  text-align: center;
  background: #f2faf6;
}

.prediction-card text:first-child,
.prediction-card text:last-child {
  display: block;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.prediction-card text:first-child {
  color: #6d8f80;
  font-size: 20rpx;
}

.prediction-card text:last-child {
  margin-top: 8rpx;
  color: #0b8a5c;
  font-size: 24rpx;
  font-weight: 800;
}

.skeleton-card {
  display: flex;
  min-height: 320rpx;
  flex-direction: column;
  align-items: center;
  justify-content: center;
}

.skeleton-title {
  color: var(--ink-2);
  font-size: 26rpx;
  font-weight: 700;
}

.score-placeholder {
  margin: 30rpx 0 14rpx;
  color: #d6dae3;
  font-size: 64rpx;
  font-weight: 800;
}

.skeleton-sub {
  color: var(--ink-3);
  font-size: 22rpx;
}

.data-title {
  margin-top: 10rpx;
}

.stats-grid {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 16rpx;
}

.stat-card {
  min-height: 150rpx;
  padding: 26rpx 14rpx 20rpx;
  border-radius: var(--radius);
  text-align: center;
  background: var(--surface);
  box-shadow: var(--shadow-soft);
}

.stat-value,
.stat-name,
.stat-desc {
  display: block;
}

.stat-value {
  color: #0b8a5c;
  font-size: 40rpx;
  font-weight: 800;
  font-variant-numeric: tabular-nums;
}

.stat-name {
  margin-top: 10rpx;
  color: var(--ink);
  font-size: 22rpx;
  font-weight: 700;
}

.stat-desc {
  margin-top: 6rpx;
  color: var(--ink-3);
  font-size: 19rpx;
}
</style>
