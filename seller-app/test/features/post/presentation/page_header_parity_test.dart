import 'package:hawkr_seller/features/post/presentation/widgets/page_header.dart';
import 'package:flutter/material.dart';
import 'package:flutter_screenutil/flutter_screenutil.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:hooks_riverpod/hooks_riverpod.dart';

void main() {
  testWidgets('seller store header uses buyer centered profile layout', (
    tester,
  ) async {
    tester.view.devicePixelRatio = 1;
    tester.view.physicalSize = const Size(390, 844);
    addTearDown(tester.view.resetDevicePixelRatio);
    addTearDown(tester.view.resetPhysicalSize);

    await tester.pumpWidget(const ProviderScope(child: _HeaderFixture()));
    await tester.pump();

    expect(
      find.byKey(const Key('seller_store_centered_identity')),
      findsOneWidget,
    );
    expect(find.text('一起吃水果'), findsOneWidget);
    expect(find.text('基隆市信義區'), findsOneWidget);
    expect(find.text('商家介紹'), findsNothing);
    expect(find.text('產地直送的當季水果。'), findsNothing);
    expect(
      tester.getSize(find.byKey(const ValueKey('store-identity-avatar-frame'))),
      const Size.square(88),
    );
    expect(tester.takeException(), isNull);
  });

  testWidgets('seller store header stays centered on accessibility layout', (
    tester,
  ) async {
    tester.view.devicePixelRatio = 1;
    tester.view.physicalSize = const Size(320, 720);
    addTearDown(tester.view.resetDevicePixelRatio);
    addTearDown(tester.view.resetPhysicalSize);

    await tester.pumpWidget(
      ProviderScope(
        child: ScreenUtilInit(
          designSize: const Size(393, 852),
          minTextAdapt: true,
          builder: (_, _) => MaterialApp(
            builder: (context, child) => MediaQuery(
              data: MediaQuery.of(
                context,
              ).copyWith(textScaler: const TextScaler.linear(2)),
              child: child!,
            ),
            home: const _HeaderScaffold(),
          ),
        ),
      ),
    );
    await tester.pump();

    expect(
      find.byKey(const Key('seller_store_centered_identity')),
      findsOneWidget,
    );
    expect(find.text('該商家沒有填寫'), findsNothing);
    expect(tester.takeException(), isNull);
  });
}

class _HeaderFixture extends StatelessWidget {
  const _HeaderFixture();

  @override
  Widget build(BuildContext context) {
    return ScreenUtilInit(
      designSize: const Size(393, 852),
      minTextAdapt: true,
      builder: (_, _) => const MaterialApp(home: _HeaderScaffold()),
    );
  }
}

class _HeaderScaffold extends StatelessWidget {
  const _HeaderScaffold();

  @override
  Widget build(BuildContext context) {
    final spec = PageHeaderLayoutSpec.of(context);
    final expandedHeight = spec.expandedAppBarHeightFor(
      hasAward: false,
      displayedLocation: '基隆市信義區',
      description: '產地直送的當季水果。',
    );
    return Scaffold(
      body: CustomScrollView(
        slivers: [
          SliverAppBar(
            pinned: true,
            expandedHeight: expandedHeight,
            collapsedHeight: PageHeaderLayoutSpec.topBarHeight,
            toolbarHeight: PageHeaderLayoutSpec.topBarHeight,
            flexibleSpace: const PageHeader(
              sellerName: '一起吃水果',
              fansCount: 12,
              location: '基隆市信義區',
              description: '產地直送的當季水果。',
            ),
          ),
          const SliverToBoxAdapter(child: SizedBox(height: 800)),
        ],
      ),
    );
  }
}
