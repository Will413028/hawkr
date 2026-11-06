---
description: Regeneration rules after editing CodeMeta documents
paths:
  - "backend/docs/代碼管理（CodeMeta）/**"
---

# CodeMeta

- 改 CodeMeta 的 `.md` 後跑 `generate.sh --all`：CI 的 drift 檢查只含 `--go --go-media --web --admin --flutter`，Java 生成物會漏。Java 產生器一個列舉一個檔、不刪舊檔，刪掉的列舉要手動 `git rm` 對應的 `.java`。`--all` 會帶出 Java 與 `bizcode.go` 的舊 drift，只留本次的變動，最後用 CI 旗標重跑確認 `git status` 乾淨。派 agent 改到這個目錄時把這條寫進 prompt。
