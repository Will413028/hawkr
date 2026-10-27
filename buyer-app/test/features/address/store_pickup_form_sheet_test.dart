import 'package:hawkr_buyer/core/theme/app_colors.dart';
import 'package:hawkr_buyer/core/theme/app_typography.dart';
import 'package:hawkr_buyer/features/cart/presentation/store_pickup_form_screen.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:hooks_riverpod/hooks_riverpod.dart';

void main() {
  testWidgets('7-11 form uses the same bottom sheet interaction as delivery', (
    tester,
  ) async {
    await _pumpStorePickupSheet(tester);

    expect(find.text('新增 7-11 取貨門市'), findsOneWidget);
    expect(
      find.byKey(const ValueKey('store-pickup-form-drag-handle')),
      findsOneWidget,
    );
    expect(find.bySemanticsLabel('關閉'), findsNothing);
    expect(
      find.byKey(const ValueKey('store-pickup-name-field')),
      findsOneWidget,
    );
    expect(
      find.byKey(const ValueKey('store-pickup-phone-field')),
      findsOneWidget,
    );
    expect(
      find.byKey(const ValueKey('store-pickup-selection-field')),
      findsOneWidget,
    );
    expect(find.text('選擇 7-11 門市'), findsOneWidget);
    expect(find.text('設為預設 7-11 門市'), findsOneWidget);
    expect(find.text('取消'), findsOneWidget);
    expect(find.text('儲存門市'), findsOneWidget);
    expect(find.text('請填寫收件資料並選擇門市'), findsOneWidget);

    final nameTop = tester
        .getTopLeft(find.byKey(const ValueKey('store-pickup-name-field')))
        .dy;
    final phoneTop = tester
        .getTopLeft(find.byKey(const ValueKey('store-pickup-phone-field')))
        .dy;
    expect(phoneTop, greaterThan(nameTop));
    final saveButton = tester.widget<FilledButton>(
      find.byKey(const ValueKey('store-pickup-save-button')),
    );
    expect(saveButton.onPressed, isNull);
    expect(tester.takeException(), isNull);
  });

  testWidgets('editing a saved store pre-fills the shared sheet', (
    tester,
  ) async {
    await _pumpStorePickupSheet(
      tester,
      existingStore: const {
        'id': 'pickup-1',
        'receiverName': '王小明',
        'receiverPhone': '0912345678',
        'storeCode': '123456',
        'storeName': '統一超商民生門市',
        'storeAddress': '臺北市中山區民生東路一段1號',
        'isDefault': true,
      },
    );

    expect(find.text('編輯 7-11 取貨門市'), findsOneWidget);
    expect(find.text('統一超商民生門市'), findsOneWidget);
    expect(find.text('店號 123456 · 臺北市中山區民生東路一段1號'), findsOneWidget);
    expect(find.text('重新選擇門市'), findsOneWidget);
    final checkbox = tester.widget<CheckboxListTile>(
      find.byKey(const ValueKey('store-pickup-default-checkbox')),
    );
    expect(checkbox.value, isTrue);
    final saveButton = tester.widget<FilledButton>(
      find.byKey(const ValueKey('store-pickup-save-button')),
    );
    expect(saveButton.onPressed, isNotNull);
    expect(tester.takeException(), isNull);
  });

  testWidgets('7-11 sheet remains scrollable at 320px with large text', (
    tester,
  ) async {
    await _pumpStorePickupSheet(tester, width: 320, textScale: 1.6);

    final scrollView = find.byKey(const ValueKey('store-pickup-form-scroll'));
    expect(scrollView, findsOneWidget);
    await tester.drag(scrollView, const Offset(0, -220));
    await tester.pump();

    expect(
      find.byKey(const ValueKey('store-pickup-cancel-button')),
      findsOneWidget,
    );
    expect(
      find.byKey(const ValueKey('store-pickup-save-button')),
      findsOneWidget,
    );
    expect(tester.takeException(), isNull);
  });
}

Future<void> _pumpStorePickupSheet(
  WidgetTester tester, {
  double width = 390,
  double textScale = 1,
  Map<String, dynamic>? existingStore,
}) async {
  tester.view.physicalSize = Size(width, 844);
  tester.view.devicePixelRatio = 1;
  addTearDown(tester.view.resetPhysicalSize);
  addTearDown(tester.view.resetDevicePixelRatio);

  await tester.pumpWidget(
    ProviderScope(
      child: MaterialApp(
        theme: ThemeData(
          useMaterial3: true,
          colorScheme:
              ColorScheme.fromSeed(
                seedColor: AppColors.brandPrimaryStrong,
              ).copyWith(
                primary: AppColors.actionPrimary,
                onPrimary: AppColors.textInverse,
                primaryContainer: AppColors.primarySoft,
                onPrimaryContainer: AppColors.brandPrimaryStrong,
                surface: AppColors.bgCard,
                onSurface: AppColors.textEmphasis,
                outlineVariant: AppColors.cardBorder,
                scrim: AppColors.bgOverlay,
              ),
          textTheme: AppTypography.textTheme,
          splashFactory: NoSplash.splashFactory,
        ),
        home: MediaQuery(
          data: MediaQueryData(
            size: Size(width, 844),
            textScaler: TextScaler.linear(textScale),
            disableAnimations: true,
          ),
          child: Scaffold(
            body: Builder(
              builder: (context) => Center(
                child: FilledButton(
                  onPressed: () => showStorePickupFormSheet(
                    context,
                    existingStore: existingStore,
                  ),
                  child: const Text('開啟'),
                ),
              ),
            ),
          ),
        ),
      ),
    ),
  );
  await tester.pumpAndSettle();
  await tester.tap(find.text('開啟'));
  await tester.pumpAndSettle();
}
