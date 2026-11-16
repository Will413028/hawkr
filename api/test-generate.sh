#!/usr/bin/env bash
# generate.sh 的行為測試：外掛的指令不在 PATH 時，失敗且不清空生成物（buf 的 clean: true 會先刪 out 目錄）。
# 用假的 buf 記錄是否被呼叫，不需要真的 buf、dart 或 pnpm。
set -euo pipefail
script="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/generate.sh"
fixture="$(mktemp -d)"
trap 'rm -rf "$fixture"' EXIT
mkdir -p "$fixture/api" "$fixture/bin" "$fixture/pkg/lib/gen"
cp "$script" "$fixture/api/generate.sh"
printf '#!/usr/bin/env bash\nexit 0\n' > "$fixture/api/check-buf-version.sh"
chmod +x "$fixture/api/generate.sh" "$fixture/api/check-buf-version.sh"
cat > "$fixture/bin/buf" <<'FAKE'
#!/usr/bin/env bash
echo called >> "$FAKE_BUF_LOG"
rm -rf lib/gen
FAKE
chmod +x "$fixture/bin/buf"
printf 'keep\n' > "$fixture/pkg/lib/gen/stub.dart"
cat > "$fixture/pkg/buf.gen.yaml" <<'YAML'
version: v2
clean: true
plugins:
  - local: [hawkr-missing-tool-xyz, run, protoc_plugin:protoc_plugin]
    out: lib/gen
  - local: protoc-gen-present
    out: lib/gen
YAML
printf '#!/bin/sh\n' > "$fixture/bin/protoc-gen-present"
chmod +x "$fixture/bin/protoc-gen-present"
export FAKE_BUF_LOG="$fixture/buf.log"
# 只放 fixture/bin 與系統基本指令，確保 hawkr-missing-tool-xyz 不存在。
result=0
(cd "$fixture/pkg" && PATH="$fixture/bin:/usr/bin:/bin" bash ../api/generate.sh buf.gen.yaml) || result=$?
fail=0
[[ "$result" -ne 0 ]] || { echo "FAIL: 缺指令時應該失敗" >&2; fail=1; }
[[ -f "$fixture/pkg/lib/gen/stub.dart" ]] || { echo "FAIL: 生成物被刪了" >&2; fail=1; }
[[ ! -f "$FAKE_BUF_LOG" ]] || { echo "FAIL: 缺指令時不應呼叫 buf" >&2; fail=1; }
# 指令都在時照常交給 buf。
sed -i.bak 's/hawkr-missing-tool-xyz/protoc-gen-present/' "$fixture/pkg/buf.gen.yaml"
(cd "$fixture/pkg" && PATH="$fixture/bin:/usr/bin:/bin" bash ../api/generate.sh buf.gen.yaml) || { echo "FAIL: 指令都在時應該成功" >&2; fail=1; }
[[ -f "$FAKE_BUF_LOG" ]] || { echo "FAIL: 指令都在時應呼叫 buf" >&2; fail=1; }
[[ "$fail" -eq 0 ]] && echo "PASS"
exit "$fail"
