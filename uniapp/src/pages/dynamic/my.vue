<template>
  <view class="safe-page my-dynamic-page">
    <view class="top-row header">
      <view>
        <text class="title-xl">我的动态</text>
        <text class="sub">管理你发布过的内容</text>
      </view>
      <button class="pill-button" @tap="goPublish">发布</button>
    </view>

    <view v-if="items.length" class="feed">
      <view v-for="item in items" :key="String(item.id || item.dynamicid)" class="feed-card card" @tap="openDetail(item)">
        <text v-if="item.title" class="content">{{ item.title }}</text>
        <view v-if="imageList(item).length" class="image-grid">
          <image v-for="image in imageList(item)" :key="image" class="feed-image" :src="image" mode="aspectFill" />
        </view>
        <image v-else-if="item.video_thumb" class="single-cover" :src="absolutizeUrl(String(item.video_thumb))" mode="aspectFill" />
        <view v-if="item.voice" class="voice-row">
          <text>语音 {{ item.length || 0 }}s</text>
        </view>
        <view class="meta-row">
          <text>{{ item.datetime || "" }}</text>
          <text>赞 {{ item.likes || 0 }}</text>
          <text>评 {{ item.comments || 0 }}</text>
        </view>
      </view>
    </view>
    <EmptyState v-else :title="loading ? '正在加载动态' : '暂无动态'" description="发布一条动态后会展示在这里。" />
  </view>
</template>

<script setup lang="ts">
import { ref } from "vue";
import { onPullDownRefresh, onReachBottom, onShow } from "@dcloudio/uni-app";
import EmptyState from "@/components/EmptyState.vue";
import { getUserDynamics } from "@/api/services";
import type { DynamicItem } from "@/types/api";
import { absolutizeUrl } from "@/utils/url";
import { getSession, requireLogin } from "@/utils/session";

const items = ref<DynamicItem[]>([]);
const page = ref(1);
const loading = ref(false);
const finished = ref(false);

function dynamicId(item: DynamicItem) {
  return String(item.id || item.dynamicid || "");
}

function imageList(item: DynamicItem) {
  return String(item.thumb || "")
    .split(";")
    .map((image) => absolutizeUrl(image))
    .filter(Boolean);
}

async function load(reset = false) {
  if (!requireLogin() || loading.value || (finished.value && !reset)) {
    uni.stopPullDownRefresh();
    return;
  }
  loading.value = true;
  if (reset) {
    page.value = 1;
    finished.value = false;
  }
  try {
    const list = await getUserDynamics(getSession().uid, page.value);
    items.value = reset ? list : items.value.concat(list);
    if (!list.length) {
      finished.value = true;
    } else {
      page.value += 1;
    }
  } catch (error: any) {
    uni.showToast({ title: error?.message || "动态加载失败", icon: "none" });
  } finally {
    loading.value = false;
    uni.stopPullDownRefresh();
  }
}

function openDetail(item: DynamicItem) {
  const id = dynamicId(item);
  if (id) {
    uni.navigateTo({ url: `/pages/dynamic/detail?id=${encodeURIComponent(id)}` });
  }
}

function goPublish() {
  if (requireLogin()) {
    uni.navigateTo({ url: "/pages/dynamic/publish" });
  }
}

onShow(() => {
  void load(true);
});

onPullDownRefresh(() => {
  void load(true);
});

onReachBottom(() => {
  void load(false);
});
</script>

<style scoped>
.header {
  margin-bottom: 24rpx;
}

.sub {
  display: block;
  margin-top: 8rpx;
  color: var(--ink-3);
  font-size: 25rpx;
}

.feed-card {
  display: block;
  width: 100%;
  padding: 24rpx;
  margin-bottom: 18rpx;
  text-align: left;
}

.content {
  display: block;
  color: #323845;
  font-size: 29rpx;
  line-height: 1.55;
}

.image-grid {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 8rpx;
  margin-top: 18rpx;
}

.feed-image,
.single-cover {
  width: 100%;
  height: 208rpx;
  border-radius: 10rpx;
  background: #f1f2f6;
}

.single-cover {
  height: 320rpx;
  margin-top: 18rpx;
}

.voice-row {
  display: inline-flex;
  align-items: center;
  height: 64rpx;
  padding: 0 24rpx;
  margin-top: 18rpx;
  border-radius: 32rpx;
  color: var(--brand);
  font-size: 25rpx;
  background: #fff2f5;
}

.meta-row {
  display: flex;
  flex-wrap: wrap;
  gap: 20rpx;
  margin-top: 20rpx;
  padding-top: 18rpx;
  border-top: 1rpx solid #f0f2f6;
  color: var(--ink-3);
  font-size: 24rpx;
}
</style>
