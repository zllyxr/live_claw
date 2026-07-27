<template>
  <view class="live-page">
    <!-- 沉浸式头部：banner 背景 + 品牌 + 搜索 + 分类 -->
    <view class="hero-head">
      <image class="hero-bg" src="/static/art/banner/live-header.webp" mode="aspectFill" />
      <view class="hero-veil" />

      <view class="head-row">
        <view class="brand-line">
          <image class="brand-logo" src="/static/brand/icon-round.webp" mode="aspectFit" />
          <text class="brand-word">星域直播</text>
        </view>
        <view class="head-actions">
          <view class="ico-btn" @tap="openRank">
            <image src="/static/icons/nav-rank.svg" mode="aspectFit" />
          </view>
          <view class="ico-btn" @tap="openMessages">
            <image src="/static/icons/nav-bell.svg" mode="aspectFit" />
          </view>
        </view>
      </view>

      <view class="search-bar" @tap="openSearch">
        <image class="search-ico" src="/static/icons/nav-search.svg" mode="aspectFit" />
        <text class="search-ph">搜索主播 / 房间号</text>
      </view>

      <view class="quick-row">
        <view class="quick-card" @tap="openFollow">
          <image class="quick-ico" src="/static/icons/nav-follow.svg" mode="aspectFit" />
          <view class="quick-copy">
            <text class="quick-title">我的关注</text>
            <text class="quick-sub">{{ followLiveText }}</text>
          </view>
        </view>
        <view class="quick-card" @tap="openRank">
          <image class="quick-ico" src="/static/icons/nav-rank.svg" mode="aspectFit" />
          <view class="quick-copy">
            <text class="quick-title">人气榜</text>
            <text class="quick-sub">今日主播排名</text>
          </view>
        </view>
      </view>
    </view>

    <!-- 分类 tab -->
    <scroll-view scroll-x class="cat-strip" :show-scrollbar="false">
      <view
        v-for="cat in liveCats"
        :key="cat.key"
        class="cat-chip"
        :class="{ on: activeCat === cat.key }"
        @tap="switchCat(cat.key)"
      >
        {{ cat.name }}
      </view>
    </scroll-view>

    <view v-if="rooms.length" class="live-grid">
      <view v-for="room in rooms" :key="String(room.uid || room.stream)" class="live-card" @tap="openRoom(room)">
        <view class="cover-box">
          <SafeImage class="cover" :src="coverOf(room)" :fallback="coverFallback(room)" mode="aspectFill" />
          <view class="live-badge">
            <view class="live-pulse" />
            <text>直播中</text>
          </view>
          <view class="audience-chip">{{ displayCount(room.nums || room.hotvotes) }}人在看</view>
          <view class="cover-mask">
            <text class="room-title">{{ room.title || anchorName(room) || "星域直播间" }}</text>
            <view class="anchor-row">
              <SafeImage class="avatar" :src="avatarOf(room)" fallback="/static/icons/avatar-anchor.svg" round mode="aspectFill" />
              <text class="anchor">{{ anchorName(room) || "主播" }}</text>
            </view>
          </view>
        </view>
      </view>
    </view>
    <view v-else-if="loading" class="live-grid">
      <view v-for="i in 4" :key="i" class="skeleton-card">
        <view class="skeleton-shimmer" />
      </view>
    </view>
    <EmptyState v-else kind="live" title="暂无直播房间" description="下拉页面可重新刷新。" />
  </view>
</template>

<script setup lang="ts">
import { computed, ref } from "vue";
import { onPullDownRefresh, onReachBottom, onShow } from "@dcloudio/uni-app";
import EmptyState from "@/components/EmptyState.vue";
import SafeImage from "@/components/SafeImage.vue";
import { getFollowLive, getHotLive } from "@/api/services";
import type { LiveRoom } from "@/types/api";
import { absolutizeUrl, firstText } from "@/utils/url";
import { displayCount } from "@/utils/format";
import { isLoggedIn, requireLogin } from "@/utils/session";

const rooms = ref<LiveRoom[]>([]);
const page = ref(1);
const loading = ref(false);
const finished = ref(false);
const followLivingCount = ref<number | undefined>();
let loadedOnce = false;

