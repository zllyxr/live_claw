# 小游戏架构

> 本文仅保留现有实现记录，不再作为新增游戏的产品与资金架构依据。
> 新游戏必须遵循 [FUNDS_GAME_ARCHITECTURE.md](FUNDS_GAME_ARCHITECTURE.md) 的准入、资金、结算、风控和素材规范。

整理日期: 2026-07-26

目标：新增一款小游戏**不需要改客户端、不需要发版**，只在数据库加一行即可上架。

## 1. 总览

```
                 ┌─ cmf_minigame (注册表, 唯一数据源)
                 │
core-go ─────────┤  MiniGame.list   读目录 → 客户端渲染入口
                 └─ MiniGame.enter  校验登录 → 下发带签名的 launch_url
                          │
uniapp 游戏页 ─→ 小游戏大厅 pages/minigame/index ─→ webview 打开 launch_url
                          │
Apache 同源反代 /minigame/<code>/ ─→ 各游戏容器/静态目录
```

**关键设计：客户端不写死任何游戏。** 大厅页只认注册表返回的 `code / name / cover / launch_url`。

## 2. 注册表 cmf_minigame

见 [docs/sql/minigame_registry_20260726.sql](sql/minigame_registry_20260726.sql)。核心字段：

| 字段 | 作用 |
| --- | --- |
| `code` | 唯一标识，同时用作反代路径 `/minigame/<code>/` 的约定来源 |
| `category` | `arcade` 电子 / `casual` 休闲 / `battle` 对战，决定大厅分组 |
| `cover` | 封面图；缺省时前端按 category 兜底，不会开天窗 |
| `entry_type` | `iframe` 同源内嵌 / `external` 外链 |
| `entry_url` | 入口地址，同源统一用 `/minigame/xxx/` |
| `players_min/max` | 展示"1-4人" |
| `play_mode` | `realtime` / `single` / `local-keyboard` / `local-turn-based` / `webrtc` |
| `need_login` | 是否要求登录，`MiniGame.enter` 会强制校验 |
| **`use_wallet`** | **统一为 1，全部使用平台业务钱包** |
| `orientation` | `auto` / `portrait` / `landscape` |
| `license` / `upstream` | 开源合规审计用 |
| `status` | 1 上架 0 下架，可随时灰度 |

## 3. 接口

### MiniGame.list

`POST /core-api/appapi/` `service=MiniGame.list[&category=arcade]`

返回 `games`（全量）+ `categories`（按分类分组，**空分类自动省略**）。无需登录，便于未登录用户浏览。

### MiniGame.enter

`service=MiniGame.enter&code=<code>[&room=<房间号>]`

- `need_login=1` 且未登录 → 返回 `code=700 请先登录`
- 通过后返回 `launch_url`，附带 `uid`、`name`(昵称)、`ts`、`sig`

`sig` 是 HMAC-SHA256(code|uid|ts) 前 32 位，密钥 `CORE_MINIGAME_SECRET`。
游戏侧若要确认身份可用同一密钥校验并检查 `ts` 时效；不校验则忽略这些参数即可正常运行。

所有游戏默认进入匹配池。core 会把连续进入的用户装入同一张桌，坐满后轮转下一桌，
并在启动 URL 附加 `match=1&table=1..1000`。每款游戏都有独立的 1000 桌编号空间。

## 4. 同源反向代理

Apache（[docker/php/apache-vhost.conf](../docker/php/apache-vhost.conf)）：

```apache
# WebSocket 升级规则必须写在普通 ProxyPass 之前
ProxyPass        /minigame/fish/socket.io/ ws://fishing-game:3000/minigame/fish/socket.io/
ProxyPassReverse /minigame/fish/socket.io/ ws://fishing-game:3000/minigame/fish/socket.io/
ProxyPass        /minigame/fish/ http://fishing-game:3000/minigame/fish/
ProxyPassReverse /minigame/fish/ http://fishing-game:3000/minigame/fish/
```

同源的好处：避免跨端口 CORS、HTTPS 下不产生混合内容、App 内 webview 与 H5 行为一致、生产环境只需暴露 80/443。

**注意**：vhost 是构建时 COPY 进镜像的，改完必须 `docker compose up -d --build php` 才生效（单纯 restart 无效）。

