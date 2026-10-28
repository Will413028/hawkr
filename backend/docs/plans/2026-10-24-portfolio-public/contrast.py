#!/usr/bin/env python3
"""步驟 4b 的一次性檢查：三端新色票的前景色 × 背景色 WCAG 對比。

讀三端與 admin 的 token 值（不寫死色碼）：
  - website  styles/foundation/00-color-config.css、01-tokens-theme.css、styles/components/ui-kit.css
  - admin    src/app/globals.css、src/styles/AdminVisualRefresh.css（側欄深底）
  - buyer-app  lib/core/theme/app_colors.dart
  - seller-app lib/core/theme/app_colors.dart

另掃使用處：淡底（primary 的半透明選取底、surface-panel）上的強調文字必須用 text-accent-on-tint
（App 是 textAccentOnTint），不得用 primaryStrong 系的 token；違反就列出檔案與行號並 exit 1。
另掃四類使用處（scan_usage_extra）：(a) 輔色（secondary／accentFill）當文字色，或輔色底上放淺色字；
(b) 透明度寫法的底（bg-hk-*/NN、rgb(var(--x-rgb) / a)、Dart withOpacity／withValues(alpha:)）以白與 page 合成後的對比；
(c) order_color_tokens.dart 這類前景與底成對定義的狀態 badge；(d) 暗色底上的主色文字與圖形（文字 4.5、圖形 3）。

另掃第三類（scan_usage_text）：(1) 品牌色系的淺色 token（對白底 < 3:1，如 secondarySoft、primarySoft、secondary）被用在文字 `color:` 或 Dart `TextStyle(color:)`，
同規則／同 widget 沒有夠深的底就不合格；(2) Dart 依「解析到的最終色」判斷（別名先解析，解析後等於輔色的字就不合格），不看名字；
(3) 掃 admin 的 CSS 與 TSX（admin 沒有暗色模式，只看亮色）；(4) 固定色（品牌色 token）的亮色規則沒有暗色覆寫時，在暗色 page／card／elevated 上也算一次
（半透明底疊上去），TSX 的 `dark:bg-*` 群組與同變體的字色比對。
限制（這支腳本是啟發式，不是語意分析）：
  - 找「同 widget 的底」只往前看 14 行；CSS 只看同一條規則，不看祖先元素的底；找不到底的 on-dark 色只放行白、cream、inkOnDark 三個值。
  - 狀態色（status-*、warning、danger…）與中性灰的文字不在 (1) 範圍（D11：狀態色保留原值）；Dart (1) 只判斷名稱鏈含 brand／primary／secondary／accent 的別名。
  - (4) 的既有缺口記在 KNOWN_DARK_GAPS（每個檔的筆數，舊綠色時代就沒有暗色覆寫，值只是跟著換成橘；不得增加），不是已修好；新增的規則會被擋。
  - 純裝飾字（DECORATIVE_SELECTORS）與深色漸層底（ON_DARK_FILES）是明列的例外。
  - TSX 只看字串字面值內的 Tailwind class，不看條件式組出來的 class。

規則（WCAG 2.x）：一般文字 < 4.5 或非文字 UI 元件 < 3.0 就 exit 1；
大字（>= 24px，或 >= 18.66px 粗體）門檻 3.0。狀態色（成功、警告、危險、資訊）值沒變，
列在「狀態色（值未變，只列出不判定）」，不參與 exit code。

用法：python3 contrast.py [--root <repo 根目錄>] [--quiet]
"""

from __future__ import annotations

import argparse
import fnmatch
import re
import subprocess
import sys
from pathlib import Path

DEFAULT_ROOT = Path(__file__).resolve().parents[4]


def tracked_files(root: Path, directory: str, pattern: str = "*") -> list[Path]:
    """git 追蹤中、位於 directory 之下、檔名符合 pattern 的檔（不掃 .gitignore 排除的 build 輸出）。"""
    out = subprocess.run(
        ["git", "ls-files", "-z", "--", directory],
        cwd=root, check=True, capture_output=True,
    ).stdout.decode("utf8")
    paths = [root / p for p in out.split("\0") if p and fnmatch.fnmatch(Path(p).name, pattern)]
    return sorted(p for p in paths if p.is_file())

RGBA = tuple[float, float, float, float]


# ---------------------------------------------------------------- 色彩運算


def parse_hex(text: str) -> RGBA:
    h = text.strip().lstrip("#")
    if len(h) == 3:
        h = "".join(c * 2 for c in h)
    if len(h) == 6:
        h += "ff"
    if len(h) != 8:
        raise ValueError(f"不是 hex 色碼：{text}")
    r, g, b, a = (int(h[i : i + 2], 16) for i in (0, 2, 4, 6))
    return (r, g, b, a / 255)


def over(fg: RGBA, bg: RGBA) -> RGBA:
    a = fg[3]
    if a >= 1:
        return fg
    out = tuple(fg[i] * a + bg[i] * (1 - a) for i in range(3))
    return (*out, 1.0)


def mix(a: RGBA, pa: float, b: RGBA) -> RGBA:
    """srgb 空間的 color-mix(a pa%, b)。"""
    return tuple(a[i] * pa + b[i] * (1 - pa) for i in range(4))  # type: ignore[return-value]


def _lin(c: float) -> float:
    c /= 255
    return c / 12.92 if c <= 0.04045 else ((c + 0.055) / 1.055) ** 2.4


def luminance(c: RGBA) -> float:
    return 0.2126 * _lin(c[0]) + 0.7152 * _lin(c[1]) + 0.0722 * _lin(c[2])


def ratio(fg: RGBA, bg: RGBA) -> float:
    la, lb = luminance(fg), luminance(bg)
    hi, lo = max(la, lb), min(la, lb)
    return (hi + 0.05) / (lo + 0.05)


def hexs(c: RGBA) -> str:
    return "#%02x%02x%02x" % tuple(round(x) for x in c[:3])


# ---------------------------------------------------------------- CSS


def _decls(body: str) -> dict[str, str]:
    out: dict[str, str] = {}
    i, n = 0, len(body)
    while i < n:
        while i < n and body[i].isspace():
            i += 1
        if i >= n:
            break
        m = re.compile(r"(--[\w-]+)\s*:").match(body, i)
        if not m:
            # 跳過註解與其他宣告
            if body.startswith("/*", i):
                j = body.find("*/", i)
                i = n if j < 0 else j + 2
                continue
            j = i
            depth = 0
            while j < n:
                ch = body[j]
                if ch == "(":
                    depth += 1
                elif ch == ")":
                    depth -= 1
                elif ch == ";" and depth == 0:
                    break
                j += 1
            i = j + 1
            continue
        name = m.group(1)
        j = m.end()
        depth = 0
        start = j
        while j < n:
            if body.startswith("/*", j):
                k = body.find("*/", j)
                j = n if k < 0 else k + 2
                continue
            ch = body[j]
            if ch == "(":
                depth += 1
            elif ch == ")":
                depth -= 1
            elif ch == ";" and depth == 0:
                break
            j += 1
        raw = re.sub(r"/\*.*?\*/", "", body[start:j], flags=re.S)
        out[name] = " ".join(raw.split())
        i = j + 1
    return out


def _blocks(source: str, selector: str) -> list[str]:
    """回傳頂層 `selector {…}` 區塊的內容。"""
    out = []
    for m in re.finditer(r"(?m)^" + re.escape(selector) + r"\s*\{", source):
        depth, j = 1, m.end()
        while depth and j < len(source):
            if source[j] == "{":
                depth += 1
            elif source[j] == "}":
                depth -= 1
            j += 1
        out.append(source[m.end() : j - 1])
    return out


class Css:
    def __init__(self, root: Path, files: list[Path] | None = None):
        base = root / "website/styles"
        files = files or [
            base / "foundation/00-color-config.css",
            base / "foundation/01-tokens-theme.css",
            base / "components/ui-kit.css",
            base / "globals.css",
        ]
        self.light: dict[str, str] = {}
        self.dark: dict[str, str] = {}
        for f in files:
            src = f.read_text(encoding="utf8")
            for blk in _blocks(src, ":root") + _blocks(src, "@theme"):
                self.light.update(_decls(blk))
            for blk in _blocks(src, '[data-theme="dark"]'):
                self.dark.update(_decls(blk))

    def pool(self, theme: str) -> dict[str, str]:
        return {**self.light, **self.dark} if theme == "dark" else self.light

    def text(self, value: str, theme: str, seen: tuple[str, ...] = ()) -> str:
        pool = self.pool(theme)

        def sub(m: re.Match[str]) -> str:
            name, fb = m.group(1), m.group(2)
            if name in pool and name not in seen:
                return self.text(pool[name], theme, seen + (name,))
            if fb is not None:
                return self.text(fb.strip(), theme, seen)
            raise KeyError(f"CSS 變數 {name} 解不出來（{theme}）")

        pat = re.compile(r"var\(\s*(--[\w-]+)\s*(?:,((?:[^()]|\([^()]*\))*))?\)")
        prev = None
        while prev != value:
            prev = value
            value = pat.sub(sub, value)
        return value

    def color(self, value: str, theme: str, bg: RGBA | None = None) -> RGBA:
        t = self.text(value, theme).strip()
        return parse_color_text(t, bg)

    def token(self, name: str, theme: str = "light", bg: RGBA | None = None) -> RGBA:
        pool = self.pool(theme)
        if name not in pool:
            raise KeyError(f"沒有 CSS 變數 {name}")
        return self.color(pool[name], theme, bg)


