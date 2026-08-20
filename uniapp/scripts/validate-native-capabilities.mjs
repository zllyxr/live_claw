#!/usr/bin/env node

import { existsSync, readFileSync, statSync } from "node:fs";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";

const here = dirname(fileURLToPath(import.meta.url));
const manifestPath = join(here, "../src/manifest.json");
const raw = readFileSync(manifestPath, "utf8");
const manifest = JSON.parse(
  raw
    .replace(/\/\*[\s\S]*?\*\//g, "")
    .replace(/(^|[^:"'\\])\/\/.*$/gm, "$1")
);

const modules = manifest["app-plus"]?.modules || {};
const android = manifest["app-plus"]?.distribute?.android || {};
const iosDescriptions =
  manifest["app-plus"]?.distribute?.ios?.privacyDescription || {};
const permissions = Array.isArray(android.permissions)
  ? android.permissions.join("\n")
  : "";

const requiredModules = [
  "Audio",
  "Camera",
  "Record",
  "SQLite",
  "VideoPlayer"
];
const requiredPermissions = [
  "android.permission.ACCESS_NETWORK_STATE",
  "android.permission.ACCESS_WIFI_STATE",
  "android.permission.CAMERA",
  "android.permission.INTERNET",
  "android.permission.MODIFY_AUDIO_SETTINGS",
  "android.permission.READ_MEDIA_IMAGES",
  "android.permission.READ_MEDIA_VIDEO",
  "android.permission.RECORD_AUDIO",
  "android.permission.REQUEST_INSTALL_PACKAGES",
  "android.permission.VIBRATE",
  "android.permission.WAKE_LOCK",
  "android.permission.FOREGROUND_SERVICE",
  "android.permission.FOREGROUND_SERVICE_MEDIA_PROJECTION",
  "android.permission.POST_NOTIFICATIONS"
];
const requiredIOSDescriptions = [
  "NSCameraUsageDescription",
  "NSMicrophoneUsageDescription",
  "NSPhotoLibraryAddUsageDescription",
  "NSPhotoLibraryUsageDescription"
];

const missing = [
  ...requiredModules
    .filter((name) => !Object.prototype.hasOwnProperty.call(modules, name))
    .map((name) => `app-plus.modules.${name}`),
  ...requiredPermissions
    .filter((name) => !permissions.includes(name))
    .map((name) => `Android permission ${name}`),
  ...requiredIOSDescriptions
    .filter((name) => !String(iosDescriptions[name] || "").trim())
    .map((name) => `iOS privacyDescription.${name}`)
];

if (missing.length) {
  console.error("原生母包能力校验失败：");
  for (const item of missing) {
    console.error(`- 缺少 ${item}`);
  }
  process.exit(1);
}

const versionCode = Number(manifest.versionCode || 0);
if (!Number.isInteger(versionCode) || versionCode < 216) {
  console.error("远程协助原生母包 versionCode 必须为不小于 216 的整数");
  process.exit(1);
}

if (Number(android.minSdkVersion || 0) < 29 || Number(android.targetSdkVersion || 0) !== 36) {
  console.error("远程协助母包必须使用 minSdk 29 和 targetSdk 36");
  process.exit(1);
}

const aarPath = join(
  here,
  "../src/uni_modules/claw-rustdesk-host/utssdk/app-android/libs/claw-remote-host-1.0.0.aar"
);
if (!existsSync(aarPath) || statSync(aarPath).size < 30_000) {
  console.error("远程协助 Host SDK AAR 不存在或无效；请运行 native/rustdesk-host-sdk/scripts/build-host-sdk.sh");
  process.exit(1);
}

console.log(
  `原生母包能力校验通过：${manifest.versionName} (${manifest.versionCode})`
);