## 5. 游戏侧改造：子路径可挂载

深海猎手原来用绝对路径（`/styles.css`、`/socket.io/`），挂子路径会 404。已改为：

| 文件 | 改动 |
| --- | --- |
| `game/src/server.js` | 新增 `BASE_PATH` 环境变量；socket.io 的 `path` 与 express.static 同时挂根路径和子路径；`/health` 双路径 |
| `game/public/index.html` | 资源引用改相对路径（`styles.css`、`game.js`、`socket.io/socket.io.js`）|
| `game/public/game.js` | socket 客户端按 `location.pathname` 推导 `path`，根挂载与子路径都能连上 |

因此**直连 `http://127.0.0.1:18082/` 与经反代 `/minigame/fish/` 都可用**，本地调试不受影响。

## 6. 新增一款游戏的步骤

以静态 HTML5 游戏为例：

1. 游戏文件放到能被 Web 访问的位置（静态目录或独立容器）
2. 若是独立容器，在 `docker-compose.yml` 加服务，并在 vhost 加两条 ProxyPass（有 WS 就加 ws 规则）
3. 确保游戏资源用**相对路径**，或支持 `BASE_PATH` 之类的前缀配置
4. 往注册表插一行：

```sql
INSERT INTO cmf_minigame
  (code, name, category, cover, entry_type, entry_url, players_min, players_max,
   play_mode, need_login, use_wallet, orientation, sort, status, create_time, update_time)
VALUES
  ('my_game', '我的游戏', 'casual', '/static/art/minigame/my.webp', 'iframe',
   '/minigame/my_game/', 1, 2, 'single', 1, 0, 'auto', 30, 1, UNIX_TIMESTAMP(), UNIX_TIMESTAMP());
```

5. 客户端无需改动、无需发版，下次进大厅即可见。

封面建议 4:3、webp、80KB 内，用 `scripts/slice-sprite.mjs` 从雪碧图批量切。

## 7. 待办与风险（重要）

### 钱包边界

所有已上架游戏统一使用 `cmf_user.coin`。游戏容器只可通过 core 的内部接口查询余额和提交
幂等资金订单，不能直连用户表。`cmf_minigame_wallet_order.order_no` 唯一，确保重连、超时重试
不会重复扣款或派彩；每笔变化同时写平台 `cmf_user_coinrecord`。

- 斗地主/麻将：开局统一扣 100 星币入桌，结束后把本金与输赢返还；先收齐全桌入桌金才发牌。
- 捕鱼：每发一炮先扣对应炮值，扣款成功后炮弹才进入模拟；捕获奖励按结算 ID 幂等入账。
- 启动凭证 30 分钟有效，由 core 与游戏服务使用 HMAC 校验。

### 开源许可证

`game2/` 六款游戏的许可证差异很大，已记进注册表 `license` 字段，且**默认全部 `status=0` 未上架**：

| 游戏 | 许可证 | 风险 |
| --- | --- | --- |
| 贪吃曲线 achtung_kurve | GPL-3.0 | 强 copyleft |
| 飞行棋 libreludo | **AGPL-3.0-only** | **最强，网络使用即触发开源义务** |
| 焦土坦克 scorch | MIT | 低，保留版权声明 |
| 攻城战争 siege_wars | MIT + 素材 CC-BY-3.0 | 需保留素材署名 |
| 流体乒乓 fluid_table_tennis | 上游宽松许可 | 需人工确认具体条款 |
| 迷宫对枪 p2p_maze_shooter | MIT | 依赖公网 WebRTC 信令 |

**商用上架前建议逐个过法务**，尤其 AGPL 那款。我把它们留在注册表里但下架，方便你确认后逐个 `status=1`。

### 其他

- 这些游戏多为**桌面键盘操作**，移动端触屏适配需要逐个验证后再上架
- 大厅页已做封面兜底与骨架屏，新游戏没配图也不会开天窗

---

## 第二批游戏：斗地主 · 麻将（2026-07-26）

新增服务 `cardgames`（Node + Express），一个容器承载两款游戏，反代到 `/minigame/cards/`。

### 目录

