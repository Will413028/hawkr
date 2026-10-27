/// 帶 etag 的寫入撞到 `VERSION_CONFLICT` 時給使用者的文字，句型和 website 的 `lib/hooks/useVersionConflict.ts` 相同：
/// 「{subject}已在其他地方更新」。
String versionConflictHeadline(String subject) => '$subject已在其他地方更新';

/// 「已載入最新版本，你輸入的內容仍保留，請確認後再{action}。」
String versionConflictGuidance([String action = '儲存']) =>
    '已載入最新版本，你輸入的內容仍保留，請確認後再$action。';

/// 一句話的衝突提示（`versionConflictNotice('店家資料', '送出')`）。
String versionConflictNotice(String subject, [String action = '儲存']) =>
    '${versionConflictHeadline(subject)}，${versionConflictGuidance(action)}';

/// 換圖遇到衝突時的說明（圖片要重新上傳）。
const versionConflictImageGuidance = '已載入最新版本，請重新上傳圖片。';

/// 衝突之後重讀最新版本失敗。
const versionReloadFailedNotice = '無法載入最新版本，請稍後再試。';

/// 讀不到店家狀態（暫時失敗），不能當成沒有店家。
const storeStateUnavailableNotice = '目前無法確認商店狀態，請稍後再試一次。';
