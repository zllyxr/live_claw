<template>
  <view class="safe-page settings-page">
    <view class="menu card">
      <view class="menu-item" @tap="openProfile">
        <text>我的资料</text>
        <text class="arrow">›</text>
      </view>
      <view class="menu-item" @tap="openPassword">
        <text>修改密码</text>
        <text class="arrow">›</text>
      </view>
      <view class="menu-item" @tap="openCancel">
        <text>账号注销</text>
        <text class="arrow">›</text>
      </view>
    </view>

    <view class="menu card">
      <view class="menu-item" @tap="openInvalid">
        <text>登录失效页</text>
        <text class="arrow">›</text>
      </view>
    </view>

    <button class="logout" @tap="logout">退出登录</button>
  </view>
</template>

<script setup lang="ts">
import { clearSession, requireLogin } from "@/utils/session";

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

function openCancel() {
  if (requireLogin()) {
    uni.navigateTo({ url: "/pages/settings/cancel" });
  }
}

function openInvalid() {
  uni.navigateTo({ url: "/pages/auth/invalid?msg=当前登录状态需要重新验证" });
}

function logout() {
  uni.showModal({
    title: "退出登录",
    content: "确认退出当前账号？",
    confirmColor: "#ff5878",
    success: ({ confirm }) => {
      if (!confirm) {
        return;
      }
      clearSession();
      uni.showToast({ title: "已退出登录", icon: "none" });
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
