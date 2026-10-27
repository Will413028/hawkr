import 'package:hawkr_seller/core/theme/app_theme.dart';
import 'package:hawkr_seller/features/post/presentation/widgets/page_shop_tab.dart';
import 'package:flutter/material.dart';
import 'package:flutter/rendering.dart';
import 'package:flutter_screenutil/flutter_screenutil.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:hooks_riverpod/hooks_riverpod.dart';

void main() {
  for (final width in [320.0, 393.0, 402.0, 414.0, 440.0]) {
    testWidgets('seller product card fits ${width.toInt()}pt at 1.4x text', (
      tester,
    ) async {
      tester.view.devicePixelRatio = 1;
      tester.view.physicalSize = Size(width, 956);
      addTearDown(tester.view.resetDevicePixelRatio);
      addTearDown(tester.view.resetPhysicalSize);

      await tester.pumpWidget(
        ProviderScope(
          child: ScreenUtilInit(
            designSize: const Size(414, 896),
            enableScaleWH: () => false,
            enableScaleText: () => false,
            minTextAdapt: false,
            splitScreenMode: true,
            builder: (_, _) => MaterialApp(
              theme: AppTheme.light,
              builder: (context, child) => MediaQuery(
                data: MediaQuery.of(
                  context,
                ).copyWith(textScaler: const TextScaler.linear(1.4)),
                child: child!,
              ),
              home: Scaffold(
                body: Align(
                  alignment: Alignment.topCenter,
                  child: Padding(
                    padding: const EdgeInsets.all(16),
                    child: buildSellerProductManagementCardForTest(
                      product: const <String, dynamic>{
                        'id': 'fixture-product',
                        'productName': '[範例生鮮優選] 文旦綜合家庭分享大份量禮盒',
                        'priceMin': 88888888,
                        'priceMax': 99999999,
                        'salePriceMin': 88888888,
                        'salePriceMax': 99999999,
                        'listPriceMin': 99999999,
                        'listPriceMax': 111111111,
                        'status': '4004',
                      },
                    ),
                  ),
                ),
              ),
            ),
          ),
        ),
      );
      await tester.pump();

      expect(find.text('下架'), findsOneWidget);
      expect(find.text('刪除'), findsOneWidget);
      expect(find.text('編輯'), findsOneWidget);
      for (final label in ['下架', '刪除', '編輯']) {
        final actionSize = tester.getSize(
          find.byKey(ValueKey('seller-product-action-$label')),
        );
        expect(actionSize.height, greaterThanOrEqualTo(48));
        expect(actionSize.width, greaterThanOrEqualTo(48));
        expect(tester.getSize(find.text(label)).height, lessThanOrEqualTo(32));
      }
      expect(tester.takeException(), isNull);
      for (final key in [
        'seller-product-price',
        'seller-product-original-price',
      ]) {
        final price = find.byKey(ValueKey(key));
        final text = tester.widget<Text>(price);
        expect(text.maxLines, isNull);
        expect(text.overflow, isNot(TextOverflow.ellipsis));
        expect(
          tester.renderObject<RenderParagraph>(price).didExceedMaxLines,
          isFalse,
        );
      }
      final share = tester.getSize(
        find.byKey(const ValueKey('seller-product-share')),
      );
      expect(share.width, greaterThanOrEqualTo(48));
      expect(share.height, greaterThanOrEqualTo(48));

      // Cancelling all three confirmations keeps the listed product intact;
      // no product repository or mutation is needed by this fixture.
      for (final action in ['下架', '刪除', '編輯']) {
        await tester.tap(find.byKey(ValueKey('seller-product-action-$action')));
        await tester.pumpAndSettle();
        final cancel = find.descendant(
          of: find.byType(Dialog),
          matching: find.text('取消'),
        );
        expect(cancel, findsOneWidget);
        final button = find.ancestor(
          of: cancel,
          matching: find.byType(InkWell),
        );
        expect(tester.getSize(button).height, greaterThanOrEqualTo(48));
        await tester.tap(cancel);
        await tester.pumpAndSettle();
        expect(find.byType(Dialog), findsNothing);
        expect(find.text('下架'), findsOneWidget);
        expect(tester.takeException(), isNull);
      }
    });
  }
}
