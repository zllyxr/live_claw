# 捕鱼美术资源来源

## 当前生产素材

### 用户提供的动态鱼素材

当前鱼群使用用户提供并明确要求接入的 `HTML5捕鱼达人游戏源码/images` 逐帧精灵条：

- 文件：`fish1.png` 至 `fish10.png`、`shark1.png`、`shark2.png`
- 生产位置：`public/assets/fish-animated/`
- 用途：鱼群游动逐帧动画；运行时按方向镜像，并保留素材原始帧尺寸
- 当前实际使用：`fish1`、`fish2`、`fish3`、`fish4`、`fish7`、`fish8`、`fish9`、`fish10`、`shark1`、`shark2`

### AI 生成海域背景

以下背景于 2026-07-26 使用 OpenAI 图像生成工具为本项目生成。

- 文件：`public/assets/arcade-ocean-arena-v3.png`
- 尺寸：1672×941，RGB PNG
- 用途：16:9 捕鱼主战场与进入页主视觉
- 构图：中央水域留空，珊瑚、礁石、沉船宝藏和光束集中在边缘

## 已停用的生成鱼图集

以下两张 AI 生成静态图集仍保留在仓库中，但当前生产画面已经不再加载：

- 文件：`public/assets/arcade-creatures-common-v3.png`
- 尺寸：1536×1024，RGBA PNG
- 网格：3 列 × 2 行，每格 512×512
- 顺序：金枪鱼、狮子鱼、河豚、石斑、海龟、蝠鲼
- 朝向：全部朝左；运行时根据游动方向镜像

### 高倍率鱼图集

- 文件：`public/assets/arcade-creatures-boss-v3.png`
- 尺寸：1254×1254，RGBA PNG
- 网格：2 列 × 2 行，每格 627×627
- 顺序：锤头鲨、巨型章鱼、虎鲸、鮟鱇
- 朝向：全部朝左；运行时根据游动方向镜像

两张鱼类图集以纯洋红背景生成，再使用 imagegen 技能附带的 `remove_chroma_key.py` 转为透明 PNG。处理参数为自动采样边缘色、软边阈值 10/72、收边 1 像素。

完整生成提示词见 [`GENERATED_ASSET_PROMPTS.md`](GENERATED_ASSET_PROMPTS.md)。
