import 'package:hawkr_buyer/core/theme/app_colors.dart';
import 'package:hawkr_buyer/core/theme/app_typography.dart';
import 'package:hawkr_buyer/features/address/data/address_model.dart';
import 'package:hawkr_buyer/features/address/presentation/address_management_screen.dart';
import 'package:hawkr_buyer/shared/widgets/app_dropdown.dart';
import 'package:hawkr_twaddr/hawkr_twaddr.dart';
import 'package:flutter/material.dart';
import 'package:flutter_screenutil/flutter_screenutil.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:hooks_riverpod/hooks_riverpod.dart';

void main() {
  testWidgets('address form sheet follows the web mobile hierarchy', (
    tester,
  ) async {
    await _pumpAddressSheet(tester);

    expect(find.text('新增宅配地址'), findsOneWidget);
    expect(
      find.byKey(const ValueKey('address-form-drag-handle')),
      findsOneWidget,
    );
    expect(find.text('標籤類型'), findsNothing);
    expect(find.byKey(const ValueKey('address-form-close')), findsNothing);
    expect(find.byKey(const ValueKey('address-name-field')), findsOneWidget);
    expect(find.byKey(const ValueKey('address-phone-field')), findsOneWidget);
    expect(find.byKey(const ValueKey('address-city-dropdown')), findsOneWidget);
    expect(
      find.byKey(const ValueKey('address-district-dropdown')),
      findsOneWidget,
    );
    // D2：地址驗證步驟已移除，表單填完即可儲存，不再有驗證按鈕。
    expect(find.byKey(const ValueKey('address-verify-button')), findsNothing);
    expect(find.text('設為預設地址'), findsOneWidget);
    expect(find.text('取消'), findsOneWidget);
    expect(find.text('儲存地址'), findsOneWidget);

    final nameTop = tester
        .getTopLeft(find.byKey(const ValueKey('address-name-field')))
        .dy;
    final phoneTop = tester
        .getTopLeft(find.byKey(const ValueKey('address-phone-field')))
        .dy;
    expect(phoneTop, greaterThan(nameTop));

    final saveButton = tester.widget<FilledButton>(
      find.byKey(const ValueKey('address-save-button')),
    );
    expect(saveButton.onPressed, isNull);
    expect(tester.takeException(), isNull);
  });

  testWidgets('city and district use the shared app dropdown behavior', (
    tester,
  ) async {
    await _pumpAddressSheet(tester);

    final cityFinder = find.byKey(const ValueKey('address-city-dropdown'));
    final districtFinder = find.byKey(
      const ValueKey('address-district-dropdown'),
    );
    final initialDistrict = tester.widget<AppDropdown<Town>>(districtFinder);
    expect(initialDistrict.enabled, isFalse);
    expect(initialDistrict.placeholder, '請先選擇縣市');

    final taipei = countyByCode('63000')!;
    tester.widget<AppDropdown<County>>(cityFinder).onSelected(taipei);
    await tester.pump();

    final selectedCity = tester.widget<AppDropdown<County>>(cityFinder);
    final enabledDistrict = tester.widget<AppDropdown<Town>>(districtFinder);
    expect(selectedCity.selectedItem?.name, '臺北市');
    expect(enabledDistrict.enabled, isTrue);
    expect(enabledDistrict.items, townsByCounty('63000'));
    expect(enabledDistrict.items.map((t) => t.name), contains('中山區'));
    expect(enabledDistrict.selectedItem, isNull);
    expect(enabledDistrict.placeholder, '鄉鎮市區');
    expect(tester.takeException(), isNull);
  });

  testWidgets('choosing another county clears the chosen town', (tester) async {
    await _pumpAddressSheet(tester);
    final cityFinder = find.byKey(const ValueKey('address-city-dropdown'));
    final districtFinder = find.byKey(
      const ValueKey('address-district-dropdown'),
    );

    tester
        .widget<AppDropdown<County>>(cityFinder)
        .onSelected(countyByCode('63000')!);
    await tester.pump();
    tester
        .widget<AppDropdown<Town>>(districtFinder)
        .onSelected(townByCode('63000040')!);
    await tester.pump();
    expect(
      tester.widget<AppDropdown<Town>>(districtFinder).selectedItem?.code,
      '63000040',
    );

    final other = counties.firstWhere((c) => c.code != '63000');
    tester.widget<AppDropdown<County>>(cityFinder).onSelected(other);
    await tester.pump();
    expect(
      tester.widget<AppDropdown<Town>>(districtFinder).selectedItem,
      isNull,
    );
    expect(
      tester.widget<AppDropdown<Town>>(districtFinder).items,
      townsByCounty(other.code),
    );
  });

  testWidgets('address form sheet remains scrollable at 320px and large text', (
    tester,
  ) async {
    await _pumpAddressSheet(tester, width: 320, textScale: 1.6);

    final scrollView = find.byKey(const ValueKey('address-form-scroll'));
    expect(scrollView, findsOneWidget);
    await tester.drag(scrollView, const Offset(0, -160));
    await tester.pump();

    expect(find.byKey(const ValueKey('address-cancel-button')), findsOneWidget);
    expect(find.byKey(const ValueKey('address-save-button')), findsOneWidget);
    expect(tester.takeException(), isNull);
  });

  testWidgets(
    'editing shows the MOI names of the stored town code in shared dropdowns',
    (tester) async {
      await _pumpAddressSheet(
        tester,
        address: const Address(
          etag: 'v1',
          id: 1,
          receiverName: '王小明',
          receiverPhone: '0912345678',
          townCode: '63000040',
          addressDetail: '民生東路一段1號',
        ),
      );

      final city = tester.widget<AppDropdown<County>>(
        find.byKey(const ValueKey('address-city-dropdown')),
      );
      final district = tester.widget<AppDropdown<Town>>(
        find.byKey(const ValueKey('address-district-dropdown')),
      );
      expect(city.selectedItem, countyByCode('63000'));
      expect(district.selectedItem, townByCode('63000040'));
      expect(district.items, contains(townByCode('63000040')));
      // D2：欄位齊全就能儲存，不需要先經過地址驗證。
      final saveButton = tester.widget<FilledButton>(
        find.byKey(const ValueKey('address-save-button')),
      );
      expect(saveButton.onPressed, isNotNull);
    },
  );

  testWidgets('saving returns the code of the town chosen in the dropdown', (
    tester,
  ) async {
    AddressFormResult? result;
    await _pumpAddressSheet(
      tester,
      address: const Address(
        etag: 'v1',
        id: 1,
        receiverName: '王小明',
        receiverPhone: '0912345678',
        townCode: '63000040',
        addressDetail: '民生東路一段1號',
      ),
      onResult: (value) => result = value,
    );
    final districtFinder = find.byKey(
      const ValueKey('address-district-dropdown'),
    );
    final other = townsByCounty('63000').last;
    expect(other.code, isNot('63000040'));
    tester.widget<AppDropdown<Town>>(districtFinder).onSelected(other);
    await tester.pump();

    await tester.tap(find.byKey(const ValueKey('address-save-button')));
    await tester.pumpAndSettle();

    expect(result!.townCode, other.code);
  });

  for (final isDefault in [true, false]) {
    testWidgets(
      'saving returns the typed form result (isDefault: $isDefault)',
      (tester) async {
        AddressFormResult? result;
        await _pumpAddressSheet(
          tester,
          address: Address(
            etag: 'v1',
            id: 1,
            receiverName: '王小明',
            receiverPhone: '0912345678',
            townCode: '63000040',
            addressDetail: '民生東路一段1號',
            isDefault: isDefault,
          ),
          onResult: (value) => result = value,
        );

        await tester.tap(find.byKey(const ValueKey('address-save-button')));
        await tester.pumpAndSettle();

        expect(result, isNotNull);
        expect(result!.receiverName, '王小明');
        expect(result!.receiverPhone, '0912345678');
        expect(result!.townCode, '63000040');
        expect(result!.addressDetail, '民生東路一段1號');
        expect(result!.isDefault, isDefault);
      },
    );
  }
}

Future<void> _pumpAddressSheet(
  WidgetTester tester, {
  double width = 390,
  double textScale = 1,
  Address? address,
  void Function(AddressFormResult?)? onResult,
}) async {
  tester.view.physicalSize = Size(width, 844);
  tester.view.devicePixelRatio = 1;
  addTearDown(tester.view.resetPhysicalSize);
  addTearDown(tester.view.resetDevicePixelRatio);

  await tester.pumpWidget(
    ProviderScope(
      child: ScreenUtilInit(
        designSize: const Size(414, 896),
        builder: (context, child) => MaterialApp(
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
                    onPressed: () async {
                      final result = await showAddressFormSheet(
                        context,
                        address: address,
                      );
                      onResult?.call(result);
                    },
                    child: const Text('開啟'),
                  ),
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
