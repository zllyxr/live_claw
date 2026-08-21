<template>
  <view class="safe-page detail-hub">
    <view v-if="contentMode" class="content-page">
      <text class="content-title">{{ contentPage?.title || (loading ? t("misc.common.loading") : t("misc.detail.pageContent")) }}</text>
      <text class="content-body">{{ contentPage?.content || "" }}</text>
    </view>

    <view v-else-if="authMode" class="auth-status-card">
      <view class="auth-icon">{{ authIcon }}</view>
      <text class="auth-title">{{ textOf(authStatus, "status_text", loading ? t("misc.detail.reading") : t("misc.detail.notVerified")) }}</text>
      <text class="auth-desc">{{ authDescription }}</text>
      <text v-if="textOf(authStatus, 'reject_reason')" class="reject-reason">
        {{ textOf(authStatus, "reject_reason") }}
      </text>
      <button v-if="String(authStatus?.verified || '0') !== '1'" class="primary-button" @tap="openVerify">
        {{ String(authStatus?.status || "-1") === "0" ? t("misc.detail.viewVerification") : t("misc.detail.submitVerification") }}
      </button>
    </view>

    <template v-else>
    <view class="hub-head">
      <text>{{ t("misc.detail.center") }}</text>
      <text>{{ t("misc.detail.centerDescription") }}</text>
    </view>
    <view class="hub-list">
      <view v-for="item in hubItems" :key="item.type" class="hub-row" @tap="openType(item.type)">
        <view class="hub-icon">{{ item.icon }}</view>
        <view class="hub-main">
          <text>{{ item.title }}</text>
          <text>{{ item.desc }}</text>
        </view>
        <text class="arrow">›</text>
      </view>
    </view>
    </template>
  </view>
</template>

<script setup lang="ts">
import { computed, ref } from "vue";
import { onLoad } from "@dcloudio/uni-app";
import { getContentPage, getVerificationStatus } from "@/api/services";
import { t } from "@/i18n";

const authMode = ref(false);
const contentMode = ref(false);
const loading = ref(false);
const authStatus = ref<Record<string, unknown>>();
const contentPage = ref<{ title: string; content: string }>();

const hubItems = computed(() => [
  { type: "detail", icon: "≡", title: t("misc.detail.myDetails"), desc: t("misc.detail.myDetailsDesc") },
  { type: "charge", icon: "+", title: t("misc.detail.topUpDetails"), desc: t("misc.detail.topUpDetailsDesc") },
  { type: "cash", icon: "↑", title: t("misc.detail.withdrawals"), desc: t("misc.detail.withdrawalsDesc") },
  { type: "auth", icon: "✓", title: t("misc.detail.verificationDetails"), desc: t("misc.detail.verificationDetailsDesc") }
]);

const authIcon = computed(() => {
  if (String(authStatus.value?.verified || "0") === "1") {
    return "✓";
  }
  if (String(authStatus.value?.status || "") === "2") {
    return "!";
  }
  return "✓";
});

const authDescription = computed(() => {
  const status = String(authStatus.value?.status || "-1");
  if (status === "1") {
    return t("misc.detail.verifiedDescription");
  }
  if (status === "0") {
    return t("misc.detail.pendingDescription");
  }
  if (status === "2") {
    return t("misc.detail.rejectedDescription");
  }
  return t("misc.detail.notSubmittedDescription");
});

function textOf(source: Record<string, unknown> | undefined, key: string, fallback = "") {
  const value = source?.[key];
  return value !== undefined && value !== null && String(value).trim() ? String(value) : fallback;
}

async function loadAuthStatus() {
  authMode.value = true;
  loading.value = true;
  uni.setNavigationBarTitle({ title: t("misc.detail.verificationDetails") });
  try {
    authStatus.value = await getVerificationStatus();
  } catch (error: any) {
    uni.showToast({ title: error?.message || t("misc.detail.verificationLoadFailed"), icon: "none" });
  } finally {
    loading.value = false;
  }
}

async function loadContentPage() {
  contentMode.value = true;
  loading.value = true;
  uni.setNavigationBarTitle({ title: t("misc.detail.rechargeAgreement") });
  try {
    contentPage.value = await getContentPage("recharge_agreement");
    if (contentPage.value?.title) {
      uni.setNavigationBarTitle({ title: contentPage.value.title });
    }
  } catch (error: any) {
    uni.showToast({ title: error?.message || t("misc.detail.contentLoadFailed"), icon: "none" });
  } finally {
    loading.value = false;
  }
}

