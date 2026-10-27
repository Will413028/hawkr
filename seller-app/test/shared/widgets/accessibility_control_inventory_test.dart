import 'dart:io';

import 'package:flutter_test/flutter_test.dart';

void main() {
  test('toolbar inventory keeps 17 named controls on HkTapTarget', () {
    final source = File(
      'lib/shared/widgets/rich_text_editor_toolbar.dart',
    ).readAsStringSync();
    const labels = <String>[
      '新增內容區塊',
      '粗體',
      '斜體',
      '底線',
      '插入連結',
      '文字顏色',
      '靠左對齊',
      '置中對齊',
      '靠右對齊',
      '文字樣式',
      '插入圖片',
      '插入表格',
      '插入項目符號清單',
      '插入編號清單',
      '撤銷',
      '重做',
      '隱藏鍵盤',
    ];

    for (final label in labels) {
      expect(source, contains("'$label'"), reason: label);
    }
    expect(source, isNot(contains('GestureDetector(')));
    expect(
      RegExp(r'height:\s*48').allMatches(source).length,
      greaterThanOrEqualTo(2),
    );
  });

  test('comments inventory leaves only full-row long-press gestures raw', () {
    final source = File(
      'lib/shared/widgets/comments_bottom_sheet.dart',
    ).readAsStringSync();

    for (final label in <String>[
      '關閉留言',
      '取消回覆',
      '送出留言',
      '回覆 \$displayName 的留言',
      '喜歡 \$displayName 的留言',
      '喜歡 \$displayName 的回覆',
    ]) {
      expect(source, contains(label), reason: label);
    }
    expect(RegExp(r'GestureDetector\(').allMatches(source).length, 2);
    expect(source, contains("key: const ValueKey('send-comment-action')"));
  });

  test('notification and customer small-control hotspots stay migrated', () {
    final notifications = File(
      'lib/features/notification/presentation/notifications_screen.dart',
    ).readAsStringSync();
    final customers = File(
      'lib/features/customer/presentation/customer_detail_screen.dart',
    ).readAsStringSync();

    expect(
      notifications,
      contains("key: ValueKey('notification-row-\${n.id}')"),
    );
    expect(notifications, contains('checked: widget.isDeleteMode'));
    expect(notifications, contains('button: !widget.isDeleteMode'));
    final checkboxStart = notifications.indexOf(
      'if (widget.isDeleteMode)\n',
      notifications.indexOf('checked: widget.isDeleteMode'),
    );
    final checkboxEnd = notifications.indexOf('Stack(', checkboxStart);
    expect(checkboxStart, greaterThanOrEqualTo(0));
    expect(checkboxEnd, greaterThan(checkboxStart));
    expect(
      notifications.substring(checkboxStart, checkboxEnd),
      isNot(contains('GestureDetector(')),
    );

    for (final label in <String>['複製顧客 ID', '管理顧客標籤', '新增顧客標籤', '完成標籤管理']) {
      expect(customers, contains("'$label'"), reason: label);
    }
    final copyIcon = customers.indexOf('AppIcons.copyOutlined');
    final copyTarget = customers.lastIndexOf('HkTapTarget(', copyIcon);
    expect(copyIcon, greaterThanOrEqualTo(0));
    expect(copyTarget, greaterThanOrEqualTo(0));
    expect(copyIcon - copyTarget, lessThan(600));

    final closeControl = File(
      'lib/shared/widgets/dialog_close_button.dart',
    ).readAsStringSync();
    expect(closeControl, contains('HkTapTarget('));
    expect(closeControl, isNot(contains('GestureDetector(')));
  });

  test('order and settings guideline fixes stay in production code', () {
    final orderCard = File(
      'lib/features/order/presentation/order_card.dart',
    ).readAsStringSync();
    final orderPill = File(
      'lib/features/order/presentation/order_card_widgets/pill_button.dart',
    ).readAsStringSync();
    final address = File(
      'lib/features/settings/presentation/shipping_settings/widgets/'
      'address_autocomplete_field.dart',
    ).readAsStringSync();

    expect(orderCard, contains('excludeFromSemantics: true'));
    expect(orderPill, contains('HkTapTarget('));
    expect(orderPill, contains('semanticLabel: label'));
    expect(address, contains('minHeight: HkControl.minTap'));
    expect(address, contains('child: DecoratedBox('));
  });
}
