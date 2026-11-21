#!/usr/bin/env python3
"""檢查 test-map.md 與 Java 測試原始碼一致、沒有空格。

預期的列清單不寫死：掃 backend/services/java/media-service/src/test 底下每個 *Test.java 的 @Test，取得
(相對路徑, 方法宣告行, 方法名)，逐列比對 test-map.md 的對照表：

  - 列數必須等於掃到的 @Test 數（缺列、多列、重複都失敗）；
  - 每列的「檔:行」與「測試方法」必須與掃描結果相符（Java 測試被改動、行號漂移時失敗）；
  - 「Go 對應」與「退場理由」不得為空，且恰好一欄有內容、另一欄是 `—`。

用法：
  python3 check-test-map.py [test-map.md]   # 檢查，失敗回 1
  python3 check-test-map.py --self-test     # 對檔案做幾種突變，確認每一種都會被抓到；全部抓到回 0
"""
import re
import sys
from pathlib import Path

HERE = Path(__file__).resolve().parent
BACKEND = HERE.parents[2]
JAVA_TEST_ROOT = BACKEND / "services/java/media-service/src/test/java/io/github/will413028/hawkr/media"
DEFAULT_MAP = HERE / "test-map.md"
DASH = "—"

TEST_RE = re.compile(r"^\s*@Test\b")
METHOD_RE = re.compile(r"\bvoid\s+(\w+)\s*\(")
ROW_RE = re.compile(r"^\| (\d+) \| `([^`]+):(\d+)` \| `([^`]+)` \| (.*?) \| (.*?) \|$")


def scan_java():
    expected = []
    for path in sorted(JAVA_TEST_ROOT.rglob("*Test.java")):
        lines = path.read_text(encoding="utf-8").split("\n")
        rel = path.relative_to(JAVA_TEST_ROOT).as_posix()
        i = 0
        while i < len(lines):
            if TEST_RE.match(lines[i]):
                j = i
                while j < len(lines) and not METHOD_RE.search(lines[j]):
                    j += 1
                if j == len(lines):
                    raise SystemExit(f"{rel}:{i + 1} 的 @Test 後面找不到方法宣告")
                expected.append((rel, j + 1, METHOD_RE.search(lines[j]).group(1)))
                i = j
            i += 1
    return expected


def check(text, expected):
    errors = []
    rows = []
    for n, line in enumerate(text.split("\n"), 1):
        if not line.startswith("| "):
            continue
        m = ROW_RE.match(line)
        if m:
            rows.append((n, m))
        elif re.match(r"^\| \d+ \|", line):
            errors.append(f"test-map.md:{n}：表格列格式不符（欄位數或反引號）")
    if len(rows) != len(expected):
        errors.append(f"列數 {len(rows)} 不等於 Java 掃到的 @Test 數 {len(expected)}")
    seen = set()
    for pos, (n, m) in enumerate(rows):
        idx, rel, ln, method, go, retire = int(m[1]), m[2], int(m[3]), m[4], m[5].strip(), m[6].strip()
        key = (rel, ln, method)
        if idx != pos + 1:
            errors.append(f"test-map.md:{n}：序號 {idx}，預期 {pos + 1}")
        if key in seen:
            errors.append(f"test-map.md:{n}：重複的列 {rel}:{ln} {method}")
        seen.add(key)
        if pos < len(expected) and key != expected[pos]:
            e = expected[pos]
            errors.append(f"test-map.md:{n}：列是 {rel}:{ln} {method}，Java 掃描的第 {pos + 1} 個是 {e[0]}:{e[1]} {e[2]}")
        if not go or not retire:
            errors.append(f"test-map.md:{n}：#{idx} 有空格（Go 對應與退場理由都要有內容，沒有的那欄寫 {DASH}）")
        elif (go == DASH) == (retire == DASH):
            errors.append(f"test-map.md:{n}：#{idx} 兩欄{'都是' if go == DASH else '都填了'}，必須恰好一欄有內容、另一欄是 {DASH}")
    missing = [e for e in expected if e not in seen]
    for rel, ln, method in missing:
        errors.append(f"缺列：{rel}:{ln} {method}")
    return errors, len(rows), len(expected)


def self_test(text, expected):
    """每一種突變都必須讓 check 回報錯誤。"""
    lines = text.split("\n")
    row_idx = [i for i, l in enumerate(lines) if ROW_RE.match(l)]
    if not row_idx:
        return ["自我測試：test-map.md 沒有任何表格列"]
    first, mid = row_idx[0], row_idx[len(row_idx) // 2]

    def mutate(fn):
        copy = list(lines)
        fn(copy)
        return "\n".join(copy)

    def blank_cell(copy):
        m = ROW_RE.match(copy[mid])
        copy[mid] = f"| {m[1]} | `{m[2]}:{m[3]}` | `{m[4]}` |  | {m[6]} |"

    def both_dash(copy):
        m = ROW_RE.match(copy[mid])
        copy[mid] = f"| {m[1]} | `{m[2]}:{m[3]}` | `{m[4]}` | {DASH} | {DASH} |"

    def both_filled(copy):
        m = ROW_RE.match(copy[mid])
        copy[mid] = f"| {m[1]} | `{m[2]}:{m[3]}` | `{m[4]}` | x | y |"

    def drop_row(copy):
        del copy[first]

    def dup_row(copy):
        copy.insert(first + 1, copy[first])

    def drift_line(copy):
        m = ROW_RE.match(copy[mid])
        copy[mid] = f"| {m[1]} | `{m[2]}:{int(m[3]) + 1}` | `{m[4]}` | {m[5]} | {m[6]} |"

    def rename_method(copy):
        m = ROW_RE.match(copy[mid])
        copy[mid] = f"| {m[1]} | `{m[2]}:{m[3]}` | `{m[4]}X` | {m[5]} | {m[6]} |"

    cases = {
        "清空一格": blank_cell,
        "兩欄都是 —": both_dash,
        "兩欄都填": both_filled,
        "刪掉第一列": drop_row,
        "重複第一列": dup_row,
        "行號漂移一行": drift_line,
        "方法名改錯": rename_method,
    }
    failures = []
    if check(text, expected)[0]:
        failures.append("自我測試：原檔本身就沒通過")
    for name, fn in cases.items():
        if not check(mutate(fn), expected)[0]:
            failures.append(f"自我測試：突變「{name}」沒有被抓到")
    return failures


def main(argv):
    do_self_test = "--self-test" in argv
    args = [a for a in argv[1:] if not a.startswith("--")]
    path = Path(args[0]) if args else DEFAULT_MAP
    expected = scan_java()
    text = path.read_text(encoding="utf-8")
    if do_self_test:
        failures = self_test(text, expected)
        for f in failures:
            print(f, file=sys.stderr)
        print("自我測試：7 種突變全部被抓到" if not failures else "自我測試失敗")
        return 1 if failures else 0
    errors, n_rows, n_expected = check(text, expected)
    for e in errors:
        print(e, file=sys.stderr)
    if errors:
        print(f"失敗：{len(errors)} 個問題（表格 {n_rows} 列、Java @Test {n_expected} 個）", file=sys.stderr)
        return 1
    print(f"通過：表格 {n_rows} 列、Java @Test {n_expected} 個，逐列對得上，沒有空格")
    return 0


if __name__ == "__main__":
    sys.exit(main(sys.argv))