```
cardgames/
  src/ddz/rules.js       斗地主规则引擎（牌型识别/比较/枚举出法）
  src/ddz/game.js        对局状态机（真人座位 + 回归测试 AI）
  src/mahjong/rules.js   麻将规则引擎（胡牌判定/番型/向听）
  src/mahjong/game.js    对局状态机（真人座位 + 回归测试 AI）
  src/server.js          Socket.IO 多人匹配桌 + 兼容规则测试接口
  public/ddz/            斗地主界面
  public/mahjong/        麻将界面
  tests/                 37 项测试
```

### 玩法与规则覆盖

**斗地主**（3 位平台用户）：单张/对子/三张/三带一/三带二/顺子(≥5)/连对(≥3)/飞机/飞机带翅膀/四带二/炸弹/王炸。
压制规则：同型同长比大小；炸弹压普通牌，大炸弹压小炸弹，王炸最大。炸弹与王炸自动翻倍。
顺子严格限制在 3..A（不含 2 和王）。

**麻将**（4 位平台用户，推倒胡）：可碰、可明杠/暗杠，不吃。
胡型：标准型（4 面子 + 1 将，副露顶替面子）与七小对（四张同牌算两对）。
番型：平胡 1，七小对 +3，碰碰胡 +2，清一色 +3，每杠 +1，自摸 +1。
额外提供**听牌提示**与**向听数**（服务端计算，客户端不实现规则）。

### 服务端权威

洗牌用 crypto 强随机；发牌、牌型判定、真人操作校验、胡牌与番数全在服务端。
客户端提交动作后服务端会校验：是否为手牌子集、牌型是否合法、能否压过上家、是否轮到你。
**视图裁剪**：`viewFor()` 只返回自己的手牌，他人只给张数，且不含牌墙内容——避免前端抓包看牌。

### 多人匹配

- 斗地主必须 3 位真人坐齐才发牌；麻将必须 4 位真人坐齐才发牌。
- 每个客户端只收到自己的手牌，对手只返回张数、副露与公开牌河。
- 断线用户可凭同一平台身份回到原桌；重复连接会替换旧连接，不增加座位。

### 测试

`cd cardgames && npm test` → **37 项全通**，覆盖：

- 牌型识别边界（四连非顺子、含 2/王非顺子、8万9万1条非顺子、三带二必须带对）
- 炸弹/王炸压制关系
- 胡牌判定（标准型、七小对、带副露张数、四张算两对）
- 听牌检测（返回的每张牌都真的能胡）
- **完整对局跑通**：12 局斗地主 + 8 局麻将，无死循环、牌数守恒、赢家手牌为空
- 视图不泄露他人手牌与牌墙

### 已知边界

- 斗地主未实现「抢地主多轮加倍」，按叫分者中手牌最强者定地主。
- 麻将未实现吃牌、杠上开花、抢杠胡等特殊番型。
- 当前桌状态在单个 cardgames 进程内；水平扩容前需要增加桌号粘性路由或共享房间层。

### 踩坑记录（重要）

1. **vite 插件会重写代码里的字面量 `"/static/"`**
   `uniapp/vite.config.ts` 的 `xingyu-h5-static-base` 插件把构建产物里所有 `"/static/"`
   替换成 `"/h5/static/"`。我在 `coverOf()` 里写 `raw.startsWith("/static/")` 判断，
   结果**判断条件本身被改成了 `startsWith("/h5/static/")`**，而接口返回的是 `/static/...`，
   条件永远不成立，封面全部回落到兜底图。
   → 修法：判断写成不带尾斜杠的 `startsWith("/static")`，该正则要求 `/static/` 才匹配。

2. **接口运行时返回的本地资源路径必须手动拼基路径**
   构建期的字面量会被插件重写，但 API 运行时返回的字符串不会。
   H5 挂在 `/h5/` 下，`/static/xxx` 会被浏览器当站点根解析导致 404。
   → 新增 `utils/url.ts` 的 `localAssetUrl()`，运行时从 `location.pathname` 推导基路径
   （不用 `import.meta.env.BASE_URL`，uni-app H5 下不可靠）。

3. **改 Apache vhost 必须重建 php 镜像**，配置是构建时 COPY 进去的，restart 不生效。
