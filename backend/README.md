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
- 独立代理后台、按白名单授权的业务模块及一名代理对应多个团队邀请码前缀；
- 客服会话独立数据模型和独立 Go 服务。

## 验证码邮箱发送

管理后台“系统设置 → 邮箱发送”（`/admin/app#system/email`）可配置 SMTP
服务器、端口、TLS/STARTTLS、登录账号、发件邮箱和应用专用密码。保存及测试
发送要求 `system.write` 权限和 CSRF 校验，查看要求 `system.read` 权限。
密码留空保留原值，仅返回是否已配置；Gmail 使用应用专用密码。

配置加密保存在 `system_settings` 的 `mail.smtp` 项，使用
`V2_DATA_ENCRYPTION_KEY` 派生的独立 AES-GCM 密钥；设置和不含密码的审计
在同一事务提交。通用 JSON 编辑器不能修改此项。API 每次验证码请求读取已提交
配置，保存即生效，停用时不会回退到环境变量。首次后台保存前兼容 `V2_SMTP_*`
环境配置，因此管理员和 API 容器应使用相同配置和数据加密密钥。

测试按钮使用已保存且启用的配置，发送不含验证码的测试邮件，每分钟最多一次。
成功表示 SMTP 服务接受邮件，实际收件仍需检查收件箱/垃圾邮件。SMTP 未配置、
停用或解密失败时，正式环境不会返回固定验证码。

## BEpusdt 支付

BEpusdt 作为独立容器运行，主后端只通过 HTTP API 与它通信，不复制或链接其
GPL 源码。Compose 固定使用 BEpusdt `v1.24.1` 对应的提交镜像与清单摘要；
SQLite 数据和网关日志分别保存在独立持久卷。

本地网关管理入口仅绑定在 `127.0.0.1:28081`。生产容器的维护端口仍绑定在
服务器回环地址 `127.0.0.1:29002`。管理后台通过独立 HTTPS 域名
`pay.travelig.vip` 访问，必须使用 BEpusdt 初始化生成的安全入口路径及账号密码；
这些凭证仅私密交付，不写入 Nginx 配置。登录限频，Cookie 强制 Secure/HttpOnly，
管理页面禁止缓存，入口路径不记访问日志。该域名不代理签名建单或收银台接口。

主站 `travelig.vip` 保留原有收银台路径：

- `/pay/`：订单专属收银台；
- `/checkout/`：收银台静态资源；
- `/api/v1/pay/`：收银台查询与选择付款网络。

主站不代理 BEpusdt 后台及登录接口，签名建单接口只在内部网络调用。不要用
健康检查或监控访问 BEpusdt 根路径；首次根路径访问会展示一次性初始化凭据。
首次部署必须先完成下面的凭证捕获，确认 `system_install_lock=1`，再发布
管理域名。Origin 证书及 DNS 必须覆盖 `pay.travelig.vip`。

接入步骤：

1. 生产容器禁用 Docker 日志，避免上游首次启动输出的一次性密码与 Token
   落入日志文件。部署时通过回环入口只访问一次根页面，将结果保存为
   `0600` 的 root 专用文件，再立即更换管理员密码和 API Token。
2. 在 BEpusdt 后台配置至少一个收款钱包、可靠的链 RPC、确认数、汇率和金额
   匹配策略。USDT.TRC20 生产环境建议配置 TronGrid Key。
3. 在 Claw 管理后台“支付管理”填写 API 地址、公开收银台地址、Token、法币、
   交易类型和超时时间。Docker 内部 API 地址为 `http://bepusdt:8080`；
   生产公开地址为站点 HTTPS 地址。
4. 使用“协议检查”确认网络连通，再启用通道。通道启用前不会出现在客户端。
5. 先做一笔小额真链测试，核对回调、钱包流水、充值订单和链上交易哈希。

API Token 使用 `V2_DATA_ENCRYPTION_KEY` 派生的 AES-256-GCM 密钥保存在
`payment_channels.config_ciphertext`，管理 API 只返回“是否已配置”，不会
回传明文。BEpusdt 回调入口为 `/api/v2/payments/bepusdt/notify`；回调必须
通过签名、金额、渠道交易号与订单归属校验后才会在同一数据库事务内入账。

备份主数据库时也必须备份 `bepusdt_data` 卷中的 SQLite 数据库及 WAL/SHM
文件。不要只复制正在写入的单个 `sqlite.db` 文件；应先使用 SQLite 在线备份
命令，或短暂停止 BEpusdt 后复制整个数据卷。

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

独立代理端：`http://127.0.0.1:28080/agent-console`。代理账号由主后台“代理管理”
创建；代理只能使用获授权的游戏、直播、彩票、体育、投注、App 模块，并只能查看
归属自己的三位团队前缀、当前在队人数和成员最小资料。成员列表不返回联系方式、
资金或邀请关系。代理生成的是空团队前缀，首个普通用户仍由平台管理员在用户管理中
分配。

独立团队端：`http://127.0.0.1:28080/team-console`。非系统团队的首个普通成员由
平台管理员转入后会自动成为团队负责人；负责人使用自己的普通用户账号登录，只能
查看当前仍在本团队的用户 ID、昵称、账号状态和入队时间。负责人转队、被冻结、
团队停用或负责人被替换后，团队端会话立即失效。

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

## 注册与找回密码邮件

生产 API 通过 SMTP 发送 6 位随机验证码，10 分钟内有效，只保存哈希。
成功重发后旧码失效；每个码最多允许 5 次错误尝试，成功验证只能使用一次。
Redis 限制同一邮箱 60 秒一次、每小时 5 次，同一 IP 每小时 20 次，注册和找回密码共用额度。
生产未配置 SMTP 或发送失败时返回错误，不回显验证码；只有明确的
`local/development/test` 环境且未配置 SMTP 时保留本地测试码。

在服务器受保护的 `.env` 配置下列变量，仅 API 容器接收邮件凭证：

```dotenv
V2_SMTP_HOST=smtp.gmail.com
V2_SMTP_PORT=587
V2_SMTP_TLS_MODE=starttls
V2_SMTP_USERNAME=your-sender@gmail.com
V2_SMTP_PASSWORD=your-google-app-password
V2_SMTP_FROM=your-sender@gmail.com
```

Gmail 使用专用密码；不要填 Google 登录密码，不要将真实值提交 Git。
支持 `starttls`（通常 587）或 `tls`（通常 465），始终验证 TLS 证书。
配置后按正常 GitHub 拉取流程构建并更新 API。验收需确认真实收件箱收到邮件，
API 成功只代表 SMTP 服务已接受投递。

邮件生命周期与限频集成测试使用隔离 MySQL 和 Redis：设置
`CLAW_TEST_MYSQL_DSN` 与 `CLAW_TEST_REDIS_SOCKET`，运行
`go test -race ./internal/mailer ./internal/server -run 'Test(SMTP|ProductionVerification|Verification)'`。
