<template>
  <view class="safe-page service-page">
    <view class="hero">
      <view class="hero-icon">
        <image src="/static/native/me_service_support.svg" mode="aspectFit" />
      </view>
      <text class="hero-title">在线客服</text>
      <text class="hero-desc">平台客服可处理充值、账号、游戏体育、直播间相关问题。</text>
    </view>

    <view class="service-card">
      <view class="service-main">
        <text class="service-label">平台客服</text>
        <text class="service-url">{{ serviceUrl || (loading ? "正在获取客服链接" : "客服链接未配置") }}</text>
      </view>
      <button class="service-button" :disabled="loading" @tap="openCustomerService">
        {{ loading ? "加载中" : "联系在线客服" }}
      </button>
    </view>

    <view class="action-list card">
      <view class="action-row" @tap="openMessages">
        <view>
          <text>消息中心</text>
          <text>查看系统通知与私信</text>
        </view>
        <text>›</text>
      </view>
      <view class="action-row" @tap="openRecharge">
        <view>
          <text>充值与支付</text>
          <text>查看充值档位、支付方式和明细</text>
        </view>
        <text>›</text>
      </view>
      <view class="action-row" @tap="openInvite">
        <view>
          <text>邀请奖励</text>
          <text>查看邀请码与绑定入口</text>
        </view>
        <text>›</text>
      </view>
    </view>

    <view class="tips card">
      <text>联系客服前</text>
      <text>支付未到账：准备订单时间、支付金额和支付方式。</text>
      <text>账号异常：准备账号 ID、手机号和异常截图。</text>
      <text>投注问题：先进入对应投注记录页，带上期号和投注内容。</text>
    </view>
  </view>
</template>

<script setup lang="ts">
import { computed, ref } from "vue";
import { onLoad, onPullDownRefresh } from "@dcloudio/uni-app";
import { getCachedConfig, getConfig } from "@/api/services";
import { normalizePageUrl, openWebView } from "@/utils/navigation";

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
      uni.showToast({ title: error?.message || "客服配置获取失败", icon: "none" });
    }
  } finally {
    loading.value = false;
    uni.stopPullDownRefresh();
  }
}

async function openCustomerService() {
  if (!serviceUrl.value) {
    await refreshConfig(false);
  }
  if (!serviceUrl.value) {
    uni.showToast({ title: "客服链接未配置", icon: "none" });
    return;
  }
  openWebView(serviceUrl.value, "在线客服");
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
