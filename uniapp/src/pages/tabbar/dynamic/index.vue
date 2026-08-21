<template>
  <view class="dynamic-page">
    <view class="dynamic-header">
      <view class="tab-row">
        <view
          v-for="tabItem in tabs"
          :key="tabItem.key"
          class="tab"
          :class="{ active: activeTab === tabItem.key }"
          @tap="switchTab(tabItem.key)"
        >
          <text>{{ tabLabel(tabItem.key) }}</text>
          <view class="tab-line" />
        </view>
      </view>
      <view class="header-actions">
        <view class="msg-btn" @tap="openMessages">
          <image src="/static/icons/nav-bell.svg" mode="aspectFit" />
        </view>
        <view class="publish-button" @tap="goPublish">
          <text class="pub-plus">+</text>
          <text>{{ t("commerce.dynamic.publish") }}</text>
        </view>
      </view>
    </view>

    <swiper class="feed-swiper" :current="activeIndex" :duration="220" :disable-touch="swiperTouchDisabled" @change="onSwiperChange">
      <swiper-item v-for="tabItem in tabs" :key="tabItem.key">
        <scroll-view
          scroll-y
          class="feed-scroll"
          :show-scrollbar="false"
          :lower-threshold="120"
          :refresher-enabled="canRefresh(tabItem.key)"
          :refresher-triggered="stateOf(tabItem.key).refreshing"
          @scrolltolower="loadMore(tabItem.key)"
          @refresherrefresh="refreshTab(tabItem.key)"
          @touchstart="onFeedTouchStart($event, tabItem.key)"
          @touchmove="onFeedTouchMove"
          @touchend="onFeedTouchEnd"
          @touchcancel="onFeedTouchEnd"
        >
          <view class="feed-inner">
            <view v-if="stateOf(tabItem.key).items.length" class="feed">
              <view
                v-for="post in stateOf(tabItem.key).items"
                :key="String(post.id || post.dynamicid)"
                class="feed-card"
                @tap="openDetail(post)"
              >
                <view class="author-row" @tap.stop="openUser(post)">
                  <image class="avatar" :src="avatarOf(post)" mode="aspectFill" />
                  <view class="author-main">
                    <text class="name">{{ post.userinfo?.user_nicename || post.userinfo?.user_nickname || t("commerce.dynamic.defaultUser") }}</text>
                    <text class="gender-dot">♀</text>
                  </view>
                  <view class="follow-pill" @tap.stop="follow(post)">{{ t("commerce.dynamic.follow") }}</view>
                </view>

                <text v-if="post.title" class="content">{{ post.title }}</text>

                <view v-if="imageList(post).length" class="image-grid">
                  <image
                    v-for="image in imageList(post)"
                    :key="image"
                    class="feed-image"
                    :src="image"
                    mode="aspectFill"
                    @tap.stop="preview(image, imageList(post))"
                  />
                </view>
                <video
                  v-if="post.href"
                  class="feed-video"
                  :src="absolutizeUrl(post.href)"
                  :poster="absolutizeUrl(String(post.video_thumb || ''))"
                  controls
                />
                <view v-if="post.voice" class="voice-row">
                  <text>{{ t("commerce.dynamic.voiceDuration").replace("{seconds}", String(post.length || 0)) }}</text>
                </view>

                <text class="time-line">{{ post.datetime || "" }}</text>

                <view class="meta-row">
                  <view class="meta" @tap.stop="openDetail(post)">
                    <view class="comment-symbol" />
                    <text>{{ post.comments || 0 }}</text>
                  </view>
                  <view class="meta like-meta" :class="{ liked: Number(post.islike || 0) }" @tap.stop="like(post)">
                    <text class="heart">♥</text>
                    <text>{{ post.likes || 0 }}</text>
                  </view>
                  <view class="meta more-meta" @tap.stop="noop">
                    <text>…</text>
                  </view>
                </view>
              </view>
            </view>
            <EmptyState
              v-else
              kind="feed"
              :title="stateOf(tabItem.key).loading ? t('commerce.dynamic.loading') : t('commerce.dynamic.empty')"
              :description="t('commerce.dynamic.emptyDescription')"
            />
          </view>
        </scroll-view>
      </swiper-item>
    </swiper>
  </view>
</template>

<script setup lang="ts">
import { computed, reactive, ref } from "vue";
import { onPullDownRefresh, onReachBottom, onShow } from "@dcloudio/uni-app";
import EmptyState from "@/components/EmptyState.vue";
import { getDynamics, likeDynamic, setAttention } from "@/api/services";
import type { DynamicItem } from "@/types/api";
import { absolutizeUrl } from "@/utils/url";
import { requireLogin } from "@/utils/session";
import { useI18n } from "@/i18n";

