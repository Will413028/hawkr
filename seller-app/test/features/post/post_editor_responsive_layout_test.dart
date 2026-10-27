import 'package:hawkr_seller/features/seller/data/current_store.dart';
import 'package:hawkr_seller/config/flavor.dart';
import 'package:hawkr_seller/core/theme/app_theme.dart';
import 'package:hawkr_seller/features/post/data/post_model.dart';
import 'package:hawkr_seller/features/post/data/post_repository.dart';
import 'package:hawkr_seller/features/post/presentation/post_compose_screen.dart';
import 'package:hawkr_seller/features/product/data/product_repository.dart';
import 'package:hawkr_seller/features/post/presentation/post_edit_screen.dart';
import 'package:hawkr_seller/shared/icons/app_icons.dart';
import 'package:hawkr_seller/shared/widgets/hk_tap_target.dart';
import 'package:flutter/material.dart';
import 'package:flutter/rendering.dart';
import 'package:flutter_screenutil/flutter_screenutil.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:hooks_riverpod/hooks_riverpod.dart';
import 'package:mocktail/mocktail.dart';

class _Repo extends Mock implements PostRepository {}

class _Products extends Mock implements ProductRepository {}

const _name = '屏東無毒金鑽鳳梨家庭分享箱・五台斤產地直送';

void main() {
  setUpAll(() {
    if (!F.isInitialized) F.init(Flavor.dev);
  });
  for (final editing in [false, true]) {
    for (final width in [320.0, 402.0, 440.0, 874.0]) {
      for (final scale in [1.0, 1.4]) {
        testWidgets(
          '${editing ? 'edit' : 'compose'} $width/$scale keeps sale amounts readable and selection operable',
          (tester) async {
            tester.view.devicePixelRatio = 1;
            tester.view.physicalSize = Size(width, width == 874 ? 402 : 874);
            addTearDown(tester.view.resetDevicePixelRatio);
            addTearDown(tester.view.resetPhysicalSize);
            final repo = _Repo();
            final products = _Products();
            when(() => products.getMyProducts()).thenAnswer(
              (_) async => {
                'data': {
                  'records': [
                    {
                      'id': 9,
                      'name': _name,
                      'price': 1599,
                      'salePrice': 1299,
                      'unit': '箱',
                    },
                  ],
                },
              },
            );
            await tester.pumpWidget(
              ProviderScope(
                overrides: [
                  postRepositoryProvider.overrideWithValue(repo),
                  productRepositoryProvider.overrideWithValue(products),
                  sellerStoreIdProvider.overrideWithValue('7'),
                ],
                child: ScreenUtilInit(
                  designSize: const Size(414, 896),
                  enableScaleWH: () => false,
                  enableScaleText: () => false,
                  minTextAdapt: false,
                  builder: (_, _) => MaterialApp(
                    theme: AppTheme.light,
                    builder: (context, child) => MediaQuery(
                      data: MediaQuery.of(context).copyWith(
                        textScaler: TextScaler.linear(scale),
                        padding: width == 874
                            ? const EdgeInsets.fromLTRB(59, 0, 59, 21)
                            : const EdgeInsets.only(top: 62, bottom: 34),
                      ),
                      child: child!,
                    ),
                    home: editing
                        ? const PostEditScreen(
                            post: Post(
                              id: 42,
                              sellerId: 7,
                              contentType: 'TEXT',
                              content: '當季產地商品介紹',
                            ),
                          )
                        : const PostComposeScreen(contentType: 'image'),
                  ),
                ),
              ),
            );
            await tester.pumpAndSettle();
            expect(tester.takeException(), isNull);
            for (final label in ['更換貼文圖片', if (!editing) '編輯貼文圖片']) {
              final control = find.byWidgetPredicate(
                (widget) =>
                    widget is HkTapTarget && widget.semanticLabel == label,
              );
              expect(tester.getSize(control), const Size.square(48));
              if (width == 874) {
                expect(tester.getRect(control).left, greaterThanOrEqualTo(59));
                expect(
                  tester.getRect(control).right,
                  lessThanOrEqualTo(width - 59),
                );
              }
            }
            final sale = find.text(r'NT$ 1299');
            await tester.ensureVisible(sale);
            await tester.pumpAndSettle();
            for (final amount in [r'NT$ 1299', r'NT$ 1599']) {
              final text = find.text(amount);
              final paragraph = tester.renderObject<RenderParagraph>(text);
              expect(
                paragraph.didExceedMaxLines,
                isFalse,
                reason: '$amount must remain fully readable at $width/$scale',
              );
              expect(tester.getRect(text).left, greaterThanOrEqualTo(0));
              expect(tester.getRect(text).right, lessThanOrEqualTo(width));
            }
            await tester.tap(sale);
            await tester.pumpAndSettle();
            expect(find.byIcon(AppIcons.check), findsOneWidget);
            await tester.tap(sale);
            await tester.pumpAndSettle();
            expect(find.byIcon(AppIcons.check), findsNothing);
            expect(tester.takeException(), isNull);
          },
        );
      }
    }
  }
}
