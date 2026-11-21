import 'package:hawkr_seller/core/router/app_router.dart';
import 'package:hawkr_seller/features/seller/presentation/profile_other_options_card.dart';
import 'package:hawkr_seller/features/seller/presentation/widgets/option_row.dart';
import 'package:hawkr_seller/shared/widgets/hk_icon.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:go_router/go_router.dart';

Future<void> _pump(
  WidgetTester tester, {
  bool canManageBlocks = true,
  int pendingInquiries = 0,
}) async {
  tester.view.physicalSize = const Size(800, 3000);
  tester.view.devicePixelRatio = 1;
  addTearDown(tester.view.reset);
  final router = GoRouter(
    routes: [
      GoRoute(
        path: '/',
        builder: (_, _) => Scaffold(
          body: SingleChildScrollView(
            child: ProfileOtherOptionsCard(
              onShippingTap: () {},
              canManageBlocks: canManageBlocks,
              pendingInquiries: pendingInquiries,
            ),
          ),
        ),
      ),
      GoRoute(
        path: SellerRoutes.inquiries,
        builder: (_, _) => const Scaffold(body: Text('INQUIRIES')),
      ),
      GoRoute(
        path: SellerRoutes.measures,
        builder: (_, _) => const Scaffold(body: Text('VIOLATIONS')),
      ),
    ],
  );
  addTearDown(router.dispose);
  await tester.pumpWidget(
    ProviderScope(child: MaterialApp.router(routerConfig: router)),
  );
  await tester.pumpAndSettle();
}

void main() {
  testWidgets('營運設定有「措施與申覆」（附說明與圖示），在「封鎖名單」之前', (tester) async {
    await _pump(tester);

    expect(find.text('措施與申覆'), findsOneWidget);
    expect(find.text('查看平台的措施、期間與申覆進度'), findsOneWidget);
    final measures = tester.getTopLeft(find.text('措施與申覆')).dy;
    final blocks = tester.getTopLeft(find.text('封鎖名單')).dy;
    expect(measures, lessThan(blocks));

    final row = find.ancestor(
      of: find.text('措施與申覆'),
      matching: find.byType(OptionRow),
    );
    expect(
      find.descendant(of: row, matching: find.byType(HkIcon)),
      findsWidgets,
    );
  });

  testWidgets('點下去開措施與申覆列表', (tester) async {
    await _pump(tester);

    await tester.tap(find.text('措施與申覆'));
    await tester.pumpAndSettle();

    expect(find.text('VIOLATIONS'), findsOneWidget);
  });

  testWidgets('每一列都是 button semantics，最小高度 72', (tester) async {
    final semantics = tester.ensureSemantics();
    await _pump(tester);

    final row = find.ancestor(
      of: find.text('措施與申覆'),
      matching: find.byType(OptionRow),
    );
    expect(tester.getSize(row).height, greaterThanOrEqualTo(72));
    final node = tester.getSemantics(row);
    expect(node.flagsCollection.isButton, isTrue);
    semantics.dispose();
  });

  testWidgets('沒有內容契約能力時不顯示「封鎖名單」', (tester) async {
    await _pump(tester, canManageBlocks: false);
    expect(find.text('封鎖名單'), findsNothing);
    expect(find.text('措施與申覆'), findsOneWidget);
  });

  testWidgets('「平台詢問」顯示待回覆數的徽章與說明，點下去開列表', (tester) async {
    await _pump(tester, pendingInquiries: 3);
    final row = find.ancestor(
      of: find.text('平台詢問'),
      matching: find.byType(OptionRow),
    );
    expect(tester.widget<OptionRow>(row).badgeCount, 3);
    expect(find.text('有 3 則待回覆'), findsOneWidget);
    await tester.tap(find.text('平台詢問'));
    await tester.pumpAndSettle();
    expect(find.text('INQUIRIES'), findsOneWidget);
  });

  testWidgets('沒有待回覆時「平台詢問」不顯示徽章數', (tester) async {
    await _pump(tester);
    final row = find.ancestor(
      of: find.text('平台詢問'),
      matching: find.byType(OptionRow),
    );
    expect(tester.widget<OptionRow>(row).badgeCount, 0);
  });
}
