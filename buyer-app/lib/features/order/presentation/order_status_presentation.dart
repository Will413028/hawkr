import 'package:flutter/material.dart';

import '../../../core/codemeta/code_meta.dart';
import '../../../shared/widgets/hk_icon.dart';
import '../data/order_model.dart';
import '../order_color_tokens.dart';

/// 訂單詳情與列表共用的狀態視覺規格。
///
/// 後端的訂單狀態與物流貨態是兩條不同狀態機：訂單狀態決定交易階段，
/// 物流貨態補充實際包裹位置。所有訂單 UI 都應透過本檔解析，避免同一筆
/// 訂單在列表與詳情顯示不同名稱、顏色或下一步文案。
@immutable
class OrderStatusPresentation {
  const OrderStatusPresentation({
    required this.label,
    required this.description,
    required this.icon,
    required this.color,
    required this.softColor,
  });

  final String label;
  final String description;
  final HkIconName icon;
  final Color color;
  final Color softColor;

  static OrderStatusPresentation fromOrder(Order order) {
    final fallback = order.statusSummary.trim();
    String description(String value) => fallback.isNotEmpty ? fallback : value;

    return switch (order.status) {
      '0' => OrderStatusPresentation(
        label: order.paymentDeadlineExpired ? '付款已逾期' : '待付款',
        description: description(
          order.paymentDeadlineExpired ? '付款期限已過，請回列表確認最新狀態。' : '請在期限內完成付款。',
        ),
        icon: HkIconName.orderWaitingPayment,
        color: OrderColorTokens.attention,
        softColor: OrderColorTokens.attentionSurface,
      ),
      '10' => OrderStatusPresentation(
        label: '已付款',
        description: description('付款已完成，等待賣家確認訂單。'),
        icon: HkIconName.orderPaid,
        color: OrderColorTokens.primary,
        softColor: OrderColorTokens.primarySurface,
      ),
      '20' => OrderStatusPresentation(
        label: '待出貨',
        description: description('賣家正在備貨，完成後會交付物流。'),
        icon: HkIconName.orderPreparing,
        color: OrderColorTokens.primary,
        softColor: OrderColorTokens.primarySurface,
      ),
      '30' => OrderStatusPresentation(
        label: '配送中',
        description: description('包裹正在配送途中，請留意物流更新。'),
        icon: HkIconName.orderShipping,
        color: OrderColorTokens.progress,
        softColor: OrderColorTokens.progressSurface,
      ),
      '32' => OrderStatusPresentation(
        label: '待取貨',
        description: description('包裹已到門市，請在期限內完成取貨。'),
        icon: HkIconName.orderPickup,
        color: OrderColorTokens.progress,
        softColor: OrderColorTokens.progressSurface,
      ),
      '35' => OrderStatusPresentation(
        label: '物流異常',
        description: description('配送遇到異常，請查看物流訊息或聯繫客服。'),
        icon: HkIconName.orderLogisticsIssue,
        color: OrderColorTokens.attention,
        softColor: OrderColorTokens.attentionSurface,
      ),
      '40' => OrderStatusPresentation(
        label: '已送達',
        description: description('商品已送達，請確認商品狀況與收貨。'),
        icon: HkIconName.orderDelivered,
        color: OrderColorTokens.primary,
        softColor: OrderColorTokens.primarySurface,
      ),
      '50' => OrderStatusPresentation(
        label: '已完成',
        description: description(
          !order.isReviewed && order.canReview ? '訂單已完成，歡迎留下評價。' : '交易已順利完成。',
        ),
        icon: HkIconName.orderCompleted,
        color: OrderColorTokens.primary,
        softColor: OrderColorTokens.primarySurface,
      ),
      '60' => OrderStatusPresentation(
        label: '部分退款',
        description: description('部分商品已進入退款流程，請查看退款紀錄。'),
        icon: HkIconName.orderPartialRefund,
        color: OrderColorTokens.attention,
        softColor: OrderColorTokens.attentionSurface,
      ),
      '70' => OrderStatusPresentation(
        label: '全額退款',
        description: description('全額退款已處理，入帳時間依付款方式而定。'),
        icon: HkIconName.orderRefunded,
        color: OrderColorTokens.neutral,
        softColor: OrderColorTokens.neutralSurface,
      ),
      '80' => OrderStatusPresentation(
        label: '已取消',
        description: description(
          order.needRefundPage ? '訂單已取消，請補填退款帳戶資料。' : _cancelDescription(order),
        ),
        icon: HkIconName.orderCancelled,
        color: OrderColorTokens.neutral,
        softColor: OrderColorTokens.neutralSurface,
      ),
      _ => OrderStatusPresentation(
        label: '狀態更新中',
        description: description('訂單狀態尚未同步完成，請稍後重新整理。'),
        icon: HkIconName.orderUnknown,
        color: OrderColorTokens.neutral,
        softColor: OrderColorTokens.neutralSurface,
      ),
    };
  }

