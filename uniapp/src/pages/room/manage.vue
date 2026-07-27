<template>
  <view class="safe-page room-manage-page">
    <view class="hero">
      <text>房间管理</text>
      <text>查看我的直播间房管与基础管理入口</text>
    </view>

    <view class="quick-grid">
      <view class="quick-card card" @tap="openProfile">
        <text>资料</text>
        <text>编辑主播资料</text>
      </view>
      <view class="quick-card card" @tap="openVerify">
        <text>认证</text>
        <text>主播认证信息</text>
      </view>
    </view>

    <view class="section-head">
      <text>房管列表</text>
      <button @tap="load">刷新</button>
    </view>

    <view v-if="admins.length" class="admin-list card">
      <view v-for="item in admins" :key="String(item.id || item.uid)" class="admin-row" @tap="openUser(item)">
        <image :src="avatarOf(item)" mode="aspectFill" />
        <view>
          <text>{{ item.user_nicename || item.user_nickname || "星域用户" }}</text>
          <text>ID：{{ item.id || item.uid || "-" }}</text>
        </view>
        <text>›</text>
      </view>
    </view>
    <EmptyState v-else :title="loading ? '正在加载房管' : '暂无房管'" description="直播间设置过房管后会显示在这里。" />
  </view>
</template>

<script setup lang="ts">
import { ref } from "vue";
import { onPullDownRefresh, onShow } from "@dcloudio/uni-app";
import EmptyState from "@/components/EmptyState.vue";
import { getLiveAdminList } from "@/api/services";
import type { UserProfile } from "@/types/api";
import { getSession, requireLogin } from "@/utils/session";
import { absolutizeUrl } from "@/utils/url";

const admins = ref<UserProfile[]>([]);
const loading = ref(false);

function avatarOf(item: UserProfile) {
  return absolutizeUrl(String(item.avatar_thumb || item.avatar || "")) || "/static/brand/icon-round.webp";
}

async function load() {
  if (!requireLogin()) {
    uni.stopPullDownRefresh();
    return;
  }
  loading.value = true;
  try {
    const data = await getLiveAdminList(getSession().uid);
    admins.value = data?.list || [];
  } catch (error: any) {
    uni.showToast({ title: error?.message || "房管加载失败", icon: "none" });
  } finally {
    loading.value = false;
    uni.stopPullDownRefresh();
  }
}

function openUser(item: UserProfile) {
  const uid = String(item.id || item.uid || "");
  if (uid) {
    uni.navigateTo({ url: `/pages/user/home?uid=${encodeURIComponent(uid)}` });
  }
}

function openProfile() {
  uni.navigateTo({ url: "/pages/profile/edit" });
}

function openVerify() {
  uni.navigateTo({ url: "/pages/verify/index" });
}

onShow(() => {
  void load();
});

onPullDownRefresh(() => {
  void load();
});
</script>

<style scoped>
.room-manage-page {
  background: var(--bg);
}

.hero {
  min-height: 176rpx;
  padding: 32rpx;
  border-radius: 24rpx;
  color: #fff;
  background: linear-gradient(135deg, #23283d, #7c5cff);
}

.hero text {
  display: block;
}

.hero text:first-child {
  font-size: 40rpx;
  font-weight: 900;
}

.hero text:last-child {
  margin-top: 16rpx;
  color: rgba(255, 255, 255, 0.82);
  font-size: 25rpx;
}

.quick-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 16rpx;
  margin-top: 22rpx;
}

.quick-card {
  min-height: 130rpx;
  padding: 24rpx;
}

.quick-card text {
  display: block;
}

.quick-card text:first-child {
  color: var(--ink);
  font-size: 30rpx;
  font-weight: 900;
}

.quick-card text:last-child {
  margin-top: 12rpx;
  color: var(--ink-3);
  font-size: 23rpx;
}

.section-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin: 34rpx 0 18rpx;
}

.section-head text {
  color: var(--ink);
  font-size: 31rpx;
  font-weight: 900;
}

.section-head button {
  display: flex;
  min-width: 104rpx;
  height: 56rpx;
  align-items: center;
  justify-content: center;
  border-radius: 28rpx;
  color: #7c5cff;
  font-size: 24rpx;
  font-weight: 900;
  background: #f0edff;
}

.admin-list {
  overflow: hidden;
}

.admin-row {
  display: grid;
  grid-template-columns: 72rpx minmax(0, 1fr) 34rpx;
  gap: 18rpx;
  align-items: center;
  min-height: 112rpx;
  padding: 20rpx 24rpx;
  border-bottom: 1rpx solid #f0f2f6;
}

.admin-row:last-child {
  border-bottom: 0;
}

.admin-row image {
  width: 72rpx;
  height: 72rpx;
  border-radius: 36rpx;
}

.admin-row text {
  display: block;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.admin-row view text:first-child {
  color: var(--ink);
  font-size: 28rpx;
  font-weight: 900;
}

.admin-row view text:last-child {
  margin-top: 8rpx;
  color: var(--ink-3);
  font-size: 23rpx;
}

.admin-row > text {
  color: #b8bfcc;
  font-size: 42rpx;
}
</style>
