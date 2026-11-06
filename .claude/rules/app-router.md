---
description: go_router shell-route rules for seller-app
paths:
  - "seller-app/lib/**/*.dart"
---

# App router

- `StatefulShellRoute.indexedStack` 的 branch 路由一律用 `go`，不用 `push`；判斷頁面是否可見用 `TickerMode.valuesOf(context).enabled`。
