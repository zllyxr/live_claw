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

    <view class="hero compact">
      <view class="brand-card">
        <image class="brand" src="/static/brand/icon-round.webp" mode="aspectFit" />
      </view>
      <text class="title">找回密码</text>
    </view>

    <view class="form-panel">
      <view class="field-group">
        <text class="label">手机号</text>
        <view class="phone-row">
          <view class="country" @tap="openCountry">+{{ countryCode }}</view>
          <input v-model.trim="phone" class="input-box phone-input" type="number" placeholder="请输入手机号" placeholder-class="ph" />
        </view>
      </view>

      <view class="field-group">
        <text class="label">邮箱</text>
        <input v-model.trim="email" class="input-box" placeholder="请输入绑定邮箱" placeholder-class="ph" />
      </view>

      <view class="field-group">
        <text class="label">验证码</text>
        <view class="code-row">
          <input v-model.trim="code" class="input-box code-input" placeholder="请输入邮箱验证码" placeholder-class="ph" />
          <view class="code-button" :class="{ disabled: !canSendCode || sending || countdown > 0 }" @tap="sendCode">
            {{ countdown > 0 ? `${countdown}s` : "获取验证码" }}
          </view>
        </view>
      </view>

      <view class="field-group">
        <text class="label">新密码</text>
        <input v-model.trim="password" class="input-box" password placeholder="请输入新密码" placeholder-class="ph" />
      </view>

      <view class="field-group last">
        <text class="label">确认新密码</text>
        <input v-model.trim="password2" class="input-box" password placeholder="请再次输入新密码" placeholder-class="ph" />
      </view>

      <view class="primary-action" :class="{ disabled: !canSubmit || loading }" @tap="submit">
        {{ loading ? "提交中" : "重置密码" }}
      </view>

      <view class="single-link" @tap="goLogin">返回登录</view>
    </view>
  </view>
</template>

<script setup lang="ts">
import { computed, onUnmounted, ref } from "vue";
import { onShow } from "@dcloudio/uni-app";
import { getForgotCode, resetPassword } from "@/api/services";
import { takeSelectedCountry } from "@/utils/country";

const AUTH_FROM = "forgot";
const phone = ref("");
const email = ref("");
const code = ref("");
const password = ref("");
const password2 = ref("");
const countryCode = ref("86");
const loading = ref(false);
const sending = ref(false);
const countdown = ref(0);
let timer: number | undefined;

const canSendCode = computed(() => phone.value.length > 0 && /^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(email.value));
const canSubmit = computed(
  () => canSendCode.value && code.value.length > 0 && password.value.length > 0 && password2.value.length > 0
);

onShow(() => {
  const selected = takeSelectedCountry(AUTH_FROM);
  if (selected?.tel) {
    countryCode.value = selected.tel;
  }
});

function startCountdown() {
  countdown.value = 60;
  timer = setInterval(() => {
    countdown.value -= 1;
    if (countdown.value <= 0 && timer) {
      clearInterval(timer);
      timer = undefined;
    }
  }, 1000) as unknown as number;
}

async function sendCode() {
  if (!canSendCode.value || sending.value || countdown.value > 0) {
    return;
  }
  sending.value = true;
  try {
    const res = await getForgotCode(phone.value, email.value, countryCode.value);
    uni.showToast({ title: res.msg || "验证码已发送", icon: "none" });
    startCountdown();
  } catch (error: any) {
    uni.showToast({ title: error?.message || "发送失败", icon: "none" });
  } finally {
    sending.value = false;
  }
}

async function submit() {
  if (!canSubmit.value || loading.value) {
    return;
  }
  if (password.value !== password2.value) {
    uni.showToast({ title: "两次密码不一致", icon: "none" });
    return;
  }
  loading.value = true;
  try {
    const res = await resetPassword(
      phone.value,
      email.value,
      code.value,
      password.value,
      password2.value,
      countryCode.value
    );
    uni.showToast({ title: res.msg || "密码已重置", icon: "success" });
    setTimeout(() => uni.navigateBack(), 350);
  } catch (error: any) {
    uni.showToast({ title: error?.message || "重置失败", icon: "none" });
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
  uni.navigateTo({ url: "/pages/auth/login" });
}

function goLogin() {
  uni.navigateTo({ url: "/pages/auth/login" });
}

onUnmounted(() => {
  if (timer) {
    clearInterval(timer);
  }
});
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
  height: 520rpx;
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
  top: 160rpx;
  left: -100rpx;
  width: 280rpx;
  height: 280rpx;
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
  padding: 16rpx 0 36rpx;
  text-align: center;
}

.brand-card {
  display: flex;
  width: 128rpx;
  height: 128rpx;
  align-items: center;
  justify-content: center;
  border-radius: 34rpx;
  background: rgba(255, 255, 255, 0.14);
  box-shadow: 0 16rpx 40rpx rgba(20, 12, 56, 0.3);
  backdrop-filter: blur(10px);
}

.brand {
  width: 96rpx;
  height: 96rpx;
  border-radius: 26rpx;
}

.title {
  margin-top: 20rpx;
  color: #fff;
  font-size: 44rpx;
  font-weight: 800;
  line-height: 1;
  letter-spacing: 2rpx;
}

.form-panel {
  position: relative;
  z-index: 1;
  padding: 38rpx 34rpx 34rpx;
  border-radius: var(--radius-lg);
  background: var(--surface);
  box-shadow: 0 24rpx 60rpx rgba(40, 30, 90, 0.14);
}

.field-group {
  margin-bottom: 24rpx;
}

.field-group.last {
  margin-bottom: 0;
}

.label {
  display: block;
  margin-bottom: 14rpx;
  color: var(--ink);
  font-size: 24rpx;
  font-weight: 700;
}

.input-box {
  width: 100%;
  height: 84rpx;
  padding: 0 26rpx;
  border: 2rpx solid transparent;
  border-radius: 20rpx;
  color: var(--ink);
  font-size: 28rpx;
  background: var(--bg);
}

.ph {
  color: var(--ink-3);
}

.phone-row {
  display: flex;
  align-items: center;
  gap: 16rpx;
}

.country {
  display: flex;
  width: 128rpx;
  height: 84rpx;
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

.code-row {
  display: flex;
  align-items: center;
  gap: 16rpx;
}

.code-input {
  flex: 1;
  min-width: 0;
}

.code-button {
  display: flex;
  width: 184rpx;
  height: 84rpx;
  flex-shrink: 0;
  align-items: center;
  justify-content: center;
  border-radius: 20rpx;
  color: #fff;
  font-size: 24rpx;
  font-weight: 800;
  background: var(--grad-brand);
}

.code-button.disabled {
  opacity: 0.45;
}

.primary-action {
  display: flex;
  width: 100%;
  height: 94rpx;
  align-items: center;
  justify-content: center;
  margin-top: 40rpx;
  border-radius: 47rpx;
  color: #fff;
  font-size: 30rpx;
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

.single-link {
  margin-top: 30rpx;
  color: var(--brand);
  font-size: 26rpx;
  font-weight: 700;
  text-align: center;
}
</style>
