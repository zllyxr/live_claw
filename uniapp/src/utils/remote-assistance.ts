import type { RemoteEnrollment, RemotePermissionStatus } from "@/api/remote";
import { unbindRemoteDevice } from "@/api/remote";
import { API_HOST } from "@/constants/config";
import * as remoteHost from "@/uni_modules/claw-rustdesk-host";
import { t } from "@/i18n";

const INSTALL_ID_KEY = "claw_remote_install_id";

export interface NativeRemoteStatus {
  available: boolean;
  running: boolean;
  device_code?: string;
  service_status: string;
  permissions: RemotePermissionStatus;
  message?: string;
}

interface NativeRemotePlugin {
  initialize(options: Record<string, unknown>, callback: (result: NativeRemoteStatus) => void): void;
  start(callback: (result: NativeRemoteStatus) => void): void;
  stop(options: { clear_credentials?: boolean }, callback: (result: NativeRemoteStatus) => void): void;
  getStatus(callback: (result: NativeRemoteStatus) => void): void;
  openPermissionSettings(permission: string, callback: (result: NativeRemoteStatus) => void): void;
}

function nativePlugin(): NativeRemotePlugin | undefined {
  return remoteHost as unknown as NativeRemotePlugin;
}

function invoke(method: keyof NativeRemotePlugin, ...args: unknown[]): Promise<NativeRemoteStatus> {
  const plugin = nativePlugin();
  if (!plugin || typeof plugin[method] !== "function") {
    return Promise.resolve({
      available: false, running: false, service_status: "unsupported", permissions: {},
      message: t("core.remoteComponentMissing")
    });
  }
  return new Promise((resolve, reject) => {
    const callback = (result: NativeRemoteStatus) => resolve(result || {
      available: false, running: false, service_status: "error", permissions: {}
    });
    try {
      (plugin[method] as (...values: unknown[]) => void)(...args, callback);
    } catch (error) {
      reject(error);
    }
  });
}

export function getRemoteInstallId() {
  const existing = String(uni.getStorageSync(INSTALL_ID_KEY) || "").trim();
  if (existing.length >= 16) return existing;
  const random = Array.from({ length: 32 }, () => Math.floor(Math.random() * 16).toString(16)).join("");
  const installId = `android-${Date.now().toString(36)}-${random}`;
  uni.setStorageSync(INSTALL_ID_KEY, installId);
  return installId;
}

export function nativeDeviceMetadata() {
  const info = uni.getSystemInfoSync();
  const app = typeof uni.getAppBaseInfo === "function" ? uni.getAppBaseInfo() : {};
  return {
    device_name: String(info.deviceModel || info.model || "Android device"),
    manufacturer: String(info.deviceBrand || info.brand || ""),
    model: String(info.deviceModel || info.model || ""),
    android_version: String(info.osVersion || info.system || ""),
    android_sdk: Number((info as unknown as { osAndroidAPILevel?: number }).osAndroidAPILevel || 29),
    app_version: String((app as { appVersion?: string }).appVersion || ""),
    app_native_code: Number((app as { appVersionCode?: string }).appVersionCode || 0),
    plugin_version: "1.0.0"
  };
}

export function initializeRemoteNative(enrollment: RemoteEnrollment) {
  return invoke("initialize", {
    backend_url: `${API_HOST.replace(/\/$/, "")}/api/v2`,
    device_id: enrollment.device_id,
    device_token: enrollment.device_token,
    notification_title: t("core.remoteRunning")
  });
}

export function getRemoteNativeStatus() {
  return invoke("getStatus");
}

export function startRemoteNative() {
  return invoke("start");
}

export function stopRemoteNative(clearCredentials = false) {
  return invoke("stop", { clear_credentials: clearCredentials });
}

export function openRemotePermissionSettings(permission: string) {
  return invoke("openPermissionSettings", permission);
}

export function cleanupRemoteOnSessionEnd(session: { uid: string; token: string }) {
  const installId = getRemoteInstallId();
  void stopRemoteNative(true);
  void unbindRemoteDevice(installId, session).catch(() => undefined);
}
