<template>
  <view class="safe-page favorite-page">
    <view class="top-row">
      <text class="title-lg">{{ t("misc.favorite.title") }}</text>
      <button class="pill-button" @tap="load(true)">{{ t("misc.common.refresh") }}</button>
    </view>

    <view v-if="items.length" class="video-grid">
      <view v-for="item in items" :key="videoKey(item)" class="video-card card" @tap="openVideo(item)">
        <image class="cover" :src="coverOf(item)" mode="aspectFill" />
        <view class="video-copy">
          <text>{{ titleOf(item) }}</text>
          <text>{{ item.datetime || "" }} · {{ item.likes || 0 }} {{ t("misc.common.likes") }}</text>
        </view>
      </view>
    </view>
    <EmptyState v-else :title="loading ? t('misc.favorite.loading') : t('misc.favorite.empty')" :description="t('misc.favorite.emptyDescription')" />
  </view>
</template>

<script setup lang="ts">
import { ref } from "vue";
import { onPullDownRefresh, onReachBottom, onShow } from "@dcloudio/uni-app";
import EmptyState from "@/components/EmptyState.vue";
import { getLikeVideos } from "@/api/services";
import type { VideoItem } from "@/types/api";
import { absolutizeUrl, firstText } from "@/utils/url";
import { t } from "@/i18n";

const items = ref<VideoItem[]>([]);
const page = ref(1);
const loading = ref(false);
const finished = ref(false);
let loadedOnce = false;

function videoKey(item: VideoItem) {
  return String(item.id || item.videoid || item.href || Math.random());
}

function videoId(item: VideoItem) {
  return firstText(item.id, item.videoid);
}

function titleOf(item: VideoItem) {
  return firstText(item.title, item.des, item.content, t("misc.video.defaultTitle"));
}

function coverOf(item: VideoItem) {
  return absolutizeUrl(firstText(item.video_thumb, item.thumb, item.thumb_s, item.href)) || "/static/brand/icon.webp";
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
    const list = await getLikeVideos(page.value);
    items.value = reset ? list : items.value.concat(list);
    if (!list.length) {
      finished.value = true;
    } else {
      page.value += 1;
    }
  } catch (error: any) {
    uni.showToast({ title: error?.message || t("misc.favorite.loadFailed"), icon: "none" });
  } finally {
    loading.value = false;
    uni.stopPullDownRefresh();
  }
}

function openVideo(item: VideoItem) {
  const id = videoId(item);
  if (id) {
    uni.navigateTo({ url: `/pages/video/detail?id=${encodeURIComponent(id)}` });
  }
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
.favorite-page {
  background: var(--bg);
}

.video-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 18rpx;
  margin-top: 22rpx;
}

.video-card {
  overflow: hidden;
}

.cover {
  width: 100%;
  height: 280rpx;
  background: var(--line);
}

.video-copy {
  padding: 18rpx;
}

.video-copy text {
  display: block;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.video-copy text:first-child {
  color: var(--ink);
  font-size: 27rpx;
  font-weight: 900;
}

.video-copy text:last-child {
  margin-top: 10rpx;
  color: var(--ink-3);
  font-size: 22rpx;
}
</style>
