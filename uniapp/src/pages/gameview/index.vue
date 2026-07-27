<template>
  <view class="game-shell">
    <web-view v-if="url" :src="url" @message="handleWebViewMessage" />
  </view>
</template>

<script setup lang="ts">
import { onMounted, onUnmounted, ref } from "vue";
import { onLoad, onUnload } from "@dcloudio/uni-app";

const url = ref("");
let lockedLandscape = false;

function messageRequestsBack(payload: any) {
  const value = payload?.detail?.data ?? payload?.data ?? payload;
  const messages = Array.isArray(value) ? value : [value];
  return messages.some((item) => item?.type === "claw:minigame-back" || item?.action === "back");
}

function closeGame() {
  if (getCurrentPages().length > 1) {
    uni.navigateBack({ delta: 1 });
    return;
  }
  uni.reLaunch({ url: "/pages/tabbar/game/index" });
}

function handleWebViewMessage(event: any) {
  if (messageRequestsBack(event)) {
    closeGame();
  }
}

function handleWindowMessage(event: MessageEvent) {
  if (messageRequestsBack(event)) {
    closeGame();
  }
}

async function enterFullscreenLandscape() {
  lockedLandscape = true;
  // #ifdef APP-PLUS
  plus.navigator.setFullscreen(true);
  plus.screen.lockOrientation("landscape-primary");
  // #endif
  // #ifdef H5
  try {
    await document.documentElement.requestFullscreen?.();
  } catch {
    // iOS Safari 不允许自动进入系统全屏时，仍保持无导航的横向游戏画布。
  }
  try {
    await (globalThis.screen?.orientation as any)?.lock?.("landscape");
  } catch {
    // 浏览器不支持锁定时由游戏页面自动旋转横向画布。
  }
  // #endif
}

onLoad((query) => {
  url.value = decodeURIComponent(String(query?.url || ""));
  void enterFullscreenLandscape();
});

onMounted(() => {
  // H5 的 web-view 是 iframe，游戏内返回按钮通过 postMessage 关闭本页。
  globalThis.addEventListener?.("message", handleWindowMessage);
});

onUnmounted(() => {
  globalThis.removeEventListener?.("message", handleWindowMessage);
});

onUnload(() => {
  if (!lockedLandscape) {
    return;
  }
  // #ifdef APP-PLUS
  plus.screen.lockOrientation("portrait-primary");
  plus.navigator.setFullscreen(false);
  // #endif
  // #ifdef H5
  try {
    if (document.fullscreenElement) {
      void document.exitFullscreen?.();
    }
    (globalThis.screen?.orientation as any)?.unlock?.();
  } catch {
    // 浏览器不支持时无需处理。
  }
  // #endif
});
</script>

<style>
page,
.game-shell {
  width: 100%;
  height: 100%;
  min-height: 100dvh;
  overflow: hidden;
  background: #041b13;
}
</style>
