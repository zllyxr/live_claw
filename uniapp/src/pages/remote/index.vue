<template>
  <view class="remote-page">
    <view class="notice-card">
      <text class="notice-title">仅在你需要协助时开启</text>
      <text class="notice-copy">开启后，管理员可在你授权的范围内查看屏幕并协助操作。系统会持续显示“星域远程协助正在运行”通知；重启、强制停止或录屏授权失效后，需要你再次确认。</text>
    </view>

    <view class="status-card">
      <view class="status-head">
        <view>
          <text class="status-title">协助状态</text>
          <text class="status-sub">{{ statusText }}</text>
        </view>
        <view class="status-dot" :class="{ active: running }" />
      </view>
      <view class="id-row">
        <text>设备代码</text>
        <text class="id-value">{{ deviceCode || "尚未分配" }}</text>
      </view>
      <view class="id-row">
        <text>服务器连接</text>
        <text>{{ serverOnline ? "在线" : "离线" }}</text>
      </view>
    </view>

    <view class="permission-card">
      <view class="section-head">
        <text>权限与系统状态</text>
        <text>点按可前往设置</text>
      </view>
      <view
        v-for="item in permissionItems"
        :key="item.key"
        class="permission-row"
        @tap="openPermission(item.key)"
      >
        <view>
          <text class="permission-name">{{ item.name }}</text>
          <text class="permission-description">{{ item.description }}</text>
        </view>
        <text class="permission-state" :class="{ ok: item.granted }">{{ item.granted ? "已开启" : "未开启" }} ›</text>
      </view>
    </view>

    <button v-if="!running" class="primary-button" :loading="busy" :disabled="busy" @tap="enableRemote">
      开启远程协助
    </button>
    <button v-else class="stop-button" :loading="busy" :disabled="busy" @tap="stopRemote">
      停止远程协助
    </button>
    <button v-if="enrolled" class="unbind-button" :disabled="busy" @tap="unbindRemote">解绑此设备</button>

    <text class="privacy-copy">管理员无法隐藏系统通知，也无法绕过 Android 的屏幕共享确认。协助授权和画面不会写入业务日志。</text>
  </view>
</template>

<script setup lang="ts">
import { computed, ref } from "vue";
import { onPullDownRefresh, onShow } from "@dcloudio/uni-app";
import { enrollRemoteDevice, getCurrentRemoteDevice, unbindRemoteDevice } from "@/api/remote";
import { requireLogin } from "@/utils/session";
import {
  getRemoteInstallId,
  getRemoteNativeStatus,
  initializeRemoteNative,
  nativeDeviceMetadata,
  openRemotePermissionSettings,
  startRemoteNative,
  stopRemoteNative
} from "@/utils/remote-assistance";
import type { NativeRemoteStatus } from "@/utils/remote-assistance";

const busy = ref(false);
const enrolled = ref(false);
const serverOnline = ref(false);
const nativeStatus = ref<NativeRemoteStatus>({
  available: false, running: false, service_status: "stopped", permissions: {}
});
const serverDeviceCode = ref("");

const running = computed(() => nativeStatus.value.running);
const deviceCode = computed(() => nativeStatus.value.device_code || serverDeviceCode.value);
const statusText = computed(() => {
  if (!nativeStatus.value.available) return nativeStatus.value.message || "当前安装包不支持";
  if (running.value) return "正在运行，等待管理员连接";
  if (enrolled.value) return "已绑定，当前未运行";
  return "尚未开启";
});

const permissionDefinitions = [
  ["notification", "前台通知", "运行期间固定显示，Android 13+ 需要通知权限"],
  ["media_projection", "屏幕共享", "每次失效后都必须由你重新确认"],
  ["accessibility", "无障碍服务", "用于触控和文字输入协助"],
  ["battery", "电池白名单", "降低系统在后台结束服务的概率"]
] as const;

const permissionItems = computed(() => permissionDefinitions.map(([key, name, description]) => ({
  key, name, description, granted: Boolean(nativeStatus.value.permissions?.[key])
})));

async function refresh() {
  if (!requireLogin()) {
    uni.stopPullDownRefresh();
    return;
  }
  const installId = getRemoteInstallId();
  try {
    const [native, server] = await Promise.all([
      getRemoteNativeStatus(),
      getCurrentRemoteDevice(installId).catch(() => undefined)
    ]);
    nativeStatus.value = native;
    enrolled.value = Boolean(server);
    serverOnline.value = Boolean(server?.online);
    serverDeviceCode.value = server?.device_code || "";
  } finally {
    uni.stopPullDownRefresh();
  }
}

