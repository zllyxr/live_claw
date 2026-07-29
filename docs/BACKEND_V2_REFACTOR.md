# Backend v2 重构基线

更新时间：2026-07-28

## 已确认的边界

- 运行时只允许 Nginx、Go、MySQL 8、Redis 7、MinIO 和 Layui。
- PHP/Apache、Node 游戏服务、OpenIM、MongoDB、Kafka、etcd、SRS 和 Python 在 v2 切流后全部退出。
- 当前 UniApp 中仍可见的登录、资料、关注、动态、直播互动、资金、IM、体育、彩票和游戏功能继续兼容。
- 新建独立 `claw_v2` 数据库并行迁移；旧库只读归档，禁止直接在生产旧库上批量 DROP。

## 目标架构

```text
Nginx
  ├─ /h5/、/minigame/               静态资源
  ├─ /api/v2/、/appapi/             Go API + 旧接口兼容层
  ├─ /admin/                        Go 模板 + Layui
  └─ /ws/im、/ws/game/*             Go WebSocket

Go services（六个独立程序）
  ├─ admin       Layui 后台与管理员 API
  ├─ api         App API、旧接口兼容与页面聚合接口
  ├─ im          单聊、群聊、WebSocket、IM Outbox
  ├─ scheduler   体育同步/赛果、彩票期号/开奖/结算、定时任务
  ├─ game        捕鱼、扑克、麻将、桌位和机器人权威状态
  └─ support     客服会话与客服消息

MySQL        业务真相、账本、历史、审计
Redis        会话、缓存、在线状态、匹配队列、短期桌状态
MinIO        头像、动态媒体、IM 附件、App/WGT 包
```

六个 Go 程序可共享领域包和同一套 MySQL/Redis/MinIO 基础设施，但不共享 HTTP
入口、后台路由、WebSocket 连接或定时循环。每个程序独立构建、独立容器化、独立
健康检查和扩缩容；Nginx 只按路径或旧接口 `service` 名分流。Redis 负责 IM
投递、游戏节点租约、匹配队列和短期桌状态。

## 接口策略

- 新接口统一位于 `/api/v2`，JSON 返回 `request_id/code/message/data`。
- `/appapi/?service=Module.Method` 保留为兼容入口，内部调用同一应用服务并转换成旧 `data.code/msg/info` 格式，不复制业务逻辑。
- 首页新增 `GET /api/v2/home` 和兼容方法 `Home.dashboard`。一次返回用户摘要、轮播、精选、3 场赛事及赔率、彩票推荐、捕鱼入口和首屏直播。
- 首页各区块返回独立的 `status/error/updated_at`；公共数据分区缓存，用户余额单独拼装，单区失败不拖垮整个响应。
- 所有写接口接受 `Idempotency-Key`；支付回调、提现审核、下注、礼物、红包和游戏结算必须幂等。

## 资金与游戏不变量

- 星币金额只用 `int64`；法币用最小单位和 `currency_scale`，禁止浮点计算。
- `wallet` 是余额唯一写入口。充值、提现、彩票、体育、礼物、红包、后台调账和游戏都写同一不可变流水。
- 每笔流水保存 `business_type/business_id/round_id/table_id`，后台可精确查询每一场游戏的投入、派彩、净输赢和结算前后余额。
- 捕鱼进入桌时冻结托管额度，炮弹与捕获写严格递增的桌内资金事件；退出、超时或结算周期到达时一次写回平台账本。
- 捕鱼固定三个场：新手场 `x1`、高手场 `x5`、大师场 `x10`。每场 300 桌、每桌 4 座；用户选择场次，服务端从未满桌随机选桌并从空座随机分配，断线重连保留原桌原座。
- 三个场的最低余额、单次托管额、炮值集合、RTP、限额均由后台配置；倍率同时作用于炮弹成本和捕获奖励。

## 邀请码与团队

- 邀请码固定小写 `xxx-xxxx`，字符集为 `0-9a-z`。
- 前三位是唯一且不可变的团队码；后四位是团队内唯一的个人码；完整邀请码全局唯一。
- 团队码默认由系统随机生成，后台可在创建团队时指定尚未占用的码；团队创建后不可修改。
- 用户只属于一个团队。注册时绑定邀请人即加入邀请人团队；未绑定用户进入系统默认团队。
- 旧邀请码迁移时保留旧码别名 180 天用于解析，绑定后写入新关系；新页面只展示新码。

## 上线与回滚

1. 创建 `claw_v2`，执行新 schema，旧系统继续提供服务。
2. 全量迁移用户、余额、订单、有效内容和配置；旧表只读扫描，不在迁移程序中 DROP。
3. 对余额、充值、提现、投注、游戏结算逐用户与逐日对账，差异不为零不得切流。
4. Nginx 先按内部账号/灰度标记路由到 v2；写操作在单一系统完成，不做双写。
5. 切流后保留旧库和旧容器的离线镜像，观察至少 14 天；回滚只切 Nginx 路由。
6. 观察期结束并完成备份校验后，另行生成旧表删除清单，必须人工确认后执行。

## 当前本地实现状态（2026-07-28）

- 新库 migrations 已到 `0009`，覆盖系统/用户、团队邀请码、资金、游戏、抖音直播、
  原生 IM、彩票体育、App、内容指标与后台 RBAC。
- 六个 Go 程序、MinIO、MySQL、Redis、Nginx 和嵌入式 Layui 后台已能以
  `backend/deploy/local/compose.yml` 完整启动。
- 彩票和体育投注均使用冻结资金，Scheduler 负责派奖或退款；账本保留
  `game_code/venue_code/table_no/round_no`。
- 原生 IM 已覆盖单聊、群聊、成员/角色/禁言、入群审批、黑名单、撤回与 WebSocket，
  UniApp 已移除 OpenIM SDK 运行时依赖。
- 用户媒体由 Go 鉴权后写入 MinIO；Nginx multipart 上限与 Go 的 50 MiB 限制一致。
- 本地已通过 Go 全量测试、真实 MySQL/Redis/MinIO 集成测试、`go vet`、`-race`、
  UniApp 类型检查、H5 正式构建和 Docker 网关冒烟。

生产切流仍必须先完成旧库全量导入、余额/订单/逐局流水对账和灰度验证；当前本地实现
不会自动删除、修改或双写旧库。
