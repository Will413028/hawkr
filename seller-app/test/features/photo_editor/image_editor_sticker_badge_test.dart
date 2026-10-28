import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:hawkr_seller/features/photo_editor/presentation/image_editor_models.dart';
import 'package:hawkr_seller/features/photo_editor/presentation/image_editor_sticker_badge.dart';

void main() {
  for (final badge in StickerBadge.values) {
    testWidgets('${badge.name} draws its label and icon without an image', (
      tester,
    ) async {
      await tester.pumpWidget(
        MaterialApp(home: Center(child: StickerBadgeView(badge, size: 60))),
      );

      expect(find.text(badge.label), findsOneWidget);
      expect(tester.widget<Icon>(find.byType(Icon)).icon, badge.icon);
      expect(find.byType(Image), findsNothing);
      expect(tester.getSize(find.byType(StickerBadgeView)), const Size(60, 60));
    });
  }

  test('sticker layer keeps its badge through copyWith', () {
    final layer = StickerLayer(badge: StickerBadge.fresh);
    final moved = layer.copyWith(x: 10, y: 20);

    expect(moved.badge, StickerBadge.fresh);
    expect(moved.isBadgeSticker, isTrue);
    expect(StickerLayer(emoji: '🏆').isBadgeSticker, isFalse);
  });
}
