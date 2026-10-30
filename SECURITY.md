# 安全政策

## 回報漏洞

請不要在公開的 issue、PR 或討論區揭露安全問題。

使用 GitHub 的私下回報：到本 repo 的 **Security** 分頁，選 **Report a vulnerability**，填寫下列資訊：

- 受影響的位置（子專案、檔案或 API 路徑）與版本或 commit
- 重現步驟或概念驗證
- 預期的影響（資料外洩、權限繞過、服務中斷等）

不要附上真實的使用者資料或第三方的機敏資料。

## 處理流程

這是個人維護的作品集專案，沒有 SLA，但會這樣處理：

1. 收到回報後盡快確認，並在回報串內回覆是否成立。
2. 依嚴重度排定修復期限：critical 7 天、high 30 天、medium 90 天、low 在下一次相關更新時處理。
3. 修復後在 GitHub Security Advisory 公開說明，並視回報者意願致謝。

## 範圍

- 在範圍內：本 repo 的程式碼與設定（`backend/`、`website/`、`admin/`、`buyer-app/`、`seller-app/`、`api/`、`packages/`、`.github/`）。
- 不在範圍內：第三方服務本身（Clerk、Cloudflare、雲端供應商）的漏洞，請直接回報給該服務；以及需要實體取得設備或社交工程的攻擊。

## 持續的自動檢查

CI 會定期執行依賴漏洞掃描（govulncheck、OSV-Scanner）、機敏資料掃描（gitleaks）與 workflow 安全檢查（zizmor），設定見 `.github/workflows/supply-chain.yml`。