const followLiveText = computed(() => {
  if (!isLoggedIn()) {
    return "登录后查看关注";
  }
  if (followLivingCount.value === undefined) {
    return "正在同步关注";
  }
  return `${followLivingCount.value}人正在直播`;
});

const COVERS = [
  "/static/art/cover/cover1.webp",
  "/static/art/cover/cover2.webp",
  "/static/art/cover/cover3.webp",
  "/static/art/cover/cover4.webp"
];

/** 无封面时按房间标识稳定分配一张占位图（同一房间每次都是同一张） */
function coverFallback(room: LiveRoom) {
  const seed = String(room.uid || room.stream || "0");
  let hash = 0;
  for (let i = 0; i < seed.length; i++) {
    hash = (hash * 31 + seed.charCodeAt(i)) % 997;
  }
  return COVERS[hash % COVERS.length] as string;
}

function coverOf(room: LiveRoom) {
  return absolutizeUrl(firstText(room.thumb, room.avatar_thumb, room.avatar));
}

function avatarOf(room: LiveRoom) {
  return absolutizeUrl(firstText(room.avatar_thumb, room.avatar)) || "/static/icons/avatar-anchor.svg";
}

function anchorName(room: LiveRoom) {
  return firstText(room.user_nicename, room.user_nickname);
}

function streamSourceOf(room: LiveRoom) {
  return firstText(room.pull, room.flvpull, room.stream);
}

async function load(reset = false) {
  if (loading.value || (finished.value && !reset)) {
    return;
  }
  loading.value = true;
  if (reset) {
    page.value = 1;
    finished.value = false;
  }
  try {
    const list = await getHotLive(page.value);
    rooms.value = reset ? list : rooms.value.concat(list);
    if (!list.length) {
      finished.value = true;
    } else {
      page.value += 1;
    }
  } catch (error: any) {
    uni.showToast({ title: error?.message || "直播加载失败", icon: "none" });
  } finally {
    loading.value = false;
    uni.stopPullDownRefresh();
  }
}

async function loadFollowLivingCount() {
  if (!isLoggedIn()) {
    followLivingCount.value = undefined;
    return;
  }
  try {
    const data = await getFollowLive(1);
    followLivingCount.value = Array.isArray(data?.list) ? data.list.length : 0;
  } catch {
    followLivingCount.value = 0;
  }
}

function openRoom(room: LiveRoom) {
  const src = streamSourceOf(room);
  const liveUid = firstText(room.liveuid, room.uid);
  const stream = firstText(room.stream);
  uni.navigateTo({
    url:
      `/pages/live/player?title=${encodeURIComponent(String(room.title || anchorName(room) || "直播间"))}` +
      `&src=${encodeURIComponent(src)}&cover=${encodeURIComponent(coverOf(room))}` +
      `&liveuid=${encodeURIComponent(liveUid)}&stream=${encodeURIComponent(stream)}` +
      `&avatar=${encodeURIComponent(avatarOf(room))}&anchor=${encodeURIComponent(String(anchorName(room) || "主播"))}` +
      `&nums=${encodeURIComponent(String(room.nums || 0))}&votes=${encodeURIComponent(String(room.hotvotes || 0))}`
  });
}

const liveCats = [
  { key: "hot", name: "热门" },
  { key: "new", name: "新秀" },
  { key: "nearby", name: "附近" },
  { key: "game", name: "游戏" },
  { key: "chat", name: "聊天" }
];
const activeCat = ref("hot");

function switchCat(key: string) {
  if (activeCat.value === key) {
    return;
  }
  activeCat.value = key;
  void load(true);
}

function openRank() {
  uni.navigateTo({ url: "/pages/user/list?type=rank&name=" + encodeURIComponent("人气榜") });
}

function openMessages() {
  if (requireLogin()) {
    uni.navigateTo({ url: "/pages/message/index" });
  }
}

function openSearch() {
  uni.navigateTo({ url: "/pages/search/index" });
}

function openFollow() {
  if (requireLogin()) {
    uni.navigateTo({ url: "/pages/live/follow" });
  }
}

onShow(() => {
  void loadFollowLivingCount();
  if (!loadedOnce) {
    loadedOnce = true;
    void load(true);
  }
});

onPullDownRefresh(() => {
  void loadFollowLivingCount();
  void load(true);
});

onReachBottom(() => {
  void load(false);
});
</script>

