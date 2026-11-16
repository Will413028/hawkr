#!/usr/bin/env bash
# generate.sh 的行為測試：buf 的 clean: true 會在外掛執行前清空 out 目錄，外掛失敗（指令不在 PATH、
# 依賴沒裝）時生成物不能被刪；成功時 out 目錄整個換成新的。用假的 buf 模擬，不需要真的 buf、dart 或 pnpm。
set -euo pipefail
script="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/generate.sh"
fixture="$(mktemp -d)"
trap 'rm -rf "$fixture"' EXIT
mkdir -p "$fixture/api" "$fixture/bin"
cp "$script" "$fixture/api/generate.sh"
printf '#!/usr/bin/env bash\nexit 0\n' > "$fixture/api/check-buf-version.sh"
chmod +x "$fixture/api/generate.sh" "$fixture/api/check-buf-version.sh"
# 假 buf：照 --output 前綴到 out（lib/gen），先清空（clean: true），FAKE_BUF_FAIL 時接著失敗（外掛或依賴缺）。
cat > "$fixture/bin/buf" <<'FAKE'
#!/usr/bin/env bash
base=.
while [[ $# -gt 0 ]]; do
  case "$1" in -o|--output) base="$2"; shift 2 ;; *) shift ;; esac
done
rm -rf "$base/lib/gen"
[[ -z "${FAKE_BUF_FAIL:-}" ]] || { echo "Failure: plugin exited 1" >&2; exit 1; }
mkdir -p "$base/lib/gen/sub"
echo new > "$base/lib/gen/sub/new.dart"
[[ -z "${FAKE_BUF_STRAY:-}" ]] || { mkdir -p "$base/other"; echo x > "$base/other/x.dart"; }
FAKE
chmod +x "$fixture/bin/buf"
# 區塊式的 local 與加引號的 out，舊的 sed 解析都讀不到。
cat > "$fixture/buf.gen.yaml" <<'YAML'
version: v2
clean: true
plugins:
  - local:
      - dart
      - run
      - protoc_plugin:protoc_plugin
    out: "lib/gen"
YAML

reset() {
  rm -rf "$fixture/pkg"
  mkdir -p "$fixture/pkg/lib/gen"
  printf 'keep\n' > "$fixture/pkg/lib/gen/stub.dart"
  printf 'hand\n' > "$fixture/pkg/lib/hand.dart"
  cp "$fixture/buf.gen.yaml" "$fixture/pkg/buf.gen.yaml"
}
run() { (cd "$fixture/pkg" && PATH="$fixture/bin:/usr/bin:/bin" bash ../api/generate.sh buf.gen.yaml); }
fail=0
check() { "${@:2}" || { echo "FAIL: $1" >&2; fail=1; }; }

# 1. 指令在 PATH 但依賴沒裝：buf 清空後失敗，生成物要留著。
reset
result=0; FAKE_BUF_FAIL=1 run || result=$?
check "buf 失敗時應該失敗" test "$result" -ne 0
check "buf 失敗時生成物被刪了" test -f "$fixture/pkg/lib/gen/stub.dart"

# 2. 生成了 out 以外的檔案（out 沒讀到）：失敗，生成物不動。
reset
result=0; FAKE_BUF_STRAY=1 run || result=$?
check "out 以外有生成檔時應該失敗" test "$result" -ne 0
check "out 以外有生成檔時生成物被刪了" test -f "$fixture/pkg/lib/gen/stub.dart"
check "out 以外的檔案不該換進來" test ! -e "$fixture/pkg/other"

# 3. 成功：out 目錄整個換新（舊檔不留），out 以外的手寫檔不動。
reset
result=0; run || result=$?
check "成功時應該回 0" test "$result" -eq 0
check "成功時應有新生成物" test -f "$fixture/pkg/lib/gen/sub/new.dart"
check "成功時舊生成物應被清掉" test ! -e "$fixture/pkg/lib/gen/stub.dart"
check "out 以外的手寫檔不該動" test -f "$fixture/pkg/lib/hand.dart"

[[ "$fail" -eq 0 ]] && echo "PASS"
exit "$fail"
