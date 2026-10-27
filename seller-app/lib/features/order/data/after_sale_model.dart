// ignore_for_file: invalid_annotation_target

import 'package:freezed_annotation/freezed_annotation.dart';

import '../../../core/codemeta/code_meta.dart';
import '../../../core/codemeta/code_meta_display.dart';

part 'after_sale_model.freezed.dart';
part 'after_sale_model.g.dart';

Object? _readCaseValue(Map<dynamic, dynamic> json, String key) =>
    json[key] ?? json[_camelToSnake(key)];

String _camelToSnake(String input) {
  final buffer = StringBuffer();
  for (var i = 0; i < input.length; i++) {
    final char = input[i];
    final lower = char.toLowerCase();
    final isUpper = char != lower && char.toUpperCase() == char;
    if (i > 0 && isUpper) {
      buffer.write('_');
    }
    buffer.write(lower);
  }
  return buffer.toString();
}

/// 售後案件
@freezed
abstract class AfterSaleCase with _$AfterSaleCase {
  const factory AfterSaleCase({
    @Default('') String caseNo,
    @Default('') String orderNo,
    @Default('') String buyerId,
    @Default('') String storeId,
    @Default('') String buyerName,

    /// 買家頭像顯示 URL（Gateway 組）。契約欄名 `avatarUrl`，與 `avatarKey` 成對
    /// （49 §6.1；舊名 `buyerAvatarUrl` 已於 2026-08-05 收斂移除）。
    @Default('') String avatarUrl,
    @Default('') String shippingAddress,
    @Default('') String temperatureCode,
    @Default('') String status,
    @Default('') String reason,
    @Default('') String description,
    @Default('') String refundableAmount,
    @Default('') String returnShipmentNo,
    @Default('') String returnShippingMethod,
    String? shipTradeNo,
    @JsonKey(readValue: _readCaseValue) @Default('') String providerStatus,
    @JsonKey(readValue: _readCaseValue) @Default('') String providerMessage,
    @JsonKey(readValue: _readCaseValue) @Default('') String refundTaskStatus,
    @JsonKey(readValue: _readCaseValue)
    @Default('')
    String refundFailureMessage,
    @JsonKey(readValue: _readCaseValue) @Default('') String estimatedRefundDate,
    @JsonKey(readValue: _readCaseValue) @Default('') String bankDate,
    String? odno,
    String? validationNo,
    @Default(false) bool needRefundPage,
    @Default('') String createdAt,
    @Default('') String closedAt,
    @Default([]) List<AfterSaleItem> items,
    @Default([]) List<String> shipmentPhotoUrls,
    @Default([]) List<Map<String, dynamic>> evidence,
    @Default([]) List<AfterSaleResolution> resolutions,
    // B-130 / backend issue #568：處理時限與平台接手狀態
    @Default('') String processDeadline,
    @Default(false) bool autoEscalated,
    @Default(0) int processDeadlineHours,

    /// 申訴狀態（AppealState：'0'=未申訴；'10' APPEALED / '20' INVESTIGATING
    /// 進入平台仲裁）。後端列表 summary 已回傳；賣家卡片需據此把狀態折疊成
    /// 「申訴仲裁中」，否則只看 status 會顯示已拒絕 / 待處理。
    @Default('') String appealState,
  }) = _AfterSaleCase;

  factory AfterSaleCase.fromJson(Map<String, dynamic> json) =>
      _$AfterSaleCaseFromJson(json);
}

/// 售後案件中的商品項目
@freezed
abstract class AfterSaleItem with _$AfterSaleItem {
  const factory AfterSaleItem({
    @Default('') String orderItemId,
    @Default('') String skuId,
    @Default('') String skuName,
    @Default('') String quantity,
    // PayUni #758: 新欄位（成交單價 / 原價）
    @Default('') String unitPrice,
    @Default('') String listPrice,

    /// Deprecated: 舊欄位 fallback。
    @Default('') String originalPrice,
    @Default('') String actualPayAmount,
    @Default('') String shareDiscount,

    /// 商品圖 objectKey（SSOT；49 §6.1）。Gateway 成對回 imageKey / imageUrl，
    /// 後者是它由 key 組出的衍生物
    /// （`aftersales.go` 的 `ImageUrl: buildURL(mediaBase, item.ImageKey)`），
    /// 顯示 URL 交回 buildImageUrl 唯一組裝入口。
    @Default('') String imageKey,
  }) = _AfterSaleItem;

  factory AfterSaleItem.fromJson(Map<String, dynamic> json) =>
      _$AfterSaleItemFromJson(json);
}

/// AfterSaleItem 顯示用 fallback。
extension AfterSaleItemPriceX on AfterSaleItem {
  String get effectiveUnitPrice {
    if (unitPrice.isNotEmpty) return unitPrice;
    return originalPrice;
  }

