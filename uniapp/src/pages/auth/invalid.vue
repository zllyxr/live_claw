<template>
  <view class="safe-page invalid-page">
    <image class="icon" src="/static/brand/icon-round.webp" mode="aspectFit" />
    <text class="title">{{ t("misc.auth.invalidTitle") }}</text>
    <text class="desc">{{ message }}</text>
    <button class="primary-button action" @tap="goLogin">{{ t("misc.auth.loginAgain") }}</button>
    <button class="ghost" @tap="goMe">{{ t("misc.auth.backToMe") }}</button>
  </view>
</template>

<script setup lang="ts">
import { ref } from "vue";
import { onLoad } from "@dcloudio/uni-app";
import { clearSession } from "@/utils/session";
import { t } from "@/i18n";

const message = ref(t("misc.auth.invalidDescription"));

onLoad((query) => {
  clearSession();
  message.value = decodeURIComponent(String(query?.msg || message.value));
});

function goLogin() {
  uni.redirectTo({ url: "/pages/auth/login?invalid=1" });
}

function goMe() {
  uni.switchTab({ url: "/pages/tabbar/me/index" });
}
</script>

<style scoped>
.invalid-page {
  display: flex;
  min-height: 100vh;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  text-align: center;
}

.icon {
  width: 150rpx;
  height: 150rpx;
  border-radius: 36rpx;
  box-shadow: 0 12rpx 36rpx rgba(255, 88, 120, 0.22);
}

.title {
  margin-top: 30rpx;
  color: var(--ink);
  font-size: 36rpx;
  font-weight: 900;
}

.desc {
  max-width: 540rpx;
  margin-top: 16rpx;
  color: var(--ink-3);
  font-size: 26rpx;
  line-height: 1.55;
}

.action {
  width: 430rpx;
  margin-top: 44rpx;
}

.ghost {
  width: 430rpx;
  height: 84rpx;
  margin-top: 18rpx;
  border-radius: 42rpx;
  color: var(--ink-2);
  font-size: 28rpx;
  font-weight: 800;
  background: #fff;
  border: 1rpx solid #e6e8ef;
}
</style>
