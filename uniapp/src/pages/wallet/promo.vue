<template>
  <view class="safe-page promo-page">
    <view class="hero">
      <text>星域优惠</text>
      <text>充值赠送、活动福利与账户奖励</text>
    </view>

    <view class="section-head">
      <text>充值赠送</text>
      <button @tap="openRecharge">去充值</button>
    </view>

    <view v-if="promoRules.length" class="promo-grid">
      <view v-for="rule in promoRules" :key="String(rule.id || rule.money)" class="promo-card">
        <text>{{ rule.coin || "0" }} 星币</text>
        <text>支付 ¥{{ rule.money || "0" }}</text>
        <view class="gift-line">赠送 {{ rule.give }} 星币</view>
      </view>
    </view>
    <EmptyState v-else :title="loading ? '正在加载优惠' : '暂无充值赠送活动'" description="有赠送档位时会在这里展示，也可以去充值页查看全部档位。" />

    <view class="benefit-list card">
      <view class="benefit-row">
        <view class="benefit-icon">充</view>
        <view>
          <text>充值奖励</text>
          <text>充值页会实时读取后端档位和支付方式。</text>
        </view>
      </view>
      <view class="benefit-row">
        <view class="benefit-icon pink">任</view>
        <view>
          <text>每日任务</text>
          <text>完成任务后可领取奖励，奖励进入余额。</text>
        </view>
      </view>
      <view class="benefit-row">
        <view class="benefit-icon violet">邀</view>
        <view>
          <text>邀请奖励</text>
          <text>邀请好友注册绑定后，可在邀请页查看专属信息。</text>
        </view>
      </view>
    </view>
  </view>
</template>

<script setup lang="ts">
import { computed, ref } from "vue";
import { onPullDownRefresh, onShow } from "@dcloudio/uni-app";
import EmptyState from "@/components/EmptyState.vue";
import { getWalletBalance } from "@/api/services";
import type { WalletBalance, WalletRule } from "@/types/api";

const wallet = ref<WalletBalance>();
const loading = ref(false);
const promoRules = computed<WalletRule[]>(() => (wallet.value?.rules || []).filter((item) => Number(item.give || 0) > 0));

async function load() {
  loading.value = true;
  try {
    wallet.value = await getWalletBalance();
  } catch (error: any) {
    uni.showToast({ title: error?.message || "优惠加载失败", icon: "none" });
  } finally {
    loading.value = false;
    uni.stopPullDownRefresh();
  }
}

function openRecharge() {
  uni.navigateTo({ url: "/pages/wallet/recharge" });
}

onShow(() => {
  void load();
});

onPullDownRefresh(() => {
  void load();
});
</script>

<style scoped>
.promo-page {
  background: var(--bg);
}

.hero {
  min-height: 190rpx;
  padding: 34rpx 30rpx;
  border-radius: 24rpx;
  color: #fff;
  background: linear-gradient(135deg, #522e80, var(--brand) 62%, #ffb74d);
  box-shadow: 0 14rpx 34rpx rgba(255, 88, 120, 0.18);
}

.hero text {
  display: block;
}

.hero text:first-child {
  font-size: 42rpx;
  font-weight: 900;
}

.hero text:last-child {
  margin-top: 18rpx;
  color: rgba(255, 255, 255, 0.86);
  font-size: 25rpx;
}

.section-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin: 34rpx 2rpx 20rpx;
}

.section-head text {
  color: var(--ink);
  font-size: 31rpx;
  font-weight: 900;
}

.section-head button {
  display: flex;
  min-width: 118rpx;
  height: 58rpx;
  align-items: center;
  justify-content: center;
  border-radius: 29rpx;
  color: #fff;
  font-size: 24rpx;
  font-weight: 900;
  background: var(--brand);
}

.promo-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 18rpx;
}

.promo-card {
  min-height: 168rpx;
  padding: 24rpx;
  border: 2rpx solid #ffe0e8;
  border-radius: 20rpx;
  background: #fff;
}

.promo-card text {
  display: block;
}

.promo-card text:first-child {
  color: var(--ink);
  font-size: 30rpx;
  font-weight: 900;
}

.promo-card text:nth-child(2) {
  margin-top: 12rpx;
  color: var(--ink-3);
  font-size: 23rpx;
}

.gift-line {
  display: inline-flex;
  max-width: 100%;
  height: 44rpx;
  align-items: center;
  margin-top: 18rpx;
  padding: 0 16rpx;
  border-radius: 22rpx;
  color: var(--brand);
  font-size: 22rpx;
  font-weight: 900;
  background: #fff1f4;
}

.benefit-list {
  overflow: hidden;
  margin-top: 28rpx;
}

.benefit-row {
  display: flex;
  gap: 18rpx;
  padding: 24rpx;
  border-bottom: 1rpx solid #f0f2f6;
}

.benefit-row:last-child {
  border-bottom: 0;
}

.benefit-icon {
  display: flex;
  width: 58rpx;
  height: 58rpx;
  flex: 0 0 auto;
  align-items: center;
  justify-content: center;
  border-radius: 18rpx;
  color: #fff;
  font-size: 22rpx;
  font-weight: 900;
  background: #ff8a4d;
}

.benefit-icon.pink {
  background: var(--brand);
}

.benefit-icon.violet {
  background: #7c5cff;
}

.benefit-row text {
  display: block;
}

.benefit-row text:first-child {
  color: var(--ink);
  font-size: 28rpx;
  font-weight: 900;
}

.benefit-row text:last-child {
  margin-top: 8rpx;
  color: var(--ink-3);
  font-size: 24rpx;
  line-height: 1.45;
}
</style>
