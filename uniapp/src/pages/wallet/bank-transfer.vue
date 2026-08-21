<template>
  <view class="safe-page bank-page">
    <view class="status-card" :class="stage">
      <text class="status-title">{{ stageTitle }}</text>
      <text class="status-description">{{ stageDescription }}</text>
      <text v-if="countdownText" class="countdown">{{ countdownText }}</text>
    </view>

    <view v-if="account" class="payment-card">
      <view class="amount-block">
        <text>{{ t("commerce.bankTransfer.transferAmount") }}</text>
        <text>¥{{ order?.money || order?.amount || "0" }}</text>
        <text>{{ t("commerce.bankTransfer.exactAmountHint") }}</text>
      </view>
      <view class="info-row">
        <text>{{ t("commerce.bankTransfer.bank") }}</text>
        <view><text>{{ account.bank_name || "-" }}</text><button @tap="copy(account.bank_name)">{{ t("commerce.common.copy") }}</button></view>
      </view>
      <view v-if="account.branch_name" class="info-row">
        <text>{{ t("commerce.bankTransfer.branch") }}</text>
        <view><text>{{ account.branch_name }}</text><button @tap="copy(account.branch_name)">{{ t("commerce.common.copy") }}</button></view>
      </view>
      <view class="info-row">
        <text>{{ t("commerce.bankTransfer.recipient") }}</text>
        <view><text>{{ account.holder_name || "-" }}</text><button @tap="copy(account.holder_name)">{{ t("commerce.common.copy") }}</button></view>
      </view>
      <view class="info-row card-number-row">
        <text>{{ t("commerce.bankTransfer.cardNumber") }}</text>
        <view><text>{{ formattedCardNumber }}</text><button @tap="copy(account.card_number)">{{ t("commerce.common.copy") }}</button></view>
      </view>
      <view class="info-row">
        <text>{{ t("commerce.common.orderNumber") }}</text>
        <view><text class="order-number">{{ orderNo }}</text><button @tap="copy(orderNo)">{{ t("commerce.common.copy") }}</button></view>
      </view>
      <view v-if="account.instructions" class="instructions">
        <text>{{ t("commerce.bankTransfer.paymentInstructions") }}</text>
        <text>{{ account.instructions }}</text>
      </view>
    </view>

    <view v-if="stage === 'awaiting_payment'" class="proof-card">
      <text class="section-title">{{ t("commerce.bankTransfer.uploadProof") }}</text>
      <text class="section-tip">{{ t("commerce.bankTransfer.uploadProofHint") }}</text>
      <view v-if="proofPath" class="proof-preview" @tap="chooseProof">
        <image :src="proofPath" mode="aspectFit" />
        <text>{{ t("commerce.bankTransfer.chooseAgain") }}</text>
      </view>
      <button v-else class="choose-button" @tap="chooseProof">{{ t("commerce.bankTransfer.chooseScreenshot") }}</button>
      <button class="submit-button" :disabled="!proofPath || submitting" @tap="submitProof">
        {{ submitting ? t("commerce.common.submitting") : t("commerce.bankTransfer.submitProof") }}
      </button>
    </view>

    <view v-if="stage === 'closed' && closeReason" class="reason-card">
      <text>{{ t("commerce.bankTransfer.closeReason") }}</text>
      <text>{{ closeReason }}</text>
    </view>

    <button class="refresh-button" :disabled="loading" @tap="load(false)">
      {{ loading ? t("commerce.common.refreshing") : t("commerce.common.refreshStatus") }}
    </button>
    <button class="detail-button" @tap="openDetails">{{ t("commerce.bankTransfer.viewRechargeDetails") }}</button>
  </view>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, ref } from "vue";
import { onLoad, onPullDownRefresh, onShow } from "@dcloudio/uni-app";
import { getRechargeOrderStatus, getWalletBalance, submitBankPaymentProof } from "@/api/services";
import type { RechargeOrder } from "@/types/api";
import { clearPendingPayment, expirationTimestamp, savePendingPayment } from "@/utils/payment";
import { getSession, requireLogin } from "@/utils/session";
import { useI18n } from "@/i18n";

const { t } = useI18n();

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
  waiting_assignment: t("commerce.bankTransfer.stage.waitingAssignment"),
  awaiting_payment: t("commerce.bankTransfer.stage.awaitingPayment"),
  review_pending: t("commerce.bankTransfer.stage.reviewPending"),
  paid: t("commerce.bankTransfer.stage.paid"),
  closed: t("commerce.bankTransfer.stage.closed")
} as Record<string, string>)[stage.value] || t("commerce.bankTransfer.stage.default"));
const stageDescription = computed(() => ({
  waiting_assignment: t("commerce.bankTransfer.description.waitingAssignment"),
  awaiting_payment: t("commerce.bankTransfer.description.awaitingPayment"),
  review_pending: t("commerce.bankTransfer.description.reviewPending"),
  paid: t("commerce.bankTransfer.description.paid"),
  closed: t("commerce.bankTransfer.description.closed")
} as Record<string, string>)[stage.value] || t("commerce.bankTransfer.description.default"));

async function load(silent = true) {
  if (!requireLogin() || !orderNo.value || loading.value) return;
  loading.value = true;
  try {
    const result = await getRechargeOrderStatus(orderNo.value);
    if (!result || String(result.channel || "") !== "bank") throw new Error(t("commerce.bankTransfer.orderMissing"));
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
    if (!silent) uni.showToast({ title: error?.message || t("commerce.common.orderRefreshFailed"), icon: "none" });
  } finally {
    loading.value = false;
    uni.stopPullDownRefresh();
  }
}

function copy(value?: string) {
  if (!value) return;
  uni.setClipboardData({ data: value, success: () => uni.showToast({ title: t("commerce.common.copied"), icon: "none" }) });
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
    uni.showToast({ title: t("commerce.bankTransfer.proofSubmitted"), icon: "success" });
  } catch (error: any) {
    uni.showToast({ title: error?.message || t("commerce.bankTransfer.proofSubmitFailed"), icon: "none" });
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
  uni.setNavigationBarTitle({ title: t("commerce.bankTransfer.navigationTitle") });
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
