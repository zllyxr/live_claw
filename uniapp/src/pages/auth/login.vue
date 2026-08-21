<template>
  <view class="auth-page">
    <view class="cosmic-bg">
      <view class="bg-stars" />
      <view class="bg-orb bg-orb-a" />
      <view class="bg-orb bg-orb-b" />
    </view>

    <view class="nav-row">
      <view class="back-button" @tap="goBack">
        <image class="back" src="/static/native/back.png" mode="aspectFit" />
      </view>
    </view>

    <view class="hero">
      <view class="brand-card">
        <image class="brand" :src="loginImage || '/static/brand/icon-round.webp'" mode="aspectFit" />
      </view>
      <text class="brand-name">{{ t("misc.common.brand") }}</text>
      <text class="title">{{ t("misc.auth.welcomeBack") }}</text>
    </view>

    <view class="form-panel">
      <view class="field-group">
        <text class="label">{{ t("misc.auth.phone") }}</text>
        <view class="phone-row">
          <view class="country" @tap="openCountry">+{{ countryCode }}</view>
          <input v-model.trim="phone" class="input-box phone-input" type="number" :placeholder="t('misc.auth.phonePlaceholder')" placeholder-class="ph" />
        </view>
      </view>

      <view class="field-group">
        <text class="label">{{ t("misc.auth.password") }}</text>
        <input v-model.trim="password" class="input-box" password :placeholder="t('misc.auth.passwordPlaceholder')" placeholder-class="ph" />
      </view>

      <view class="primary-action" :class="{ disabled: !canSubmit || loading }" @tap="submit">
        {{ loading ? t("misc.auth.loggingIn") : t("misc.auth.login") }}
      </view>

      <view class="link-row">
        <view @tap="goRegister">{{ t("misc.auth.registerAccount") }}</view>
        <view class="link-muted" @tap="goForgot">{{ t("misc.auth.forgotPassword") }}</view>
      </view>
    </view>
  </view>
</template>

<script setup lang="ts">
import { computed, ref } from "vue";
import { onLoad, onShow } from "@dcloudio/uni-app";
import { getLoginInfo, login } from "@/api/services";
import { absolutizeUrl } from "@/utils/url";
import { pickInviteParams, savePendingInvite } from "@/utils/invite";
import { takeSelectedCountry } from "@/utils/country";
import { t } from "@/i18n";

const AUTH_FROM = "login";
const phone = ref("");
const password = ref("");
const countryCode = ref("86");
const loginImage = ref("");
const loading = ref(false);

const canSubmit = computed(() => phone.value.length > 0 && password.value.length > 0);

onLoad((query) => {
  savePendingInvite(pickInviteParams(query as Record<string, unknown> | undefined));
  if (query?.invalid) {
    uni.showToast({ title: t("misc.auth.sessionExpired"), icon: "none" });
  }
  void getLoginInfo()
    .then((info) => {
      loginImage.value = absolutizeUrl(String(info?.login_img || ""));
    })
    .catch(() => undefined);
});

onShow(() => {
  const selected = takeSelectedCountry(AUTH_FROM);
  if (selected?.tel) {
    countryCode.value = selected.tel;
  }
});

async function submit() {
  if (!canSubmit.value || loading.value) {
    return;
  }
  loading.value = true;
  try {
    await login(phone.value, password.value, countryCode.value);
    uni.showToast({ title: t("misc.auth.loginSuccess"), icon: "success" });
    setTimeout(() => {
      uni.switchTab({ url: "/pages/tabbar/live/index" });
    }, 300);
  } catch (error: any) {
    uni.showToast({ title: error?.message || t("misc.auth.loginFailed"), icon: "none" });
  } finally {
    loading.value = false;
  }
}

function openCountry() {
  uni.navigateTo({ url: `/pages/auth/country?from=${AUTH_FROM}` });
}

function goBack() {
  if (getCurrentPages().length > 1) {
    uni.navigateBack();
    return;
  }
  uni.switchTab({ url: "/pages/tabbar/live/index" });
}

function goRegister() {
  uni.navigateTo({ url: "/pages/auth/register" });
}

function goForgot() {
  uni.navigateTo({ url: "/pages/auth/forgot" });
}
</script>

<style scoped>
.auth-page {
  position: relative;
  min-height: 100vh;
  overflow: hidden;
  padding: calc(28rpx + var(--status-bar-height)) 34rpx 46rpx;
  background: var(--bg);
}

