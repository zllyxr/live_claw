<template>
  <view v-if="visible" class="venue-mask" @tap="emit('close')">
    <view class="venue-sheet" @tap.stop>
      <view class="venue-head">
        <view>
          <text class="venue-kicker">DEEP SEA ARENA</text>
          <text class="venue-title">{{ t("misc.fishing.title") }}</text>
          <text class="venue-subtitle">{{ t("misc.fishing.subtitle") }}</text>
        </view>
        <view class="venue-close" @tap="emit('close')">×</view>
      </view>

      <view class="venue-list">
        <view
          v-for="(venue, index) in normalizedVenues"
          :key="venue.venue_code"
          class="venue-card"
          :class="`venue-card-${index + 1}`"
          @tap="emit('select', venue)"
        >
          <view class="venue-orb">
            <text>×{{ venue.multiplier }}</text>
          </view>
          <view class="venue-copy">
            <view class="venue-name-row">
              <text class="venue-name">{{ venue.venue_name }}</text>
              <text class="venue-state">{{ balanceState(venue) }}</text>
            </view>
            <text class="venue-meta">
              {{ t("misc.fishing.minimum") }} {{ formatCoin(venue.min_balance) }} · {{ t("misc.fishing.walletSettlement") }}
            </text>
            <text class="venue-bets">{{ t("misc.fishing.betLevels") }} {{ venue.bet_levels.join(" / ") }}</text>
          </view>
          <view class="venue-enter">{{ t("misc.common.enter") }}</view>
        </view>
      </view>

      <view class="venue-note">
        {{ t("misc.fishing.matchNote") }}
      </view>
    </view>
  </view>
</template>

<script setup lang="ts">
import { computed } from "vue";
import type { FishingVenue } from "@/types/api";
import { t } from "@/i18n";

type NormalizedVenue = {
  venue_id: string;
  venue_code: string;
  venue_name: string;
  multiplier: number;
  table_count: number;
  seats_per_table: number;
  min_balance: number;
  escrow_amount: number;
  bet_levels: number[];
};

const props = defineProps<{
  visible: boolean;
  venues?: FishingVenue[];
  balance?: string | number;
}>();

const emit = defineEmits<{
  close: [];
  select: [venue: NormalizedVenue];
}>();

const defaultVenues = computed<FishingVenue[]>(() => [
  {
    venue_code: "novice",
    venue_name: t("misc.fishing.novice"),
    multiplier: 1,
    table_count: 300,
    seats_per_table: 4,
    min_balance: 100,
    escrow_amount: 0,
    bet_levels: [1, 2, 5, 10]
  },
  {
    venue_code: "expert",
    venue_name: t("misc.fishing.expert"),
    multiplier: 5,
    table_count: 300,
    seats_per_table: 4,
    min_balance: 500,
    escrow_amount: 0,
    bet_levels: [5, 10, 25, 50]
  },
  {
    venue_code: "master",
    venue_name: t("misc.fishing.master"),
    multiplier: 10,
    table_count: 300,
    seats_per_table: 4,
    min_balance: 1000,
    escrow_amount: 0,
    bet_levels: [10, 20, 50, 100]
  }
]);

const normalizedVenues = computed<NormalizedVenue[]>(() => {
  const source = props.venues?.length ? props.venues : defaultVenues.value;
  return source
    .map((venue, index) => ({
      venue_id: String(venue.venue_id || ""),
      venue_code: String(venue.venue_code || ["novice", "expert", "master"][index] || ""),
      venue_name: String(venue.venue_name || [t("misc.fishing.novice"), t("misc.fishing.expert"), t("misc.fishing.master")][index] || t("misc.fishing.venue")),
      multiplier: Math.max(1, Number(venue.multiplier || 1)),
      table_count: Math.max(1, Number(venue.table_count || 300)),
      seats_per_table: Math.max(1, Number(venue.seats_per_table || 4)),
      min_balance: Math.max(0, Number(venue.min_balance || 0)),
      escrow_amount: Math.max(0, Number(venue.escrow_amount || 0)),
      bet_levels: (venue.bet_levels || []).map(Number).filter((value) => value > 0)
    }))
    .filter((venue) => venue.venue_code);
});

function formatCoin(value: number) {
  return new Intl.NumberFormat("zh-CN").format(value);
}

function balanceState(venue: NormalizedVenue) {
  const balance = Number(props.balance);
  if (!Number.isFinite(balance) || props.balance === undefined || props.balance === "") {
    return `${venue.table_count} ${t("misc.fishing.tables")}`;
  }
  return balance >= venue.min_balance ? t("misc.fishing.available") : t("misc.fishing.insufficientBalance");
}
</script>

