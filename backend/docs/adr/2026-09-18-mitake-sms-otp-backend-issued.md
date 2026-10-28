# 手機 OTP 改由後端經三竹發送

| 項目 | 值 |
|---|---|
| 狀態 | accepted；D1 的「存 Redis」與 D6 superseded by `2026-09-27-identity-opaque-sessions.md`（OTP 改存 Postgres、在寫入帳號的 transaction 內消耗；簡訊由 River job 以 `clientid` 冪等地重試）。遷移計畫 A2 落地前，現行程式仍依本檔運作 |
| 日期 | 2026-07-21 |
| 影響範圍 | `services/account-service`、`pkg/sms`、`serverless/egress-proxy`、buyer／seller App |
| 對應規格 | `docs/詳細設計書/51-簡訊發送與手機 OTP（Mitake SMS & Phone OTP）詳細設計書.md` |

> 本檔保存 `WHY THIS DESIGN` / `WHY NOT THE ALTERNATIVES`。
> 端點簽章、Redis key 設計、防濫用計數順序、錯誤碼映射一律在 51 號詳細設計書，本檔不複製。

---

## 脈絡

原本的手機 OTP 由 App 端的 Firebase Phone Auth SDK 直接觸發簡訊，後端只驗 `firebasePhoneIdToken`。
三個約束共同逼出改版：

1. **簡訊通路不在我方控制之下** —— 發不發得出去、每則多少錢、被誰刷了，後端全部看不到。
2. **三竹以帳號層鎖定來源 IP**（白名單），非白名單來源一律回 statuscode `k`；
   而 Cloud Run 預設出向 IP 浮動。
3. **三竹無公開 sandbox**，任何測試都是正式帳號實發、真實扣點。

---

## 決策

### D1 OTP 產驗移到後端

**不採用**：續用 Firebase Phone Auth。

**選擇**：account-service 產碼、存 Redis、經三竹發送、自行驗碼。
去除 Firebase 簡訊依賴、統一國內簡訊通路，防濫用與點數消耗都變成我方可量測、可設限的東西。

### D2 prod／dev 共用單一 proxy

**不採用**：各環境獨立 proxy。

三竹白名單 IP 只申報一次、減少對第三方的報備面（比照 27 號 tgos-proxy 的既有前例）。
代價是 dev 的測試流量與 prod 共用同一組三竹帳號額度。

### D3 proxy 不持三竹帳密

**不採用**：proxy 內嵌帳密。

**選擇**：帳密由呼叫端（account-service）帶入請求。
proxy 保持純轉發，持密點收斂成一個（account-service 的 secret），
proxy 的 access log 也因此不會落到帳密。

### D4 不接 DR（送達回報）callback

**不採用**：接三竹 DR callback。

OTP 輸碼成功本身就是「送達且使用者讀到了」的最強證明，比 DR（只證送達）更強。
接 DR 要另開 inbound 路由並把該路由的 IP 報給三竹，成本不划算。
未來 notification-service 接批量行銷簡訊時可重新評估。

### D5 硬切，不保留 Firebase／三竹雙軌

**不採用**：兩條路徑並存一段 grace period。

雙軌等於同時維護兩套驗證契約，而前端要判斷「這個版本走哪一條」。
改成硬切 ＋ App 強制更新放量（強更機制見 `docs/modules/content.md` 與 `packages/hawkr_app_update/docs/spec.md`）。

### D6 發送失敗不自動重試

**不採用**：重試 N 次。

自動重試會產生重複簡訊與重複扣點，而使用者看到的是「我收到兩封」。
發送失敗即釋放 cooldown，使用者可以立刻自己按重送。

---

## 後果

- 三竹是單一上游且無 sandbox ⇒ 發送失敗的處置全部落在我方（錯誤碼映射與 runbook 在 51 號 §8.3／§12.4）。
- 固定出口 IP 成為硬依賴：Cloud NAT 的 IP 一旦漂移，簡訊全數回 `k`。
- 單一 proxy ⇒ 該 Cloud Run region 級故障時 prod 與 dev 同時發不出簡訊。
- 什麼條件下該重新評估 D4：notification-service 開始發批量行銷簡訊、需要獨立的送達率統計時。

## 修訂（2026-10-28）

設計書 27（地址搜尋與驗證）已刪除：TGOS 地址查詢已從單體移除（遷移計畫 A3 決定 9a），`tgos-proxy` 的前例見 git 歷史。本文中「27 號」指向已刪除的文件。

設計書 51（簡訊發送與手機 OTP）已刪除：手機 OTP 與三竹簡訊已隨 Clerk 移除。本文中「設計書 51」、「51 號」與它的章節編號指向已刪除的文件，內容見 git 歷史。
