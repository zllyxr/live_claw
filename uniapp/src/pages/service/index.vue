<template>
  <view class="safe-page service-page">
    <view class="hero">
      <view class="hero-icon">
        <image src="/static/native/me_service_support.svg" mode="aspectFit" />
      </view>
      <text class="hero-title">{{ t("misc.service.onlineSupport") }}</text>
      <text class="hero-desc">{{ t("misc.service.description") }}</text>
    </view>

    <view class="service-card">
      <view class="service-main">
        <text class="service-label">{{ t("misc.service.platformSupport") }}</text>
        <text class="service-url">{{ serviceUrl || (loading ? t("misc.service.loadingConfig") : t("misc.service.nativeSupport")) }}</text>
      </view>
      <button class="service-button" :disabled="loading" @tap="openCustomerService">
        {{ loading ? t("misc.common.loading") : t("misc.service.contact") }}
      </button>
    </view>

    <view class="action-list card">
      <view class="action-row" @tap="openMessages">
        <view>
          <text>{{ t("misc.service.messageCenter") }}</text>
          <text>{{ t("misc.service.messageCenterDesc") }}</text>
        </view>
        <text>›</text>
      </view>
      <view class="action-row" @tap="openRecharge">
        <view>
          <text>{{ t("misc.service.rechargePayment") }}</text>
          <text>{{ t("misc.service.rechargePaymentDesc") }}</text>
        </view>
        <text>›</text>
      </view>
      <view class="action-row" @tap="openInvite">
        <view>
          <text>{{ t("misc.service.inviteRewards") }}</text>
          <text>{{ t("misc.service.inviteRewardsDesc") }}</text>
        </view>
        <text>›</text>
      </view>
    </view>

    <view class="tips card">
      <text>{{ t("misc.service.beforeContact") }}</text>
      <text>{{ t("misc.service.paymentTip") }}</text>
      <text>{{ t("misc.service.accountTip") }}</text>
      <text>{{ t("misc.service.betTip") }}</text>
    </view>
  </view>
</template>

<script setup lang="ts">
import { computed, ref } from "vue";
import { onLoad, onPullDownRefresh } from "@dcloudio/uni-app";
import { getCachedConfig, getConfig } from "@/api/services";
import { normalizePageUrl, openWebView } from "@/utils/navigation";
import { requireLogin } from "@/utils/session";
import { t } from "@/i18n";

const config = ref<Record<string, unknown>>();
const loading = ref(false);

const serviceUrl = computed(() => {
  const source = config.value || {};
  return normalizePageUrl(String(source.service_url || source.serviceUrl || ""));
});

function applyConfig(next?: Record<string, unknown>) {
  if (next) {
    config.value = next;
  }
}

async function refreshConfig(silent = false) {
  if (loading.value) {
    return;
  }
  loading.value = true;
  try {
    applyConfig(await getConfig());
  } catch (error: any) {
    if (!silent) {
      uni.showToast({ title: error?.message || t("misc.service.configFailed"), icon: "none" });
    }
  } finally {
    loading.value = false;
    uni.stopPullDownRefresh();
  }
}

async function openCustomerService() {
  if (!requireLogin()) {
    return;
  }
  if (serviceUrl.value) {
    openWebView(serviceUrl.value, t("misc.service.onlineSupport"));
    return;
  }
  uni.navigateTo({ url: "/pages/service/chat" });
}

function openMessages() {
  uni.navigateTo({ url: "/pages/message/index" });
}

function openRecharge() {
  uni.navigateTo({ url: "/pages/wallet/recharge" });
}

function openInvite() {
  uni.navigateTo({ url: "/pages/invite/index" });
}

onLoad(() => {
  applyConfig(getCachedConfig());
  void refreshConfig(true);
});

onPullDownRefresh(() => {
  void refreshConfig(false);
});
</script>

<style scoped>
.service-page {
  min-height: 100vh;
  background: var(--bg);
}

.hero {
  display: flex;
  min-height: 300rpx;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  padding: 34rpx;
  border-radius: 0 0 34rpx 34rpx;
  text-align: center;
  background: linear-gradient(145deg, #fff2f6 0%, #eef9ff 100%);
}

.hero-icon {
  display: flex;
  width: 112rpx;
  height: 112rpx;
  align-items: center;
  justify-content: center;
  border-radius: 36rpx;
  background: #fff;
  box-shadow: 0 18rpx 46rpx rgba(255, 88, 120, 0.18);
}

.hero-icon image {
  width: 70rpx;
  height: 70rpx;
}

.hero-title {
  margin-top: 22rpx;
  color: var(--ink);
  font-size: 40rpx;
  font-weight: 900;
}

.hero-desc {
  max-width: 560rpx;
  margin-top: 14rpx;
  color: #7b8494;
  font-size: 25rpx;
  line-height: 1.5;
}

.service-card {
  display: flex;
  gap: 22rpx;
  align-items: center;
  margin: 24rpx 24rpx 0;
  padding: 28rpx;
  border-radius: 18rpx;
  background: #fff;
  box-shadow: 0 14rpx 40rpx rgba(33, 38, 54, 0.06);
}

.service-main {
  min-width: 0;
  flex: 1;
}

.service-label {
  display: block;
  color: var(--ink);
  font-size: 30rpx;
  font-weight: 900;
}

.service-url {
  display: block;
  overflow: hidden;
  margin-top: 10rpx;
  color: var(--ink-3);
  font-size: 23rpx;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.service-button {
  display: flex;
  width: 220rpx;
  height: 76rpx;
  align-items: center;
  justify-content: center;
  margin: 0;
  border-radius: 999rpx;
  color: #fff;
  font-size: 25rpx;
  font-weight: 800;
  line-height: 76rpx;
  background: linear-gradient(135deg, var(--brand), #ff9a64);
}

.service-button[disabled] {
  opacity: 0.55;
}

.action-list,
.tips {
  overflow: hidden;
  margin: 22rpx 24rpx 0;
}

.action-row {
  display: flex;
  align-items: center;
  min-height: 118rpx;
  padding: 22rpx 24rpx;
  border-bottom: 1rpx solid #f0f2f6;
}

.action-row:last-child {
  border-bottom: 0;
}

.action-row view {
  min-width: 0;
  flex: 1;
}

.action-row view text {
  display: block;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.action-row view text:first-child {
  color: var(--ink);
  font-size: 28rpx;
  font-weight: 900;
}

.action-row view text:last-child {
  margin-top: 8rpx;
  color: var(--ink-3);
  font-size: 23rpx;
}

.action-row > text {
  margin-left: 18rpx;
  color: #b8bfcc;
  font-size: 42rpx;
}

.tips {
  padding: 28rpx;
}

.tips text {
  display: block;
}

.tips text:first-child {
  color: var(--ink);
  font-size: 30rpx;
  font-weight: 900;
}

.tips text:not(:first-child) {
  margin-top: 16rpx;
  color: #7b8494;
  font-size: 24rpx;
  line-height: 1.5;
}
</style>
