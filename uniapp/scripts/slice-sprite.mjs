#!/usr/bin/env node
/**
 * 雪碧图按网格切片工具
 *
 * 用法:
 *   node scripts/slice-sprite.mjs <图片> --grid 2x2 --names a,b,c,d --out src/static/art/xx
 *
 * 选项:
 *   --grid  CxR      网格列数x行数（必填）
 *   --names a,b,c    每格输出文件名（按从左到右、从上到下顺序）
 *   --out   目录     输出目录，默认 src/static/art
 *   --trim           自动裁掉四周纯色背景边缘
 *   --pad   N        每格向内缩进 N 像素再切，规避格间残留（默认 0）
 *   --size  N        输出统一缩放到 N×N（保持比例，居中留白）
 *   --webp           额外输出 webp（体积更小）
 *
 * 依赖 python3 + Pillow（项目已安装）。
 */
import { execFileSync } from "node:child_process";
import { existsSync, mkdirSync } from "node:fs";
import { resolve, dirname } from "node:path";
import { fileURLToPath } from "node:url";

const here = dirname(fileURLToPath(import.meta.url));
const root = resolve(here, "..");

function arg(name, fallback) {
  const i = process.argv.indexOf(`--${name}`);
  if (i === -1) return fallback;
  const v = process.argv[i + 1];
  return v && !v.startsWith("--") ? v : true;
}

const input = process.argv[2];
if (!input || input.startsWith("--")) {
  console.error("用法: node scripts/slice-sprite.mjs <图片> --grid 2x2 [--names a,b,c,d] [--out 目录] [--trim]");
  process.exit(1);
}
const src = resolve(process.cwd(), input.replace(/^~/, process.env.HOME || "~"));
if (!existsSync(src)) {
  console.error(`✗ 找不到文件: ${src}`);
  process.exit(1);
}

const grid = String(arg("grid", ""));
const m = grid.match(/^(\d+)x(\d+)$/i);
if (!m) {
  console.error("✗ --grid 必须形如 2x2 / 3x2");
  process.exit(1);
}
const cols = Number(m[1]);
const rows = Number(m[2]);
const names = String(arg("names", "") || "").split(",").map((s) => s.trim()).filter(Boolean);
const outDir = resolve(root, String(arg("out", "src/static/art")));
const doTrim = arg("trim") !== undefined;
const pad = Number(arg("pad", 0)) || 0;
const size = Number(arg("size", 0)) || 0;
const doWebp = arg("webp") !== undefined;

mkdirSync(outDir, { recursive: true });

const py = `
import sys, os
from PIL import Image, ImageChops

src, out_dir = sys.argv[1], sys.argv[2]
cols, rows, pad, size = int(sys.argv[3]), int(sys.argv[4]), int(sys.argv[5]), int(sys.argv[6])
do_trim, do_webp = sys.argv[7] == '1', sys.argv[8] == '1'
names = [n for n in sys.argv[9].split(',') if n]

img = Image.open(src).convert('RGBA')
W, H = img.size
cw, ch = W // cols, H // rows
print(f'源图 {W}x{H} → 每格 {cw}x{ch}')

def trim_bg(im):
    """裁掉四周与四角同色的纯背景"""
    rgb = im.convert('RGB')
    bg = Image.new('RGB', rgb.size, rgb.getpixel((0, 0)))
    diff = ImageChops.difference(rgb, bg)
    # 提高容差，避免渐变背景裁不掉
    bbox = diff.point(lambda p: 255 if p > 18 else 0).convert('L').getbbox()
    return im.crop(bbox) if bbox else im

made = []
for r in range(rows):
    for c in range(cols):
        idx = r * cols + c
        box = (c*cw + pad, r*ch + pad, (c+1)*cw - pad, (r+1)*ch - pad)
        cell = img.crop(box)
        if do_trim:
            cell = trim_bg(cell)
        if size:
            cell.thumbnail((size, size), Image.LANCZOS)
            canvas = Image.new('RGBA', (size, size), (0, 0, 0, 0))
            canvas.paste(cell, ((size - cell.width)//2, (size - cell.height)//2), cell)
            cell = canvas
        name = names[idx] if idx < len(names) else f'cell_{idx+1}'
        p = os.path.join(out_dir, name + '.png')
        cell.save(p, optimize=True)
        made.append((p, cell.size, os.path.getsize(p)))
        if do_webp:
            wp = os.path.join(out_dir, name + '.webp')
            cell.save(wp, 'WEBP', quality=88, method=6)
            made.append((wp, cell.size, os.path.getsize(wp)))

for p, sz, b in made:
    print(f'  ✔ {os.path.basename(p):<22} {sz[0]}x{sz[1]:<5} {b/1024:.1f} KB')
print(f'共输出 {len(made)} 个文件 → {out_dir}')
`;

try {
  execFileSync(
    "python3",
    ["-c", py, src, outDir, String(cols), String(rows), String(pad), String(size),
      doTrim ? "1" : "0", doWebp ? "1" : "0", names.join(",")],
    { stdio: "inherit" }
  );
} catch {
  console.error("✗ 切片失败（确认已安装 Pillow: python3 -m pip install --user Pillow）");
  process.exit(1);
}
