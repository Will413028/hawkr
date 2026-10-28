# 採購品項的分類別名由離線 LLM 生成，不逐列覆核

| 項目 | 值 |
|---|---|
| 狀態 | accepted（2026-09-13；2026-10-09 由 spec 與 plan 壓縮而成，見文末來源） |
| 日期 | 2026-09-13 |
| 影響範圍 | ai-service `internal/cache`（`CanonicalCategoryName`、`generatedCategoryAliases`）、`cmd/category-alias-gen`、採購品項解析（`internal/procurement/cropidentity`） |

## 脈絡

採購助手把買家打的品項文字解析成平台分類樹（L1／L2／L3）的節點。買家用消費者慣用名（高麗菜、地瓜、洋芋），樹上是正式名（甘藍、甘薯、馬鈴薯），確定性比對（exact、雙向包含）對這類詞一律 0 命中，只能交給 request-time 的 LLM；而 2026-09-13 實測那條 LLM 路徑因候選上界超標（519 個 L2 > 256、prompt 81,832 bytes > 64 KiB）必定失敗，買家直接落到手填價（候選上界另案以先選 L1 縮小範圍處理，本決策不依賴它）。當時人工的 `synonyms` 只有 37 條。

2026-09-13 的 dev active 樹：L1/L2/L3 = 9/519/1729；花卉 L1 底下 299 個 L2（58%），食品類 220 個 L2、547 個 L3。

約束：
- 外部：系統從未上線，**沒有買家查詢紀錄或點擊資料**可以挖同義詞。
- 外部：分類卡最後一個選項永遠是「沒有符合的品項，自行填寫單價」，錯誤別名的代價是多一次選擇，不是把買家卡死。
- 繼承：`pricebook-service/reference/README.md` 的「不做自動名稱比對」；它是為「對應關係就是最終答案」的取價情境而設（錯作物＝錯價），在有逃生艙的檢索情境不成立，本決策只在分類檢索範圍內推翻它，pricebook 的 crop 映射仍照原規則。
- 繼承：LLM client 只能經 `internal/llmprovider` 建構（backend#3329：曾有四份手刻 provider switch，DeepSeek 的 quirk 漏一份即靜默失敗）；仍成立。

## 考慮過的方案

**別名從哪來、何時生成**

| 方案 | 代價 | 判定 |
|---|---|---|
| 基準：別名掛在目錄的分類實體上（search 的 synonym dictionary，例如 Elasticsearch 的 synonym token filter、Algolia 的 synonyms），由營運在後台策劃、以查詢紀錄挖掘補充 | 沒有查詢紀錄可挖（未上線）；要動 category service 的跨服務契約（proto 加 `alias`）、schema migration、後台 UI；冷啟動仍得有人逐條填 | 否決（資料缺口是外部約束；契約成本在 2026-09 的微服務架構下是真成本） |
| 離線 LLM 逐 L2 生成，產出 commit 進版控，不逐列覆核 | 表內有未經人逐條確認的對應，錯誤別名會把買家導向錯的 L2 | 採用 |
| 離線生成 ＋ 逐列人工覆核 | 數百條逐條審；「不做自動名稱比對」的高覆核成本是為取價情境設計，在有逃生艙的檢索情境不成比例（Will 2026-09-13 裁決） | 否決 |
| Request-time 由 LLM 產生別名並快取 | 第一個打某個詞的買家仍要等 LLM，而那正是當時壞掉的路徑；快取汙染後沒有 diff 可審、沒有地方可改 | 否決 |

**產出物形式**

| 方案 | 代價 | 判定 |
|---|---|---|
| 生成 Go source（`category_alias_gen.go`，與 `synonyms` 同 package、同形狀） | 改別名要重新編譯部署；非工程師不易讀改 | 採用：隨 binary 走、零部署設定；重複的 key 是編譯錯誤；沿用「從 SSOT 生成、不得手改」的既有 pattern |
| CSV ＋ sha256 manifest（pricebook `MAPPING_SEED_CSV_PATH` 先例） | 2 個環境變數同步到 `.env.template`／compose／k8s values；檔案要進 image，缺檔啟動即中止；主要優勢（非工程師可讀改）在不逐列覆核下貶值 | 否決 |
| 存進 category service DB | 同基準列的契約與 migration 成本 | 否決 |

**取代人工覆核的品質機制**

| 方案 | 代價 | 判定 |
|---|---|---|
| `temperature 0.7`、每節點取樣 3 次、只留出現 ≥2 次的（共識過濾） | 約 660 次呼叫；同一輸入重跑結果不完全相同（可重現性改靠 git：重跑的 diff 是審閱單位，檔頭記模型、samples、minAgree、日期） | 採用 |
| `temperature 0` 單次 | 三次幾乎必然相同，共識失去意義；模型半猜的邊緣詞無從濾掉 | 否決（兩者不可兼得） |

**一致性 gate**

