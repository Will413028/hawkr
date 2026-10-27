import 'dart:ui' show Tristate;

import 'package:hawkr_seller/core/theme/app_theme.dart';
import 'package:hawkr_seller/features/product/data/product_repository.dart';
import 'package:hawkr_seller/features/product/presentation/product_preview_screen.dart';
import 'package:hawkr_seller/features/product/presentation/product_reviews_screen.dart';
import 'package:hawkr_seller/features/product/presentation/product_setup_picker/product_setup_picker_screen.dart';
import 'package:hawkr_seller/features/product/presentation/product_setup_picker/picker_l3_content_panel.dart';
import 'package:hawkr_seller/features/product/presentation/product_setup_picker/picker_category_search_row.dart';
import 'package:hawkr_seller/features/product/presentation/product_setup_info_screen.dart';
import 'package:hawkr_seller/features/product/presentation/product_setup_image_screen.dart';
import 'package:hawkr_seller/features/product/presentation/product_setup_tag_screen.dart';
import 'package:hawkr_seller/features/seller/data/seller_repository.dart';
import 'package:hawkr_seller/features/product/presentation/product_setup_spec_screen.dart';
import 'package:dio/dio.dart';
import 'package:flutter/material.dart';
import 'package:flutter/rendering.dart';
import 'package:flutter_screenutil/flutter_screenutil.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:hooks_riverpod/hooks_riverpod.dart';
import 'package:shared_preferences/shared_preferences.dart';

const _longName = '範例生鮮友善耕作當季水果綜合家庭分享禮盒';

class _FixtureSellerRepository implements SellerRepository {
  @override
  dynamic noSuchMethod(Invocation invocation) => super.noSuchMethod(invocation);
}

class _FixtureRepository extends ProductRepository {
  _FixtureRepository() : super(Dio());

  @override
  Future<Map<String, dynamic>> getProductDetail(dynamic productId) async => {
    'code': 0,
    'data': {
      'name': _longName,
      'origin': '臺灣南投縣埔里鎮友善耕作農場',
      'variants': [
        {
          'skuName': '家庭分享禮盒大份量冷藏配送規格',
          'price': 99999999,
          'salePrice': 88888888,
          'stockQty': 99999999,
        },
      ],
      'deliveryTemp': ['3201', '3202', '3203'],
    },
  };

  @override
  Future<Map<String, dynamic>> getPublicProductRatingSummary(
    dynamic productId,
  ) async => {
    'code': 0,
    'data': {'averageRating': 4.8, 'totalCount': 12345},
  };

  @override
  Future<List<Map<String, dynamic>>> listPublicProductReviews(
    String sellerId,
    dynamic skuId,
  ) async => [
    {
      'username': '測試買家名稱很長很長但仍必須保留日期',
      'rating': 5,
      'createdAt': '2026-09-22T10:00:00Z',
      'content': '配送準時，水果新鮮，包裝完整。',
    },
  ];

  @override
  Future<List<Map<String, dynamic>>> getSellerProductReviewsPublic(
    String sellerId,
    dynamic skuId,
  ) => listPublicProductReviews(sellerId, skuId);

  @override
  Future<List<Map<String, dynamic>>> fetchSellerCategoryTree() =>
      fetchCategories();

  @override
  Future<List<Map<String, dynamic>>> fetchCategories({
    bool forceRefresh = false,
  }) async => [
    {
      'id': 1,
      'name': '蔬菜',
      'children': [
        {
          'id': 11,
          'name': '友善耕作當季蔬菜',
          'children': [
            {'id': 111, 'name': '家庭分享綜合蔬菜箱長名稱'},
          ],
        },
      ],
    },
  ];

  @override
  Future<List<Map<String, dynamic>>> getSellerShippingMethods() async => [];
}

