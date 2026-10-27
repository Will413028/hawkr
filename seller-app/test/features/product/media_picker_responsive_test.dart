import 'package:hawkr_seller/core/theme/app_theme.dart';
import 'package:hawkr_seller/features/post/presentation/post_media_picker_screen.dart';
import 'package:hawkr_seller/features/product/presentation/product_setup_media_picker_screen.dart';
import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_screenutil/flutter_screenutil.dart';
import 'package:flutter_test/flutter_test.dart';

void main() {
  const photoChannel = MethodChannel('com.fluttercandies/photo_manager');
  final binding = TestWidgetsFlutterBinding.ensureInitialized();
  setUp(() {
    binding.defaultBinaryMessenger.setMockMethodCallHandler(photoChannel, (
      call,
    ) async {
      if (call.method == 'requestPermissionExtend') return 3;
      if (call.method == 'getAssetPathList') return {'data': <Object>[]};
      return null;
    });
  });
  tearDown(() {
    binding.defaultBinaryMessenger.setMockMethodCallHandler(photoChannel, null);
  });

  for (final picker in <String, Widget>{
    'product': const ProductSetupMediaPickerScreen(),
    'post': const PostMediaPickerScreen(),
  }.entries) {
    for (final size in [
      const Size(402, 874),
      const Size(874, 402),
      const Size(320, 568),
    ]) {
      testWidgets('${picker.key} media picker keeps usable grid at $size', (
        tester,
      ) async {
        tester.view.devicePixelRatio = 1;
        tester.view.physicalSize = size;
        addTearDown(tester.view.resetDevicePixelRatio);
        addTearDown(tester.view.resetPhysicalSize);
        await tester.pumpWidget(
          ScreenUtilInit(
            designSize: const Size(414, 896),
            enableScaleWH: () => false,
            enableScaleText: () => false,
            builder: (_, _) => MaterialApp(
              theme: AppTheme.light,
              builder: (context, child) => MediaQuery(
                data: MediaQuery.of(context).copyWith(
                  padding: size.width > size.height
                      ? const EdgeInsets.only(left: 62, right: 62, bottom: 21)
                      : const EdgeInsets.only(top: 62, bottom: 34),
                  textScaler: const TextScaler.linear(1.4),
                ),
                child: child!,
              ),
              home: picker.value,
            ),
          ),
        );
        await tester.pumpAndSettle();
        expect(find.byType(GridView), findsOneWidget);
        final grid = tester.getRect(find.byType(GridView));
        expect(grid.height, greaterThanOrEqualTo(120));
        expect(grid.bottom, lessThanOrEqualTo(size.height));
        expect(tester.takeException(), isNull);
      });
    }
  }
}
