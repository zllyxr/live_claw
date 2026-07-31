<template>
  <view class="safe-page bank-page">
    <view class="status-card" :class="stage">
      <text class="status-title">{{ stageTitle }}</text>
      <text class="status-description">{{ stageDescription }}</text>
      <text v-if="countdownText" class="countdown">{{ countdownText }}</text>
    </view>

    <view v-if="account" class="payment-card">
      <view class="amount-block">
        <text>本次转账金额</text>
        <text>¥{{ order?.money || order?.amount || "0" }}</text>
        <text>请按显示金额准确转账</text>
      </view>
      <view class="info-row">
        <text>银行</text>
        <view><text>{{ account.bank_name || "-" }}</text><button @tap="copy(account.bank_name)">复制</button></view>
      </view>
      <view v-if="account.branch_name" class="info-row">
        <text>开户支行</text>
        <view><text>{{ account.branch_name }}</text><button @tap="copy(account.branch_name)">复制</button></view>
      </view>
      <view class="info-row">
        <text>收款人</text>
        <view><text>{{ account.holder_name || "-" }}</text><button @tap="copy(account.holder_name)">复制</button></view>
      </view>
      <view class="info-row card-number-row">
        <text>银行卡号</text>
        <view><text>{{ formattedCardNumber }}</text><button @tap="copy(account.card_number)">复制</button></view>
      </view>
      <view class="info-row">
        <text>订单号</text>
        <view><text class="order-number">{{ orderNo }}</text><button @tap="copy(orderNo)">复制</button></view>
      </view>
      <view v-if="account.instructions" class="instructions">
        <text>付款说明</text>
        <text>{{ account.instructions }}</text>
      </view>
    </view>

    <view v-if="stage === 'awaiting_payment'" class="proof-card">
      <text class="section-title">上传付款凭证</text>
      <text class="section-tip">完成转账后上传一张清晰截图，支持 JPEG、PNG、WebP，最大10MB。</text>
      <view v-if="proofPath" class="proof-preview" @tap="chooseProof">
        <image :src="proofPath" mode="aspectFit" />
        <text>点击重新选择</text>
      </view>
      <button v-else class="choose-button" @tap="chooseProof">选择转账截图</button>
      <button class="submit-button" :disabled="!proofPath || submitting" @tap="submitProof">
        {{ submitting ? "正在提交" : "我已转账，提交凭证" }}
      </button>
    </view>

    <view v-if="stage === 'closed' && closeReason" class="reason-card">
      <text>关闭原因</text>
      <text>{{ closeReason }}</text>
    </view>

    <button class="refresh-button" :disabled="loading" @tap="load(false)">
      {{ loading ? "刷新中" : "刷新订单状态" }}
    </button>
    <button class="detail-button" @tap="openDetails">查看充值明细</button>
  </view>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, ref } from "vue";
import { onLoad, onPullDownRefresh, onShow } from "@dcloudio/uni-app";
import { getRechargeOrderStatus, getWalletBalance, submitBankPaymentProof } from "@/api/services";
import type { RechargeOrder } from "@/types/api";
import { clearPendingPayment, expirationTimestamp, savePendingPayment } from "@/utils/payment";
import { getSession, requireLogin } from "@/utils/session";

const orderNo = ref("");
const order = ref<RechargeOrder>();
const loading = ref(false);
const submitting = ref(false);
const proofPath = ref("");
const now = ref(Date.now());
let pollTimer: ReturnType<typeof setInterval> | undefined;
let clockTimer: ReturnType<typeof setInterval> | undefined;

const stage = computed(() => String(order.value?.bank_stage || "waiting_assignment"));
const account = computed(() => order.value?.bank_account);
const closeReason = computed(() => String(
  order.value?.proof_review_reason || order.value?.close_reason || order.value?.failure_reason || ""
));
const formattedCardNumber = computed(() =>
  String(account.value?.card_number || "").replace(/(.{4})/g, "$1 ").trim()
);
const remainingSeconds = computed(() => {
  const expiresAt = expirationTimestamp(order.value?.expires_at);
  return expiresAt > 0 ? Math.max(0, expiresAt - Math.floor(now.value / 1000)) : 0;
});
const countdownText = computed(() => {
  if (!["waiting_assignment", "awaiting_payment"].includes(stage.value)) return "";
  const seconds = remainingSeconds.value;
  return `${Math.floor(seconds / 60).toString().padStart(2, "0")}:${(seconds % 60).toString().padStart(2, "0")}`;
});
const stageTitle = computed(() => ({
  waiting_assignment: "等待后台分配收款卡",
  awaiting_payment: "请转账到以下银行卡",
  review_pending: "付款凭证审核中",
  paid: "充值已到账",
  closed: "订单已关闭"
} as Record<string, string>)[stage.value] || "银行卡充值");
const stageDescription = computed(() => ({
  waiting_assignment: "页面会自动刷新，10分钟内未分配将自动关闭。",
  awaiting_payment: "银行卡分配后不可更换，请在30分钟内付款并提交截图。",
  review_pending: "凭证已提交，后台确认到账后星币会自动增加。",
  paid: "后台已确认收款，星币已经加入钱包。",
  closed: "该订单不能继续付款，请返回充值页重新下单。"
} as Record<string, string>)[stage.value] || "请刷新订单状态。");