| 方案 | 代價 | 判定 |
|---|---|---|
| 對與別名表一起 commit 的樹快照（含花卉）跑 `go test` | 不知道線上樹已漂移 | 採用 |
| PR gate 直接打 dev 分類樹 | 營運在後台改一個分類名就把無關的 PR 弄紅，訓練出忽略紅燈的習慣 | 否決 |

## 決策

分類別名分兩層：人工 `synonyms` 永遠壓過離線生成的 `generatedCategoryAliases`，兩層都只改寫 query、不改分類 label。生成表由 `cmd/category-alias-gen` 產出，不在 CI 執行、不得手改；它對 220 個食品類 L2（排除花卉 L1、不做 L3）逐節點呼叫 LLM（帶 L3 子類、L1、同 L1 兄弟名作排除清單），共識過濾後再過三道機械檢查，未通過的列印到 stderr：

1. target 必須是存在、ACTIVE、Level=2 的節點。
2. alias 不得等於樹上任何節點名（L1／L2／L3，含花卉）：樹上同時有「香菇」與「濕香菇」時，收下 `香菇→濕香菇` 會讓「香菇」節點永遠命不中。
3. 同一 alias 映到多個 L2 時整組丟棄，寧可 miss 也不導錯；撞人工層的 key 也丟棄。

修一條錯誤別名是往 `synonyms` 加一行，不重跑生成器；整批品質下滑才重跑，並看丟棄理由的分布而非逐列確認。

## 後果

- 接受表內有未經人確認的對應；結構上限是逃生艙加上卡片仍要買家點選。
- 不做 L3：L3 名字的命中已由比對上提到其 L2；L3 多為品種名，LLM 硬編空間大。不做花卉：佔 58% 成本，生鮮採購用不到；`-exclude-l1` 是旗標，加回只改參數。
- 漂移只靠離線 gate 與重跑；線上樹改名會讓該列靜默 miss（退化成沒有別名時的行為），新增的節點名被既有別名遮蔽才是真傷害，目前沒有自動偵測。
- 重新評估的條件：
  - 有了真實查詢紀錄 → 回到基準，以紀錄挖掘與營運策劃為主，LLM 生成降為補充。
  - 分類樹搬進單體、由擁有分類的模組持有 → 別名改掛在分類實體上（canonical entity ＋ surface forms），不再是 ai-service 的生成 Go 檔。
  - 錯誤別名的實際代價變高（例如分類命中直接定案取價、沒有逃生艙）→ 重新評估不逐列覆核。

## 修訂與現況

- spec 原定「別名唯一命中一個 L2 就跳過 GROUP 直接進 ITEM；唯一子類與父同名則自動確認取價；命中多個 L2 退回 GROUP」（`38d3e9cbf`）。2026-09-17 品項解析改為 retrieve-then-rank（`cropidentity`：召回 → RRF → decide → adaptive facet），舊階梯 `resolveCategoryFallback`／`enterItemStage` 刪除（`322deb19c`）；別名現在是 `understandQuery` 產生的查詢變體之一（`query_understanding.go`），階段跳轉規則不再適用，原決策段保留歷史。
- 每日漂移稽核（`category-alias-drift-audit.yml`，16:13 UTC，遮蔽既有節點名紅燈、死列與覆蓋缺口警告）於 2026-09-13 加入（`418eda54a`），隨公司時期的維運 CI 刪除（`87740743c`、`aae9e43b4`，2026-09-26）。
- 2026-09-17 人工層也加了 target 存在性 gate（`67865e283`，`TestManualSynonymTargets_ExistInTreeSnapshot`）。

## 結果

- 實作：`8c7164392`（樹快照）、`ab2ea85b9`（兩層查表）、`1cf9ddfde`（LLM 生成、渲染、離線 gate）、`7ce2f8a79`（樹處理與三道檢查）、`b299d880e`（首批 851 條，模型 `deepseek-flash`，samples=3／minAgree=2）。
- 驗證：`cd backend/services/ai-service && go test -count=1 ./internal/cache/ ./cmd/category-alias-gen/`（含 `TestGeneratedAliases_ConsistentWithTreeSnapshot`、`TestCanonicalCategoryName_ManualLayerWinsOverGenerated`、`TestValidateCandidates`、`TestConsensus`）。未驗證：別名品質本身（刻意不以測試釘住）。

來源：`backend/docs/superpowers/specs/2026-09-13-category-alias-generation-design.md`（私人封存）、`backend/docs/superpowers/plans/2026-09-13-category-alias-generation.md`（私人封存）。

## 修訂：pricebook 刪除（2026-10-28，ADR `2026-10-27-general-ecommerce-scope-cut` D1；計畫 `plans/2026-10-27-ecommerce-scope-cut.md` 步驟 6）

繼承與比較表裡的 pricebook（`reference/README.md`、crop 映射、`MAPPING_SEED_CSV_PATH`）已隨 `services/pricebook-service` 刪除。分類別名的離線生成只服務分類樹檢索，不受影響。
