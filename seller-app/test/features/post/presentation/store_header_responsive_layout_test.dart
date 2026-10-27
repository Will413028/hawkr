import 'package:hawkr_seller/core/theme/app_theme.dart';
import 'package:hawkr_seller/features/post/presentation/widgets/page_header.dart';
import 'package:hawkr_seller/features/post/presentation/widgets/page_tab_bar.dart';
import 'package:hawkr_seller/shared/widgets/storefront_top_bar_icon_button.dart';
import 'package:flutter/material.dart';
import 'package:flutter_screenutil/flutter_screenutil.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:hooks_riverpod/hooks_riverpod.dart';

void main() {
  for (final scenario in const [
    (size: Size(320, 568), fans: 220000000, award: _award, scale: 1.4),
    (size: Size(402, 874), fans: 220000000, award: _award, scale: 1.4),
    (size: Size(874, 402), fans: 220000000, award: _award, scale: 1.4),
    (size: Size(402, 874), fans: 999, award: '優良獎項', scale: 1.4),
    (size: Size(320, 568), fans: 999, award: _award, scale: 0.8),
  ]) {
    final size = scenario.size;
    testWidgets(
      'store header preserves identity and circular controls at $size / ${scenario.fans} / ${scenario.scale}',
      (tester) async {
        tester.view
          ..physicalSize = size
          ..devicePixelRatio = 1
          ..padding = const FakeViewPadding(top: 20, bottom: 34)
          ..viewPadding = const FakeViewPadding(top: 20, bottom: 34);
        addTearDown(tester.view.reset);
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
                  ).copyWith(textScaler: TextScaler.linear(scenario.scale)),
                  child: child!,
                ),
                home: _HeaderPage(award: scenario.award, fans: scenario.fans),
              ),
            ),
          ),
        );
        await tester.pump();
        expect(tester.takeException(), isNull);
        final identity = tester.getRect(
          find.byKey(const Key('seller_store_centered_identity')),
        );
        final award = tester.getRect(find.text(scenario.award));
        expect(award.bottom, lessThanOrEqualTo(identity.bottom));
        final tabs = tester.getRect(find.byType(PageTabBar));
        expect(identity.bottom, lessThanOrEqualTo(tabs.top + 1));
        for (final control in tester.widgetList<StorefrontTopBarIconButton>(
          find.byType(StorefrontTopBarIconButton),
        )) {
          expect(tester.getSize(find.byWidget(control)), const Size.square(48));
        }
        await tester.drag(find.byType(CustomScrollView), const Offset(0, -500));
        await tester.pumpAndSettle();
        expect(find.byTooltip('返回').hitTestable(), findsOneWidget);
        expect(find.byTooltip('聊天').hitTestable(), findsOneWidget);
        expect(tester.takeException(), isNull);
      },
    );
  }
}

const _location = '測試縣市友善農產專區第十二鄰農園路十八號';
const _award = '產地友善耕作年度優良農產品獎項';

class _HeaderPage extends StatefulWidget {
  const _HeaderPage({required this.award, required this.fans});
  final String award;
  final int fans;
  @override
  State<_HeaderPage> createState() => _HeaderPageState();
}

class _HeaderPageState extends State<_HeaderPage>
    with SingleTickerProviderStateMixin {
  late final controller = TabController(length: 3, vsync: this);
  @override
  void dispose() {
    controller.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    final spec = PageHeaderLayoutSpec.of(context);
    return Scaffold(
      body: CustomScrollView(
        slivers: [
          SliverAppBar(
            pinned: true,
            expandedHeight: spec.expandedAppBarHeightFor(
              hasAward: true,
              awardText: widget.award,
              fansCount: widget.fans,
              displayedLocation: _location,
              description: '',
            ),
            collapsedHeight: PageHeaderLayoutSpec.topBarHeight,
            toolbarHeight: PageHeaderLayoutSpec.topBarHeight,
            automaticallyImplyLeading: false,
            flexibleSpace: PageHeader(
              sellerName: '範例生鮮友善農產直送測試商店',
              location: _location,
              prize: widget.award,
              fansCount: widget.fans,
              onBackPressed: () {},
              onChat: () {},
            ),
            bottom: PageTabBar(tabController: controller),
          ),
          const SliverToBoxAdapter(child: SizedBox(height: 1200)),
        ],
      ),
    );
  }
}
