import 'package:flutter/material.dart';

import '../../../core/codemeta/code_meta.dart';
import '../../../core/theme/app_colors.dart';
import '../../../shared/widgets/hk_icon.dart';
import '../data/order_model.dart';

/// 賣家端訂單詳情與列表共用的狀態視覺規格。
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

  static OrderStatusPresentation fromOrder(
    OrderSummary? order, {
    int? statusCode,
  }) {
    final code = statusCode ?? order?.statusCode ?? 0;
    final fallback = order?.statusSummary.trim() ?? '';
    String description(String value) => fallback.isNotEmpty ? fallback : value;
    return switch (code) {
      0 => OrderStatusPresentation(
        label: '待付款',
        description: description('買家尚未完成付款，暫時不需備貨。'),
        icon: HkIconName.clock,
        color: AppColors.warningText,
        softColor: AppColors.warningSoft,
      ),
      10 => OrderStatusPresentation(
        label: '已付款',
        description: description('請確認商品庫存並建立物流。'),
        icon: HkIconName.payment,
        color: AppColors.brandPrimaryStrong,
        softColor: AppColors.primarySoft,
      ),
      20 => OrderStatusPresentation(
        label: '待出貨',
        description: description('請在處理期限內完成包裝與交寄。'),
        icon: HkIconName.inventory,
        color: AppColors.brandPrimaryStrong,
        softColor: AppColors.primarySoft,
      ),
      30 => OrderStatusPresentation(
        label: '配送中',
        description: description('包裹配送中，請留意物流更新。'),
        icon: HkIconName.truck,
        color: AppColors.info,
        softColor: AppColors.infoSoft,
      ),
      32 => OrderStatusPresentation(
        label: '待取貨',
        description: description('包裹已到門市，等待買家取貨。'),
        icon: HkIconName.truck,
        color: AppColors.info,
        softColor: AppColors.infoSoft,
      ),
      35 => OrderStatusPresentation(
        label: '物流異常',
        description: description('配送遇到異常，請盡快查核並處理。'),
        icon: HkIconName.warning,
        color: AppColors.error,
        softColor: AppColors.dangerSoft,
      ),
      40 => OrderStatusPresentation(
        label: '已送達',
        description: description('商品已送達，等待買家確認收貨。'),
        icon: HkIconName.check,
        color: AppColors.success,
        softColor: AppColors.successSoft,
      ),
      50 => OrderStatusPresentation(
        label: '已完成',
        description: description('交易已完成並進入歷史訂單。'),
        icon: HkIconName.check,
        color: AppColors.success,
        softColor: AppColors.successSoft,
      ),
      60 => OrderStatusPresentation(
        label: '部分退款',
        description: description('部分商品已進入退款流程。'),
        icon: HkIconName.swap,
        color: AppColors.error,
        softColor: AppColors.dangerSoft,
      ),
      70 => OrderStatusPresentation(
        label: '全額退款',
        description: description('全額退款流程已更新。'),
        icon: HkIconName.returnOrder,
        color: AppColors.textSecondary,
        softColor: AppColors.surfaceMuted,
      ),
      80 => OrderStatusPresentation(
        label: '已取消',
        description: description(_cancelDescription(order)),
        icon: HkIconName.close,
        color: AppColors.textSecondary,
        softColor: AppColors.surfaceMuted,
      ),
      _ => OrderStatusPresentation(
        label: '狀態更新中',
        description: description('訂單狀態尚未同步完成，請稍後重新整理。'),
        icon: HkIconName.receipt,
        color: AppColors.textSecondary,
        softColor: AppColors.surfaceMuted,
      ),
    };
  }

  static String _cancelDescription(OrderSummary? order) =>
      switch (order?.cancelReason) {
        '9101' => '買家未在期限內付款，系統已自動取消。',
        '9102' => '訂單未在期限內出貨，系統已自動取消。',
        '9201' => '因天災影響無法配送，訂單已取消。',
        '9202' => '因商品備貨不足，訂單已取消。',
        '9203' => '因商品品質未達標準，訂單已取消。',
        '9204' => '已建議買家拆單，原訂單已取消。',
        '9299' =>
          order?.cancelRemark.trim().isNotEmpty == true
              ? '取消原因：${order!.cancelRemark.trim()}'
              : '訂單已取消。',
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
  };

  static DeliveryStatusPresentation fromOrder(
    OrderSummary? order, {
    int? statusCode,
  }) {
    final code = statusCode ?? order?.statusCode ?? 0;
    final raw =
        (order?.shipmentStatus.isNotEmpty == true
                ? order!.shipmentStatus
                : order?.shippingStatus ?? '')
            .trim();
    final shipping = ShippingStatus.fromValue(raw);
    final abnormal = code == 35 || abnormalValues.contains(raw);
    if (code == 80) {
      return const DeliveryStatusPresentation(
        label: '配送已終止',
        description: '訂單已取消，不會再繼續配送。',
        icon: HkIconName.close,
        color: AppColors.textSecondary,
        softColor: AppColors.surfaceMuted,
        progressStep: 1,
        isAbnormal: false,
      );
    }
    if (abnormal) {
      return DeliveryStatusPresentation(
        label: shipping?.label ?? '物流異常',
        description: _description(shipping, raw, abnormal: true),
        icon: HkIconName.warning,
        color: AppColors.error,
        softColor: AppColors.dangerSoft,
        progressStep: 2,
        isAbnormal: true,
      );
    }
    if (code == 0) {
      return const DeliveryStatusPresentation(
        label: '尚未建立配送',
        description: '買家付款後才能建立物流。',
        icon: HkIconName.clock,
        color: AppColors.warningText,
        softColor: AppColors.warningSoft,
        progressStep: 0,
        isAbnormal: false,
      );
    }
    final step = _step(code, shipping);
    return DeliveryStatusPresentation(
      label: shipping?.label ?? _fallbackLabel(code),
      description: order?.shipStatusDesc.trim().isNotEmpty == true
          ? order!.shipStatusDesc.trim()
          : _description(shipping, raw),
      icon: _icon(shipping, step),
      color: step >= 3
          ? AppColors.success
          : step >= 2
          ? AppColors.info
          : AppColors.brandPrimaryStrong,
      softColor: step >= 3
          ? AppColors.successSoft
          : step >= 2
          ? AppColors.infoSoft
          : AppColors.primarySoft,
      progressStep: step,
      isAbnormal: false,
    );
  }

  static int _step(int code, ShippingStatus? shipping) {
    if (code == 40 || code == 50 || shipping == ShippingStatus.pickedUp) {
      return 3;
    }
    if (code == 30 ||
        code == 32 ||
        shipping == ShippingStatus.inTransit ||
        shipping == ShippingStatus.waitingPickup) {
      return 2;
    }
    return 1;
  }

  static String _fallbackLabel(int code) => switch (code) {
    10 => '等待建立物流',
    20 => '準備出貨',
    30 => '配送中',
    32 => '待取貨',
    40 || 50 => '已送達',
    60 || 70 => '配送已結束',
    _ => '物流資訊更新中',
  };

  static HkIconName _icon(ShippingStatus? shipping, int step) {
    if (shipping == ShippingStatus.pickedUp || step >= 3) {
      return HkIconName.check;
    }
    if (shipping == ShippingStatus.returnGeneral ||
        shipping == ShippingStatus.returnToWarehouse ||
        shipping == ShippingStatus.returnByHomeDelivery) {
      return HkIconName.returnOrder;
    }
    return step >= 2 ? HkIconName.truck : HkIconName.inventory;
  }

  static String _description(
    ShippingStatus? shipping,
    String raw, {
    bool abnormal = false,
  }) {
    if (abnormal) {
      if (raw == '990') return '物流單建立失敗，請重新處理或聯繫客服。';
      return switch (shipping) {
        ShippingStatus.packageLost => '物流回報包裹遺失，請立即聯繫客服。',
        ShippingStatus.buyerNotPicked => '買家逾期未取，包裹正在退回。',
        ShippingStatus.storeTransfer => '原門市暫停服務，等待買家重選門市。',
        _ => '配送發生異常，請查看通知並盡快處理。',
      };
    }
    return switch (shipping) {
      ShippingStatus.unprocessed => '尚未建立物流單。',
      ShippingStatus.processing => '物流單正在建立。',
      ShippingStatus.processingAccepted => '物流已接單，等待交寄。',
      ShippingStatus.pendingShipment => '等待商品交寄。',
      ShippingStatus.logisticsCheck => '物流中心正在驗收包裹。',
      ShippingStatus.inTransit => '包裹正在配送途中。',
      ShippingStatus.waitingPickup => '包裹已到門市，等待買家取貨。',
      ShippingStatus.pickedUp => '買家已完成取貨。',
      ShippingStatus.returnGeneral => '包裹已進入退貨流程。',
      ShippingStatus.returnToWarehouse => '包裹正在退回物流中心。',
      ShippingStatus.returnByHomeDelivery => '包裹已改由宅配退回。',
      ShippingStatus.tcatCalled => '已預約宅配司機取件。',
      ShippingStatus.cvsCodeReceived => '已取得超商寄件碼，等待交寄。',
      ShippingStatus.sellerConfirmed => '已確認交寄，等待物流更新。',
      null => raw.isEmpty ? '物流資料尚未同步。' : '物流回傳狀態碼 $raw，資料同步中。',
      _ => '${shipping.label}，請留意後續物流通知。',
    };
  }
}
