import 'package:hawkr_seller/core/router/app_router.dart';
import 'package:hawkr_seller/features/seller/presentation/profile_other_options_card.dart';
import 'package:hawkr_seller/features/seller/presentation/widgets/option_row.dart';
import 'package:hawkr_seller/shared/widgets/hk_icon.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:go_router/go_router.dart';

Future<void> _pump(WidgetTester tester) async {
  tester.view.physicalSize = const Size(800, 3000);
  tester.view.devicePixelRatio = 1;
  addTearDown(tester.view.reset);
  final router = GoRouter(
    routes: [
      GoRoute(
        path: '/',
        builder: (_, _) => Scaffold(
          body: SingleChildScrollView(
            child: ProfileOtherOptionsCard(onShippingTap: () {}),
          ),
        ),
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
  testWidgets('營運設定有「措施與申覆」（附說明與圖示），緊接在「我的檢舉」之後', (tester) async {
    await _pump(tester);

    expect(find.text('措施與申覆'), findsOneWidget);
    expect(find.text('查看平台的措施、期間與申覆進度'), findsOneWidget);
    final report = tester.getTopLeft(find.text('我的檢舉')).dy;
    final measures = tester.getTopLeft(find.text('措施與申覆')).dy;
    final blocks = tester.getTopLeft(find.text('封鎖名單')).dy;
    expect(report, lessThan(measures));
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
}