  static String _cancelDescription(Order order) => switch (order.cancelReason) {
    '9101' => '付款期限已過，系統已自動取消訂單。',
    '9102' => '賣家未在期限內出貨，系統已取消並安排退款。',
    '9201' => '因天災影響無法配送，賣家已取消訂單。',
    '9202' => '商品備貨不足，賣家已取消訂單。',
    '9203' => '商品品質未達出貨標準，賣家已取消訂單。',
    '9204' => '此訂單需拆單處理，賣家已取消原訂單。',
    '9299' =>
      order.cancelRemark.trim().isNotEmpty
          ? '賣家取消原因：${order.cancelRemark.trim()}'
          : '賣家已取消訂單，退款將依付款方式處理。',
    _ => '訂單已取消，請查看付款與退款資訊。',
  };
}

@immutable
class DeliveryStatusPresentation {
  const DeliveryStatusPresentation({
    required this.label,
    required this.description,
    required this.icon,
    required this.color,
    required this.softColor,
    required this.progressStep,
    required this.isAbnormal,
  });

  final String label;
  final String description;
  final HkIconName icon;
  final Color color;
  final Color softColor;
  final int progressStep;
  final bool isAbnormal;

  /// 「這筆配送出了問題」的狀態碼集合（`ShippingStatus` 的 value）。
  ///
  /// 🔴 **它是唯一一份**：訂單卡與訂單詳情的物流卡都讀這裡
  /// （`OrderLogisticsSection._TrackBody._isAbnormal`）。兩份一旦分岔，
  /// 症狀是同一張訂單在列表上是紅的、在詳情頁是正常的——而沒有東西會報錯。
  ///
  /// ⚠️ `991`（建單結果未知）**刻意不在內**： 逐字寫著它「不是失敗」，
  /// 由回查排程收斂成 `91`／`990`。把它標成異常會讓買家在一個會自動好轉的
  /// 中間狀態上被嚇到，並且去開一張不需要的客服單。
  /// ⚠️ `992`（取消處理中）同理：那是一個申請已受理的正常中間狀態。
  static const abnormalValues = <String>{
    '33',
    '41',
    '43',
    '44',
    '46',
    '52',
    '55',
    '81',
    '82',
    '990',
    // 理賠受理中（遺失／毀損爭議）：已經出事，正在處理。
    '993',
  };

  static DeliveryStatusPresentation fromOrder(Order order) {
    final raw = order.shippingStatus.trim();
    final status = ShippingStatus.fromValue(raw);
    final orderStatus = order.status;
    final abnormal = orderStatus == '35' || abnormalValues.contains(raw);

    if (orderStatus == '80') {
      return const DeliveryStatusPresentation(
        label: '配送已終止',
        description: '訂單已取消，不會再繼續配送。',
        icon: HkIconName.orderCancelled,
        color: OrderColorTokens.neutral,
        softColor: OrderColorTokens.neutralSurface,
        progressStep: 1,
        isAbnormal: false,
      );
    }
    if (abnormal) {
      return DeliveryStatusPresentation(
        label: status?.label ?? '物流異常',
        description: _shippingDescription(status, raw, abnormal: true),
        icon: HkIconName.orderLogisticsIssue,
        color: OrderColorTokens.attention,
        softColor: OrderColorTokens.attentionSurface,
        progressStep: 2,
        isAbnormal: true,
      );
    }
    if (orderStatus == '0') {
      return const DeliveryStatusPresentation(
        label: '尚未建立配送',
        description: '完成付款後，賣家才會開始準備配送。',
        icon: HkIconName.orderWaitingPayment,
        color: OrderColorTokens.attention,
        softColor: OrderColorTokens.attentionSurface,
        progressStep: 0,
        isAbnormal: false,
      );
    }

    final step = _progressStep(orderStatus, status);
    final label = status?.label ?? _fallbackLabel(orderStatus);
    return DeliveryStatusPresentation(
      label: label,
      description: _shippingDescription(
        status,
        raw,
        fallback: order.statusSummary,
      ),
      icon: _iconFor(status, step),
      color: step >= 3
          ? OrderColorTokens.primary
          : step >= 2
          ? OrderColorTokens.progress
          : OrderColorTokens.primary,
      softColor: step >= 3
          ? OrderColorTokens.primarySurface
          : step >= 2
          ? OrderColorTokens.progressSurface
          : OrderColorTokens.primarySurface,
      progressStep: step,
      isAbnormal: false,
    );
  }

