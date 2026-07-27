# AI 出图提示词包（雪碧图 + 切片约束）

用途：让 ChatGPT 一次生成多个素材（雪碧图），下载后用 `uniapp/scripts/slice-sprite.mjs` 按网格切片。

## 通用约束（每条提示词都已内嵌）

1. 严格 N×M 均分网格，每格内容居中
2. 格与格之间留 ≥40px 纯背景间距（供切片容错）
3. 背景纯色 `#F5F6FA`，四周留 40px 边距
4. **画面内不要出现任何文字/字母/数字**（AI 生成的中文必错）
5. 同图内统一光源、统一风格、统一饱和度
6. 不要画分割线、边框、水印

---

## 提示词 1：游戏分类封面（4 张，2×2）

```
Create a single 1024x1024 image containing a 2x2 grid of four mobile app category cover illustrations for an online entertainment platform.

Grid layout: exactly 2 columns x 2 rows, each cell centered, at least 40px of empty background between cells, 40px outer margin. Background: solid flat #F5F6FA. No dividing lines, no borders.

Style for all four: modern flat vector illustration with subtle gradients, soft ambient glow, clean geometric shapes, premium mobile-app quality, consistent top-left light source, vibrant but not neon-harsh.

Cell contents (clockwise from top-left):
1. Live casino: stylized playing cards fanned out, poker chips stacked, gold accents. Palette: deep purple #5A2EA6 to magenta #E05A86.
2. Football/sports: a soccer ball mid-air with motion arcs and a simplified stadium silhouette. Palette: dark green #063B2C to bright green #12A06B with lime #C9FF43 accent.
3. Lottery: colorful numbered lottery balls floating out of a transparent sphere (numbers should be abstract dots, NOT readable digits). Palette: orange #FF8A4D to pink #FF5878.
4. Board games / chess: abstract chess pieces and mahjong-like tiles arranged dynamically. Palette: indigo #2A1B6E to violet #7A5CFF.

Absolutely no text, letters, numbers, or logos anywhere in the image.
```

---

## 提示词 2：直播封面兜底（4 张，2×2）

```
Create a single 1024x1024 image containing a 2x2 grid of four abstract portrait-orientation background artworks used as placeholder covers for livestream rooms.

Grid layout: exactly 2 columns x 2 rows, each cell a 3:4 portrait rectangle centered in its cell, at least 40px empty background between cells, 40px outer margin. Background: solid flat #F5F6FA. No dividing lines.

Style: abstract atmospheric gradient art, soft bokeh light orbs, gentle nebula/aurora textures, subtle film grain, no human figures, no faces, no objects — purely abstract ambient backgrounds suitable as a neutral placeholder.

Cell palettes:
1. Purple to magenta cosmic nebula (#2A1B6E, #5A2EA6, #E05A86)
2. Teal to deep blue aurora (#0EA5A5, #2A1B6E)
3. Warm sunset pink to orange haze (#FF5878, #FF8A4D)
4. Deep green to lime atmospheric glow (#063B2C, #12A06B, #C9FF43)

Absolutely no text, letters, numbers, people, or logos.
```

---

## 提示词 3：等级勋章（6 枚，3×2）

```
Create a single 1536x1024 image containing a 3x2 grid of six mobile-game rank badge / medal icons.

Grid layout: exactly 3 columns x 2 rows, each badge centered in its cell, at least 40px empty background between cells, 40px outer margin. Background: solid flat #F5F6FA. No dividing lines.

Style: premium mobile game rank insignia, flat vector with soft inner gradients and a subtle outer glow, clean silhouette, front-facing, symmetrical, consistent size across all six (each roughly 380x380px of visual content).

Progression of six tiers, increasing in visual complexity and prestige:
1. Bronze shield, simple
2. Silver shield with small wings
3. Gold shield with laurel
4. Purple crystal star emblem
5. Diamond emblem with radiant rays
6. Crown emblem with gem cluster and aura

Absolutely no text, letters, numbers, or roman numerals anywhere.
```

---

## 提示词 4：空态插画（4 张，2×2）

```
Create a single 1024x1024 image containing a 2x2 grid of four minimal "empty state" illustrations for a mobile app.

Grid layout: exactly 2 columns x 2 rows, each illustration centered, at least 40px empty background between cells, 40px outer margin. Background: solid flat #F5F6FA. No dividing lines.

Style: light, friendly, minimal line-and-shape illustration with soft pastel fills, very light purple/pink brand tint (#7A5CFF, #FF5878 at low saturation), lots of white space, thin rounded strokes, no heavy outlines, no mascot characters.

Cell contents:
1. No livestreams: a simple switched-off screen/monitor shape with a small floating planet and sparkles
2. No posts/feed: a stack of empty rounded cards with a small paper plane flying away
3. No bet records: an empty ticket/receipt shape with a soft dashed edge and a tiny football
4. No search results: a magnifier over an empty rounded panel with two small dots

Absolutely no text, letters, or numbers.
```

---

## 提示词 5：首页 Banner（单张，横幅）

```
Create a single 1536x672 wide banner illustration for the top of a mobile entertainment app's game hub.

Composition: wide cinematic banner. Left 40% intentionally kept visually calm/empty (a soft gradient area where UI text will be overlaid later). Right 60% contains the artwork.

Artwork: a dynamic cosmic scene — a glowing ringed planet, floating lottery balls and playing-card shapes orbiting it, soft light streaks, scattered star particles, subtle depth-of-field.

Palette: deep indigo #2A1B6E through violet #5A2EA6 to magenta #E05A86, with warm highlights #FF8A4D and small lime #C9FF43 sparkles.

Style: premium mobile game key-art, flat-3D hybrid, clean and glossy but not plastic, soft ambient glow, high contrast focal point on the planet.

Absolutely no text, letters, numbers, or logos.
```

---

## 切片命令

下载雪碧图到 `~/Downloads` 后：

```bash
cd uniapp
# 分类封面：2x2
node scripts/slice-sprite.mjs ~/Downloads/<文件名>.png --grid 2x2 \
  --names casino,sports,lottery,board --out src/static/art/category

# 勋章：3x2
node scripts/slice-sprite.mjs ~/Downloads/<文件名>.png --grid 3x2 \
  --names lv1,lv2,lv3,lv4,lv5,lv6 --out src/static/art/medal --trim

# 空态：2x2
node scripts/slice-sprite.mjs ~/Downloads/<文件名>.png --grid 2x2 \
  --names live,feed,bet,search --out src/static/art/empty --trim
```

`--trim` 会自动裁掉四周纯背景边缘，让素材紧贴内容。
