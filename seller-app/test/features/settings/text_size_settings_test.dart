import 'package:hawkr_seller/core/layout/app_media_query_policy.dart';
import 'package:hawkr_seller/core/layout/app_text_size.dart';
import 'package:hawkr_seller/features/seller/presentation/profile_other_options_card.dart';
import 'package:hawkr_seller/features/settings/presentation/text_size_settings_sheet.dart';
import 'package:hawkr_seller/shared/widgets/hk_button_label.dart';
import 'package:hawkr_seller/shared/widgets/glass_tab_bar.dart';
import 'package:flutter/material.dart';
import 'package:flutter/rendering.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_screenutil/flutter_screenutil.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:shared_preferences/shared_preferences.dart';

void main() {
  TestWidgetsFlutterBinding.ensureInitialized();

  testWidgets(
    'seller settings changes and persists the large text preference',
    (tester) async {
      tester.view.devicePixelRatio = 1;
      tester.view.physicalSize = const Size(320, 720);
      addTearDown(tester.view.resetDevicePixelRatio);
      addTearDown(tester.view.resetPhysicalSize);
      SharedPreferences.setMockInitialValues({});
      final container = ProviderContainer();
      addTearDown(container.dispose);
      await container.read(appTextSizeProvider.notifier).restore();

      await tester.pumpWidget(
        UncontrolledProviderScope(
          container: container,
          child: ScreenUtilInit(
            designSize: const Size(414, 896),
            enableScaleWH: () => false,
            enableScaleText: () => false,
            minTextAdapt: false,
            builder: (_, __) => MaterialApp(
              builder: (context, child) => Consumer(
                builder: (context, ref, _) {
                  final preference = ref.watch(appTextSizeProvider);
                  return MediaQuery(
                    data: AppMediaQueryPolicy.apply(
                      MediaQuery.of(context),
                      textSize: preference,
                    ),
                    child: child!,
                  );
                },
              ),
              home: Scaffold(
                body: SingleChildScrollView(
                  child: ProfileOtherOptionsCard(onShippingTap: _noop),
                ),
              ),
            ),
          ),
        ),
      );

      expect(find.text('字體大小'), findsOneWidget);
      expect(find.text('選擇小、中或大字，依閱讀習慣調整'), findsOneWidget);

      await tester.ensureVisible(find.text('字體大小'));
      await tester.tap(find.text('字體大小'));
      await tester.pumpAndSettle();

      expect(find.byType(TextSizeSettingsSheet), findsOneWidget);
      expect(find.byKey(const Key('settings-text-size-0')), findsOneWidget);
      expect(find.byKey(const Key('settings-text-size-1')), findsOneWidget);
      expect(find.byKey(const Key('settings-text-size-2')), findsOneWidget);
      expect(
        tester
            .widget<HkGlassTabBar>(find.byType(HkGlassTabBar))
            .mutuallyExclusiveSemantics,
        isTrue,
      );

      await tester.tap(find.byKey(const Key('settings-text-size-2')));
      await tester.pumpAndSettle();

      expect(container.read(appTextSizeProvider), AppTextSize.large);
      final preferences = await SharedPreferences.getInstance();
      expect(
        preferences.getString(AppTextSizeController.storageKey),
        AppTextSize.large.name,
      );

      final previewParagraph = tester.renderObject<RenderParagraph>(
        find.text('商品、訂單與物流資訊都會依這個設定顯示。'),
      );
      expect(previewParagraph.textScaler.scale(16), closeTo(18.4, 0.0001));

      final doneLabel = tester.widget<Text>(
        find.descendant(
          of: find.byType(HkButtonLabel),
          matching: find.text('完成'),
        ),
      );
      expect(doneLabel.maxLines, 1);
      expect(doneLabel.softWrap, isFalse);
      expect(doneLabel.overflow, TextOverflow.ellipsis);
      expect(tester.takeException(), isNull);
    },
  );
}

void _noop() {}
