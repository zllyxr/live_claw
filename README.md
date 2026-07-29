# Live Claw

Live Claw 是一个包含直播、即时聊天、体育、彩票和多人游戏的平台。
旧 PHP、Apache、Composer、OpenIM 以及 Node 游戏服务已从项目中移除。

## 项目组成

- `backend`：六个独立 Go 程序、Layui 管理后台、数据库迁移和本地 Nginx
- `uniapp`：保持现有 UI 的 UniApp 用户端，包含 H5 与 App 构建
- `release`：App 整包与热更新发布产物
- `docs`：当前架构、接口对齐与素材说明

后端运行时只使用 Nginx、Go、MySQL、Redis、MinIO 和 Layui。捕鱼网页及素材已嵌入
Go 游戏服务，不需要额外的 Node 或 PHP 进程。

## 本地启动

```bash
cd backend
docker compose -f deploy/local/compose.yml up -d --build
```

默认入口：

- API/Nginx：`http://127.0.0.1:28080`
- 后台：`http://127.0.0.1:28080/admin/`
- 健康检查：`http://127.0.0.1:28080/healthz`
- MySQL：`127.0.0.1:33070`
- Redis：`127.0.0.1:6380`
- MinIO：`http://127.0.0.1:29000`

前端开发与构建：

```bash
cd uniapp
pnpm install
pnpm typecheck
pnpm build:h5
```

后端检查：

```bash
cd backend
GOWORK=off go test ./...
GOWORK=off go vet ./...
```

## 单机生产部署

生产目录固定为 `/opt/claw`，Compose 只公开 Nginx 的 `80/443` 端口。MySQL、Redis
和 MinIO API 仅在容器网络内访问；MinIO Console 只监听服务器的
`127.0.0.1:29001`。

首次部署：

```bash
cd /opt/claw
test -f uniapp/dist/build/h5/index.html
cp backend/deploy/production/env.example .env
chmod 600 .env
# 把 Cloudflare Origin Certificate 和私钥分别保存到：
# /opt/claw-secrets/tls/tmpai2.com-origin.crt
# /opt/claw-secrets/tls/tmpai2.com-origin.key
chmod 600 /opt/claw-secrets/tls/tmpai2.com-origin.crt \
  /opt/claw-secrets/tls/tmpai2.com-origin.key
# 填写 .env 中所有必填空值后再继续；证书和私钥不得放进 Git 仓库
docker compose --env-file .env -f backend/deploy/production/compose.yml config --quiet
docker compose --env-file .env -f backend/deploy/production/compose.yml up -d --build
```

默认入口：

- H5：`https://tmpai2.com/h5/`
- API 健康检查：`https://tmpai2.com/healthz`
- 管理后台：`https://tmpai2.com/admin/`
- 客服桌席：`https://tmpai2.com/support-console`

Nginx 在源站终止 Cloudflare Origin TLS，HTTP 会永久跳转到 HTTPS。Cloudflare 中
应开启代理并把 SSL/TLS 模式设为 `Full (strict)`；Origin Certificate 不是公共
浏览器信任的证书，因此不应关闭 Cloudflare 代理后让用户直连源站。服务器防火墙
应尽量只允许 Cloudflare 地址段访问源站 `80/443`，并仅向管理地址开放 SSH。

管理员和客服账号只通过一次性 `bootstrap` profile 创建。先临时填写 `.env` 中对应
账号和强密码，执行后立即清空这些账号密码变量：

```bash
docker compose --env-file .env -f backend/deploy/production/compose.yml \
  --profile bootstrap run --rm admin-bootstrap
docker compose --env-file .env -f backend/deploy/production/compose.yml \
  --profile bootstrap run --rm support-agent-bootstrap
docker compose --env-file .env -f backend/deploy/production/compose.yml \
  --profile bootstrap run --rm user-bootstrap
```

普通用户账号用于 H5 登录，`V2_APP_USER_USERNAME` 必须填写 5–20 位手机号数字；
初始化会同时创建零余额星币钱包、系统团队关系和邀请码。重复执行不会覆盖已有密码。

需要从 `女生` 目录初始化虚拟直播用户时：

```bash
docker compose --env-file .env -f backend/deploy/production/compose.yml \
  --profile bootstrap run --rm virtual-live-bootstrap
```

生产服务默认使用 Secure Cookie，`V2_ADMIN_RUNTIME_ENV` 和
`V2_SUPPORT_RUNTIME_ENV` 必须保持为 `production`。

MinIO 预签名依赖公开地址，`V2_PUBLIC_URL`、`V2_MINIO_PUBLIC_ENDPOINT` 与
`V2_MINIO_PUBLIC_USE_TLS` 必须和用户实际访问的协议、域名及端口一致；本配置固定
为 `https://tmpai2.com`、`tmpai2.com` 和 `true`。原生 App 也默认访问该域名。

MySQL、Redis 和 MinIO 使用固定名称的数据卷。正常更新只执行 `up -d --build`，
不要执行 `docker compose down -v`。容器日志已限制为每个文件 `10 MB`、保留
`5` 个文件。

本地环境密码只供开发使用。生产配置必须通过环境变量注入；代码发布仍遵循先提交
Git、服务器只执行 `git pull --ff-only` 的流程。