def _split_top(s: str) -> list[str]:
    parts, depth, cur = [], 0, ""
    for ch in s:
        if ch == "(":
            depth += 1
        elif ch == ")":
            depth -= 1
        if ch == "," and depth == 0:
            parts.append(cur.strip())
            cur = ""
        else:
            cur += ch
    parts.append(cur.strip())
    return parts


def parse_color_text(t: str, bg: RGBA | None = None) -> RGBA:
    t = t.strip()
    if t.startswith("#"):
        c = parse_hex(t)
        return over(c, bg) if bg is not None and c[3] < 1 else c
    m = re.fullmatch(r"rgba?\((.*)\)", t, re.S)
    if m:
        inner = m.group(1).replace(",", " ")
        alpha = 1.0
        if "/" in inner:
            inner, a = inner.split("/")
            alpha = _num(a)
        nums = inner.split()
        if len(nums) == 4:
            alpha = _num(nums[3])
            nums = nums[:3]
        c = (_num(nums[0]), _num(nums[1]), _num(nums[2]), alpha)
        return over(c, bg) if bg is not None and alpha < 1 else c
    m = re.fullmatch(r"color-mix\((.*)\)", t, re.S)
    if m:
        parts = _split_top(m.group(1))
        if parts[0].replace(" ", "") != "insrgb":
            raise ValueError(f"只支援 srgb 的 color-mix：{t}")

        def side(p: str) -> tuple[RGBA, float | None]:
            pm = re.fullmatch(r"(.*?)\s+(\d+(?:\.\d+)?)%", p.strip(), re.S)
            if pm:
                return parse_color_text(pm.group(1), bg), float(pm.group(2)) / 100
            return parse_color_text(p, bg), None

        (a, pa), (b, pb) = side(parts[1]), side(parts[2])
        if pa is None and pb is None:
            pa = 0.5
        if pa is None:
            pa = 1 - pb  # type: ignore[operator]
        c = mix(a, pa, b)
        return over(c, bg) if bg is not None and c[3] < 1 else c
    if t == "transparent":
        return (0, 0, 0, 0)
    raise ValueError(f"解不出顏色：{t}")


def _num(s: str) -> float:
    s = s.strip()
    if s.endswith("%"):
        return float(s[:-1]) / 100
    return float(s)


# ---------------------------------------------------------------- Dart


class Dart:
    DECL = re.compile(r"static\s+const\s+Color\s+(\w+)\s*=\s*([^;]+);")

    def __init__(self, path: Path):
        src = re.sub(r"//[^\n]*", "", path.read_text(encoding="utf8"))
        self.raw = {m.group(1): m.group(2).strip() for m in self.DECL.finditer(src)}

    def token(self, name: str, bg: RGBA | None = None, seen: tuple[str, ...] = ()) -> RGBA:
        if name not in self.raw:
            raise KeyError(f"沒有 AppColors.{name}")
        v = self.raw[name]
        m = re.fullmatch(r"Color\(0x([0-9A-Fa-f]{8})\)", v)
        if m:
            h = m.group(1)
            c = parse_hex("#" + h[2:] + h[:2])
            return over(c, bg) if bg is not None and c[3] < 1 else c
        if re.fullmatch(r"\w+", v) and v not in seen:
            return self.token(v, bg, seen + (name,))
        raise ValueError(f"AppColors.{name} 解不出來：{v}")


# ---------------------------------------------------------------- 比對表


class Report:
    def __init__(self) -> None:
        self.rows: list[tuple[str, str, str, str, float, float, str]] = []
        self.fail = 0
        self.errors: list[str] = []

    def add(self, group: str, label: str, fg: RGBA, bg: RGBA, kind: str, enforce: bool = True) -> None:
        need = 4.5 if kind == "text" else 3.0
        r = ratio(over(fg, bg), bg)
        if not enforce:
            verdict = "info"
        elif r + 1e-9 >= need:
            verdict = "ok"
        else:
            verdict = "FAIL"
            self.fail += 1
        self.rows.append((group, label, hexs(over(fg, bg)), hexs(bg), r, need, verdict))

    def safe(self, group: str, label: str, fn, kind: str, enforce: bool = True) -> None:
        try:
            fg, bg = fn()
        except Exception as e:  # noqa: BLE001 — 解不出來就是檢查失敗
            self.errors.append(f"{group} / {label}: {e}")
            return
        self.add(group, label, fg, bg, kind, enforce)


WEB_SURFACES = ["--theme-surface-page", "--theme-surface-card", "--theme-surface-muted", "--theme-surface-panel", "--theme-surface-elevated"]
WEB_TEXT = ["--theme-text-heading", "--theme-text-body", "--theme-text-muted"]


