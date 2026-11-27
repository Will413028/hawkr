#!/usr/bin/env bash
set -euo pipefail
script="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/generate.sh"
fixture="$(mktemp -d)"
trap 'rm -rf "$fixture"' EXIT
mkdir -p "$fixture/api" "$fixture/bin" "$fixture/mvbin"
cp "$script" "$fixture/api/generate.sh"
printf '#!/usr/bin/env bash\nexit 0\n' > "$fixture/api/check-buf-version.sh"
chmod +x "$fixture/api/generate.sh" "$fixture/api/check-buf-version.sh"
cat > "$fixture/bin/buf" <<'FAKE'
#!/usr/bin/env bash
base=.
template=buf.gen.yaml
while [[ $# -gt 0 ]]; do
  case "$1" in
    -o|--output) base="$2"; shift 2 ;;
    --template) template="$2"; shift 2 ;;
    *) shift ;;
  esac
done
outs="$(sed -nE "s/^[[:space:]]*-?[[:space:]]*out:[[:space:]]*[\"']?([^\"'#[:space:]]+).*/\\1/p" "$template")"
for o in $outs; do rm -rf "${base:?}/$o"; done
[[ -z "${FAKE_BUF_FAIL:-}" ]] || { echo "Failure: plugin exited 1" >&2; exit 1; }
for o in $outs; do
  case " ${FAKE_BUF_SKIP:-} " in *" $o "*) continue ;; esac
  mkdir -p "$base/$o"
  echo new > "$base/$o/new.txt"
done
[[ -z "${FAKE_BUF_STRAY:-}" ]] || { mkdir -p "$base/other"; echo x > "$base/other/x.dart"; }
exit 0
FAKE
cat > "$fixture/mvbin/mv" <<'FAKE'
#!/usr/bin/env bash
n=$(( $(cat "$FAKE_MV_COUNT" 2>/dev/null || echo 0) + 1 ))
echo "$n" > "$FAKE_MV_COUNT"
if [[ "$n" == "${FAKE_MV_FAIL_ON:-}" ]]; then
  [[ -z "${FAKE_MV_KILL:-}" ]] || kill -TERM "$PPID"
  echo "mv: simulated failure" >&2
  exit 1
fi
exec /bin/mv "$@"
FAKE
chmod +x "$fixture/bin/buf" "$fixture/mvbin/mv"

single='version: v2
clean: true
plugins:
  - local:
      - dart
      - run
      - protoc_plugin:protoc_plugin
    out: "lib/gen"'

reset() {
  rm -rf "$fixture/pkg"
  mkdir -p "$fixture/pkg/lib/gen" "$fixture/pkg/src/gen"
  printf 'keep\n' > "$fixture/pkg/lib/gen/stub.dart"
  printf 'keep\n' > "$fixture/pkg/src/gen/stub.ts"
  printf 'hand\n' > "$fixture/pkg/lib/hand.dart"
  printf '%s\n' "${1:-$single}" > "$fixture/pkg/buf.gen.yaml"
  rm -f "$fixture/mvcount"
}
run() {
  (cd "$fixture/pkg" && FAKE_MV_COUNT="$fixture/mvcount" PATH="$fixture/bin:/usr/bin:/bin" bash ../api/generate.sh buf.gen.yaml)
}
run_fake_mv() {
  (cd "$fixture/pkg" && FAKE_MV_COUNT="$fixture/mvcount" PATH="$fixture/mvbin:$fixture/bin:/usr/bin:/bin" bash ../api/generate.sh buf.gen.yaml)
}
fail=0
check() { "${@:2}" || { echo "FAIL: $1" >&2; fail=1; }; }
# shellcheck disable=SC2329 # 經由 check 間接呼叫
no_leftovers() { [[ -z "$(cd "$fixture/pkg" && find . -name '.gen-staging.*' -o -name '*.gen-old')" ]]; }

