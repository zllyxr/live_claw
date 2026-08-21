<template>
  <view class="safe-page follow-live-page">
    <view class="hero">
      <text class="hero-title">{{ data?.title || t("live.following") }}</text>
      <text class="hero-desc">{{ data?.des || t("live.followHint") }}</text>
    </view>

    <view v-if="rooms.length" class="live-grid">
      <view v-for="room in rooms" :key="String(room.uid || room.stream)" class="live-card" @tap="openRoom(room)">
        <image class="cover" :src="coverOf(room)" mode="aspectFill" />
        <view class="cover-mask" />
        <view class="live-badge">LIVE</view>
        <view class="card-copy">
          <text class="room-title">{{ room.title || anchorName(room) || t("home.defaultRoom") }}</text>
          <view class="anchor-row">
            <image :src="avatarOf(room)" mode="aspectFill" />
            <text>{{ anchorName(room) || t("home.host") }}</text>
            <text>{{ t("home.people", { count: displayCount(room.nums || room.hotvotes) }) }}</text>
          </view>
        </view>
      </view>
    </view>
    <EmptyState v-else :title="loading ? t('live.loadingFollowing') : t('live.noFollowingLive')" :description="t('live.followDescription')" />
  </view>
</template>

<script setup lang="ts">
import { computed, ref } from "vue";
import { onPullDownRefresh, onReachBottom, onShow } from "@dcloudio/uni-app";
import EmptyState from "@/components/EmptyState.vue";
import { getFollowLive } from "@/api/services";
import type { FollowLiveHome, LiveRoom } from "@/types/api";
import { absolutizeUrl, firstText } from "@/utils/url";
import { useI18n } from "@/i18n";

const { t } = useI18n();
import { displayCount } from "@/utils/format";
import { requireLogin } from "@/utils/session";

const data = ref<FollowLiveHome>();
const page = ref(1);
const loading = ref(false);
const finished = ref(false);
let loadedOnce = false;

const rooms = computed(() => data.value?.list || []);

function coverOf(room: LiveRoom) {
  return absolutizeUrl(firstText(room.thumb, room.avatar_thumb, room.avatar)) || "/static/brand/icon.webp";
}

function avatarOf(room: LiveRoom) {
  return absolutizeUrl(firstText(room.avatar_thumb, room.avatar)) || "/static/brand/icon-round.webp";
}

function anchorName(room: LiveRoom) {
  return firstText(room.user_nicename, room.user_nickname);
}

function streamSourceOf(room: LiveRoom) {
  return firstText(room.pull, room.flvpull, room.stream);
}

async function load(reset = false) {
  if (!requireLogin()) {
    uni.stopPullDownRefresh();
    return;
  }
  if (loading.value || (finished.value && !reset)) {
    return;
  }
  loading.value = true;
  if (reset) {
    page.value = 1;
    finished.value = false;
  }
  try {
    const next = await getFollowLive(page.value);
    const list = next?.list || [];
    data.value = {
      ...next,
      list: reset ? list : rooms.value.concat(list)
    };
    if (!list.length) {
      finished.value = true;
    } else {
      page.value += 1;
    }
  } catch (error: any) {
    uni.showToast({ title: error?.message || t("live.followLoadFailed"), icon: "none" });
  } finally {
    loading.value = false;
    uni.stopPullDownRefresh();
  }
}

function openRoom(room: LiveRoom) {
  const src = streamSourceOf(room);
  const liveUid = firstText(room.liveuid, room.uid);
  const stream = firstText(room.stream);
  uni.navigateTo({
    url:
      `/pages/live/player?title=${encodeURIComponent(String(room.title || anchorName(room) || t("home.liveRoom")))}` +
      `&src=${encodeURIComponent(src)}&cover=${encodeURIComponent(coverOf(room))}` +
      `&liveuid=${encodeURIComponent(liveUid)}&stream=${encodeURIComponent(stream)}` +
      `&avatar=${encodeURIComponent(avatarOf(room))}&anchor=${encodeURIComponent(String(anchorName(room) || t("home.host")))}` +
      `&nums=${encodeURIComponent(String(room.nums || 0))}&votes=${encodeURIComponent(String(room.hotvotes || 0))}`
  });
}

onShow(() => {
  if (!loadedOnce) {
    loadedOnce = true;
    void load(true);
  }
});

onPullDownRefresh(() => {
  void load(true);
});

onReachBottom(() => {
  void load(false);
});
</script>

<style scoped>
.follow-live-page {
  background: #f6f7fb;
}

.hero {
  min-height: 176rpx;
  padding: 30rpx;
  border-radius: 22rpx;
  color: #fff;
  background: linear-gradient(135deg, #25253a, var(--brand));
}

.hero-title {
  display: block;
  font-size: 38rpx;
  font-weight: 900;
}

.hero-desc {
  display: block;
  margin-top: 16rpx;
  color: rgba(255, 255, 255, 0.82);
  font-size: 25rpx;
}

.live-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 18rpx;
  margin-top: 22rpx;
}

.live-card {
  position: relative;
  overflow: hidden;
  height: 420rpx;
  border-radius: 18rpx;
  background: #11131d;
}

.cover,
.cover-mask {
  position: absolute;
  inset: 0;
  width: 100%;
  height: 100%;
}

.cover-mask {
  background: linear-gradient(180deg, rgba(0, 0, 0, 0.08), rgba(0, 0, 0, 0.64));
}

.live-badge {
  position: absolute;
  top: 16rpx;
  left: 16rpx;
  height: 38rpx;
  padding: 0 14rpx;
  border-radius: 19rpx;
  color: #fff;
  font-size: 19rpx;
  font-weight: 900;
  line-height: 38rpx;
  background: var(--brand);
}

.card-copy {
  position: absolute;
  right: 16rpx;
  bottom: 16rpx;
  left: 16rpx;
}

.room-title {
  display: block;
  color: #fff;
  font-size: 28rpx;
  font-weight: 900;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.anchor-row {
  display: flex;
  align-items: center;
  gap: 10rpx;
  margin-top: 14rpx;
  color: rgba(255, 255, 255, 0.8);
  font-size: 22rpx;
}

.anchor-row image {
  width: 36rpx;
  height: 36rpx;
  border-radius: 18rpx;
}

.anchor-row text {
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.anchor-row text:first-of-type {
  flex: 1;
}
</style>
