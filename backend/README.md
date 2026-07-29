# Claw Backend

这是项目唯一的后端实现，严格基于 Nginx、Go、MySQL、Redis、MinIO 和 Layui。
旧 PHP、Apache、Composer、OpenIM 与 Node 游戏运行时均已移除。

运行程序（六个程序独立构建、独立容器、独立健康检查）：

- `cmd/admin`：Layui 管理后台及 `/admin/api`，不承载 App API。
- `cmd/api`：用户 API、首页聚合、资金/直播/彩票/体育及旧接口兼容层。
- `cmd/im`：单聊、群聊、WebSocket 与 IM Outbox 投递。
- `cmd/scheduler`：体育同步/赛果、彩票期号/开奖/结算及其他定时任务。
- `cmd/game`：捕鱼、扑克、麻将等权威游戏状态、桌位匹配和机器人。
- `cmd/support`：独立客服会话与消息接口。
- `internal/game`：捕鱼三档倍率、随机桌位匹配与权威结算服务。
- `internal`：按领域拆分的应用服务。
- `migrations`：全新的 `claw_v2` schema。
- `internal/admin/web`：嵌入 Go 二进制的 Layui 后台模板和静态资源。

当前已落地：

- 单请求聚合首页、旧 `/appapi` 核心兼容入口；
- 用户会话、`xxx-xxxx` 团队邀请码；
- 单写入资金账本、充值/提现审核、彩票/体育/游戏逐局流水；
- 捕鱼 `x1 / x5 / x10`、每场 300 桌、每桌 4 座、随机分配；
- 仅抖音直播源、原生 Go 单聊/群聊/WebSocket；
- MinIO 用户媒体上传、App 整包强更与 WGT 静默热更新；
- 数据统计、用户/团队、资金、游戏、直播、彩票、体育、投注、IM、App、RBAC、系统设置后台；
- 客服会话独立数据模型和独立 Go 服务。

本地检查：

```bash
cd backend
GOWORK=off go test ./...
GOWORK=off go vet ./...
```

本地启动：

```bash
cd backend
docker compose -f deploy/local/compose.yml up -d --build
curl http://127.0.0.1:28080/healthz
curl http://127.0.0.1:28080/api/v2/home
```

本地端口：Nginx `28080`、MySQL `33070`、Redis `6380`、MinIO API
`29000`、MinIO Console `29001`。开发环境密码只用于本机；生产必须通过环境变量注入。

需要运行真实依赖集成测试时：

```bash
CLAW_TEST_MYSQL_DSN='claw_v2:claw_v2_local@tcp(127.0.0.1:33070)/claw_v2?charset=utf8mb4&parseTime=true&multiStatements=true&loc=Asia%2FShanghai' \
CLAW_TEST_REDIS_ADDR='127.0.0.1:6380' \
CLAW_TEST_MINIO_ENDPOINT='127.0.0.1:29000' \
GOWORK=off go test ./...
```

本地管理后台：`http://127.0.0.1:28080/admin/`。管理员必须通过
`cmd/admin-bootstrap` 显式创建，不在 migration 中写入默认生产密码。

独立客服座席端：`http://127.0.0.1:28080/support-console`。座席账号与主管账号
通过主后台 RBAC 创建，或使用一次性引导程序创建：

```bash
V2_MYSQL_DSN='claw_v2:claw_v2_local@tcp(127.0.0.1:33070)/claw_v2?charset=utf8mb4&parseTime=true&multiStatements=true&loc=Asia%2FShanghai' \
V2_SUPPORT_AGENT_USERNAME='support01' \
V2_SUPPORT_AGENT_PASSWORD='请替换为至少12位的强密码' \
V2_SUPPORT_AGENT_DISPLAY_NAME='客服一号' \
V2_SUPPORT_AGENT_ROLE='support_agent' \
go run ./cmd/support-agent-bootstrap
```

主管角色使用 `support_supervisor`。纯客服账号不能登录主后台；座席端使用独立
Cookie、CSRF 与 `portal=support` 会话，无法与主后台会话互换。
