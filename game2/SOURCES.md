# 来源、固定版本与许可证

本合集只收录来源、玩家人数和许可证可以核验的开源项目。每个游戏的上游源码、README 和许可证都保存在自己的目录中；构建产物不改变原项目的版权归属。

## 版本清单

### Achtung, die Kurve!

- 上游：[maechler/kurve](https://github.com/maechler/kurve)
- 固定版本：[v1.5.1 / `02702ad86319c22e28ab5cc2038ba39a4ba722a9`](https://github.com/maechler/kurve/tree/02702ad86319c22e28ab5cc2038ba39a4ba722a9)
- 人数：上游提供 6 组键位，至少选择 2 人才可开始；本合集按需求标为 2–5 人可玩
- 模式：同一设备、同一键盘、即时对战
- 许可证：GPL-3.0，见 [`games/achtung-kurve/LICENSE.txt`](games/achtung-kurve/LICENSE.txt)
- 入口：[`games/achtung-kurve/index.html`](games/achtung-kurve/index.html)
- 构建：已从固定版本生成 `dist/`；未保留 `node_modules`
- 本地调整：合集版本禁用上游 Matomo 统计与阻挡式 cookie 接受流程；详情见目录内的 `LOCAL_CHANGES.md`

### Scorch Clone

- 上游：[webermn15/Scorch_a-scorched-earth-clone](https://github.com/webermn15/Scorch_a-scorched-earth-clone)
- 固定提交：[`5ba6c3e084842bdac4db3f201e71145156f5768b`](https://github.com/webermn15/Scorch_a-scorched-earth-clone/tree/5ba6c3e084842bdac4db3f201e71145156f5768b)
- 人数：2–4 人
- 模式：同一浏览器窗口、轮流炮战
- 许可证：MIT，见 [`games/scorch/LICENSE`](games/scorch/LICENSE)
- 入口：[`games/scorch/index.html`](games/scorch/index.html)
- 构建：原生 HTML、CSS 和 JavaScript，无需构建

### Fluid Table Tennis

- 上游：[anirudhjoshi/fluid_table_tennis](https://github.com/anirudhjoshi/fluid_table_tennis)
- 固定提交：[`2a7e2a8e509a215f48d30d34bc257bbfd765863b`](https://github.com/anirudhjoshi/fluid_table_tennis/tree/2a7e2a8e509a215f48d30d34bc257bbfd765863b)
- 人数：2 人
- 模式：同一设备、同一键盘、即时对战；进入后选择 **Begin Multiplayer**
- 许可证：项目自带 MIT 风格的宽松许可文本，GitHub 未给出 SPDX 标识；以 [`games/fluid-table-tennis/LICENSE`](games/fluid-table-tennis/LICENSE) 原文为准
- 入口：[`games/fluid-table-tennis/build/index.html`](games/fluid-table-tennis/build/index.html)
- 构建：已用上游 `utils/quick_build.sh` 生成 `build/`
- 本地调整：移除会主动加载的旧统计和社交小组件，保留作者署名与普通外链；详情见目录内的 `LOCAL_CHANGES.md`

### P2P Maze Shooter

- 上游：[arifulislamat/p2p-maze-shooter](https://github.com/arifulislamat/p2p-maze-shooter)
- 固定提交：[`29d9adbbd1ddb56e3dcf796cf306d21b232c6808`](https://github.com/arifulislamat/p2p-maze-shooter/tree/29d9adbbd1ddb56e3dcf796cf306d21b232c6808)
- 人数：2 人
- 模式：两个浏览器通过 WebRTC 直连
- 许可证：MIT，见 [`games/p2p-maze-shooter/LICENSE`](games/p2p-maze-shooter/LICENSE)
- 入口：[`games/p2p-maze-shooter/src/index.html`](games/p2p-maze-shooter/src/index.html)
- 构建：原生 HTML、CSS 和 JavaScript，无需构建
- 联网说明：运行依赖 PeerJS CDN、PeerJS 信令服务和 STUN；受限 NAT、防火墙或桌面浏览器的 mDNS 设置可能导致连接失败。语音权限通常要求 HTTPS 或 `localhost`
- 本地调整：移除 Google Analytics；保留游戏联机必需的外部依赖，详情见目录内的 `LOCAL_CHANGES.md`

### Siege Wars

- 上游：[raaaahman/siege-wars](https://github.com/raaaahman/siege-wars)
- 固定提交：[`1bd5aea38e315b036e4ad1436a0f701bf08b4a51`](https://github.com/raaaahman/siege-wars/tree/1bd5aea38e315b036e4ad1436a0f701bf08b4a51)
- 人数：2 人；游戏模型固定红、蓝双方
- 模式：同一设备、轮流攻城
- 代码许可证：MIT，见 [`games/siege-wars/LICENSE`](games/siege-wars/LICENSE)
- 素材许可：Toen Medieval Strategy 为 CC BY 3.0；SuperPowers Bitmap Fonts 与 Golden UI 为 CC0；上游署名保留在 [`games/siege-wars/README.md`](games/siege-wars/README.md)
- 入口：[`games/siege-wars/dist/index.html`](games/siege-wars/dist/index.html)
- 构建：Webpack 4 产物已生成；未保留 `node_modules`

### LibreLudo

- 上游：[priyanshurav/libreludo](https://github.com/priyanshurav/libreludo)
- 固定版本：[v2.1.0 / `2ef28e27cdbcfd301f2638a3414a93713558fd69`](https://github.com/priyanshurav/libreludo/tree/2ef28e27cdbcfd301f2638a3414a93713558fd69)
- 人数：2–4 人，可混合真人与机器人
- 模式：同一设备、轮流棋盘游戏
- 许可证：AGPL-3.0-only，见 [`games/libreludo/LICENSE`](games/libreludo/LICENSE)；构建产物还保留 `THIRD_PARTY_LICENSES.txt`
- 入口：[`games/libreludo/dist/index.html`](games/libreludo/dist/index.html)
- 本地调整：为子目录托管改用相对资源路径和 hash 路由，并同步保留对应源码；详情和重建命令见 [`games/libreludo/LOCAL_CHANGES.md`](games/libreludo/LOCAL_CHANGES.md)

## 再分发说明

- 不要把本页当成六个游戏的统一许可证；各目录内的许可证分别生效。
- 修改或公开部署 GPL/AGPL 项目时，应继续向接收者提供对应源码与许可证。
- 再分发 Siege Wars 时需继续保留 CC BY 3.0 素材署名。
- 本合集没有收录来源或许可证不清晰的项目，也没有收录只能依赖完整大型服务端平台运行的候选。
