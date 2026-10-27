import 'package:hawkr_seller/core/theme/app_theme.dart';
import 'package:hawkr_seller/features/dashboard/presentation/dashboard_screen.dart';
import 'package:flutter/material.dart';
import 'package:flutter/rendering.dart';
import 'package:flutter_screenutil/flutter_screenutil.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:hooks_riverpod/hooks_riverpod.dart';

import '../../support/dashboard_fixtures.dart';

void main() {
  for (final width in [320.0, 393.0, 402.0, 440.0, 768.0]) {
    for (final scale in [1.0, 1.4]) {
      testWidgets(
        'dashboard data remains readable at ${width}pt / $scale text',
        (tester) async {
          tester.view.devicePixelRatio = 1;
          tester.view.physicalSize = Size(width, 900);
          addTearDown(tester.view.resetDevicePixelRatio);
          addTearDown(tester.view.resetPhysicalSize);

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
                      child: DashboardOverviewContent(
                        dashboard: fullDashboard(),
                        onOpenOrderTab: _noopTab,
                        onAddProduct: _noop,
                        onOpenShipping: _noop,
                        showAnomalyBanner: false,
                      ),
                    ),
                  ),
                ),
              ),
            ),
          );
          await tester.pump();

          final revenue = find.text(r'NT$ 128,400');
          final text = tester.widget<Text>(revenue);
          final context = tester.element(revenue);
          final painter = TextPainter(
            text: TextSpan(text: text.data, style: text.style),
            textDirection: TextDirection.ltr,
            textScaler: MediaQuery.textScalerOf(context),
          )..layout(maxWidth: tester.getSize(revenue).width);
          final actualLines = painter.computeLineMetrics().length;
          painter.layout(maxWidth: width - 32 - 40);
          expect(
            actualLines,
            painter.computeLineMetrics().length,
            reason: 'The amount needs the full card width before wrapping',
          );
          painter.dispose();

          for (final label in ['商品管理', '新增商品', '配送設定']) {
            final paragraph = tester.renderObject<RenderParagraph>(
              find.text(label),
            );
            expect(
              paragraph.didExceedMaxLines,
              isFalse,
              reason: '$label is hidden',
            );
          }
          expect(MediaQuery.textScalerOf(context).scale(16), 16 * scale);
          expect(tester.takeException(), isNull);
        },
      );
    }
  }
}

void _noop() {}
void _noopTab(int _) {}
