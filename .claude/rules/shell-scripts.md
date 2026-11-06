---
description: Full-width character rules for shell scripts and Makefiles
paths:
  - "**/*.sh"
  - "**/Makefile"
---

# Shell scripts

- 寫或改 shell script、Makefile 後跑 `bash scripts/check-fullwidth-var-expansion.sh`：`$變數` 緊接全形字元要寫 `${變數}`，Makefile recipe 寫 `$${變數}`（CI `repo-hygiene.yml` 跑同一支）。
