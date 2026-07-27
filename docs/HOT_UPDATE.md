# uni-app 资源热更新（wgt）

整理日期: 2026-07-25

App 端无需重新上架即可更新 JS/CSS/静态资源。原生模块、权限、启动图等变更仍需发整包。

## 1. 链路总览

```
uniapp 源码
   │  npm run build:wgt
   ▼
release/wgt/<versionName>_<versionCode>.wgt   ← 容器只读挂载到 /data/wgt
   │
   ├── core-go: App.checkUpdate               ← 客户端查询是否有新版本
   └── core-go: /appapi/hotupdate/download    ← 客户端下载 wgt
   │
   ▼
App 启动静默检查 → 下载 → plus.runtime.install → 下次启动生效
```

## 2. 发布一个热更新

```bash
# 1. 改代码后，提升 uniapp/src/manifest.json 的 versionCode（必须严格递增）
#    versionName 用于展示，versionCode 用于比较

# 2. 打包（会自动执行 uni build -p app）
cd uniapp
npm run build:wgt -- --note "修复体育下注页封盘倒计时"

# 3. 校验服务端已发现新版本（模拟旧客户端）
curl -s -X POST http://127.0.0.1:18080/core-api/appapi/ \
  -d 'service=App.checkUpdate&version_code=<旧versionCode>&app_code=<旧versionCode>' | python3 -m json.tool
```

产物同时写出 `<版本>.json` 元信息，可手工编辑后生效（服务端 30 秒缓存）。

常用参数：

| 参数 | 说明 |
| --- | --- |
| `--note "文案"` | 更新说明，客户端非静默模式会展示 |
| `--min-app-code 211` | 该资源包要求的**最低原生壳 versionCode** |
| `--force` | 强制更新，安装完立即提示重启 |
| `--skip-build` | 跳过构建，直接用现有 `dist/build/app` 打包 |

## 3. 接口契约

### App.checkUpdate

`POST /core-api/appapi/`，参数 `service=App.checkUpdate`：

| 参数 | 含义 |
| --- | --- |
| `version_code` | 客户端当前 **wgt 资源** versionCode |
| `app_code` | 客户端**原生壳** versionCode（缺省取 version_code） |

返回 `data.info[0]`：

```json
{
  "has_update": "1",
  "version_name": "8.1.1",
  "version_code": "211",
  "size": "1822710",
  "sha256": "8eb779de…",
  "note": "修复xxx",
  "force": "0",
  "wgt_url": "http://…/core-api/appapi/hotupdate/download?file=8.1.1_211.wgt",
  "current_code": "210",
  "server_time": "1784976025"
}
```

无更新时 `has_update=0`。若存在更高版本但客户端壳太旧，返回 `native_upgrade_required=1` 与 `min_app_code`，此时**不应热更**，应引导整包升级。

### 下载

`GET /core-api/appapi/hotupdate/download?file=<名称>.wgt`

已做防护：仅允许 `.wgt` 后缀、拒绝目录穿越（实测 `../../etc/passwd` 返回 400）。

## 4. 客户端行为

代码位置 [uniapp/src/utils/hotupdate.ts](../uniapp/src/utils/hotupdate.ts)，在 `App.vue` 的 `onLaunch` 中调用：

- 仅 App 端生效（H5/小程序自动跳过）
- **默认静默**：后台下载并安装，下次启动生效，用户无感
- `force=1`：安装后弹窗提示立即重启
- 非强制且用户选择"稍后再说"：记录该 versionCode，不再重复打扰
- 任何环节失败都只打日志，**绝不阻塞启动**

需要改成"询问用户"模式：

```ts
void checkHotUpdate({ silent: false });
```

## 5. 服务端配置

| 环境变量 | 默认值 | 说明 |
| --- | --- | --- |
| `CORE_HOTUPDATE_DIR` | `/data/wgt` | 容器内 wgt 目录 |
| `CORE_HOTUPDATE_BASE_URL` | `http://127.0.0.1:18080/core-api` | 拼接下载地址的对外前缀 |

**上线注意**：`CORE_HOTUPDATE_BASE_URL` 必须改为真实对外域名（含 https），否则客户端拿到的 `wgt_url` 指向 127.0.0.1 下载不到。在 `.env` 里设置：

```
CORE_HOTUPDATE_BASE_URL=https://your-domain.com/core-api
```

## 6. 注意事项

- `versionCode` 必须整数且严格递增，服务端只按它比较。
- wgt **只能**替换前端资源。新增原生模块（如新 SDK）、改权限、改 appid/包名必须发整包，并给新 wgt 配 `min_app_code`。
- 服务端目录扫描有 30 秒缓存，刚放进去的包最多等 30 秒生效。
- wgt 内容必须位于包根（`manifest.json` 在第一层），打包脚本已保证。
- 客户端安装用 `plus.runtime.install(..., {force:false})`，失败不影响现有版本。