  String get effectiveListPrice {
    if (listPrice.isNotEmpty) return listPrice;
    return effectiveUnitPrice;
  }

  bool get shouldStrikeListPrice {
    final lp = num.tryParse(effectiveListPrice);
    final up = num.tryParse(effectiveUnitPrice);
    if (lp == null || up == null) return false;
    return lp > up;
  }
}

/// 售後決策記錄
@freezed
abstract class AfterSaleResolution with _$AfterSaleResolution {
  const factory AfterSaleResolution({
    @Default('') String resolutionType,
    @Default('') String refundAmount,
    @Default('') String couponAmount,
    @Default('') String couponMinSpend,
    @Default('') String couponMessage,
    @Default('') String rejectReason,
    @Default('') String rejectMessage,
    @Default('') String negotiationReason,
    @Default('') String createdAt,
  }) = _AfterSaleResolution;

  factory AfterSaleResolution.fromJson(Map<String, dynamic> json) =>
      _$AfterSaleResolutionFromJson(json);
}

/// 售後狀態碼
class AfterSaleStatus {
  static final String pending = AfterSaleCaseStatus.pending.value;
  static final String awaitingReturn = AfterSaleCaseStatus.returnPending.value;
  static final String returnInTransit =
      AfterSaleCaseStatus.returnShipping.value;

  /// 27 退貨已送達 — 等待賣家確認收貨；唯一允許按「確認收貨」按鈕的狀態
  static final String returnDelivered =
      AfterSaleCaseStatus.returnDelivered.value;
  static final String returnReceived = AfterSaleCaseStatus.returnReceived.value;
  static final String autoCompletedStorePickupTimeout =
      AfterSaleCaseStatus.autoCompletedStorePickupTimeout.value;
  static final String autoCompletedConfirmReceiptTimeout =
      AfterSaleCaseStatus.autoCompletedConfirmReceiptTimeout.value;
  static final String refundProcessing =
      AfterSaleCaseStatus.refundProcessing.value;
  static final String closed = AfterSaleCaseStatus.closed.value;
  static final String rejected = AfterSaleCaseStatus.rejected.value;
  static final String cancelled = AfterSaleCaseStatus.cancelled.value;
  static final String cancelledByConfirmReceipt =
      AfterSaleCaseStatus.cancelledByConfirmReceipt.value; // #1225

  /// 「已收退貨」終態集合 — 含手動 confirm(30) + 系統自動完成 (31/32)。
  /// 賣家側「已完成 tab」/ 退貨流程展開區應用此集合判斷，避免漏掉自動完成 case。
  static Set<String> get returnCompletedSet => {
    returnReceived,
    autoCompletedStorePickupTimeout,
    autoCompletedConfirmReceiptTimeout,
  };

  /// 系統自動完成（31/32）— 賣家無動作；UI 顯示應跳過確認收貨按鈕、直接顯示自動完成訊息。
  static bool isAutoCompleted(String status) =>
      status == autoCompletedStorePickupTimeout ||
      status == autoCompletedConfirmReceiptTimeout;

  /// 後端目前可能不回 27，而是在退貨送達時維持 25 並更新物流欄位。
  /// 前端以 status=27 或「退貨運送中 + 物流已送達」視為可確認收貨。
  static bool canConfirmReturnReceipt(AfterSaleCase c) {
    if (c.status == returnDelivered) return true;
    if (c.status != returnInTransit) return false;
    return AfterSaleReturnProgress.isDelivered(c);
  }

  static String label(String status) =>
      CodeMetaDisplay.afterSaleCaseStatus(status);

  /// 申訴 / 平台仲裁狀態。
  ///
  /// 後端只吐 CodeMeta `AfterSaleAppealState` 的 **Value**（'10' APPEALED /
  /// '20' INVESTIGATING）；Name 不會出現在 wire 上（gateway 收到 Name 直接 400，
  /// aftersales-service 落庫也是 Value），故此處只比 Value。
  /// DB 欄位 NOT NULL DEFAULT '0'，故 '' / '0' 都不算申訴。
  static bool isUnderAppeal(String appealState) =>
      appealState == AfterSaleAppealState.appealed.value ||
      appealState == AfterSaleAppealState.investigating.value;

  /// 申訴中卡片顯示文字（覆寫 raw status 的「已拒絕 / 待處理」）。
  static const String appealLabel = '申訴仲裁中';

  /// 折疊申訴狀態的顯示 label — 申訴中時覆寫 raw status，與詳情頁仲裁顯示一致。
  static String displayLabel(String status, String appealState) =>
      isUnderAppeal(appealState) ? appealLabel : label(status);
}

