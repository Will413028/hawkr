# App 版本門檻（客戶端）現行規格

本文是買家 App、賣家 App 的版本門檻（強制更新）行為的唯一來源：共用的檢查邏輯（本套件）與兩支 App 的接線。後端 `CheckAppVersion` 與門檻設定的規格在 `backend/docs/modules/content.md`。

## 1. 契約

- 呼叫 `ContentPublicService.CheckAppVersion(app_type, client_version)`，只讀 `update_required`、`minimum_version`、`store_url`、`notice_title`、`notice_body`。版本比較只在後端，App 不實作任何比較邏輯。
- 這個 service 是公開的，不帶身分：未登入、guest、token 過期、停權的賣家都可以呼叫，也都會被攔截。
- `client_version` 是 bare semver（`1.0.3`），不含 build number。顯示用的版本字串（`1.0.3(7)`）不可送出：後端比對失敗會放行，強更靜默永不觸發。App 取得兩者的函式分開：`fetchRawAppVersion` 回 bare semver，`fetchAppVersion` 回顯示字串；兩支 App 的 `lib/shared/utils/app_version_utils.dart` 逐位元組相同。

## 2. 檢查規則（`VersionGate`）

- 只在 iOS 檢查（App 只發 iOS）。讀不到自己的版本（空字串）也不檢查。
- 整個呼叫最多 `versionGateTimeout`（3 秒），不共用 App 預設的逾時；不重試。
- 任何失敗（逾時、連線失敗、回應不對、被限流）一律放行（fail-open），並保留上一次的結果。判準是網路不可用時使用者能不能自救：不能，所以放行。fail-open 不是 gate 失效，不得改成 fail-closed。
- 冷啟動一定檢查（`checkOnStartup`），不看節流。
- 回到前景（`checkOnResume`）時，距上次成功的檢查未滿 `versionGateRecheckInterval`（10 分鐘）就跳過；失敗不更新這個錨點，下次回到前景立刻重試。
- 放行的初始結果是 `versionGateAllowed`；每個結果都 freeze 成唯讀，訊息有值相等，結果沒變時不會通知 router 重判。

## 3. App 的接線

兩支 App 各自提供 keepAlive 的 provider（買家 `ForceUpdate`、賣家 `ForceUpdateGate`），把 `VersionGate` 的回傳值放進 state；畫面與路由攔截留在各自的 App。

- 冷啟動：bootstrap 中 `await` 檢查，完成才進路由，否則會先進首頁再彈更新畫面。位置早於 Firebase 與推播初始化，通知導頁在攔截時還沒啟動，不需另外抑制。
- 例外在 bootstrap 內自己 try/catch 吃掉：`BrandStartupGate` 的 bootstrap 失敗時不解除啟動畫面，例外冒上去 App 會永遠停在啟動畫面。
- 前景恢復：`AppLifecycleState.resumed` 時呼叫 `checkOnResume`（買家在 `main.dart`，賣家在 `app.dart` 並沿用 `runtimeSideEffectsEnabled` 的 guard）。
- 攔截只有一條路徑：GoRouter 的頂層 redirect 讀 provider 的 `updateRequired`，為真就導向 `/force-update`；冷啟動與前景恢復共用。
- redirect 判斷是 top-level 純函式（買家 `resolveBuyerRedirect`、賣家 `resolveAuthRedirect`），參數由呼叫端顯式傳入，不讀全域狀態或 Riverpod。強更判斷排在所有其他判斷之前：未登入、onboarding、停權的使用者一樣被導到更新頁；已在 `/force-update` 時回傳 null，不自我循環。賣家的 `needsForceUpdate` 參數預設 `false`，生產呼叫點必須傳入。
- 買家有 App Store 送審截圖模式（`isScreenshotMode`），此模式豁免強更，否則審核的 guest 流程會被更新畫面攔死。賣家目前沒有截圖模式；日後加入時必須同時把它接進 `resolveAuthRedirect` 並豁免強更。
- 賣家的 `/force-update` 路由帶 `parentNavigatorKey: rootNavigatorKey`，否則被 ShellRoute 的 bottom nav 包住。`/force-update` 不在賣家的停權白名單與能力表內。
- 下列不變式由兩支 App 的 `redirect_logic_test.dart` 釘住：強更早於登入判斷；截圖模式不被強更攔截（買家）；已在更新頁時回 null；`needsForceUpdate=false` 時既有分支行為不變；買家 `/force-update` 訪客可達（`route_access_policy_test.dart`）。

## 4. 更新畫面（`ForceUpdateScreen`）

- 全螢幕、不可跳過：`PopScope(canPop: false)`，沒有返回鍵，唯一出路是更新後重啟。
- 標題與內文用後端回的 `notice_title`、`notice_body`（去除前後空白）；為空時分別用預設「請更新至最新版本」與「此版本已停止支援，請前往商店更新後繼續使用。」。
- `minimum_version` 有值時顯示「最低可用版本」卡片，為空時整張隱藏。
- `store_url` 為空字串時隱藏「前往更新」按鈕，改顯示文字指引，不導向任何猜測的網址。有值時按鈕以外部應用程式開啟；開啟失敗時以 SnackBar 顯示同一段文字指引。

## 5. 版本 header

兩支 App 對所有打後端的請求（Connect、舊 REST、SSE、WebSocket 握手）帶 `X-App-Version`（bare semver）與 `X-App-Type`（`BUYER`／`SELLER`），不帶平台。

- 純可觀測性用途，供 access log 統計各版本的請求佔比；伺服器不因缺少它們拒絕請求，也不得拿它們當授權或攔截依據。
- 版本在 bootstrap 開頭讀一次並快取（`PackageInfo` 是非同步，而 client 建構點只能同步讀）。尚未讀到時，買家送空欄位，賣家的 Connect 不送版本 header；請求照送，不得讓 App 起不來。
- 不打後端的請求（直接上傳到物件儲存、WebView 導覽、圖片網址直取）不帶；長連線只在握手時帶，所以請求佔比不含長連線後的訊息。
- 買家入口在 `core/networking/app_version_headers.dart` 的 `AppVersionHeaders`；賣家在同路徑的 `appConnectHeaders()`（Connect）與 `appVersionHeaders()`（舊 REST、SSE、WebSocket，另帶 `X-Active-Role`）。

## 6. 限制

- 沒有內建 gate 的舊版 App 無法被強制更新：它不檢查版本，也沒有更新畫面。
- 「沒有 `X-App-Version`」不等於舊 App：網頁的伺服器端取數與服務間呼叫同樣沒有這個 header，不得據此在伺服端硬擋。
- 因為 gate 在 client 且 fail-open，後端不可用期間舊版 App 仍可運作；機制能保證的是各版本佔比可觀測，以及絕大多數情況可強制，不是保證。
