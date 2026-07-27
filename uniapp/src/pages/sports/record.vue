<template>
  <view class="safe-page sports-record-page">
    <view class="record-hero">
      <view class="hero-bg" />
      <view class="hero-content">
        <text class="hero-title">体育投注记录</text>
        <text class="hero-sub">赛事、盘口、赛果与派彩明细</text>
      </view>
      <button class="hero-refresh" @tap="refresh">刷新</button>
    </view>

    <view class="summary-grid">
      <view class="summary-card">
        <text>{{ amountOf(data, ["total_bet"]) }}</text>
        <text>总下注</text>
      </view>
      <view class="summary-card blue">
        <text>{{ amountOf(data, ["total_payout"]) }}</text>
        <text>总派彩</text>
      </view>
      <view class="summary-card gold">
        <text>{{ amountOf(data, ["profit_loss", "net_amount"]) }}</text>
        <text>总盈亏</text>
      </view>
    </view>

    <view class="section-title">
      <text>投注记录</text>
      <text>{{ records.length ? `${records.length} 条` : "" }}</text>
    </view>

    <view v-if="records.length" class="record-list">
      <view v-for="order in records" :key="rowKey(order)" class="record-card">
        <view class="record-top">
          <view>
            <text class="match-title">{{ matchName(order) }}</text>
            <text class="match-meta">{{ matchMeta(order) }}</text>
          </view>
          <text class="status-pill">{{ statusOf(order) }}</text>
        </view>

        <view class="score-box">
          <view>
            <text>{{ teamName(order, "home") }}</text>
            <text>{{ teamName(order, "away") }}</text>
          </view>
          <view>
            <text>{{ scoreOf(order) }}</text>
            <text>{{ resultStatus(order) }}</text>
          </view>
        </view>

        <view class="order-meta">
          <text>{{ orderMeta(order) }}</text>
          <text v-if="timeOf(order)">下注时间：{{ timeOf(order) }}</text>
          <text v-if="settleTimeOf(order)">结算时间：{{ settleTimeOf(order) }}</text>
        </view>

        <view class="amount-grid">
          <view>
            <text>下注</text>
            <text>{{ amountOf(order, ["total_bet", "bet_money", "money"]) }}</text>
          </view>
          <view>
            <text>派彩</text>
            <text>{{ amountOf(order, ["total_payout", "win_money"]) }}</text>
          </view>
          <view>
            <text>盈亏</text>
            <text>{{ amountOf(order, ["net_amount", "profit_loss"]) }}</text>
          </view>
        </view>

        <view class="bet-items">
          <view v-for="item in recordItems(order)" :key="rowKey(item)" class="bet-item">
            <view class="bet-item-main">
              <text>{{ sportsItemTitle(item) }}</text>
              <text>{{ itemStatusOf(item) }}</text>
            </view>
            <view class="bet-item-grid">
              <text>赔率 {{ textOf(item, ["odds", "rate"], "-") }}</text>
              <text>投注 {{ amountOf(item, ["bet_amount", "amount", "money"]) }}</text>
              <text>派彩 {{ amountOf(item, ["payout_amount", "win_money"], "-") }}</text>
            </view>
          </view>
        </view>
      </view>

      <text v-if="finished" class="list-footer">没有更多记录了</text>
    </view>
    <EmptyState v-else :title="loading ? '正在加载体育投注记录' : '暂无体育投注记录'" description="下拉刷新可重新同步记录。" />
  </view>
</template>

<script setup lang="ts">
import { computed, ref } from "vue";
import { onLoad, onPullDownRefresh, onReachBottom } from "@dcloudio/uni-app";
import EmptyState from "@/components/EmptyState.vue";
import { getSportsBetRecords } from "@/api/services";
import { requireLogin } from "@/utils/session";

type AnyRecord = Record<string, unknown>;

const matchId = ref("");
const data = ref<AnyRecord>({});
const list = ref<AnyRecord[]>([]);
const page = ref(1);
const loading = ref(false);
const finished = ref(false);

const records = computed(() => list.value);

function isRecord(value: unknown): value is AnyRecord {
  return !!value && typeof value === "object" && !Array.isArray(value);
}

function recordOf(source: unknown, key: string) {
  if (!isRecord(source)) {
    return undefined;
  }
  const value = source[key];
  return isRecord(value) ? value : undefined;
}

function firstArray(source: unknown, keys: string[]) {
  if (!isRecord(source)) {
    return [] as AnyRecord[];
  }
  for (const key of keys) {
    const value = source[key];
    if (Array.isArray(value)) {
      return value.filter(isRecord);
    }
  }
  return [] as AnyRecord[];
}

function textOf(source: unknown, keys: string[], fallback = "") {
  if (!isRecord(source)) {
    return fallback;
  }
  for (const key of keys) {
    const value = source[key];
    if (value !== undefined && value !== null && String(value).trim() !== "") {
      return String(value);
    }
  }
  return fallback;
}

function amountOf(source: unknown, keys: string[], fallback = "0") {
  return textOf(source, keys, fallback);
}

