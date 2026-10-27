import 'package:hawkr_buyer/features/product/presentation/widgets/product_image_thumbnail_grid.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

void main() {
  Widget app({
    int imageCount = 2,
    int currentIndex = 0,
    ValueChanged<int>? onSelected,
  }) {
    return MaterialApp(
      home: Scaffold(
        body: SizedBox(
          width: 390,
          child: ProductImageThumbnailGrid(
            images: List.generate(
              imageCount,
              (index) => 'assets/hawkrlogo.png',
            ),
            currentIndex: currentIndex,
            onSelected: onSelected ?? (_) {},
          ),
        ),
      ),
    );
  }

  testWidgets('keeps all thumbnails in one horizontal row', (tester) async {
    await tester.pumpWidget(app(imageCount: 6));

    final first = find.byKey(const ValueKey('product-image-thumbnail-slot-0'));
    final fifth = find.byKey(const ValueKey('product-image-thumbnail-slot-4'));
    final list = tester.widget<ListView>(
      find.byKey(const ValueKey('product-image-thumbnail-list')),
    );

    expect(tester.getSize(first).width, closeTo(74.8, 0.01));
    expect(tester.getTopLeft(fifth).dy, tester.getTopLeft(first).dy);
    expect(list.scrollDirection, Axis.horizontal);
  });

  testWidgets('supports dragging the thumbnail list left and right', (
    tester,
  ) async {
    await tester.pumpWidget(app(imageCount: 9));

    final scrollable = find.descendant(
      of: find.byKey(const ValueKey('product-image-thumbnail-list')),
      matching: find.byType(Scrollable),
    );
    final initialOffset = tester
        .state<ScrollableState>(scrollable)
        .position
        .pixels;

    await tester.drag(
      find.byKey(const ValueKey('product-image-thumbnail-list')),
      const Offset(-260, 0),
    );
    await tester.pumpAndSettle();

    final draggedOffset = tester
        .state<ScrollableState>(scrollable)
        .position
        .pixels;
    expect(draggedOffset, greaterThan(initialOffset));

    await tester.drag(
      find.byKey(const ValueKey('product-image-thumbnail-list')),
      const Offset(260, 0),
    );
    await tester.pumpAndSettle();

    expect(
      tester.state<ScrollableState>(scrollable).position.pixels,
      lessThan(draggedOffset),
    );
  });

  testWidgets('selected thumbnail has brand border and changes main image', (
    tester,
  ) async {
    int? selectedIndex;
    await tester.pumpWidget(
      app(currentIndex: 0, onSelected: (index) => selectedIndex = index),
    );

    final selected = tester.widget<AnimatedContainer>(
      find.byKey(const ValueKey('product-image-thumbnail-0')),
    );
    final unselected = tester.widget<AnimatedContainer>(
      find.byKey(const ValueKey('product-image-thumbnail-1')),
    );
    final selectedBorder =
        (selected.decoration as BoxDecoration).border! as Border;
    final unselectedBorder =
        (unselected.decoration as BoxDecoration).border! as Border;
    expect(selectedBorder.top.color, isNot(Colors.transparent));
    expect(unselectedBorder.top.color, isNot(Colors.transparent));
    expect((selected.decoration as BoxDecoration).boxShadow, isNotEmpty);

    await tester.tap(
      find.byKey(const ValueKey('product-image-thumbnail-target-1')),
    );
    expect(selectedIndex, 1);
  });

  testWidgets('centers an incomplete row horizontally and vertically', (
    tester,
  ) async {
    await tester.pumpWidget(app(imageCount: 3));

    final first = find.byKey(const ValueKey('product-image-thumbnail-slot-0'));
    final third = find.byKey(const ValueKey('product-image-thumbnail-slot-2'));

    final leftSpace = tester.getTopLeft(first).dx;
    final rightSpace = 390 - tester.getTopRight(third).dx;
    final listCenter = tester.getCenter(
      find.byKey(const ValueKey('product-image-thumbnail-list')),
    );

    expect(leftSpace, closeTo(rightSpace, 0.01));
    expect(tester.getCenter(first).dy, tester.getCenter(third).dy);
    expect(tester.getCenter(first).dy, closeTo(listCenter.dy, 0.01));
  });

  testWidgets('caps the gallery at the web maximum of nine images', (
    tester,
  ) async {
    await tester.pumpWidget(app(imageCount: 12));
    await tester.drag(
      find.byKey(const ValueKey('product-image-thumbnail-list')),
      const Offset(-1000, 0),
    );
    await tester.pumpAndSettle();

    expect(
      find.byKey(const ValueKey('product-image-thumbnail-8')),
      findsOneWidget,
    );
    expect(
      find.byKey(const ValueKey('product-image-thumbnail-9')),
      findsNothing,
    );
  });
}
