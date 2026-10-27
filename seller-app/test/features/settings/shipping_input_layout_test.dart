import 'package:hawkr_seller/core/theme/app_theme.dart';
import 'package:hawkr_seller/features/settings/presentation/shipping_settings/widgets/address_autocomplete_field.dart';
import 'package:hawkr_seller/features/settings/presentation/shipping_settings/widgets/common_widgets.dart';
import 'package:flutter/material.dart';
import 'package:flutter_screenutil/flutter_screenutil.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:hooks_riverpod/hooks_riverpod.dart';

void main() {
  for (final scale in [1.0, 1.4]) {
    testWidgets('shipping input keeps the complete text line at $scale text', (
      tester,
    ) async {
      tester.view.devicePixelRatio = 1;
      tester.view.physicalSize = const Size(320, 720);
      addTearDown(tester.view.resetDevicePixelRatio);
      addTearDown(tester.view.resetPhysicalSize);
      final name = TextEditingController(text: '陳小明');
      final address = TextEditingController(text: '中正路 123 號');
      addTearDown(name.dispose);
      addTearDown(address.dispose);
      await tester.pumpWidget(
        ProviderScope(
          child: ScreenUtilInit(
            designSize: const Size(414, 896),
            enableScaleWH: () => false,
            enableScaleText: () => false,
            builder: (_, _) => MaterialApp(
              theme: AppTheme.light,
              builder: (context, child) => MediaQuery(
                data: MediaQuery.of(
                  context,
                ).copyWith(textScaler: TextScaler.linear(scale)),
                child: child!,
              ),
              home: Scaffold(
                body: SingleChildScrollView(
                  padding: const EdgeInsets.all(16),
                  child: Column(
                    children: [
                      CustomInput(controller: name, hint: '請輸入姓名'),
                      const SizedBox(height: 16),
                      AddressAutocompleteField(controller: address),
                    ],
                  ),
                ),
              ),
            ),
          ),
        ),
      );
      for (final state in tester.stateList<EditableTextState>(
        find.byType(EditableText),
      )) {
        final render = state.renderEditable;
        expect(
          render.size.height,
          greaterThanOrEqualTo(render.preferredLineHeight),
          reason: 'The input must grow with Dynamic Type instead of clipping',
        );
      }
      expect(tester.takeException(), isNull);
    });
  }
}
