import 'dart:async';
import 'package:hawkr_api/gen/hawkr/social/v1alpha1/social.connect.client.dart';
import 'package:hawkr_api/gen/hawkr/social/v1alpha1/social.connect.spec.dart';
import 'package:hawkr_api/gen/hawkr/social/v1alpha1/social.pb.dart' as pb;
import 'package:hawkr_api/testing.dart';
import 'package:hawkr_seller/core/networking/connect_provider.dart';
import 'package:hawkr_seller/core/networking/dio_provider.dart';
import 'package:dio/dio.dart';
import 'package:hawkr_seller/features/post/presentation/widgets/hidden_post_badge.dart';
import 'package:hawkr_seller/features/post/presentation/widgets/page_home_tab.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_screenutil/flutter_screenutil.dart';
import 'package:flutter_test/flutter_test.dart';

void main() {
  TestWidgetsFlutterBinding.ensureInitialized();

  testWidgets(
    '500 posts paginate instead of fetching everything up front, and keep hidden state visible',
    (tester) async {
      tester.view.devicePixelRatio = 1;
      tester.view.physicalSize = const Size(390, 844);
      addTearDown(tester.view.resetDevicePixelRatio);
      addTearDown(tester.view.resetPhysicalSize);

      final posts = List.generate(
        500,
        (index) => pb.Post(
          id: '${index + 1}',
          body: '貼文 ${index + 1}',
          takedownMeasureId: index == 0 ? 'measure-1' : null,
        )..mergeFromProto3Json({'commentCount': '$index'}),
      );
      final requestedPageSizes = <int>[];
      final client = SocialAppServiceClient(
        FakeTransportBuilder()
            .unary(SocialAppService.listMyPosts, (request, _) {
              requestedPageSizes.add(request.pageSize);
              final start = request.pageToken.isEmpty
                  ? 0
                  : int.parse(request.pageToken);
              final end = (start + request.pageSize).clamp(0, posts.length);
              return pb.ListMyPostsResponse(
                posts: posts.sublist(start, end),
                nextPageToken: end < posts.length ? '$end' : '',
              );
            })
            .unary(
              SocialAppService.listPendingSchedules,
              (_, _) => pb.ListPendingSchedulesResponse(),
            )
            .build(),
      );
      var refreshCount = 0;

      await tester.pumpWidget(
        ProviderScope(
          overrides: [
            dioProvider.overrideWithValue(Dio()),
            socialClientProvider.overrideWithValue(client),
            socialPublicClientProvider.overrideWithValue(
              SocialPublicServiceClient(FakeTransportBuilder().build()),
            ),
          ],
          child: ScreenUtilInit(
            designSize: const Size(390, 844),
            builder: (_, _) => MaterialApp(
              home: Scaffold(
                body: PageHomeTab(
                  sellerName: '測試店家',
                  onReloadSeller: () async => refreshCount++,
                ),
              ),
            ),
          ),
        ),
      );
      await tester.pump();
      await tester.pump(const Duration(milliseconds: 50));

      // 初次載入只拉第 1 頁（20 筆），不是整批 500 筆——這是本次修法要守住的核心
      // 行為：舊實作對 500 篇貼文各自補打一次 detail API，總請求數會是 501。
      expect(requestedPageSizes, [20]);
      expect(find.byType(GridView), findsNothing);
      expect(find.byType(SliverGrid), findsOneWidget);
      expect(find.byType(HiddenPostBadge), findsOneWidget);

      final initiallyBuilt = _builtPostTiles(tester);
      expect(initiallyBuilt, isNotEmpty);
      expect(initiallyBuilt.length, lessThanOrEqualTo(20));

      final scroll = find.byKey(
        const PageStorageKey('storefront-post-grid-scroll'),
      );
      final beforeOffset = tester
          .state<ScrollableState>(
            find.descendant(of: scroll, matching: find.byType(Scrollable)),
          )
          .position
          .pixels;

      // 捲到接近底部觸發下一頁載入。
      await tester.drag(scroll, const Offset(0, -2000));
      await tester.pump();
      await tester.pump(const Duration(milliseconds: 50));

      final afterOffset = tester
          .state<ScrollableState>(
            find.descendant(of: scroll, matching: find.byType(Scrollable)),
          )
          .position
          .pixels;
      expect(afterOffset, greaterThan(beforeOffset));

      // 觸發了第 2 頁，且拿到的還是 20 筆（不是整批剩餘的 480 筆）。
      expect(requestedPageSizes.length, greaterThanOrEqualTo(2));
      expect(requestedPageSizes.length, lessThan(25));
      expect(requestedPageSizes.every((size) => size == 20), isTrue);

      final refresh = tester.state<RefreshIndicatorState>(
        find.byType(RefreshIndicator),
      );
      unawaited(refresh.show());
      await tester.pump();
      await tester.pump(const Duration(seconds: 2));
      // FruitRefreshIndicator 的 onRefresh 呼叫的是 widget.onReloadSeller
      // （父層擁有的整頁重載，非本頁自己的 _loadPosts），本測試只釘住這條
      // callback 有被呼叫——跟改動前的行為一致，不是本次修法要驗證的範圍。
      expect(refreshCount, 1);
      expect(tester.takeException(), isNull);
    },
  );

  testWidgets('R2-6 each page reads pending schedules again', (tester) async {
    tester.view.devicePixelRatio = 1;
    tester.view.physicalSize = const Size(390, 844);
    addTearDown(tester.view.resetDevicePixelRatio);
    addTearDown(tester.view.resetPhysicalSize);

    final posts = List.generate(
      60,
      (index) => pb.Post(id: '${index + 1}', body: '貼文 ${index + 1}'),
    );
    var postPages = 0;
    var scheduleReads = 0;
    final client = SocialAppServiceClient(
      FakeTransportBuilder()
          .unary(SocialAppService.listMyPosts, (request, _) {
            postPages++;
            final start = request.pageToken.isEmpty
                ? 0
                : int.parse(request.pageToken);
            final end = (start + request.pageSize).clamp(0, posts.length);
            return pb.ListMyPostsResponse(
              posts: posts.sublist(start, end),
              nextPageToken: end < posts.length ? '$end' : '',
            );
          })
          .unary(SocialAppService.listPendingSchedules, (_, _) {
            scheduleReads++;
            return pb.ListPendingSchedulesResponse();
          })
          .build(),
    );

    await tester.pumpWidget(
      ProviderScope(
        overrides: [
          dioProvider.overrideWithValue(Dio()),
          socialClientProvider.overrideWithValue(client),
          socialPublicClientProvider.overrideWithValue(
            SocialPublicServiceClient(FakeTransportBuilder().build()),
          ),
        ],
        child: ScreenUtilInit(
          designSize: const Size(390, 844),
          builder: (_, _) => MaterialApp(
            home: Scaffold(
              body: PageHomeTab(
                sellerName: '測試店家',
                onReloadSeller: () async {},
              ),
            ),
          ),
        ),
      ),
    );
    await tester.pump();
    await tester.pump(const Duration(milliseconds: 50));
    await tester.drag(
      find.byKey(const PageStorageKey('storefront-post-grid-scroll')),
      const Offset(0, -2000),
    );
    await tester.pump();
    await tester.pump(const Duration(milliseconds: 50));

    expect(postPages, greaterThanOrEqualTo(2));
    expect(scheduleReads, postPages);
  });
}

Set<String> _builtPostTiles(WidgetTester tester) {
  return tester
      .widgetList<Material>(
        find.byWidgetPredicate(
          (widget) =>
              widget is Material &&
              widget.key is ValueKey<String> &&
              (widget.key! as ValueKey<String>).value.startsWith(
                'storefront-post-grid-item-',
              ),
        ),
      )
      .map((widget) => (widget.key! as ValueKey<String>).value)
      .toSet();
}
