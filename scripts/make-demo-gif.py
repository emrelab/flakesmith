#!/usr/bin/env python3
"""Render docs/go-flake-hunter-demo.gif from the binary's real output.

Why a script and not a screen recording: the demo is a claim about the tool, so
it has to be re-makeable when the output changes. A hand-recorded GIF is a
picture nobody can regenerate, and the last one still showed a prompt, a capsule
prefix and a build tag that no longer existed.

The output is captured by running the built binary against
testdata/fixtures/order-basic — every line in the GIF is bytes the tool printed.
Only the rendering is synthetic: a typing effect over the real command line and
the real output appearing line by line, in a terminal-window frame.

    python3 scripts/make-demo-gif.py            # writes docs/go-flake-hunter-demo.gif
"""

from __future__ import annotations

import argparse
import os
import shutil
import subprocess
import sys
import tempfile

from PIL import Image, ImageDraw, ImageFont

WIDTH, HEIGHT = 900, 640
TITLE_H = 30
PAD = 16
FONT_SIZE = 13
BG = (29, 30, 32)
BAR = (46, 47, 50)
FG = (222, 222, 222)
DIM = (138, 141, 146)
GREEN = (126, 199, 124)
CYAN = (110, 178, 209)
YELLOW = (224, 187, 108)

DEMO_DIR = os.path.join("testdata", "fixtures", "order-basic")
COMMANDS = (
    "go-flake-hunter hunt .",
    "go-flake-hunter compile --print",
)


def load_font() -> ImageFont.FreeTypeFont:
    for path in (
        "/System/Library/Fonts/Menlo.ttc",
        "/System/Library/Fonts/SFNSMono.ttf",
        "/usr/share/fonts/truetype/dejavu/DejaVuSansMono.ttf",
    ):
        if os.path.exists(path):
            return ImageFont.truetype(path, FONT_SIZE)
    sys.exit("make-demo-gif: no monospace font found")


def run(binary: str, args: list[str], cwd: str) -> list[str]:
    proc = subprocess.run([binary, *args], cwd=cwd, capture_output=True, text=True)
    if proc.returncode != 0:
        sys.exit(f"make-demo-gif: {' '.join(args)} exited {proc.returncode}\n{proc.stderr}")
    return proc.stdout.rstrip("\n").split("\n")


def colour(line: str) -> tuple[int, int, int]:
    if line.startswith("flake found"):
        return GREEN
    if line.startswith("  tried"):
        return DIM
    if line.lstrip().startswith("- "):
        return YELLOW
    if line.startswith("//"):
        return CYAN
    if line.startswith(("//go:build", "package ")):
        return GREEN
    return FG


def render(lines: list[str], partial: str, font: ImageFont.FreeTypeFont) -> Image.Image:
    img = Image.new("RGB", (WIDTH, HEIGHT), BG)
    draw = ImageDraw.Draw(img)

    # Window frame: title bar, traffic lights, then the terminal body.
    draw.rectangle([0, 0, WIDTH, TITLE_H], fill=BAR)
    for i, c in enumerate(((255, 95, 86), (255, 189, 46), (39, 201, 63))):
        draw.ellipse([12 + i * 20, 9, 24 + i * 20, 21], fill=c)
    title = "go-flake-hunter - demo"
    tw = draw.textlength(title, font=font)
    draw.text(((WIDTH - tw) / 2, 7), title, font=font, fill=(190, 190, 190))

    y = TITLE_H + PAD
    # The last visible line is the command being typed; everything above is output.
    for line in lines:
        draw.text((PAD, y), line, font=font, fill=colour(line))
        y += FONT_SIZE + 4
    if partial:
        draw.text((PAD, y), "$ ", font=font, fill=GREEN)
        draw.text((PAD + draw.textlength("$ ", font=font), y), partial, font=font, fill=FG)
    return img


def main() -> int:
    ap = argparse.ArgumentParser()
    ap.add_argument("--binary", default="./go-flake-hunter")
    ap.add_argument("--out", default="docs/go-flake-hunter-demo.gif")
    args = ap.parse_args()

    binary = os.path.abspath(args.binary)
    if not os.path.exists(binary):
        sys.exit(f"make-demo-gif: {binary} missing — run `make build` first")

    font = load_font()
    per_line = FONT_SIZE + 4
    # One line of slack at the bottom: a full viewport clips the descenders of
    # the last line, which reads as a rendering bug in the docs.
    viewport = (HEIGHT - TITLE_H - 2 * PAD) // per_line - 1

    # Capture everything first: the rendering below must never wait on a process.
    demo = os.path.abspath(DEMO_DIR)
    capsule_dir = os.path.join(demo, ".go-flake-hunter")
    shutil.rmtree(capsule_dir, ignore_errors=True)
    steps: list[tuple[str, object]] = []
    try:
        for command in COMMANDS:
            steps.append(("type", command))
            for line in run(binary, command.split()[1:], demo):
                steps.append(("line", line))
            steps.append(("line", ""))
    finally:
        shutil.rmtree(capsule_dir, ignore_errors=True)

    frames: list[Image.Image] = []
    screen: list[str] = []

    def snap(partial: str = "") -> None:
        frames.append(render(screen[-viewport:], partial, font))

    for kind, payload in steps:
        if kind == "type":
            command = str(payload)
            for i in range(3, len(command) + 3, 3):  # three characters per frame
                snap(command[: min(i, len(command))])
            screen.append("$ " + command)
        else:
            snap()  # the screen before this line lands
            screen.append(str(payload))
    for _ in range(14):  # hold on the finished screen
        snap()

    out = os.path.abspath(args.out)
    os.makedirs(os.path.dirname(out), exist_ok=True)
    # One palette per frame (median cut) keeps text crisp; a single shared palette
    # bands the anti-aliased glyphs into unreadable blocks.
    palette = [f.convert("RGB").quantize(colors=24, method=Image.Quantize.MEDIANCUT) for f in frames]
    palette[0].save(
        out,
        save_all=True,
        append_images=palette[1:],
        duration=60,
        loop=0,
        optimize=True,
        disposal=1,
    )
    print(f"make-demo-gif: {out} ({len(frames)} frames, {os.path.getsize(out)} bytes)")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
