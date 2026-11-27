#!/usr/bin/env bash
set -euo pipefail

usage() {
  cat <<'USAGE'
check-fullwidth-var-expansion.sh — shell 變數展開緊接全角字元檢查

用法:
  scripts/check-fullwidth-var-expansion.sh        執行檢查（CI 與本地共用）
  scripts/check-fullwidth-var-expansion.sh --help 顯示本說明

bash 按位元組決定變數名邊界，`$var，` 會把全角逗號的位元組吃進變數名，於 set -u
下變成 unbound variable 而中止——且只在被踩到的那條路徑上爆炸。改寫成 `${var}`
即可。詳見腳本開頭註解與 issue #2025。
USAGE
}

if [ "${1:-}" = "--help" ] || [ "${1:-}" = "-h" ]; then
  usage
  exit 0
fi

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$REPO_ROOT"

python3 - <<'PY'
import re, subprocess, sys, pathlib

SHEBANG = re.compile(rb"^#!.*\b(bash|sh|zsh)\b")
SHELL_EXT = {".sh", ".bash", ".bats"}

# $name 後緊接非 ASCII。name 必須以字母/底線開頭（排除 $1 / $? / $# 等安全情況）。
PAT = re.compile(r"\$([A-Za-z_][A-Za-z0-9_]*)([^\x00-\x7F])")
# Makefile 的 recipe 裡，shell 變數是 $$name（$(name) 是 make 的變數，不在範圍內）。
PAT_MAKE = re.compile(r"\$\$([A-Za-z_][A-Za-z0-9_]*)([^\x00-\x7F])")


def is_makefile(path: pathlib.Path) -> bool:
    return path.name == "Makefile" or path.suffix == ".mk"

EXEMPT = "fullwidth-var-ok"


def candidates():
    listing = subprocess.run(
        ["git", "ls-files", "-z"], check=True, capture_output=True
    ).stdout
    for raw in listing.split(b"\0"):
        if not raw:
            continue
        path = pathlib.Path(raw.decode("utf-8", "surrogateescape"))
        if not path.is_file():
            continue
        if path.suffix in SHELL_EXT or is_makefile(path):
            yield path
            continue
        try:
            with path.open("rb") as fh:
                if SHEBANG.match(fh.readline()):
                    yield path
        except OSError:
            continue


def expandable_spans(line: str, heredoc: dict):
    """回傳本行「會被 shell 展開」的 (start, end) 區間清單。

    輕量狀態機，不是完整 shell parser：追蹤單/雙引號與行內註解，並在呼叫端追蹤
    heredoc。刻意偏保守（寧可漏報也不誤擋 CI）：無法判定的結構一律當成不展開。
    """
    # quoted heredoc（<<'EOF'）內整段不展開；unquoted heredoc（<<EOF）內會展開。
    if heredoc.get("tag") is not None:
        return [(0, len(line))] if heredoc["expand"] else []

    spans, i, n = [], 0, len(line)
    start = 0  # 當前「展開區」起點
    quote = None
    while i < n:
        ch = line[i]
        if quote == "'":
            if ch == "'":
                quote = None
                start = i + 1
            i += 1
            continue
        if ch == "\\":  # 轉義：跳過下一個字元（\$ 因此不被視為展開）
            i += 2
            continue
        if quote == '"':
            if ch == '"':
                quote = None
            i += 1
            continue
        # 非引號狀態
        if ch == "'":
            spans.append((start, i))
            quote = "'"
            i += 1
            continue
        if ch == '"':
            quote = '"'
            i += 1
            continue
        if ch == "#" and (i == 0 or line[i - 1].isspace()):
            spans.append((start, i))  # 行內註解起點 → 後面全部不展開
            return spans
        i += 1
    spans.append((start, n))
    return spans


def is_escaped(line: str, pos: int) -> bool:
    """line[pos] 的 `$` 是否被反斜線轉義（前面連續反斜線為奇數個）。

    spans 只判斷「這一段是否在會展開的引號狀態」，逐字元的 `\\$` 字面值要在這裡濾
    掉，否則會誤報印 API 路徑樣板的 `\\$id`。
    """
    count = 0
    j = pos - 1
    while j >= 0 and line[j] == "\\":
        count += 1
        j -= 1
    return count % 2 == 1


def track_heredoc(line: str, heredoc: dict):
    """更新 heredoc 狀態（在判定完本行之後呼叫）。"""
    tag = heredoc.get("tag")
    if tag is not None:
        if line.strip() == tag:  # 收尾行（<<- 的 tab 縮排已被 strip 吃掉）
            heredoc["tag"] = None
        return
    m = re.search(r"<<-?\s*(['\"]?)([A-Za-z_][A-Za-z0-9_]*)\1", line)
    if m:
        heredoc["tag"] = m.group(2)
        heredoc["expand"] = m.group(1) == ""  # 有引號 → 不展開


violations = []
scanned = 0

for path in candidates():
    try:
        text = path.read_text(encoding="utf-8")
    except (OSError, UnicodeDecodeError):
        continue
    scanned += 1
    make = is_makefile(path)
    pat = PAT_MAKE if make else PAT
    has_u = re.search(r"^\s*set\s+-[a-zA-Z]*u", text, re.M) is not None
    heredoc = {"tag": None, "expand": True}
    for idx, line in enumerate(text.splitlines(), start=1):
        if EXEMPT in line or (make and not line.startswith("\t")):
            track_heredoc(line, heredoc)
            continue
        for s, e in expandable_spans(line, heredoc):
            for m in pat.finditer(line, s, e):
                if is_escaped(line, m.start()):
                    continue
                consequence = (
                    "set -u → unbound variable，該路徑直接中止"
                    if has_u
                    else "無 set -u → 展開成空字串，訊息靜默少字"
                )
                sigil = "$$" if make else "$"
                violations.append(
                    f"{path}:{idx} — `{sigil}{m.group(1)}` 緊接全角字元 "
                    f"`{m.group(2)}`；{consequence}。改寫成 `{sigil}{{{m.group(1)}}}`"
                )
        track_heredoc(line, heredoc)

if not scanned:
    print("::error::掃描分母為 0——一支 shell 腳本都沒掃到，檔案清單或路徑壞了，"
          "不是「沒有腳本」。", file=sys.stderr)
    sys.exit(1)

if violations:
    print(
        "shell 變數展開緊接全角字元（bash 會把多位元組字元吃進變數名）：\n",
        file=sys.stderr,
    )
    for v in violations:
        print(f"  ✗ {v}", file=sys.stderr)
    print(
        "\n修法：把 `$var` 改成 `${var}` 明確界定變數名邊界。"
        "\n這類 bug 只在被踩到的那條路徑上爆炸（dev.sh 的健康檢查異常分支因此失效 9 天、"
        "\nprod 冒煙測試因此從未跑完），光靠端到端測試擋不住。"
        "\n確有必要保留裸寫法，在該行加 `# fullwidth-var-ok` 並於 PR 說明理由。",
        file=sys.stderr,
    )
    sys.exit(1)

print(f"shell 變數展開檢查通過（掃了 {scanned} 支腳本與 Makefile）✓")
PY