async function enableRemote() {
  if (busy.value || !requireLogin()) return;
  busy.value = true;
  let enrolledDuringAttempt = false;
  try {
    const enrollment = await enrollRemoteDevice(getRemoteInstallId(), nativeDeviceMetadata());
    enrolledDuringAttempt = true;
    const initialized = await initializeRemoteNative(enrollment);
    if (!initialized.available) throw new Error(initialized.message || "当前安装包缺少远程协助组件");
    nativeStatus.value = await startRemoteNative();
    enrolled.value = true;
    uni.showToast({ title: "请按系统提示确认屏幕共享", icon: "none" });
    setTimeout(() => void refresh(), 1500);
  } catch (error: any) {
    if (enrolledDuringAttempt) {
      await stopRemoteNative(true).catch(() => undefined);
      await unbindRemoteDevice(getRemoteInstallId()).catch(() => undefined);
      enrolled.value = false;
    }
    uni.showModal({ title: "无法开启", content: error?.message || "远程协助暂不可用", showCancel: false });
  } finally {
    busy.value = false;
  }
}

async function stopRemote() {
  if (busy.value) return;
  busy.value = true;
  try {
    nativeStatus.value = await stopRemoteNative(false);
    serverOnline.value = false;
    uni.showToast({ title: "远程协助已停止", icon: "none" });
  } finally {
    busy.value = false;
  }
}

function unbindRemote() {
  if (busy.value) return;
  uni.showModal({
    title: "解绑此设备",
    content: "解绑会立即停止服务、撤销设备凭据并清除远程控制授权。确定继续？",
    confirmColor: "#e5484d",
    success: async ({ confirm }) => {
      if (!confirm) return;
      busy.value = true;
      try {
        await stopRemoteNative(true);
        await unbindRemoteDevice(getRemoteInstallId());
        enrolled.value = false;
        serverOnline.value = false;
        serverDeviceCode.value = "";
        await refresh();
      } catch (error: any) {
        uni.showToast({ title: error?.message || "解绑失败", icon: "none" });
      } finally {
        busy.value = false;
      }
    }
  });
}

async function openPermission(permission: string) {
  try {
    nativeStatus.value = await openRemotePermissionSettings(permission);
  } catch (error: any) {
    uni.showToast({ title: error?.message || "无法打开设置", icon: "none" });
  }
}

onShow(() => void refresh());
onPullDownRefresh(() => void refresh());
</script>

<style scoped>
.remote-page { min-height: 100vh; padding: 24rpx 24rpx calc(48rpx + env(safe-area-inset-bottom)); background: #f5f6fa; color: #222533; }
.notice-card, .status-card, .permission-card { margin-bottom: 22rpx; padding: 28rpx; border-radius: 24rpx; background: #fff; box-shadow: 0 8rpx 30rpx rgba(38, 32, 74, .06); }
.notice-card { background: linear-gradient(135deg, #f0edff, #fff1f5); }
.notice-title { display: block; margin-bottom: 12rpx; font-size: 32rpx; font-weight: 900; color: #5137a5; }
.notice-copy, .privacy-copy { display: block; line-height: 1.65; font-size: 25rpx; color: #686b79; }
.status-head, .id-row, .permission-row, .section-head { display: flex; align-items: center; justify-content: space-between; }
.status-title { display: block; font-size: 31rpx; font-weight: 900; }
.status-sub { display: block; margin-top: 8rpx; font-size: 24rpx; color: #767986; }
.status-dot { width: 28rpx; height: 28rpx; border-radius: 50%; background: #c7cad2; box-shadow: 0 0 0 10rpx rgba(199, 202, 210, .2); }
.status-dot.active { background: #34b36b; box-shadow: 0 0 0 10rpx rgba(52, 179, 107, .14); }
.id-row { padding-top: 24rpx; margin-top: 24rpx; border-top: 1rpx solid #eceef3; font-size: 26rpx; color: #686b79; }
.id-value { font-size: 30rpx; font-weight: 900; color: #3c2d88; letter-spacing: 1rpx; }
.section-head { padding-bottom: 18rpx; border-bottom: 1rpx solid #eceef3; font-size: 29rpx; font-weight: 900; }
.section-head text:last-child { font-size: 22rpx; font-weight: 500; color: #999ca8; }
.permission-row { min-height: 112rpx; border-bottom: 1rpx solid #f0f1f5; }
.permission-row:last-child { border-bottom: 0; }
.permission-name, .permission-description { display: block; }
.permission-name { font-size: 27rpx; font-weight: 800; }
.permission-description { max-width: 470rpx; margin-top: 7rpx; font-size: 21rpx; color: #9699a5; }
.permission-state { color: #df5b64; font-size: 23rpx; white-space: nowrap; }
.permission-state.ok { color: #2d9e5c; }
.primary-button, .stop-button, .unbind-button { width: 100%; height: 92rpx; margin-top: 20rpx; border-radius: 46rpx; font-size: 29rpx; font-weight: 900; }
.primary-button { color: #fff; background: linear-gradient(135deg, #6c4de6, #c54a91); }
.stop-button { color: #fff; background: #e5484d; }
.unbind-button { color: #6f7280; background: #fff; border: 1rpx solid #dfe1e8; }
.privacy-copy { padding: 26rpx 12rpx 0; text-align: center; font-size: 22rpx; }
</style>
