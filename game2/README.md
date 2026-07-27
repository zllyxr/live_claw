# Game2：2–5 人网页对战游戏集

这是一个独立的开源网页游戏合集。游戏来自各自的官方公开仓库，并按固定版本收录；`game2` 之外的现有项目不参与运行。

## 快速启动

在项目目录执行：

```bash
./game2/serve.sh
```

脚本会监听 `0.0.0.0:4173`。本机可访问 <http://127.0.0.1:4173>；网关应转发到运行主机的 TCP 4173 端口。如需更换端口，可执行 `PORT=8080 ./game2/serve.sh`。

不建议直接双击入口文件：部分浏览器会限制 `file://` 页面加载模块、字体或 WebRTC 功能。P2P Maze Shooter 建立双浏览器连接时还需要网络和 WebRTC 支持。

## 游戏列表

| 游戏 | 可用人数 | 模式 | 本地入口 |
| --- | ---: | --- | --- |
| Achtung, die Kurve! | 2–5（上游最多 6 人） | 同屏、同键盘、即时对战 | `games/achtung-kurve/index.html` |
| Scorch Clone | 2–4 | 同屏、轮流炮战 | `games/scorch/index.html` |
| Fluid Table Tennis | 2 | 同屏、即时对战 | `games/fluid-table-tennis/build/index.html` |
| P2P Maze Shooter | 2 | 两个浏览器、WebRTC 联机 | `games/p2p-maze-shooter/src/index.html` |
| Siege Wars | 2 | 同屏、轮流攻城 | `games/siege-wars/dist/index.html` |
| LibreLudo | 2–4 | 同屏、轮流棋盘游戏 | `games/libreludo/dist/index.html` |

## 目录说明

- `index.html`：中文游戏入口与人数筛选。
- `assets/`：入口页自己的样式和交互。
- `games/`：六个第三方游戏的源码、许可证与构建产物。
- `SOURCES.md`：固定版本、上游链接、许可证和运行限制。
- `serve.sh`：监听 `0.0.0.0` 的静态服务器启动脚本。

## 许可说明

每个第三方游戏仍由其原作者持有版权，并分别遵循自身目录内的许可证。合集入口页不会改变、合并或替代这些许可证；再分发或修改前请阅读对应的 `LICENSE`、`COPYING`、素材署名文件及 [`SOURCES.md`](SOURCES.md)。
