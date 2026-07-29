# Backend v2 接口对齐状态

更新时间：2026-07-29

本文只记录已经由本地代码和请求验证的状态。“有数据表”不等于“接口已对齐”，
“有用户接口”也不等于“后台已可管理”。

## 已完成的核心链路

| 业务 | App/兼容接口 | 后台入口 | 独立程序 |
|---|---|---|---|
| 聚合首页 | `GET /api/v2/home`、`Home.dashboard` | 数据统计 | `api` |
| 彩票 | `LotteryGame.home/detail/currentIssue/issueHistory/bet/orderList` | 彩票管理：分类、游戏、玩法、选项、期号、封盘、开奖 | 查询/下注在 `api`，期号/开奖/结算在 `scheduler` |
| 体育 | `Sports.home/matchDetail`、`SportsBet.matchMarkets/bet/orderList/recordList` | 体育管理：赛事、盘口、赔率、赛果、提交结算 | 查询/下注在 `api`，同步/赛果/结算在 `scheduler` |
| 投注 | 彩票/体育下注与订单接口，游戏逐场资金流水 | 投注管理：彩票订单、体育订单、游戏逐场结算 | `api` + `game` + `scheduler` |
| 捕鱼 | `GET /api/v2/games`、捕鱼 enter/leave、`MiniGame.list/enter` | 游戏管理、逐场结算 | `game` |
| IM | `/api/v2/im/*`、`/ws/im` | IM 管理 | `im` |
| 客服 | `/api/v2/support/conversations/*` | 客服会话、历史消息、坐席回复、优先级和结单 | `support`（用户端）+ `admin`（坐席端） |

本地已验证以下后台接口均返回 `code=0`：

- `GET /admin/api/dashboard`
- `GET /admin/api/lottery/catalog`
- `GET /admin/api/sports/matches`
- `GET /admin/api/bets/dashboard`

后台页面已验证无加载错误：

- `#lottery`：彩票游戏、期号、玩法、选项、分类。
- `#sports`：赛事列表及盘口/赔率维护入口。
- `#bets`：彩票投注、体育投注、游戏逐场结算。

## 当前 UniApp 接口对齐结论

当前 `uniapp/src/api` 中的业务调用均已指向本地 Go 服务：

- 普通兼容接口统一由 `api` 的 `/appapi/` 承载。
- `MiniGame.list/enter` 只转发到独立 `game` 程序。
- IM HTTP 与 WebSocket 只进入独立 `im` 程序。
- 客服 HTTP 只进入独立 `support` 程序。
- 体育详情、资金明细、认证状态和充值协议均为原生页面，不再打开旧后台页面。
- 用户资料、认证、任务、社交、视频、直播互动、红包、资金和邀请功能均已有 Go 接口。

## 依赖外部配置的功能

代码链路已接通，但本地没有以下生产凭据或业务数据，因此不会伪造内容：

- 体育实时赛事与赔率需要 `V2_SPORTS_API_KEY`。
- 热门直播需要在后台录入并审核真实抖音直播间。
- 充值需要配置并启用真实支付渠道；未配置时前端明确显示“当前没有可用支付方式”。
- App 安装包/WGT 更新需要先上传 MinIO 并由后台发布版本。

扑克、麻将的 Go 权威规则、断线恢复和机器人不属于当前 UniApp 已开放入口，仍是后续
游戏服务扩展项；捕鱼三档倍率、300 桌 × 4 座和随机分配已经实现。
