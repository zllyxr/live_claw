# Live Claw

Live Claw 是一个包含直播、即时聊天、体育、彩票和多人小游戏的完整平台项目。

## 项目组成

- `uniapp`：UniApp 用户端，包含 H5 与 App 构建
- `admin`：ThinkCMF/PHP 后台及业务接口
- `services/core`：Go 核心服务，负责体育、彩票、OpenIM 会话和小游戏钱包
- `game`：捕鱼游戏服务
- `cardgames`：麻将、斗地主、炸金花、跑得快等牌类游戏服务
- `game2`：其他小游戏资源
- OpenIM、MySQL、Redis、MongoDB、Kafka、MinIO、SRS：由 Docker Compose 统一运行

## 本地启动

```bash
cp .env.example .env
cp admin/.example.env admin/.env
docker compose up -d --build
```

默认入口：

- H5：`http://127.0.0.1:18080/h5/`
- 后台：`http://127.0.0.1:18080/admin`
- 核心健康检查：`http://127.0.0.1:18081/healthz`

前端开发与构建：

```bash
cd uniapp
npm install
npm run typecheck
npm run build:h5
```

## 生产部署

```bash
cp .env.example .env
cp docker/php/admin-production.env.example docker/php/admin-production.env
# 修改两个环境文件中的密码、密钥、域名与外部地址
docker compose -f docker-compose.yml -f docker-compose.prod.yml up -d --build
```

生产环境必须使用随机强密码，并在反向代理/CDN 上开启 WebSocket。不要提交 `.env`、生产部署文档、数据库备份、支付证书或服务器私钥。

仓库附带的初始化数据库已经脱敏，管理员密码不可用于登录。部署者应设置自己的 `DATABASE_AUTHCODE`，并在首次上线前生成新的管理员密码。
