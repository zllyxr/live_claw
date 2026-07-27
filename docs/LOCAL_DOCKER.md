# Docker 运行手册

## 配置

根目录 `.env` 至少配置：

```dotenv
API_FOOTBALL_KEY=replace_me
API_FOOTBALL_BASE_URL=https://v3.football.api-sports.io
OPENIM_SECRET=replace_with_a_long_random_value
OPENIM_MONGO_PASSWORD=replace_me
OPENIM_REDIS_PASSWORD=replace_me
OPENIM_MINIO_PASSWORD=replace_me
OPENIM_MINIO_EXTERNAL_ADDRESS=http://127.0.0.1:10005
```

## 启动与检查

```sh
docker compose config --quiet
docker compose up -d --build --remove-orphans
docker compose ps
curl http://127.0.0.1:18081/healthz
```

主要地址：

| 功能 | 地址 |
| --- | --- |
| Web/PHP | `http://127.0.0.1:18080/` |
| Go Core | `http://127.0.0.1:18081/` |
| MySQL | `127.0.0.1:18306` |
| Redis | `127.0.0.1:18379` |
| OpenIM WebSocket | `ws://127.0.0.1:10001` |
| OpenIM API | `http://127.0.0.1:10002` |
| OpenIM MinIO | `http://127.0.0.1:10005` |

## 数据库迁移

已有数据卷不会自动重新执行 `/docker-entrypoint-initdb.d`。升级前先备份，再手动执行：

```sh
docker compose exec -T db mysqldump -uroot -pclaw_root_pwd --single-transaction claw_live > claw_live_pre_core.sql
docker compose exec -T db mysql -uroot -pclaw_root_pwd claw_live < docker/mysql/init/02-core-rebuild.sql
```

迁移是幂等的，会创建本地开奖/审计/体育快照表，并删除旧彩票采集表和本地 IM 表。

## 验收

```sh
docker compose exec -T core wget -qO- http://127.0.0.1:8080/healthz
curl 'http://127.0.0.1:18080/core-api/appapi/?service=LotteryGame.home'
curl 'http://127.0.0.1:18080/core-api/appapi/?service=Sports.home&tab=live'
docker compose logs --tail=100 core openim-server
```

