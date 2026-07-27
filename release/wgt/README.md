# wgt 热更新包目录

放置 uni-app 资源热更新包，容器内以只读方式挂载到 `/data/wgt`。

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
  "force": false
}
```

- `note`：更新说明，客户端可展示
- `min_app_code`：可安装该包的**最低原生壳 versionCode**。当新资源用到了新的原生模块/权限时必须设置，
  低于该值的壳会收到 `native_upgrade_required=1`，提示用户去整包升级而不是热更。
- `force`：是否强制更新（客户端不提供"稍后再说"）

## 生成方式

```bash
cd uniapp && npm run build:wgt
```

产物会自动输出到本目录。
