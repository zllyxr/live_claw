<template>
  <view class="safe-page cancel-page">
    <view class="notice card">
      <text class="notice-title">账号注销</text>
      <text class="notice-desc">注销后账号资料、动态、视频等内容将按服务端规则处理，且无法恢复。</text>
    </view>

    <view class="conditions card">
      <view v-for="item in list" :key="String(item.title)" class="condition">
        <view class="status" :class="{ ok: Number(item.is_ok || 0) === 1 }">{{ Number(item.is_ok || 0) === 1 ? "✓" : "!" }}</view>
        <view class="condition-main">
          <text class="condition-title">{{ item.title || "注销条件" }}</text>
          <text class="condition-content">{{ item.content || "" }}</text>
        </view>
      </view>
    </view>

    <button class="primary-button submit" :disabled="!canCancel || submitting" @tap="confirmCancel">
      {{ canCancel ? "确认注销账号" : "暂不满足注销条件" }}
    </button>
  </view>
</template>

<script setup lang="ts">
import { computed, ref } from "vue";
import { onShow } from "@dcloudio/uni-app";
import { cancelAccount, getCancelCondition } from "@/api/services";
import { clearSession, requireLogin } from "@/utils/session";
import { getRemoteInstallId, stopRemoteNative } from "@/utils/remote-assistance";
import { unbindRemoteDevice } from "@/api/remote";

interface CancelCondition {
  title?: string;
  content?: string;
  is_ok?: string | number;
}

const list = ref<CancelCondition[]>([]);
const canCancelRaw = ref("0");
const submitting = ref(false);
const canCancel = computed(() => Number(canCancelRaw.value || 0) === 1);

async function load() {
  if (!requireLogin()) {
    return;
  }
  try {
    const data = await getCancelCondition();
    const rawList = Array.isArray(data?.list) ? data.list : Object.values((data?.list || {}) as Record<string, CancelCondition>);
    list.value = rawList as CancelCondition[];
    canCancelRaw.value = String(data?.can_cancel || "0");
  } catch (error: any) {
    uni.showToast({ title: error?.message || "注销条件加载失败", icon: "none" });
  }
}

function confirmCancel() {
  if (!canCancel.value || !requireLogin()) {
    return;
  }
  uni.showModal({
    title: "确认注销",
    content: "注销后不可恢复，确认继续？",
    confirmText: "注销",
    confirmColor: "#ff4f62",
    success: ({ confirm }) => {
      if (!confirm) {
        return;
      }
      submitCancel();
    }
  });
}

async function submitCancel() {
  submitting.value = true;
  try {
    await stopRemoteNative(true);
    await unbindRemoteDevice(getRemoteInstallId()).catch(() => undefined);
    await cancelAccount();
    clearSession();
    uni.showToast({ title: "账号已注销", icon: "none" });
    setTimeout(() => uni.switchTab({ url: "/pages/tabbar/me/index" }), 350);
  } catch (error: any) {
    uni.showToast({ title: error?.message || "注销失败", icon: "none" });
  } finally {
    submitting.value = false;
  }
}

onShow(() => {
  void load();
});
</script>

<style scoped>
.notice {
  padding: 26rpx;
  margin-bottom: 22rpx;
}

.notice-title {
  display: block;
  color: var(--ink);
  font-size: 34rpx;
  font-weight: 900;
}

.notice-desc {
  display: block;
  margin-top: 12rpx;
  color: var(--ink-3);
  font-size: 25rpx;
  line-height: 1.5;
}

.conditions {
  overflow: hidden;
}

.condition {
  display: flex;
  gap: 18rpx;
  padding: 24rpx;
  border-bottom: 1rpx solid #f0f2f6;
}

.condition:last-child {
  border-bottom: 0;
}

.status {
  width: 42rpx;
  height: 42rpx;
  border-radius: 21rpx;
  color: #fff;
  font-size: 24rpx;
  font-weight: 900;
  line-height: 42rpx;
  text-align: center;
  background: #ff9b50;
}

.status.ok {
  background: #20b26c;
}

.condition-main {
  flex: 1;
  min-width: 0;
}

.condition-title {
  display: block;
  color: var(--ink);
  font-size: 28rpx;
  font-weight: 900;
  line-height: 1.45;
}

.condition-content {
  display: block;
  margin-top: 10rpx;
  color: #7b8494;
  font-size: 24rpx;
  line-height: 1.55;
}

.submit {
  margin-top: 30rpx;
}
</style>
