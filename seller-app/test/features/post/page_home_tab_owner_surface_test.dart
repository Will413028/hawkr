import 'package:hawkr_seller/config/flavor.dart';
import 'package:hawkr_seller/core/router/app_router.dart';
import 'package:hawkr_seller/core/theme/app_theme.dart';
import 'package:hawkr_seller/features/post/data/post_repository.dart';
import 'package:hawkr_seller/features/post/presentation/widgets/page_home_tab.dart';
import 'package:flutter/material.dart';
import 'package:flutter_screenutil/flutter_screenutil.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:go_router/go_router.dart';
import 'package:hooks_riverpod/hooks_riverpod.dart';
import 'package:mocktail/mocktail.dart';

class _Repo extends Mock implements PostRepository {}

void main() {
  setUpAll(() {
    if (!F.isInitialized) F.init(Flavor.dev);
  });

  Future<List<Object?>> pumpGrid(
    WidgetTester tester,
    List<Map<String, dynamic>> posts,
  ) async {
    tester.view.physicalSize = const Size(390, 844);
    tester.view.devicePixelRatio = 1;
    addTearDown(tester.view.resetPhysicalSize);
    addTearDown(tester.view.resetDevicePixelRatio);
    final repo = _Repo();
    when(
      () => repo.listMyPostsRaw(
        page: any(named: 'page'),
        pageSize: any(named: 'pageSize'),
      ),
    ).thenAnswer((_) async => PostPageResult(posts: posts, hasMore: false));
    when(() => repo.listComments(any())).thenAnswer((_) async => []);
    final extras = <Object?>[];
    final router = GoRouter(
      routes: [
        GoRoute(
          path: '/',
          builder: (_, _) => Scaffold(
            body: PageHomeTab(sellerName: '測試店家', onReloadSeller: () async {}),
          ),
        ),
        GoRoute(
          path: PostRoutes.feed,
          builder: (_, state) {
            extras.add(state.extra);
            return const Text('feed');
          },
        ),
      ],
    );
    addTearDown(router.dispose);
    await tester.pumpWidget(
      ProviderScope(
        overrides: [postRepositoryProvider.overrideWithValue(repo)],
        child: ScreenUtilInit(
          designSize: const Size(390, 844),
          enableScaleWH: () => false,
          enableScaleText: () => false,
          builder: (_, _) =>
              MaterialApp.router(theme: AppTheme.light, routerConfig: router),
        ),
      ),
    );
    await tester.pumpAndSettle();
    return extras;
  }

  testWidgets('點格子帶 postId 開全螢幕頁（前面有被平台下架的貼文也對得上）', (tester) async {
    final extras = await pumpGrid(tester, [
      {'id': 41, 'contentType': 'text', 'hiddenAt': 1760000000},
      {'id': 42, 'contentType': 'text', 'publishStatus': '4402'},
    ]);

    await tester.tap(
      find.byKey(const ValueKey('storefront-post-grid-item-42')),
    );
    await tester.pumpAndSettle();

    expect(find.text('feed'), findsOneWidget);
    expect(extras.single, {'postId': 42, 'sellerName': '測試店家'});
  });

  testWidgets('平台下架的格子顯示「平台下架」角標', (tester) async {
    await pumpGrid(tester, [
      {'id': 41, 'contentType': 'text', 'hiddenAt': 1760000000},
    ]);
    expect(find.text('平台下架'), findsOneWidget);
    expect(find.text('已下架'), findsNothing);
  });

  testWidgets('長按預覽卡沒有按讚入口，讚數只顯示', (tester) async {
    await pumpGrid(tester, [
      {
        'id': 42,
        'contentType': 'text',
        'publishStatus': '4402',
        'likeCount': 7,
        'content': '貼文內容',
      },
    ]);
    await tester.longPress(
      find.byKey(const ValueKey('storefront-post-grid-item-42')),
    );
    await tester.pumpAndSettle();

    expect(find.text('7 個讚'), findsOneWidget);
    expect(find.bySemanticsLabel('共 7 個讚'), findsOneWidget);
    expect(find.bySemanticsLabel(RegExp('按讚')), findsNothing);
  });
}
