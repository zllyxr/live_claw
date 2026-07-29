#!/usr/bin/env node

import { readFileSync } from "node:fs";
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
  "android.permission.WAKE_LOCK"
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
if (!Number.isInteger(versionCode) || versionCode < 211) {
  console.error("原生母包 versionCode 必须为不小于 211 的整数");
  process.exit(1);
}

console.log(
  `原生母包能力校验通过：${manifest.versionName} (${manifest.versionCode})`
);