def check_web(css: Css, rep: Report) -> None:
    g = "web 亮色"

    def pair(fg_name: str, bg_name: str, theme: str = "light"):
        def fn():
            bg = css.token(bg_name, theme)
            bg = over(bg, css.token("--theme-surface-page", theme)) if bg[3] < 1 else bg
            return css.token(fg_name, theme, bg), bg

        return fn

    def lit(fg_name: str, bg_literal: str, theme: str = "light"):
        def fn():
            bg = parse_color_text(bg_literal)
            return css.token(fg_name, theme, bg), bg

        return fn

    # 按鈕：白字 × action.primary / hover / pressed
    for bg_name in ("--theme-action-primary", "--theme-action-primary-hover"):
        rep.safe(g, f"按鈕文字(white) × {bg_name}", lambda b=bg_name: (css.token("--color-neutral-white"), css.token(b)), "text")
        rep.safe(g, f"按鈕文字(cream, text-inverse) × {bg_name}", lambda b=bg_name: (css.token("--theme-text-inverse"), css.token(b)), "text")
    rep.safe(g, "hover 與 primary 可分辨", lambda: (css.token("--theme-action-primary-hover"), css.token("--theme-action-primary")), "ui", enforce=False)
    # hkui 按鈕
    rep.safe(g, "hkui-primary-foreground × hkui-primary", lambda: (css.token("--hkui-primary-foreground"), css.token("--hkui-primary")), "text")
    rep.safe(g, "hkui-primary-foreground × hkui-primary-700 (hover)", lambda: (css.token("--hkui-primary-foreground"), css.token("--hkui-primary-700")), "text")
    rep.safe(g, "hkui-secondary-foreground × hkui-secondary", lambda: (css.token("--hkui-secondary-foreground"), css.token("--hkui-secondary")), "text")
    rep.safe(g, "hkui-accent-foreground × hkui-accent", lambda: (css.token("--hkui-accent-foreground"), css.token("--hkui-accent")), "text")
    # 連結與選取文字 × 各 surface
    for s in WEB_SURFACES:
        # surface-panel 上的強調文字一律用 text-accent-on-tint（primaryStrong 在 panel 上只有 4.48:1）
        on_panel = s == "--theme-surface-panel"
        link = "--theme-text-accent-on-tint" if on_panel else "--theme-action-primary"
        accent = "--theme-text-accent-on-tint" if on_panel else "--theme-text-accent"
        secondary = "--theme-text-accent-on-tint" if on_panel else "--theme-action-secondary"
        rep.safe(g, f"連結 {link} × {s}", pair(link, s), "text")
        rep.safe(g, f"選取文字 {accent} × {s}", pair(accent, s), "text")
        rep.safe(g, f"hover 連結 action.primary-hover × {s}", pair("--theme-action-primary-hover", s), "text")
        rep.safe(g, f"action.secondary 文字 {secondary} × {s}", pair(secondary, s), "text")
        rep.safe(g, f"text-accent-on-tint × {s}", pair("--theme-text-accent-on-tint", s), "text")
        for t in WEB_TEXT:
            rep.safe(g, f"{t} × {s}", pair(t, s), "text")
        rep.safe(g, f"focus / 邊框元件 action.primary × {s}", pair("--theme-action-primary", s), "ui")
    # 選取態：強調文字疊在 primary 的半透明填色上（subtle 10%、soft 18%）
    for tint in ("--theme-primary-subtle", "--theme-primary-soft"):
        for s in ("--theme-surface-page", "--theme-surface-card"):
            rep.safe(g, f"選取文字 text-accent-on-tint × {tint} over {s}", lambda tint=tint, s=s: (css.token("--theme-text-accent-on-tint"), over(css.token(tint), css.token(s))), "text")
    # 品牌色字 × 淡階與輔色
    rep.safe(g, "brand-ink × brand-secondary", lambda: (css.token("--color-brand-ink"), css.token("--color-brand-secondary")), "text")
    rep.safe(g, "brand-ink × brand-secondary-soft", lambda: (css.token("--color-brand-ink"), css.token("--color-brand-secondary-soft")), "text")
    rep.safe(g, "brand-ink × brand-primary-soft", lambda: (css.token("--color-brand-ink"), css.token("--color-brand-primary-soft")), "text")
    rep.safe(g, "brand-primary-strong × brand-primary-soft（標籤）", lambda: (css.token("--color-brand-primary-strong"), css.token("--color-brand-primary-soft")), "text")
    rep.safe(g, "brand-primary-strong × brand-secondary-soft（標籤）", lambda: (css.token("--color-brand-primary-strong"), css.token("--color-brand-secondary-soft")), "text")
    rep.safe(g, "brand-primary-deep × brand-primary-soft", lambda: (css.token("--color-brand-primary-deep"), css.token("--color-brand-primary-soft")), "text")
    rep.safe(g, "brand-primary-deep × white（pressed 文字）", lambda: (css.token("--color-brand-primary-deep"), css.token("--color-neutral-white")), "text")
    rep.safe(g, "白字 × brand-primary-deep", lambda: (css.token("--color-neutral-white"), css.token("--color-brand-primary-deep")), "text")
    rep.safe(g, "brand-ink-on-dark × brand-primary-strong", lambda: (css.token("--color-brand-ink-on-dark"), css.token("--color-brand-primary-strong")), "text")
    rep.safe(g, "brand-ink-on-dark × brand-primary-deep", lambda: (css.token("--color-brand-ink-on-dark"), css.token("--color-brand-primary-deep")), "text")
    rep.safe(g, "brand-ink-on-dark × brand-ink", lambda: (css.token("--color-brand-ink-on-dark"), css.token("--color-brand-ink")), "text")
    rep.safe(g, "大字 brand-primary × 頁面 surface", pair("--color-brand-primary", "--theme-surface-page"), "large")
    rep.safe(g, "大字 brand-primary × 卡片 surface", pair("--color-brand-primary", "--theme-surface-card"), "large")
    rep.safe(g, "大字 白 × brand-primary（>=24px）", lambda: (css.token("--color-neutral-white"), css.token("--color-brand-primary")), "large")
    rep.safe(g, "logo brand-primary × 頁面 surface（圖形）", pair("--brand-logo-on-light", "--theme-surface-page"), "ui")
    rep.safe(g, "logo 檯面 brand-ink × 頁面 surface（圖形）", pair("--brand-logo-counter", "--theme-surface-page"), "ui")
    rep.safe(g, "logo on-dark × brand-primary-strong 實心底（圖形）", lambda: (css.token("--brand-logo-on-dark"), css.token("--color-brand-primary-strong")), "ui")
    # 聊天泡泡、側欄選取、tag 等實際用到的組合
    rep.safe(g, "聊天自己的泡泡 白字 × bubble-mine-bg", lambda: (css.token("--color-neutral-white"), css.token("--page-chat-bubble-mine-bg")), "text")
    rep.safe(g, "聊天標題列 header-fg × header-bg", lambda: (css.token("--page-chat-thread-header-fg"), css.token("--page-chat-thread-header-bg")), "text")
    rep.safe(g, "聊天卡片價格 × 白底", lambda: (css.token("--page-chat-card-price"), css.token("--color-neutral-white")), "text")
    rep.safe(g, "聊天卡片狀態 status-fg × status-bg(over 白)", lambda: (css.token("--page-chat-card-status-fg"), over(css.token("--page-chat-card-status-bg"), css.token("--color-neutral-white"))), "text")
    rep.safe(g, "聊天 chip-fg × chip-bg(over 白)", lambda: (css.token("--page-chat-empty-chip-fg"), over(css.token("--page-chat-empty-chip-bg"), css.token("--color-neutral-white"))), "text")
    rep.safe(g, "導覽 seller-cta-fg × 頁面 surface", pair("--page-nav-seller-cta-fg", "--theme-surface-page"), "text")
    rep.safe(g, "首頁 accent-700 文字 × 白(70%) 近似白", lambda: (css.token("--color-hk-home-accent-700"), css.token("--color-neutral-white")), "text")
    rep.safe(g, "首頁 accent-500 底 × 白字", lambda: (css.token("--color-neutral-white"), css.token("--color-hk-home-accent-500")), "text")
    rep.safe(g, "首頁 stage-cta 墨字 × brand-secondary 底", lambda: (css.token("--color-brand-ink"), css.token("--color-brand-secondary")), "text")
    rep.safe(g, "首頁 primary-deep 文字 × paper", lambda: (css.token("--color-hk-home-primary-deep"), css.token("--color-hk-home-paper")), "text")
    # 季節卡：ink 文字 × 12% 季節色混 light
    for season in ("spring", "summer", "autumn", "winter"):
        def fn(season=season):
            base = css.token(f"--page-home-season-{season}")
            light = css.token("--page-home-season-light")
            wash = mix(base, 0.12, light)
            return css.token("--theme-text-heading"), wash

        rep.safe(g, f"首頁季節 {season}：標題文字 × 12% wash", fn, "text")
        rep.safe(g, f"首頁季節 {season}：body 文字 × 12% wash", lambda season=season: (css.token("--theme-text-body"), mix(css.token(f"--page-home-season-{season}"), 0.12, css.token("--page-home-season-light"))), "text")
    rep.safe(g, "首頁季節 focus 外框 season-ink × cream", lambda: (css.token("--page-home-season-ink"), css.token("--page-home-season-light")), "ui")

    # 暗色
    g = "web 暗色"
    dark_surfaces = ["--theme-surface-page", "--theme-surface-card", "--theme-surface-muted", "--theme-surface-panel", "--theme-surface-elevated"]
    rep.safe(g, "按鈕文字(text-inverse) × action.primary", lambda: (css.token("--theme-text-inverse", "dark"), css.token("--theme-action-primary", "dark")), "text")
    rep.safe(g, "按鈕文字(text-inverse) × action.primary-hover", lambda: (css.token("--theme-text-inverse", "dark"), css.token("--theme-action-primary-hover", "dark")), "text")
    for s in dark_surfaces:
        rep.safe(g, f"連結 action.primary × {s}", pair("--theme-action-primary", s, "dark"), "text")
        rep.safe(g, f"hover 連結 action.primary-hover × {s}", pair("--theme-action-primary-hover", s, "dark"), "text")
        rep.safe(g, f"選取文字 text-accent × {s}", pair("--theme-text-accent", s, "dark"), "text")
        rep.safe(g, f"text-accent-on-tint × {s}", pair("--theme-text-accent-on-tint", s, "dark"), "text")
        rep.safe(g, f"action.secondary 文字 × {s}", pair("--theme-action-secondary", s, "dark"), "text")
        for t in WEB_TEXT:
            rep.safe(g, f"{t} × {s}", pair(t, s, "dark"), "text")
        # 品牌色在暗色底上：primary 只作大字或圖形（>= 3:1）；primaryStrong／primary 當一般文字不合格，只列出不判定。
        rep.safe(g, f"brand-primary 大字／圖形 × {s}", lambda s=s: (css.token("--color-brand-primary"), css.token(s, "dark")), "large")
        rep.safe(g, f"brand-primary 當一般文字（不得使用）× {s}", lambda s=s: (css.token("--color-brand-primary"), css.token(s, "dark")), "text", enforce=False)
        rep.safe(g, f"brand-primary-strong 當一般文字（不得使用）× {s}", lambda s=s: (css.token("--color-brand-primary-strong"), css.token(s, "dark")), "text", enforce=False)
        rep.safe(g, f"brand-ink-on-dark × {s}", lambda s=s: (css.token("--color-brand-ink-on-dark"), css.token(s, "dark")), "text")
    for tint in ("--theme-primary-subtle", "--theme-primary-soft"):
        for s in ("--theme-surface-page", "--theme-surface-card"):
            rep.safe(g, f"text-accent-on-tint × {tint} over {s}", lambda tint=tint, s=s: (css.token("--theme-text-accent-on-tint", "dark"), over(css.token(tint, "dark"), css.token(s, "dark"))), "text")
    for fg in ("--color-brand-ink-on-dark",):
        for bg in ("--color-brand-primary-strong", "--color-brand-primary-deep"):
            rep.safe(g, f"{fg} × {bg}（暗色底上的主按鈕）", lambda fg=fg, bg=bg: (css.token(fg, "dark"), css.token(bg, "dark")), "text")
    rep.safe(g, "站內連結 a:not(button) × 頁面 surface", lambda: (parse_color_text(css.text("var(--color-brand-secondary)", "dark")), css.token("--theme-surface-page", "dark")), "text")
    rep.safe(g, "logo on-dark × 頁面 surface（圖形）", pair("--brand-logo-on-dark", "--theme-surface-page", "dark"), "ui")
    rep.safe(g, "logo 檯面 × 頁面 surface（圖形）", pair("--brand-logo-counter", "--theme-surface-page", "dark"), "ui")
    rep.safe(g, "logo brand-primary × 頁面 surface（圖形）", pair("--color-brand-primary", "--theme-surface-page", "dark"), "ui")
    rep.safe(g, "lottie-primary × 頁面 surface（圖形）", pair("--lottie-primary", "--theme-surface-page", "dark"), "ui")
    rep.safe(g, "lottie-secondary × 頁面 surface（圖形）", pair("--lottie-secondary", "--theme-surface-page", "dark"), "ui")
    rep.safe(g, "聊天自己的泡泡 白字 × bubble-mine-bg", lambda: (css.token("--color-neutral-white", "dark"), css.token("--page-chat-bubble-mine-bg", "dark")), "text")
    rep.safe(g, "聊天卡片價格 × 卡片底", lambda: (css.token("--page-chat-card-price", "dark"), css.token("--theme-surface-elevated", "dark")), "text")
    rep.safe(g, "地圖 selected × 頁面 surface（圖形）", pair("--page-map-selected", "--theme-surface-page", "dark"), "ui")