const { t } = useI18n();

type DynamicTab = "recommend" | "follow" | "newest";
type GestureLock = "" | "horizontal" | "vertical";

interface DynamicTabState {
  items: DynamicItem[];
  page: number;
  loading: boolean;
  finished: boolean;
  loaded: boolean;
  refreshing: boolean;
}

const tabs: Array<{ key: DynamicTab }> = [
  { key: "recommend" },
  { key: "follow" },
  { key: "newest" }
];
const DYNAMIC_DIRTY_KEY = "claw_dynamic_dirty";

const activeIndex = ref(0);
const activeTab = computed<DynamicTab>(() => tabs[activeIndex.value]?.key || "recommend");
const gestureLock = ref<GestureLock>("");
const gestureTab = ref<DynamicTab>("recommend");
const touchStartX = ref(0);
const touchStartY = ref(0);
const anyRefreshing = computed(() => tabs.some((item) => stateOf(item.key).refreshing));
const swiperTouchDisabled = computed(() => gestureLock.value === "vertical" || anyRefreshing.value);
const tabStates = reactive<Record<DynamicTab, DynamicTabState>>({
  recommend: createState(),
  follow: createState(),
  newest: createState()
});
let gestureResetTimer: ReturnType<typeof setTimeout> | undefined;

function tabLabel(tab: DynamicTab) {
  return t(`commerce.dynamic.tabs.${tab}`);
}

function createState(): DynamicTabState {
  return {
    items: [],
    page: 1,
    loading: false,
    finished: false,
    loaded: false,
    refreshing: false
  };
}

function stateOf(tab: DynamicTab) {
  return tabStates[tab];
}

function canRefresh(tab: DynamicTab) {
  return activeTab.value === tab && gestureLock.value !== "horizontal";
}

function touchPoint(event: any) {
  const touch = event?.touches?.[0] || event?.changedTouches?.[0] || event?.detail || {};
  return {
    x: Number(touch.clientX ?? touch.pageX ?? 0),
    y: Number(touch.clientY ?? touch.pageY ?? 0)
  };
}

function stopBubble(event: any) {
  if (typeof event?.stopPropagation === "function") {
    event.stopPropagation();
  }
}

function resetGestureLock(delay = 0) {
  if (gestureResetTimer) {
    clearTimeout(gestureResetTimer);
  }
  gestureResetTimer = setTimeout(() => {
    gestureLock.value = "";
  }, delay);
}

function onFeedTouchStart(event: any, tab: DynamicTab) {
  if (gestureResetTimer) {
    clearTimeout(gestureResetTimer);
  }
  const point = touchPoint(event);
  gestureTab.value = tab;
  gestureLock.value = "";
  touchStartX.value = point.x;
  touchStartY.value = point.y;
}

function onFeedTouchMove(event: any) {
  const point = touchPoint(event);
  const deltaX = point.x - touchStartX.value;
  const deltaY = point.y - touchStartY.value;
  const absX = Math.abs(deltaX);
  const absY = Math.abs(deltaY);

  if (!gestureLock.value && Math.max(absX, absY) > 12) {
    if (absX > absY + 6) {
      gestureLock.value = "horizontal";
    } else if (absY > absX + 6) {
      gestureLock.value = "vertical";
    }
  }

  if (gestureLock.value === "vertical" || anyRefreshing.value) {
    stopBubble(event);
  }
}

function onFeedTouchEnd() {
  resetGestureLock(gestureLock.value === "horizontal" ? 280 : 80);
}

function avatarOf(item: DynamicItem) {
  return absolutizeUrl(String(item.userinfo?.avatar_thumb || item.userinfo?.avatar || "")) || "/static/brand/icon-round.webp";
}

function imageList(item: DynamicItem) {
  return String(item.thumb || "")
    .split(";")
    .map((image) => absolutizeUrl(image))
    .filter(Boolean);
}

async function load(tab: DynamicTab = activeTab.value, reset = false) {
  const state = stateOf(tab);
  if (state.loading || (state.finished && !reset)) {
    if (reset) {
      state.refreshing = false;
    }
    uni.stopPullDownRefresh();
    return;
  }
  state.loading = true;
  if (reset) {
    state.page = 1;
    state.finished = false;
  }
  try {
    const list = await getDynamics(tab, state.page);
    state.items = reset ? list : state.items.concat(list);
    state.loaded = true;
    if (!list.length) {
      state.finished = true;
    } else {
      state.page += 1;
    }
  } catch (error: any) {
    uni.showToast({ title: error?.message || t("commerce.dynamic.loadFailed"), icon: "none" });
  } finally {
    state.loading = false;
    state.refreshing = false;
    uni.stopPullDownRefresh();
  }
}

function loadMore(tab: DynamicTab) {
  void load(tab, false);
}