function matchOf(order: AnyRecord) {
  return recordOf(order, "match") || {};
}

function rowKey(item: AnyRecord) {
  return textOf(item, ["order_no", "id", "option_id", "match_id", "bet_time"], JSON.stringify(item));
}

function teamName(order: AnyRecord, side: "home" | "away") {
  const match = matchOf(order);
  const direct = side === "home" ? ["home_name", "home"] : ["away_name", "away"];
  return textOf(match, direct, textOf(order, direct, side === "home" ? "主队" : "客队"));
}

function matchName(order: AnyRecord) {
  const direct = textOf(order, ["match_title", "match_name"], "");
  if (direct) {
    return direct;
  }
  return `${teamName(order, "home")} VS ${teamName(order, "away")}`;
}

function matchMeta(order: AnyRecord) {
  const match = matchOf(order);
  const competition = textOf(match, ["competition", "league_name"], textOf(order, ["competition", "league_name"], "赛事"));
  const kickoff = textOf(match, ["kickoff_text", "kickoff_time_text", "match_time"], textOf(order, ["kickoff_time_text", "match_time"], ""));
  return kickoff ? `${competition} · ${kickoff}` : competition;
}

function scoreOf(order: AnyRecord) {
  const match = matchOf(order);
  const home = textOf(match, ["home_score"], textOf(order, ["home_score"], ""));
  const away = textOf(match, ["away_score"], textOf(order, ["away_score"], ""));
  if (home !== "" && away !== "") {
    return `${home} : ${away}`;
  }
  return "待同步";
}

function resultStatus(order: AnyRecord) {
  const match = matchOf(order);
  const label = textOf(match, ["status_text"], textOf(order, ["match_status_text"], ""));
  if (label) {
    return label;
  }
  const settle = textOf(match, ["settle_status"], textOf(order, ["settle_status"], ""));
  if (settle === "1" || settle === "2") {
    return "已结算";
  }
  if (settle === "0") {
    return "待结算";
  }
  return "赛果";
}

function statusOf(order: AnyRecord) {
  return textOf(order, ["status_text", "status_name", "state_text"], "处理中");
}

function orderMeta(order: AnyRecord) {
  const matchNo = textOf(order, ["display_match_id", "public_match_id", "match_id"], "--");
  const no = textOf(order, ["order_no", "orderid", "id"], "--");
  return `赛事编号 ${matchNo} · 订单 ${no}`;
}

function timeOf(order: AnyRecord) {
  return textOf(order, ["bet_time_text", "addtime", "datetime", "time"], "");
}

function settleTimeOf(order: AnyRecord) {
  return textOf(order, ["settle_time_text", "settle_time"], "");
}

function recordItems(order: AnyRecord) {
  const items = firstArray(order, ["items", "details"]);
  if (items.length) {
    return items;
  }
  return [order];
}

function sportsItemTitle(item: AnyRecord) {
  const market = textOf(item, ["market_name", "market_code", "bet_name", "play_name"], "盘口");
  const option = textOf(item, ["option_name", "option_code"], "投注项");
  return `${market} · ${option}`;
}

function itemStatusOf(item: AnyRecord) {
  return textOf(item, ["win_status_text", "status_text", "state_text"], "待结算");
}

async function load(reset = false) {
  if (!requireLogin()) {
    uni.stopPullDownRefresh();
    return;
  }
  if (loading.value || (finished.value && !reset)) {
    uni.stopPullDownRefresh();
    return;
  }
  if (reset) {
    page.value = 1;
    finished.value = false;
  }
  loading.value = true;
  try {
    const next = await getSportsBetRecords(matchId.value, page.value);
    data.value = isRecord(next) ? next : {};
    const rows = firstArray(next, ["list", "items", "orders"]);
    list.value = reset ? rows : list.value.concat(rows);
    const total = Number(textOf(next, ["total_count", "total"], "0"));
    finished.value = rows.length === 0 || (total > 0 && list.value.length >= total);
    if (!finished.value) {
      page.value += 1;
    }
  } catch (error: any) {
    uni.showToast({ title: error?.message || "记录加载失败", icon: "none" });
  } finally {
    loading.value = false;
    uni.stopPullDownRefresh();
  }
}

function refresh() {
  void load(true);
}

onLoad((query) => {
  matchId.value = String(query?.match_id || "");
  void load(true);
});

onPullDownRefresh(() => {
  void load(true);
});

onReachBottom(() => {
  void load(false);
});
</script>

<style scoped>
.sports-record-page {
  min-height: 100vh;
  padding: calc(var(--status-bar-height) + 20rpx) 24rpx 40rpx;
  background: #eef7f4;
}

