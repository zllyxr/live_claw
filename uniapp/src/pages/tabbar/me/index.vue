<template>
  <view class="me-page">
    <view v-if="!loggedIn" class="login-panel">
      <view class="top-actions">
        <view class="round-action" @tap="openMessages">
          <image src="/static/native/homehot_message.png" mode="aspectFit" />
        </view>
        <view class="round-action" @tap="openSettings">
          <image src="/static/native/me_setting.png" mode="aspectFit" />
        </view>
      </view>
      <view class="login-orb login-orb-a" />
      <view class="login-orb login-orb-b" />
      <view class="login-brand-card">
        <image class="login-icon" src="/static/brand/icon-round.webp" mode="aspectFit" />
      </view>
      <text class="login-title">欢迎来到星域</text>
      <text class="login-sub">登录后查看我的资产与消息</text>
      <view class="login-button" @tap="goLogin">立即登录</view>
      <view class="register-link" @tap="goRegister">还没有账号？<text>注册账号</text></view>
    </view>

    <template v-else>
      <view class="profile-hero">
        <view class="hero-stars" />
        <view class="top-actions">
          <view class="round-action" @tap="openMessages">
            <image src="/static/native/homehot_message.png" mode="aspectFit" />
          </view>
          <view class="round-action" @tap="openSettings">
            <image src="/static/native/me_setting.png" mode="aspectFit" />
          </view>
        </view>

        <view class="profile-main" @tap="openProfileEdit">
          <image class="profile-avatar" :src="avatarUrl" mode="aspectFill" />
          <view class="profile-copy">
            <view class="name-row">
              <text class="name">{{ name }}</text>
              <text class="sex-badge" :class="{ male: sexLabel === '♂' }">{{ sexLabel }}</text>
              <image class="level-medal" :src="medalSrc" mode="aspectFit" />
            </view>
            <text class="id-line">ID:{{ user?.liang_name || user?.id || session.uid }}</text>
          </view>
          <text class="profile-arrow">›</text>
        </view>

        <view class="stats-card">
          <view class="stat-item" @tap="openUserList('fans')">
            <text>{{ displayCount(user?.fans) }}</text>
            <text>粉丝</text>
          </view>
          <view class="stat-item" @tap="openUserList('follow')">
            <text>{{ displayCount(user?.follows) }}</text>
            <text>关注</text>
          </view>
          <view class="stat-item" @tap="openFavorite">
            <text>{{ favoriteCountText }}</text>
            <text>收藏</text>
          </view>
        </view>
      </view>

      <view class="asset-row">
        <view class="asset-card recharge" @tap="openRecharge">
          <view class="asset-title">
            <text>充值</text>
            <text class="asset-ribbon">充值奖励</text>
          </view>
          <text class="asset-sub">{{ user?.coin || "0" }}星币</text>
        </view>
        <view class="asset-card detail" @tap="openWalletDetail">
          <view class="asset-title">
            <text>明细</text>
          </view>
          <text class="asset-sub">查看我的明细</text>
        </view>
        <view class="asset-card verify" @tap="openVerify">
          <view class="asset-title">
            <text>认证</text>
          </view>
          <text class="asset-sub">前去认证</text>
        </view>
      </view>

      <view class="section-block">
        <text class="section-title">我的服务</text>
        <view class="service-grid">
          <view v-for="item in services" :key="item.name" class="service-item" @tap="openMenu(item)">
            <view class="service-icon">
              <image :src="item.iconSrc" mode="aspectFit" />
            </view>
            <text>{{ item.name }}</text>
          </view>
        </view>
      </view>

      <view class="section-block">
        <text class="section-title">更多服务</text>
        <view class="more-list">
          <view v-for="item in moreServices" :key="item.name" class="more-item" @tap="openMenu(item)">
            <view class="line-icon">
              <image :src="item.iconSrc" mode="aspectFit" />
            </view>
            <text>{{ item.name }}</text>
            <text class="arrow">›</text>
          </view>
        </view>
        <view class="logout-button" @tap="logout">退出登录</view>
      </view>
    </template>
  </view>
