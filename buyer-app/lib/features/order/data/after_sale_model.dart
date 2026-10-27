import 'package:freezed_annotation/freezed_annotation.dart';
import 'package:hawkr_buyer/core/codemeta/code_meta.dart';

part 'after_sale_model.freezed.dart';
part 'after_sale_model.g.dart';

/// 售後案件
@freezed
abstract class AfterSaleCase with _$AfterSaleCase {
  const factory AfterSaleCase({
    @Default('') String caseNo,
    @Default('') String orderNo,
    @Default('') String buyerId,
    @Default('') String storeId,
    @Default('') String status,
    @Default('') String reason,
    @Default('') String description,
    @Default('') String refundableAmount,
    @Default('') String returnShipmentNo,
    @Default('') String returnShippingMethod,

    /// 寄件交易單號（退貨物流拋單後後端回填，顯示於退貨物流 timeline）
    @Default('') String shipTradeNo,
    @Default(false) bool needRefundPage,
    @Default('') String createdAt,
    @Default('') String closedAt,
    @Default([]) List<AfterSaleItem> items,
    @Default([]) List<String> shipmentPhotoUrls,
    @Default([]) List<AfterSaleEvidence> evidence,
    @Default([]) List<AfterSaleResolution> resolutions,

    /// 申訴狀態（'0'=未申訴，其他值見後端 AppealState codemeta）
    @Default('') String appealState,

    /// 是否可以發起申訴（賣家拒絕後買家可於期限內申訴）
    @Default(false) bool canAppeal,
    // B-130 / backend issue #568：賣家處理時限與平台接手狀態
    @Default('') String processDeadline,
    @Default(false) bool autoEscalated,
    @Default(0) int processDeadlineHours,
    // B-125 / B-114 退款進度（對齊 backend issue #568 規格）
    /// ATM 退款任務狀態（[RefundTaskStatus]）：後端 WorkerTaskStatus，保留舊數字碼相容。
    @Default('') String refundTaskStatus,

    /// 上次填寫失敗訊息（後端已依狀態收斂，前台可直接顯示）
    @Default('') String refundFailureMessage,

    /// 預計入帳日期（YYYY-MM-DD，銀行處理中時顯示）
    @Default('') String estimatedRefundDate,

    /// 實際匯款日期（YYYY-MM-DD，退款完成時顯示）
    @Default('') String bankDate,
    @Default('') String providerStatus,
    @Default('') String providerMessage,
    @Default('') String appealDeadline,
    // B-93 7-11 出貨便代碼（三段碼，買家寄退時憑此到 ibon 列印託運單）
    /// 母代碼（3 碼）— 顯示條件：resolutionType=1 + returnShippingMethod=3002 + status≥20
    @Default('') String partnerId,

    /// 出貨單編號（8 碼）
    @Default('') String odno,

    /// C2C 驗證碼（4 碼）
    @Default('') String validationNo,

    /// 決議類型：1=退貨退款、2=部分退款、3=全額退款免退貨
    /// （case 層級欄位，供買家側快速判斷顯示條件，與 resolutions 陣列同步）
    @Default('') String caseResolutionType,

    /// 寄回截止時間（ISO 8601 UTC，狀態 20 時由後端回填，預設賣家同意後 7 天）
    /// 用於買家側顯示「剩餘寄回時間」倒數與 D-1 警示 banner。
    /// status=33 為「待寄回逾時自動關案」終態（無倒數）。
    @Default('') String shipBackDeadlineAt,
  }) = _AfterSaleCase;

  factory AfterSaleCase.fromJson(Map<String, dynamic> json) =>
      _$AfterSaleCaseFromJson(json);
}

/// 售後案件證據（買家舉證 / 賣家驗貨）
///
/// evidenceType 見 [EvidenceType]：
/// - `8601` = 買家提交舉證
/// - `8602` = 賣家驗貨照片（B-96）
@freezed
abstract class AfterSaleEvidence with _$AfterSaleEvidence {
  const factory AfterSaleEvidence({
    @Default('') String assetId,
    @Default('') String imageUrl,
    @Default('') String evidenceType,
    @Default('') String createdAt,
  }) = _AfterSaleEvidence;

  factory AfterSaleEvidence.fromJson(Map<String, dynamic> json) =>
      _$AfterSaleEvidenceFromJson(json);
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
    @Default('') String imageUrl,
  }) = _AfterSaleItem;

  factory AfterSaleItem.fromJson(Map<String, dynamic> json) =>
      _$AfterSaleItemFromJson(json);
}

/// AfterSaleItem 顯示用 fallback：unitPrice -> originalPrice
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
    // B-96 驗貨扣款（resolutionType=6 INSPECTION_DEDUCTION）
    /// 扣款原因（賣家驗貨後填寫，部分退款場景）
    @Default('') String deductionReason,

    /// 驗貨備註（賣家私註）
    @Default('') String inspectionNote,
  }) = _AfterSaleResolution;

  factory AfterSaleResolution.fromJson(Map<String, dynamic> json) =>
      _$AfterSaleResolutionFromJson(json);
}