.cosmic-bg {
  position: absolute;
  top: 0;
  right: 0;
  left: 0;
  height: 620rpx;
  overflow: hidden;
  background: linear-gradient(158deg, #2a1b6e 0%, #5a2ea6 46%, #a4409b 78%, #e05a86 100%);
}

.cosmic-bg::after {
  position: absolute;
  right: -10%;
  bottom: -140rpx;
  left: -10%;
  height: 240rpx;
  content: "";
  border-radius: 50% 50% 0 0;
  background: var(--bg);
}

.bg-stars {
  position: absolute;
  inset: 0;
  background-image:
    radial-gradient(3rpx 3rpx at 14% 26%, rgba(255, 255, 255, 0.9), transparent 100%),
    radial-gradient(2rpx 2rpx at 32% 58%, rgba(255, 255, 255, 0.55), transparent 100%),
    radial-gradient(3rpx 3rpx at 52% 20%, rgba(255, 255, 255, 0.75), transparent 100%),
    radial-gradient(2rpx 2rpx at 70% 44%, rgba(255, 255, 255, 0.5), transparent 100%),
    radial-gradient(3rpx 3rpx at 86% 28%, rgba(255, 255, 255, 0.8), transparent 100%);
}

.bg-orb {
  position: absolute;
  border-radius: 50%;
  pointer-events: none;
}

.bg-orb-a {
  top: -120rpx;
  right: -80rpx;
  width: 320rpx;
  height: 320rpx;
  background: radial-gradient(circle at 36% 36%, rgba(255, 173, 205, 0.5), rgba(255, 173, 205, 0) 70%);
}

.bg-orb-b {
  top: 200rpx;
  left: -100rpx;
  width: 300rpx;
  height: 300rpx;
  background: radial-gradient(circle at 60% 40%, rgba(122, 92, 255, 0.42), rgba(122, 92, 255, 0) 70%);
}

.nav-row {
  position: relative;
  z-index: 1;
  display: flex;
  align-items: center;
  height: 72rpx;
}

.back-button {
  display: flex;
  width: 68rpx;
  height: 68rpx;
  align-items: center;
  justify-content: center;
  border-radius: 50%;
  background: rgba(255, 255, 255, 0.16);
  backdrop-filter: blur(8px);
}

.back {
  width: 40rpx;
  height: 40rpx;
  filter: brightness(4);
}

.hero {
  position: relative;
  z-index: 1;
  display: flex;
  flex-direction: column;
  align-items: center;
  padding: 30rpx 0 44rpx;
  text-align: center;
}

.brand-card {
  display: flex;
  width: 164rpx;
  height: 164rpx;
  align-items: center;
  justify-content: center;
  border-radius: 42rpx;
  background: rgba(255, 255, 255, 0.14);
  box-shadow: 0 20rpx 50rpx rgba(20, 12, 56, 0.32);
  backdrop-filter: blur(10px);
}

.brand {
  width: 124rpx;
  height: 124rpx;
  border-radius: 32rpx;
}

.brand-name {
  margin-top: 22rpx;
  color: rgba(255, 255, 255, 0.85);
  font-size: 26rpx;
  font-weight: 700;
  letter-spacing: 8rpx;
}

.title {
  margin-top: 14rpx;
  color: #fff;
  font-size: 48rpx;
  font-weight: 800;
  line-height: 1;
  letter-spacing: 2rpx;
}

.form-panel {
  position: relative;
  z-index: 1;
  padding: 44rpx 36rpx 38rpx;
  border-radius: var(--radius-lg);
  background: var(--surface);
  box-shadow: 0 24rpx 60rpx rgba(40, 30, 90, 0.14);
}

.field-group {
  margin-bottom: 30rpx;
}

.label {
  display: block;
  margin-bottom: 16rpx;
  color: var(--ink);
  font-size: 25rpx;
  font-weight: 700;
}

.phone-row {
  display: flex;
  align-items: center;
  gap: 16rpx;
}

.country {
  display: flex;
  width: 132rpx;
  height: 88rpx;
  flex-shrink: 0;
  align-items: center;
  justify-content: center;
  border-radius: 20rpx;
  color: var(--brand);
  font-size: 26rpx;
  font-weight: 800;
  line-height: 1;
  background: var(--brand-soft);
}

.phone-input {
  flex: 1;
  min-width: 0;
}

.input-box {
  width: 100%;
  height: 88rpx;
  padding: 0 26rpx;
  border: 2rpx solid transparent;
  border-radius: 20rpx;
  color: var(--ink);
  font-size: 29rpx;
  background: var(--bg);
  transition: border-color 0.2s ease;
}

.ph {
  color: var(--ink-3);
}

.primary-action {
  display: flex;
  width: 100%;
  height: 94rpx;
  align-items: center;
  justify-content: center;
  margin-top: 46rpx;
  border-radius: 47rpx;
  color: #fff;
  font-size: 31rpx;
  font-weight: 800;
  letter-spacing: 4rpx;
  background: var(--grad-brand);
  box-shadow: var(--shadow-brand);
  transition: transform 0.15s ease;
}

.primary-action:active {
  transform: scale(0.98);
}

.primary-action.disabled {
  opacity: 0.45;
}

.link-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-top: 34rpx;
  color: var(--brand);
  font-size: 26rpx;
  font-weight: 700;
}

.link-muted {
  color: var(--ink-3);
  font-weight: 500;
}
</style>