</template>

<script setup lang="ts">
import { computed, ref } from "vue";
import { onPullDownRefresh, onShow } from "@dcloudio/uni-app";
import { getBaseInfo, getLikeVideos } from "@/api/services";
import type { UserMenuItem, UserProfile } from "@/types/api";
import { clearSession, getSession, isLoggedIn } from "@/utils/session";
import { displayCount } from "@/utils/format";
import { absolutizeUrl, firstText } from "@/utils/url";
import { medalForLevel } from "@/utils/level";

const user = ref<UserProfile>();
const loggedIn = ref(false);
const favoriteCount = ref<number | undefined>();
const session = computed(() => getSession());

const services = [
  { name: "视频", iconSrc: "/static/icons/svc-video.svg", path: "/pages/video/my" },
  { name: "动态", iconSrc: "/static/icons/svc-dynamic.svg", path: "/pages/dynamic/my" },
  { name: "收益", iconSrc: "/static/icons/svc-income.svg", path: "/pages/wallet/detail?type=cash" },
  { name: "每日任务", iconSrc: "/static/icons/svc-task.svg", path: "/pages/task/index" },
  { name: "房间管理", iconSrc: "/static/icons/svc-room.svg", path: "/pages/room/manage" }
];

const moreServices = [
  { name: "邀请奖励", iconSrc: "/static/icons/svc-invite.svg", path: "/pages/invite/index" },
  { name: "中奖记录", iconSrc: "/static/icons/svc-prize.svg" },
  { name: "在线客服", iconSrc: "/static/icons/svc-support.svg", path: "/pages/service/index" }
];

const name = computed(() => user.value?.user_nicename || user.value?.user_nickname || "星域用户");
const avatarUrl = computed(() => absolutizeUrl(firstText(user.value?.avatar_thumb, user.value?.avatar, session.value.user?.avatar_thumb, session.value.user?.avatar)) || "/static/icons/avatar-default.svg");
const sexLabel = computed(() => (String(user.value?.sex || "") === "1" ? "♂" : "♀"));
const medalSrc = computed(() => medalForLevel(user.value?.level));
const favoriteCountText = computed(() => favoriteCount.value === undefined ? "-" : displayCount(favoriteCount.value));

async function load() {
  loggedIn.value = isLoggedIn();
  if (!loggedIn.value) {
    user.value = undefined;
    favoriteCount.value = undefined;
    uni.stopPullDownRefresh();
    return;
  }
  try {
    const [profile, favorites] = await Promise.all([
      getBaseInfo(),
      getLikeVideos(1).catch(() => undefined)
    ]);
    user.value = profile;
    favoriteCount.value = Array.isArray(favorites) ? favorites.length : undefined;
  } catch (error: any) {
    uni.showToast({ title: error?.message || "资料加载失败", icon: "none" });
  } finally {
    uni.stopPullDownRefresh();
  }
}

function goLogin() {
  uni.navigateTo({ url: "/pages/auth/login" });
}

function goRegister() {
  uni.navigateTo({ url: "/pages/auth/register" });
}

function openMessages() {
  if (loggedIn.value) {
    uni.navigateTo({ url: "/pages/message/index" });
    return;
  }
  goLogin();
}

function openProfileEdit() {
  uni.navigateTo({ url: "/pages/profile/edit" });
}

function openSettings() {
  if (loggedIn.value) {
    uni.navigateTo({ url: "/pages/settings/index" });
    return;
  }
  goLogin();
}

function openRecharge() {
  if (!loggedIn.value) {
    goLogin();
    return;
  }
  uni.navigateTo({ url: "/pages/wallet/recharge" });
}

function openWalletDetail() {
  if (!loggedIn.value) {
    goLogin();
    return;
  }
  uni.navigateTo({ url: "/pages/wallet/detail?type=detail" });
}

function openVerify() {
  if (!loggedIn.value) {
    goLogin();
    return;
  }
  uni.navigateTo({ url: "/pages/verify/index" });
}

