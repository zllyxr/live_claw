<template>
  <view class="safe-page sports-detail-page">
    <view class="match-card">
      <view class="match-head">
        <text>{{ textOf(match, ["competition", "league_name"], t("commerce.sportsDetail.sportsEvent")) }}</text>
        <text class="status-pill">{{ textOf(match, ["bet_status_text", "status_text"], t("commerce.sportsDetail.marketClosed")) }}</text>
      </view>
      <view class="teams">
        <view class="team">
          <SafeImage
            class="team-logo"
            :src="textOf(match, ['home_logo'])"
            fallback="/static/icons/league-default.svg"
            mode="aspectFit"
          />
          <text>{{ textOf(match, ["home_name"], t("commerce.sportsDetail.homeTeam")) }}</text>
        </view>
        <view class="score">
          <text>{{ scoreText }}</text>
          <text>{{ textOf(match, ["kickoff_text", "match_time"], "") }}</text>
        </view>
        <view class="team">
          <SafeImage
            class="team-logo"
            :src="textOf(match, ['away_logo'])"
            fallback="/static/icons/league-default.svg"
            mode="aspectFit"
          />
          <text>{{ textOf(match, ["away_name"], t("commerce.sportsDetail.awayTeam")) }}</text>
        </view>
      </view>
      <view class="match-foot">
        <text>{{ betOpen ? `${t("commerce.sportsDetail.untilClose")} ${countdownText}` : t("commerce.sportsDetail.currentMarketClosed") }}</text>
        <span @tap="openRecord" style="font-size: 22rpx">{{ t("commerce.sportsDetail.betRecords") }} &gt;</span>
      </view>
    </view>

    <view class="section-head">
      <text>{{ t("commerce.sportsDetail.markets") }}</text>
      <text>{{ markets.length ? t("commerce.common.itemCount").replace("{count}", String(markets.length)) : "" }}</text>
    </view>

    <view v-if="markets.length" class="market-list">
      <view v-for="market in markets" :key="String(market.id || market.market_code)" class="market-card">
        <text class="market-name">{{ market.market_name || t("commerce.sportsDetail.market") }}</text>
        <view class="option-grid">
          <view
            v-for="option in market.options || []"
            :key="String(option.id || option.option_code)"
            class="option"
            :class="{ active: String(selectedOption?.id || '') === String(option.id || '') }"
            @tap="selectOption(option, market)"
          >
            <text>{{ option.option_name || option.option_code || t("commerce.common.betOption") }}</text>
            <text>{{ option.odds || "-" }}</text>
          </view>
        </view>
      </view>
    </view>
    <EmptyState
      v-else
      :title="loading ? t('commerce.sportsDetail.loadingMarkets') : t('commerce.sportsDetail.noMarkets')"
      :description="t('commerce.sportsDetail.noMarketsDescription')"
    />

    <view v-if="selectedOption" class="bet-panel">
      <view class="bet-selection">
        <view>
          <text>{{ selectedMarketName }}</text>
          <text>{{ selectedOption.option_name || selectedOption.option_code }}</text>
        </view>
        <text>{{ t("commerce.common.odds") }} {{ selectedOption.odds || "-" }}</text>
      </view>
      <view class="amount-row">
        <view>
          <text>{{ t("commerce.common.betAmount") }}</text>
          <text>{{ t("commerce.common.balance") }} {{ bundle?.coin || "0" }} {{ t("commerce.common.coin") }}</text>
        </view>
        <input v-model.trim="amount" type="number" :placeholder="t('commerce.common.enterValue')" />
      </view>
      <button class="primary-button" :disabled="submitting || !betOpen || !amount" @tap="submit">
        {{ submitting ? t("commerce.common.submitting") : betOpen ? t("commerce.sportsDetail.confirmBet") : t("commerce.sportsDetail.marketClosed") }}
      </button>
    </view>
  </view>
</template>

<script setup lang="ts">
import { computed, ref } from "vue";
import { onLoad, onPullDownRefresh, onUnload } from "@dcloudio/uni-app";
import EmptyState from "@/components/EmptyState.vue";
import SafeImage from "@/components/SafeImage.vue";
import { getSportsBetMarkets, submitSportsBet } from "@/api/services";
import type {
  SportsMarket,
  SportsMarketBundle,
  SportsMarketOption,
  SportsMatch
} from "@/types/api";
import { requireLogin } from "@/utils/session";
import { useI18n } from "@/i18n";

