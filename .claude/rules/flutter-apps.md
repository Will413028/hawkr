---
description: Code generation, verification and coverage rules for the Flutter apps
paths:
  - "seller-app/**"
  - "buyer-app/**"
---

# Flutter apps

- build_runner 一律加 `--delete-conflicting-outputs`（舊版在非互動環境遇到既有輸出會 NPE；CI 的生成檢查也帶）。diff 裡任何含 `part '*.g.dart'` 或 `part '*.freezed.dart'` 的檔有改動就重新生成再 commit（hash 跟著整個檔的內容走）；派 agent 時照這個觸發寫，不寫成「改到 provider 才跑」。
- 開發中只跑 `analyze`、`custom_lint`、改到的功能的測試（`fvm flutter test <目錄或檔案>`）、`packages/` 改到的 dart package 測試，再加上全部以字串讀原始碼的守門測試（`rg -l "readAsStringSync|listSync" test`）：它們不 import 被改的檔，依 import 挑測試會漏掉。
- 完整測試與 coverage ratchet 交給 CI 的 `Unit and widget tests`，回報列為「待 CI」直到該 job 綠。commit 前每支 App 跑 `bash scripts/deadcode.sh`（CI `_flutter-quality.yml` 的 hygiene step 擋沒人用的檔與未宣告的直接 import），design:check、storybook 同步照舊。派 agent 做 App 的改動時照這條寫。
- 刪掉 production 程式後 coverage ratchet 會失敗：推上去後下載該 run 的 `<app>-coverage` artifact，用 `fvm dart run tool/coverage_ratchet.dart --lcov <lcov.info> --print-baseline > tool/coverage_baseline.json` 依 CI 實測重算（`docs/testing_strategy.md` 允許「production source 合理移除」時更新）；本機 LCOV 重算的結果與 CI 不同。
