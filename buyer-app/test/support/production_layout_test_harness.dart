import 'package:hawkr_buyer/core/layout/app_media_query_policy.dart';
import 'package:hawkr_buyer/core/layout/app_text_size.dart';
import 'package:hawkr_buyer/core/theme/app_colors.dart';
import 'package:flutter/material.dart';
import 'package:flutter_screenutil/flutter_screenutil.dart';
import 'package:flutter_test/flutter_test.dart';

/// Pumps a widget with the same ScreenUtil policy as production.
///
/// The platform scaler is injected before [AppMediaQueryPolicy] so a 2.0x
/// system preference exercises the production 1.4x accessibility ceiling.
Future<void> pumpProductionLayoutTestApp(
  WidgetTester tester, {
  required Size size,
  required Widget home,
  double systemTextScale = 2,
  AppTextSize textSize = AppTextSize.standard,
  EdgeInsets viewPadding = EdgeInsets.zero,
  EdgeInsets viewInsets = EdgeInsets.zero,
  TargetPlatform? platform,
}) async {
  tester.view.devicePixelRatio = 1;
  tester.view.physicalSize = size;
  addTearDown(tester.view.resetPhysicalSize);
  addTearDown(tester.view.resetDevicePixelRatio);

  await tester.pumpWidget(
    ScreenUtilInit(
      designSize: const Size(414, 896),
      enableScaleWH: () => false,
      enableScaleText: () => false,
      minTextAdapt: false,
      splitScreenMode: true,
      builder: (context, _) => MaterialApp(
        theme: ThemeData(
          useMaterial3: true,
          platform: platform,
          colorScheme: ColorScheme.fromSeed(
            seedColor: AppColors.brandPrimaryStrong,
          ),
        ),
        builder: (context, child) {
          final platformMediaQuery = MediaQuery.of(context).copyWith(
            size: size,
            padding: viewPadding,
            viewPadding: viewPadding,
            viewInsets: viewInsets,
            textScaler: TextScaler.linear(systemTextScale),
          );
          return MediaQuery(
            data: AppMediaQueryPolicy.apply(
              platformMediaQuery,
              textSize: textSize,
            ),
            child: child ?? const SizedBox.shrink(),
          );
        },
        home: home,
      ),
    ),
  );
  await tester.pump();
}
