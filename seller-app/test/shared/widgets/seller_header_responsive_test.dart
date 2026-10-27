import 'package:hawkr_seller/core/theme/app_theme.dart';
import 'package:hawkr_seller/shared/widgets/hk_glass_app_bar.dart';
import 'package:hawkr_seller/shared/widgets/green_header_bar.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:flutter_screenutil/flutter_screenutil.dart';

const _title = '產地直送當季蔬果商品與訂單管理';
const _subtitle = '顧客消費紀錄與商品配送狀態';

void main() {
  for (final width in [320.0, 402.0, 440.0, 768.0]) {
    for (final scale in [1.0, 1.4]) {
      testWidgets('header reserves side actions at $width / $scale', (
        tester,
      ) async {
        var saved = false;
        await _pump(
          tester,
          width: width,
          scale: scale,
          glass: false,
          onSave: () => saved = true,
        );
        final title = tester.getRect(find.text(_title));
        final subtitle = tester.getRect(find.text(_subtitle));
        final back = tester.getRect(find.byTooltip('返回'));
        final save = tester.getRect(find.byType(HeaderTextButton));
        final header = tester.getRect(find.byType(GreenHeaderBar));
        expect(title.left, greaterThanOrEqualTo(back.right));
        expect(title.right, lessThanOrEqualTo(save.left));
        expect(subtitle.right, lessThanOrEqualTo(save.left));
        expect(subtitle.bottom, lessThanOrEqualTo(header.bottom));
        expect(title.top, greaterThanOrEqualTo(header.top));
        await tester.tap(find.text('儲存'));
        expect(saved, isTrue);
        expect(tester.takeException(), isNull);
      });

      testWidgets('glass title and subtitle fit toolbar at $width / $scale', (
        tester,
      ) async {
        await _pump(
          tester,
          width: width,
          scale: scale,
          glass: true,
          onSave: () {},
        );
        final bar = tester.getRect(find.byType(AppBar));
        final title = tester.getRect(find.text(_title));
        final subtitle = tester.getRect(find.text(_subtitle));
        expect(title.top, greaterThanOrEqualTo(62));
        expect(subtitle.bottom, lessThanOrEqualTo(bar.bottom));
        expect(subtitle.top, greaterThan(title.bottom));
        expect(tester.getTopLeft(find.byKey(const Key('body'))).dy, bar.bottom);
        expect(tester.takeException(), isNull);
      });
    }
  }
}

Future<void> _pump(
  WidgetTester tester, {
  required double width,
  required double scale,
  required bool glass,
  required VoidCallback onSave,
}) async {
  tester.view.devicePixelRatio = 3;
  tester.view.physicalSize = Size(width * 3, 874 * 3);
  tester.view.padding = const FakeViewPadding(top: 62 * 3, bottom: 34 * 3);
  tester.view.viewPadding = const FakeViewPadding(top: 62 * 3, bottom: 34 * 3);
  addTearDown(tester.view.resetPhysicalSize);
  addTearDown(tester.view.resetDevicePixelRatio);
  addTearDown(tester.view.resetPadding);
  addTearDown(tester.view.resetViewPadding);
  await tester.pumpWidget(
    ScreenUtilInit(
      designSize: const Size(414, 896),
      enableScaleWH: () => false,
      enableScaleText: () => false,
      minTextAdapt: false,
      builder: (_, _) => MaterialApp(
        theme: AppTheme.light,
        builder: (context, child) => MediaQuery(
          data: MediaQuery.of(
            context,
          ).copyWith(textScaler: TextScaler.linear(scale)),
          child: child!,
        ),
        home: glass
            ? Scaffold(
                appBar: HkGlassAppBar(
                  title: const Text(_title),
                  subtitle: _subtitle,
                  actions: [HeaderTextButton(text: '儲存', onPressed: onSave)],
                ),
                body: const SizedBox.expand(key: Key('body')),
              )
            : Scaffold(
                body: SafeArea(
                  child: Column(
                    children: [
                      GreenHeaderBar.titleWithSubtitle(
                        title: _title,
                        subtitle: _subtitle,
                        showBackButton: true,
                        trailing: [
                          HeaderTextButton(text: '儲存', onPressed: onSave),
                        ],
                      ),
                    ],
                  ),
                ),
              ),
      ),
    ),
  );
}
