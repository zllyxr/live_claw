/**
 * App 资源热更新与原生整包升级。
 *
 * WGT：后台下载、校验并安装；静默包下次启动生效，强制包安装后立即重启。
 * Native：强制版本使用不可取消弹窗，Android 下载 APK 后交给系统安装器。
 */
import CryptoJS from "crypto-js";
import { CORE_API_BASE, DEFAULT_LANGUAGE } from "@/constants/config";

interface NativeUpdateInfo {
  version_name?: string;
  version_code?: string;
  size?: string;
  sha256?: string;
  note?: string;
  force?: string;
  download_url?: string;
}

interface UpdateInfo {
  has_update?: string;
  version_name?: string;
  version_code?: string;
  size?: string;
  sha256?: string;
  note?: string;
  force?: string;
  silent?: string;
  wgt_url?: string;
  native_upgrade_required?: string;
  min_app_code?: string;
  native_update?: NativeUpdateInfo;
}

type HotUpdateOptions = {
  /** true 时把非强制、非后台静默版本也作为静默更新处理。 */
  silent?: boolean;
  /** 忽略前台检查间隔。 */
  forceCheck?: boolean;
};

const STORAGE_SKIPPED = "claw_wgt_skipped_code";
const STORAGE_DEVICE = "claw_update_device_id";
const CHECK_INTERVAL = 5 * 60 * 1000;

let activeCheck: Promise<void> | undefined;
let lastCheckedAt = 0;

function isAppPlus() {
  // #ifdef APP-PLUS
  return true;
  // #endif
  // eslint-disable-next-line no-unreachable
  return false;
}

function plusAPI() {
  return (globalThis as unknown as { plus?: any }).plus;
}

function runtime() {
  return plusAPI()?.runtime;
}

function updatePlatform() {
  return String(plusAPI()?.os?.name || "").toLowerCase() === "ios" ? "ios" : "android";
}

function updateDeviceID() {
  const nativeID = String(plusAPI()?.device?.uuid || "").trim();
  if (nativeID) return nativeID;
  const cached = String(uni.getStorageSync(STORAGE_DEVICE) || "").trim();
  if (cached) return cached;
  const created = `app_${Date.now()}_${Math.random().toString(36).slice(2, 14)}`;
  uni.setStorageSync(STORAGE_DEVICE, created);
  return created;
}

function currentWgtCode(): Promise<number> {
  return new Promise((resolve) => {
    const activeRuntime = runtime();
    if (!activeRuntime?.getProperty) {
      resolve(0);
      return;
    }
    activeRuntime.getProperty(activeRuntime.appid, (info: any) => {
      resolve(Number(info?.versionCode || 0) || 0);
    });
  });
}

function nativeAppCode() {
  return Number(runtime()?.appVersionCode || 0) || 0;
}

async function fetchUpdateInfo(
  versionCode: number,
  appCode: number
): Promise<UpdateInfo | undefined> {
  return new Promise((resolve) => {
    uni.request({
      url: CORE_API_BASE,
      method: "POST",
      header: { "Content-Type": "application/x-www-form-urlencoded" },
      data: {
        service: "App.checkUpdate",
        version_code: versionCode,
        app_code: appCode,
        platform: updatePlatform(),
        device_id: updateDeviceID(),
        language: DEFAULT_LANGUAGE
      },
      timeout: 30_000,
      success: (response) => {
        const payload = response.data as { data?: { code?: unknown; info?: UpdateInfo[] } };
        const body = payload?.data;
        resolve(String(body?.code ?? "") === "0" ? (body?.info || [])[0] : undefined);
      },
      fail: () => resolve(undefined)
    });
  });
}

function confirmUpdate(options: {
  title: string;
  content: string;
  force: boolean;
  confirmText?: string;
}) {
  return new Promise<boolean>((resolve) => {
    uni.showModal({
      title: options.title,
      content: options.content,
      showCancel: !options.force,
      confirmText: options.confirmText || "立即更新",
      cancelText: "稍后再说",
      success: (result) => resolve(Boolean(result.confirm)),
      fail: () => resolve(false)
    });
  });
}

function showProgress(prefix: string, percent = 0) {
  uni.showLoading({
    title: `${prefix} ${Math.max(0, Math.min(100, Math.round(percent)))}%`,
    mask: true
  });
}

function downloadPackage(url: string, onProgress?: (percent: number) => void) {
  return new Promise<string>((resolve, reject) => {
    const task = uni.downloadFile({
      url,
      timeout: 5 * 60 * 1000,
      success: (result) => {
        if (result.statusCode === 200 && result.tempFilePath) {
          resolve(result.tempFilePath);
          return;
        }
        reject(new Error(`下载失败（${result.statusCode || 0}）`));
      },
      fail: (error) => reject(new Error(error.errMsg || "更新包下载失败"))
    });
    task?.onProgressUpdate?.((event) => onProgress?.(Number(event.progress || 0)));
  });
}

function localFileSHA256(filePath: string): Promise<string | undefined> {
  return new Promise((resolve) => {
    const io = plusAPI()?.io;
    if (!io?.resolveLocalFileSystemURL || !io?.FileReader) {
      resolve(undefined);
      return;
    }
    io.resolveLocalFileSystemURL(
      filePath,
      (entry: any) => {
        entry.file(
          (file: any) => {
            const reader = new io.FileReader();
            reader.onloadend = (event: any) => {
              try {
                const dataURL = String(reader.result || event?.target?.result || "");
                const encoded = dataURL.includes(",") ? dataURL.slice(dataURL.indexOf(",") + 1) : "";
                resolve(
                  encoded
                    ? CryptoJS.SHA256(CryptoJS.enc.Base64.parse(encoded)).toString()
                    : undefined
                );
              } catch {
                resolve(undefined);
              }
            };
            reader.onerror = () => resolve(undefined);
            reader.readAsDataURL(file);
          },
          () => resolve(undefined)
        );
      },
      () => resolve(undefined)
    );
  });
}

