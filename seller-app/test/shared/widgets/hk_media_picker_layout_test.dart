import 'package:hawkr_seller/core/theme/app_theme.dart';
import 'package:hawkr_seller/shared/widgets/hk_media_picker_layout.dart';
import 'package:flutter/material.dart';
import 'package:flutter_screenutil/flutter_screenutil.dart';
import 'package:flutter_test/flutter_test.dart';

void main() {
  for (final size in [
    const Size(402, 874),
    const Size(874, 402),
    const Size(320, 568),
    const Size(320, 300),
    const Size(768, 1024),
  ]) {
    testWidgets('preview and scrollable selectable gallery fit $size', (
      tester,
    ) async {
      tester.view.devicePixelRatio = 1;
      tester.view.physicalSize = size;
      addTearDown(tester.view.resetDevicePixelRatio);
      addTearDown(tester.view.resetPhysicalSize);
      int? selected;
      await tester.pumpWidget(
        ScreenUtilInit(
          designSize: const Size(414, 896),
          enableScaleWH: () => false,
          enableScaleText: () => false,
          builder: (_, _) => MaterialApp(
            theme: AppTheme.light,
            builder: (context, child) => MediaQuery(
              data: MediaQuery.of(context).copyWith(
                padding: const EdgeInsets.only(left: 20, right: 20, bottom: 34),
                textScaler: const TextScaler.linear(1.4),
              ),
              child: child!,
            ),
            home: Scaffold(
              appBar: AppBar(title: const Text('選擇照片')),
              body: HkMediaPickerLayout(
                preview: const ColoredBox(color: Colors.black),
                grid: GridView.builder(
                  gridDelegate: const SliverGridDelegateWithFixedCrossAxisCount(
                    crossAxisCount: 4,
                  ),
                  itemCount: 40,
                  itemBuilder: (_, index) => InkWell(
                    key: ValueKey('photo-$index'),
                    onTap: () => selected = index,
                    child: Center(child: Text('$index')),
                  ),
                ),
              ),
            ),
          ),
        ),
      );
      final preview = tester.getRect(
        find.byKey(const ValueKey('media-picker-preview')),
      );
      final grid = tester.getRect(
        find.byKey(const ValueKey('media-picker-grid')),
      );
      expect(preview.height, greaterThan(0));
      expect(grid.height, greaterThan(48));
      expect(grid.left, greaterThanOrEqualTo(20));
      expect(grid.right, lessThanOrEqualTo(size.width - 20));
      expect(grid.bottom, lessThanOrEqualTo(size.height - 34));
      expect(preview.overlaps(grid), isFalse);
      if (size == const Size(402, 874)) {
        expect(
          preview.height,
          preview.width,
          reason: 'Tall portrait retains its original square preview',
        );
      }
      if (size.width > size.height && size.width >= 480) {
        expect(grid.left, greaterThanOrEqualTo(preview.right));
        expect(preview.height, preview.width);
      }
      await tester.scrollUntilVisible(
        find.byKey(const ValueKey('photo-39')),
        200,
        scrollable: find.byType(Scrollable),
      );
      await tester.pumpAndSettle();
      await tester.ensureVisible(find.byKey(const ValueKey('photo-39')));
      await tester.pumpAndSettle();
      await tester.tap(find.byKey(const ValueKey('photo-39')));
      expect(selected, 39);
      expect(tester.takeException(), isNull);
    });
  }
}
