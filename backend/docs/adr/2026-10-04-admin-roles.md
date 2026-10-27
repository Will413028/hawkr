# 後台角色：單體保存角色，逐方法宣告權限，Access 只管進門

| 項目 | 值 |
|---|---|
| 狀態 | accepted（Will 2026-10-04） |
| 日期 | 2026-10-04 |
| 影響範圍 | `internal/modules/identity`（角色與事件表、transaction 內事件 `AdminRoleRevoked`）、`internal/httpapi`（admin 的宣告表與 interceptor、`/api/files` 的宣告）、`internal/legacy/gatewayproxy`（舊後台 REST 的轉發閘）、`internal/modules/delegation`（撤角色時撤代行、開始代行時檢查權限）、`cmd/hawkr`（`admin-role`） |
| 計畫 | `docs/plans/2026-09-29-a3-buyer-seller.md` 步驟 8f-2 |

## 脈絡

後台沒有任何授權：通過 Cloudflare Access 的人第一次登入就建立管理員列（`identity.admins`），之後能呼叫每一支 `*AdminService`（目前 4 個 service、39 支方法）、讀 `/api/files/admin/…` 的 KYC 證件端點，並經單體轉發到舊 gateway 的全部後台 REST。單體唯一的檢查是「principal 是管理員」。要停用一個人只能把他移出 Access 白名單。8g 要把治理（違規、FDA 通報、申訴裁決）搬進單體，這些操作比現有的更需要「誰能做」的答案，所以角色要先於 8g 存在。

identity ADR 的〈修訂：後台改用 Cloudflare Access〉已定：誰是管理員由 Access 決定；角色由單體的管理員列決定，不放進 Access。本 ADR 決定角色的資料、權限的定義與檢查點、授予的方式、舊 REST 的處理。

管理員與角色是兩件事：管理員是身分（一個通過 Access 的人，對應一列 `identity.admins`）；角色是一組權限，授予給管理員。一個管理員可以有零到多個角色。

約束：
- 外部：Cloudflare Access 的授權資料在 JWT 裡，效期內改不掉；設定在 Zero Trust 後台、不在 repo；本機沒有 Access。
- 外部：現在只有一位管理員（Will），職務分工不存在；任何非 OWNER 角色的權限內容現在都無從定義。
- 繼承：舊 gateway 的後台 REST（約 170 條路由）在階段 C 隨模組搬遷逐步刪除；在那之前它仍經單體轉發。現在仍成立：模組搬遷的排程由 monolith 計畫決定。
- 繼承：代行資格（`delegation.admin_eligibilities`）獨立於角色，A10 搬代行時併入。現在仍成立：代行仍在舊 gateway 的路由上。

## 考慮過的方案

**角色存在哪裡**

| 方案 | 代價 | 判定 |
|---|---|---|
| 基準：IdP 管身分與 MFA，應用程式自己保存角色指派並在每個請求檢查（GitHub organization roles、Stripe Dashboard team roles、Retool／Django admin 的 groups；Google IAP 文件也建議 IAP 只做進入控制、應用內授權由應用負責） | 單體多兩張表、一支 CLI | ✓ `identity.admin_roles`（現況）＋ append-only 的 `identity.admin_role_events`，同一個 transaction 寫入 |
| 角色放進 Access 的群組（或依路徑分 application），單體讀 JWT 的 groups claim | 每加一個權限都要改 Zero Trust 的設定；JWT 在效期內角色改不掉；本機沒有 Access | ✗ |
| 只留事件表，現況由事件推導 | 每個請求都要折疊事件；沒有比較安全 | ✗ |

**權限的定義與檢查點**

| 方案 | 代價 | 判定 |
|---|---|---|
| 基準：權限是固定的細粒度字串、角色是權限的集合、每個操作宣告它要的權限（GCP IAM 的 `service.resource.verb` 與 predefined roles、Kubernetes RBAC 的 verb×resource、AWS IAM action） | 每支方法都要宣告一次 | ✓ 權限 `<模組>.<資源>.<動詞>`；每支 admin 方法與每條 `/api/files/admin/…` 端點各宣告一個權限或明列的豁免；啟動時雙向對照 service descriptor 與檔案路由表，缺漏或多餘就啟動失敗；沒有宣告的 procedure 一律拒絕（default deny） |
| 只有 OWNER，不建權限目錄，檢查「有沒有 OWNER」 | 第二個角色出現時要回頭逐支補宣告，那時 8g 等模組已經寫完 | ✗ 計畫已定：目錄與宣告現在就做 |
| 在 handler 內各自檢查 | 漏寫的方法就是全開；沒有啟動時的對照 | ✗ |

角色與權限的對應寫在程式裡（GCP predefined roles、GitLab built-in roles 的做法；K8s ClusterRole、Django groups 放 DB，適合自訂角色），DB 只存「誰有哪個角色」。OWNER 等於目錄裡的全部權限，由程式計算，不逐條列。出現自訂角色的需求時再評估放 DB。

