#!/usr/bin/env python3
"""Build complete image-only poker and Mahjong sets from GPT-generated component atlases."""

from pathlib import Path
from PIL import Image, ImageDraw, ImageFont

ROOT = Path(__file__).resolve().parents[1]
POKER_ROOT = ROOT / "public/assets/gpt/playing-cards-v2"
MAHJONG_ROOT = ROOT / "public/assets/gpt/mahjong-tiles-v2"
POKER_OUT = POKER_ROOT / "deck"
MAHJONG_OUT = MAHJONG_ROOT / "tiles"

ARIAL_BOLD = "/System/Library/Fonts/Supplemental/Arial Bold.ttf"
SONGTI = "/System/Library/Fonts/Supplemental/Songti.ttc"


def atlas_cell(image: Image.Image, col: int, row: int, columns=4, rows=2) -> Image.Image:
    x0 = round(image.width * col / columns)
    x1 = round(image.width * (col + 1) / columns)
    y0 = round(image.height * row / rows)
    y1 = round(image.height * (row + 1) / rows)
    return image.crop((x0, y0, x1, y1))


def alpha_crop(image: Image.Image, padding=2) -> Image.Image:
    alpha = image.getchannel("A")
    bbox = alpha.getbbox()
    if not bbox:
        return image
    left = max(0, bbox[0] - padding)
    top = max(0, bbox[1] - padding)
    right = min(image.width, bbox[2] + padding)
    bottom = min(image.height, bbox[3] + padding)
    return image.crop((left, top, right, bottom))


