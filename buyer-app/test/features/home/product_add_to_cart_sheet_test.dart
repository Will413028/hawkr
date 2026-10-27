import 'package:hawkr_buyer/core/theme/app_colors.dart';
import 'package:hawkr_buyer/core/theme/app_typography.dart';
import 'package:hawkr_buyer/features/home/presentation/home_widget/product_add_to_cart_sheet.dart';
import 'package:hawkr_buyer/features/product/data/product_repository.dart';
import 'package:hawkr_buyer/features/product/presentation/widgets/product_purchase_panel.dart';
import 'package:hawkr_buyer/shared/widgets/hk_icon.dart';
import 'package:flutter/material.dart';
import 'package:flutter/rendering.dart';
import 'package:flutter_screenutil/flutter_screenutil.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:hooks_riverpod/hooks_riverpod.dart';
import 'package:mocktail/mocktail.dart';

class _MockProductRepository extends Mock implements ProductRepository {}

void main() {
  testWidgets('uses exactly one visible drag handle with the app theme', (
    tester,
  ) async {
    await _pumpLoadedSheet(tester);

    final handles = _visibleDragHandles(tester);
    expect(
      handles,
      hasLength(1),
      reason:
          'The app theme already supplies a Material drag handle; the sheet '
          'must not paint a second handle in its content.',
    );
  });

  testWidgets('summary grows naturally and does not crop temperature content', (
    tester,
  ) async {
    await _pumpLoadedSheet(tester, width: 320, textScale: 1.4);

    final summaryRect = tester.getRect(
      find.byKey(const ValueKey('product-purchase-summary')),
    );
    final temperatureRowRect = tester.getRect(
      find.byKey(const ValueKey('product-purchase-delivery-metadata')),
    );
    final temperatureTagRect = tester.getRect(
      find.byKey(const ValueKey('product-purchase-temperature-3203')),
    );

    _expectRectInside(summaryRect, temperatureRowRect);
    _expectRectInside(summaryRect, temperatureTagRect);
    _expectTextHasNaturalHeight(tester, '配送溫層');
    _expectTextHasNaturalHeight(tester, '冷凍');
    expect(tester.takeException(), isNull);
  });

  testWidgets('shipping mark is legible and sheet copy follows type tiers', (
    tester,
  ) async {
    await _pumpLoadedSheet(tester);

    final shippingIconSize = tester.getSize(
      find.byKey(const ValueKey('product-purchase-carrier-3001')),
    );
    expect(shippingIconSize, const Size.square(24));
    final carrierIcon = tester.widget<HkIcon>(
      find.descendant(
        of: find.byKey(const ValueKey('product-purchase-carrier-3001')),
        matching: find.byType(HkIcon),
      ),
    );
    expect(carrierIcon.name, HkIconName.truck);
    final carrierFrameRect = tester.getRect(
      find.byKey(const ValueKey('product-purchase-carrier-3001')),
    );
    final carrierIconRect = tester.getRect(
      find.descendant(
        of: find.byKey(const ValueKey('product-purchase-carrier-3001')),
        matching: find.byType(HkIcon),
      ),
    );
    expect(
      carrierIconRect.size,
      Size.square(carrierIcon.size!),
      reason: 'the frame must not squeeze the mark below its requested size',
    );
    _expectRectInside(carrierFrameRect.deflate(1), carrierIconRect);
    final metadataIcon = tester.widget<HkIcon>(
      find
          .descendant(
            of: find.byKey(
              const ValueKey('product-purchase-delivery-metadata'),
            ),
            matching: find.byType(HkIcon),
          )
          .first,
    );
    expect(metadataIcon.name, HkIconName.document);
    expect(metadataIcon.size, 16);

    _expectTextTier(tester, '冷凍烏魚子', AppTypography.productName);
    _expectTextTier(tester, '配送溫層', AppTypography.meta);
    _expectTextTier(
      tester,
      '冷凍',
      const TextStyle(
        fontSize: 11,
        height: 16 / 11,
        letterSpacing: 0,
        fontWeight: FontWeight.w700,
        color: AppColors.temperatureFrozen,
      ),
    );
    _expectTextTier(tester, '暫無評價', AppTypography.metaMuted);
    _expectTextTier(tester, '選擇規格', AppTypography.sectionTitle);
    _expectTextTier(
      tester,
      '約300g/包',
      AppTypography.meta.copyWith(height: 1.2, fontWeight: FontWeight.w700),
      compareColor: false,
    );
    _expectTextTier(tester, '數量', AppTypography.body);
    _expectTextTier(tester, '庫存：100 件', AppTypography.body);
    _expectTextTier(
      tester,
      '加入購物車',
      AppTypography.buttonLarge.copyWith(
        height: 1,
        color: AppColors.commerceCartForeground,
      ),
    );
    _expectTextTier(
      tester,
      '立即結帳',
      AppTypography.buttonLarge.copyWith(
        height: 1,
        color: AppColors.commerceCheckoutForeground,
      ),
    );
    expect(find.byType(ProductPurchaseSelector), findsOneWidget);
    expect(find.byType(ProductPurchaseFooter), findsOneWidget);
  });

  testWidgets('self-owned product detail fails closed before cart mutation', (
    tester,
  ) async {
    final semantics = tester.ensureSemantics();
    try {
      await _pumpLoadedSheet(
        tester,
        productDetail: const <String, dynamic>{
          ..._productDetail,
          'selfOwned': true,
        },
      );

      expect(
        find.byKey(const ValueKey('product-purchase-self-owned-notice')),
        findsOneWidget,
      );
      expect(find.bySemanticsLabel('自家商品不可購買'), findsOneWidget);
      for (final key in const [
        'product-purchase-add',
        'product-purchase-buy',
      ]) {
        final action = find.byKey(ValueKey(key));
        final detector = tester.widget<GestureDetector>(
          find.descendant(of: action, matching: find.byType(GestureDetector)),
        );
        expect(detector.onTap, isNull, reason: '$key must stay disabled');
      }
    } finally {
      semantics.dispose();
    }
  });

  for (final width in <double>[320, 390, 768]) {
    testWidgets('does not overflow at ${width.toInt()}px and 1.4x text', (
      tester,
    ) async {
      await _pumpLoadedSheet(tester, width: width, textScale: 1.4);

      expect(find.text('冷凍烏魚子'), findsOneWidget);
      expect(find.text('冷凍'), findsOneWidget);
      expect(find.text('約300g/包'), findsOneWidget);
      final specChip = tester.getSize(
        find.byKey(const ValueKey('product-purchase-spec-11')),
      );
      expect(specChip.width, greaterThanOrEqualTo(96));
      expect(specChip.width, lessThan(220));
      expect(tester.takeException(), isNull);
    });
  }
}