function openType(type: string) {
  if (type === "recharge_agreement") {
    void loadContentPage();
    return;
  }
  if (type === "auth") {
    void loadAuthStatus();
    return;
  }
  if (type === "detail" || type === "charge" || type === "cash") {
    uni.navigateTo({ url: `/pages/wallet/detail?type=${type}` });
  }
}

function openVerify() {
  uni.navigateTo({ url: "/pages/verify/index" });
}

onLoad((query) => {
  const type = String(query?.type || "");
  if (type) {
    openType(type);
    return;
  }
  uni.setNavigationBarTitle({ title: t("misc.detail.center") });
});
</script>

<style scoped>
.detail-hub {
  background: var(--bg);
}

.hub-head {
  min-height: 170rpx;
  padding: 32rpx 30rpx;
  border-radius: 24rpx;
  color: #fff;
  background: linear-gradient(135deg, #23283d, var(--brand));
}

.hub-head text:first-child {
  display: block;
  font-size: 38rpx;
  font-weight: 900;
}

.hub-head text:last-child {
  display: block;
  margin-top: 18rpx;
  color: rgba(255, 255, 255, 0.86);
  font-size: 25rpx;
}

.hub-list {
  overflow: hidden;
  margin-top: 22rpx;
  border: 1rpx solid #e9edf4;
  border-radius: 18rpx;
  background: #fff;
}

.hub-row {
  display: grid;
  grid-template-columns: 64rpx minmax(0, 1fr) 34rpx;
  gap: 18rpx;
  align-items: center;
  min-height: 118rpx;
  padding: 20rpx 24rpx;
  border-bottom: 1rpx solid #f0f2f6;
}

.hub-row:last-child {
  border-bottom: 0;
}

.hub-icon {
  display: flex;
  width: 58rpx;
  height: 58rpx;
  align-items: center;
  justify-content: center;
  border-radius: 18rpx;
  color: var(--brand);
  font-size: 22rpx;
  font-weight: 900;
  background: #fff1f4;
}

.hub-main {
  min-width: 0;
}

.hub-main text:first-child {
  display: block;
  color: var(--ink);
  font-size: 28rpx;
  font-weight: 900;
}

.hub-main text:last-child {
  display: block;
  margin-top: 10rpx;
  color: var(--ink-3);
  font-size: 23rpx;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.arrow {
  color: #a3aab7;
  font-size: 46rpx;
  line-height: 1;
}

.auth-status-card {
  display: flex;
  min-height: 520rpx;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  padding: 48rpx 34rpx;
  border: 1rpx solid #e9edf4;
  border-radius: 24rpx;
  background: #fff;
}

.content-page {
  min-height: 420rpx;
  padding: 34rpx 30rpx;
  border: 1rpx solid #e9edf4;
  border-radius: 24rpx;
  background: #fff;
}

.content-title {
  display: block;
  color: var(--ink);
  font-size: 36rpx;
  font-weight: 900;
}

.content-body {
  display: block;
  margin-top: 26rpx;
  color: var(--ink-2);
  font-size: 26rpx;
  line-height: 1.9;
  white-space: pre-wrap;
}

.auth-icon {
  display: flex;
  width: 120rpx;
  height: 120rpx;
  align-items: center;
  justify-content: center;
  border-radius: 50%;
  color: #fff;
  font-size: 52rpx;
  font-weight: 900;
  background: linear-gradient(135deg, #31364f, var(--brand));
}

.auth-title {
  margin-top: 28rpx;
  color: var(--ink);
  font-size: 36rpx;
  font-weight: 900;
}

.auth-desc {
  max-width: 560rpx;
  margin-top: 18rpx;
  color: var(--ink-3);
  font-size: 24rpx;
  line-height: 1.6;
  text-align: center;
}

.reject-reason {
  width: 100%;
  padding: 18rpx;
  margin-top: 22rpx;
  border-radius: 14rpx;
  color: #c44b5e;
  font-size: 23rpx;
  text-align: center;
  background: #fff2f4;
}

.auth-status-card .primary-button {
  width: 100%;
  margin-top: 34rpx;
}
</style>
