#!/usr/bin/env node
/**
 * 生成 uni-app 资源热更新包 (.wgt)
 *
 * 用法:
 *   npm run build:wgt                  # 读 manifest.json 里的版本号打包
 *   npm run build:wgt -- --note "修复xxx"
 *   npm run build:wgt -- --min-app-code 211 --force --silent
 *   npm run build:wgt -- --skip-build --output-dir /tmp/wgt-check
 *
 * 产物:
 *   ../release/wgt/<versionName>_<versionCode>.wgt
 *   ../release/wgt/<versionName>_<versionCode>.json   (含 note / min_app_code / force)
 *
 * 说明:
 *   wgt 本质是 app-plus 构建产物目录的 zip 包（内含 manifest.json）。
 *   只能更新 JS/CSS/静态资源；新增原生模块或权限必须发整包并配 min_app_code。
 */
import { execFileSync } from "node:child_process";
import { existsSync, mkdirSync, readFileSync, rmSync, writeFileSync, statSync } from "node:fs";
import { createHash } from "node:crypto";
import { dirname, join, resolve } from "node:path";
import { fileURLToPath } from "node:url";

const here = dirname(fileURLToPath(import.meta.url));
const uniappRoot = resolve(here, "..");
const buildDir = join(uniappRoot, "dist/build/app");

function arg(name, fallback = undefined) {
  const index = process.argv.indexOf(`--${name}`);
  if (index === -1) return fallback;
  const next = process.argv[index + 1];
  return next && !next.startsWith("--") ? next : true;
}

function readManifest() {
  const raw = readFileSync(join(uniappRoot, "src/manifest.json"), "utf8");
  // manifest.json 允许注释，解析前先剥掉
  const stripped = raw
    .replace(/\/\*[\s\S]*?\*\//g, "")
    .replace(/(^|[^:"'\\])\/\/.*$/gm, "$1");
  return JSON.parse(stripped);
}

function run(command, args, cwd) {
  execFileSync(command, args, { cwd, stdio: "inherit" });
}

const outDir = resolve(uniappRoot, String(arg("output-dir", "../release/wgt")));
const manifest = readManifest();
const versionName = String(manifest.versionName || "").trim();
const versionCode = Number(manifest.versionCode || 0);

if (!versionName || !Number.isInteger(versionCode) || versionCode <= 0) {
  console.error("✗ manifest.json 的 versionName / versionCode 无效");
  process.exit(1);
}

console.log(`▶ 打包 wgt: ${versionName} (versionCode=${versionCode})`);

// 1. 构建 app-plus 资源
if (arg("skip-build") !== undefined) {
  console.log("· 跳过构建，直接使用现有 dist/build/app");
} else {
  console.log("· 构建 app-plus 资源…");
  rmSync(buildDir, { recursive: true, force: true });
  run(process.platform === "win32" ? "npx.cmd" : "npx", ["uni", "build", "-p", "app"], uniappRoot);
}

if (!existsSync(join(buildDir, "manifest.json"))) {
  console.error(`✗ 构建产物缺少 manifest.json: ${buildDir}`);
  console.error("  请确认 uni build -p app 成功执行");
  process.exit(1);
}

const builtManifest = JSON.parse(readFileSync(join(buildDir, "manifest.json"), "utf8"));
const builtVersionName = String(builtManifest?.version?.name || "").trim();
const builtVersionCode = Number(builtManifest?.version?.code || 0);
if (
  String(builtManifest?.id || "") !== String(manifest.appid || "") ||
  builtVersionName !== versionName ||
  builtVersionCode !== versionCode
) {
  console.error("✗ App 构建产物与 src/manifest.json 不一致");
  console.error(
    `  源码 ${manifest.appid} ${versionName} (${versionCode})；` +
      `产物 ${builtManifest?.id || "(无)"} ${builtVersionName || "(无)"} (${builtVersionCode || 0})`
  );
  process.exit(1);
}

// 2. 打成 wgt（zip，内容位于包根，不含顶层目录）
mkdirSync(outDir, { recursive: true });
const base = `${versionName}_${versionCode}`;
const wgtPath = join(outDir, `${base}.wgt`);
rmSync(wgtPath, { force: true });

console.log("· 压缩为 wgt…");
run("zip", ["-q", "-r", "-X", wgtPath, "."], buildDir);

// 3. 写元信息
const size = statSync(wgtPath).size;
const sha256 = createHash("sha256").update(readFileSync(wgtPath)).digest("hex");
const minAppCode = Number(arg("min-app-code", versionCode));
if (!Number.isInteger(minAppCode) || minAppCode < 1) {
  console.error("✗ min_app_code 必须是正整数");
  process.exit(1);
}
const meta = {
  note: typeof arg("note") === "string" ? arg("note") : "",
  min_app_code: minAppCode,
  force: arg("force") !== undefined,
  silent: arg("silent") !== undefined,
  size,
  sha256
};
writeFileSync(join(outDir, `${base}.json`), JSON.stringify(meta, null, 2) + "\n");

console.log("");
console.log(`✔ ${wgtPath}`);
console.log(`  大小   ${(size / 1024 / 1024).toFixed(2)} MB`);
console.log(`  sha256 ${sha256}`);
console.log(`  元信息 note=${meta.note || "(空)"} min_app_code=${meta.min_app_code} force=${meta.force} silent=${meta.silent}`);
console.log("");
console.log("下一步：在管理后台「App 管理」上传该 WGT，填写版本信息并发布。");