Future<void> _pump(
  WidgetTester tester,
  Widget screen, {
  required Size size,
  double textScale = 1.4,
}) async {
  SharedPreferences.setMockInitialValues({});
  tester.view.devicePixelRatio = 1;
  tester.view.physicalSize = size;
  addTearDown(tester.view.resetDevicePixelRatio);
  addTearDown(tester.view.resetPhysicalSize);
  await tester.pumpWidget(
    ProviderScope(
      overrides: [
        productRepositoryProvider.overrideWithValue(_FixtureRepository()),
        sellerRepositoryProvider.overrideWithValue(_FixtureSellerRepository()),
      ],
      child: ScreenUtilInit(
        designSize: const Size(390, 844),
        enableScaleWH: () => false,
        enableScaleText: () => false,
        builder: (_, _) => MaterialApp(
          theme: AppTheme.light,
          builder: (context, child) => MediaQuery(
            data: MediaQuery.of(
              context,
            ).copyWith(textScaler: TextScaler.linear(textScale)),
            child: child!,
          ),
          home: screen,
        ),
      ),
    ),
  );
  await tester.pumpAndSettle();
}

void main() {
  final screens = <String, Widget>{
    'picker': const ProductSetupPickerScreen(initialType: 'Vegetables'),
    'reviews': const ProductReviewsScreen(
      sellerId: '0',
      skuId: 'fixture-product',
      productName: _longName,
    ),
    'preview': const ProductPreviewScreen(
      product: {
        'id': 'fixture-product',
        'storeId': 'fixture-store',
        'name': _longName,
      },
    ),
    'info': const ProductSetupInfoScreen(
      initialProductName: _longName,
      initialCategory: '當季蔬果與生鮮食品',
      initialSubCategory: '綜合水果家庭分享禮盒',
    ),
    'images': const ProductSetupImageScreen(
      productName: _longName,
      category: '蔬果',
      subCategory: '水果',
    ),
    'tags': const ProductSetupTagScreen(
      images: [],
      productName: _longName,
      category: '蔬果',
      subCategory: '水果',
    ),
    'specs': const ProductSetupSpecScreen(
      images: [],
      productName: _longName,
      category: '蔬果',
      subCategory: '水果',
      selectedTags: {},
    ),
  };

  for (final entry in screens.entries) {
    for (final size in [
      const Size(320, 700),
      const Size(402, 874),
      const Size(440, 956),
      const Size(874, 402),
    ]) {
      for (final scale in [1.0, 1.4]) {
        testWidgets('${entry.key} fits ${size.width} at ${scale}x text', (
          tester,
        ) async {
          await _pump(tester, entry.value, size: size, textScale: scale);
          expect(tester.takeException(), isNull);
          if (entry.key == 'picker') {
            await tester.scrollUntilVisible(
              find.text('家庭分享綜合蔬菜箱長名稱'),
              100,
              scrollable: find.descendant(
                of: find.byType(PickerL3ContentPanel),
                matching: find.byType(Scrollable),
              ),
            );
            expect(find.text('家庭分享綜合蔬菜箱長名稱'), findsOneWidget);
            expect(tester.takeException(), isNull);
          }
          if (entry.key == 'preview') {
            for (final amount in [r'$88888888', r'$99999999']) {
              final finder = find.text(amount);
              expect(finder, findsOneWidget);
              expect(
                tester.renderObject<RenderParagraph>(finder).didExceedMaxLines,
                isFalse,
              );
            }
          }
        });
      }
    }
  }

  testWidgets(
    'picker search keeps one surface while focused and submits typed text',
    (tester) async {
      final controller = TextEditingController();
      addTearDown(controller.dispose);
      var submitted = '';
      var submissions = 0;
      await _pump(
        tester,
        Scaffold(
          body: PickerCategorySearchRow(
            majors: const ['Vegetables'],
            topTabLabels: const {'Vegetables': '蔬菜'},
            activeTop: 'Vegetables',
            onTopChanged: (_) {},
            searchCtrl: controller,
            onSearch: () {
              submitted = controller.text;
              submissions++;
            },
          ),
        ),
        size: const Size(320, 700),
      );
      final field = find.byType(TextField);
      expect(field, findsOneWidget);
      for (final focused in [false, true]) {
        if (focused) {
          await tester.tap(field);
          await tester.pumpAndSettle();
        }
        final decorator = tester.widget<InputDecorator>(
          find.byType(InputDecorator),
        );
        expect(decorator.isFocused, focused);
        expect(decorator.decoration.filled, isFalse);
        for (final border in [
          decorator.decoration.border,
          decorator.decoration.enabledBorder,
          decorator.decoration.focusedBorder,
          decorator.decoration.disabledBorder,
          decorator.decoration.errorBorder,
          decorator.decoration.focusedErrorBorder,
        ]) {
          expect(border, InputBorder.none);
        }
      }
      await tester.enterText(field, '家庭分享蔬菜');
      await tester.testTextInput.receiveAction(TextInputAction.done);
      await tester.pumpAndSettle();
      expect(submitted, '家庭分享蔬菜');
      expect(submissions, 1);
      expect(find.byType(EditableText), findsOneWidget);
      expect(tester.takeException(), isNull);
    },
  );

  testWidgets('certification dialog remains scrollable above the keyboard', (
    tester,
  ) async {
    await _pump(tester, screens['tags']!, size: const Size(320, 700));
    await tester.tap(find.text('有機農產品標章'));
    await tester.pumpAndSettle();
    tester.view.viewInsets = const FakeViewPadding(bottom: 300);
    addTearDown(tester.view.resetViewInsets);
    await tester.pumpAndSettle();
    expect(tester.takeException(), isNull);
    final confirmation = tester.getSemantics(find.bySemanticsLabel('確認認證'));
    expect(
      confirmation.getSemanticsData().flagsCollection.isEnabled,
      Tristate.isFalse,
    );
    await tester.ensureVisible(find.byType(TextField));
    await tester.enterText(find.byType(TextField), '1-001-123456');
    await tester.pump();
    expect(find.text('1-001-123456'), findsOneWidget);
    expect(
      tester
          .getSemantics(find.bySemanticsLabel('確認認證'))
          .getSemanticsData()
          .flagsCollection
          .isEnabled,
      Tristate.isTrue,
    );
    final cancel = find.bySemanticsLabel('取消認證');
    await tester.ensureVisible(cancel);
    expect(tester.getSize(cancel).height, greaterThanOrEqualTo(48));
    await tester.tap(cancel);
    await tester.pumpAndSettle();
    expect(find.byType(Dialog), findsNothing);
    expect(tester.takeException(), isNull);
  });

  for (final size in [const Size(320, 700), const Size(874, 402)]) {
    testWidgets('info keyboard fits $size and focused field stays reachable', (
      tester,
    ) async {
      await _pump(tester, screens['info']!, size: size);
      tester.view.viewInsets = FakeViewPadding(
        bottom: size.width < 400 ? 300 : 180,
      );
      addTearDown(tester.view.resetViewInsets);
      await tester.pumpAndSettle();
      expect(tester.takeException(), isNull);
      final name = find.widgetWithText(TextField, _longName);
      await tester.ensureVisible(name);
      await tester.tap(name);
      await tester.pumpAndSettle();
      expect(
        tester.getRect(name).bottom,
        lessThanOrEqualTo(size.height - tester.view.viewInsets.bottom),
      );
      expect(tester.takeException(), isNull);
    });
  }

  testWidgets('specs landscape keyboard leaves the price form reachable', (
    tester,
  ) async {
    await _pump(tester, screens['specs']!, size: const Size(874, 402));
    tester.view.viewInsets = const FakeViewPadding(bottom: 180);
    addTearDown(tester.view.resetViewInsets);
    await tester.pumpAndSettle();
    expect(tester.takeException(), isNull);
    final grade = find.widgetWithText(TextField, '填寫規格 ex:S');
    await tester.ensureVisible(grade);
    await tester.tap(grade);
    await tester.pumpAndSettle();
    expect(tester.getRect(grade).bottom, lessThanOrEqualTo(222));
    expect(tester.takeException(), isNull);
  });
}
