import 'package:hawkr_buyer/core/codemeta/code_meta.dart';
import 'package:flutter/foundation.dart';

/// 買家的物流追蹤單點狀態（`GET /app/orders/{order_no}/logistics`）。
///
/// 🔴 **單點狀態，不是時間軸**：後端即時向 provider 同步當前狀態後回傳，
/// PayUni 的物流查詢 API 只提供「當前狀態 ＋ 時間」，沒有多筆軌跡歷史
/// （gateway `OrderLogisticsResp` 的 doc comment 逐字寫了這一條）。
/// 所以這個畫面**不能**畫成一條有歷史節點的 timeline——那會讓使用者以為
/// 中間那些節點是真的發生過並被記錄下來的。
///
/// 🔴 **最小揭露**：後端刻意不回 `recipientPhone`／`recipientAddress`／
/// `labelUrl`／寄件人資訊。這裡也不要為了「畫面比較完整」去別處湊那些欄位。
@immutable
class LogisticsTrack {
  const LogisticsTrack({
    required this.orderNo,
    required this.shipTradeNo,
    required this.shippingMethod,
    required this.status,
    required this.statusLabel,
    required this.providerMessage,
    required this.statusTime,
    required this.storeName,
    required this.cvsStoreCode,
    required this.recipientName,
    required this.createdAt,
  });

  /// 欄名取自 `gateway/internal/handler/shipping.go` 的 `OrderLogisticsResp`。
  ///
  /// ⚠️ **`providerStatus` 刻意不收**：那是 provider 的原始碼（例如 PayUni 的
  /// 兩碼狀態），對買家沒有意義，而把它畫出來會讓人以為那是可以拿去查詢的單號。
  /// 維運要看原始碼值走 log／ADM，不走買家畫面。
  factory LogisticsTrack.fromWire(String orderNo, Map<String, dynamic> data) {
    String text(String key) => data[key]?.toString().trim() ?? '';
    return LogisticsTrack(
      orderNo: text('orderNo').isEmpty ? orderNo : text('orderNo'),
      shipTradeNo: text('shipTradeNo'),
      shippingMethod: text('shippingMethod'),
      status: text('status'),
      statusLabel: text('statusLabel'),
      providerMessage: text('providerMessage'),
      statusTime: text('statusTime'),
      storeName: text('storeName'),
      cvsStoreCode: text('cvsStoreCode'),
      recipientName: text('recipientName'),
      createdAt: text('createdAt'),
    );
  }

  final String orderNo;

  /// 託運單號（PayUni 物流序號）。尚未交運時為空。
  final String shipTradeNo;

  /// `ShippingMethod` 的 **value**（`'3001'`～`'3005'`）。
  ///
  /// ⚠️ gateway DTO 的註解寫的是 `CONVENIENCE_STORE / HOME_DELIVERY`
  /// ——那串與 CodeMeta 的 code 名對不上，實際落地的是
  /// `shipments.shipping_method`，而該欄的 CHECK 是 `IN ('3001'…'3005')`
  /// （shipping-service `V11__extend_shipments_for_fulfillment_platform.sql`）。
  /// 本欄一律交給 `shippingPresentationOf` 判定，它 value／code 兩種都吃。
  final String shippingMethod;

  /// 內部物流狀態碼（`ShippingStatus` 的 value，數字字串）。
  final String status;

  /// 後端給的狀態文案（`ShippingStatus.label`，CodeMeta 為 SSOT）。
  ///
  /// 🔴 **優先用它而不是前端自己查表**：後端是這個值的權威，它先變、App 後跟。
  /// 前端的 CodeMeta 拷貝是**回退**（見 [LogisticsTrackX.normalizedStatusLabel]），
  /// 不是第一來源。
  final String statusLabel;

  /// provider 的狀態說明（人看得懂的那一句；可能為空）。
  final String providerMessage;

  /// 最新狀態更新時間（ISO 8601）。
  final String statusTime;

  /// 超商門市名稱（超商配送才有）。
  final String storeName;

  /// 超商門市代號（超商配送才有）。
  final String cvsStoreCode;

  /// 收件人姓名（買家本人）。
  final String recipientName;

  /// 物流單建立時間（ISO 8601）。
  final String createdAt;
}

extension LogisticsTrackX on LogisticsTrack {
  /// 正規化後的狀態文案，三層回退：
  ///
  /// 1. 後端的 [LogisticsTrack.statusLabel]（權威）
  /// 2. App 內的 CodeMeta 拷貝（後端漏填時）
  /// 3. 🔴 **原樣顯示碼值**——不是「配送中」這種猜測。
  ///    App 的 CodeMeta 是拷貝，後端新增狀態時它一定比較舊；
  ///    那時猜一個最接近的文案，等於對使用者陳述一件沒有發生的事。
  ///    顯示 `狀態 931` 難看但誠實，而且客服搜得到。
  String get normalizedStatusLabel {
    if (statusLabel.isNotEmpty) return statusLabel;
    final known = ShippingStatus.fromValue(status);
    if (known != null) return known.label;
    return status.isEmpty ? '狀態同步中' : '狀態 $status';
  }

  /// 後端給的狀態碼是否在 App 的 CodeMeta 值域內。
  ///
  /// 用來決定要不要顯示 [LogisticsTrack.providerMessage]：碼值不認得時，
  /// provider 的那一句原文是使用者唯一拿得到的線索。
  bool get isKnownStatus => ShippingStatus.fromValue(status) != null;
}
