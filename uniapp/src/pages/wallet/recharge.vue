<template>
  <view class="safe-page recharge-page">
    <view class="balance-card">
      <view>
        <text class="label">我的星币</text>
        <text class="coin">{{ wallet?.coin || "0" }}</text>
      </view>
      <view class="balance-side">
        <text>积分 {{ wallet?.score || "0" }}</text>
        <button class="function-button" :disabled="loading || checkingPending" @tap="refreshAll">刷新</button>
      </view>
    </view>

    <view class="section-head">
      <text class="section-title">充值金额</text>
      <button class="function-button" @tap="openChargeDetail">充值明细</button>
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
      <button class="agreement-button" @tap="openAgreement">充值协议</button>
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
          <text class="pay-name">{{ pay.name || payName(pay.id) }}</text>
          <text class="pay-description">{{ payDescription(pay) }}</text>
          <text v-if="!paymentAvailable(pay)" class="pay-preview-tag">
            {{ pay.status_text || "配置中" }}
          </text>
        </view>
        <view class="radio" />
      </view>
    </view>
    <view v-else class="pay-empty">当前没有可用支付方式</view>

    <button class="primary-button charge-button" :disabled="!canTapCharge" @tap="charge">
      {{ chargeButtonText }}
    </button>

    <view v-if="pending" class="pending-card">
      <view class="pending-copy">
        <text class="pending-title">待处理充值订单</text>
        <text class="pending-number">{{ pending.orderNo }}</text>
      </view>
      <view class="pending-actions">
        <button
          v-if="pending.paymentUrl"
          class="pending-continue-button"
          :disabled="checkingPending"
          @tap="resumePendingPayment"
        >
          继续支付
        </button>
        <button
          class="pending-refresh-button"
          :disabled="checkingPending"
          @tap="checkPendingOrder(false)"
        >
          {{ checkingPending ? "查询中" : "刷新状态" }}
        </button>
      </view>
    </view>
  </view>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, ref } from "vue";
import { onPullDownRefresh, onShow } from "@dcloudio/uni-app";
import EmptyState from "@/components/EmptyState.vue";
import {
  createCoinOrder,
  createPaymentTraceId,
  getRechargeOrderStatus,
  getWalletBalance
} from "@/api/services";
import type { RechargeOrder, WalletBalance, WalletPayMethod, WalletRule } from "@/types/api";
import { absolutizeUrl, firstText } from "@/utils/url";
import { getSession, onSessionChange, requireLogin } from "@/utils/session";
import {
  clearForeignPendingPayments,
  clearPaymentCreateAttempt,
  clearPendingPayment,
  normalizePaymentUrl,
  openPaymentCashier,
  readPaymentCreateAttempt,
  readPendingPayment,
  rechargeIsExpired,
  rechargeIsPaid,
  rechargeIsTerminal,
  rechargePaymentUrl,
  rechargeStatusText,
  savePaymentCreateAttempt,
  savePendingPayment,
  type PendingPayment
} from "@/utils/payment";

const wallet = ref<WalletBalance>();
const selectedRuleIndex = ref(0);
const selectedPayIndex = ref(0);
const loading = ref(false);
const submitting = ref(false);
const checkingPending = ref(false);
const pending = ref<PendingPayment>();
const walletUID = ref("");
let pendingRequestSequence = 0;
let walletRequestSequence = 0;