STATUS_PAIRS = [
    ("success", "--color-status-success", "--hkui-success-foreground"),
    ("warning", "--color-status-warning", "--hkui-warning-foreground"),
    ("danger", "--color-status-danger", "--hkui-danger-foreground"),
    ("info", "--color-status-info", "--hkui-info-foreground"),
    ("info-strong", "--color-status-info-strong", "--hkui-info-foreground"),
]


def check_status_web(css: Css, rep: Report) -> None:
    g = "狀態色（值未變，只列出不判定）web"
    for name, bg, fg in STATUS_PAIRS:
        rep.safe(g, f"{name}：前景 × 底", lambda bg=bg, fg=fg: (css.token(fg), css.token(bg)), "text", enforce=False)


def check_dart(app: str, d: Dart, rep: Report) -> None:
    g = f"{app}-app"

    def t(name: str, bg: RGBA | None = None) -> RGBA:
        return d.token(name, bg)

    surfaces = ["surfacePage", "surfaceCream", "surfaceMuted", "surfacePanel", "neutralWhite"]
    on_primary = "textInverse"  # 按鈕上的白字（neutralCream）
    for bg in ("actionPrimary", "primaryDark", "brandPrimaryStrong", "brandPrimaryDeep"):
        rep.safe(g, f"按鈕文字({on_primary}) × {bg}", lambda bg=bg: (t(on_primary), t(bg)), "text")
        rep.safe(g, f"白字(neutralWhite) × {bg}", lambda bg=bg: (t("neutralWhite"), t(bg)), "text")
    rep.safe(g, "commerceCheckoutForeground × commerceCheckoutAction" if "commerceCheckoutAction" in d.raw else "brandInk × brandSecondary",
             (lambda: (t("commerceCheckoutForeground"), t("commerceCheckoutAction"))) if "commerceCheckoutAction" in d.raw else (lambda: (t("brandInk"), t("brandSecondary"))), "text")
    if "commerceCartAction" in d.raw:
        rep.safe(g, "commerceCartForeground × commerceCartAction", lambda: (t("commerceCartForeground"), t("commerceCartAction")), "text")
    for s in surfaces:
        # surfacePanel 上的強調文字一律用 textAccentOnTint（primary 在 panel 上只有 4.48:1）
        fg_link = "textAccentOnTint" if s == "surfacePanel" else "primary"
        rep.safe(g, f"連結／互動文字 {fg_link} × {s}", lambda s=s, f=fg_link: (t(f), t(s)), "text")
        rep.safe(g, f"secondary 文字 {fg_link} × {s}", lambda s=s, f=fg_link: (t("secondary" if f == "primary" else f), t(s)), "text")
        rep.safe(g, f"textAccentOnTint × {s}", lambda s=s: (t("textAccentOnTint"), t(s)), "text")
        rep.safe(g, f"focus／邊框元件 primary × {s}", lambda s=s: (t("primary"), t(s)), "ui")
        rep.safe(g, f"textPrimary × {s}", lambda s=s: (t("textPrimary"), t(s)), "text")
        rep.safe(g, f"textMuted × {s}", lambda s=s: (t("textMuted"), t(s)), "text")
        rep.safe(g, f"textEmphasis × {s}", lambda s=s: (t("textEmphasis"), t(s)), "text")
        rep.safe(g, f"brandInk × {s}", lambda s=s: (t("brandInk"), t(s)), "text")
    # 淡底上的互動文字（primarySoft 是 primary 的 alpha，疊在 surface 上）
    for soft in ("primarySoft", "primaryHover", "primaryActive"):
        for s in ("surfaceCream", "surfacePage"):
            def fn(soft=soft, s=s):
                bg = t(s)
                bgc = over(t(soft), bg)
                return t("textAccentOnTint"), bgc

            rep.safe(g, f"互動文字 textAccentOnTint × {soft} over {s}", fn, "text")
    rep.safe(g, "brandInk × brandSecondary", lambda: (t("brandInk"), t("brandSecondary")), "text")
    rep.safe(g, "brandInk × brandSecondarySoft", lambda: (t("brandInk"), t("brandSecondarySoft")), "text")
    rep.safe(g, "brandInk × brandPrimarySoft", lambda: (t("brandInk"), t("brandPrimarySoft")), "text")
    rep.safe(g, "brandPrimaryStrong × brandPrimarySoft", lambda: (t("brandPrimaryStrong"), t("brandPrimarySoft")), "text")
    rep.safe(g, "brandPrimaryDeep × brandPrimarySoft", lambda: (t("brandPrimaryDeep"), t("brandPrimarySoft")), "text")
    rep.safe(g, "brandInkOnDark × brandPrimaryStrong", lambda: (t("brandInkOnDark"), t("brandPrimaryStrong")), "text")
    rep.safe(g, "brandInkOnDark × brandPrimaryDeep", lambda: (t("brandInkOnDark"), t("brandPrimaryDeep")), "text")
    rep.safe(g, "brandInkOnDark × brandInk", lambda: (t("brandInkOnDark"), t("brandInk")), "text")
    rep.safe(g, "大字 brandPrimary × surfaceCream", lambda: (t("brandPrimary"), t("surfaceCream")), "large")
    rep.safe(g, "大字 白 × brandPrimary（>=24px）", lambda: (t("neutralWhite"), t("brandPrimary")), "large")
    rep.safe(g, "logo brandPrimary × surfaceCream（圖形）", lambda: (t("brandPrimary"), t("surfaceCream")), "ui")
    rep.safe(g, "logo 檯面 brandInk × surfaceCream（圖形）", lambda: (t("brandInk"), t("surfaceCream")), "ui")
    rep.safe(g, "logo 檯面 brandInkOnDark × neutralInk（深色 splash，圖形）", lambda: (t("brandInkOnDark"), t("neutralInk")), "ui")
    rep.safe(g, "logo brandPrimary × neutralInk（深色 splash，圖形）", lambda: (t("brandPrimary"), t("neutralInk")), "ui")
    rep.safe(g, "warning 底上的 onWarning", lambda: (t("onWarning"), t("warning")), "text")
    rep.safe(g, "warningText × warningSoft over surfaceCream", lambda: (t("warningText"), over(t("warningSoft"), t("surfaceCream"))), "text")
    rep.safe(g, "邊框 borderInput × surfaceCream（輸入框 UI 元件）", lambda: (t("borderInput"), t("surfaceCream")), "ui", enforce=False)
    g2 = f"狀態色（值未變，只列出不判定）{app}"
    for name, fg, bg in (("success", "neutralWhite", "success"), ("danger", "neutralWhite", "danger"), ("info", "neutralWhite", "info"), ("warning", "onWarning", "warning")):
        rep.safe(g2, f"{name}：前景 × 底", lambda fg=fg, bg=bg: (t(fg), t(bg)), "text", enforce=False)


# ---------------------------------------------------------------- 首頁範圍（.hkh-page 重設語意 token）


