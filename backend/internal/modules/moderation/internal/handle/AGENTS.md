# moderation/internal/handle

案件的處理：管理員對進行中的案件下決定並結案，處置與結案同一個 transaction。

## 不變量

- 案件結案是終態，不重開、不改判；結案 UPDATE 以 `(id, version, closed_at IS NULL)` 轉移，0 列回 `VersionConflict` 並回滾處置（`TestDecideRollsBackTheActionWhenCloseLosesTheRace`）；新檢舉掛進既有案件讓 `version` 加一，管理員讀到之後才掛進來的檢舉讓結案衝突（`TestAReportAttachedAfterTheAdminReadMakesTheCloseConflict`）。決定只存在案件，檢舉的結果由案件推導，不存逐筆 outcome。
- 內容已被刪除時，警告與停權用快照裡凍結的作者；隱藏視為成功、不呼叫擁有者、不寫隱藏紀錄（`TestHideOfDeletedBuyerContentSucceedsWithoutCallingSocial`、評價與聊天訊息的 `TestHideOfAPurgedReviewSucceedsWithoutAHideRecord`），內容下架用快照凍結的店家驗證並建立措施（`TestTakedownOfDeletedStoreContentVerifiesAgainstTheSnapshotAndStillCreatesTheMeasure`、`TestWarnAndBanUseTheFrozenAuthorWhenTheContentIsGone`）。
- 評價與聊天訊息：作者一律取快照凍結的作者（不回頭問 review、im）；HIDE 評價走 `review.Hide`、聊天訊息走 `im.Hide`（`TestChatHideAndReverseThroughIM`）；擁有者回 NotFound 視為成功；隱藏與還原不取擁有者的守門（`TestHideWorksWhileTheAuthorIsDeactivated`）；店家寄的聊天訊息 HIDE 與買家 WARN 回 FailedPrecondition（店家寄的聊天訊息只能下架）。
- 店家寄的聊天訊息（CHAT_MESSAGE）與店家的評價回覆（REVIEW_REPLY，`content_id` 是評價 ID）只能 TAKEDOWN：措施快照依凍結的種類組（`measureSnapshot`，連結全空），`Decide` 先呼叫 seller 建立措施、im／review 的訂閱者鎖內容列並核對歸屬，案件列最後寫（鎖序 seller → im／review → moderation，`TestTakedownRacesWithEraseAndSendWithoutDeadlock`、`TestReplyTakedownRacesWithReviewHideWithoutDeadlock`）；回覆的 HIDE、WARN、BAN 是 InvalidInput（`AllowedOn`）。
- 撤銷隱藏（`Reverse`）：案件維持結案、決定不改，只對已結案的 HIDE 案件、只一次；同 transaction 先呼叫擁有內容的模組 `Restore`（review、im、social，帶 `SourceRefPrefix`+案件 id 只還原本案造成的隱藏，別案的隱藏回 FailedPrecondition；擁有者回 NotFound 視為成功並照常記撤銷）、最後寫 `cases.reversed_*` 與 `adminaudit.Record`（`TestReviewReportHideAndReverse`、`TestReverseHideRequiresAClosedHideCase`）。
- 後台決定留稽核：`DecideCase` 在結案後同 transaction 呼叫 `adminaudit.Record`（`Actor.Action` 必填，是後台 RPC 的 procedure 名稱，缺漏是接線錯誤、回未分類的 internal error），版本衝突回滾時紀錄跟著消失（`TestDecideWritesTheAdminAuditRecordInTheSameTransaction`）。
- 處置與結案在同一個 transaction；案件列最後才寫：擁有處分的模組（social、identity、seller）的列都排在 moderation 之前，先呼叫它們、後寫自己的列（`internal/platform/lockorder`）；每次處理只走一條處置路徑。
- 店家寫的內容只能走內容下架，店家取自案件的內容快照，內容還在時 social 現在的作者與歸屬必須是同一家店（`CheckTakedownStore`）；建立的措施 ID 同 transaction 寫進 `cases.measure_id`；內容已有生效中的下架措施時不建新措施，案件綁定既有措施並結案、不另產生認定；買家不能隱藏或警告店家，店家不能被停權。
- 結案事件（D24）：`Decide` 在結案 UPDATE 之後、稽核之前同 transaction 發 `CaseClosedEvent`（案件 ID、對象種類、結果類別、政策分類；只有違規有政策分類，不帶處置種類與檢舉人清單）；版本衝突回滾，事件不發。撤銷隱藏由擁有者模組發 `ContentRestored` 等，moderation 不發。守門：`TestCaseClosedEventFailureRollsBackTheDecisionAndTheHide`、`TestStaleDecidePublishesNoCaseClosed`。
