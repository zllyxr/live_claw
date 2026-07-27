<template>
  <view class="empty-state">
    <image v-if="artSrc" class="empty-art-img" :src="artSrc" mode="aspectFit" />
    <image v-else-if="image" class="empty-image" :src="image" mode="aspectFit" />
    <view v-else class="empty-art">
      <view class="orb" />
      <view class="ring" />
      <view class="spark spark-a" />
      <view class="spark spark-b" />
      <view class="spark spark-c" />
    </view>
    <text class="empty-title">{{ title }}</text>
    <text v-if="description" class="empty-desc">{{ description }}</text>
  </view>
</template>

<script setup lang="ts">
import { computed } from "vue";

/** 空态类型 → AI 插画映射 */
const ART: Record<string, string> = {
  live: "/static/art/empty/live.webp",
  feed: "/static/art/empty/feed.webp",
  bet: "/static/art/empty/bet.webp",
  search: "/static/art/empty/search.webp"
};

const props = defineProps<{
  title: string;
  description?: string;
  /** 自定义图片，优先级低于 kind */
  image?: string;
  /** 空态类型：live / feed / bet / search */
  kind?: string;
}>();

const artSrc = computed(() => (props.kind ? ART[props.kind] || "" : ""));
</script>

<style scoped>
.empty-state {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  min-height: 300rpx;
  padding: 64rpx 28rpx;
}

.empty-image {
  width: 132rpx;
  height: 132rpx;
  margin-bottom: 22rpx;
}

.empty-art-img {
  width: 380rpx;
  height: 300rpx;
  margin-bottom: 8rpx;
}

.empty-art {
  position: relative;
  width: 168rpx;
  height: 168rpx;
  margin-bottom: 26rpx;
}

.orb {
  position: absolute;
  inset: 20rpx;
  border-radius: 50%;
  background:
    radial-gradient(circle at 32% 28%, rgba(255, 255, 255, 0.85), transparent 42%),
    linear-gradient(135deg, #efe9ff 0%, #ffe3ec 100%);
  box-shadow: inset 0 -10rpx 24rpx rgba(122, 92, 255, 0.12);
}

.ring {
  position: absolute;
  top: 50%;
  left: 50%;
  width: 190rpx;
  height: 64rpx;
  border: 4rpx solid rgba(122, 92, 255, 0.28);
  border-radius: 50%;
  transform: translate(-50%, -50%) rotate(-18deg);
}

.spark {
  position: absolute;
  border-radius: 50%;
  background: linear-gradient(135deg, #ff8fb0, #7a5cff);
  opacity: 0.7;
}

.spark-a {
  top: 10rpx;
  right: 18rpx;
  width: 14rpx;
  height: 14rpx;
}

.spark-b {
  bottom: 22rpx;
  left: 8rpx;
  width: 10rpx;
  height: 10rpx;
}

.spark-c {
  top: 44rpx;
  left: 0;
  width: 8rpx;
  height: 8rpx;
  opacity: 0.45;
}

.empty-title {
  color: #6a7080;
  font-size: 28rpx;
  font-weight: 700;
}

.empty-desc {
  max-width: 520rpx;
  margin-top: 12rpx;
  color: #a4aab8;
  font-size: 24rpx;
  line-height: 1.5;
  text-align: center;
}
</style>