def check_home_scope(root: Path, rep: Report) -> None:
    src = (root / "website/styles/features/home.css").read_text(encoding="utf8")
    base = Css(root)
    for theme, selector, group in (("light", ".hkh-page", "web 首頁亮色範圍"), ("dark", '[data-theme="dark"] .hkh-page', "web 首頁暗色範圍")):
        css = Css(root)
        pool = css.dark if theme == "dark" else css.light
        for blk in _blocks(src, selector):
            pool.update(_decls(blk))

        def pair(fg_name: str, bg_name: str, css=css, theme=theme):
            def fn():
                bg = css.token(bg_name, theme)
                bg = over(bg, css.token("--theme-surface-page", theme)) if bg[3] < 1 else bg
                return css.token(fg_name, theme, bg), bg

            return fn

        for sname in WEB_SURFACES:
            for r in ("--theme-action-primary", "--theme-text-accent", "--theme-text-accent-on-tint"):
                rep.safe(group, f"{r} × {sname}", pair(r, sname), "text")
            for r in WEB_TEXT:
                # 亮色首頁的文字是既有的暖棕半透明色，值不在 D11 範圍內：只列出不判定。
                rep.safe(group, f"{r} × {sname}" + ("（既有暖棕，值未改）" if theme == "light" else ""), pair(r, sname), "text", enforce=theme == "dark")
        for tint in ("--theme-primary-subtle",):
            for sname in ("--theme-surface-page", "--theme-surface-card"):
                rep.safe(group, f"text-accent-on-tint × {tint} over {sname}", lambda tint=tint, sname=sname, css=css, theme=theme: (css.token("--theme-text-accent-on-tint", theme), over(css.token(tint, theme), css.token(sname, theme))), "text")
    del base


# ---------------------------------------------------------------- admin 側欄（深底）


def _rule_props(source: str, selector: str) -> dict[str, str]:
    out: dict[str, str] = {}
    for m in re.finditer(r"(?m)^" + re.escape(selector) + r"\s*\{([^{}]*)\}", source):
        for decl in m.group(1).split(";"):
            if ":" in decl:
                k, v = decl.split(":", 1)
                out[k.strip()] = " ".join(v.split())
    return out


def check_admin_sidebar(root: Path, rep: Report) -> None:
    g = "admin 側欄"
    files = [root / "admin/src/app/globals.css", root / "admin/src/styles/AdminVisualRefresh.css"]
    css = Css(root, files)
    src = files[1].read_text(encoding="utf8")

    def color(value: str, bg: RGBA | None = None) -> RGBA:
        return css.color(value, "light", bg)

    side = _rule_props(src, ".sidebar")
    nav = _rule_props(src, ".nav-item")
    active = _rule_props(src, ".nav-item.active")
    title = _rule_props(src, ".sidebar-title")
    section = _rule_props(src, ".sidebar-section-title")
    role = _rule_props(src, ".user-role")
    footer = _rule_props(src, ".sidebar-footer")
    try:
        side_bg = color(side["background"])
        footer_bg = over(color(footer["background"]), side_bg)
    except (KeyError, ValueError) as e:
        rep.errors.append(f"{g}: 讀不到側欄底色：{e}")
        return
    rep.add(g, "側欄文字 .sidebar color × 側欄底", color(side["color"], side_bg), side_bg, "text")
    for label, props, bg in (
        ("品牌副標 .sidebar-title", title, side_bg),
        ("區段標題 .sidebar-section-title", section, side_bg),
        ("選單項 .nav-item", nav, side_bg),
        ("使用者角色 .user-role", role, footer_bg),
    ):
        try:
            rep.add(g, f"{label} × 底", color(props["color"], bg), bg, "text")
        except KeyError as e:
            rep.errors.append(f"{g} / {label}: 讀不到 {e}")
    try:
        act_bg = color(active["background"], side_bg)
        rep.add(g, "選取項 .nav-item.active 文字 × 填色", color(active["color"], act_bg), act_bg, "text")
        indicator = re.search(r"inset\s+3px\s+0\s+0\s+(var\(--[\w-]+\))", active.get("box-shadow", ""))
        if not indicator:
            rep.errors.append(f"{g}: 選取項沒有左側色條（box-shadow inset 3px）")
        else:
            rep.add(g, "選取項左側色條（橘）× 側欄底（非文字）", color(indicator.group(1)), side_bg, "ui")
    except KeyError as e:
        rep.errors.append(f"{g} / 選取項: 讀不到 {e}")
    rep.safe(g, "主按鈕 白字 × admin-primary-strong", lambda: (parse_color_text("#ffffff"), css.token("--admin-primary-strong")), "text")
    rep.safe(g, "brand.inkOnDark × brand.ink（logo 檯面，圖形）", lambda: (css.token("--color-ink-50"), css.token("--color-ink-900")), "ui")


# ---------------------------------------------------------------- 使用處掃描：淡底上的強調文字


TINT_BG = re.compile(r"primary-subtle|primary-soft|surface-panel|--theme-primary-border|color-brand-primary-soft")
ACCENT_TEXT = re.compile(r"(?<![-\w])color:\s*var\(--(?:theme-(?:text-accent|action-primary|action-secondary|action-primary-hover)|color-brand-primary-strong)(?![\w-])")
TW_TINT_BG = re.compile(r"(?<![\w-])(?:[a-z-]+:)*!?bg-hk-(?:primary-subtle|primary-soft|surface-panel|primary-border)\b")
TW_ACCENT_TEXT = re.compile(r"(?<![\w-])(?:[a-z-]+:)*!?text-hk-(?:accent|primary|secondary|primary-hover|primary-strong)(?![\w-])")
DART_TINT_BG = re.compile(r"(?:color|backgroundColor):\s*(?:[^,;]*\?\s*)?AppColors\.(?:primarySoft|primaryActive|primaryHover|surfacePanel|neutralPanel)\b")
DART_ACCENT_TEXT = re.compile(r"color:\s*(?:[^,;]*\?\s*)?(?:const\s+)?AppColors\.(?:primary|secondary|actionPrimary|brandPrimaryStrong)\b(?!\w)")


def _line_of(source: str, pos: int) -> int:
    return source.count("\n", 0, pos) + 1


def scan_usage(root: Path, rep: Report) -> list[str]:
    bad: list[str] = []
    web = root / "website"
    for path in tracked_files(root, "website"):
        if "node_modules" in path.parts or ".next" in path.parts or not path.is_file():
            continue
        rel = path.relative_to(root)
        if path.suffix == ".css":
            src = path.read_text(encoding="utf8")
            for m in re.finditer(r"([^{}]+)\{([^{}]*)\}", src):
                body = m.group(2)
                if not ACCENT_TEXT.search(body):
                    continue
                if any(TINT_BG.search(b) for b in re.findall(r"background(?:-color)?\s*:[^;]*", body)):
                    bad.append(f"{rel}:{_line_of(src, m.start(2))}: {m.group(1).strip().splitlines()[-1].strip()[:60]}")
        elif path.suffix in (".tsx", ".ts") and "tests" not in path.parts:
            src = path.read_text(encoding="utf8")
            for m in re.finditer(r"\"[^\"\n]*\"|'[^'\n]*'|`[^`]*`", src):
                lit = m.group(0)
                if TW_TINT_BG.search(lit) and TW_ACCENT_TEXT.search(lit):
                    bad.append(f"{rel}:{_line_of(src, m.start())}: {lit[:80]}")
    for app in ("buyer", "seller"):
        for path in tracked_files(root, f"{app}-app/lib", "*.dart"):
            if path.name == "app_colors.dart":
                continue
            lines = path.read_text(encoding="utf8").split("\n")
            for i, line in enumerate(lines):
                if not DART_TINT_BG.search(line) or "border" in line.lower():
                    continue
                depth = 0
                for j in range(i + 1, min(len(lines), i + 24)):
                    nxt = lines[j]
                    depth += nxt.count("(") - nxt.count(")")
                    if depth < -1:
                        break
                    if DART_ACCENT_TEXT.search(nxt):
                        back = " ".join(lines[max(i + 1, j - 5) : j + 1])
                        last_text = max(back.rfind("TextStyle("), back.rfind("copyWith("))
                        last_other = max(back.rfind("BoxDecoration("), back.rfind("Border"), back.rfind("Icon("), back.rfind("Indicator("), back.rfind("ColorFilter"))
                        if last_text > last_other:
                            bad.append(f"{path.relative_to(root)}:{j + 1}: {nxt.strip()[:70]}")
    for b in bad:
        rep.errors.append(f"淡底上的強調文字沒有用 on-tint token：{b}")
    return bad


# ---------------------------------------------------------------- 使用處掃描 2：輔色當文字、半透明底、狀態 badge、暗色圖形


