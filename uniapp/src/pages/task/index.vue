<template>
  <view class="safe-page task-page">
    <view class="hero">
      <text>每日任务</text>
      <text>{{ data?.tip_m || "完成任务后领取奖励，奖励会进入余额。" }}</text>
    </view>

    <view v-if="tasks.length" class="task-list">
      <view v-for="task in tasks" :key="String(task.id)" class="task-card card">
        <view class="task-main">
          <text>{{ task.title || "任务" }}</text>
          <text>{{ task.tip || task.tip_m || "完成后可领取奖励" }}</text>
        </view>
        <button :class="{ ready: statusOf(task) === 1, done: statusOf(task) === 2 }" @tap="receive(task)">
          {{ statusText(task) }}
        </button>
      </view>
    </view>
    <EmptyState v-else :title="loading ? '正在加载每日任务' : '暂无每日任务'" description="任务开关关闭或今日暂无任务时，这里会保持空态。" />
  </view>
</template>

<script setup lang="ts">
import { ref } from "vue";
import { onPullDownRefresh, onShow } from "@dcloudio/uni-app";
import EmptyState from "@/components/EmptyState.vue";
import { getDailyTasks, receiveDailyTaskReward } from "@/api/services";
import type { DailyTaskBundle, DailyTaskItem } from "@/types/api";

const data = ref<DailyTaskBundle>();
const tasks = ref<DailyTaskItem[]>([]);
const loading = ref(false);
const receiving = ref("");

function statusOf(task: DailyTaskItem) {
  return Number(task.status ?? task.state ?? 0);
}

function statusText(task: DailyTaskItem) {
  const status = statusOf(task);
  if (status === 1) {
    return receiving.value === String(task.id) ? "领取中" : "领取";
  }
  if (status === 2) {
    return "已领取";
  }
  return "去完成";
}

async function load() {
  loading.value = true;
  try {
    const next = await getDailyTasks();
    data.value = next;
    tasks.value = next?.list || [];
  } catch (error: any) {
    uni.showToast({ title: error?.message || "任务加载失败", icon: "none" });
  } finally {
    loading.value = false;
    uni.stopPullDownRefresh();
  }
}

async function receive(task: DailyTaskItem) {
  if (statusOf(task) !== 1 || !task.id || receiving.value) {
    return;
  }
  receiving.value = String(task.id);
  try {
    const res = await receiveDailyTaskReward(task.id);
    uni.showToast({ title: res.msg || "已领取奖励", icon: "none" });
    await load();
  } catch (error: any) {
    uni.showToast({ title: error?.message || "领取失败", icon: "none" });
  } finally {
    receiving.value = "";
  }
}

onShow(() => {
  void load();
});

onPullDownRefresh(() => {
  void load();
});
</script>

<style scoped>
.task-page {
  background: var(--bg);
}

.hero {
  min-height: 190rpx;
  padding: 34rpx 30rpx;
  border-radius: 24rpx;
  color: #fff;
  background: linear-gradient(135deg, #1f2638, var(--brand));
}

.hero text {
  display: block;
}

.hero text:first-child {
  font-size: 42rpx;
  font-weight: 900;
}

.hero text:last-child {
  margin-top: 18rpx;
  color: rgba(255, 255, 255, 0.84);
  font-size: 24rpx;
  line-height: 1.45;
}

.task-list {
  margin-top: 24rpx;
}

.task-card {
  display: flex;
  align-items: center;
  gap: 20rpx;
  min-height: 132rpx;
  padding: 24rpx;
  margin-bottom: 16rpx;
}

.task-main {
  flex: 1;
  min-width: 0;
}

.task-main text {
  display: block;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.task-main text:first-child {
  color: var(--ink);
  font-size: 29rpx;
  font-weight: 900;
}

.task-main text:last-child {
  margin-top: 12rpx;
  color: var(--ink-3);
  font-size: 24rpx;
}

.task-card button {
  display: flex;
  width: 124rpx;
  height: 58rpx;
  align-items: center;
  justify-content: center;
  border-radius: 29rpx;
  color: var(--ink-3);
  font-size: 23rpx;
  font-weight: 900;
  background: #f2f4f7;
}

.task-card button.ready {
  color: #fff;
  background: var(--brand);
}

.task-card button.done {
  color: #fff;
  background: #9aa3b3;
}
</style>
