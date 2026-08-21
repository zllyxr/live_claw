<template>
  <view class="safe-page sports-record-page">
    <view class="record-hero">
      <view class="hero-bg" />
      <view class="hero-content">
        <text class="hero-title">{{ t("commerce.sportsRecord.title") }}</text>
        <text class="hero-sub">{{ t("commerce.sportsRecord.subtitle") }}</text>
      </view>
      <button class="hero-refresh" @tap="refresh">{{ t("commerce.common.refresh") }}</button>
    </view>

    <view class="summary-grid">
      <view class="summary-card">
        <text>{{ amountOf(data, ["total_bet"]) }}</text>
        <text>{{ t("commerce.common.totalBet") }}</text>
      </view>
      <view class="summary-card blue">
        <text>{{ amountOf(data, ["total_payout"]) }}</text>
        <text>{{ t("commerce.common.totalPayout") }}</text>
      </view>
      <view class="summary-card gold">
        <text>{{ amountOf(data, ["profit_loss", "net_amount"]) }}</text>
        <text>{{ t("commerce.common.totalProfitLoss") }}</text>
      </view>
    </view>

    <view class="section-title">
      <text>{{ t("commerce.common.betRecords") }}</text>
      <text>{{ records.length ? t("commerce.common.recordCount").replace("{count}", String(records.length)) : "" }}</text>
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
          <text v-if="timeOf(order)">{{ t("commerce.common.betTime") }}: {{ timeOf(order) }}</text>
          <text v-if="settleTimeOf(order)">{{ t("commerce.common.settlementTime") }}: {{ settleTimeOf(order) }}</text>
        </view>

        <view class="amount-grid">
          <view>
            <text>{{ t("commerce.common.bet") }}</text>
            <text>{{ amountOf(order, ["total_bet", "bet_money", "money"]) }}</text>
          </view>
          <view>
            <text>{{ t("commerce.common.payout") }}</text>
            <text>{{ amountOf(order, ["total_payout", "win_money"]) }}</text>
          </view>
          <view>
            <text>{{ t("commerce.common.profitLoss") }}</text>
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
              <text>{{ t("commerce.common.odds") }} {{ textOf(item, ["odds", "rate"], "-") }}</text>
              <text>{{ t("commerce.common.bet") }} {{ amountOf(item, ["bet_amount", "amount", "money"]) }}</text>
              <text>{{ t("commerce.common.payout") }} {{ amountOf(item, ["payout_amount", "win_money"], "-") }}</text>
            </view>
          </view>
        </view>
      </view>

      <text v-if="finished" class="list-footer">{{ t("commerce.common.noMoreRecords") }}</text>
    </view>
    <EmptyState
      v-else
      :title="loading ? t('commerce.sportsRecord.loading') : t('commerce.sportsRecord.empty')"
      :description="t('commerce.sportsRecord.emptyDescription')"
    />
  </view>
</template>

<script setup lang="ts">
import { computed, ref } from "vue";
import { onLoad, onPullDownRefresh, onReachBottom } from "@dcloudio/uni-app";
import EmptyState from "@/components/EmptyState.vue";
import { getSportsBetRecords } from "@/api/services";
import { requireLogin } from "@/utils/session";
import { useI18n } from "@/i18n";

const { t } = useI18n();

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
  return textOf(
    match,
    direct,
    textOf(order, direct, side === "home" ? t("commerce.sportsDetail.homeTeam") : t("commerce.sportsDetail.awayTeam"))
  );
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
  const competition = textOf(
    match,
    ["competition", "league_name"],
    textOf(order, ["competition", "league_name"], t("commerce.sportsRecord.event"))
  );
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
  return t("commerce.common.awaitingSync");
}

function resultStatus(order: AnyRecord) {
  const match = matchOf(order);
  const label = textOf(match, ["status_text"], textOf(order, ["match_status_text"], ""));
  if (label) {
    return label;
  }
  const settle = textOf(match, ["settle_status"], textOf(order, ["settle_status"], ""));
  if (settle === "1" || settle === "2") {
    return t("commerce.sportsRecord.settled");
  }
  if (settle === "0") {
    return t("commerce.sportsRecord.awaitingSettlement");
  }
  return t("commerce.sportsRecord.result");
}

function statusOf(order: AnyRecord) {
  return textOf(order, ["status_text", "status_name", "state_text"], t("commerce.common.processing"));
}

function orderMeta(order: AnyRecord) {
  const matchNo = textOf(order, ["display_match_id", "public_match_id", "match_id"], "--");
  const no = textOf(order, ["order_no", "orderid", "id"], "--");
  return t("commerce.sportsRecord.eventOrder")
    .replace("{match}", matchNo)
    .replace("{order}", no);
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
  const market = textOf(item, ["market_name", "market_code", "bet_name", "play_name"], t("commerce.sportsDetail.market"));
  const option = textOf(item, ["option_name", "option_code"], t("commerce.common.betOption"));
  return `${market} · ${option}`;
}

function itemStatusOf(item: AnyRecord) {
  return textOf(item, ["win_status_text", "status_text", "state_text"], t("commerce.sportsRecord.awaitingSettlement"));
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
    uni.showToast({ title: error?.message || t("commerce.common.recordsLoadFailed"), icon: "none" });
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
  uni.setNavigationBarTitle({ title: t("commerce.sportsRecord.title") });
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
