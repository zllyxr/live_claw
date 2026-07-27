<template>
  <view class="safe-image" :class="{ round }">
    <!-- 骨架：加载中显示 -->
    <view v-if="loading" class="ph-skeleton" />
    <image
      v-if="displaySrc"
      class="ph-img"
      :class="{ hidden: loading }"
      :src="displaySrc"
      :mode="mode"
      :lazy-load="lazyLoad"
      @load="onLoad"
      @error="onError"
    />
    <!-- 兜底：无地址或加载失败且无兜底图时的占位 -->
    <view v-if="!displaySrc" class="ph-fallback">
      <view class="ph-mark" />
    </view>
  </view>
</template>

<script setup lang="ts">
import { computed, ref, watch } from "vue";

const props = withDefaults(
  defineProps<{
    /** 主图地址 */
    src?: string;
    /** 加载失败后的兜底图，未提供则显示纯色占位 */
    fallback?: string;
    mode?: string;
    /** 圆形裁剪 */
    round?: boolean;
    lazyLoad?: boolean;
  }>(),
  {
    src: "",
    fallback: "",
    mode: "aspectFill",
    round: false,
    lazyLoad: true
  }
);

const failed = ref(false);
const loading = ref(true);

const displaySrc = computed(() => {
  const primary = String(props.src || "").trim();
  if (!failed.value && primary) {
    return primary;
  }
  return String(props.fallback || "").trim();
});

// 换图时重置状态，避免沿用上一张的失败标记
watch(
  () => props.src,
  () => {
    failed.value = false;
    loading.value = Boolean(String(props.src || "").trim() || props.fallback);
  }
);

function onLoad() {
  loading.value = false;
}

function onError() {
  // 主图失败 → 切兜底图；兜底图也失败 → 显示纯色占位
  if (!failed.value && props.fallback) {
    failed.value = true;
    return;
  }
  failed.value = true;
  loading.value = false;
}
</script>

<style scoped>
.safe-image {
  position: relative;
  width: 100%;
  height: 100%;
  overflow: hidden;
  background: #eef0f6;
}

.safe-image.round {
  border-radius: 50%;
}

.ph-img {
  display: block;
  width: 100%;
  height: 100%;
  transition: opacity 0.25s ease;
}

.ph-img.hidden {
  opacity: 0;
}

.ph-skeleton {
  position: absolute;
  inset: 0;
  background: linear-gradient(100deg, #eceef4 30%, #f7f8fc 50%, #eceef4 70%);
  background-size: 220% 100%;
  animation: shimmer 1.2s ease-in-out infinite;
}

@keyframes shimmer {
  0% {
    background-position: 130% 0;
  }
  100% {
    background-position: -90% 0;
  }
}

.ph-fallback {
  position: absolute;
  inset: 0;
  display: flex;
  align-items: center;
  justify-content: center;
  background: linear-gradient(160deg, #eef0f6, #f6f2fa);
}

.ph-mark {
  width: 40%;
  height: 40%;
  max-width: 72rpx;
  max-height: 72rpx;
  border-radius: 18rpx;
  background: linear-gradient(135deg, rgba(122, 92, 255, 0.25), rgba(255, 88, 120, 0.25));
}
</style>