.record-hero {
  position: relative;
  display: flex;
  overflow: hidden;
  min-height: 196rpx;
  align-items: center;
  justify-content: space-between;
  gap: 20rpx;
  padding: 30rpx 28rpx;
  border-radius: 24rpx;
  color: #fff;
  background: linear-gradient(134deg, #063b2c 0%, #0b7a52 58%, #12a06b 100%);
}

.hero-bg {
  position: absolute;
  inset: 0;
  background: repeating-linear-gradient(
    96deg,
    rgba(255, 255, 255, 0.05) 0,
    rgba(255, 255, 255, 0.05) 70rpx,
    transparent 70rpx,
    transparent 140rpx
  );
}

.hero-content,
.hero-refresh {
  position: relative;
  z-index: 1;
}

.hero-title,
.hero-sub {
  display: block;
}

.hero-title {
  font-size: 40rpx;
  font-weight: 900;
}

.hero-sub {
  margin-top: 14rpx;
  color: rgba(255, 255, 255, 0.86);
  font-size: 24rpx;
}

.hero-refresh {
  flex: 0 0 auto;
  height: 64rpx;
  min-width: 124rpx;
  padding: 0 26rpx;
  border-radius: 999rpx;
  color: #0d6d4c;
  font-size: 24rpx;
  font-weight: 900;
  line-height: 64rpx;
  background: #fff;
}

.summary-grid {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 16rpx;
  margin: 22rpx 0 30rpx;
}

.summary-card {
  min-width: 0;
  padding: 20rpx 10rpx;
  border-radius: 18rpx;
  text-align: center;
  background: #fff;
}

.summary-card text:first-child {
  display: block;
  color: #16a36f;
  font-size: 30rpx;
  font-weight: 900;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.summary-card.blue text:first-child {
  color: #2e6ed6;
}

.summary-card.gold text:first-child {
  color: #c87424;
}

.summary-card text:last-child {
  display: block;
  margin-top: 12rpx;
  color: #53665f;
  font-size: 22rpx;
}

.section-title {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 18rpx;
}

.section-title text:first-child {
  color: #12251e;
  font-size: 30rpx;
  font-weight: 900;
}

.section-title text:last-child {
  color: #72857d;
  font-size: 22rpx;
}

.record-card {
  padding: 24rpx;
  margin-bottom: 18rpx;
  border: 1rpx solid #dfe8e4;
  border-radius: 20rpx;
  background: #fff;
}

.record-top {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 18rpx;
}

.record-top > view {
  flex: 1;
  min-width: 0;
}

.match-title,
.match-meta {
  display: block;
}

.match-title {
  color: #12251e;
  font-size: 28rpx;
  font-weight: 900;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.match-meta {
  margin-top: 10rpx;
  color: #6b7c75;
  font-size: 22rpx;
  line-height: 32rpx;
}

.status-pill {
  flex: 0 0 auto;
  max-width: 150rpx;
  padding: 8rpx 16rpx;
  border-radius: 999rpx;
  color: #0d8b5b;
  font-size: 22rpx;
  font-weight: 900;
  text-align: center;
  background: #e8f7f0;
}

.score-box {
  display: grid;
  grid-template-columns: 1fr auto;
  gap: 16rpx;
  margin-top: 18rpx;
  padding: 18rpx;
  border-radius: 16rpx;
  background: #f3faf7;
}

.score-box view:first-child text,
.score-box view:last-child text {
  display: block;
}

.score-box view:first-child text {
  color: #13251e;
  font-size: 24rpx;
  font-weight: 800;
  line-height: 38rpx;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.score-box view:last-child {
  min-width: 132rpx;
  text-align: right;
}

.score-box view:last-child text:first-child {
  color: #0f7e55;
  font-size: 32rpx;
  font-weight: 900;
}

.score-box view:last-child text:last-child {
  margin-top: 8rpx;
  color: #789087;
  font-size: 21rpx;
}

.order-meta {
  display: grid;
  gap: 8rpx;
  margin-top: 16rpx;
  color: #7c8b86;
  font-size: 22rpx;
  line-height: 32rpx;
}

.order-meta text {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.amount-grid {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 12rpx;
  margin-top: 18rpx;
  padding: 14rpx 0;
}

.amount-grid view {
  min-width: 0;
}

.amount-grid text {
  display: block;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.amount-grid text:first-child {
  color: #8a9994;
  font-size: 21rpx;
}

.amount-grid text:last-child {
  margin-top: 8rpx;
  color: #13251e;
  font-size: 25rpx;
  font-weight: 900;
}

.bet-items {
  border-top: 1rpx solid #e8f0ed;
}

.bet-item {
  padding: 16rpx 0;
  border-bottom: 1rpx solid #edf3f0;
}

.bet-item-main {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16rpx;
}

.bet-item-main text:first-child {
  flex: 1;
  min-width: 0;
  color: #13251e;
  font-size: 25rpx;
  font-weight: 900;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.bet-item-main text:last-child {
  flex: 0 0 auto;
  color: #16a36f;
  font-size: 22rpx;
  font-weight: 900;
}

.bet-item-grid {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 10rpx;
  margin-top: 12rpx;
}

.bet-item-grid text {
  min-width: 0;
  color: #778680;
  font-size: 22rpx;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.list-footer {
  display: block;
  padding: 18rpx 0 8rpx;
  color: #7f9189;
  font-size: 22rpx;
  text-align: center;
}
</style>