const rules = computed(() => parseList<WalletRule>(wallet.value?.rules));
const payMethods = computed(() => parseList<WalletPayMethod>(wallet.value?.paylist));
const selectedRule = computed(() => rules.value[selectedRuleIndex.value]);
const selectedPay = computed(() => payMethods.value[selectedPayIndex.value]);
const canTapCharge = computed(
  () =>
    !loading.value &&
    !submitting.value &&
    !pending.value &&
    walletUID.value === currentUID() &&
    Boolean(selectedRule.value) &&
    Boolean(selectedPay.value)
);
const chargeButtonText = computed(() => {
  if (pending.value) {
    return "请先完成待支付订单";
  }
  if (selectedPay.value && !paymentAvailable(selectedPay.value)) {
    return "支付通道配置中 · 点击查看";
  }
  if (selectedRule.value && !ruleAvailable(selectedRule.value)) {
    return "充值档位预览 · 点击查看";
  }
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

function flagIsTrue(value: unknown) {
  if (value === true || value === 1) {
    return true;
  }
  return ["1", "true", "yes", "enabled", "available"].includes(
    String(value ?? "").trim().toLowerCase()
  );
}

function statusAllowsUse(value: unknown) {
  if (value === undefined || value === null || value === "") {
    return true;
  }
  return flagIsTrue(value);
}

function paymentAvailable(pay?: WalletPayMethod) {
  if (!pay) {
    return false;
  }
  const id = String(pay.id || "").trim().toLowerCase();
  if (
    !["ali", "wx", "paypal", "usdt"].includes(id) ||
    id.startsWith("preview-") ||
    flagIsTrue(pay.preview) ||
    !statusAllowsUse(pay.status)
  ) {
    return false;
  }
  if (pay.available === undefined || pay.available === null || pay.available === "") {
    return true;
  }
  return flagIsTrue(pay.available);
}

function ruleAvailable(rule?: WalletRule) {
  if (!rule) {
    return false;
  }
  const id = String(rule.id || "").trim();
  if (
    flagIsTrue(rule.preview) ||
    id.toLowerCase().startsWith("preview-") ||
    !/^[1-9]\d*$/.test(id) ||
    !statusAllowsUse(rule.status)
  ) {
    return false;
  }
  if (rule.available === undefined || rule.available === null || rule.available === "") {
    return true;
  }
  return flagIsTrue(rule.available);
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

function payDescription(pay: WalletPayMethod) {
  const custom = firstText(pay.description);
  const id = String(pay.id || "").toLowerCase();
  const provider = String(pay.provider || "").toLowerCase();
  const mode = String(pay.mode || "").toLowerCase();
  const tradeType = String(pay.trade_type || "").toLowerCase();
  const network = String(pay.network || "").toLowerCase();
  const descriptor = `${provider} ${mode} ${custom}`.toLowerCase();
  if (
    id === "usdt" ||
    descriptor.includes("bepusdt") ||
    descriptor.includes("usdt") ||
    tradeType.startsWith("usdt.")
  ) {
    const networkLabel =
      tradeType.includes("trc20") || network === "tron"
        ? "TRC20"
        : tradeType.includes("erc20") || network === "ethereum"
          ? "ERC20"
          : tradeType.includes("bep20") || network === "bsc"
            ? "BEP20"
            : String(pay.network || "TRC20").toUpperCase();
    return `USDT · ${networkLabel} 链上收银台`;
  }
  if (custom) {
    return custom;
  }
  if (["cashier", "web", "h5", "external"].includes(mode)) {
    return "安全外部收银台";
  }
  return provider ? `${String(pay.provider)} 在线收银台` : "安全在线支付";
}

function coinOf(rule: WalletRule) {
  if (String(selectedPay.value?.id || "") === "paypal") {
    return firstText(rule.coin_paypal, rule.coin, "0");
  }
  return firstText(rule.coin, "0");
}

function payIcon(pay: WalletPayMethod) {
  const remote = absolutizeUrl(String(pay.thumb || ""));
  if (remote) {
    return remote;
  }
  const icons: Record<string, string> = {
    ali: "/static/icons/payment-alipay.svg",
    wx: "/static/icons/payment-wechat.svg",
    paypal: "/static/icons/payment-paypal.svg",
    usdt: "/static/icons/payment-usdt.svg"
  };
  return icons[String(pay.id || "").toLowerCase()] || "/static/native/me_wallet.png";
}

function currentUID() {
  return String(getSession().uid || "");
}

function openPaymentUrl(url: string, title = "支付") {
  if (openPaymentCashier(url, title)) {
    return true;
  }
  uni.showModal({
    title: "支付链接无效",
    content: "支付通道没有返回本站安全收银台地址，请稍后重试。",
    showCancel: false,
    confirmColor: "#ff5878"
  });
  return false;
}

async function load() {
  if (!requireLogin()) {
    return;
  }
  const uid = currentUID();
  const requestSequence = ++walletRequestSequence;
  if (walletUID.value !== uid) {
    wallet.value = undefined;
    walletUID.value = "";
  }
  loading.value = true;
  try {
    const nextWallet = await getWalletBalance();
    if (currentUID() !== uid || requestSequence !== walletRequestSequence) {
      return;
    }
    wallet.value = nextWallet;
    walletUID.value = uid;
    selectedRuleIndex.value = Math.min(
      selectedRuleIndex.value,
      Math.max(0, parseList(nextWallet?.rules).length - 1)
    );
    selectedPayIndex.value = Math.min(
      selectedPayIndex.value,
      Math.max(0, parseList(nextWallet?.paylist).length - 1)
    );
  } catch (error: any) {
    if (currentUID() === uid && requestSequence === walletRequestSequence) {
      uni.showToast({ title: error?.message || "充值配置加载失败", icon: "none" });
    }
  } finally {
    if (requestSequence === walletRequestSequence) {
      loading.value = false;
    }
  }
}

function mergedPendingOrder(order: RechargeOrder, stored: PendingPayment): RechargeOrder {
  return {
    ...order,
    order_no: firstText(order.order_no, order.orderid, stored.orderNo),
    payment_url: firstText(rechargePaymentUrl(order), stored.paymentUrl),
    provider_trade_id: firstText(
      order.provider_trade_id,
      order.provider_order_no,
      stored.providerTradeId
    ),
    status: firstText(order.status, stored.status, "0"),
    expires_at: firstText(order.expires_at, stored.expiresAt)
  };
}

async function checkPendingOrder(silent = true) {
  const uid = currentUID();
  const requestSequence = ++pendingRequestSequence;
  clearForeignPendingPayments(uid);
  const stored = readPendingPayment(uid);
  pending.value = stored;
  if (!stored) {
    checkingPending.value = false;
    return;
  }
  checkingPending.value = true;
  try {
    const result = await getRechargeOrderStatus(stored.orderNo);
    if (currentUID() !== uid || requestSequence !== pendingRequestSequence) {
      return;
    }
    if (!result) {
      throw new Error("充值订单不存在");
    }
    const order = mergedPendingOrder(result, stored);
    if (rechargeIsPaid(order)) {
      clearPendingPayment(uid);
      pending.value = undefined;
      await load();
      uni.showToast({ title: "充值已到账", icon: "success" });
      return;
    }
    if (rechargeIsTerminal(order) || rechargeIsExpired(order)) {
      clearPendingPayment(uid);
      pending.value = undefined;
      if (!silent) {
        uni.showToast({
          title: rechargeIsExpired(order) ? "充值订单已过期" : rechargeStatusText(order),
          icon: "none"
        });
      }
      return;
    }
    pending.value = savePendingPayment(uid, order);
    if (!silent) {
      uni.showToast({ title: rechargeStatusText(order), icon: "none" });
    }
  } catch (error: any) {
    if (!silent && currentUID() === uid && requestSequence === pendingRequestSequence) {
      uni.showToast({ title: error?.message || "支付状态查询失败", icon: "none" });
    }
  } finally {
    if (requestSequence === pendingRequestSequence) {
      checkingPending.value = false;
    }
  }
}

async function charge() {
  if (!requireLogin()) {
    return;
  }
  const rule = selectedRule.value;
  const pay = selectedPay.value;
  if (!canTapCharge.value || !rule || !pay) {
    return;
  }
  // Preview and malformed catalog entries must stop before trace lookup,
  // local storage writes, or any payment-order request.
  if (!paymentAvailable(pay) || !ruleAvailable(rule)) {
    uni.showModal({
      title: "效果预览",
      content:
        "当前支付方式和充值档位已展示。配置收款钱包与 API Token 后即可真实下单；目前不会生成充值订单。",
      showCancel: false,
      confirmColor: "#ff5878"
    });
    return;
  }
  const uid = currentUID();
  const productId = String(rule.id || "");
  const payId = String(pay.id || "");
  const previousAttempt = readPaymentCreateAttempt(uid);
  const clientTraceId =
    previousAttempt?.productId === productId && previousAttempt.payId === payId
      ? previousAttempt.traceId
      : createPaymentTraceId();
  savePaymentCreateAttempt({
    uid,
    productId,
    payId,
    traceId: clientTraceId,
    createdAt: previousAttempt?.traceId === clientTraceId
      ? previousAttempt.createdAt
      : Date.now()
  });
  submitting.value = true;
  try {
    const result = await createCoinOrder(
      rule,
      pay,
      clientTraceId
    );
    if (currentUID() !== uid) {
      throw new Error("账号已切换，请在当前账号重新发起充值");
    }
    if (!result) {
      throw new Error("支付通道未返回充值订单");
    }
    const order: RechargeOrder = {
      ...result,
      client_trace_id: firstText(result.client_trace_id, clientTraceId)
    };
    const saved = savePendingPayment(uid, order);
    if (!saved) {
      throw new Error("支付通道返回的订单号无效");
    }
    clearPaymentCreateAttempt(uid, clientTraceId);
    pending.value = saved;
    if (rechargeIsPaid(order)) {
      clearPendingPayment(uid);
      pending.value = undefined;
      await load();
      uni.showToast({ title: "充值已到账", icon: "success" });
      return;
    }
    if (rechargeIsTerminal(order) || rechargeIsExpired(order)) {
      clearPendingPayment(uid);
      pending.value = undefined;
      throw new Error(
        rechargeIsExpired(order) ? "充值订单已过期，请重新发起" : rechargeStatusText(order)
      );
    }
    const paymentUrl = normalizePaymentUrl(rechargePaymentUrl(order));
    if (paymentUrl && openPaymentUrl(paymentUrl, pay.name || "支付")) {
      return;
    }
    uni.showModal({
      title: "支付暂不可用",
      content: "订单已经创建，但支付通道未返回可用收银台。可在充值明细中刷新或继续支付。",
      showCancel: false,
      confirmColor: "#ff5878"
    });
  } catch (error: any) {
    uni.showToast({ title: error?.message || "创建订单失败", icon: "none" });
  } finally {
    submitting.value = false;
  }
}

function resumePendingPayment() {
  const uid = currentUID();
  const stored = pending.value;
  if (!stored || stored.uid !== uid || !openPaymentUrl(stored.paymentUrl, "支付")) {
    void checkPendingOrder(false);
  }
}

async function refreshAll() {
  await Promise.all([load(), checkPendingOrder(false)]);
  uni.stopPullDownRefresh();
}

function openAgreement() {
  uni.navigateTo({ url: "/pages/detail/index?type=recharge_agreement" });
}

function openChargeDetail() {
  uni.navigateTo({ url: "/pages/wallet/detail?type=charge" });
}

function resetAccountState() {
  walletRequestSequence += 1;
  pendingRequestSequence += 1;
  wallet.value = undefined;
  walletUID.value = "";
  pending.value = undefined;
  loading.value = false;
  checkingPending.value = false;
  selectedRuleIndex.value = 0;
  selectedPayIndex.value = 0;
}

const stopSessionChange = onSessionChange(() => {
  // Clear account-bound data immediately, including while this page is hidden.
  resetAccountState();
});

onBeforeUnmount(() => {
  stopSessionChange();
});

onShow(() => {
  const uid = currentUID();
  if (walletUID.value !== uid) {
    resetAccountState();
  }
  if (!requireLogin()) {
    return;
  }
  clearForeignPendingPayments(uid);
  pending.value = readPendingPayment(uid);
  void Promise.all([load(), checkPendingOrder(true)]);
});

onPullDownRefresh(() => {
  void refreshAll();
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
  display: flex !important;
  min-width: 116rpx;
  height: 52rpx;
  align-items: center !important;
  justify-content: center !important;
  border-radius: 26rpx;
  color: var(--brand);
  font-size: 23rpx;
  font-weight: 800;
  line-height: 1 !important;
  text-align: center !important;
  background: #fff;
}

.section-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin: 38rpx 2rpx 20rpx;
}
.section-title,
.section-head-start {
  display: inline-flex !important;
  align-items: center !important;
  justify-content: center !important;
  font-size: 26rpx;
  font-weight: bold;
  line-height: 1.2;
  text-align: center !important;
}

.agreement-button {
  display: inline-flex !important;
  min-width: 126rpx !important;
  align-items: center !important;
  justify-content: center !important;
  border-color: #d8eee6 !important;
  color: #0a8f66 !important;
  text-align: center !important;
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
  display: flex !important;
  flex-direction: column !important;
  min-height: 148rpx;
  align-items: center !important;
  justify-content: center !important;
  padding: 24rpx 12rpx;
  border: 2rpx solid #edf0f5;
  border-radius: 18rpx;
  text-align: center !important;
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
  text-align: center !important;
}

.rule-money {
  display: block;
  margin-top: 18rpx;
  color: var(--brand);
  font-size: 25rpx;
  font-weight: 900;
  text-align: center !important;
}

.give {
  position: absolute;
  top: -1rpx;
  right: -1rpx;
  display: inline-flex !important;
  min-width: 72rpx;
  height: 34rpx;
  padding: 0 10rpx;
  align-items: center !important;
  justify-content: center !important;
  border-radius: 0 16rpx 0 16rpx;
  color: #fff;
  font-size: 19rpx;
  line-height: 1 !important;
  text-align: center !important;
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
  position: relative;
  display: flex !important;
  align-items: center !important;
  justify-content: center !important;
  min-height: 112rpx;
  padding: 20rpx 24rpx;
  border-bottom: 1rpx solid #f0f2f6;
  text-align: center !important;
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
  display: flex !important;
  flex: 0 1 auto;
  flex-direction: column !important;
  min-width: 0;
  align-items: center !important;
  justify-content: center !important;
  text-align: center !important;
}

.pay-name {
  display: flex !important;
  align-items: center !important;
  justify-content: center !important;
  color: var(--ink);
  font-size: 28rpx;
  font-weight: 900;
  line-height: 1.2 !important;
  text-align: center !important;
}

.pay-description {
  display: flex !important;
  align-items: center !important;
  justify-content: center !important;
  margin-top: 10rpx;
  color: var(--ink-3);
  font-size: 23rpx;
  line-height: 1.2 !important;
  text-align: center !important;
}

.pay-preview-tag {
  display: inline-flex !important;
  min-width: 88rpx;
  height: 36rpx;
  margin-top: 12rpx;
  padding: 0 12rpx;
  align-items: center !important;
  justify-content: center !important;
  border-radius: 18rpx;
  color: #9a6700;
  font-size: 20rpx;
  font-weight: 800;
  line-height: 1 !important;
  text-align: center !important;
  background: #fff4ce;
}

.radio {
  position: absolute;
  right: 24rpx;
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

.pending-card {
  display: flex;
  min-height: 112rpx;
  align-items: center;
  justify-content: space-between;
  gap: 20rpx;
  padding: 22rpx 24rpx;
  margin-top: 20rpx;
  border: 1rpx solid #e9edf4;
  border-radius: 18rpx;
  background: #fff;
}

.pending-copy {
  min-width: 0;
}

.pending-title,
.pending-number {
  display: block;
}

.pending-title {
  color: var(--ink);
  font-size: 25rpx;
  font-weight: 900;
}

.pending-number {
  margin-top: 9rpx;
  overflow: hidden;
  color: var(--ink-3);
  font-size: 21rpx;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.pending-actions {
  display: flex;
  flex: 0 0 auto;
  align-items: center;
  justify-content: center;
  gap: 12rpx;
}

.pending-continue-button,
.pending-refresh-button {
  display: inline-flex !important;
  flex: 0 0 auto;
  min-width: 126rpx;
  height: 58rpx;
  padding: 0 16rpx;
  align-items: center !important;
  justify-content: center !important;
  border: 1rpx solid #f1d7de;
  border-radius: 29rpx;
  color: var(--brand);
  font-size: 23rpx;
  font-weight: 800;
  line-height: 1 !important;
  text-align: center !important;
  background: #fff5f7;
}

.pending-continue-button {
  border-color: #ccecdf;
  color: #087b59;
  background: #effbf7;
}

.pending-continue-button[disabled],
.pending-refresh-button[disabled] {
  opacity: 0.5;
}
</style>