async function load(silent = true) {
  if (!requireLogin() || !orderNo.value || loading.value) return;
  loading.value = true;
  try {
    const result = await getRechargeOrderStatus(orderNo.value);
    if (!result || String(result.channel || "") !== "bank") throw new Error("银行卡充值订单不存在");
    order.value = result;
    const uid = String(getSession().uid || "");
    if (["paid", "closed"].includes(String(result.bank_stage || ""))) {
      clearPendingPayment(uid);
      if (result.bank_stage === "paid") {
        void getWalletBalance();
      }
    } else {
      savePendingPayment(uid, result);
    }
  } catch (error: any) {
    if (!silent) uni.showToast({ title: error?.message || "订单刷新失败", icon: "none" });
  } finally {
    loading.value = false;
    uni.stopPullDownRefresh();
  }
}

function copy(value?: string) {
  if (!value) return;
  uni.setClipboardData({ data: value, success: () => uni.showToast({ title: "已复制", icon: "none" }) });
}

function chooseProof() {
  uni.chooseImage({
    count: 1,
    sizeType: ["compressed", "original"],
    sourceType: ["album", "camera"],
    success: (result) => { proofPath.value = result.tempFilePaths[0] || ""; }
  });
}

async function submitProof() {
  if (!proofPath.value || submitting.value) return;
  submitting.value = true;
  try {
    await submitBankPaymentProof(orderNo.value, proofPath.value);
    proofPath.value = "";
    await load(true);
    uni.showToast({ title: "凭证已提交", icon: "success" });
  } catch (error: any) {
    uni.showToast({ title: error?.message || "凭证提交失败", icon: "none" });
  } finally {
    submitting.value = false;
  }
}

function openDetails() {
  uni.navigateTo({ url: "/pages/wallet/detail?type=charge" });
}

function startTimers() {
  if (!clockTimer) clockTimer = setInterval(() => { now.value = Date.now(); }, 1000);
  if (!pollTimer) pollTimer = setInterval(() => {
    if (["waiting_assignment", "review_pending"].includes(stage.value)) void load(true);
  }, 3000);
}

onLoad((options) => {
  orderNo.value = decodeURIComponent(String(options?.order_no || "")).trim();
});
onShow(() => { startTimers(); void load(false); });
onPullDownRefresh(() => { void load(false); });
onBeforeUnmount(() => {
  if (pollTimer) clearInterval(pollTimer);
  if (clockTimer) clearInterval(clockTimer);
});
</script>

<style scoped>
.bank-page { padding: 24rpx; background: #f5f7fb; min-height: 100vh; }
.status-card, .payment-card, .proof-card, .reason-card { background: #fff; border-radius: 24rpx; padding: 30rpx; margin-bottom: 24rpx; box-shadow: 0 8rpx 28rpx rgba(29, 54, 92, .08); }
.status-card { display: flex; flex-direction: column; align-items: center; text-align: center; }
.status-card.paid { background: #effcf4; }.status-card.closed { background: #fff3f3; }
.status-title { font-size: 36rpx; font-weight: 700; color: #17233d; }
.status-description { margin-top: 14rpx; color: #6c778b; font-size: 26rpx; line-height: 1.6; }
.countdown { margin-top: 20rpx; font-size: 44rpx; color: #ff5b64; font-weight: 700; letter-spacing: 4rpx; }
.amount-block { display: flex; flex-direction: column; align-items: center; padding-bottom: 28rpx; border-bottom: 1rpx solid #edf0f5; }
.amount-block text:nth-child(1), .amount-block text:nth-child(3) { color: #7a8497; font-size: 24rpx; }
.amount-block text:nth-child(2) { margin: 12rpx 0; font-size: 56rpx; font-weight: 800; color: #ff5068; }
.info-row { display: flex; justify-content: space-between; gap: 24rpx; padding: 24rpx 0; border-bottom: 1rpx solid #edf0f5; color: #788296; }
.info-row > view { flex: 1; display: flex; justify-content: flex-end; align-items: center; gap: 16rpx; color: #17233d; text-align: right; }
.info-row button { margin: 0; padding: 0 18rpx; height: 52rpx; line-height: 52rpx; font-size: 22rpx; color: #3478f6; background: #edf4ff; }
.card-number-row text:last-of-type { font-size: 30rpx; font-weight: 700; letter-spacing: 1rpx; }
.order-number { word-break: break-all; font-size: 22rpx; }.instructions { padding-top: 24rpx; display: flex; flex-direction: column; gap: 10rpx; color: #7a8497; }.instructions text:last-child { color: #303a4d; line-height: 1.6; }
.section-title { font-size: 32rpx; font-weight: 700; color: #17233d; }.section-tip { display: block; margin: 12rpx 0 24rpx; color: #7a8497; line-height: 1.6; }
.proof-preview { height: 380rpx; background: #f2f5fa; border-radius: 18rpx; overflow: hidden; text-align: center; }.proof-preview image { width: 100%; height: 320rpx; }.proof-preview text { font-size: 24rpx; color: #3478f6; }
.choose-button, .submit-button, .refresh-button, .detail-button { border-radius: 44rpx; font-size: 28rpx; }
.choose-button { color: #3478f6; background: #edf4ff; }.submit-button { margin-top: 22rpx; color: #fff; background: linear-gradient(135deg, #ff6b81, #ff405f); }
.submit-button[disabled] { opacity: .45; }.refresh-button { color: #fff; background: #3478f6; }.detail-button { margin-top: 18rpx; color: #59657a; background: #e9edf4; }
.reason-card { display: flex; flex-direction: column; gap: 12rpx; color: #b83945; }.reason-card text:first-child { font-weight: 700; }
</style>