<style scoped>
.venue-mask {
  position: fixed;
  z-index: 9900;
  inset: 0;
  display: flex;
  align-items: flex-end;
  justify-content: center;
  padding: 24rpx;
  background: rgba(1, 8, 25, 0.72);
  backdrop-filter: blur(12rpx);
}

.venue-sheet {
  width: 100%;
  max-width: 720rpx;
  padding: 30rpx 26rpx calc(24rpx + env(safe-area-inset-bottom));
  border: 2rpx solid rgba(139, 185, 255, 0.3);
  border-radius: 38rpx;
  background:
    radial-gradient(circle at 88% 4%, rgba(95, 102, 255, 0.28), transparent 34%),
    linear-gradient(155deg, #102c5a, #071a3c 62%, #06142f);
  box-shadow: 0 -28rpx 80rpx rgba(0, 5, 24, 0.46);
}

.venue-head {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 20rpx;
  padding: 0 4rpx 24rpx;
}

.venue-kicker,
.venue-title,
.venue-subtitle {
  display: block;
}

.venue-kicker {
  color: #75d9ff;
  font-size: 20rpx;
  font-weight: 800;
  letter-spacing: 4rpx;
}

.venue-title {
  margin-top: 6rpx;
  color: #fff;
  font-size: 38rpx;
  font-weight: 900;
}

.venue-subtitle {
  margin-top: 8rpx;
  color: #9db1d3;
  font-size: 23rpx;
}

.venue-close {
  display: flex;
  width: 64rpx;
  height: 64rpx;
  flex: 0 0 64rpx;
  align-items: center;
  justify-content: center;
  border: 2rpx solid rgba(255, 255, 255, 0.14);
  border-radius: 50%;
  color: #dfeaff;
  background: rgba(255, 255, 255, 0.08);
  font-size: 42rpx;
  line-height: 1;
  text-align: center;
}

.venue-list {
  display: grid;
  gap: 16rpx;
}

.venue-card {
  display: flex;
  min-height: 142rpx;
  align-items: center;
  gap: 18rpx;
  padding: 18rpx;
  border: 2rpx solid rgba(255, 255, 255, 0.12);
  border-radius: 28rpx;
  background: rgba(255, 255, 255, 0.08);
}

.venue-card-2 {
  border-color: rgba(153, 128, 255, 0.32);
  background: linear-gradient(100deg, rgba(111, 79, 224, 0.2), rgba(255, 255, 255, 0.07));
}

.venue-card-3 {
  border-color: rgba(255, 190, 84, 0.38);
  background: linear-gradient(100deg, rgba(202, 126, 35, 0.22), rgba(255, 255, 255, 0.07));
}

.venue-orb {
  display: flex;
  width: 92rpx;
  height: 92rpx;
  flex: 0 0 92rpx;
  align-items: center;
  justify-content: center;
  border-radius: 28rpx;
  color: #04234d;
  background: linear-gradient(145deg, #9efff2, #5cb7ff);
  box-shadow: inset 0 2rpx 0 rgba(255, 255, 255, 0.56);
  font-size: 30rpx;
  font-weight: 900;
  text-align: center;
}

.venue-card-2 .venue-orb {
  color: #fff;
  background: linear-gradient(145deg, #b79cff, #7257e8);
}

.venue-card-3 .venue-orb {
  color: #4b2700;
  background: linear-gradient(145deg, #ffe59a, #ffad3f);
}

.venue-copy {
  min-width: 0;
  flex: 1;
}

.venue-name-row {
  display: flex;
  align-items: center;
  gap: 12rpx;
}

.venue-name {
  color: #fff;
  font-size: 30rpx;
  font-weight: 900;
}

.venue-state {
  display: inline-flex;
  min-height: 34rpx;
  align-items: center;
  justify-content: center;
  padding: 0 12rpx;
  border-radius: 999rpx;
  color: #9debdc;
  background: rgba(69, 219, 188, 0.13);
  font-size: 19rpx;
  font-weight: 700;
  text-align: center;
}

.venue-meta,
.venue-bets {
  display: block;
  overflow: hidden;
  margin-top: 7rpx;
  color: #b5c3dc;
  font-size: 22rpx;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.venue-bets {
  color: #7089b0;
  font-size: 20rpx;
}

.venue-enter {
  display: flex;
  min-width: 82rpx;
  height: 58rpx;
  align-items: center;
  justify-content: center;
  border-radius: 20rpx;
  color: #071a3c;
  background: #fff;
  font-size: 23rpx;
  font-weight: 900;
  text-align: center;
}

.venue-note {
  display: flex;
  min-height: 62rpx;
  align-items: center;
  justify-content: center;
  padding-top: 18rpx;
  color: #7287aa;
  font-size: 21rpx;
  text-align: center;
}
</style>