def contain(image: Image.Image, size: tuple[int, int], inset=0) -> Image.Image:
    target = Image.new("RGBA", size, (0, 0, 0, 0))
    available = (max(1, size[0] - inset * 2), max(1, size[1] - inset * 2))
    fitted = alpha_crop(image).copy()
    fitted.thumbnail(available, Image.Resampling.LANCZOS)
    target.alpha_composite(fitted, ((size[0] - fitted.width) // 2, (size[1] - fitted.height) // 2))
    return target


def save_webp(image: Image.Image, path: Path):
    path.parent.mkdir(parents=True, exist_ok=True)
    image.save(path, "WEBP", quality=96, method=6, lossless=False)


def colored_icon(icon: Image.Image, size: tuple[int, int]) -> Image.Image:
    return contain(icon, size)


def build_poker():
    atlas = Image.open(POKER_ROOT / "atlas-v2.png").convert("RGBA")
    components = [
        alpha_crop(atlas_cell(atlas, col, row))
        for row in range(2)
        for col in range(4)
    ]
    face, back, diamond, club, heart, spade, joker_small, joker_big = components
    icons = [diamond, club, heart, spade]
    ranks = ["3", "4", "5", "6", "7", "8", "9", "10", "J", "Q", "K", "A", "2"]
    font = ImageFont.truetype(ARIAL_BOLD, 42)
    font_small = ImageFont.truetype(ARIAL_BOLD, 34)
    joker_font = ImageFont.truetype(SONGTI, 27)
    canvas_size = (180, 252)
    base_face = contain(face, canvas_size)
    save_webp(contain(back, canvas_size), POKER_OUT / "back.webp")

    for rank_index, rank in enumerate(ranks):
        for suit in range(4):
            card_id = rank_index * 4 + suit
            card = base_face.copy()
            draw = ImageDraw.Draw(card)
            text_font = font_small if rank == "10" else font
            color = (190, 35, 38, 255) if suit in (0, 2) else (24, 27, 29, 255)
            draw.text((19, 14), rank, font=text_font, fill=color, stroke_width=1, stroke_fill=(255, 244, 211, 220))
            corner_icon = colored_icon(icons[suit], (30, 30))
            card.alpha_composite(corner_icon, (18, 59))
            center_icon = colored_icon(icons[suit], (76, 76))
            card.alpha_composite(center_icon, ((canvas_size[0] - 76) // 2, 101))

            corner = Image.new("RGBA", (55, 82), (0, 0, 0, 0))
            corner_draw = ImageDraw.Draw(corner)
            corner_draw.text((2, 0), rank, font=text_font, fill=color, stroke_width=1, stroke_fill=(255, 244, 211, 220))
            corner.alpha_composite(corner_icon, (1, 48))
            corner = corner.rotate(180, resample=Image.Resampling.BICUBIC, expand=False)
            card.alpha_composite(corner, (109, 162))
            save_webp(card, POKER_OUT / f"{card_id}.webp")

    for card_id, medallion, label, color in [
        (52, joker_small, "小王", (25, 91, 155, 255)),
        (53, joker_big, "大王", (181, 36, 35, 255)),
    ]:
        card = base_face.copy()
        icon = contain(medallion, (126, 148))
        card.alpha_composite(icon, (27, 61))
        draw = ImageDraw.Draw(card)
        draw.text((18, 13), label, font=joker_font, fill=color, stroke_width=1, stroke_fill=(255, 244, 211, 230))
        save_webp(card, POKER_OUT / f"{card_id}.webp")


def label_center(draw: ImageDraw.ImageDraw, text: str, y: int, font: ImageFont.FreeTypeFont, fill, width=180):
    bbox = draw.textbbox((0, 0), text, font=font, stroke_width=1)
    x = (width - (bbox[2] - bbox[0])) // 2
    draw.text((x, y), text, font=font, fill=fill, stroke_width=1, stroke_fill=(255, 247, 221, 230))


def build_mahjong():
    atlas = Image.open(MAHJONG_ROOT / "atlas-v2.png").convert("RGBA")
    components = [
        alpha_crop(atlas_cell(atlas, col, row))
        for row in range(2)
        for col in range(4)
    ]
    blank, wan, bamboo, coin, wind, dragon, fortune, whiteboard = components
    style_tiles = [wan, bamboo, coin]
    canvas_size = (180, 240)
    number_font = ImageFont.truetype(ARIAL_BOLD, 54)
    suit_font = ImageFont.truetype(SONGTI, 36)
    honor_font = ImageFont.truetype(SONGTI, 66)
    suit_labels = ["万", "条", "筒"]
    suit_colors = [(180, 37, 31, 255), (19, 113, 67, 255), (29, 79, 147, 255)]

    for suit in range(3):
        styled = contain(style_tiles[suit], canvas_size)
        for number in range(1, 10):
            tile = styled.copy()
            draw = ImageDraw.Draw(tile)
            draw.rounded_rectangle((54, 50, 126, 155), radius=18, fill=(255, 250, 230, 212))
            label_center(draw, str(number), 54, number_font, suit_colors[suit])
            label_center(draw, suit_labels[suit], 124, suit_font, suit_colors[suit])
            save_webp(tile, MAHJONG_OUT / f"{suit * 9 + number - 1}.webp")

    honors = [
        (27, wind, "东", (21, 102, 70, 255)),
        (28, wind, "南", (21, 102, 70, 255)),
        (29, wind, "西", (21, 102, 70, 255)),
        (30, wind, "北", (21, 102, 70, 255)),
        (31, dragon, "中", (187, 36, 31, 255)),
        (32, fortune, "发", (18, 113, 62, 255)),
        (33, whiteboard, "白", (32, 73, 142, 255)),
    ]
    for tile_id, background, label, color in honors:
        tile = contain(background, canvas_size)
        draw = ImageDraw.Draw(tile)
        draw.ellipse((50, 66, 130, 154), fill=(255, 250, 230, 205))
        bbox = draw.textbbox((0, 0), label, font=honor_font, stroke_width=1)
        x = (canvas_size[0] - (bbox[2] - bbox[0])) // 2
        draw.text((x, 65), label, font=honor_font, fill=color, stroke_width=1, stroke_fill=(255, 247, 221, 235))
        save_webp(tile, MAHJONG_OUT / f"{tile_id}.webp")

    back = contain(blank, canvas_size)
    overlay = Image.new("RGBA", canvas_size, (0, 0, 0, 0))
    overlay_draw = ImageDraw.Draw(overlay)
    overlay_draw.rounded_rectangle((18, 16, 162, 222), radius=25, fill=(7, 78, 57, 238), outline=(219, 176, 60, 255), width=5)
    overlay_draw.rounded_rectangle((31, 29, 149, 209), radius=18, outline=(245, 213, 111, 180), width=3)
    back.alpha_composite(overlay)
    wind_mark = contain(wind, (86, 108))
    wind_mark.putalpha(wind_mark.getchannel("A").point(lambda value: int(value * 0.42)))
    back.alpha_composite(wind_mark, (47, 66))
    save_webp(back, MAHJONG_OUT / "back.webp")


if __name__ == "__main__":
    build_poker()
    build_mahjong()
    poker_count = len(list(POKER_OUT.glob("*.webp")))
    mahjong_count = len(list(MAHJONG_OUT.glob("*.webp")))
    print(f"Built {poker_count} poker assets and {mahjong_count} Mahjong assets")
