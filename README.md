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

本地环境密码只供开发使用。生产配置必须通过环境变量注入；代码发布仍遵循先提交
Git、服务器只执行 `git pull --ff-only` 的流程。