  static int _progressStep(String orderStatus, ShippingStatus? shipping) {
    if (orderStatus == '40' ||
        orderStatus == '50' ||
        shipping == ShippingStatus.pickedUp) {
      return 3;
    }
    if (orderStatus == '30' || orderStatus == '32') return 2;
    if (shipping == ShippingStatus.inTransit ||
        shipping == ShippingStatus.waitingPickup ||
        // 930 已離開賣家手上，與 31 同屬「配送中」那一格。
        shipping == ShippingStatus.handedOver) {
      return 2;
    }
    return 1;
  }

  static String _fallbackLabel(String orderStatus) => switch (orderStatus) {
    '10' => '等待賣家處理',
    '20' => '準備出貨',
    '30' => '配送中',
    '32' => '待取貨',
    '40' || '50' => '已送達',
    '60' || '70' => '配送已結束',
    _ => '物流資訊更新中',
  };

  static HkIconName _iconFor(ShippingStatus? status, int step) {
    if (status == ShippingStatus.waitingPickup) {
      return HkIconName.orderPickup;
    }
    if (status == ShippingStatus.pickedUp || step >= 3) {
      return HkIconName.orderDelivered;
    }
    if (status == ShippingStatus.returnGeneral ||
        status == ShippingStatus.returnToWarehouse ||
        status == ShippingStatus.returnByHomeDelivery) {
      return HkIconName.orderReturning;
    }
    return step >= 2 ? HkIconName.orderShipping : HkIconName.orderPreparing;
  }

  static String _shippingDescription(
    ShippingStatus? status,
    String raw, {
    bool abnormal = false,
    String fallback = '',
  }) {
    if (fallback.trim().isNotEmpty) return fallback.trim();
    if (abnormal) {
      return switch (status) {
        ShippingStatus.packageLost => '物流回報包裹遺失，客服將協助後續處理。',
        ShippingStatus.buyerNotPicked => '已超過取貨期限，包裹正在退回。',
        ShippingStatus.storeTransfer => '原門市暫停服務，請依通知重新選擇門市。',
        ShippingStatus.createFailed => '物流單建立失敗，請聯繫客服協助。',
        ShippingStatus.claimPending => '理賠案件處理中，客服將與您聯繫。',
        _ => '配送發生異常，請查看通知或聯繫客服。',
      };
    }
    return switch (status) {
      ShippingStatus.unprocessed => '賣家尚未建立物流單。',
      ShippingStatus.processing => '物流單正在建立，請稍候更新。',
      ShippingStatus.pendingShipment => '物流單已建立，等待賣家出貨。',
      ShippingStatus.logisticsCheck => '包裹已交寄，物流中心正在驗收。',
      ShippingStatus.inTransit => '包裹正在前往收件地址。',
      ShippingStatus.waitingPickup => '包裹已到門市，請在期限內取貨。',
      ShippingStatus.pickedUp => '買家已完成取貨。',
      ShippingStatus.returnGeneral => '包裹已進入退貨配送流程。',
      ShippingStatus.returnToWarehouse => '包裹正在退回物流中心。',
      ShippingStatus.returnByHomeDelivery => '包裹已改由宅配退回。',
      ShippingStatus.tcatCalled => '已預約宅配取件，等待司機收件。',
      ShippingStatus.cvsCodeReceived => '已取得超商寄件碼，等待賣家交寄。',
      ShippingStatus.sellerConfirmed => '賣家已確認交寄，等待物流更新。',
      ShippingStatus.handedOver => '包裹已交運，正在前往收件地址。',
      // 991／992 不是異常（見 abnormalValues 的說明），但也不能沿用
      // 「請留意後續物流通知」——那句話對「系統正在自己確認」沒有說明力。
      ShippingStatus.createUnknown => '物流單建立結果確認中，稍後會自動更新。',
      ShippingStatus.cancelPending => '取消申請已受理，等待物流商確認。',
      null => raw.isEmpty ? '物流資料尚未同步。' : '物流回傳狀態碼 $raw，資料同步中。',
      _ => '${status.label}，請留意後續物流通知。',
    };
  }
}