reset
result=0; FAKE_BUF_FAIL=1 run || result=$?
check "buf 失敗時應該失敗" test "$result" -ne 0
check "buf 失敗時生成物被刪了" test -f "$fixture/pkg/lib/gen/stub.dart"
check "buf 失敗時不該留暫存目錄" no_leftovers

reset
result=0; FAKE_BUF_STRAY=1 run || result=$?
check "out 以外有生成檔時應該失敗" test "$result" -ne 0
check "out 以外有生成檔時生成物被刪了" test -f "$fixture/pkg/lib/gen/stub.dart"
check "out 以外的檔案不該換進來" test ! -e "$fixture/pkg/other"

reset
result=0; run || result=$?
check "成功時應該回 0" test "$result" -eq 0
check "成功時應有新生成物" test -f "$fixture/pkg/lib/gen/new.txt"
check "成功時舊生成物應被清掉" test ! -e "$fixture/pkg/lib/gen/stub.dart"
check "out 以外的手寫檔不該動" test -f "$fixture/pkg/lib/hand.dart"
check "成功時不該留暫存或 .gen-old" no_leftovers

reset 'version: v2
plugins:
  - local: a
    out: ./lib/gen/
  - local: b
    out: src/gen'
result=0; run || result=$?
check "多個 out 成功時應該回 0" test "$result" -eq 0
check "多個 out：lib/gen 應換新" test -f "$fixture/pkg/lib/gen/new.txt" -a ! -e "$fixture/pkg/lib/gen/stub.dart"
check "多個 out：src/gen 應換新" test -f "$fixture/pkg/src/gen/new.txt" -a ! -e "$fixture/pkg/src/gen/stub.ts"

reset 'version: v2
plugins:
  - local: a
    out: lib/gen
  - local: b
    out: lib/gen/connect'
result=0; run || result=$?
check "巢狀 out 成功時應該回 0" test "$result" -eq 0
check "巢狀 out：上層生成物" test -f "$fixture/pkg/lib/gen/new.txt"
check "巢狀 out：子 out 生成物" test -f "$fixture/pkg/lib/gen/connect/new.txt"
check "巢狀 out：舊檔應清掉" test ! -e "$fixture/pkg/lib/gen/stub.dart"

reset 'version: v2
plugins:
  - local: a
    out: lib/gen
  - local: b
    out: src/gen'
result=0; FAKE_BUF_SKIP=src/gen run || result=$?
check "沒寫出檔案的 out 應該失敗" test "$result" -ne 0
check "沒寫出檔案的 out 不該被刪" test -f "$fixture/pkg/src/gen/stub.ts"
check "失敗時其他 out 也不該動" test -f "$fixture/pkg/lib/gen/stub.dart"

reset 'version: v2
plugins:
  - local: a
    out: lib/../../escape'
result=0; run || result=$?
check "含 .. 段的 out 應該失敗" test "$result" -ne 0
check "含 .. 段的 out 時生成物不動" test -f "$fixture/pkg/lib/gen/stub.dart"
reset 'version: v2
plugins:
  - local: a
    out: lib/a..b'
result=0; run || result=$?
check "名字含 .. 的 out 應該可以" test "$result" -eq 0 -a -f "$fixture/pkg/lib/a..b/new.txt"

reset
result=0; FAKE_MV_FAIL_ON=2 run_fake_mv || result=$?
check "搬入失敗時應該失敗" test "$result" -ne 0
check "搬入失敗時舊生成物應還原" test -f "$fixture/pkg/lib/gen/stub.dart"
check "搬入失敗時不該留 .gen-old" no_leftovers

reset
result=0; FAKE_MV_FAIL_ON=2 FAKE_MV_KILL=1 run_fake_mv || result=$?
check "中斷時應該失敗" test "$result" -ne 0
check "中斷時舊生成物應還原" test -f "$fixture/pkg/lib/gen/stub.dart"
check "中斷時不該留 .gen-old" no_leftovers

[[ "$fail" -eq 0 ]] && echo "PASS"
exit "$fail"
