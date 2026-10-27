import '../../../core/codemeta/code_meta.dart';

/// 發布與下架按鈕的方向。一顆按鈕，方向由目前的發布狀態決定：沒有「改回未發布」這個目標
/// （端點不接受），所以型別上只有這兩個值。
enum PostPublishTarget { publish, unpublish }

/// 由發布狀態碼推出「按一下」要送的目標；缺值（上游沒給）回 null，呼叫端什麼都不畫：
/// 方向不明的按鈕比沒有按鈕更糟，下架是回不去的。
PostPublishTarget? postPublishTarget(String? publishStatus) {
  if (publishStatus == PostPublishStatus.draft.value ||
      publishStatus == PostPublishStatus.unpublished.value) {
    return PostPublishTarget.publish;
  }
  if (publishStatus == PostPublishStatus.published.value) {
    return PostPublishTarget.unpublish;
  }
  return null;
}

const String kPostPublishLabel = '發布';
const String kPostUnpublishLabel = '下架';
const String kPostPublishPendingLabel = '發布中…';
const String kPostUnpublishPendingLabel = '下架中…';

const String kPostPublishConfirm = '發布這則貼文？發布後買家會在動態牆看到它。';
const String kPostUnpublishConfirm =
    '下架這則貼文？內容會保留，但買家不再看得到。下架後可以再次發布，但無法改回「未發布」。';

/// 有排程的貼文按發布時，確認視窗多加的一句。
const String kPostScheduledPublishWarning = '這則貼文已排程，現在發布會取消排程。';

/// 沒有照片時的提示標題（建立貼文與發布草稿共用）。
const String kPostImageRequiredTitle = '請先補上照片';

/// 建立貼文沒有照片時的說明。
const String kPostCreateImageRequiredHint = '每則貼文至少需要 1 張照片才能發布。';

/// 發布沒有照片的草稿時的說明。
const String kPostPublishImageRequiredHint = '請先編輯貼文並加入至少 1 張照片，再回來發布。';

/// 平台下架的標籤（貼文徽章與縮圖角標共用）。
const String kPostPlatformHiddenLabel = '平台下架';

/// 被平台下架的貼文，發布按鈕停用的說明。
const String kPostPlatformHiddenPublishReason = '已被平台下架，發布後買家仍看不到';
