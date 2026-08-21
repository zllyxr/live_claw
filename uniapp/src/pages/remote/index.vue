<template>
  <view class="remote-page">
    <view class="notice-card">
      <text class="notice-title">{{ t("misc.remote.noticeTitle") }}</text>
      <text class="notice-copy">{{ t("misc.remote.noticeDescription") }}</text>
    </view>

    <view class="status-card">
      <view class="status-head">
        <view>
          <text class="status-title">{{ t("misc.remote.status") }}</text>
          <text class="status-sub">{{ statusText }}</text>
        </view>
        <view class="status-dot" :class="{ active: running }" />
      </view>
      <view class="id-row">
        <text>{{ t("misc.remote.deviceCode") }}</text>
        <text class="id-value">{{ deviceCode || t("misc.remote.notAssigned") }}</text>
      </view>
      <view class="id-row">
        <text>{{ t("misc.remote.serverConnection") }}</text>
        <text>{{ serverOnline ? t("misc.remote.online") : t("misc.remote.offline") }}</text>
      </view>
    </view>

    <view class="permission-card">
      <view class="section-head">
        <text>{{ t("misc.remote.permissions") }}</text>
        <text>{{ t("misc.remote.tapSettings") }}</text>
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
        <text class="permission-state" :class="{ ok: item.granted }">{{ item.granted ? t("misc.remote.enabled") : t("misc.remote.disabled") }} ›</text>
      </view>
    </view>

    <button v-if="!running" class="primary-button" :loading="busy" :disabled="busy" @tap="enableRemote">
      {{ t("misc.remote.enable") }}
    </button>
    <button v-else class="stop-button" :loading="busy" :disabled="busy" @tap="stopRemote">
      {{ t("misc.remote.stop") }}
    </button>
    <button v-if="enrolled" class="unbind-button" :disabled="busy" @tap="unbindRemote">{{ t("misc.remote.unbind") }}</button>

    <text class="privacy-copy">{{ t("misc.remote.privacy") }}</text>
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
import { t } from "@/i18n";

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
  if (!nativeStatus.value.available) return nativeStatus.value.message || t("misc.remote.notSupported");
  if (running.value) return t("misc.remote.running");
  if (enrolled.value) return t("misc.remote.boundNotRunning");
  return t("misc.remote.notEnabled");
});

type PermissionKey = "notification" | "media_projection" | "accessibility" | "battery";

const permissionItems = computed(() => {
  const definitions: Array<[PermissionKey, string, string]> = [
    ["notification", t("misc.remote.foregroundNotification"), t("misc.remote.foregroundNotificationDesc")],
    ["media_projection", t("misc.remote.screenSharing"), t("misc.remote.screenSharingDesc")],
    ["accessibility", t("misc.remote.accessibility"), t("misc.remote.accessibilityDesc")],
    ["battery", t("misc.remote.batteryWhitelist"), t("misc.remote.batteryWhitelistDesc")]
  ];
  return definitions.map(([key, name, description]) => ({
    key, name, description, granted: Boolean(nativeStatus.value.permissions?.[key])
  }));
});

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
    if (!initialized.available) throw new Error(initialized.message || t("misc.remote.componentMissing"));
    nativeStatus.value = await startRemoteNative();
    enrolled.value = true;
    uni.showToast({ title: t("misc.remote.confirmScreenSharing"), icon: "none" });
    setTimeout(() => void refresh(), 1500);
  } catch (error: any) {
    if (enrolledDuringAttempt) {
      await stopRemoteNative(true).catch(() => undefined);
      await unbindRemoteDevice(getRemoteInstallId()).catch(() => undefined);
      enrolled.value = false;
    }
    uni.showModal({ title: t("misc.remote.cannotEnable"), content: error?.message || t("misc.remote.unavailable"), showCancel: false });
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
    uni.showToast({ title: t("misc.remote.stopped"), icon: "none" });
  } finally {
    busy.value = false;
  }
}

function unbindRemote() {
  if (busy.value) return;
  uni.showModal({
    title: t("misc.remote.unbind"),
    content: t("misc.remote.unbindConfirm"),
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
        uni.showToast({ title: error?.message || t("misc.remote.unbindFailed"), icon: "none" });
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
    uni.showToast({ title: error?.message || t("misc.remote.openSettingsFailed"), icon: "none" });
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
