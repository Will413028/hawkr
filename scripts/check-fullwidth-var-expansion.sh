#!/usr/bin/env bash
# =============================================================================
# check-fullwidth-var-expansion.sh
#   shell 腳本中 `$var` 不得緊接全角／非 ASCII 字元（要寫 `${var}`）
# =============================================================================
# 為何存在（issue #2025 週邊，2026-07-30）：bash 決定變數名邊界時是按「位元組」掃
# 的，遇到多位元組（UTF-8）字元不會停——`$total，` 會被解析成變數名 `total\xef`
# （全角逗號的第一個位元組被吃進名字裡）。於是：
#   * `set -u` 下 → `total?: unbound variable`，非互動 shell 直接中止
#   * 無 `set -u` → 展開成空字串，訊息靜默少字（假的輸出）
#
# 本 repo 全繁體中文輸出，`echo "...$var（...）"`、`...$var，...` 是最自然的寫法，
# 因此這個地雷密度高、且**只在被踩到的那條路徑上爆炸**，靜態掃描是唯一可靠防線：
#
#   1. `dev.sh:459` — `($healthy/$total，$problem 異常)`，健康檢查的 DEGRADED 分支
#      從來沒執行成功過（只吐一行 bash 錯誤、不給判定）。6 個服務壞 9 天沒被發現。
#   2. `scripts/prod-smoke-test.sh` — `（ID=$SELLER_ID）` 在 **token 產生成功**
#      的正常路徑上，整支 prod 部署後冒煙測試在打第一個 API 前就死掉。
#   3. `docs/代碼管理（CodeMeta）/scripts/generate.sh:939` — 靠第 23 行為了 awk 而設
#      的 `export LC_ALL=C` 意外擋住（C locale 下 bash 不把 UTF-8 位元組收進識別
#      字）。那行註解寫的是「awk 多位元組 offset」，與此毫無關聯——哪天 awk 換掉、
#      那行被刪，CodeMeta 生成器的「找不到 md 檔」錯誤路徑立刻變成 unbound。
#
# 判定規則：`$name` 後緊接非 ASCII 位元組，且該處會被 shell 展開。以下**不算**違規：
#   * `${name}` / `"$name"`（已有明確邊界）
#   * `$1` / `$?` / `$#` 等位置參數與特殊變數（只吃一個字元，實測安全）
#   * 單引號字串、註解、quoted heredoc（`<<'EOF'`）內 —— 不展開
#   * 轉義字面值 `\$name`（印出變數名而非其值，例如 API 路徑樣板）
# 真有必要保留裸寫法時，在該行加行內註解 `# fullwidth-var-ok` 豁免（請附理由）。
#
# Makefile（含 *.mk）的 recipe 行（tab 開頭）也掃：recipe 是 shell，其中 shell 變數寫成 `$$name`，
# 同樣要寫成 `$${name}`（2026-09-27 backend/Makefile 的 sqlc 略過訊息印出亂碼）。
#
# 已知未覆蓋（刻意的範圍界定，不是漏掉）：
#   * `.github/workflows/*.yml` 的 `run:` block —— 需 YAML 解析才能正確定位行號；
#     且 Actions 的 run 是 `bash -e`（無 `-u`），後果僅「log 少一個字」而非中止。
#   * `.claude/` `.agents/` 等未進版控的本機工具目錄（CI 檢不到，也不影響 CI）。
#
# 相依：純 bash + python3 標準庫（對齊 self-hosted light runner 慣例）。
# 用法：scripts/check-fullwidth-var-expansion.sh [--help]
#   exit 0 = 乾淨；exit 1 = 有違規（列出 檔案:行號 + 該變數 + 後果）。
# =============================================================================
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

# 候選檔＝版控內的 shell 腳本與 Makefile。副檔名認 .sh/.bash/.bats，另外收有 shell shebang 的
# 無副檔名檔（用 git ls-files 而非 find，避免掃到 node_modules / 生成物）。
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