async function verifyPackageSHA256(filePath: string, expectedHash: unknown) {
  const expected = String(expectedHash || "").trim().toLowerCase();
  if (!expected) return;
  const actual = await localFileSHA256(filePath);
  if (!actual) {
    throw new Error("无法校验更新包完整性");
  }
  if (actual.toLowerCase() !== expected) {
    throw new Error("更新包完整性校验失败");
  }
}

function installPackage(filePath: string, options: Record<string, unknown> = {}) {
  return new Promise<void>((resolve, reject) => {
    const activeRuntime = runtime();
    if (!activeRuntime?.install) {
      reject(new Error("当前运行环境不支持安装更新包"));
      return;
    }
    activeRuntime.install(
      filePath,
      options,
      () => resolve(),
      (error: any) => reject(new Error(error?.message || "更新包安装失败"))
    );
  });
}

function scheduleForcedRetry(message: string, retry: () => Promise<void>) {
  uni.hideLoading();
  void confirmUpdate({
    title: "必须完成更新",
    content: `${message}\n请检查网络后重试。`,
    force: true,
    confirmText: "重新更新"
  }).then((confirmed) => {
    if (confirmed) void retry();
  });
}

async function applyWgt(info: UpdateInfo, options: HotUpdateOptions) {
  if (String(info.has_update) !== "1" || !info.wgt_url) return;
  const force = String(info.force) === "1";
  const serverSilent = String(info.silent) === "1";
  const targetCode = String(info.version_code || "");

  if (!force && uni.getStorageSync(STORAGE_SKIPPED) === targetCode) return;
  if (!force && !serverSilent && !options.silent) {
    const confirmed = await confirmUpdate({
      title: `发现资源更新 ${info.version_name || ""}`,
      content: info.note || "包含体验优化与问题修复",
      force: false
    });
    if (!confirmed) {
      uni.setStorageSync(STORAGE_SKIPPED, targetCode);
      return;
    }
  }

  try {
    if (force) showProgress("强制更新", 0);
    const filePath = await downloadPackage(info.wgt_url, (percent) => {
      if (force) showProgress("强制更新", percent);
    });
    await verifyPackageSHA256(filePath, info.sha256);
    await installPackage(filePath, { force: false });
    uni.removeStorageSync(STORAGE_SKIPPED);
    uni.hideLoading();
    if (force) {
      uni.showToast({ title: "更新完成，正在重启", icon: "none", mask: true });
      setTimeout(() => runtime()?.restart?.(), 600);
    }
  } catch (error: any) {
    if (force) {
      scheduleForcedRetry(error?.message || "强制更新失败", () => applyWgt(info, options));
      return;
    }
    uni.hideLoading();
    console.warn("[hotupdate] WGT 更新失败", error);
  }
}

async function applyNativeUpdate(
  info: NativeUpdateInfo,
  force: boolean,
  options: HotUpdateOptions
) {
  const downloadURL = String(info.download_url || "");
  if (!downloadURL) return;
  if (!force && options.silent) return;

  const confirmed = await confirmUpdate({
    title: `${force ? "必须更新" : "发现新版本"} ${info.version_name || ""}`,
    content: info.note || (force ? "当前版本已停止使用，请更新后继续。" : "建议升级到最新版本。"),
    force
  });
  if (!confirmed) return;

  if (updatePlatform() === "ios") {
    runtime()?.openURL?.(downloadURL);
    return;
  }

  try {
    showProgress("下载新版", 0);
    const filePath = await downloadPackage(downloadURL, (percent) =>
      showProgress("下载新版", percent)
    );
    await verifyPackageSHA256(filePath, info.sha256);
    showProgress("准备安装", 100);
    await installPackage(filePath, { force: true });
    uni.hideLoading();
  } catch (error: any) {
    uni.hideLoading();
    if (force) {
      scheduleForcedRetry(error?.message || "新版安装失败", () =>
        applyNativeUpdate(info, true, options)
      );
      return;
    }
    uni.showToast({ title: error?.message || "新版安装失败", icon: "none" });
  }
}

async function runCheck(options: HotUpdateOptions) {
  const versionCode = await currentWgtCode();
  const info = await fetchUpdateInfo(versionCode, nativeAppCode());
  if (!info) return;

  const native = info.native_update;
  const nativeRequired =
    String(info.native_upgrade_required) === "1" || String(native?.force) === "1";
  if (native && nativeRequired) {
    await applyNativeUpdate(native, true, options);
    return;
  }

  const forcedWgt = String(info.has_update) === "1" && String(info.force) === "1";
  await applyWgt(info, options);
  if (forcedWgt) return;

  if (native) {
    await applyNativeUpdate(native, false, options);
  } else if (String(info.native_upgrade_required) === "1") {
    console.warn("[hotupdate] 当前资源需要更高原生壳，但后台尚未发布对应安装包");
  }
}

export function checkHotUpdate(options: HotUpdateOptions = {}) {
  if (!isAppPlus()) return Promise.resolve();
  if (activeCheck) return activeCheck;
  if (!options.forceCheck && Date.now() - lastCheckedAt < CHECK_INTERVAL) {
    return Promise.resolve();
  }
  lastCheckedAt = Date.now();
  activeCheck = runCheck(options)
    .catch((error) => console.warn("[hotupdate] 检查失败", error))
    .finally(() => {
      activeCheck = undefined;
    });
  return activeCheck;
}
