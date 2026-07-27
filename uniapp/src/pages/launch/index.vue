<template>
  <view class="launch-page">
    <image class="launch-icon" src="/static/brand/icon-round.webp" mode="aspectFit" />
    <text class="launch-name">星域</text>
    <text class="launch-tip">{{ tip }}</text>
  </view>
</template>

<script setup lang="ts">
import { ref } from "vue";
import { onReady } from "@dcloudio/uni-app";
import { checkToken, getConfig } from "@/api/services";
import { isLoggedIn } from "@/utils/session";
import { sleep } from "@/utils/format";

const tip = ref("正在启动");

async function boot() {
  try {
    tip.value = "正在读取配置";
    await getConfig().catch(() => undefined);
    if (isLoggedIn()) {
      tip.value = "正在校验登录";
      await checkToken().catch(() => undefined);
    }
  } finally {
    await sleep(550);
    uni.switchTab({ url: "/pages/tabbar/live/index" });
  }
}

onReady(() => {
  void boot();
});
</script>

<style scoped>
.launch-page {
  display: flex;
  min-height: 100vh;
  padding-bottom: 110rpx;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  background:
    radial-gradient(circle at 30% 14%, rgba(255, 138, 77, 0.2), transparent 34%),
    linear-gradient(180deg, #fff5f7 0%, #ffffff 46%, var(--bg) 100%);
}

.launch-icon {
  width: 168rpx;
  height: 168rpx;
  border-radius: 36rpx;
  box-shadow: 0 18rpx 42rpx rgba(255, 88, 120, 0.24);
}

.launch-name {
  margin-top: 30rpx;
  color: var(--ink);
  font-size: 48rpx;
  font-weight: 900;
}

.launch-tip {
  margin-top: 18rpx;
  color: var(--ink-3);
  font-size: 26rpx;
}
</style>
