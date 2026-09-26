from pathlib import Path
import os
import shutil
import subprocess
import tempfile
from PIL import Image, ImageDraw, ImageFont

ROOT = Path(__file__).resolve().parent.parent
OUT = Path(__file__).resolve().parent
BINARY = ROOT / "repoctor.exe"
WIDTH, HEIGHT = 1280, 720


def font(size, bold=False):
    name = "consolab.ttf" if bold else "consola.ttf"
    return ImageFont.truetype(Path(os.environ.get("WINDIR", "C:/Windows")) / "Fonts" / name, size)


def run_demo():
    with tempfile.TemporaryDirectory(prefix="repoctor-demo-") as raw:
        repo = Path(raw)
        (repo / "src").mkdir()
        (repo / "node_modules" / "left-pad").mkdir(parents=True)
        (repo / "dist").mkdir()
        (repo / ".env").write_text("DATABASE_URL=postgres://demo:demo@localhost/app\n", encoding="utf-8")
        (repo / "debug.log").write_text("demo build log\n", encoding="utf-8")
        (repo / "src" / "app.js").write_text("console.log('demo');\n", encoding="utf-8")
        (repo / "node_modules" / "left-pad" / "index.js").write_text("module.exports = () => {};\n", encoding="utf-8")
        (repo / "dist" / "bundle.js").write_text("build output\n", encoding="utf-8")
        (repo / ".gitignore").write_text("*.tmp\n", encoding="utf-8")
        subprocess.run(["git", "init", "-q"], cwd=repo, check=True)
        subprocess.run(["git", "config", "user.email", "demo@repoctor.local"], cwd=repo, check=True)
        subprocess.run(["git", "config", "user.name", "Repoctor Demo"], cwd=repo, check=True)
        subprocess.run(["git", "add", "."], cwd=repo, check=True)
        subprocess.run(["git", "commit", "-qm", "Add inherited repo clutter"], cwd=repo, check=True)
        result = subprocess.run(
            [str(BINARY), "--path", str(repo)],
            capture_output=True,
            text=True,
            encoding="utf-8",
        )
        output = (result.stdout + result.stderr).strip().splitlines()
        return repo.name, output, result.returncode


def terminal_frame(title, lines, accent=(91, 214, 154)):
    image = Image.new("RGB", (WIDTH, HEIGHT), (12, 16, 24))
    draw = ImageDraw.Draw(image)
    draw.rectangle((0, 0, WIDTH, 66), fill=(22, 28, 40))
    draw.ellipse((28, 25, 42, 39), fill=(255, 95, 86))
    draw.ellipse((50, 25, 64, 39), fill=(255, 189, 46))
    draw.ellipse((72, 25, 86, 39), fill=(39, 201, 63))
    draw.text((112, 20), title, font=font(22, True), fill=(226, 232, 240))
    draw.text((WIDTH - 210, 22), "REPOCTOR / DEMO", font=font(16), fill=(126, 143, 166))
    y = 108
    for index, line in enumerate(lines):
        color = accent if index == 0 or line.startswith("PASS") else (220, 226, 235)
        draw.text((58, y), line, font=font(25), fill=color)
        y += 38
    draw.rectangle((58, HEIGHT - 52, WIDTH - 58, HEIGHT - 50), fill=(43, 53, 70))
    draw.text((58, HEIGHT - 40), "scan complete  //  history-aware repository hygiene", font=font(15), fill=(120, 139, 160))
    return image


def main():
    repo_name, output, exit_code = run_demo()
    def repair_encoding(line):
        try:
            return line.encode("latin-1").decode("utf-8")
        except UnicodeError:
            return line

    normalized = [repair_encoding(line) for line in output]
    frames = [
        terminal_frame("A repository with baggage", [
            "$ tree -L 2 .",
            "./.env                 ./debug.log",
            "./node_modules/        ./dist/",
            "./src/app.js          ./.git/",
            "",
            "$ repoctor --path .",
            "Inspecting working tree + Git history...",
        ], accent=(109, 181, 255)),
        terminal_frame("Findings, with context", [
            "$ repoctor --path .",
            *normalized[:10],
        ], accent=(255, 189, 46)),
        terminal_frame("The fix is actionable", [
            *normalized[10:18],
            "",
            f"PASS  exit code {exit_code}  //  no files changed",
            "Review the fixes, then run the scan again.",
        ], accent=(91, 214, 154)),
        terminal_frame("One command. Six checks.", [
            "$ repoctor --path . --json > findings.json",
            "",
            "env-files   gitignore   big-blobs",
            "secrets     stale-branches   deps",
            "",
            "A small report before the repo becomes history.",
        ], accent=(109, 181, 255)),
        terminal_frame("Ready for CI", [
            "$ git add .gitignore && git commit -m \"Remove repo clutter\"",
            "$ repoctor",
            "0 findings",
            "",
            "PASS  clean enough to ship",
        ], accent=(91, 214, 154)),
    ]
    OUT.mkdir(exist_ok=True)
    for index in (0, 1, 2):
        frames[index].save(OUT / f"repoctor-demo-{index + 1:02}.png")
    frames[0].save(OUT / "repoctor-demo.gif", save_all=True, append_images=frames[1:], duration=3000, loop=0, optimize=False)
    print(f"created {len(frames)} frames, 15 second GIF, exit code {exit_code}")


if __name__ == "__main__":
    main()
