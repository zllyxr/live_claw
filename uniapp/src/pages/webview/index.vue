<template>
  <web-view v-if="url" :src="url" />
</template>

<script setup lang="ts">
import { ref } from "vue";
import { onLoad, onUnload } from "@dcloudio/uni-app";

const url = ref("");
let lockedLandscape = false;

async function lockLandscape() {
  lockedLandscape = true;
  // #ifdef APP-PLUS
  plus.screen.lockOrientation("landscape-primary");
  // #endif
  try {
    await (globalThis.screen?.orientation as any)?.lock?.("landscape");
  } catch {
    // H5/iOS 是否允许旋转由系统决定；游戏页本身仍会显示横屏提示。
  }
}

onLoad((query) => {
  url.value = decodeURIComponent(String(query?.url || ""));
  const title = decodeURIComponent(String(query?.title || "星域"));
  const orientation = decodeURIComponent(String(query?.orientation || "auto"));
  uni.setNavigationBarTitle({ title });
  if (orientation === "landscape") {
    void lockLandscape();
  }
});

onUnload(() => {
  if (!lockedLandscape) {
    return;
  }
  // #ifdef APP-PLUS
  plus.screen.lockOrientation("portrait-primary");
  // #endif
  try {
    (globalThis.screen?.orientation as any)?.unlock?.();
  } catch {
    // 浏览器不支持时无需处理。
  }
});
</script>
