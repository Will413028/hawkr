import 'package:hawkr_seller/core/theme/app_colors.dart';
import 'package:hawkr_seller/core/theme/app_theme.dart';
import 'package:hawkr_seller/features/post/presentation/widgets/page_header.dart';
import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_screenutil/flutter_screenutil.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:hooks_riverpod/hooks_riverpod.dart';

void main() {
  for (final device in const [
    (name: 'compact phone', size: Size(320, 568), top: 20.0, ratio: 2.0),
    (name: 'notched phone', size: Size(393, 852), top: 59.0, ratio: 3.0),
    (name: 'iPhone 16 Pro', size: Size(402, 874), top: 62.0, ratio: 3.0),
  ]) {
    testWidgets(
      '${device.name} changes status contrast exactly when the cover scrolls away',
      (tester) async {
        addTearDown(tester.view.reset);
        final padding = FakeViewPadding(
          top: device.top * device.ratio,
          bottom: 34 * device.ratio,
        );
        tester.view
          ..devicePixelRatio = device.ratio
          ..physicalSize = device.size * device.ratio
          ..padding = padding
          ..viewPadding = padding;
        final controller = ScrollController();
        addTearDown(controller.dispose);

        await tester.pumpWidget(_HeaderTestApp(controller: controller));
        await tester.pump();

        final banner = find.byKey(
          const ValueKey('seller-store-banner-system-ui'),
        );
        final bannerBottom = tester.getRect(banner).bottom;
        final statusSampleY = device.top / 2;
        expect(
          SystemChrome.latestStyle?.statusBarBrightness,
          Brightness.dark,
          reason: 'The dark cover initially needs white iOS status icons.',
        );

        controller.jumpTo(bannerBottom - statusSampleY - 1);
        await tester.pump();
        expect(
          SystemChrome.latestStyle?.statusBarBrightness,
          Brightness.dark,
          reason: 'The cover still spans the actual status-bar sample point.',
        );

        controller.jumpTo(bannerBottom - statusSampleY + 1);
        await tester.pump();
        expect(
          SystemChrome.latestStyle?.statusBarBrightness,
          Brightness.light,
          reason:
              'Cream profile details need dark icons, even with the '
              'rest of the header still visible.',
        );
        expect(tester.getRect(find.byType(PageHeader)).bottom, greaterThan(0));

        controller.jumpTo(600);
        await tester.pump();
        expect(
          SystemChrome.latestStyle?.statusBarBrightness,
          Brightness.light,
          reason: 'A clipped, offscreen cover must not leak its annotation.',
        );

        controller.jumpTo(0);
        await tester.pump();
        expect(
          SystemChrome.latestStyle?.statusBarBrightness,
          Brightness.dark,
          reason: 'Returning to the cover restores white icons.',
        );

        await tester.pumpWidget(
          _HeaderTestApp(controller: controller, pinned: true),
        );
        controller.jumpTo(600);
        await tester.pump();
        expect(
          SystemChrome.latestStyle?.statusBarBrightness,
          Brightness.dark,
          reason:
              'The store keeps its dark cover pinned under the status bar '
              'after collapse, so its icons stay white.',
        );
        expect(tester.takeException(), isNull);
        await tester.pumpWidget(const SizedBox.shrink());
      },
      variant: const TargetPlatformVariant({TargetPlatform.iOS}),
    );
  }
}

class _HeaderTestApp extends StatelessWidget {
  const _HeaderTestApp({required this.controller, this.pinned = false});

  final ScrollController controller;
  final bool pinned;

  @override
  Widget build(BuildContext context) => ProviderScope(
    child: ScreenUtilInit(
      designSize: const Size(414, 896),
      enableScaleWH: () => false,
      enableScaleText: () => false,
      builder: (_, _) => MaterialApp(
        theme: AppTheme.light,
        home: _ScrollingProfileHeader(controller: controller, pinned: pinned),
      ),
    ),
  );
}

/// Uses the same unpinned header structure as ProfileScreen, without its data
/// loading or backend side effects. The production PageHeader supplies the
/// region whose composited status style is verified above.
class _ScrollingProfileHeader extends StatelessWidget {
  const _ScrollingProfileHeader({
    required this.controller,
    this.pinned = false,
  });

  final ScrollController controller;
  final bool pinned;

  @override
  Widget build(BuildContext context) {
    final spec = PageHeaderLayoutSpec.of(context);
    final height =
        MediaQuery.viewPaddingOf(context).top +
        spec.expandedProfileHeightFor(
          hasAward: false,
          displayedLocation: '基隆市信義區',
          description: '',
        );
    return AnnotatedRegion<SystemUiOverlayStyle>(
      value: SystemUiOverlayStyle.dark.copyWith(
        statusBarColor: Colors.transparent,
      ),
      child: Scaffold(
        backgroundColor: AppColors.bgPrimary,
        body: CustomScrollView(
          controller: controller,
          slivers: [
            if (pinned)
              SliverAppBar(
                pinned: true,
                expandedHeight: height - MediaQuery.viewPaddingOf(context).top,
                collapsedHeight: PageHeaderLayoutSpec.topBarHeight,
                toolbarHeight: PageHeaderLayoutSpec.topBarHeight,
                systemOverlayStyle: SystemUiOverlayStyle.light.copyWith(
                  statusBarColor: Colors.transparent,
                ),
                flexibleSpace: const PageHeader(
                  sellerName: '一起吃水果',
                  location: '基隆市信義區',
                ),
              )
            else
              SliverToBoxAdapter(
                child: SizedBox(
                  height: height,
                  child: const PageHeader(
                    sellerName: '一起吃水果',
                    location: '基隆市信義區',
                  ),
                ),
              ),
            const SliverToBoxAdapter(child: SizedBox(height: 1200)),
          ],
        ),
      ),
    );
  }
}