TW_ALPHA_BG = re.compile(r"(?<![\w-])((?:[a-z0-9\[\]=-]+:)*)!?bg-hk-(primary|brand-primary|primary-strong|primary-hover|secondary|brand-secondary)/(\d+)(?![\w-])")
TW_TEXT_FG = re.compile(r"(?<![\w-])((?:[a-z0-9\[\]=-]+:)*)!?text-hk-(accent|primary|primary-strong|primary-hover|secondary|brand-secondary|brand-primary)(?![\w-])")
TW_TOKEN = {
    "primary": "--theme-action-primary",
    "brand-primary": "--color-brand-primary",
    "primary-strong": "--color-brand-primary-strong",
    "primary-hover": "--theme-action-primary-hover",
    "secondary": "--theme-action-secondary",
    "brand-secondary": "--color-brand-secondary",
    "accent": "--theme-text-accent",
}
CSS_ALPHA_BG = re.compile(r"rgba?\(\s*var\(\s*(--[\w-]+?)-rgb\s*\)\s*/\s*([\d.]+%?)\s*\)")
CSS_FG = re.compile(r"(?<![-\w])(?:color|fill|stroke):\s*(var\([^;{}]*\)|#[0-9a-fA-F]{3,8})\s*(?:!important)?\s*;")
BRAND_FG = re.compile(r"brand-primary(?!-deep)|theme-text-accent(?!-on-tint)|theme-action-|hkui-primary(?!-foreground)")
DARK_SEL = re.compile(r"\.dark\b|\[data-theme=\"dark\"\]|prefers-color-scheme:\s*dark")
DART_TEXT_CTX = ("TextStyle(", "copyWith(")
DART_FILL_CTX = ("BoxDecoration(", "Border", "Icon(", "Indicator(", "ColorFilter", "Material(", "Container(", "DecoratedBox(", "Chip(")
DART_SECONDARY = r"AppColors\.(?:accentFill|accent|brandSecondary)\b"
DART_SECONDARY_TEXT = re.compile(r"color:\s*(?:[^,;]*\?\s*)?(?:const\s+)?" + DART_SECONDARY)
DART_SECONDARY_BG = re.compile(r"(?:color|backgroundColor):\s*(?:[^,;]*\?\s*)?(?:const\s+)?" + DART_SECONDARY)
DART_LIGHT_FG = re.compile(r"color:\s*(?:const\s+)?AppColors\.(?:textInverse|bgCard|neutralWhite|neutralCream|white)\b")
DART_ALPHA_PRIMARY_BG = re.compile(r"(?:color|backgroundColor):\s*(?:const\s+)?AppColors\.(brandPrimary|brandPrimaryStrong|primary)\.(?:withOpacity\(\s*([\d.]+)\s*\)|withValues\(\s*alpha:\s*([\d.]+)\s*\))")
DART_PRIMARY_FG = re.compile(r"color:\s*(?:const\s+)?AppColors\.(primary|brandPrimaryStrong|actionPrimary)\b(?!\w)")


def _ratio_vs(fg: RGBA, bg: RGBA) -> float:
    return ratio(over(fg, bg), bg)


def scan_usage_extra(root: Path, css: Css, darts: dict[str, Dart], rep: Report) -> list[str]:
    bad: list[str] = []
    white = (255.0, 255.0, 255.0, 1.0)
    page = css.token("--theme-surface-page")
    card_dark = css.token("--theme-surface-card", "dark")
    elev_dark = css.token("--theme-surface-elevated", "dark")

    def tw_color(key: str) -> RGBA:
        return css.token(TW_TOKEN[key])

    web = root / "website"
    for path in tracked_files(root, "website"):
        if "node_modules" in path.parts or ".next" in path.parts or not path.is_file():
            continue
        rel = path.relative_to(root)
        if path.suffix == ".css":
            src = path.read_text(encoding="utf8")
            for m in re.finditer(r"([^{}]+)\{([^{}]*)\}", src):
                sel, body = m.group(1).strip(), m.group(2)
                line = _line_of(src, m.start(2))
                dark = bool(DARK_SEL.search(sel))
                fg_m = CSS_FG.search(body)
                if not fg_m:
                    continue
                try:
                    fg = css.color(fg_m.group(1), "dark" if dark else "light")
                except Exception:  # noqa: BLE001 — 解不出來的 fg 不是本掃描的對象
                    continue
                need = 3.0 if re.search(r"icon|glyph|dot|badge-ring", sel) else 4.5
                label = f"{rel}:{line}: {sel.splitlines()[-1].strip()[:50]}"
                if dark:
                    # (d) 暗色底上的圖形與文字：fg 對暗色卡片／elevated 要過門檻
                    for surf in (card_dark, elev_dark):
                        r = _ratio_vs(fg, surf)
                        if r + 1e-9 < need and re.search(r"brand-primary-strong|brand-primary-deep", fg_m.group(1)):
                            bad.append(f"(d) 暗色底上 {need:g}:1 不足（{r:.2f}）{label}")
                            break
                    continue
                # (a) 輔色當文字
                if abs(fg[0] - css.token("--color-brand-secondary")[0]) < 1 and abs(fg[1] - css.token("--color-brand-secondary")[1]) < 1:
                    bg_decl = re.findall(r"background(?:-color)?\s*:[^;]*", body)
                    if not any("ink" in b for b in bg_decl):
                        bad.append(f"(a) 輔色當文字色 {label}")
                # (b) 透明度寫法的底，用白與 page 合成
                for bg_decl in re.findall(r"background(?:-color)?\s*:[^;]*", body):
                    am = CSS_ALPHA_BG.search(bg_decl)
                    if not am:
                        continue
                    if not re.fullmatch(r"--color-brand-(?:primary|primary-strong|secondary)", am.group(1)) or not BRAND_FG.search(fg_m.group(1)):
                        continue
                    try:
                        base = css.token(am.group(1))
                    except KeyError:
                        continue
                    a = _num(am.group(2))
                    worst = min(_ratio_vs(fg, over((*base[:3], a), surf)) for surf in (white, page))
                    if worst + 1e-9 < need:
                        bad.append(f"(b) 半透明底上對比 {worst:.2f} < {need:g} {label}")
        elif path.suffix in (".tsx", ".ts") and "tests" not in path.parts and ".test." not in path.name:
            src = path.read_text(encoding="utf8")
            for m in re.finditer(r"\"[^\"\n]*\"|'[^'\n]*'|`[^`]*`", src):
                lit = m.group(0)
                if "bg-hk-" not in lit:
                    continue
                bgs = [b for b in TW_ALPHA_BG.finditer(lit) if "dark:" not in b.group(1)]
                fgs = [f for f in TW_TEXT_FG.finditer(lit) if "dark:" not in f.group(1)]
                for b in bgs:
                    base = tw_color(b.group(2))
                    a = int(b.group(3)) / 100
                    for f in fgs:
                        fg = tw_color(f.group(2))
                        worst = min(_ratio_vs(fg, over((*base[:3], a), surf)) for surf in (white, page))
                        if worst + 1e-9 < 4.5:
                            bad.append(f"(b) {rel}:{_line_of(src, m.start())}: {b.group(0)} × {f.group(0)} = {worst:.2f}")
                for f in fgs:
                    if f.group(2) == "brand-secondary":
                        bad.append(f"(a) 輔色當文字色 {rel}:{_line_of(src, m.start())}: {f.group(0)}")
    for app, d in darts.items():
        base_dir = root / f"{app}-app/lib"
        for path in tracked_files(root, f"{app}-app/lib", "*.dart"):
            if path.name == "app_colors.dart":
                continue
            lines = path.read_text(encoding="utf8").split("\n")
            for i, line in enumerate(lines):
                rel = f"{path.relative_to(root)}:{i + 1}"
                back = " ".join(lines[max(0, i - 5) : i + 1])
                last_text = max(back.rfind(k) for k in DART_TEXT_CTX)
                last_fill = max(back.rfind(k) for k in DART_FILL_CTX)
                # (a) 輔色當文字色／圖示色
                if DART_SECONDARY_TEXT.search(line) and last_text > last_fill:
                    bad.append(f"(a) 輔色當文字色 {rel}: {line.strip()[:70]}")
                # (a) 輔色底上放淺色字
                if DART_SECONDARY_BG.search(line) and last_fill >= last_text:
                    for j in range(i + 1, min(len(lines), i + 14)):
                        if DART_LIGHT_FG.search(lines[j]) and any(k in " ".join(lines[max(i, j - 4) : j + 1]) for k in DART_TEXT_CTX):
                            bad.append(f"(a) 輔色底上放淺色字 {path.relative_to(root)}:{j + 1}: {lines[j].strip()[:60]}")
                            break
                        if lines[j].count(")") > lines[j].count("(") + 2:
                            break
                # (b) Dart 半透明主色底上的主色文字
                am = DART_ALPHA_PRIMARY_BG.search(line)
                if am:
                    a = float(am.group(2) or am.group(3))
                    base = d.token(am.group(1))
                    surfs = [d.token("neutralWhite"), d.token("surfaceCream"), d.token("surfacePage")]
                    for j in range(i + 1, min(len(lines), i + 14)):
                        nxt = lines[j]
                        if DART_PRIMARY_FG.search(nxt) and any(k in " ".join(lines[max(i, j - 4) : j + 1]) for k in DART_TEXT_CTX):
                            fg = d.token(DART_PRIMARY_FG.search(nxt).group(1))  # type: ignore[union-attr]
                            worst = min(_ratio_vs(fg, over((*base[:3], a), sf)) for sf in surfs)
                            if worst + 1e-9 < 4.5:
                                bad.append(f"(b) 半透明底上對比 {worst:.2f} {path.relative_to(root)}:{j + 1}: {nxt.strip()[:60]}")
                            break
    # (c) 前景與底成對定義的狀態 badge 表
    table = root / "buyer-app/lib/features/order/order_color_tokens.dart"
    if table.exists():
        raw = {m.group(1): m.group(2) for m in re.finditer(r"static const (\w+) = AppColors\.(\w+);", table.read_text(encoding="utf8"))}
        d = darts["buyer"]
        text = table.read_text(encoding="utf8")
        fn = re.search(r"foregroundFor\(Color background\)\s*\{(.*?)\n  \}", text, re.S)
        backgrounds = {"attentionFill": "onAttention", "neutralSurface": "onAttention", "primary": "onPrimary", "neutral": "onPrimary", "progress": "onPrimary"}
        for bg_name, fg_name in backgrounds.items():
            if bg_name not in raw or fg_name not in raw:
                continue
            if fn is None:
                continue
            # foregroundFor 對 attentionFill、neutralSurface 回傳 onAttention，其餘 onPrimary
            expected = "onAttention" if re.search(rf"background == {bg_name}\b", fn.group(1)) else "onPrimary"
            r = _ratio_vs(d.token(raw[expected]), d.token(raw[bg_name]))
            if r + 1e-9 < 4.5:
                bad.append(f"(c) badge {expected}×{bg_name} 對比 {r:.2f}：order_color_tokens.dart")
    for b in bad:
        rep.errors.append(f"使用處掃描：{b}")
    return bad


