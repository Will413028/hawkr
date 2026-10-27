import 'dart:io';

import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:hawkr_buyer/shared/widgets/hk_icon.dart';

const _emptyStateIcons = <HkIconName>[
  HkIconName.emptyMemberCard,
  HkIconName.emptyCoupon,
  HkIconName.emptyAddress,
  HkIconName.emptyChat,
  HkIconName.emptySearch,
  HkIconName.emptyMerchants,
  HkIconName.emptyPosts,
  HkIconName.emptyProducts,
  HkIconName.emptyNotifications,
  HkIconName.emptyAfterSale,
  HkIconName.emptyOrders,
  HkIconName.emptyReviews,
  HkIconName.emptyFavorites,
  HkIconName.emptyRecentViews,
  HkIconName.emptyMerchantSearch,
  HkIconName.emptySubscriptions,
];

void main() {
  testWidgets('every empty-state icon is a Material icon', (tester) async {
    final glyphs = <IconData>{};
    for (final name in _emptyStateIcons) {
      await tester.pumpWidget(MaterialApp(home: HkIcon(name)));
      final icon = tester.widget<Icon>(find.byType(Icon)).icon!;
      expect(icon.fontFamily, 'MaterialIcons', reason: name.name);
      glyphs.add(icon);
    }
    expect(
      glyphs.length,
      greaterThanOrEqualTo(_emptyStateIcons.length - 2),
      reason: 'empty states should stay visually distinguishable',
    );
  });

  test('feature empty states never fall back to legacy generic assets', () {
    final emptyAssetImage = RegExp(
      r'(SvgPicture|Image)\.asset\([^)]*empty',
      caseSensitive: false,
    );
    final violations = <String>[];
    for (final entity in Directory('lib').listSync(recursive: true)) {
      if (entity is! File || !entity.path.endsWith('.dart')) continue;
      final source = entity.readAsStringSync();
      if (emptyAssetImage.hasMatch(source)) {
        violations.add(entity.path);
      }
    }

    expect(
      violations,
      isEmpty,
      reason: 'Use a semantic HkIconName empty-state icon: $violations',
    );
  });
}
