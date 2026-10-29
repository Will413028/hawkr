import 'dart:convert';
import 'dart:typed_data';

import 'package:hawkr_buyer/config/flavor.dart';
import 'package:hawkr_buyer/core/exceptions/app_exception.dart';
import 'package:hawkr_buyer/core/networking/dio_provider.dart';
import 'package:hawkr_buyer/features/social/data/social_repository.dart';
import 'package:hawkr_buyer/features/social/presentation/post_detail_screen.dart';
import '../../support/fake_seller_repository.dart';
import 'package:dio/dio.dart';
import 'package:flutter/material.dart';
import 'package:flutter_screenutil/flutter_screenutil.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:go_router/go_router.dart';
import 'package:hooks_riverpod/hooks_riverpod.dart';

/// 回固定的狀態碼與 envelope。
class _StatusAdapter implements HttpClientAdapter {
  _StatusAdapter(this.status, this.body);

  final int status;
  final Map<String, Object?> body;
  int requests = 0;

  @override
  Future<ResponseBody> fetch(
    RequestOptions options,
    Stream<Uint8List>? requestStream,
    Future<void>? cancelFuture,
  ) async {
    requests++;
    return ResponseBody.fromString(
      jsonEncode(body),
      status,
      headers: {
        Headers.contentTypeHeader: [Headers.jsonContentType],
      },
    );
  }

  @override
  void close({bool force = false}) {}
}

Future<void> _pumpDetail(WidgetTester tester, HttpClientAdapter adapter) async {
  tester.view.devicePixelRatio = 1;
  tester.view.physicalSize = const Size(390, 844);
  addTearDown(tester.view.resetDevicePixelRatio);
  addTearDown(tester.view.resetPhysicalSize);
  final dio = Dio(BaseOptions(baseUrl: 'https://example.test'))
    ..httpClientAdapter = adapter;
  addTearDown(dio.close);
  final router = GoRouter(
    initialLocation: '/social/post/101',
    routes: [
      GoRoute(
        path: '/social/post/:postId',
        builder: (_, state) => PostDetailScreen(
          postId: int.parse(state.pathParameters['postId']!),
        ),
      ),
    ],
  );
  addTearDown(router.dispose);
  await tester.pumpWidget(
    ProviderScope(
      overrides: [dioProvider.overrideWithValue(dio)],
      child: ScreenUtilInit(
        designSize: const Size(390, 844),
        enableScaleWH: () => false,
        enableScaleText: () => false,
        builder: (_, _) => MaterialApp.router(routerConfig: router),
      ),
    ),
  );
  await tester.pump();
  await tester.pump(const Duration(milliseconds: 100));
  await tester.pump();
}

void main() {
  if (!F.isInitialized) F.init(Flavor.dev);

  testWidgets('HTTP 404 shows the not-found state, not a network error', (
    tester,
  ) async {
    await _pumpDetail(
      tester,
      _StatusAdapter(404, {'code': 404, 'message': 'not found'}),
    );

    expect(find.text('找不到這則貼文'), findsOneWidget);
    expect(find.text('網路連線失敗，請稍後再試'), findsNothing);
    expect(find.text('重試'), findsNothing);
  });

  testWidgets('an envelope with code 404 shows the not-found state', (
    tester,
  ) async {
    await _pumpDetail(
      tester,
      _StatusAdapter(200, {'code': 404, 'message': 'not found'}),
    );

    expect(find.text('找不到這則貼文'), findsOneWidget);
    expect(find.text('重試'), findsNothing);
  });

  testWidgets(
    'a not-found post makes one attempt (detail and comments), no retry',
    (tester) async {
      final adapter = _StatusAdapter(404, {
        'code': 404,
        'message': 'not found',
      });
      await _pumpDetail(tester, adapter);
      // 一次嘗試 = 詳情與留言各一個請求。
      const oneAttempt = 2;
      expect(adapter.requests, oneAttempt);

      await tester.pump(const Duration(seconds: 10));
      expect(adapter.requests, oneAttempt);
      expect(find.text('找不到這則貼文'), findsOneWidget);

      await tester.pumpWidget(const SizedBox.shrink());
    },
  );

  testWidgets('a transient failure is retried automatically (control)', (
    tester,
  ) async {
    final adapter = _StatusAdapter(503, {'code': 503, 'message': 'down'});
    await _pumpDetail(tester, adapter);
    final first = adapter.requests;

    await tester.pump(const Duration(seconds: 10));
    expect(adapter.requests, greaterThan(first));

    await tester.pumpWidget(const SizedBox.shrink());
  });

  test('a server failure stays a network error, not a not-found', () async {
    final dio = Dio(BaseOptions(baseUrl: 'https://example.test'))
      ..httpClientAdapter = _StatusAdapter(503, {
        'code': 503,
        'message': 'unavailable',
      });
    addTearDown(dio.close);

    await expectLater(
      SocialRepository(dio, sellers: fakeSellerRepository()).getPostDetail(101),
      throwsA(isA<NetworkException>()),
    );
    await expectLater(
      SocialRepository(dio, sellers: fakeSellerRepository()).getComments(101),
      throwsA(isA<NetworkException>()),
    );
  });
}