# ---------------------------------------------------------------- 使用處掃描 3：淺色 token 當文字、無暗色覆寫、Dart 依解析值、admin、TSX dark:


ACCENT_FAMILY = re.compile(r"brand-(?:primary|secondary)|--hk-(?:primary|secondary)|admin-primary|--(?:theme|hkui)-(?:text-accent|action|primary|secondary|accent)")
ON_DARK_NAME = re.compile(r"foreground|-on-|inverse|contrast|ink-on-dark")
FIXED_TOKEN = re.compile(r"var\(\s*--color-brand-(?:primary|primary-strong|primary-deep|secondary)\s*[,)]")
DECORATIVE_SELECTORS = {".benefit-num"}  # 純裝飾的序號（base 就是 18% 透明度），不承載資訊
ON_DARK_FILES = {  # 深色漸層底上的淺色字：檔案 -> 底的 AppColors 名稱（腳本照實算對比）
    "seller-app/lib/features/coupon/presentation/widgets/preview_card.dart": ("brandPrimaryStrong", "primaryDark"),
}
# (4) 的既有缺口（舊綠色時代就沒有暗色覆寫，值只是跟著換成橘）：每個檔的筆數不得增加，新增的規則才會被擋
KNOWN_DARK_GAPS: dict[str, int] = {
    "website/styles/components/sheet.css": 3,
    "website/styles/components/ui-kit.css": 1,
    "website/styles/features/cart.css": 9,
    "website/styles/features/delegated-operations.css": 2,
    "website/styles/features/market.css": 2,
    "website/styles/features/messages.css": 1,
    "website/styles/features/seller-center.css": 2,
    "website/styles/legacy/prd-v2-commerce.css": 1,
}
PRINT_GAPS = False
ON_DARK_OK = {"#ffffff", "#fffdf7", "#f5f7fa"}  # 明確的「深底上的字」色：白、cream、inkOnDark
CSS_COLOR_PROP = re.compile(r"(?<![-\w])color:\s*(var\([^;{}]*\)|#[0-9a-fA-F]{3,8}|rgba?\([^;{}]*\))\s*(?:!important)?\s*;")
CSS_BG_DECL = re.compile(r"background(?:-color)?\s*:\s*([^;]*)")
DART_COLOR_CLAUSE = re.compile(r"(?<![\w])color:\s*([^,;]*)")
DART_APP_COLOR = re.compile(r"(?:const\s+)?AppColors\.(\w+)(\.with\w+\([^)]*\))?")
DART_FILL_LINE = re.compile(r"(?:color|backgroundColor):\s*(?:const\s+)?(?:AppColors\.(\w+)|Color\(0x([0-9A-Fa-f]{8})\))")
TW_CLASS = re.compile(r"(?<![\w-])((?:[a-z0-9\[\]=&>_-]+:)*)(!?)(bg|text)-hk-([a-z0-9-]+?)(?:/(\d+))?(?![\w-])")


def _dart_chain(d: "Dart", name: str) -> list[str]:
    out, seen = [name], {name}
    while name in d.raw and re.fullmatch(r"\w+", d.raw[name]) and d.raw[name] not in seen:
        name = d.raw[name]
        out.append(name)
        seen.add(name)
    return out


def _dart_brand_family(d: "Dart", name: str) -> bool:
    return any(re.match(r"brand(?!Ink)|primary|secondary|accent", n) for n in _dart_chain(d, name))


def _is_light(c: RGBA) -> bool:
    return ratio(c, (255.0, 255.0, 255.0, 1.0)) < 3.0


def _canonical_on_dark(c: RGBA) -> bool:
    return hexs(c) in ON_DARK_OK


