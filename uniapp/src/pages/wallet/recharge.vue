<template>
  <view class="safe-page recharge-page">
    <view class="balance-card">
      <view>
        <text class="label">我的星币</text>
        <text class="coin">{{ wallet?.coin || "0" }}</text>
      </view>
      <view class="balance-side">
        <text>积分 {{ wallet?.score || "0" }}</text>
        <button @tap="load">刷新</button>
      </view>
    </view>

    <view class="section-head">
      <text>充值金额</text>
      <button @tap="openChargeDetail">充值明细</button>
    </view>

    <view v-if="rules.length" class="rule-grid">
      <view
        v-for="(rule, index) in rules"
        :key="String(rule.id || index)"
        class="rule-card"
        :class="{ active: selectedRuleIndex === index }"
        @tap="selectedRuleIndex = index"
      >
        <text class="rule-coin">{{ coinOf(rule) }} 星币</text>
        <text class="rule-money">¥{{ rule.money || "0" }}</text>
        <text v-if="Number(rule.give || 0) > 0" class="give">赠 {{ rule.give }}</text>
      </view>
    </view>
    <EmptyState v-else :title="loading ? '正在加载充值档位' : '暂无充值档位'" description="下拉刷新可重新获取支付配置。" />

    <view class="section-head pay-head">
      <text class="section-head-start">支付方式</text>
      <text class="section-head-end" @tap="openAgreement">充值协议</text>
    </view>

    <view v-if="payMethods.length" class="pay-list">
      <view
        v-for="(pay, index) in payMethods"
        :key="String(pay.id || index)"
        class="pay-row"
        :class="{ active: selectedPayIndex === index }"
        @tap="selectedPayIndex = index"
      >
        <image class="pay-icon" :src="payIcon(pay)" mode="aspectFit" />
        <view class="pay-main">
          <text>{{ pay.name || payName(pay.id) }}</text>
          <text>{{ pay.href ? "H5/外部收银台" : "App 原生支付" }}</text>
        </view>
        <view class="radio" />
      </view>
    </view>
    <view v-else class="pay-empty">当前没有可用支付方式</view>

    <button class="primary-button charge-button" :disabled="submitting || !selectedRule || !selectedPay" @tap="charge">
      {{ chargeButtonText }}
    </button>
  </view>
</template>

<script setup lang="ts">
import { computed, ref } from "vue";
import { onPullDownRefresh, onShow } from "@dcloudio/uni-app";
import EmptyState from "@/components/EmptyState.vue";
import { createCoinOrder, getWalletBalance } from "@/api/services";
import type { WalletBalance, WalletPayMethod, WalletRule } from "@/types/api";
import { absolutizeUrl, firstText } from "@/utils/url";
import { requireLogin } from "@/utils/session";
import { openWebView } from "@/utils/navigation";
import { API_HOST } from "@/constants/config";

const wallet = ref<WalletBalance>();
const selectedRuleIndex = ref(0);
const selectedPayIndex = ref(0);
const loading = ref(false);
const submitting = ref(false);

const rules = computed(() => parseList<WalletRule>(wallet.value?.rules));
const payMethods = computed(() => parseList<WalletPayMethod>(wallet.value?.paylist));
const selectedRule = computed(() => rules.value[selectedRuleIndex.value]);
const selectedPay = computed(() => payMethods.value[selectedPayIndex.value]);
const chargeButtonText = computed(() => {
  if (!selectedRule.value) {
    return "请选择充值金额";
  }
  const coin = coinOf(selectedRule.value);
  return submitting.value ? "正在创建订单" : `立即充值 ${coin} 星币`;
});

function parseList<T>(value: unknown): T[] {
  if (Array.isArray(value)) {
    return value as T[];
  }
  if (typeof value === "string" && value.trim()) {
    try {
      const parsed = JSON.parse(value);
      return Array.isArray(parsed) ? (parsed as T[]) : [];
    } catch {
      return [];
    }
  }
  return [];
}

function payName(id?: string) {
  const map: Record<string, string> = {
    ali: "支付宝",
    wx: "微信支付",
    paypal: "PayPal",
    usdt: "USDT"
  };
  return map[String(id || "")] || "支付方式";
}

function coinOf(rule: WalletRule) {
  if (String(selectedPay.value?.id || "") === "paypal") {
    return firstText(rule.coin_paypal, rule.coin, "0");
  }
  return firstText(rule.coin, "0");
}

function payIcon(pay: WalletPayMethod) {
  return absolutizeUrl(String(pay.thumb || "")) || "/static/native/me_wallet.png";
}

function normalizeHref(href: string) {
  const raw = href.trim();
  if (/^[a-z][a-z0-9+.-]*:/i.test(raw)) {
    return raw;
  }
  if (raw.startsWith("//")) {
    return `https:${raw}`;
  }
  if (raw.startsWith("/")) {
    return `${API_HOST}${raw}`;
  }
  return raw;
}

function openPaymentUrl(url: string, title = "支付") {
  const normalized = normalizeHref(url);
  if (/^https?:\/\//i.test(normalized)) {
    openWebView(normalized, title);
    return;
  }
  const plusRuntime = (globalThis as any).plus?.runtime;
  if (plusRuntime?.openURL) {
    plusRuntime.openURL(normalized);
    return;
  }
  uni.showModal({
    title: "支付链接",
    content: normalized,
    showCancel: false,
    confirmColor: "#ff5878"
  });
}

