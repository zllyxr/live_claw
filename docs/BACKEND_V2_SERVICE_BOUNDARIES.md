# Backend v2 服务边界

更新时间：2026-07-28

六个程序是部署边界，不是同一个二进制的六个路由分组。共享 Go 领域包不等于共享
进程职责。

| 程序 | 唯一职责 | 对外入口 | 不允许承担 |
|---|---|---|---|
| `admin` | Layui 后台、管理员鉴权、审计、业务管理 | `/admin/`、`/admin/api/` | App API、IM、游戏循环、定时任务 |
| `api` | App 聚合/兼容 API、账号、钱包、直播、彩票/体育查询下注 | `/api/v2/`、普通 `/appapi/` | 后台页面、IM WS、游戏权威状态、定时开奖 |
| `im` | 单聊、群聊、成员/禁言、消息、WebSocket、IM Outbox | `/api/v2/im/`、`/ws/im` | 彩票/体育结算、游戏桌、后台 |
| `scheduler` | 体育同步/赛果、彩票期号/封盘/开奖/结算、对账及定时任务 | 无业务 HTTP | 用户请求、后台页面、IM 连接、游戏桌 |
| `game` | 捕鱼/扑克/麻将、300 桌与座位、机器人、权威判定与逐局结算 | `/api/v2/games`、`MiniGame.*`、`/ws/game/` | 彩票、体育、后台、IM |
| `support` | 客服会话、排队和消息 | `/api/v2/support/` | 普通 IM 群聊、后台、游戏、定时开奖 |

## Nginx 分流

- `/admin/` 只转发到 `admin`。
- `/api/v2/im/` 与 `/ws/im` 只转发到 `im`。
- `/api/v2/games`、旧 `MiniGame.*` 与 `/ws/game/` 只转发到 `game`。
- `/api/v2/support/` 只转发到 `support`。
- 其余 `/api/v2/` 和 `/appapi/` 转发到 `api`。
- `scheduler` 不接受用户流量。

## 当前实现说明

- 彩票与体育下注由 `api` 接单并通过 Wallet 冻结资金；开奖和赛果准备完成后，
  `scheduler` 负责幂等结算或退款。
- IM 写消息后写 Outbox；只有 `im` 程序消费 `im.message.created` 并发布到 Redis，
  `scheduler` 不处理 IM。
- 捕鱼三档倍率、随机桌位和结算已由 `game` 程序承载。扑克、麻将的 Go 权威规则与
  机器人仍需逐个迁移，未迁移完成前不能宣称已替代旧 Node 游戏进程。
- 体育上游供应商和彩票自动开奖规则必须通过明确配置接入；未配置时保留后台人工录入，
  不允许程序虚构赛果或开奖。
- `support` 提供独立会话/消息用户接口；`admin` 提供坐席列表、历史消息、回复和结单
  管理入口，用户端消息请求仍只进入 `support` 程序。