const { t } = useI18n();

const matchId = ref("");
const bundle = ref<SportsMarketBundle>();
const selectedOption = ref<SportsMarketOption>();
const selectedMarket = ref<SportsMarket>();
const amount = ref("");
const loading = ref(false);
const submitting = ref(false);
const remainingSeconds = ref(0);
let timer: ReturnType<typeof setInterval> | undefined;

const match = computed<SportsMatch>(() => bundle.value?.match || {});
const markets = computed(() => bundle.value?.markets || []);
const betOpen = computed(() => {
  const value = bundle.value?.bet_open ?? bundle.value?.bet_enabled;
  return value === 1 || value === "1";
});
const scoreText = computed(() => {
  const started = match.value.has_started === 1 || match.value.has_started === "1";
  if (!started) {
    return "VS";
  }
  if (match.value.score_text) {
    return String(match.value.score_text);
  }
  const home = textOf(match.value, ["home_score"], "-");
  const away = textOf(match.value, ["away_score"], "-");
  return `${home} : ${away}`;
});
const selectedMarketName = computed(() =>
  String(selectedMarket.value?.market_name || selectedMarket.value?.market_code || t("commerce.sportsDetail.markets"))
);
const countdownText = computed(() => {
  const seconds = Math.max(0, remainingSeconds.value);
  const hours = Math.floor(seconds / 3600);
  const minutes = Math.floor((seconds % 3600) / 60);
  const rest = seconds % 60;
  return [hours, minutes, rest].map((value) => String(value).padStart(2, "0")).join(":");
});

function textOf(source: Record<string, unknown>, keys: string[], fallback = "") {
  for (const key of keys) {
    const value = source[key];
    if (value !== undefined && value !== null && String(value).trim()) {
      return String(value);
    }
  }
  return fallback;
}

function startCountdown() {
  if (timer) {
    clearInterval(timer);
  }
  remainingSeconds.value = Number(bundle.value?.close_countdown || 0);
  timer = setInterval(() => {
    remainingSeconds.value = Math.max(0, remainingSeconds.value - 1);
  }, 1000);
}

async function load() {
  if (!requireLogin() || !matchId.value) {
    uni.stopPullDownRefresh();
    return;
  }
  loading.value = true;
  try {
    bundle.value = await getSportsBetMarkets(matchId.value);
    startCountdown();
  } catch (error: any) {
    uni.showToast({ title: error?.message || t("commerce.sportsDetail.loadFailed"), icon: "none" });
  } finally {
    loading.value = false;
    uni.stopPullDownRefresh();
  }
}

function selectOption(option: SportsMarketOption, market: SportsMarket) {
  if (!betOpen.value) {
    uni.showToast({ title: t("commerce.sportsDetail.currentMarketClosed"), icon: "none" });
    return;
  }
  selectedOption.value = option;
  selectedMarket.value = market;
}

async function submit() {
  if (
    !requireLogin() ||
    submitting.value ||
    !betOpen.value ||
    !selectedOption.value?.id ||
    Number(amount.value) <= 0
  ) {
    return;
  }
  submitting.value = true;
  try {
    await submitSportsBet({
      matchId: matchId.value,
      optionId: selectedOption.value.id,
      amount: amount.value
    });
    uni.showToast({ title: t("commerce.sportsDetail.betSuccess"), icon: "none" });
    amount.value = "";
    selectedOption.value = undefined;
    selectedMarket.value = undefined;
    await load();
  } catch (error: any) {
    uni.showToast({ title: error?.message || t("commerce.sportsDetail.betFailed"), icon: "none" });
  } finally {
    submitting.value = false;
  }
}

function openRecord() {
  uni.navigateTo({
    url: `/pages/sports/record?match_id=${encodeURIComponent(matchId.value)}`
  });
}

onLoad((query) => {
  matchId.value = String(query?.match_id || "");
  uni.setNavigationBarTitle({ title: t("commerce.sportsDetail.navigationTitle") });
  void load();
});

onPullDownRefresh(load);

onUnload(() => {
  if (timer) {
    clearInterval(timer);
  }
});
</script>

<style scoped>
.sports-detail-page {
  min-height: 100vh;
  padding: 24rpx 24rpx 44rpx;
  background: var(--bg);
}

