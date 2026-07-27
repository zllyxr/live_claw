#!/usr/bin/env python3
from pathlib import Path

from PIL import Image


ROOT = Path(__file__).resolve().parents[1]
BRAND_DIR = ROOT / "docs" / "brand"
LOGO_RAW = BRAND_DIR / "xingyu-logo-raw.png"
SPLASH_RAW = BRAND_DIR / "xingyu-splash-raw.png"
LOGO_SOURCE = BRAND_DIR / "xingyu-logo-source.png"
SPLASH_SOURCE = BRAND_DIR / "xingyu-splash-source.png"

def cover(image, size, focus_y=0.5):
    width, height = image.size
    target_w, target_h = size
    scale = max(target_w / width, target_h / height)
    resized = image.resize((round(width * scale), round(height * scale)), Image.Resampling.LANCZOS)
    x = max(0, (resized.width - target_w) // 2)
    extra_y = max(0, resized.height - target_h)
    y = round(extra_y * focus_y)
    return resized.crop((x, y, x + target_w, y + target_h))


def make_logo_source():
    image = Image.open(LOGO_RAW).convert("RGBA")
    logo = cover(image, (1024, 1024), focus_y=0.5)
    logo.save(LOGO_SOURCE)
    return logo


def make_splash_source():
    raw = Image.open(SPLASH_RAW).convert("RGBA")
    splash = cover(raw, (1242, 2688), focus_y=0.5)
    splash.save(SPLASH_SOURCE)
    return splash


def save_icon_set(logo):
    ios_dir = ROOT / "ios" / "YBLive" / "Assets.xcassets" / "AppIcon.appiconset"
    icon_sizes = {
        "40_40.png": 40,
        "58_58.png": 58,
        "60_60.png": 60,
        "80_80.png": 80,
        "87_87.png": 87,
        "120_120.png": 120,
        "120_120 1.png": 120,
        "180_180.png": 180,
        "1024_1024.png": 1024,
    }
    for filename, size in icon_sizes.items():
        output = logo.resize((size, size), Image.Resampling.LANCZOS)
        output.convert("RGB").save(ios_dir / filename)

    android_sizes = {
        "mipmap-mdpi": 48,
        "mipmap-hdpi": 72,
        "mipmap-xhdpi": 96,
        "mipmap-xxhdpi": 144,
        "mipmap-xxxhdpi": 192,
    }
    for folder, size in android_sizes.items():
        android_dir = ROOT / "android" / "app" / "src" / "main" / "res" / folder
        output = logo.resize((size, size), Image.Resampling.LANCZOS)
        output.save(android_dir / "ic_launcher.webp", "WEBP", quality=95, method=6)
        output.save(android_dir / "ic_launcher_round.webp", "WEBP", quality=95, method=6)

    foreground_sizes = {
        "mipmap-mdpi": 108,
        "mipmap-hdpi": 162,
        "mipmap-xhdpi": 216,
        "mipmap-xxhdpi": 324,
        "mipmap-xxxhdpi": 432,
    }
    for folder, size in foreground_sizes.items():
        android_dir = ROOT / "android" / "app" / "src" / "main" / "res" / folder
        output = logo.resize((size, size), Image.Resampling.LANCZOS)
        output.save(android_dir / "ic_launcher_foreground.webp", "WEBP", quality=95, method=6)

    logo.resize((500, 500), Image.Resampling.LANCZOS).save(
        ROOT / "android" / "app" / "src" / "main" / "res" / "mipmap-mdpi" / "icon_app.png"
    )


def save_launch_set(splash):
    ios_dir = ROOT / "ios" / "YBLive" / "Assets.xcassets" / "LaunchImage.launchimage"
    launch_sizes = {
        "640_960.png": (640, 960),
        "640_1136.png": (640, 1136),
        "750_1334.png": (750, 1334),
        "1125_2436.png": (1125, 2436),
        "1242_2208.png": (1242, 2208),
        "1242_2688.png": (1242, 2688),
        "828_1792.png": (828, 1792),
    }
    for filename, size in launch_sizes.items():
        cover(splash, size, focus_y=0.72).convert("RGB").save(ios_dir / filename)

    android_screen = cover(splash, (720, 1280), focus_y=0.72)
    android_screen.save(ROOT / "android" / "app" / "src" / "main" / "res" / "mipmap-mdpi" / "screen.png")


def main():
    BRAND_DIR.mkdir(parents=True, exist_ok=True)
    logo = make_logo_source()
    splash = make_splash_source()
    save_icon_set(logo)
    save_launch_set(splash)


if __name__ == "__main__":
    main()