function refreshTab(tab: DynamicTab) {
  const state = stateOf(tab);
  if (tab !== activeTab.value || gestureLock.value === "horizontal") {
    state.refreshing = false;
    return;
  }
  gestureLock.value = "vertical";
  state.refreshing = true;
  void load(tab, true);
}

function switchTab(tab: DynamicTab) {
  const index = tabs.findIndex((item) => item.key === tab);
  if (index < 0) {
    return;
  }
  activeIndex.value = index;
  const state = stateOf(tab);
  if (!state.loaded) {
    void load(tab, true);
  }
}

function onSwiperChange(event: any) {
  const index = Number(event?.detail?.current ?? 0);
  if (!Number.isFinite(index)) {
    resetGestureLock(80);
    return;
  }
  activeIndex.value = Math.max(0, Math.min(index, tabs.length - 1));
  resetGestureLock(80);
  const tab = activeTab.value;
  const state = stateOf(tab);
  if (!state.loaded) {
    void load(tab, true);
  }
}

async function like(item: DynamicItem) {
  if (!requireLogin()) {
    return;
  }
  const id = item.id || item.dynamicid;
  if (!id) {
    return;
  }
  try {
    const res = await likeDynamic(id);
    item.islike = (res?.islike as string | number | undefined) ?? 1;
    item.likes = (res?.likes as string | number | undefined) ?? Number(item.likes || 0) + 1;
  } catch (error: any) {
    uni.showToast({ title: error?.message || t("commerce.common.operationFailed"), icon: "none" });
  }
}

async function follow(item: DynamicItem) {
  if (!requireLogin()) {
    return;
  }
  const uid = String(item.uid || item.userinfo?.id || item.userinfo?.uid || "");
  if (!uid) {
    return;
  }
  try {
    await setAttention(uid);
    uni.showToast({ title: t("commerce.dynamic.followed"), icon: "none" });
  } catch (error: any) {
    uni.showToast({ title: error?.message || t("commerce.dynamic.followFailed"), icon: "none" });
  }
}

function preview(current: string, urls: string[]) {
  uni.previewImage({ current, urls });
}

function dynamicId(item: DynamicItem) {
  return String(item.id || item.dynamicid || "");
}

function openDetail(item: DynamicItem) {
  const id = dynamicId(item);
  if (id) {
    uni.navigateTo({ url: `/pages/dynamic/detail?id=${encodeURIComponent(id)}` });
  }
}

function openUser(item: DynamicItem) {
  const uid = String(item.uid || item.userinfo?.id || item.userinfo?.uid || "");
  if (uid) {
    uni.navigateTo({ url: `/pages/user/home?uid=${encodeURIComponent(uid)}` });
  }
}

function openMessages() {
  if (requireLogin()) {
    uni.navigateTo({ url: "/pages/message/index" });
  }
}

function goPublish() {
  if (requireLogin()) {
    uni.navigateTo({ url: "/pages/dynamic/publish" });
  }
}

function noop() {
  // Keep operation-area taps from bubbling into the card detail route.
}

onShow(() => {
  try {
    if (uni.getStorageSync(DYNAMIC_DIRTY_KEY)) {
      uni.removeStorageSync(DYNAMIC_DIRTY_KEY);
      activeIndex.value = tabs.findIndex((item) => item.key === "newest");
      void load("newest", true);
      return;
    }
  } catch {
    // Ignore storage failures and continue with the normal first-load path.
  }
  const state = stateOf(activeTab.value);
  if (!state.loaded) {
    void load(activeTab.value, true);
  }
});

onPullDownRefresh(() => {
  void load(activeTab.value, true);
});

onReachBottom(() => {
  void load(activeTab.value, false);
});
</script>

<style scoped>
.dynamic-page {
  height: 100vh;
  overflow: hidden;
  color: var(--ink);
  background: var(--bg);
}

.dynamic-header {
  position: relative;
  z-index: 10;
  display: flex;
  align-items: flex-end;
  justify-content: space-between;
  height: calc(128rpx + var(--status-bar-height));
  padding: var(--status-bar-height) 28rpx 16rpx;
  background: #fff;
  border-bottom: 1rpx solid #eef0f3;
}

.tab-row {
  display: flex;
  align-items: flex-end;
  gap: 34rpx;
}

.tab {
  position: relative;
  display: flex;
  flex-direction: column;
  align-items: center;
  color: #8a8d93;
  font-size: 31rpx;
  font-weight: 900;
  line-height: 1;
}

.tab.active {
  color: #181a1f;
}

.tab-line {
  width: 0;
  height: 7rpx;
  margin-top: 8rpx;
  border-radius: 7rpx;
  background: var(--grad-brand);
  transition: width 0.2s ease;
}