.match-card {
  overflow: hidden;
  padding: 28rpx;
  border-radius: 26rpx;
  color: #fff;
  background: linear-gradient(145deg, #15382f, #215f4e 60%, #2c846a);
  box-shadow: 0 18rpx 42rpx rgba(22, 83, 68, 0.2);
}

.match-head,
.match-foot,
.bet-selection,
.amount-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
}

.match-head > text:first-child {
  font-size: 26rpx;
  font-weight: 900;
}

.status-pill {
  padding: 9rpx 18rpx;
  border-radius: 24rpx;
  font-size: 21rpx;
  background: rgba(255, 255, 255, 0.16);
}

.teams {
  display: grid;
  grid-template-columns: minmax(0, 1fr) 150rpx minmax(0, 1fr);
  align-items: center;
  margin: 34rpx 0 30rpx;
}

.team,
.score {
  display: flex;
  min-width: 0;
  flex-direction: column;
  align-items: center;
}

.team-logo {
  width: 82rpx;
  height: 82rpx;
  padding: 8rpx;
  border-radius: 50%;
  background: #fff;
}

.team text {
  overflow: hidden;
  width: 100%;
  margin-top: 14rpx;
  font-size: 24rpx;
  font-weight: 800;
  text-align: center;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.score text:first-child {
  font-size: 42rpx;
  font-weight: 900;
}

.score text:last-child {
  margin-top: 12rpx;
  color: rgba(255, 255, 255, 0.75);
  font-size: 21rpx;
}

.match-foot {
  padding-top: 22rpx;
  border-top: 1rpx solid rgba(255, 255, 255, 0.14);
}

.match-foot text {
  color: rgba(255, 255, 255, 0.84);
  font-size: 22rpx;
}

.match-foot button {
  height: 52rpx;
  padding: 0 20rpx;
  border-radius: 26rpx;
  color: #fff;
  font-size: 22rpx;
  background: rgba(255, 255, 255, 0.16);
}

.section-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin: 34rpx 4rpx 18rpx;
}

.section-head text:first-child {
  color: var(--ink);
  font-size: 31rpx;
  font-weight: 900;
}

.section-head text:last-child {
  color: var(--ink-3);
  font-size: 23rpx;
}

.market-card {
  padding: 24rpx;
  margin-bottom: 18rpx;
  border: 1rpx solid #e8edf0;
  border-radius: 22rpx;
  background: #fff;
}

.market-name {
  color: var(--ink);
  font-size: 26rpx;
  font-weight: 900;
}

.option-grid {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 14rpx;
  margin-top: 20rpx;
}

.option {
  display: flex;
  min-height: 92rpx;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  border: 2rpx solid #e8edf0;
  border-radius: 16rpx;
  background: #f8faf9;
}

.option text:first-child {
  color: var(--ink-2);
  font-size: 22rpx;
}

.option text:last-child {
  margin-top: 8rpx;
  color: #208667;
  font-size: 27rpx;
  font-weight: 900;
}

.option.active {
  border-color: #2f9b78;
  background: #eaf8f3;
}

.bet-panel {
  padding: 26rpx;
  margin-top: 22rpx;
  border: 1rpx solid #e8edf0;
  border-radius: 24rpx;
  background: #fff;
  box-shadow: 0 16rpx 40rpx rgba(31, 48, 44, 0.08);
}

.bet-selection > view text,
.amount-row > view text {
  display: block;
}

.bet-selection > view text:first-child,
.amount-row > view text:first-child {
  color: var(--ink);
  font-size: 25rpx;
  font-weight: 900;
}

.bet-selection > view text:last-child,
.amount-row > view text:last-child {
  margin-top: 8rpx;
  color: var(--ink-3);
  font-size: 21rpx;
}

.bet-selection > text {
  color: #208667;
  font-size: 28rpx;
  font-weight: 900;
}

.amount-row {
  margin: 24rpx 0;
}

.amount-row input {
  width: 210rpx;
  height: 72rpx;
  padding: 0 20rpx;
  border-radius: 16rpx;
  color: var(--ink);
  font-size: 28rpx;
  text-align: right;
  background: #f4f6f5;
}

.primary-button {
  background: linear-gradient(135deg, #2a9674, #1f725a);
}
</style>