/// 退貨流程 ShippingStatus code（對齊 CodeMeta）。
/// 出處：backend-payuni/docs/代碼管理（CodeMeta）/業務狀態碼.md#ShippingStatus
///
/// 等買家寄：
///   `21` PENDING_SHIPMENT       待出貨（尚未進入運送）
/// 在途：
///   `22` LOGISTICS_CHECK        物流驗收中
///   `31` IN_TRANSIT             配送中
///   `51` RETURN_GENERAL         一般退貨（退貨便/黑貓退貨）
///   `53` RETURN_TO_WAREHOUSE    廠退（回物流中心，非賣家取件點）
///   `56` RETURN_BY_HOME_DELIVERY 已轉宅配退回
///   `210` TCAT_CALLED           已叫件（等黑貓取件）
/// 已送達 / 完成：
///   `32` WAITING_PICKUP         待取貨（包裹配達取件門市，**退貨已送達**）
///   `11` PICKED_UP              已取貨（賣家已取走，最終態）
/// 異常：
///   `33` ABNORMAL / `41` CANCELLED / `44` PACKAGE_LOST / `46` PACKAGE_ABANDONED
///   `52` BUYER_NOT_PICKED / `55` SELLER_NOT_PICKED / `82` PENDING_HOME_RETURN
class ReturnShipmentStatus {
  ReturnShipmentStatus._();
  static const String pendingShipment = '21';
  static const String logisticsCheck = '22';
  static const String inTransit = '31';
  static const String awaitingPickup = '32';
  static const String pickedUp = '11';
  static const String abnormal = '33';
  static const String cancelled = '41';
  static const String packageLost = '44';
  static const String returnGeneral = '51';
  static const String buyerNotPicked = '52';
  static const String returnToWarehouse = '53';
  static const String sellerNotPicked = '55';
  static const String returnByHomeDelivery = '56';
  static const String pendingHomeReturn = '82';
  static const String tcatCalled = '210';

  /// 已寄出 = 物流商已收件且未取消。
  static const Set<String> shipped = {
    logisticsCheck,
    inTransit,
    awaitingPickup,
    pickedUp,
    returnGeneral,
    returnToWarehouse,
    returnByHomeDelivery,
    tcatCalled,
  };

  /// 已送達 = 抵達賣家取件點或已被取走。
  /// 注意 `53 RETURN_TO_WAREHOUSE` 是回物流中心，不是取件點，**不列入**。
  static const Set<String> delivered = {awaitingPickup, pickedUp};

  /// 異常 / 失敗 / 需人工介入。
  static const Set<String> abnormalSet = {
    abnormal,
    cancelled,
    packageLost,
    buyerNotPicked,
    sellerNotPicked,
  };
}

class AfterSaleReturnProgress {
  /// 物流商已收件 / 運送中 / 抵達取件點 / 已取貨 — 任一即視為「買家已寄」。
  static bool hasShipped(AfterSaleCase c) {
    final code = _text(c.providerStatus);
    if (ReturnShipmentStatus.shipped.contains(code)) return true;
    final msg = _text(c.providerMessage);
    if (msg.isEmpty) return false;
    if (msg.contains('配送中') ||
        msg.contains('運送中') ||
        msg.contains('已取貨') ||
        msg.contains('已收件') ||
        msg.contains('取件中')) {
      return true;
    }
    return _deliveredKeywords.any(msg.contains);
  }

  static bool isDelivered(AfterSaleCase c) {
    final code = _text(c.providerStatus);
    if (ReturnShipmentStatus.delivered.contains(code)) return true;
    final msg = _text(c.providerMessage);
    if (msg.isEmpty) return false;
    if (msg.contains('配送中') || msg.contains('運送中') || msg.contains('取件中')) {
      return false;
    }
    return _deliveredKeywords.any(msg.contains);
  }

  /// 物流異常 / 取消 / 遺失 / 未取 — UI 可顯紅字提示。
  static bool isAbnormal(AfterSaleCase c) =>
      ReturnShipmentStatus.abnormalSet.contains(_text(c.providerStatus));
  static bool isCancelled(AfterSaleCase c) =>
      _text(c.providerStatus) == ReturnShipmentStatus.cancelled;

  static const _deliveredKeywords = [
    '配達完成',
    '已送達',
    '送達',
    '配達',
    '退貨完成',
    '退回完成',
    '已送回',
    '送回賣家',
    '已到賣家',
  ];

  static String _text(String? value) => value?.trim() ?? '';
}

/// 售後申請原因
class AfterSaleReason {
  static final String defective = RefundReason.defective.code;
  static final String wrongItem = RefundReason.wrongItem.code;
  static final String notAsDescribed = RefundReason.notAsDescribed.code;
  static final String changeOfMind = RefundReason.changeOfMind.code;
  static final String other = RefundReason.other.code;

  static String label(String reason) => CodeMetaDisplay.refundReason(reason);
}
