import 'package:hawkr_seller/core/theme/app_theme.dart';
import 'package:hawkr_seller/features/product/data/product_repository.dart';
import 'package:hawkr_seller/features/product/presentation/product_preview_screen.dart';
import 'package:hawkr_seller/features/product/presentation/widgets/product_temperature_tags.dart';
import 'package:dio/dio.dart';
import 'package:flutter/material.dart';
import 'package:flutter_screenutil/flutter_screenutil.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:hooks_riverpod/hooks_riverpod.dart';
import 'package:shared_preferences/shared_preferences.dart';

class _DetailRepository extends ProductRepository {
  _DetailRepository(this.result) : super(Dio());
  final Map<String, dynamic> result;

  @override
  Future<Map<String, dynamic>> getProductDetail(dynamic productId) async =>
      result;
}

void main() {
  Future<void> pumpPreview(
    WidgetTester tester,
    Map<String, dynamic> response,
  ) async {
    SharedPreferences.setMockInitialValues({});
    await tester.pumpWidget(
      ProviderScope(
        overrides: [
          productRepositoryProvider.overrideWithValue(
            _DetailRepository(response),
          ),
        ],
        child: ScreenUtilInit(
          designSize: const Size(390, 844),
          enableScaleWH: () => false,
          enableScaleText: () => false,
          builder: (_, _) => MaterialApp(
            theme: AppTheme.light,
            home: const ProductPreviewScreen(
              product: {
                'id': 'product-1',
                'name': '溫層測試商品',
                'deliveryTemp': ['3203'],
              },
            ),
          ),
        ),
      ),
    );
    await tester.pumpAndSettle();
  }

  for (final entry in <String, Map<String, dynamic>>{
    'transport failure': {'code': -1, 'message': 'offline'},
    'partial detail': {
      'code': 0,
      'data': {'name': '溫層測試商品'},
    },
    'invalid temperature': {
      'code': 0,
      'data': {
        'deliveryTemp': {'label': '冷藏'},
      },
    },
    'failed envelope containing data': {
      'code': -1,
      'data': {
        'deliveryTemp': ['3202'],
      },
    },
  }.entries) {
    testWidgets('preview keeps known temperature on ${entry.key}', (
      tester,
    ) async {
      await pumpPreview(tester, entry.value);
      final badges = find.byType(ProductTemperatureTags);
      expect(
        find.descendant(of: badges, matching: find.text('冷凍')),
        findsOneWidget,
      );
      expect(
        find.descendant(of: badges, matching: find.text('尚未指定')),
        findsNothing,
      );
      expect(tester.takeException(), isNull);
    });
  }

  testWidgets('successful detail replaces the caller temperature', (
    tester,
  ) async {
    await pumpPreview(tester, {
      'code': 0,
      'data': {
        'deliveryTemp': ['3202'],
      },
    });
    final badges = find.byType(ProductTemperatureTags);
    expect(
      find.descendant(of: badges, matching: find.text('冷藏')),
      findsOneWidget,
    );
    expect(
      find.descendant(of: badges, matching: find.text('冷凍')),
      findsNothing,
    );
    expect(tester.takeException(), isNull);
  });

  testWidgets('explicit empty temperature clears the caller temperature', (
    tester,
  ) async {
    await pumpPreview(tester, {
      'code': 0,
      'data': {'deliveryTemp': []},
    });
    final badges = find.byType(ProductTemperatureTags);
    expect(
      find.descendant(of: badges, matching: find.text('尚未指定')),
      findsOneWidget,
    );
    expect(
      find.descendant(of: badges, matching: find.text('冷凍')),
      findsNothing,
    );
    expect(tester.takeException(), isNull);
  });
}