function openUserList(type: "follow" | "fans") {
  uni.navigateTo({ url: `/pages/user/list?type=${type}&uid=${encodeURIComponent(session.value.uid)}&name=${encodeURIComponent(name.value)}` });
}

function openFavorite() {
  if (!loggedIn.value) {
    goLogin();
    return;
  }
  uni.navigateTo({ url: "/pages/favorite/index" });
}

function openMenu(item: UserMenuItem & { path?: string }) {
  if (item.name === "中奖记录") {
    openWinningRecord();
    return;
  }
  if (item.path) {
    uni.navigateTo({ url: item.path });
    return;
  }
  uni.navigateTo({ url: "/pages/detail/index" });
}

function openWinningRecord() {
  if (!loggedIn.value) {
    goLogin();
    return;
  }
  uni.navigateTo({ url: "/pages/game/record?title=%E4%B8%AD%E5%A5%96%E8%AE%B0%E5%BD%95" });
}

function logout() {
  clearSession();
  user.value = undefined;
  loggedIn.value = false;
  favoriteCount.value = undefined;
  uni.showToast({ title: "已退出登录", icon: "none" });
}

onShow(() => {
  void load();
});

onPullDownRefresh(() => {
  void load();
});
</script>

<style scoped>
.me-page {
  min-height: 100vh;
  overflow-x: hidden;
  padding-bottom: calc(128rpx + env(safe-area-inset-bottom));
  color: var(--ink);
  background: var(--bg);
}

.top-actions {
  position: absolute;
  top: calc(32rpx + var(--status-bar-height));
  right: 28rpx;
  z-index: 3;
  display: flex;
  gap: 22rpx;
}

.round-action {
  display: flex;
  width: 66rpx;
  height: 66rpx;
  flex: 0 0 66rpx;
  align-items: center;
  justify-content: center;
  border-radius: 50%;
  background: rgba(255, 255, 255, 0.9);
  box-shadow: var(--shadow-soft);
  backdrop-filter: blur(8px);
}

.round-action image {
  width: 36rpx;
  height: 36rpx;
}

/* ---------- logged out ---------- */
.login-panel {
  position: relative;
  display: flex;
  min-height: 100vh;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  overflow: hidden;
  padding: calc(40rpx + var(--status-bar-height)) 28rpx calc(160rpx + env(safe-area-inset-bottom));
  text-align: center;
  background:
    radial-gradient(60% 40% at 18% 8%, rgba(122, 92, 255, 0.1), transparent 100%),
    radial-gradient(52% 36% at 88% 14%, rgba(255, 88, 130, 0.1), transparent 100%),
    var(--bg);
}

.login-orb {
  position: absolute;
  border-radius: 50%;
  pointer-events: none;
  filter: blur(2rpx);
}

