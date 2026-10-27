import 'dart:async';
import 'dart:convert';
import 'dart:typed_data';

import 'package:hawkr_seller/core/networking/dio_provider.dart';
import 'package:hawkr_seller/features/post/presentation/widgets/hidden_post_badge.dart';
import 'package:hawkr_seller/features/post/presentation/widgets/page_home_tab.dart';
import 'package:dio/dio.dart';
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
        (index) => <String, dynamic>{
          'id': index + 1,
          'content': '貼文 ${index + 1}',
          'contentType': 'text',
          'commentCount': index,
          if (index == 0) 'hiddenAt': 1,
          if (index == 0) 'hidden_reason': '測試下架原因',
        },
      );
      final adapter = _PaginatedPostListAdapter(posts, pageSize: 20);
      final dio = Dio(BaseOptions(baseUrl: 'https://example.test'))
        ..httpClientAdapter = adapter;
      var refreshCount = 0;

      await tester.pumpWidget(
        ProviderScope(
          overrides: [dioProvider.overrideWithValue(dio)],
          child: ScreenUtilInit(
            designSize: const Size(390, 844),
            builder: (_, __) => MaterialApp(
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
      expect(adapter.requestCount, 1);
      expect(adapter.requestedPageSizes, [20]);
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
      expect(adapter.requestCount, 2);
      expect(adapter.requestedPageSizes, [20, 20]);

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

/// 模擬 gateway 分頁 envelope（`data: {records, total, page, pageSize}`），
/// 依 query 的 page/page_size 切片回傳，讓測試能驗證「一次只拉一頁」。
class _PaginatedPostListAdapter implements HttpClientAdapter {
  _PaginatedPostListAdapter(this.posts, {required this.pageSize});

  final List<Map<String, dynamic>> posts;
  final int pageSize;
  int requestCount = 0;
  final List<int> requestedPageSizes = [];

  @override
  Future<ResponseBody> fetch(
    RequestOptions options,
    Stream<Uint8List>? requestStream,
    Future<void>? cancelFuture,
  ) async {
    requestCount++;
    expect(options.path, '/api/v1/app/sellers/me/posts');
    final page = int.tryParse('${options.queryParameters['page'] ?? '1'}') ?? 1;
    final size =
        int.tryParse(
          '${options.queryParameters['page_size'] ?? '$pageSize'}',
        ) ??
        pageSize;
    requestedPageSizes.add(size);
    final start = (page - 1) * size;
    final end = (start + size).clamp(0, posts.length);
    final slice = start >= posts.length ? const [] : posts.sublist(start, end);
    return ResponseBody.fromString(
      jsonEncode({
        'code': 0,
        'data': {
          'records': slice,
          'total': '${posts.length}',
          'page': '$page',
          'pageSize': '$size',
        },
      }),
      200,
      headers: {
        Headers.contentTypeHeader: [Headers.jsonContentType],
      },
    );
  }

  @override
  void close({bool force = false}) {}
}