Future<void> _pumpLoadedSheet(
  WidgetTester tester, {
  double width = 390,
  double textScale = 1,
  Map<String, dynamic> productDetail = _productDetail,
}) async {
  final height = width >= 600 ? 1024.0 : 844.0;
  tester.view.devicePixelRatio = 1;
  tester.view.physicalSize = Size(width, height);
  addTearDown(tester.view.resetPhysicalSize);
  addTearDown(tester.view.resetDevicePixelRatio);

  final repository = _MockProductRepository();
  when(
    () => repository.getProductFullInfo('product-42'),
  ).thenAnswer((_) async => productDetail);

  await tester.pumpWidget(
    ProviderScope(
      overrides: [productRepositoryProvider.overrideWithValue(repository)],
      child: ScreenUtilInit(
        designSize: const Size(390, 844),
        enableScaleWH: () => false,
        enableScaleText: () => false,
        builder: (_, __) => MaterialApp(
          theme: ThemeData(
            useMaterial3: true,
            textTheme: AppTypography.textTheme,
            bottomSheetTheme: const BottomSheetThemeData(
              backgroundColor: AppColors.bgElevated,
              surfaceTintColor: Colors.transparent,
              showDragHandle: true,
              dragHandleColor: AppColors.dragHandle,
            ),
          ),
          builder: (context, child) => MediaQuery(
            data: MediaQuery.of(
              context,
            ).copyWith(textScaler: TextScaler.linear(textScale)),
            child: child!,
          ),
          home: const _SheetLauncher(),
        ),
      ),
    ),
  );

  await tester.tap(find.byKey(const ValueKey('open-product-sheet')));
  await tester.pumpAndSettle();
}

