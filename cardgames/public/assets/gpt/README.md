# GPT 统一牌桌美术资源

生成日期：2026-07-26  
生成方式：OpenAI 内置 `imagegen` 默认生成模式；先生成无文字图集，再按固定网格切片。  
视觉规范：深翡翠绿、暖金、朱砂红，精致 3D 手游图标，统一正面柔光、统一材质、无品牌与水印。

## 扑克 UI 图集

- 原始图：`poker-ui/atlas-source-v1.png`
- 去底透明图：`poker-ui/atlas-v1.png`
- 网格：4 列 × 3 行，每格 362 × 362
- 切片顺序：金/红/蓝筹码、庄家钮、交叉牌背、皇冠、闪电、盾牌、星星、沙漏、准备徽章、奖杯

最终提示词：

> Create one exact 4x3 sprite atlas of twelve premium Chinese card-game UI assets: gold, red and blue chip stacks; dealer button; crossed blank playing-card backs; crown; lightning; shield; star; hourglass; ready emblem; trophy. Premium jade, gold and cinnabar palette, polished 3D mobile-game icon style, consistent front soft lighting, each object centered and isolated, flat #ff00ff chroma background, no text, logo, watermark, grid lines or shadows crossing cells.

## 麻将 UI 图集

- 原始图：`mahjong-ui/atlas-source-v1.png`
- 去底透明图：`mahjong-ui/atlas-v1.png`
- 网格：4 列 × 3 行，每格 362 × 362
- 切片顺序：东风、红中、发财、白板、骰子、庄家、吃、碰、杠、胡、倒计时灯笼、奖杯

最终提示词：

> Create one exact 4x3 sprite atlas of twelve premium Chinese Mahjong UI assets: east-wind tile, red-dragon tile, fortune-dragon tile, white-dragon tile, dice, dealer emblem, chow emblem, pung emblem, kong emblem, win emblem, countdown lantern, trophy. Premium ivory tile, jade, gold and cinnabar materials, refined 3D mobile-game icon style, consistent front soft lighting, every asset centered and isolated, flat #ff00ff chroma background, no words, logo, watermark, grid lines or shadows crossing cells.

## 游戏封面图集

- 图集：`covers/atlas-v1.png`
- 网格：2 列 × 2 行，每格 724 × 543（4:3）
- 切片：炸金花、跑得快、血战麻将、红中麻将
- 客户端成品：`uniapp/src/static/art/minigame/*-v1.webp`

最终提示词：

> Create one exact 2x2 atlas of four cinematic 4:3 mobile game covers: top-left Golden Flower with three dramatic playing cards and gold chips; top-right Run Fast with flying cards and strong speed trails; bottom-left Sichuan blood-battle Mahjong with clashing jade tiles and fiery energy; bottom-right red-center Mahjong focused on a glowing red-dragon tile. Premium Chinese jade, warm gold and cinnabar 3D key art, dynamic depth, consistent lighting and quality, no text, logo, watermark or grid lines.

说明：图集使用纯色键控去底；PNG 图标保留透明通道。v2 牌面把准确点数与文字固化进最终 WebP，避免生成式文字错误。

## 完整扑克牌面 v2

- GPT 原始图集：`playing-cards-v2/atlas-source-v2.png`
- 透明组件图集：`playing-cards-v2/atlas-v2.png`
- 完整切片：`playing-cards-v2/deck/0.webp` 至 `53.webp`，另含 `back.webp`
- 用途：斗地主、跑得快、炸金花共用

最终提示词：

> Create one exact 4x2 sprite atlas of eight premium playing-card components: blank ivory gold-edged card face, ornate jade-and-gold card back, red diamond, black club, red heart, black spade, blue-gold small-joker royal jester medallion, red-gold big-joker royal emperor-jester medallion. Polished 3D Chinese mobile-game style, consistent lighting, flat #ff00ff chroma background, no text, numbers, logos, watermarks or grid lines.

组件去底后，由构建脚本把准确点数、花色与大小王固化到 54 张 WebP 图片中。游戏页面只加载成品图片，不再用 HTML 文字拼牌。

## 完整麻将牌面 v2

- GPT 原始图集：`mahjong-tiles-v2/atlas-source-v2.png`
- 透明组件图集：`mahjong-tiles-v2/atlas-v2.png`
- 完整切片：`mahjong-tiles-v2/tiles/0.webp` 至 `33.webp`，另含 `back.webp`
- 用途：经典麻将、红中麻将共用

最终提示词：

> Create one exact 4x2 sprite atlas of eight premium Chinese Mahjong components: blank ivory ceramic tile, cinnabar seal motif, green bamboo motif, blue concentric coin motif, jade-gold wind compass, red dragon flame emblem, green fortune knot, cobalt-blue whiteboard frame. Premium 3D Chinese mobile-game style, consistent lighting, flat #ff00ff chroma background, no readable text, numbers, logos, watermarks or grid lines.

构建脚本：`cardgames/scripts/build-gpt-card-assets.py`。脚本把准确的万、条、筒、东南西北中发白固化到 34 种 WebP 牌面中。
