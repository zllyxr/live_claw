<template>
  <view class="safe-page my-video-page">
    <view class="top-row">
      <text class="title-lg">{{ t("misc.video.myVideos") }}</text>
    </view>

    <view v-if="items.length" class="list">
      <view v-for="item in items" :key="videoKey(item)" class="video-row card" @tap="openVideo(item)">
        <image class="cover" :src="coverOf(item)" mode="aspectFill" />
        <view class="row-main">
          <text>{{ titleOf(item) }}</text>
          <text>{{ item.datetime || "" }}</text>
          <text>{{ item.likes || 0 }} {{ t("misc.common.likes") }} · {{ item.comments || 0 }} {{ t("misc.common.comments") }}</text>
        </view>
      </view>
    </view>
    <EmptyState v-else :title="loading ? t('misc.video.loadingMine') : t('misc.video.empty')" :description="t('misc.video.emptyMineDescription')" />
  </view>
</template>

<script setup lang="ts">
import { ref } from "vue";
import { onPullDownRefresh, onReachBottom, onShow } from "@dcloudio/uni-app";
import EmptyState from "@/components/EmptyState.vue";
import { getMyVideos } from "@/api/services";
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
  return absolutizeUrl(firstText(item.video_thumb, item.thumb, item.thumb_s)) || "/static/brand/icon.webp";
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
    const list = await getMyVideos(page.value);
    items.value = reset ? list : items.value.concat(list);
    if (!list.length) {
      finished.value = true;
    } else {
      page.value += 1;
    }
  } catch (error: any) {
    uni.showToast({ title: error?.message || t("misc.video.loadFailed"), icon: "none" });
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
.my-video-page {
  background: var(--bg);
}

.list {
  margin-top: 22rpx;
}

.video-row {
  display: flex;
  gap: 18rpx;
  min-height: 168rpx;
  padding: 18rpx;
  margin-bottom: 16rpx;
}

.cover {
  width: 132rpx;
  height: 132rpx;
  flex: 0 0 auto;
  border-radius: 14rpx;
  background: var(--line);
}

.row-main {
  flex: 1;
  min-width: 0;
}

.row-main text {
  display: block;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.row-main text:first-child {
  color: var(--ink);
  font-size: 29rpx;
  font-weight: 900;
}

.row-main text:not(:first-child) {
  margin-top: 14rpx;
  color: var(--ink-3);
  font-size: 23rpx;
}
</style>