豁免只有「我是誰」與登出：`GetSession`、`SignOut`。這兩個在 GitHub、GCP 對任何已登入的身分都允許，不屬於任何權限。8f-2 不改後台 UI：零角色的管理員在每一頁看到既有的無權操作錯誤。

**授予與撤銷**

| 方案 | 代價 | 判定 |
|---|---|---|
| 基準：後台 UI 由有管理權限的人指派角色，另有 break-glass 程序（GitHub org owner、Google Workspace super admin 的復原流程） | 要一組新 RPC、UI 與「誰能指派誰」的規則 | ✗ 現在只有一位管理員，UI 沒有使用者；改用 CLI，觸發條件見後果 |
| CLI `admin-role grant|revoke`：直接寫 DB，任何時候可用，必填理由 | 能用 CLI 的人等於能部署、能碰 DB 的人；CLI 沒有經驗證的操作者身分 | ✓ |

事件的操作者：CloudTrail、Kubernetes audit 記錄經驗證的 principal；CLI 沒有，所以事件記來源 `cli`，另記 DB 的 `session_user` 與作業系統使用者，兩者未經驗證、只供參考，不提供自填操作者的參數。經驗證的操作者在授予搬進後台 UI 時才有。

最後一個 OWNER：GitHub、Google Workspace 禁止移除最後一位 owner，因為 UI 是唯一的路，移掉就鎖死。這裡 CLI 本身就是 break-glass，鎖死也救得回來，所以允許，CLI 印出警告。

授予的對象是已存在的管理員列：Access 白名單裡的人登入一次（得到零角色、被拒）後才能授予。GitHub、Retool 以 email 邀請、首次登入綁定；這裡不這樣做，因為 `access_subject` 只能從 Access 的 JWT 得到，以 email 綁定要多處理 email 換人使用的情況。管理員超過幾位、先登入一次變得不便時再評估邀請。

撤掉 OWNER 時，identity 在同一個 transaction 發出 `AdminRoleRevoked`，delegation 在同一個 transaction 撤銷這位管理員進行中的代行，不等代行自己到期（identity 不依賴 delegation，沿用登出、移除管理員的事件做法）。開始代行也要在 delegation 自己的 transaction 內讀角色並檢查權限，讀取與撤銷互相鎖住；只在入口檢查一次的話，入口通過之後、代行寫入之前撤掉角色，會留下零角色的人的代行。

**舊 gateway 的後台 REST**

| 方案 | 代價 | 判定 |
|---|---|---|
| 基準：在進入點對每條路由宣告權限 | 約 170 列的對照表，每一列都會隨模組搬進單體而刪掉 | ✗ 白工 |
| 單體轉發前檢查一個目錄裡的權限（只有 OWNER 有）：principal 是管理員而沒有這個權限就回 403，不轉發 | 舊後台功能要等模組搬進單體才能交給非 OWNER | ✓ 同一套權限檢查，不寫死角色 |
| 照舊全轉發 | 零角色的管理員仍能操作舊後台 | ✗ |

代行工作區不受影響：website 的代行 BFF 呼叫上游只帶 `X-HK-Operation-Session`、不帶管理員的 Bearer（`website/app/(delegated)/adm/api/_upstream.ts`），到單體時沒有管理員 principal，閘門不適用。

## 決策

角色存在單體的 `identity.admin_roles`，每次變更同一個 transaction 寫入 append-only 的 `identity.admin_role_events`；Cloudflare Access 只管進門與 MFA。每支 admin 方法與 `/api/files/admin/…` 端點逐一宣告 `<模組>.<資源>.<動詞>` 權限，啟動時對照、default deny。角色先只有 OWNER（全部權限），零角色等於停用。授予與撤銷只有 CLI `admin-role grant|revoke`，必填理由，撤 OWNER 時撤銷進行中的代行。舊 gateway 的請求由目錄裡一個只有 OWNER 有的權限把關，不建逐路由表。

## 後果

- 每個請求在認證管理員時一併讀角色，撤銷立即生效，不快取。
- 新增 admin 方法或 `/api/files/admin/…` 端點時，沒寫宣告就啟動失敗。
- 現在新加入的管理員要有人跑 CLI 才能用；上線前 Will 要先授予自己 OWNER。
- 本機與測試：seed 的 dev 管理員與 integration、characterize 的 fixture 授予 OWNER，並各有零角色被拒的負向測試。
- 宣告框架：「雙向對照 descriptor、建構時失敗」抽成 seller 與 admin 共用的 helper；需求型別（seller 的能力與額度、admin 的權限）各自保留。
- 重新評估的條件：
  - 第二位管理員加入且職務確定時：定義第一個非 OWNER 角色；`GetSession` 回 permissions、後台 UI 依權限隱藏（UI 改動先問 Will）；評估把授予搬進後台 UI。
  - 有人需要某個舊後台功能但不該有 OWNER：優先把那個模組搬進單體，不建舊路由的權限表。
  - A10 搬代行時：代行資格併入角色，刪除 `delegation.admin_eligibilities`。
