import 'code_meta.dart';

/// 「這筆訂單的物流 UI 該走哪一套」的**唯一**判定點。
///
/// ## 為什麼需要它
///
/// `ShippingMethod` 是四元（`3001` 黑貓／`3002` 7-11 店到店／`3003` Uber 即時配送（已刪除的功能）／
/// `3005` 商家自行配送），而畫面層長期寫的是
/// `isTaqbin ? 黑貓 : 超商` 這種二元式 —— 於是 `3003`／`3005` 與任何後端
/// 新增的碼值都會被**靜默畫成 7-11 或黑貓**：型別正確、不拋例外、測試全綠，
/// 使用者看到的卻是另一家物流商的 logo、代碼欄位與操作按鈕。
///
/// ## 使用規約
///
/// 1. 畫面層一律先取得 [ShippingPresentation]，再用 `switch` 分派；
///    **禁止**在畫面層重新寫 `method == ShippingMethod.taqbin.value ? A : B`。
/// 2. [ShippingPresentation.unsupported] 必須 fail-safe：
///    不得畫成別家物流商、不得送出別家物流商的 payload、不得憑空消失無說明。
/// 3. 新增 provider 時只改本檔的 [shippingPresentationOf]。
///
/// ## 這個機制擋得住什麼
///
/// [shippingPresentationOf] 內是**沒有 `default` / `_` 的 switch expression**，
/// 所以 CodeMeta 新增第五個 `ShippingMethod` 成員時**編譯直接失敗**，
/// 逼人回到本檔決定它該走哪一套；擋不住的是「畫面層自己另外寫一次二元判斷」，
/// 那一種只能靠 review 與 `test/core/codemeta/shipping_presentation_test.dart`
/// 的變異驗證。
enum ShippingPresentation {
  /// 宅配到府（黑貓宅急便 `3001`）—— 託運單 / 呼叫取件 / PDF 那一套。
  homeDelivery,

  /// 超商店到店（7-11 店到店 `3002`）—— 取號 / 出貨便代碼 / ibon 那一套。
  cvsStorePickup,

  /// 本版 App 沒有對應 UI 的配送方式：`3003`／`3005`、空字串、
  /// 以及任何不在 [ShippingMethod] 值域內的碼值。
  unsupported,
}

/// 由 `shippingMethod` 原始值（CodeMeta value 或 code）判定該走哪一套物流 UI。
///
/// 大小寫不敏感的 code 查找沿用 `CodeMetaDisplay` 的既有慣例
/// （後端在不同端點上會回 value 或 code）。
ShippingPresentation shippingPresentationOf(String? rawShippingMethod) {
  final raw = rawShippingMethod?.trim() ?? '';
  if (raw.isEmpty || raw == 'null') return ShippingPresentation.unsupported;
  final method =
      ShippingMethod.fromValue(raw) ??
      ShippingMethod.fromCode(raw.toUpperCase());

  // 🔴 刻意不寫 `default` / `_`：CodeMeta 新增成員時這裡要編譯失敗。
  return switch (method) {
    ShippingMethod.taqbin => ShippingPresentation.homeDelivery,
    ShippingMethod.sevenElevenStorePickup =>
      ShippingPresentation.cvsStorePickup,
    // 3003 Uber 即時配送：功能已刪除（ADR 2026-10-27-general-ecommerce-scope-cut D4）。
    ShippingMethod.uberInstant => ShippingPresentation.unsupported,
    // 3005 商家自行配送：RFQ 訂單的站外業者自行出貨，無外部 provider API。
    ShippingMethod.merchantDelivery => ShippingPresentation.unsupported,
    null => ShippingPresentation.unsupported,
  };
}

extension ShippingPresentationX on ShippingPresentation {
  /// 本版 App 是否有對應這個配送方式的專屬 UI 與操作。
  bool get isSupported => this != ShippingPresentation.unsupported;

  /// 這個配送方式的收件端是**買家的收件地址**（而不是超商門市）。
  ///
  /// 決定 `/app/checkout/preview`／`/confirm` 要不要帶 `receiverZipCode` ＋
  /// `recipientName`／`recipientPhone`／`shippingAddress`，還是帶
  /// `storeId`／`storeName`。
  ///
  /// 🔴 有它是為了避免「哪些方式要地址」這條規則在
  /// cart_screen／preview／confirm **各寫一次**——那三份拷貝一旦分岔，
  /// 症狀是某一支送出一筆沒有收件人的訂單，而編譯與測試都不會有意見。
  /// 判準是「收件端是什麼」，不是「是不是黑貓」。
  ///
  /// 🔴 同樣刻意沒有 `default` / `_`。
  bool get usesRecipientAddress => switch (this) {
    ShippingPresentation.homeDelivery => true,
    ShippingPresentation.cvsStorePickup => false,
    ShippingPresentation.unsupported => false,
  };
}
