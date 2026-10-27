import 'package:hawkr_seller/core/layout/app_text_size.dart';
import 'package:hawkr_seller/core/theme/hk_tokens.dart';
import 'package:hawkr_seller/features/dashboard/presentation/dashboard_screen.dart';
import 'package:hawkr_seller/features/settings/presentation/text_size_settings_sheet.dart';
import 'package:flutter/material.dart';
import 'package:hawkr_seller/shared/widgets/hk_icon.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_screenutil/flutter_screenutil.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:shared_preferences/shared_preferences.dart';

void main() {
  TestWidgetsFlutterBinding.ensureInitialized();

  testWidgets('320px dashboard header announces S M L and opens the sheet', (
    tester,
  ) async {
    tester.view.devicePixelRatio = 1;
    tester.view.physicalSize = const Size(320, 720);
    addTearDown(tester.view.resetDevicePixelRatio);
    addTearDown(tester.view.resetPhysicalSize);
    SharedPreferences.setMockInitialValues({});

    final container = ProviderContainer();
    addTearDown(container.dispose);
    await container.read(appTextSizeProvider.notifier).restore();
    final semantics = tester.ensureSemantics();

    await tester.pumpWidget(
      UncontrolledProviderScope(
        container: container,
        child: ScreenUtilInit(
          designSize: const Size(414, 896),
          enableScaleWH: () => false,
          enableScaleText: () => false,
          minTextAdapt: false,
          builder: (_, __) => MaterialApp(
            builder: (context, child) => MediaQuery(
              data: MediaQuery.of(
                context,
              ).copyWith(textScaler: const TextScaler.linear(1.4)),
              child: child!,
            ),
            home: Scaffold(
              body: SizedBox(
                width: 320,
                height:
                    92 +
                    dashboardHeaderTextScaleExtra(const TextScaler.linear(1.4)),
                child: Padding(
                  padding: const EdgeInsets.symmetric(horizontal: HkSpace.lg),
                  child: DashboardHeader(
                    sellerName: '範例生鮮超長店舖名稱用來驗證窄螢幕',
                    storeCode: 'SELLER-123456789',
                    notificationUnread: 108,
                    chatUnread: 7,
                    onAvatarTap: _noop,
                    onNotificationTap: _noop,
                    onChatTap: _noop,
                  ),
                ),
              ),
            ),
          ),
        ),
      ),
    );
    await tester.pump(const Duration(milliseconds: 100));

    final action = find.byKey(const ValueKey('dashboard-text-size-action'));
    final icon = find.byKey(const ValueKey('dashboard-text-size-icon'));
    expect(action, findsOneWidget);
    expect(tester.getSize(action), const Size.square(HkControl.minTap));
    expect(tester.widget<HkIcon>(icon).name, HkIconName.textSize);
    expect(tester.widget<HkIcon>(icon).size, HkControl.icon);

    for (final preference in AppTextSize.values) {
      expect(
        await container
            .read(appTextSizeProvider.notifier)
            .setTextSize(preference),
        isTrue,
      );
      await tester.pump();
      expect(
        find.bySemanticsLabel('字體大小，目前為${preference.label}'),
        findsOneWidget,
      );
    }

    await tester.tap(action);
    await tester.pump();

    expect(find.byType(TextSizeSettingsSheet), findsOneWidget);
    expect(tester.takeException(), isNull);

    await tester.ensureVisible(find.text('完成'));
    await tester.pumpAndSettle();
    await tester.tap(find.text('完成'));
    await tester.pumpAndSettle();
    expect(tester.takeException(), isNull);
    semantics.dispose();
  });
}

void _noop() {}