<style scoped>
.live-page {
  min-height: 100vh;
  overflow-x: hidden;
  padding: calc(30rpx + var(--status-bar-height)) 28rpx calc(128rpx + env(safe-area-inset-bottom));
  color: var(--ink);
  background:
    radial-gradient(56% 240rpx at 12% 0%, rgba(255, 88, 120, 0.08), transparent 100%),
    radial-gradient(46% 220rpx at 92% 2%, rgba(122, 92, 255, 0.07), transparent 100%),
    var(--bg);
}

/* ---------- 沉浸式头部 ---------- */
.hero-head {
  position: relative;
  overflow: hidden;
  margin: calc(-30rpx - var(--status-bar-height)) -28rpx 0;
  padding: calc(28rpx + var(--status-bar-height)) 28rpx 26rpx;
  border-radius: 0 0 36rpx 36rpx;
  background: linear-gradient(135deg, #2a1b6e, #5a2ea6 55%, #a4409b);
  box-shadow: 0 14rpx 40rpx rgba(48, 26, 112, 0.28);
}

.hero-bg {
  position: absolute;
  inset: 0;
  width: 100%;
  height: 100%;
}

.hero-veil {
  position: absolute;
  inset: 0;
  background:
    linear-gradient(100deg, rgba(24, 14, 62, 0.86) 0%, rgba(24, 14, 62, 0.42) 46%, rgba(24, 14, 62, 0.14) 72%),
    linear-gradient(180deg, rgba(18, 10, 48, 0.28), rgba(18, 10, 48, 0));
}

.head-row {
  position: relative;
  z-index: 1;
  display: flex;
  align-items: center;
  justify-content: space-between;
}

.brand-line {
  display: flex;
  align-items: center;
  gap: 14rpx;
}

.brand-logo {
  width: 56rpx;
  height: 56rpx;
  border-radius: 16rpx;
  background: rgba(255, 255, 255, 0.14);
}

.brand-word {
  color: #fff;
  font-size: 34rpx;
  font-weight: 800;
  letter-spacing: 2rpx;
}

.head-actions {
  display: flex;
  gap: 16rpx;
}

.ico-btn {
  display: flex;
  width: 64rpx;
  height: 64rpx;
  align-items: center;
  justify-content: center;
  border-radius: 50%;
  background: rgba(255, 255, 255, 0.16);
  backdrop-filter: blur(8px);
  transition: transform 0.15s ease;
}

.ico-btn:active {
  transform: scale(0.9);
}

.ico-btn image {
  width: 34rpx;
  height: 34rpx;
}

/* 搜索栏 */
.search-bar {
  position: relative;
  z-index: 1;
  display: flex;
  align-items: center;
  gap: 14rpx;
  height: 78rpx;
  margin-top: 26rpx;
  padding: 0 26rpx;
  border-radius: 999rpx;
  background: rgba(255, 255, 255, 0.18);
  backdrop-filter: blur(10px);
}

.search-ico {
  width: 30rpx;
  height: 30rpx;
  opacity: 0.9;
}

.search-ph {
  color: rgba(255, 255, 255, 0.72);
  font-size: 26rpx;
}

/* 快捷入口 */
.quick-row {
  position: relative;
  z-index: 1;
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 14rpx;
  margin-top: 22rpx;
}

.quick-card {
  display: flex;
  align-items: center;
  gap: 14rpx;
  padding: 18rpx 20rpx;
  border: 2rpx solid rgba(255, 255, 255, 0.16);
  border-radius: 22rpx;
  background: rgba(255, 255, 255, 0.12);
  backdrop-filter: blur(10px);
  transition: transform 0.15s ease;
}

.quick-card:active {
  transform: scale(0.96);
}

.quick-ico {
  width: 40rpx;
  height: 40rpx;
  flex: 0 0 auto;
}

.quick-copy {
  min-width: 0;
}

.quick-title {
  display: block;
  color: #fff;
  font-size: 25rpx;
  font-weight: 800;
}

.quick-sub {
  display: block;
  margin-top: 4rpx;
  color: rgba(255, 255, 255, 0.6);
  font-size: 19rpx;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

/* ---------- 分类 tab ---------- */
.cat-strip {
  width: calc(100% + 56rpx);
  margin: 24rpx -28rpx 22rpx;
  padding: 0 28rpx;
  white-space: nowrap;
}

.cat-chip {
  display: inline-flex;
  align-items: center;
  height: 64rpx;
  margin-right: 14rpx;
  padding: 0 30rpx;
  border-radius: 999rpx;
  color: var(--ink-2);
  font-size: 26rpx;
  font-weight: 600;
  background: var(--surface);
  box-shadow: var(--shadow-soft);
  transition: all 0.18s ease;
}

.cat-chip.on {
  color: #fff;
  font-weight: 800;
  background: var(--grad-brand);
  box-shadow: 0 10rpx 22rpx rgba(255, 77, 110, 0.28);
}

.live-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 20rpx;
  width: 100%;
}

.live-card {
  display: block;
  width: 100%;
  min-width: 0;
  overflow: hidden;
  border-radius: var(--radius);
  background: var(--surface);
  box-shadow: var(--shadow-soft);
  transition: transform 0.15s ease;
}

.live-card:active {
  transform: scale(0.97);
}

.cover-box {
  position: relative;
  width: 100%;
  aspect-ratio: 3 / 4;
  overflow: hidden;
  border-radius: var(--radius);
  background: linear-gradient(160deg, #eceef6, #f6f2fa);
}

.cover {
  width: 100%;
  height: 100%;
}

.live-badge {
  position: absolute;
  top: 14rpx;
  left: 14rpx;
  z-index: 2;
  display: flex;
  align-items: center;
  gap: 8rpx;
  height: 40rpx;
  padding: 0 16rpx;
  border-radius: 999rpx;
  color: #fff;
  font-size: 20rpx;
  font-weight: 800;
  background: linear-gradient(135deg, rgba(255, 77, 110, 0.95), rgba(255, 122, 77, 0.95));
  backdrop-filter: blur(6px);
}

.live-pulse {
  width: 10rpx;
  height: 10rpx;
  border-radius: 50%;
  background: #fff;
  animation: pulse 1.4s ease-in-out infinite;
}

@keyframes pulse {
  0%,
  100% {
    opacity: 1;
    transform: scale(1);
  }
  50% {
    opacity: 0.4;
    transform: scale(0.72);
  }
}

.audience-chip {
  position: absolute;
  top: 14rpx;
  right: 14rpx;
  z-index: 2;
  display: flex;
  align-items: center;
  height: 40rpx;
  padding: 0 14rpx;
  border-radius: 999rpx;
  color: rgba(255, 255, 255, 0.96);
  font-size: 19rpx;
  font-weight: 600;
  background: rgba(15, 17, 26, 0.42);
  backdrop-filter: blur(6px);
}

.cover-mask {
  position: absolute;
  right: 0;
  bottom: 0;
  left: 0;
  z-index: 1;
  padding: 72rpx 18rpx 16rpx;
  background: linear-gradient(180deg, rgba(10, 12, 20, 0) 0%, rgba(10, 12, 20, 0.68) 100%);
}

.room-title {
  display: block;
  color: #fff;
  font-size: 26rpx;
  font-weight: 800;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.anchor-row {
  display: flex;
  align-items: center;
  margin-top: 12rpx;
  min-width: 0;
}

.avatar {
  width: 36rpx;
  height: 36rpx;
  flex: 0 0 auto;
  border-radius: 50%;
  border: 2rpx solid rgba(255, 255, 255, 0.85);
  background: #2a2d3a;
}

.anchor {
  min-width: 0;
  margin-left: 10rpx;
  color: rgba(255, 255, 255, 0.92);
  font-size: 21rpx;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.skeleton-card {
  position: relative;
  width: 100%;
  aspect-ratio: 3 / 4;
  overflow: hidden;
  border-radius: var(--radius);
  background: #ecedf3;
}

.skeleton-shimmer {
  position: absolute;
  inset: 0;
  background: linear-gradient(100deg, transparent 30%, rgba(255, 255, 255, 0.65) 50%, transparent 70%);
  background-size: 220% 100%;
  animation: shimmer 1.3s ease-in-out infinite;
}

@keyframes shimmer {
  0% {
    background-position: 130% 0;
  }
  100% {
    background-position: -90% 0;
  }
}

.live-page :deep(.empty-state) {
  margin-top: 60rpx;
}
</style>