.tab.active .tab-line {
  width: 44rpx;
}

.header-actions {
  display: flex;
  align-items: center;
  gap: 24rpx;
  margin-bottom: 4px;
}

.msg-btn {
  display: flex;
  width: 45rpx;
  height: 45rpx;
  align-items: center;
  justify-content: center;
  border-radius: 50%;
  background: var(--grad-cosmic);
  box-shadow: 0 8rpx 18rpx rgba(122, 92, 255, 0.28);
  transition: transform 0.15s ease;
}

.msg-btn:active {
  transform: scale(0.9);
}

.msg-btn image {
  width: 32rpx;
  height: 32rpx;
}

.publish-button {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 6rpx;
  min-width: 124rpx;
  height: 45rpx;
  border-radius: 999rpx;
  color: #fff;
  font-size: 25rpx;
  font-weight: 800;
  background: var(--grad-brand);
  box-shadow: 0 8rpx 18rpx rgba(255, 77, 110, 0.24);
}

.pub-plus {
  font-size: 30rpx;
  font-weight: 300;
  line-height: 1;
  margin-top: -2rpx;
}

.feed-swiper {
  height: calc(100vh - 128rpx - var(--status-bar-height));
  background: #f4f5f7;
}

.feed-scroll {
  height: 100%;
}

.feed-inner {
  min-height: 100%;
  padding-bottom: calc(128rpx + env(safe-area-inset-bottom));
}

.feed-card {
  padding: 26rpx 28rpx 0;
  margin: 20rpx 24rpx 0;
  border-radius: var(--radius);
  background: var(--surface);
  box-shadow: var(--shadow-soft);
}

.author-row {
  display: flex;
  align-items: center;
  gap: 16rpx;
}

.avatar {
  width: 76rpx;
  height: 76rpx;
  border-radius: 50%;
  background: #eff1f5;
}

.author-main {
  flex: 1;
  min-width: 0;
}

.name {
  display: block;
  color: #272a31;
  font-size: 29rpx;
  font-weight: 500;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.gender-dot {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 34rpx;
  height: 34rpx;
  margin-top: 8rpx;
  border-radius: 50%;
  color: #fff;
  font-size: 20rpx;
  line-height: 1;
  background: linear-gradient(135deg, #d24cff, #9b39ff);
}

.follow-pill {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 104rpx;
  height: 50rpx;
  border: none;
  border-radius: 999rpx;
  color: var(--brand);
  font-size: 24rpx;
  font-weight: 800;
  background: var(--brand-soft);
}

.content {
  display: block;
  margin-top: 24rpx;
  color: #303238;
  font-size: 24rpx;
  line-height: 1.42;
  white-space: pre-wrap;
}

.image-grid {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 10rpx;
  margin-top: 20rpx;
}

.feed-image {
  width: 100%;
  aspect-ratio: 1 / 1;
  border-radius: 14rpx;
  background: #eef0f4;
}

.feed-video {
  width: 100%;
  height: 360rpx;
  margin-top: 18rpx;
  border-radius: 10rpx;
  overflow: hidden;
  background: #111;
}

.voice-row {
  display: inline-flex;
  align-items: center;
  height: 58rpx;
  padding: 0 22rpx;
  margin-top: 18rpx;
  border-radius: 29rpx;
  color: #ff5878;
  font-size: 24rpx;
  background: #fff2f5;
}

.time-line {
  display: block;
  margin-top: 20rpx;
  color: #9b9fa8;
  font-size: 23rpx;
}

.meta-row {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  align-items: center;
  height: 82rpx;
  margin-top: 20rpx;
  border-top: 1rpx solid #f0f1f4;
}

.meta {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 12rpx;
  color: #9a9da5;
  font-size: 24rpx;
}

.comment-symbol {
  position: relative;
  width: 36rpx;
  height: 28rpx;
  border: 4rpx solid #9ea2aa;
  border-radius: 12rpx;
}

.comment-symbol::after {
  position: absolute;
  bottom: -8rpx;
  left: 7rpx;
  width: 10rpx;
  height: 10rpx;
  content: "";
  border-left: 4rpx solid #9ea2aa;
  border-bottom: 4rpx solid #9ea2aa;
  transform: rotate(-24deg);
}

.heart {
  color: #ff3f5f;
  font-size: 38rpx;
  line-height: 1;
}

.like-meta.liked {
  color: #ff3f5f;
}

.more-meta text {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 40rpx;
  height: 40rpx;
  border: 4rpx solid #a5a9b1;
  border-radius: 50%;
  color: #a5a9b1;
  font-size: 28rpx;
  line-height: 22rpx;
}

.dynamic-page :deep(.empty-state) {
  margin-top: 90rpx;
}
</style>
