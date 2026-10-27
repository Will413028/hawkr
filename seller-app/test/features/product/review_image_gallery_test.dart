import 'package:hawkr_seller/features/product/presentation/widgets/review_image_gallery.dart';
import 'package:flutter/material.dart';
import 'package:flutter_screenutil/flutter_screenutil.dart';
import 'package:flutter_test/flutter_test.dart';

void main() {
  Widget imageBuilder(BuildContext context, String source, BoxFit fit) {
    return ColoredBox(
      key: ValueKey('test-image-$source-${fit.name}'),
      color: source == 'first' ? Colors.red : Colors.blue,
    );
  }

  Widget testApp(List<String> images) {
    return ScreenUtilInit(
      designSize: const Size(390, 844),
      enableScaleWH: () => false,
      enableScaleText: () => false,
      builder: (_, __) => MaterialApp(
        home: Scaffold(
          body: ReviewImageGallery(images: images, imageBuilder: imageBuilder),
        ),
      ),
    );
  }

  testWidgets('review thumbnails open a light zoomable gallery', (
    tester,
  ) async {
    tester.view.physicalSize = const Size(390, 844);
    tester.view.devicePixelRatio = 1;
    addTearDown(tester.view.resetPhysicalSize);
    addTearDown(tester.view.resetDevicePixelRatio);

    final semantics = tester.ensureSemantics();
    await tester.pumpWidget(testApp(const ['first', 'second']));

    expect(find.byKey(const ValueKey('review-image-gallery')), findsOneWidget);
    expect(find.bySemanticsLabel('查看第 2 張評價圖片，共 2 張'), findsOneWidget);

    await tester.tap(find.byKey(const ValueKey('review-image-thumbnail-1')));
    await tester.pumpAndSettle();

    expect(
      find.byKey(const ValueKey('review-image-viewer-dialog')),
      findsOneWidget,
    );
    expect(find.text('2 / 2'), findsOneWidget);
    expect(find.byType(InteractiveViewer), findsWidgets);
    expect(find.text('可左右切換、雙指縮放'), findsNothing);
    expect(find.byKey(const ValueKey('review-image-previous')), findsNothing);
    expect(find.byKey(const ValueKey('review-image-next')), findsNothing);

    final barrier = tester.widget<ModalBarrier>(find.byType(ModalBarrier).last);
    expect(barrier.color?.a, closeTo(0.18, 0.01));

    await tester.drag(
      find.byKey(const ValueKey('review-image-viewer-pages')),
      const Offset(300, 0),
    );
    await tester.pumpAndSettle();
    expect(find.text('1 / 2'), findsOneWidget);

    await tester.tap(find.byKey(const ValueKey('review-image-viewer-close')));
    await tester.pumpAndSettle();
    expect(
      find.byKey(const ValueKey('review-image-viewer-dialog')),
      findsNothing,
    );
    semantics.dispose();
  });

  testWidgets('single review image keeps an accessible 88pt tap target', (
    tester,
  ) async {
    await tester.pumpWidget(testApp(const ['first']));

    final thumbnail = tester.getSize(
      find.byKey(const ValueKey('review-image-thumbnail-0')),
    );
    expect(thumbnail.width, 88);
    expect(thumbnail.height, 88);
  });

  testWidgets('image viewer stays stable on small portrait and landscape', (
    tester,
  ) async {
    addTearDown(tester.view.resetPhysicalSize);
    addTearDown(tester.view.resetDevicePixelRatio);
    tester.view.devicePixelRatio = 1;

    for (final size in const [Size(320, 568), Size(844, 390)]) {
      tester.view.physicalSize = size;
      await tester.pumpWidget(testApp(const ['first']));
      await tester.tap(find.byKey(const ValueKey('review-image-thumbnail-0')));
      await tester.pumpAndSettle();

      expect(
        find.byKey(const ValueKey('review-image-viewer-dialog')),
        findsOneWidget,
      );
      expect(tester.takeException(), isNull, reason: 'failed at $size');

      await tester.tap(find.byKey(const ValueKey('review-image-viewer-close')));
      await tester.pumpAndSettle();
    }
  });
}