async function load() {
  if (!requireLogin()) {
    uni.stopPullDownRefresh();
    return;
  }
  loading.value = true;
  try {
    wallet.value = await getWalletBalance();
    selectedRuleIndex.value = 0;
    selectedPayIndex.value = 0;
  } catch (error: any) {
    uni.showToast({ title: error?.message || "充值配置加载失败", icon: "none" });
  } finally {
    loading.value = false;
    uni.stopPullDownRefresh();
  }
}

async function charge() {
  if (!requireLogin() || !selectedRule.value || !selectedPay.value || submitting.value) {
    return;
  }
  const href = String(selectedPay.value.href || "");
  if (href) {
    openPaymentUrl(href, selectedPay.value.name || "支付");
    return;
  }
  submitting.value = true;
  try {
    const order = await createCoinOrder(selectedRule.value, selectedPay.value);
    const paymentUrl = firstText(order?.payment_url, order?.payurl, order?.url, order?.href, order?.qrcode);
    if (paymentUrl) {
      openPaymentUrl(paymentUrl, selectedPay.value.name || "支付");
      return;
    }
    uni.showModal({
      title: "支付暂不可用",
      content: "支付通道未返回可用收银台，请稍后重试或更换支付方式。",
      showCancel: false,
      confirmColor: "#ff5878"
    });
  } catch (error: any) {
    uni.showToast({ title: error?.message || "创建订单失败", icon: "none" });
  } finally {
    submitting.value = false;
  }
}

function openAgreement() {
  uni.navigateTo({ url: "/pages/detail/index?type=recharge_agreement" });
}

function openChargeDetail() {
  uni.navigateTo({ url: "/pages/wallet/detail?type=charge" });
}

onShow(() => {
  void load();
});

onPullDownRefresh(() => {
  void load();
});
</script>

<style scoped>
.recharge-page {
  background: var(--bg);
}

.balance-card {
  display: flex;
  justify-content: space-between;
  min-height: 190rpx;
  padding: 30rpx;
  border-radius: 22rpx;
  color: #fff;
  background: linear-gradient(135deg, var(--brand) 0%, #ff8a4d 100%);
  box-shadow: 0 14rpx 34rpx rgba(255, 88, 120, 0.22);
}

.label {
  display: block;
  font-size: 24rpx;
  opacity: 0.88;
}

.coin {
  display: block;
  margin-top: 22rpx;
  font-size: 56rpx;
  font-weight: 900;
  line-height: 1;
}

.balance-side {
  display: flex;
  flex-direction: column;
  align-items: flex-end;
  justify-content: space-between;
  font-size: 24rpx;
}

.balance-side button,
.section-head button {
  display: flex;
  min-width: 116rpx;
  height: 52rpx;
  align-items: center;
  justify-content: center;
  border-radius: 26rpx;
  color: var(--brand);
  font-size: 23rpx;
  font-weight: 800;
  background: #fff;
}

.section-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin: 38rpx 2rpx 20rpx;
}
.section-head-end{
  color: #0a8f66;
  font-size: 24rpx;

}
.section-head-start{
  font-size: 26rpx;
  font-weight: bold;
}

.section-head button {
  border: 1rpx solid #f1d7de;
  color: var(--brand);
}

.rule-grid {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 16rpx;
}

.rule-card {
  position: relative;
  min-height: 148rpx;
  padding: 24rpx 12rpx;
  border: 2rpx solid #edf0f5;
  border-radius: 18rpx;
  text-align: center;
  background: #fff;
}

.rule-card.active {
  border-color: var(--brand);
  background: #fff5f7;
}

.rule-coin {
  display: block;
  color: var(--ink);
  font-size: 27rpx;
  font-weight: 900;
}

.rule-money {
  display: block;
  margin-top: 18rpx;
  color: var(--brand);
  font-size: 25rpx;
  font-weight: 900;
}

.give {
  position: absolute;
  top: -1rpx;
  right: -1rpx;
  min-width: 72rpx;
  height: 34rpx;
  padding: 0 10rpx;
  border-radius: 0 16rpx 0 16rpx;
  color: #fff;
  font-size: 19rpx;
  line-height: 34rpx;
  background: #8b5cf6;
}

.pay-head {
  margin-top: 42rpx;
}

.pay-list {
  overflow: hidden;
  border: 1rpx solid #e9edf4;
  border-radius: 18rpx;
  background: #fff;
}

.pay-row {
  display: flex;
  align-items: center;
  min-height: 112rpx;
  padding: 20rpx 24rpx;
  border-bottom: 1rpx solid #f0f2f6;
}

.pay-row:last-child {
  border-bottom: 0;
}

.pay-row.active .radio {
  border-color: var(--brand);
  background: var(--brand);
  box-shadow: inset 0 0 0 8rpx #fff;
}

.pay-icon {
  width: 58rpx;
  height: 58rpx;
  margin-right: 18rpx;
}

.pay-main {
  flex: 1;
  min-width: 0;
}

.pay-main text:first-child {
  display: block;
  color: var(--ink);
  font-size: 28rpx;
  font-weight: 900;
}

.pay-main text:last-child {
  display: block;
  margin-top: 10rpx;
  color: var(--ink-3);
  font-size: 23rpx;
}

.radio {
  width: 36rpx;
  height: 36rpx;
  border: 3rpx solid #cbd2df;
  border-radius: 50%;
}

.pay-empty {
  padding: 36rpx;
  border-radius: 18rpx;
  color: var(--ink-3);
  font-size: 25rpx;
  text-align: center;
  background: #fff;
}

.charge-button {
  margin-top: 48rpx;
}
</style>
