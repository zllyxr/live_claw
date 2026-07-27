# 项目结构

| 路径 | 说明 |
| --- | --- |
| `services/core/` | Go 核心服务：彩票、体育、订单、OpenIM 网关 |
| `docker/mysql/init/02-core-rebuild.sql` | 幂等数据库迁移与旧表清理 |
| `docker/php/` | PHP/Apache 镜像和 Core/OpenIM 反向代理 |
| `admin/` | 保留的 PHP 业务和管理后台 |
| `uniapp/` | 当前客户端，体育/彩票走 Core，IM 走 OpenIM SDK |
| `game/` | 四座实时网页版捕鱼：写实 Canvas 场景、Socket.IO 权威概率结算与联机测试 |
| `docs/CORE_ARCHITECTURE.md` | 新架构、数据边界和运行约束 |
| `docker-compose.yml` | Core、OpenIM、数据库、Redis、SRS 编排 |

核心代码入口：

- `services/core/main.go`：进程生命周期和连接池。
- `services/core/http.go`：兼容 App API 与 IM 会话接口。
- `services/core/lottery_engine.go`：本地期号、开奖、审计、结算。
- `services/core/sports_collector.go`：赛程与实时比分后台采集。
- `services/core/sports_odds_collector.go`：赔率后台采集和本地盘口。
- `services/core/openim.go`：OpenIM 用户、token 和直播群组网关。
- `uniapp/src/utils/openim.ts`：多端 OpenIM SDK 封装。
