import 'package:hawkr_buyer/shared/widgets/product_grid_spec.dart';
import 'package:flutter_test/flutter_test.dart';

void main() {
  test('matches the Web masonry breakpoints', () {
    const expectations = <({double width, int columns})>[
      (width: 320, columns: 1),
      (width: 360, columns: 1),
      (width: 361, columns: 2),
      (width: 390, columns: 2),
      (width: 600, columns: 2),
      (width: 680, columns: 2),
      (width: 681, columns: 3),
      (width: 768, columns: 3),
      (width: 1024, columns: 4),
      (width: 1440, columns: 4),
    ];

    for (final entry in expectations) {
      expect(
        ProductGridSpec.forWidth(entry.width).columnCount,
        entry.columns,
        reason: '${entry.width}px should use ${entry.columns} columns',
      );
    }
  });

  test('keeps 4:3 media plus a 72px default information panel', () {
    for (final width in <double>[320, 390, 680, 768, 1024, 1440]) {
      final spec = ProductGridSpec.forWidth(width);
      expect(
        spec.mainAxisExtent,
        closeTo(
          spec.itemWidth / ProductGridSpec.mediaAspectRatio +
              ProductGridSpec.bodyExtent,
          0.001,
        ),
      );
      expect(spec.delegate.crossAxisCount, spec.columnCount);
      expect(spec.delegate.mainAxisExtent, spec.mainAxisExtent);
    }
  });

  test('centres wide grids inside the Web max content width', () {
    final spec = ProductGridSpec.forWidth(1440);
    expect(1440 - spec.horizontalInset * 2, 1120);
    expect(spec.padding.horizontal, spec.horizontalInset * 2);
  });

  test('grid body grows with supported accessibility text scale', () {
    final normal = ProductGridSpec.forWidth(390);
    final enlarged = ProductGridSpec.forWidth(390, textScale: 1.4);

    expect(
      enlarged.mainAxisExtent - normal.mainAxisExtent,
      closeTo(ProductGridSpec.bodyTextScaleGrowth * 0.4, 0.001),
    );
    expect(
      ProductGridSpec.forWidth(390, textScale: 2).mainAxisExtent,
      enlarged.mainAxisExtent,
    );
  });
}
