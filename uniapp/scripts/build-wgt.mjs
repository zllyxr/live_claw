#!/usr/bin/env node
/**
 * 生成 uni-app 资源热更新包 (.wgt)
 *
 * 用法:
 *   npm run build:wgt                  # 读 manifest.json 里的版本号打包
 *   npm run build:wgt -- --note "修复xxx"
 *   npm run build:wgt -- --min-app-code 211 --force
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
const outDir = resolve(uniappRoot, "../release/wgt");
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
const meta = {
  note: typeof arg("note") === "string" ? arg("note") : "",
  min_app_code: Number(arg("min-app-code", 0)) || 0,
  force: arg("force") !== undefined
};
writeFileSync(join(outDir, `${base}.json`), JSON.stringify(meta, null, 2) + "\n");

console.log("");
console.log(`✔ ${wgtPath}`);
console.log(`  大小   ${(size / 1024 / 1024).toFixed(2)} MB`);
console.log(`  sha256 ${sha256}`);
console.log(`  元信息 note=${meta.note || "(空)"} min_app_code=${meta.min_app_code} force=${meta.force}`);
console.log("");
console.log("服务端已挂载该目录，30 秒内自动生效。验证:");
console.log(`  curl -s -X POST http://127.0.0.1:18080/core-api/appapi/ -d 'service=App.checkUpdate&version_code=${versionCode - 1}'`);
