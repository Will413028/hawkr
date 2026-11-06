---
description: Spring Boot auto-configuration ordering in shared Java modules
paths:
  - "backend/services/java/shared/**/*AutoConfiguration.java"
---

# Java auto-configuration

- 帶 `@ConditionalOnBean`／`@ConditionalOnMissingBean` 的自動設定一律宣告 `before`／`after`，不靠類別全名的排序。
