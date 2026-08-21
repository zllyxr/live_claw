<template>
  <view class="safe-page settings-page">
    <view class="menu card">
      <view class="menu-item" @tap="openProfile">
        <text>{{ t("settings.profile") }}</text>
        <text class="arrow">›</text>
      </view>
      <view class="menu-item" @tap="openPassword">
        <text>{{ t("settings.password") }}</text>
        <text class="arrow">›</text>
      </view>
      <view class="menu-item" @tap="openRemote">
        <text>{{ t("settings.remote") }}</text>
        <text class="arrow">›</text>
      </view>
      <view class="menu-item" @tap="openCancel">
        <text>{{ t("settings.cancel") }}</text>
        <text class="arrow">›</text>
      </view>
    </view>

    <view class="menu card">
      <picker :range="languageNames" :value="languageIndex" @change="changeLanguage">
        <view class="menu-item">
          <text>{{ t("language.title") }}</text>
          <view><text class="current-language">{{ languageNames[languageIndex] }}</text><text class="arrow">›</text></view>
        </view>
      </picker>
      <view class="menu-item" @tap="openInvalid">
        <text>{{ t("settings.invalid") }}</text>
        <text class="arrow">›</text>
      </view>
    </view>

    <button class="logout" @tap="logout">{{ t("settings.logout") }}</button>
  </view>
</template>

<script setup lang="ts">
import { clearSession, requireLogin } from "@/utils/session";
import { computed } from "vue";
import { onShow } from "@dcloudio/uni-app";
import { supportedLocales, useI18n, type AppLocale } from "@/i18n";

const { locale, t, setLocale } = useI18n();
const languageNames = computed(() => supportedLocales.map((code) => t(`language.${code === "zh-CN" ? "zh" : code}`)));
const languageIndex = computed(() => Math.max(0, supportedLocales.indexOf(locale.value)));

function changeLanguage(event: any) {
  const next = supportedLocales[Number(event.detail.value)] as AppLocale;
  setLocale(next);
  uni.setNavigationBarTitle({ title: t("settings.title") });
  uni.showToast({ title: t("language.changed"), icon: "none" });
}

onShow(() => uni.setNavigationBarTitle({ title: t("settings.title") }));

function openProfile() {
  if (requireLogin()) {
    uni.navigateTo({ url: "/pages/profile/edit" });
  }
}

function openPassword() {
  if (requireLogin()) {
    uni.navigateTo({ url: "/pages/settings/password" });
  }
}

function openRemote() {
  if (requireLogin()) {
    uni.navigateTo({ url: "/pages/remote/index" });
  }
}

function openCancel() {
  if (requireLogin()) {
    uni.navigateTo({ url: "/pages/settings/cancel" });
  }
}

function openInvalid() {
  uni.navigateTo({ url: `/pages/auth/invalid?msg=${encodeURIComponent(t("settings.sessionRevalidate"))}` });
}

function logout() {
  uni.showModal({
    title: t("settings.logout"),
    content: t("settings.logoutConfirm"),
    confirmColor: "#ff5878",
    success: ({ confirm }) => {
      if (!confirm) {
        return;
      }
      clearSession();
      uni.showToast({ title: t("settings.loggedOut"), icon: "none" });
      setTimeout(() => uni.switchTab({ url: "/pages/tabbar/me/index" }), 250);
    }
  });
}
</script>

<style scoped>
.menu {
  overflow: hidden;
  margin-bottom: 22rpx;
}

.menu-item {
  display: flex;
  align-items: center;
  justify-content: space-between;
  width: 100%;
  height: 96rpx;
  padding: 0 26rpx;
  color: var(--ink);
  font-size: 29rpx;
  font-weight: 800;
  border-bottom: 1rpx solid #f0f2f6;
}

.menu-item:last-child {
  border-bottom: 0;
}

.arrow {
  color: #b8bfcc;
  font-size: 46rpx;
}

.current-language { color: var(--ink-3); font-size: 26rpx; font-weight: 500; }

.logout {
  width: 100%;
  height: 88rpx;
  margin-top: 28rpx;
  border-radius: 44rpx;
  color: #ff4f62;
  font-size: 29rpx;
  font-weight: 900;
  background: #fff;
  border: 1rpx solid #ffd5dc;
}
</style>