def scan_usage_text(root: Path, css: Css, admin_css: Css, darts: dict[str, Dart], rep: Report) -> list[str]:
    """(1) 淺色 token 當文字；(2) Dart 依解析值；(3) admin；(4) 沒有暗色覆寫的規則在暗色 surface 上也算一次；TSX 的 dark: 群組。"""
    bad: list[str] = []
    white = (255.0, 255.0, 255.0, 1.0)
    dark_surfs = [css.token(n, "dark") for n in ("--theme-surface-page", "--theme-surface-card", "--theme-surface-elevated")]

    def css_files(base: Path):
        for path in tracked_files(root, str(base.relative_to(root)), "*.css"):
            if "node_modules" in path.parts or ".next" in path.parts:
                continue
            yield path

    gaps: dict[str, list[str]] = {}
    for scope, c, base, has_dark in (("website", css, root / "website", True), ("admin", admin_css, root / "admin/src", False)):
        for path in css_files(base):
            src = path.read_text(encoding="utf8")
            rules = [(m.group(1).strip(), m.group(2), _line_of(src, m.start(2))) for m in re.finditer(r"([^{}@]+)\{([^{}]*)\}", src)]
            dark_tails = {part.split()[-1] for sel, _, _ in rules if DARK_SEL.search(sel) for part in sel.split(",") if part.split()}
            for sel, body, line in rules:
                fm = CSS_COLOR_PROP.search(body)
                if not fm:
                    continue
                raw = fm.group(1)
                dark = bool(DARK_SEL.search(sel))
                label = f"{path.relative_to(root)}:{line}: {sel.splitlines()[-1].strip()[:50]}"
                try:
                    fg = c.color(raw, "dark" if dark else "light")
                except Exception:  # noqa: BLE001 — 解不出來的 fg 不是本掃描的對象
                    continue
                need = 3.0 if re.search(r"icon|glyph|dot|badge-ring", sel) else 4.5
                family = bool(ACCENT_FAMILY.search(raw)) and not ON_DARK_NAME.search(raw) and sel.split()[-1] not in DECORATIVE_SELECTORS and not any(part.split()[-1] in DECORATIVE_SELECTORS for part in sel.split(",") if part.split())
                bg_decl = CSS_BG_DECL.search(body)
                bg = None
                if bg_decl and "gradient" not in bg_decl.group(1) and "url(" not in bg_decl.group(1):
                    try:
                        bg = c.color(bg_decl.group(1).strip().replace("!important", "").strip(), "dark" if dark else "light")
                    except Exception:  # noqa: BLE001
                        bg = None
                if not dark:
                    # (1) 淺色的品牌／狀態 token 當文字：有同規則的不透明底就比對，沒有底就直接不合格
                    if family and _is_light(fg) and not _canonical_on_dark(fg):
                        if bg is None and not bg_decl:
                            bad.append(f"(1) 淺色 token 當文字（{raw[:40]} = {hexs(fg)}，沒有深色底）{label}")
                        elif bg is not None and bg[3] >= 1:
                            r = ratio(fg, bg)
                            if r + 1e-9 < need:
                                bad.append(f"(1) 淺色 token × 同規則底 {r:.2f} < {need:g} {label}")
                    # (4) 固定色（品牌色、狀態色）的規則沒有暗色覆寫：在暗色 surface（半透明底就疊上去）上也算一次
                    fixed = FIXED_TOKEN.search(raw) is not None
                    if has_dark and fixed and any(part.split()[-1] not in dark_tails for part in sel.split(",") if part.split()) and (bg is None or bg[3] < 1):
                        worst = min(_ratio_vs(fg, over(bg, sf) if bg is not None else sf) for sf in dark_surfs)
                        if worst + 1e-9 < need:
                            gaps.setdefault(str(path.relative_to(root)), []).append(f"(4) 沒有暗色覆寫，暗色 surface 上 {worst:.2f} < {need:g} {label}")

    for f, labels in sorted(gaps.items()):
        if len(labels) > KNOWN_DARK_GAPS.get(f, 0):
            bad.extend(labels[KNOWN_DARK_GAPS.get(f, 0) :])
    if PRINT_GAPS:
        print("KNOWN_DARK_GAPS = " + repr({f: len(v) for f, v in sorted(gaps.items())}))

    # (3) admin TSX：inline style 與 text-[var(...)]
    admin_tsx_color = re.compile(r"""(?:color:\s*["']|text-\[)(var\(--[\w-]+\))""")
    for path in tracked_files(root, "admin/src", "*.tsx"):
        src = path.read_text(encoding="utf8")
        for m in admin_tsx_color.finditer(src):
            try:
                fg = admin_css.color(m.group(1), "light")
            except Exception:  # noqa: BLE001
                continue
            if _is_light(fg) and not _canonical_on_dark(fg) and re.search(r"brand-|--hk-(?:primary|secondary)|admin-primary", m.group(1)):
                near = src[max(0, m.start() - 160) : m.end() + 160]
                if not re.search(r"background:\s*[\"'](?:var\(--(?:admin-primary|color-ink-[6-9]|color-brand-ink)|#[0-3])", near):
                    bad.append(f"(1) 淺色 token 當文字（{hexs(fg)}）{path.relative_to(root)}:{_line_of(src, m.start())}: {m.group(0)[:50]}")

    # TSX（website）：dark: 群組也比，不只比同一字串的亮色群組
    for path in tracked_files(root, "website", "*.ts*"):
        if "node_modules" in path.parts or ".next" in path.parts or "tests" in path.parts or ".test." in path.name or path.suffix not in (".ts", ".tsx"):
            continue
        src = path.read_text(encoding="utf8")
        for m in re.finditer(r"\"[^\"\n]*\"|'[^'\n]*'|`[^`]*`", src):
            lit = m.group(0)
            if "dark:" not in lit or "bg-hk-" not in lit:
                continue
            bgs = [b for b in TW_CLASS.finditer(lit) if b.group(3) == "bg" and "dark:" in b.group(1)]
            texts = [f for f in TW_CLASS.finditer(lit) if f.group(3) == "text"]
            has_dark_text = re.search(r"dark:(?:[a-z0-9\[\]=&>_-]+:)*!?text-", lit) is not None
            for b in bgs:
                try:
                    base = css.token(f"--color-hk-{b.group(4)}", "dark")
                except KeyError:
                    continue
                a = int(b.group(5)) / 100 if b.group(5) else 1.0
                variant = b.group(1)[len("dark:") :] if b.group(1).startswith("dark:") else None
                if variant is None:
                    continue
                if any(f.group(1) == b.group(1) for f in texts) or has_dark_text:
                    fgs = [f for f in texts if f.group(1) == b.group(1)]
                else:
                    fgs = [f for f in texts if f.group(1) == variant]  # 沒有 dark: 的字色，在暗色底上沿用
                for f in fgs:
                    try:
                        fgc = css.token(f"--color-hk-{f.group(4)}", "dark")
                    except KeyError:
                        continue
                    worst = min(_ratio_vs(fgc, over((*base[:3], a), sf)) for sf in dark_surfs)
                    if worst + 1e-9 < 4.5 and not _canonical_on_dark(fgc):
                        bad.append(f"(4) TSX dark: 群組 {worst:.2f} {path.relative_to(root)}:{_line_of(src, m.start())}: {b.group(0)} × {f.group(0)}")

    # (2) Dart：別名先解析到最終色，再判斷
    secondary_values = {app: d.token("brandSecondary") for app, d in darts.items()}
    for app, d in darts.items():
        for path in tracked_files(root, f"{app}-app/lib", "*.dart"):
            if path.name == "app_colors.dart":
                continue
            lines = path.read_text(encoding="utf8").split("\n")
            for i, line in enumerate(lines):
                cm = DART_COLOR_CLAUSE.search(line)
                if not cm:
                    continue
                back = " ".join(lines[max(0, i - 5) : i + 1])
                if max(back.rfind(k) for k in DART_TEXT_CTX) <= max(back.rfind(k) for k in DART_FILL_CTX):
                    continue
                for am in DART_APP_COLOR.finditer(cm.group(1)):
                    try:
                        fg = d.token(am.group(1))
                    except (KeyError, ValueError):
                        continue
                    rel = f"{path.relative_to(root)}:{i + 1}"
                    sv = secondary_values[app]
                    if abs(fg[0] - sv[0]) < 1 and abs(fg[1] - sv[1]) < 1 and abs(fg[2] - sv[2]) < 1:
                        bad.append(f"(2) 解析後是輔色的字 AppColors.{am.group(1)} {rel}")
                        continue
                    if not _is_light(fg) or _canonical_on_dark(fg) or (am.group(2) and "alpha" in am.group(2)) or not _dart_brand_family(d, am.group(1)):
                        continue
                    rel_file = str(path.relative_to(root))
                    if rel_file in ON_DARK_FILES:
                        worst = min(ratio(fg, d.token(n)) for n in ON_DARK_FILES[rel_file])
                        if worst + 1e-9 < 4.5:
                            bad.append(f"(1) 深色漸層底上的淺色字 {worst:.2f} {rel}")
                        continue
                    # 往前找同一個 widget 的底；找不到就只放行明確的 on-dark 色
                    fills = []
                    for j in range(max(0, i - 14), i):
                        fm = DART_FILL_LINE.search(lines[j])
                        ctx = " ".join(lines[max(0, j - 3) : j + 1])
                        if fm and max(ctx.rfind(k) for k in DART_FILL_CTX) > max(ctx.rfind(k) for k in DART_TEXT_CTX):
                            try:
                                fills.append(d.token(fm.group(1)) if fm.group(1) else parse_hex("#" + fm.group(2)[2:] + fm.group(2)[:2]))
                            except (KeyError, ValueError):
                                pass
                    if fills:
                        worst = min(_ratio_vs(fg, over(fills[-1], sf)) for sf in (white, d.token("surfacePage")))
                        if worst + 1e-9 < 4.5:
                            bad.append(f"(1) 淺色字 AppColors.{am.group(1)} × 同 widget 的底 {worst:.2f} {rel}")
                    elif not _canonical_on_dark(fg):
                        bad.append(f"(1) 淺色 token 當文字 AppColors.{am.group(1)}（{hexs(fg)}，找不到深色底）{rel}")
    for b in bad:
        rep.errors.append(f"使用處掃描 3：{b}")
    return bad


def main(argv: list[str]) -> int:
    ap = argparse.ArgumentParser()
    ap.add_argument("--root", default=str(DEFAULT_ROOT))
    ap.add_argument("--quiet", action="store_true", help="只印失敗的列")
    ap.add_argument("--print-gaps", action="store_true", help="印出目前的 (4) 既有缺口筆數，用來更新 KNOWN_DARK_GAPS")
    args = ap.parse_args(argv)
    root = Path(args.root)
    global PRINT_GAPS
    PRINT_GAPS = args.print_gaps

    rep = Report()
    css = Css(root)
    check_web(css, rep)
    check_status_web(css, rep)
    check_home_scope(root, rep)
    check_admin_sidebar(root, rep)
    scan_usage(root, rep)
    darts = {app: Dart(root / f"{app}-app/lib/core/theme/app_colors.dart") for app in ("buyer", "seller")}
    scan_usage_extra(root, css, darts, rep)
    admin_css = Css(root, [root / "admin/src/app/globals.css", root / "admin/src/styles/AdminVisualRefresh.css"])
    scan_usage_text(root, css, admin_css, darts, rep)
    for app, d in darts.items():
        check_dart(app, d, rep)

    last = None
    for group, label, fg, bg, r, need, verdict in rep.rows:
        if args.quiet and verdict != "FAIL":
            continue
        if group != last:
            print(f"\n## {group}")
            last = group
        print(f"{verdict:4}  {r:5.2f} (需 {need:g})  {fg} / {bg}  {label}")
    total = len(rep.rows)
    enforced = sum(1 for r in rep.rows if r[6] != "info")
    print(f"\n共 {total} 組，判定 {enforced} 組，不合格 {rep.fail} 組，解不出來 {len(rep.errors)} 組")
    for e in rep.errors:
        print("ERROR", e)
    return 1 if rep.fail or rep.errors else 0


if __name__ == "__main__":
    sys.exit(main(sys.argv[1:]))
