import 'package:hawkr_buyer/core/layout/app_media_query_policy.dart';
import 'package:hawkr_buyer/core/layout/app_text_size.dart';
import 'package:hawkr_buyer/core/theme/app_theme.dart';
import 'package:hawkr_buyer/core/theme/app_typography.dart';
import 'package:hawkr_buyer/features/cart/data/cart_model.dart';
import 'package:hawkr_buyer/features/cart/presentation/widgets/cart_checkout_action.dart';
import 'package:hawkr_buyer/features/cart/presentation/widgets/cart_product_tile.dart';
import 'package:hawkr_buyer/features/cart/presentation/widgets/cart_top_bar.dart';
import 'package:hawkr_buyer/features/cart/presentation/widgets/delivery_notes_section.dart';
import 'package:hawkr_buyer/features/cart/presentation/widgets/payment_details_section.dart';
import 'package:flutter/material.dart';
import 'package:flutter/rendering.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:hooks_riverpod/hooks_riverpod.dart';
import 'package:shared_preferences/shared_preferences.dart';

void main() {
  for (final width in [320.0, 390.0]) {
    for (final platformScale in [1.0, 1.4]) {
      testWidgets(
        'cart semantic roles track saved small/standard/large at $width / $platformScale',
        (tester) async {
          SharedPreferences.setMockInitialValues({});
          final container = ProviderContainer();
          addTearDown(container.dispose);
          final notes = TextEditingController(text: '保留原本的配送備註');
          addTearDown(notes.dispose);
          tester.view.physicalSize = Size(width, 1200);
          tester.view.devicePixelRatio = 1;
          addTearDown(tester.view.resetPhysicalSize);
          addTearDown(tester.view.resetDevicePixelRatio);
          await tester.pumpWidget(
            UncontrolledProviderScope(
              container: container,
              child: MaterialApp(
                theme: AppTheme.light,
                home: Consumer(
                  builder: (context, ref, _) {
                    final preference = ref.watch(appTextSizeProvider);
                    return MediaQuery(
                      data: AppMediaQueryPolicy.apply(
                        MediaQuery.of(context).copyWith(
                          textScaler: TextScaler.linear(platformScale),
                        ),
                        textSize: preference,
                      ),
                      child: Scaffold(
                        body: SingleChildScrollView(
                          child: Column(
                            children: [
                              CartTopBar(
                                onBack: () {},
                                onClear: () {},
                                chatAction: IconButton(
                                  tooltip: '訊息',
                                  onPressed: () {},
                                  icon: const Icon(Icons.chat_bubble_outline),
                                ),
                              ),
                              CartProductTile(
                                item: const CartItem(
                                  id: 1,
                                  skuId: 101,
                                  count: 1,
                                  price: 680,
                                  title: '產地直送當季蔬果箱',
                                  specName: '家庭箱／約 3 公斤',
                                ),
                                isChecked: true,
                                remainingStock: 5,
                                onCheckChanged: (_) {},
                                onQuantityChanged: (_) {},
                              ),
                              DeliveryNotesSection(controller: notes),
                              PaymentDetailsSection(
                                productTotal: 680,
                                shippingTotal: 120,
                                couponDiscount: 0,
                                grandTotal: 800,
                                canSelectCoupon: true,
                                onSelectCoupon: () {},
                              ),
                              CartCheckoutAction(onPressed: () {}),
                            ],
                          ),
                        ),
                      ),
                    );
                  },
                ),
              ),
            ),
          );

          for (final preference in AppTextSize.values) {
            expect(
              await container
                  .read(appTextSizeProvider.notifier)
                  .setTextSize(preference),
              isTrue,
            );
            await tester.pump();
            final expectedScale = platformScale == 1
                ? {
                    AppTextSize.small: 0.9,
                    AppTextSize.standard: 1.0,
                    AppTextSize.large: 1.15,
                  }[preference]!
                : {
                    AppTextSize.small: 1.3,
                    AppTextSize.standard: 1.4,
                    AppTextSize.large: 1.4,
                  }[preference]!;
            final title = tester.widget<Text>(find.text('付款詳情')).style!;
            expect(
              (title.fontSize, title.height, title.fontWeight),
              (22, 28 / 22, FontWeight.w600),
            );
            final pageTitle = tester.widget<Text>(find.text('購物車')).style!;
            expect(
              (pageTitle.fontSize, pageTitle.height, pageTitle.fontWeight),
              (22, 28 / 22, FontWeight.w600),
            );
            final titleBounds = tester.getRect(find.text('購物車'));
            final backBounds = tester.getRect(find.byTooltip('返回'));
            final clearBounds = tester.getRect(find.byTooltip('清空購物車'));
            expect(titleBounds.left, greaterThanOrEqualTo(backBounds.right));
            expect(titleBounds.right, lessThanOrEqualTo(clearBounds.left));
            for (final bounds in [backBounds, clearBounds]) {
              expect(bounds.width, greaterThanOrEqualTo(48));
              expect(bounds.height, greaterThanOrEqualTo(48));
            }
            expect(
              tester
                  .renderObject<RenderParagraph>(find.text('購物車'))
                  .didExceedMaxLines,
              isFalse,
            );
            final name = tester.widget<Text>(find.text('產地直送當季蔬果箱')).style!;
            expect(
              (name.fontSize, name.height, name.fontWeight),
              (16, 1.5, FontWeight.w600),
            );
            expect(name.fontFamily, AppTypography.family);
            final body = tester.widget<Text>(find.text('商品總金額')).style!;
            expect(
              (body.fontSize, body.height, body.fontWeight),
              (16, 1.5, FontWeight.w400),
            );
            final support = tester
                .widget<Text>(find.text('規格：家庭箱／約 3 公斤'))
                .style!;
            expect(
              (support.fontSize, support.height, support.fontWeight),
              (14, 20 / 14, FontWeight.w400),
            );
            final count = tester.widget<Text>(find.text('1')).style!;
            expect(
              (count.fontSize, count.height, count.fontWeight),
              (16, 1.5, FontWeight.w600),
            );
            expect(
              tester
                  .renderObject<RenderParagraph>(find.text('1'))
                  .textScaler
                  .scale(count.fontSize!),
              closeTo(16 * expectedScale, 0.001),
            );
            final total = tester.widget<Text>(find.text('NT\$ 800')).style!;
            expect(
              (total.fontSize, total.height, total.fontWeight),
              (24, 32 / 24, FontWeight.w600),
            );
            final actualBody = tester.renderObject<RenderParagraph>(
              find.text('商品總金額'),
            );
            expect(
              actualBody.textScaler.scale(body.fontSize!),
              closeTo(16 * expectedScale, 0.001),
            );
            expect(
              tester.getSize(find.bySemanticsLabel('增加數量')),
              const Size.square(48),
            );
            final checkout = find.byKey(const ValueKey('cart-checkout-action'));
            expect(tester.getSize(checkout).height, greaterThanOrEqualTo(48));
            expect(
              tester
                  .widget<FilledButton>(checkout)
                  .style!
                  .textStyle!
                  .resolve({})!
                  .fontWeight,
              FontWeight.w600,
            );
            expect(notes.text, '保留原本的配送備註');
            expect(tester.takeException(), isNull);
          }
          final restored = ProviderContainer();
          addTearDown(restored.dispose);
          await restored.read(appTextSizeProvider.notifier).restore();
          expect(restored.read(appTextSizeProvider), AppTextSize.large);
        },
      );
    }
  }
}
