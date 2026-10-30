---
name: backend-e2e-verify
description: Proves a backend change end-to-end on the local stack (monolith as the entry point, legacy gateway and services behind it, local Postgres and RabbitMQ) with a dev token instead of a real login, reading results back from the DB and capturing the RabbitMQ wire payload. Use when a backend change needs live evidence beyond unit and integration tests, when an event's wire shape or absence must be proven, or when designing the scenario list for a characterization test.
---

# backend-e2e-verify

單元與 integration 測試證明程式碼的意圖；這份 skill 證明組裝起來的系統真的這樣做：請求經過單體的認證與
gateway 轉送、寫進 DB 的是哪個身分、送上 RabbitMQ 的欄位長什麼樣。只在本機跑，不連任何外部環境（根目錄
`AGENTS.md`「不連到舊公司」）。

## 1. 起環境

依 `backend/AGENTS.md`「指令」（本機的 docker context、port 衝突等差異看 `AGENTS.local.md`）：

```bash
cd backend
make infra-up            # Postgres、Redis、RabbitMQ
make migrate             # 單體的 schema
make serve               # 入口 :8080；Connect 在根路徑（/hawkr.<package>.<Service>/…），/api/v1/* 等轉給 gateway（:9900）
make worker              # River job 與 legacy 事件轉送；驗事件時必須起，只起 serve 驗不到
./dev.sh                 # 舊服務與 gateway（docker-compose-local.yml），驗還沒搬進單體的路徑才需要
```

`curl -fsS localhost:8080/readyz` 回 204 才開始。

## 2. 取得身分

```bash
TOKEN=$(make -s dev-token args='-account 2001')   # seed 的買家（為既有帳號建 App 種類的 session）
make -s dev-token args='-account 9'               # seed 的賣家
make -s dev-token args='-admin -user dev_ops'              # 後台
```

- seed 帳號在 `infra/seed-data/01-account.sql`，帳號 ID 與 `buyer_id`、`store_id` 刻意不同（`buyer_id = account_id + 50000`）。驗身分時用這些帳號，兩個 ID 空間互換的缺陷才藏不住。
- 需要全新帳號時不帶 `-account`（`make -s dev-token`）：建立一個沒有登入身分的新帳號，ID 印到 stderr，不會動到其他測試資料。帳號 ID 不存在是錯誤，不會默默建立。
- 驗證產生的資料用可辨識的前綴（店名、備註），驗完清掉；不要沿用前一輪留下的帳號或資料 ID。
- token 是本機金鑰簽的，不要貼進 commit、issue 或 log。

## 3. 驅動與讀回

```bash
curl -sS -H "Authorization: Bearer $TOKEN" -H 'Content-Type: application/json' \
  -X POST localhost:8080/hawkr.<package>.<Service>/<Method> -d '{...}'
curl -sS -H "Authorization: Bearer $TOKEN" localhost:8080/api/v1/app/...
```

Connect 的路徑與欄位以根目錄 `api/` 的 proto 為準；舊 REST 以 `backend/api/openapi/openapi.yaml` 為準。

- **寫入回 200 不證明寫對了身分**：讀回來（GET 一次，或直接查 DB 的業務鍵欄位）才算。寫在錯的身分底下，從呼叫端看與成功一模一樣。
- 查 DB：`docker exec -i hawkr-postgres psql -U dbadmin -d hawkr -c '<SQL>'`。單體的表寫完整名稱（`identity.accounts`、`buyer.buyers`）；舊服務各自是 `hawkr_<服務>` database。
- 業務表用業務鍵（`buyer_id`、`store_id`），`account_id` 只是登入 principal。斷言讀回的值落在哪個 ID 空間。

## 4. 擷取 RabbitMQ 事件

證明「事件真的送出、欄位形狀正確」，或「這個操作什麼都不送」。

1. 找 exchange 與 routing key：grep 發布端（單體的 legacy 轉送在 `internal/legacy/rabbitmq`，舊服務是各自的 publisher）。不要從事件名稱猜：多個邏輯事件可能共用一個 routing key，靠 payload 欄位區分。
2. 把 [mq_capture.go.tmpl](mq_capture.go.tmpl) 複製到 `backend/cmd/mq-capture-<隨機 6 碼>/main.go`（每個 session 自己的路徑），填 exchange 與 routing key，在 `backend/` 背景執行 `GOWORK=off go run ./cmd/mq-capture-<碼>`。
3. 驅動會發事件的真實流程，讀擷取輸出，檢查受測欄位與 ID 空間。
4. 證明「不送」：routing key 用 `#` 綁整個 exchange，擷取期間驅動該操作，同時驅動一個**會送**的操作當正對照；正對照沒抓到，整輪不算數。
5. 結束後停掉程式、刪除 `backend/cmd/mq-capture-<碼>`，確認 `git status` 乾淨。

## 5. 情境矩陣

每個斷言都要有這一輪的實際回應作證據（HTTP 狀態、錯誤原因、必要時 DB 讀回），不沿用以前的結果。最少覆蓋：

- happy path，用 seed 或當場建立的資料；
- 改動新增或修改的每個分支與錯誤原因，各自的觸發條件；
- 權限邊界：跨帳號存取（IDOR）、缺角色或 capability、管理員與一般帳號；
- 冪等與重放：同一個冪等鍵重送、重複確認、first-accept-wins；
- 事件側：有沒有送、欄位形狀、ID 空間（`account_id`／`buyer_id`／`store_id`）；
- 遷移中的模組：同一份情境打舊服務與單體，結果一致。

沒跑的情境寫「未跑（原因）」，不留空白。

## 6. 從手動到測試

同一組情境需要第二次手動重跑時，改寫成特性測試，讓 CI 與遷移都能重用：`make new-characterization name=<模組> legacy=<舊服務>`
產生骨架，`scripts/characterize.sh <模組>` 經入口執行（見 `backend/AGENTS.md`）。手動驗證只留給還沒有測試骨架、或需要看 wire 的情況。
