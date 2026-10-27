import "dart:io";

import 'package:hawkr_buyer/core/theme/app_colors.dart';
import 'package:hawkr_buyer/features/order/data/order_model.dart';
import 'package:hawkr_buyer/features/order/data/order_status.dart';
import 'package:hawkr_buyer/features/order/order_color_tokens.dart';
import 'package:hawkr_buyer/features/order/presentation/order_status_presentation.dart';
import 'package:hawkr_buyer/shared/widgets/hk_icon.dart';
import 'package:hawkr_buyer/shared/widgets/step_indicator.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

void main() {
  const labels = <String, String>{
    '0': '待付款',
    '10': '已付款',
    '20': '待出貨',
    '30': '配送中',
    '32': '待取貨',
    '35': '物流異常',
    '40': '已送達',
    '50': '已完成',
    '60': '部分退款',
    '70': '全額退款',
    '80': '已取消',
  };
  const icons = <String, HkIconName>{
    '0': HkIconName.orderWaitingPayment,
    '10': HkIconName.orderPaid,
    '20': HkIconName.orderPreparing,
    '30': HkIconName.orderShipping,
    '32': HkIconName.orderPickup,
    '35': HkIconName.orderLogisticsIssue,
    '40': HkIconName.orderDelivered,
    '50': HkIconName.orderCompleted,
    '60': HkIconName.orderPartialRefund,
    '70': HkIconName.orderRefunded,
    '80': HkIconName.orderCancelled,
  };

  testWidgets('11 種訂單主狀態都有穩定文案與獨立 Material 圖示', (tester) async {
    final glyphs = <IconData>{};
    for (final entry in labels.entries) {
      final visual = OrderStatusPresentation.fromOrder(
        Order(status: entry.key),
      );
      expect(visual.label, entry.value, reason: 'status=${entry.key}');
      expect(visual.description, isNotEmpty, reason: 'status=${entry.key}');
      expect(visual.icon, icons[entry.key], reason: 'status=${entry.key}');

      await tester.pumpWidget(MaterialApp(home: HkIcon(visual.icon)));
      final glyph = tester.widget<Icon>(find.byType(Icon)).icon!;
      expect(glyph.fontFamily, 'MaterialIcons', reason: entry.key);
      glyphs.add(glyph);
    }
    expect(icons.values.toSet(), hasLength(labels.length));
    expect(glyphs, hasLength(labels.length));
  });

  test('付款過期與 ATM 退款資料缺口有專屬文案', () {
    final expired = OrderStatusPresentation.fromOrder(
      Order(
        status: '0',
        paymentDeadline: DateTime.now()
            .subtract(const Duration(minutes: 1))
            .toIso8601String(),
      ),
    );
    final refund = OrderStatusPresentation.fromOrder(
      const Order(status: '80', needRefundPage: true),
    );

    expect(expired.label, '付款已逾期');
    expect(refund.description, contains('退款帳戶'));
  });

  test('所有已知物流碼都可解析，異常碼不會被誤標成一般配送', () {
    const known = <String>{
      '91',
      '92',
      '98',
      '21',
      '22',
      '31',
      '32',
      '33',
      '11',
      '41',
      '43',
      '44',
      '46',
      '51',
      '52',
      '53',
      '55',
      '56',
      '81',
      '82',
      '210',
      '920',
      '921',
      '990',
    };
    const abnormal = <String>{
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

    for (final value in known) {
      final visual = DeliveryStatusPresentation.fromOrder(
        Order(status: '30', shippingStatus: value),
      );
      expect(visual.label, isNotEmpty, reason: 'shippingStatus=$value');
      expect(
        visual.isAbnormal,
        abnormal.contains(value),
        reason: 'shippingStatus=$value',
      );
    }
  });

  test('未知訂單與物流狀態使用安全 fallback', () {
    final order = OrderStatusPresentation.fromOrder(const Order(status: '999'));
    final delivery = DeliveryStatusPresentation.fromOrder(
      const Order(status: '30', shippingStatus: '777'),
    );

    expect(order.label, '狀態更新中');
    expect(order.icon, HkIconName.orderUnknown);
    expect(delivery.description, contains('777'));
  });

  test('物流狀態同樣回傳語意圖示名稱', () {
    final waiting = DeliveryStatusPresentation.fromOrder(
      const Order(status: '0'),
    );
    final shipping = DeliveryStatusPresentation.fromOrder(
      const Order(status: '30', shippingStatus: '31'),
    );
    final pickup = DeliveryStatusPresentation.fromOrder(
      const Order(status: '32', shippingStatus: '32'),
    );
    final returned = DeliveryStatusPresentation.fromOrder(
      const Order(status: '30', shippingStatus: '51'),
    );

    expect(waiting.icon, HkIconName.orderWaitingPayment);
    expect(shipping.icon, HkIconName.orderShipping);
    expect(pickup.icon, HkIconName.orderPickup);
    expect(returned.icon, HkIconName.orderReturning);
  });

  test('訂單與物流狀態只使用四大主色衍生角色', () {
    final accentColors = [
      OrderColorTokens.primary,
      OrderColorTokens.progress,
      OrderColorTokens.attention,
      OrderColorTokens.neutral,
    ];
    final surfaceColors = [
      OrderColorTokens.primarySurface,
      OrderColorTokens.progressSurface,
      OrderColorTokens.attentionSurface,
      OrderColorTokens.neutralSurface,
    ];

    for (final status in {...labels.keys, '999'}) {
      final visual = OrderStatusPresentation.fromOrder(Order(status: status));
      expect(
        accentColors,
        contains(visual.color),
        reason: 'order status=$status',
      );
      expect(
        surfaceColors,
        contains(visual.softColor),
        reason: 'order status=$status',
      );
    }

    const deliveries = [
      Order(status: '0'),
      Order(status: '30', shippingStatus: '31'),
      Order(status: '30', shippingStatus: '33'),
      Order(status: '40', shippingStatus: '56'),
      Order(status: '80'),
    ];
    for (final order in deliveries) {
      final visual = DeliveryStatusPresentation.fromOrder(order);
      expect(
        accentColors,
        contains(visual.color),
        reason: 'delivery ${order.status}/${order.shippingStatus}',
      );
      expect(
        surfaceColors,
        contains(visual.softColor),
        reason: 'delivery ${order.status}/${order.shippingStatus}',
      );
    }
  });

  test('溫層標籤依 Web 常溫灰、冷藏橘、冷凍藍顯示', () {
    expect(DeliveryTempLabel.labelFor(' 冷藏 '), '冷藏');
    expect(DeliveryTempLabel.colorFor(' 冷藏 '), AppColors.temperatureChilled);
    expect(DeliveryTempLabel.colorFor('3201'), AppColors.temperatureAmbient);
    expect(DeliveryTempLabel.colorFor('3202'), AppColors.temperatureChilled);
    expect(DeliveryTempLabel.colorFor('3203'), AppColors.temperatureFrozen);
  });

  test('核心訂單 UI 不可直接引用額外語意色', () {
    const files = [
      'lib/features/order/presentation/order_status_presentation.dart',
      'lib/features/order/presentation/order_card_widgets/order_action_buttons.dart',
      'lib/features/order/presentation/order_card_widgets/order_flag_banner.dart',
      'lib/features/order/presentation/order_card_widgets/order_info_column.dart',
      'lib/features/order/presentation/order_card_widgets/order_expanded_section.dart',
      'lib/features/order/presentation/order_card_widgets/status_info_box.dart',
    ];
    final forbidden = RegExp(
      r'AppColors\.(?:info|danger|warning|success|topGreen)\b',
    );

    for (final path in files) {
      expect(
        File(path).readAsStringSync(),
        isNot(matches(forbidden)),
        reason: path,
      );
    }
  });

  test('物流與售後步驟都使用 Material 圖示', () {
    for (final steps in [
      StepIndicator.logisticsSteps,
      StepIndicator.cancelledSteps,
      StepIndicator.afterSaleSteps,
      StepIndicator.afterSaleStepsRejected,
    ]) {
      for (final step in steps) {
        expect(step.icon.fontFamily, 'MaterialIcons', reason: step.label);
      }
    }
  });
}
