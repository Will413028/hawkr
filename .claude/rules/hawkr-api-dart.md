---
description: Code generation and formatting rules for the Dart API package
paths:
  - "packages/hawkr_api/**"
---

# hawkr_api (Dart)

- 新 worktree 第一次跑 `../../api/generate.sh buf.gen.yaml` 前，先各暖機一次 `echo '' | dart run protoc_plugin:protoc_plugin >/dev/null 2>&1; echo '' | dart run connectrpc:protoc-gen-connect-dart >/dev/null 2>&1`：buf 同時起兩個要編譯的外掛會 `signal: killed` 並清空 `lib/gen`；被清空時 `git restore -- packages/hawkr_api/lib/gen` 後重跑。派 agent 生成 Dart 時把這兩行寫進 prompt。
- 不跑 `dart format lib`：它會格式化 `lib/gen` 的生成碼（dart format 不能排除目錄）；只格式化手寫檔，誤改時 `git restore -- packages/hawkr_api/lib/gen`。
