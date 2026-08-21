<template>
  <view class="safe-page lottery-record-page">
    <view class="record-hero">
      <view>
        <text class="hero-title">{{ pageTitle }}</text>
        <text class="hero-sub">{{ t("commerce.betRecord.lotterySubtitle") }}</text>
      </view>
      <button class="hero-refresh" @tap="refresh">{{ t("commerce.common.refresh") }}</button>
    </view>

    <view class="summary-grid">
      <view class="summary-card">
        <text>{{ amountOf(bundle, ["total_bet"]) }}</text>
        <text>{{ t("commerce.common.totalBet") }}</text>
      </view>
      <view class="summary-card payout">
        <text>{{ amountOf(bundle, ["total_payout"]) }}</text>
        <text>{{ t("commerce.common.totalPayout") }}</text>
      </view>
      <view class="summary-card profit">
        <text>{{ amountOf(bundle, ["profit_loss", "net_amount"]) }}</text>
        <text>{{ t("commerce.common.totalProfitLoss") }}</text>
      </view>
    </view>

    <view class="section-bar">
      <text>{{ t("commerce.common.betRecords") }}</text>
      <text>{{ records.length ? t("commerce.common.recordCount").replace("{count}", String(records.length)) : "" }}</text>
    </view>

    <view v-if="records.length" class="record-list">
      <view v-for="order in records" :key="recordKey(order)" class="record-card">
        <view class="record-head">
          <view>
            <text class="record-title">{{ orderTitle(order) }}</text>
            <text class="record-meta">{{ lotteryOrderMeta(order) }}</text>
          </view>
          <text class="status-pill">{{ statusOf(order) }}</text>
        </view>

        <view class="open-box">
          <view>
            <text>{{ t("commerce.betRecord.drawNumbers") }}</text>
            <text>{{ openCodeOf(order) }}</text>
          </view>
          <view>
            <text>{{ t("commerce.betRecord.drawStatus") }}</text>
            <text>{{ issueStatusOf(order) }}</text>
          </view>
          <view>
            <text>{{ t("commerce.betRecord.drawTime") }}</text>
            <text>{{ openTimeOf(order) }}</text>
          </view>
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
            <text>{{ amountOf(order, ["profit_loss", "net_amount"]) }}</text>
          </view>
        </view>

        <view v-if="recordItems(order).length" class="bet-items">
          <view v-for="item in recordItems(order)" :key="recordKey(item)" class="bet-item">
            <view class="bet-item-main">
              <text>{{ lotteryItemTitle(item) }}</text>
              <text>{{ itemStatusOf(item) }}</text>
            </view>
            <view class="bet-item-grid">
              <text>{{ t("commerce.common.odds") }} {{ textOf(item, ["odds", "rate"], "-") }}</text>
              <text>{{ t("commerce.common.bet") }} {{ amountOf(item, ["bet_amount", "amount", "money"]) }}</text>
              <text>{{ t("commerce.common.payout") }} {{ amountOf(item, ["payout_amount", "win_money"], "-") }}</text>
            </view>
          </view>
        </view>

        <view class="record-time">
          <text>{{ t("commerce.common.betTime") }}: {{ timeOf(order) }}</text>
          <text v-if="settleTimeOf(order)">{{ t("commerce.common.settlement") }}: {{ settleTimeOf(order) }}</text>
        </view>
      </view>

      <text v-if="finished" class="list-footer">{{ t("commerce.common.noMoreRecords") }}</text>
    </view>
    <EmptyState
      v-else
      :title="loading ? t('commerce.betRecord.loadingLottery') : t('commerce.betRecord.noLotteryRecords')"
      :description="t('commerce.betRecord.lotteryEmptyDescription')"
    />
  </view>
</template>

<script setup lang="ts">
import { computed, ref } from "vue";
import { onLoad, onPullDownRefresh, onReachBottom } from "@dcloudio/uni-app";
import EmptyState from "@/components/EmptyState.vue";
import { getLotteryBetRecords } from "@/api/services";
import { firstText } from "@/utils/url";
import { requireLogin } from "@/utils/session";
import { useI18n } from "@/i18n";

