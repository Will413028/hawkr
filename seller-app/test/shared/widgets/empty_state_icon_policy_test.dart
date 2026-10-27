import 'dart:io';

import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:hawkr_seller/shared/widgets/hk_icon.dart';

const _emptyStateIcons = <HkIconName>[
  HkIconName.box,
  HkIconName.goods,
  HkIconName.users,
  HkIconName.post,
  HkIconName.coupon,
  HkIconName.notification,
  HkIconName.chat,
  HkIconName.chart,
  HkIconName.document,
  HkIconName.search,
  HkIconName.star,
  HkIconName.truck,
];

void main() {
  testWidgets('every seller empty-state icon is its own Material glyph', (
    tester,
  ) async {
    await tester.pumpWidget(
      MaterialApp(
        home: Wrap(
          children: [for (final name in _emptyStateIcons) HkIcon(name)],
        ),
      ),
    );

    final glyphs = tester
        .widgetList<Icon>(find.byType(Icon))
        .map((icon) => icon.icon)
        .toList();
    expect(glyphs, hasLength(_emptyStateIcons.length));
    expect(glyphs.every((glyph) => glyph?.fontFamily == 'MaterialIcons'), true);
    expect(glyphs.toSet(), hasLength(_emptyStateIcons.length));
  });

  test('lib does not reference the removed UI-icon asset directories', () {
    const removedDirectories = [
      'assets/icons/',
      'assets/images/',
      'assets/common/',
      'assets/bar_icon/',
      'assets/order_icon/',
      'assets/post_icon/',
      'assets/notification/',
      'assets/after_sales/',
      'assets/coupon/',
      'assets/Auth_Screen/',
      'assets/Customer_status_Setting/',
      'assets/setting_icon/',
      'assets/product/',
      'assets/lottie/',
    ];
    final violations = <String>[];
    for (final entity in Directory('lib').listSync(recursive: true)) {
      if (entity is! File || !entity.path.endsWith('.dart')) continue;
      final source = entity.readAsStringSync();
      if (removedDirectories.any(source.contains)) {
        violations.add(entity.path);
      }
    }

    expect(
      violations,
      isEmpty,
      reason:
          'Use HkEmptyState or HkIcon instead of removed assets: $violations',
    );
  });
}