/// 售後決議類型（resolutionType）
class ResolutionType {
  ResolutionType._();
  static const String approved = '1'; // 同意退款（決議金額）
  static const String coupon = '2'; // 改發優惠券
  static const String reject = '3'; // 拒絕
  static const String negotiation = '4'; // 協商中
  static const String inspectionDeduction = '6'; // 驗貨扣款（B-96）
}

/// 證據類型（evidence.evidenceType）
class EvidenceType {
  EvidenceType._();
  static const String buyerProof = '8601'; // 買家提交的舉證
  static const String sellerInspection = '8602'; // 賣家驗貨照片（B-96）
}

/// 退貨方式
class ReturnShippingMethod {
  ReturnShippingMethod._();
  static const String kuroneko = '3001'; // 黑貓宅配
  static const String sevenEleven = '3002'; // 7-11 店到店
}

/// B-93 出貨便代碼（7-11）顯示條件判斷
extension AfterSale711CodeX on AfterSaleCase {
  /// 是否應顯示 7-11 三段碼（resolutionType=1 + 7-11 + status≥20）
  bool get shouldShow711ReturnCode {
    if (caseResolutionType != ResolutionType.approved) return false;
    if (returnShippingMethod != ReturnShippingMethod.sevenEleven) return false;
    final s = int.tryParse(status) ?? 0;
    return s >= 20 && partnerId.isNotEmpty && odno.isNotEmpty;
  }

  /// 完整出貨便代碼（顯示用，三段空格分隔）
  String get formatted711Code => '$partnerId $odno $validationNo';
}

/// 售後狀態碼（對齊 backend docs/代碼管理（CodeMeta）/業務狀態碼.md#AfterSaleCaseStatus）
class AfterSaleStatus {
  static const String pending = '0';
  static const String awaitingReturn = '20'; // RETURN_PENDING 待退貨
  static const String returnInTransit = '25'; // RETURN_SHIPPING 退貨運送中
  static const String returnReceived = '30'; // RETURN_RECEIVED 已收退貨
  /// 待寄回逾時自動關案（PR #983 新增）
  /// 買家未在 shipBackDeadlineAt 前寄回退貨商品，系統自動關案；72h 內可申訴。
  static const String autoCompletedReturnPendingTimeout = '33';
  static const String refundProcessing = '40';
  static const String closed = '50';
  static const String rejected = '60';
  static const String cancelled = '70';
  static const String cancelledByConfirmReceipt = '71'; // #1225 確認收貨取消

  static String label(String status) => switch (status) {
    '0' => '待處理',
    '20' => '賣家已同意退貨',
    '25' => '退貨運送中',
    '30' => '已收退貨',
    '33' => '待寄回逾時自動關案',
    '40' => '退款處理中',
    '50' => '已結案',
    '60' => '已拒絕',
    '70' => '已取消',
    '71' => '買家已確認收貨',
    _ => '未知',
  };

  /// 申訴 / 平台仲裁狀態。
  ///
  /// 後端只吐 CodeMeta `AfterSaleAppealState` 的 **Value**（'10' APPEALED /
  /// '20' INVESTIGATING）；Name 不會出現在 wire 上（gateway 收到 Name 直接 400，
  /// aftersales-service 落庫也是 Value），故此處只比 Value。
  /// DB 欄位 NOT NULL DEFAULT '0'，故 '' / '0' 都不算申訴。
  static bool isUnderAppeal(String appealState) =>
      appealState == AfterSaleAppealState.appealed.value ||
      appealState == AfterSaleAppealState.investigating.value;

  /// 申訴中列表/卡片顯示文字（覆寫 raw status 的「已拒絕 / 待處理」）。
  static const String appealLabel = '申訴仲裁中';

  /// 折疊申訴狀態的顯示 label — 申訴中時覆寫 raw status，與詳情頁仲裁 banner 一致。
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
///   `53` RETURN_TO_WAREHOUSE    廠退（回物流中心）
///   `56` RETURN_BY_HOME_DELIVERY 已轉宅配退回
///   `210` TCAT_CALLED           已叫件（等黑貓取件）
/// 已送達 / 完成：
///   `32` WAITING_PICKUP         待取貨（包裹配達取件門市，**這就是退貨已送達**）
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

  /// 已寄出 = 物流商已收件且未取消（買家不再持有包裹）。
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

  /// 已送達 = 已抵達賣家可取件的點或已被取走。
  /// 注意 `53 RETURN_TO_WAREHOUSE` 雖然有「配達」字樣但是退回**物流中心**而非
  /// 賣家取件點，屬中繼站，**不算 delivered**。
  static const Set<String> delivered = {awaitingPickup, pickedUp};