const { t } = useI18n();

type AnyRecord = Record<string, unknown>;

const bundle = ref<AnyRecord>({});
const list = ref<AnyRecord[]>([]);
const loading = ref(false);
const finished = ref(false);
const page = ref(1);
const gameId = ref("");
const gameCode = ref("");
const title = ref("");

const pageTitle = computed(() => title.value || t("commerce.betRecord.lotteryTitle"));
const records = computed(() => list.value);

function isRecord(value: unknown): value is AnyRecord {
  return !!value && typeof value === "object" && !Array.isArray(value);
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

function recordKey(item: AnyRecord) {
  return textOf(item, ["order_no", "id", "option_id", "bet_time"], JSON.stringify(item));
}

function orderTitle(order: AnyRecord) {
  return textOf(order, ["game_name", "game_name_en", "game_code"], t("commerce.betRecord.lotteryOrder"));
}

function statusOf(order: AnyRecord) {
  return textOf(order, ["status_text", "status_name", "status"], t("commerce.betRecord.awaitingDraw"));
}

function lotteryOrderMeta(order: AnyRecord) {
  const issue = textOf(order, ["issue_num", "issue"], "--");
  const no = textOf(order, ["order_no", "orderid", "id"], "--");
  return t("commerce.betRecord.issueOrder")
    .replace("{issue}", issue)
    .replace("{order}", no);
}

function openCodeOf(order: AnyRecord) {
  return textOf(order, ["open_code", "award_code", "result_code"], t("commerce.betRecord.awaitingDraw"));
}

function issueStatusOf(order: AnyRecord) {
  return textOf(order, ["issue_status_text", "issue_state_text"], statusOf(order));
}

function openTimeOf(order: AnyRecord) {
  return textOf(order, ["open_time_text", "draw_time_text"], t("commerce.common.awaitingSync"));
}

function timeOf(order: AnyRecord) {
  return textOf(order, ["bet_time_text", "addtime", "datetime", "time"], "-");
}

function settleTimeOf(order: AnyRecord) {
  return textOf(order, ["settle_time_text", "settle_time"], "");
}

function recordItems(order: AnyRecord) {
  return firstArray(order, ["items", "details", "list"]);
}

function lotteryItemTitle(item: AnyRecord) {
  const play = textOf(item, ["play_name", "play_code"], t("commerce.betRecord.play"));
  const option = textOf(item, ["option_name", "option_code"], t("commerce.common.betOption"));
  return `${play} · ${option}`;
}

function itemStatusOf(item: AnyRecord) {
  return textOf(item, ["win_status_text", "status_text", "state_text"], t("commerce.betRecord.awaitingDraw"));
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
    const next = await getLotteryBetRecords({ id: gameId.value, game_code: gameCode.value }, page.value);
    bundle.value = isRecord(next) ? next : {};
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
  gameId.value = firstText(query?.game_id, query?.id);
  gameCode.value = firstText(query?.game_code);
  title.value = firstText(query?.title);
  uni.setNavigationBarTitle({ title: pageTitle.value });
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
.lottery-record-page {
  min-height: 100vh;
  padding: calc(var(--status-bar-height) + 20rpx) 24rpx 40rpx;
  background: #fff6fa;
}

.record-hero {
  display: flex;
  min-height: 178rpx;
  align-items: center;
  justify-content: space-between;
  gap: 20rpx;
  padding: 28rpx;
  border-radius: 24rpx;
  color: #fff;
  background: linear-gradient(135deg, var(--brand) 0%, #7b45ff 100%);
  box-shadow: 0 16rpx 34rpx rgba(255, 88, 120, 0.18);
}

.hero-title,
.hero-sub {
  display: block;
}

.hero-title {
  font-size: 38rpx;
  font-weight: 900;
}

.hero-sub {
  margin-top: 14rpx;
  color: rgba(255, 255, 255, 0.84);
  font-size: 24rpx;
}

.hero-refresh {
  flex: 0 0 auto;
  height: 64rpx;
  min-width: 124rpx;
  padding: 0 26rpx;
  border-radius: 999rpx;
  color: var(--brand);
  font-size: 24rpx;
  font-weight: 900;
  line-height: 64rpx;
  background: #fff;
}

.summary-grid {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 16rpx;
  margin: 22rpx 0 28rpx;
}

.summary-card {
  min-width: 0;
  padding: 20rpx 10rpx;
  border-radius: 18rpx;
  text-align: center;
  background: #fff;
  box-shadow: 0 10rpx 24rpx rgba(40, 31, 47, 0.05);
}

.summary-card text:first-child {
  display: block;
  color: var(--brand);
  font-size: 30rpx;
  font-weight: 900;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.summary-card.payout text:first-child {
  color: #3468d9;
}

.summary-card.profit text:first-child {
  color: #18a96f;
}

.summary-card text:last-child {
  display: block;
  margin-top: 10rpx;
  color: #8c8a96;
  font-size: 22rpx;
}

.section-bar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 16rpx;
}

.section-bar text:first-child {
  color: #24222d;
  font-size: 30rpx;
  font-weight: 900;
}

.section-bar text:last-child {
  color: #9995a4;
  font-size: 22rpx;
}

.record-card {
  padding: 24rpx;
  margin-bottom: 18rpx;
  border: 1rpx solid #f1dfe7;
  border-radius: 22rpx;
  background: #fff;
  box-shadow: 0 10rpx 24rpx rgba(40, 31, 47, 0.04);
}

.record-head {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 18rpx;
}

.record-head > view {
  flex: 1;
  min-width: 0;
}

.record-title,
.record-meta {
  display: block;
}

.record-title {
  color: #24222d;
  font-size: 29rpx;
  font-weight: 900;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.record-meta {
  margin-top: 10rpx;
  color: #8d8794;
  font-size: 22rpx;
  line-height: 32rpx;
}

.status-pill {
  flex: 0 0 auto;
  max-width: 150rpx;
  padding: 8rpx 16rpx;
  border-radius: 999rpx;
  color: var(--brand);
  font-size: 22rpx;
  font-weight: 900;
  text-align: center;
  background: rgba(255, 88, 120, 0.1);
}

.open-box,
.amount-grid {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 10rpx;
  margin-top: 18rpx;
}

.open-box {
  padding: 14rpx;
  border-radius: 16rpx;
  background: #fff5f8;
}

.open-box view,
.amount-grid view {
  min-width: 0;
}

.open-box text,
.amount-grid text {
  display: block;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.open-box text:first-child,
.amount-grid text:first-child {
  color: #9b96a3;
  font-size: 21rpx;
}

.open-box text:last-child,
.amount-grid text:last-child {
  margin-top: 8rpx;
  color: #2a2632;
  font-size: 24rpx;
  font-weight: 900;
}

.amount-grid {
  padding: 14rpx 0 4rpx;
}

.bet-items {
  margin-top: 12rpx;
  border-top: 1rpx solid #f1e7ec;
}

.bet-item {
  padding: 16rpx 0;
  border-bottom: 1rpx solid #f4edf1;
}

.bet-item-main,
.record-time {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16rpx;
}

.bet-item-main text:first-child {
  flex: 1;
  min-width: 0;
  color: #2a2632;
  font-size: 25rpx;
  font-weight: 900;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.bet-item-main text:last-child {
  flex: 0 0 auto;
  color: #18a96f;
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
  color: #888392;
  font-size: 22rpx;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.record-time {
  flex-wrap: wrap;
  margin-top: 16rpx;
  color: #aaa4b0;
  font-size: 21rpx;
  line-height: 32rpx;
}

.list-footer {
  display: block;
  padding: 18rpx 0 8rpx;
  color: #aaa4b0;
  font-size: 22rpx;
  text-align: center;
}
</style>
