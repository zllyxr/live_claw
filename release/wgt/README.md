# wgt 热更新包目录

该目录保存本地生成的 uni-app 资源热更新包。生成后需要在管理后台
“App 管理 → 上传新版本”上传到 MinIO，并创建、发布对应版本。

## 命名规则（必须遵守）

```
<versionName>_<versionCode>.wgt        例: 8.1.1_211.wgt
<versionName>_<versionCode>.json       可选元信息，例: 8.1.1_211.json
```

`versionCode` 必须是整数且严格递增，服务端按它判断"是否有新版本"。

## 可选元信息 json

```json
{
  "note": "修复体育下注页封盘倒计时",
  "min_app_code": 210,
  "force": false,
  "silent": true
}
```

- `note`：更新说明，客户端可展示
- `min_app_code`：可安装该包的**最低原生壳 versionCode**。当新资源用到了新的原生模块/权限时必须设置，
  低于该值的壳会收到 `native_upgrade_required=1`，提示用户去整包升级而不是热更。
- `force`：是否强制更新（客户端不提供"稍后再说"）
- `silent`：是否后台无感下载安装；安装完成后在下次启动生效

## 生成方式

```bash
cd uniapp && pnpm build:wgt -- --note "本次更新说明" --silent
```

产物会自动输出到本目录。发布前必须同步递增 `src/manifest.json` 中的
`versionName` 和 `versionCode`，后台填写的版本必须与 WGT 文件名一致。
脚本默认把 `min_app_code` 设置为当前 `versionCode`，防止依赖新母包能力的
WGT 下发给旧壳；后续仅资源升级时，应显式传入最近一次母包的版本号，例如
`--min-app-code 211`。

如果更新新增了 SQLite 等原生模块、Android 权限或原生 SDK，不能只发 WGT；
必须先发布新的 APK，并把 WGT 的 `min_app_code` 设置为该 APK 的版本号。

远程协助页面依赖 `claw-rustdesk-host` AAR，首个可用母包为 **8.2.0 (216)**。
所有包含 `pages/remote/index` 的 WGT 必须设置 `min_app_code: 216`，并且只能在
书面商业授权、RustDesk 1.4.9 Host SDK PoC 和实体机验收全部通过后发布。
签名 APK 发布前还必须运行 `uniapp/scripts/verify-remote-apk.sh <apk>`，检查合并后的
Manifest、签名、仅 ARM64 ABI，以及所有原生库的 16 KB LOAD 对齐。