.login-orb-a {
  top: 16%;
  left: 12%;
  width: 26rpx;
  height: 26rpx;
  background: linear-gradient(135deg, #b9a6ff, #ff9cba);
  opacity: 0.5;
}

.login-orb-b {
  top: 30%;
  right: 14%;
  width: 16rpx;
  height: 16rpx;
  background: linear-gradient(135deg, #ff9cba, #b9a6ff);
  opacity: 0.4;
}

.login-brand-card {
  display: flex;
  width: 176rpx;
  height: 176rpx;
  align-items: center;
  justify-content: center;
  border-radius: 44rpx;
  background: var(--surface);
  box-shadow: 0 20rpx 50rpx rgba(88, 66, 180, 0.16);
}

.login-icon {
  width: 128rpx;
  height: 128rpx;
  border-radius: 32rpx;
}

.login-title {
  margin-top: 36rpx;
  color: var(--ink);
  font-size: 40rpx;
  font-weight: 800;
}

.login-sub {
  margin-top: 16rpx;
  color: var(--ink-3);
  font-size: 26rpx;
}

.login-button {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 420rpx;
  height: 92rpx;
  margin-top: 56rpx;
  border-radius: 46rpx;
  color: #fff;
  font-size: 30rpx;
  font-weight: 800;
  letter-spacing: 2rpx;
  background: var(--grad-brand);
  box-shadow: var(--shadow-brand);
  transition: transform 0.15s ease;
}

.login-button:active {
  transform: scale(0.97);
}

.register-link {
  margin-top: 30rpx;
  color: var(--ink-3);
  font-size: 25rpx;
}

.register-link text {
  margin-left: 6rpx;
  color: var(--brand);
  font-weight: 700;
}

/* ---------- logged in ---------- */
.profile-hero {
  position: relative;
  overflow: hidden;
  margin-bottom: 96rpx;
  padding: calc(130rpx + var(--status-bar-height)) 28rpx 0;
  background: linear-gradient(150deg, #3b2a86 0%, #6a3fb5 48%, #b04a96 100%);
}

.hero-stars {
  position: absolute;
  inset: 0;
  pointer-events: none;
  background-image:
    radial-gradient(3rpx 3rpx at 16% 24%, rgba(255, 255, 255, 0.8), transparent 100%),
    radial-gradient(2rpx 2rpx at 38% 60%, rgba(255, 255, 255, 0.5), transparent 100%),
    radial-gradient(3rpx 3rpx at 58% 18%, rgba(255, 255, 255, 0.7), transparent 100%),
    radial-gradient(2rpx 2rpx at 76% 48%, rgba(255, 255, 255, 0.45), transparent 100%),
    radial-gradient(3rpx 3rpx at 90% 22%, rgba(255, 255, 255, 0.65), transparent 100%);
}

.profile-main {
  position: relative;
  z-index: 1;
  display: flex;
  align-items: center;
  gap: 26rpx;
  padding-bottom: 36rpx;
}

.profile-avatar {
  width: 132rpx;
  height: 132rpx;
  flex: 0 0 auto;
  border: 6rpx solid rgba(255, 255, 255, 0.55);
  border-radius: 50%;
  background: #eef1f7;
  box-shadow: 0 12rpx 32rpx rgba(20, 12, 56, 0.3);
}

.profile-copy {
  flex: 1;
  min-width: 0;
}

.name-row {
  display: flex;
  align-items: center;
  gap: 14rpx;
  min-width: 0;
}

.name {
  min-width: 0;
  color: #fff;
  font-size: 38rpx;
  font-weight: 800;
  line-height: 1.1;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.sex-badge {
  display: flex;
  width: 38rpx;
  height: 38rpx;
  flex: 0 0 auto;
  align-items: center;
  justify-content: center;
  border-radius: 50%;
  color: #fff;
  font-size: 22rpx;
  line-height: 1;
  background: rgba(255, 130, 170, 0.9);
}

.level-medal {
  width: 46rpx;
  height: 46rpx;
  flex: 0 0 auto;
}

.sex-badge.male {
  background: rgba(96, 140, 255, 0.9);
}

.id-line {
  display: block;
  margin-top: 14rpx;
  color: rgba(255, 255, 255, 0.65);
  font-size: 24rpx;
}

.profile-arrow {
  flex: 0 0 auto;
  color: rgba(255, 255, 255, 0.6);
  font-size: 56rpx;
  font-weight: 300;
  line-height: 1;
}

.stats-card {
  position: relative;
  z-index: 2;
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  height: 148rpx;
  margin-bottom: -74rpx;
  align-items: center;
  border-radius: var(--radius);
  background: var(--surface);
  box-shadow: var(--shadow-card);
}

.stat-item {
  position: relative;
  text-align: center;
}

.stat-item:not(:last-child)::after {
  position: absolute;
  top: 50%;
  right: 0;
  width: 2rpx;
  height: 44rpx;
  content: "";
  transform: translateY(-50%);
  background: var(--line);
}

.stat-item text:first-child {
  display: block;
  color: var(--ink);
  font-size: 34rpx;
  font-weight: 800;
}

.stat-item text:last-child {
  display: block;
  margin-top: 10rpx;
  color: var(--ink-3);
  font-size: 23rpx;
}

.asset-row {
  display: grid;
  grid-template-columns: 1.2fr 1fr 1fr;
  gap: 16rpx;
  padding: 0 28rpx;
  margin-bottom: 40rpx;
}

.asset-card {
  min-width: 0;
  min-height: 140rpx;
  padding: 26rpx 22rpx;
  border-radius: var(--radius);
  background: var(--surface);
  box-shadow: var(--shadow-soft);
  transition: transform 0.15s ease;
}

.asset-card:active {
  transform: scale(0.96);
}

.asset-card.recharge {
  background: linear-gradient(140deg, #fff 30%, #ffeef3 100%);
}

.asset-card.detail {
  background: linear-gradient(140deg, #fff 30%, #efecff 100%);
}

.asset-card.verify {
  background: linear-gradient(140deg, #fff 30%, #e6f8f0 100%);
}

.asset-title {
  display: flex;
  align-items: center;
  gap: 10rpx;
  min-width: 0;
}

.asset-title text:first-child {
  color: var(--ink);
  font-size: 30rpx;
  font-weight: 800;
}

.asset-ribbon {
  display: flex;
  align-items: center;
  justify-content: center;
  height: 34rpx;
  padding: 0 14rpx;
  border-radius: 999rpx;
  color: #fff;
  font-size: 19rpx;
  font-weight: 700;
  background: var(--grad-brand);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.asset-sub {
  display: block;
  margin-top: 24rpx;
  color: var(--ink-2);
  font-size: 23rpx;
  font-weight: 600;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.section-block {
  margin: 0 28rpx 32rpx;
  padding: 30rpx 26rpx;
  border-radius: var(--radius);
  background: var(--surface);
  box-shadow: var(--shadow-soft);
}

.section-title {
  display: block;
  color: var(--ink);
  font-size: 29rpx;
  font-weight: 800;
}

.service-grid {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  row-gap: 34rpx;
  column-gap: 10rpx;
  margin-top: 32rpx;
}

.service-item {
  display: flex;
  min-width: 0;
  flex-direction: column;
  align-items: center;
  gap: 14rpx;
  color: var(--ink-2);
  font-size: 23rpx;
  font-weight: 600;
  text-align: center;
  transition: transform 0.15s ease;
}

.service-item:active {
  transform: scale(0.92);
}

.service-icon {
  display: flex;
  width: 88rpx;
  height: 88rpx;
  align-items: center;
  justify-content: center;
  border-radius: 28rpx;
  background: var(--bg);
}

.service-icon image {
  display: block;
  width: 52rpx;
  height: 52rpx;
}

.service-item > text {
  display: block;
  width: 100%;
  overflow: hidden;
  text-align: center;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.more-list {
  margin-top: 12rpx;
}

.more-item {
  display: grid;
  grid-template-columns: 64rpx minmax(0, 1fr) 40rpx;
  gap: 20rpx;
  align-items: center;
  min-height: 104rpx;
}

.more-item:not(:last-child) {
  border-bottom: 2rpx solid var(--line);
}

.line-icon {
  display: flex;
  width: 64rpx;
  height: 64rpx;
  align-items: center;
  justify-content: center;
  border-radius: 20rpx;
  background: var(--bg);
}

.line-icon image {
  display: block;
  width: 40rpx;
  height: 40rpx;
}

.more-item text:nth-child(2) {
  display: block;
  color: var(--ink);
  font-size: 27rpx;
  font-weight: 600;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.arrow {
  color: #c4c8d2;
  font-size: 44rpx;
  font-weight: 300;
  line-height: 1;
}

.logout-button {
  display: flex;
  align-items: center;
  justify-content: center;
  height: 88rpx;
  margin-top: 30rpx;
  border-radius: 44rpx;
  color: var(--brand);
  font-size: 27rpx;
  font-weight: 700;
  background: var(--brand-soft);
  transition: transform 0.15s ease;
}

.logout-button:active {
  transform: scale(0.97);
}
</style>
