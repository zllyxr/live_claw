/**
 * uni-app 资源热更新（wgt）
 *
 * 流程：启动时静默向 core 查询是否有更高 versionCode 的 wgt，
 * 有则后台下载 → plus.runtime.install 静默安装 → 下次启动生效（force 时立即重启）。
 *
 * 仅在 App 端（plus 环境）生效；H5 / 小程序直接跳过。
 */
import { CORE_API_BASE, DEFAULT_LANGUAGE } from "@/constants/config";

interface UpdateInfo {
  has_update?: string;
  version_name?: string;
  version_code?: string;
  size?: string;
  sha256?: string;
  note?: string;
  force?: string;
  wgt_url?: string;
  native_upgrade_required?: string;
  min_app_code?: string;
}

const STORAGE_SKIPPED = "claw_wgt_skipped_code";

function isAppPlus(): boolean {
  // #ifdef APP-PLUS
  return true;
  // #endif
  // eslint-disable-next-line no-unreachable
  return false;
}

function plusRuntime(): any {
  const globalPlus = (globalThis as unknown as { plus?: any }).plus;
  return globalPlus?.runtime;
}

/** 当前 wgt 资源版本号（非原生壳版本） */
function currentWgtCode(): Promise<number> {
  return new Promise((resolve) => {
    const runtime = plusRuntime();
    if (!runtime?.getProperty) {
      resolve(0);
      return;
    }
    runtime.getProperty(runtime.appid, (info: any) => {
      resolve(Number(info?.versionCode || 0) || 0);
    });
  });
}

/** 原生壳版本号，用于 min_app_code 判定 */
function nativeAppCode(): number {
  const runtime = plusRuntime();
  return Number(runtime?.appVersionCode || 0) || 0;
}

async function fetchUpdateInfo(versionCode: number, appCode: number): Promise<UpdateInfo | undefined> {
  return new Promise((resolve) => {
    uni.request({
      url: CORE_API_BASE,
      method: "POST",
      header: { "Content-Type": "application/x-www-form-urlencoded" },
      data: {
        service: "App.checkUpdate",
        version_code: versionCode,
        app_code: appCode,
        language: DEFAULT_LANGUAGE
      },
      timeout: 8000,
      success: (res) => {
        const payload = res.data as { data?: { code?: unknown; info?: UpdateInfo[] } };
        const body = payload?.data;
        if (String(body?.code ?? "") !== "0") {
          resolve(undefined);
          return;
        }
        resolve((body?.info || [])[0]);
      },
      fail: () => resolve(undefined)
    });
  });
}

function downloadWgt(url: string, onProgress?: (percent: number) => void): Promise<string | undefined> {
  return new Promise((resolve) => {
    const task = uni.downloadFile({
      url,
      timeout: 120000,
      success: (res) => resolve(res.statusCode === 200 ? res.tempFilePath : undefined),
      fail: () => resolve(undefined)
    });
    if (onProgress && task?.onProgressUpdate) {
      task.onProgressUpdate((event) => onProgress(event.progress));
    }
  });
}

function installWgt(filePath: string, force: boolean): Promise<boolean> {
  return new Promise((resolve) => {
    const runtime = plusRuntime();
    if (!runtime?.install) {
      resolve(false);
      return;
    }
    runtime.install(
      filePath,
      { force: false },
      () => {
        if (force) {
          uni.showModal({
            title: "更新完成",
            content: "需要重启应用以应用本次更新",
            showCancel: false,
            confirmText: "立即重启",
            success: () => runtime.restart()
          });
        }
        resolve(true);
      },
      () => resolve(false)
    );
  });
}

/**
 * 静默检查并应用热更新。
 * @param options.silent true 时不弹任何提示（默认），false 时非强制更新也会询问用户
 */
export async function checkHotUpdate(options: { silent?: boolean } = {}): Promise<void> {
  if (!isAppPlus()) {
    return;
  }
  const silent = options.silent !== false;

  try {
    const versionCode = await currentWgtCode();
    const appCode = nativeAppCode();
    const info = await fetchUpdateInfo(versionCode, appCode);
    if (!info) {
      return;
    }

    if (String(info.native_upgrade_required) === "1") {
      // 新资源依赖更高原生壳，热更新无法覆盖，交由整包升级流程处理
      console.info("[hotupdate] 需要整包升级，min_app_code=", info.min_app_code);
      return;
    }
    if (String(info.has_update) !== "1" || !info.wgt_url) {
      return;
    }

    const force = String(info.force) === "1";
    const targetCode = String(info.version_code || "");

    // 用户已跳过该版本且非强制时不再打扰
    if (!force && uni.getStorageSync(STORAGE_SKIPPED) === targetCode) {
      return;
    }

    if (!silent && !force) {
      const confirmed = await new Promise<boolean>((resolve) => {
        uni.showModal({
          title: `发现新版本 ${info.version_name || ""}`,
          content: info.note || "包含体验优化与问题修复",
          confirmText: "立即更新",
          cancelText: "稍后再说",
          success: (res) => resolve(Boolean(res.confirm)),
          fail: () => resolve(false)
        });
      });
      if (!confirmed) {
        uni.setStorageSync(STORAGE_SKIPPED, targetCode);
        return;
      }
    }

    const filePath = await downloadWgt(info.wgt_url);
    if (!filePath) {
      return;
    }

    const installed = await installWgt(filePath, force);
    if (installed) {
      uni.removeStorageSync(STORAGE_SKIPPED);
      console.info("[hotupdate] 已安装 wgt", targetCode, "下次启动生效");
    }
  } catch (error) {
    // 热更新失败绝不能影响应用启动
    console.warn("[hotupdate] 检查失败", error);
  }
}