  /// 異常 / 失敗 / 需人工介入 — UI 應顯紅字提示。
  static const Set<String> abnormalSet = {
    abnormal,
    cancelled,
    packageLost,
    buyerNotPicked,
    sellerNotPicked,
  };
}

/// 退貨進度判斷（從 providerStatus / providerMessage 推導真實物流狀態，
/// 避免單看 case status — backend 在 7-11 拋單成功就把 status 推到 25
/// returnInTransit，但 providerStatus 仍是 21 待出貨，買家還沒去寄）。
class AfterSaleReturnProgress {
  AfterSaleReturnProgress._();

  /// 物流商已收件 / 運送中 / 抵達門市 / 已取貨 — 任一情況都視為「買家已寄出」。
  static bool hasShipped(AfterSaleCase c) {
    final code = c.providerStatus.trim();
    if (ReturnShipmentStatus.shipped.contains(code)) return true;
    // providerStatus 沒有 / 未推時 fallback 看 message 文案
    final msg = c.providerMessage.trim();
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

  /// 已送達賣家取件點（含已被取走）。
  static bool isDelivered(AfterSaleCase c) {
    final code = c.providerStatus.trim();
    if (ReturnShipmentStatus.delivered.contains(code)) return true;
    final msg = c.providerMessage.trim();
    if (msg.isEmpty) return false;
    if (msg.contains('配送中') || msg.contains('運送中') || msg.contains('取件中')) {
      return false;
    }
    return _deliveredKeywords.any(msg.contains);
  }

  /// 物流異常 / 取消 / 遺失 / 未取（UI 可顯紅字提示）。
  static bool isAbnormal(AfterSaleCase c) =>
      ReturnShipmentStatus.abnormalSet.contains(c.providerStatus.trim());
  static bool isCancelled(AfterSaleCase c) =>
      c.providerStatus.trim() == ReturnShipmentStatus.cancelled;

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
}

/// ATM 退款任務狀態（B-125 / backend issue #568）
class RefundTaskStatus {
  RefundTaskStatus._();

  static const String pending = 'PENDING';
  static const String awaitingBuyerRefundPage = 'AWAITING_BUYER_REFUND_PAGE';
  static const String processing = 'PROCESSING';
  static const String awaitingWebhook = 'AWAITING_WEBHOOK';
  static const String done = 'DONE';
  static const String failed = 'FAILED';
  static const String manualIntervention = 'MANUAL_INTERVENTION';

  static const String pendingFormLegacy = '5';
  static const String bankProcessingLegacy = '15';
  static const String completedLegacy = '20';
  static const String failedLegacy = '30';

  static String normalize(String s) => s.trim().toUpperCase();

  static bool isPending(String s) => normalize(s) == pending;

  static bool needsBuyerForm(String s) {
    final value = normalize(s);
    return value == awaitingBuyerRefundPage || value == pendingFormLegacy;
  }

  static bool isBankProcessing(String s) {
    final value = normalize(s);
    return value == processing ||
        value == awaitingWebhook ||
        value == bankProcessingLegacy;
  }

  static bool isCompleted(String s) {
    final value = normalize(s);
    return value == done || value == completedLegacy;
  }

  static bool isFailed(String s) {
    final value = normalize(s);
    return value == failed ||
        value == manualIntervention ||
        value == failedLegacy;
  }

  /// 根據 case 狀態回傳買家側顯示文案（B-114）
  static String buyerProgressLabel({
    required String taskStatus,
    required String failureMessage,
    required String estimatedRefundDate,
    required String bankDate,
  }) {
    final value = normalize(taskStatus);
    if (needsBuyerForm(value)) {
      if (failureMessage.isNotEmpty) {
        return '⚠ 上次填寫失敗：$failureMessage';
      }
      return '請填寫退款帳號';
    }
    if (isPending(value)) {
      return '退款排隊中';
    }
    if (isBankProcessing(value)) {
      return estimatedRefundDate.isNotEmpty
          ? '銀行處理中，預計 $estimatedRefundDate 入帳'
          : '銀行處理中';
    }
    if (isCompleted(value)) {
      return bankDate.isNotEmpty ? '退款成功，已於 $bankDate 匯款' : '退款成功';
    }
    if (isFailed(value)) {
      if (failureMessage.isNotEmpty) {
        return '退款失敗：$failureMessage';
      }
      if (value == manualIntervention) {
        return '退款需人工處理，請聯繫客服';
      }
      return '退款失敗，請聯繫客服';
    }
    return '';
  }
}

/// 售後申請原因
class AfterSaleReason {
  static String get defective => RefundReason.defective.value;
  static String get wrongItem => RefundReason.wrongItem.value;
  static String get notAsDescribed => RefundReason.notAsDescribed.value;
  static String get changeOfMind => RefundReason.changeOfMind.value;
  static String get other => RefundReason.other.value;

  static String label(String reason) =>
      RefundReason.fromValue(reason)?.label ?? reason;

  static String value(String reason) =>
      RefundReason.fromValue(reason)?.label ?? reason;
}