class _SheetLauncher extends ConsumerWidget {
  const _SheetLauncher();

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    return Scaffold(
      body: Center(
        child: FilledButton(
          key: const ValueKey('open-product-sheet'),
          onPressed: () => showProductAddToCartSheet(
            context,
            ref,
            skuId: 'product-42',
            productName: '冷凍烏魚子',
            imageUrl: '',
            initialPrice: 260,
          ),
          child: const Text('開啟商品規格'),
        ),
      ),
    );
  }
}

const _productDetail = <String, dynamic>{
  'shippingMethods': <Map<String, dynamic>>[
    <String, dynamic>{'type': '3001'},
  ],
  'deliveryTemp': <String>['3203'],
  'rating': 0,
  'reviews': <Map<String, dynamic>>[],
  'skus': <Map<String, dynamic>>[
    <String, dynamic>{
      'id': '11',
      'skuName': '約300g/包',
      'price': 260,
      'stockQty': 100,
      'unit': '包',
    },
  ],
};

List<Size> _visibleDragHandles(WidgetTester tester) {
  final sheet = find.byType(BottomSheet);
  expect(sheet, findsOneWidget);

  final result = <Size>[];
  for (final element
      in find
          .descendant(of: sheet, matching: find.byType(Container))
          .evaluate()) {
    final widget = element.widget as Container;
    final decoration = widget.decoration;
    if (decoration is! BoxDecoration ||
        decoration.color == null ||
        decoration.borderRadius == null) {
      continue;
    }
    final size = tester.getSize(find.byWidget(widget));
    if (size.width >= 24 &&
        size.width <= 48 &&
        size.height >= 2 &&
        size.height <= 6) {
      result.add(size);
    }
  }
  return result;
}

void _expectTextHasNaturalHeight(WidgetTester tester, String label) {
  final paragraph = tester.renderObject<RenderParagraph>(find.text(label));
  final naturalHeight = paragraph.getMinIntrinsicHeight(paragraph.size.width);
  expect(
    paragraph.size.height,
    greaterThanOrEqualTo(naturalHeight - 0.1),
    reason: '$label must receive its natural scaled line height',
  );
}

void _expectTextTier(
  WidgetTester tester,
  String label,
  TextStyle expected, {
  bool compareColor = true,
}) {
  final actual = tester.widget<Text>(find.text(label)).style!;
  expect(actual.fontFamily, expected.fontFamily, reason: '$label font family');
  expect(actual.fontSize, expected.fontSize, reason: '$label font size');
  expect(actual.fontWeight, expected.fontWeight, reason: '$label font weight');
  expect(actual.height, expected.height, reason: '$label line height');
  expect(
    actual.letterSpacing,
    expected.letterSpacing,
    reason: '$label letter spacing',
  );
  if (compareColor) {
    expect(actual.color, expected.color, reason: '$label color');
  }
}

void _expectRectInside(Rect outer, Rect inner) {
  const tolerance = 0.1;
  expect(inner.left, greaterThanOrEqualTo(outer.left - tolerance));
  expect(inner.top, greaterThanOrEqualTo(outer.top - tolerance));
  expect(inner.right, lessThanOrEqualTo(outer.right + tolerance));
  expect(inner.bottom, lessThanOrEqualTo(outer.bottom + tolerance));
}
