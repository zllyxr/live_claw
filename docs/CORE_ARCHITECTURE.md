# 核心系统架构

## 数据边界

```text
体育上游 API -> Go 定时采集 -> MySQL -> Go 查询 API -> 前端
本地定时器 -> Go 彩票开奖 -> MySQL -> Go 查询 API -> 前端
业务登录 -> Go IM 会话网关 -> OpenIM Server -> OpenIM SDK
小游戏浏览器 -> 游戏服务权威判定 -> core 幂等钱包结算 -> cmf_user.coin / 钱包流水
```

- 前端请求只读取 MySQL，不在用户请求中访问体育上游。
- 彩票没有任何外部开奖源。Go 服务预生成期号、封盘、本地开奖、审计并事务结算。
- 体育 Go 后台任务分别采集实时比分、三日赛程和赛前赔率；比赛结束后从数据库状态触发事务结算。上游日额度耗尽时自动退避一小时，数据库查询与本地结算不会停机。
- 金额使用数据库整数，赔率按四位定点数计算，下注接口通过客户端订单号保证幂等。
- 私信和直播群聊统一使用 OpenIM。业务 token 只用于调用 Go 会话网关，OpenIM 管理密钥不下发到客户端。

## 服务职责

| 服务 | 职责 |
| --- | --- |
| `php` | 保留的直播、用户、支付和后台管理功能 |
| `core` | 彩票、体育、订单结算、OpenIM 用户/群组会话网关 |
| `db` / `redis` | 业务持久化、会话校验与缓存 |
| `openim-server` | IM 服务端 |
| `openim-mongo` / `openim-redis` / `openim-etcd` / `openim-kafka` / `openim-minio` | OpenIM 官方运行依赖 |
| `srs` | 直播流媒体 |
| `fishing-game` | 四座捕鱼 1000 桌自动匹配；每次开炮与捕获奖励统一调用 core 钱包 |
| `cardgames` | 斗地主/麻将 1000 桌多人实时匹配与服务端权威规则；资金统一调用 core 钱包 |

旧 PHP 彩票/体育接口、旧常驻采集脚本、本地 IM 表、腾讯 IM 签名链路和旧 Socket.IO 服务不再属于系统。`fishing-game` 使用的是隔离的新房间服务，不承担聊天、彩票或业务结算。

## 核心接口

- `GET /core-api/healthz`
- `GET|POST /core-api/appapi/?service=LotteryGame.*`
- `GET|POST /core-api/appapi/?service=Sports.*`
- `GET|POST /core-api/appapi/?service=SportsBet.*`
- `POST /core-api/v1/im/session`
- `POST /core-api/v1/im/live-session`
- `/openim-api/` 和 `/openim-ws/` 由 Apache 反向代理到 OpenIM。

## 运行约束

- `API_FOOTBALL_KEY` 必须通过根目录 `.env` 提供；为空时只停止采集，数据库查询和订单结算仍继续运行。
- `OPENIM_SECRET`、MongoDB、Redis、MinIO 密码在生产环境必须覆盖 Compose 的本地默认值。
- `OPENIM_MINIO_EXTERNAL_ADDRESS` 必须配置成客户端可访问的 HTTPS 地址。
- UniApp 原生 Android/iOS 包必须在 HBuilderX 中导入 OpenIM 官方原生插件并制作自定义基座；H5 直接使用 `@openim/client-sdk`。
