import 'package:hawkr_buyer/core/layout/app_text_size.dart';
import 'package:hawkr_buyer/core/theme/hk_tokens.dart';
import 'package:hawkr_buyer/features/cart/presentation/cart_controller.dart';
import 'package:hawkr_buyer/features/notification/presentation/notification_controller.dart';
import 'package:hawkr_buyer/shared/widgets/hk_icon.dart';
import 'package:hawkr_buyer/shared/widgets/logo_home_button.dart';
import 'package:hawkr_buyer/shared/widgets/main_top_bar.dart';
import 'package:hawkr_buyer/shared/widgets/text_size_settings_sheet.dart';
import 'package:hawkr_buyer/shared/widgets/text_size_top_bar_action.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:hooks_riverpod/hooks_riverpod.dart';
import 'package:shared_preferences/shared_preferences.dart';

import '../../support/secure_storage_mock.dart';

void main() {
  TestWidgetsFlutterBinding.ensureInitialized();

  setUp(() {
    SharedPreferences.setMockInitialValues({});
    mockSecureStorage({});
  });

  testWidgets('top-bar shortcut exposes the current size and opens the sheet', (
    tester,
  ) async {
    final semantics = tester.ensureSemantics();
    final container = ProviderContainer();
    addTearDown(container.dispose);

    await tester.pumpWidget(
      UncontrolledProviderScope(
        container: container,
        child: const MaterialApp(
          home: Scaffold(
            body: Align(
              alignment: Alignment.topRight,
              child: TextSizeTopBarAction(),
            ),
          ),
        ),
      ),
    );
    await tester.pumpAndSettle();

    final action = find.byKey(const ValueKey('topbar-text-size-action'));
    expect(tester.getSize(action), const Size.square(48));
    expect(find.bySemanticsLabel('字體大小，目前中'), findsOneWidget);
    expect(
      tester
          .widget<HkIcon>(find.byKey(const ValueKey('topbar-text-size-icon')))
          .name,
      HkIconName.textSize,
    );

    await tester.tap(action);
    await tester.pump();
    await tester.pump(const Duration(milliseconds: 100));

    expect(find.byType(TextSizeSettingsSheet), findsOneWidget);

    await tester.pumpAndSettle();
    await tester.tap(find.byKey(const ValueKey('settings-text-size-2')));
    await tester.pumpAndSettle();

    expect(container.read(appTextSizeProvider), AppTextSize.large);
    final preferences = await SharedPreferences.getInstance();
    expect(
      preferences.getString(AppTextSizeController.storageKey),
      AppTextSize.large.name,
    );

    await tester.tap(find.widgetWithText(FilledButton, '完成'));
    await tester.pumpAndSettle();
    expect(find.bySemanticsLabel('字體大小，目前大'), findsOneWidget);
    semantics.dispose();
    expect(tester.takeException(), isNull);
  });

  testWidgets('text-size sheet uses compact iOS handle rhythm and safe inset', (
    tester,
  ) async {
    tester.view.devicePixelRatio = 1;
    tester.view.physicalSize = const Size(320, 568);
    addTearDown(tester.view.resetDevicePixelRatio);
    addTearDown(tester.view.resetPhysicalSize);

    await tester.pumpWidget(
      ProviderScope(
        child: MaterialApp(
          theme: ThemeData(useMaterial3: true),
          builder: (context, child) {
            final mediaQuery = MediaQuery.of(context);
            const safeInset = EdgeInsets.only(bottom: 34);
            return MediaQuery(
              data: mediaQuery.copyWith(
                textScaler: TextScaler.linear(1.4),
                padding: const EdgeInsets.only(bottom: 114),
                viewPadding: safeInset,
              ),
              child: child!,
            );
          },
          home: Builder(
            builder: (context) => Scaffold(
              body: FilledButton(
                onPressed: () => showTextSizeSettingsSheet(context),
                child: const Text('開啟字體設定'),
              ),
            ),
          ),
        ),
      ),
    );

    await tester.tap(find.text('開啟字體設定'));
    await tester.pumpAndSettle();

    final bottomSheetFinder = find.byType(BottomSheet);
    final bottomSheet = tester.widget<BottomSheet>(bottomSheetFinder);
    expect(bottomSheet.showDragHandle, isFalse);

    final handleFinder = find.byKey(
      const ValueKey('text-size-sheet-drag-handle'),
    );
    expect(handleFinder, findsOneWidget);
    final indicatorFinder = find.descendant(
      of: handleFinder,
      matching: find.byType(SizedBox),
    );
    expect(indicatorFinder, findsOneWidget);

    final sheetRect = tester.getRect(bottomSheetFinder);
    final indicatorRect = tester.getRect(indicatorFinder);
    final titleRect = tester.getRect(find.text('字體大小'));
    final doneRect = tester.getRect(find.widgetWithText(FilledButton, '完成'));

    expect(indicatorRect.top - sheetRect.top, closeTo(HkSpace.sm, 0.01));
    expect(titleRect.top - indicatorRect.bottom, closeTo(HkSpace.md, 0.01));
    expect(sheetRect.bottom - doneRect.bottom, closeTo(34, 0.01));
    expect(tester.takeException(), isNull);
  });

  for (final scenario in <({double width, bool compactBrand})>[
    (width: 320, compactBrand: true),
    (width: 390, compactBrand: false),
  ]) {
    testWidgets(
      'home top bar fits ${scenario.width.toInt()}px without shrinking actions',
      (tester) async {
        tester.view.devicePixelRatio = 1;
        tester.view.physicalSize = Size(scenario.width, 844);
        addTearDown(tester.view.resetDevicePixelRatio);
        addTearDown(tester.view.resetPhysicalSize);

        await tester.pumpWidget(
          ProviderScope(
            overrides: [
              cartCountProvider.overrideWith((_) async => 0),
              notificationUnreadCountProvider.overrideWith((_) async => 0),
            ],
            child: MaterialApp(
              home: Builder(
                builder: (context) => Scaffold(
                  appBar: buildMainTopBar(context, showTextSize: true),
                ),
              ),
            ),
          ),
        );
        await tester.pumpAndSettle();

        expect(tester.takeException(), isNull);
        expect(
          find.byKey(
            ValueKey(
              scenario.compactBrand ? 'hawkr-compact-brand' : 'hawkr-wordmark',
            ),
          ),
          findsOneWidget,
        );
        expect(
          tester.getSize(find.byKey(const ValueKey('topbar-text-size-action'))),
          const Size.square(48),
        );
        expect(
          tester
              .widget<HawkrWordmarkButton>(find.byType(HawkrWordmarkButton))
              .compact,
          scenario.compactBrand,
        );
      },
    );
  }
}
