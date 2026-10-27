import 'dart:async';
import 'dart:convert';
import 'dart:io';
import 'dart:typed_data';

import 'package:hawkr_buyer/config/flavor.dart';
import 'package:hawkr_api/gen/hawkr/buyer/v1alpha1/buyer.connect.client.dart';
import 'package:hawkr_api/gen/hawkr/buyer/v1alpha1/buyer.connect.spec.dart';
import 'package:hawkr_api/gen/hawkr/buyer/v1alpha1/buyer.pb.dart' as buyer;
import 'package:hawkr_api/gen/hawkr/identity/v1alpha1/identity.connect.client.dart';
import 'package:hawkr_api/testing.dart';
import 'package:hawkr_buyer/core/networking/dio_provider.dart';
import 'package:hawkr_buyer/features/auth/data/auth_repository.dart';
import 'package:hawkr_buyer/core/session/session_model.dart';
import 'package:hawkr_buyer/core/session/session_provider.dart';
import 'package:hawkr_buyer/core/theme/app_theme.dart';
import 'package:hawkr_buyer/shared/widgets/comments_bottom_sheet.dart';
import 'package:hawkr_buyer/shared/widgets/hk_frosted_icon_button.dart';
import 'package:dio/dio.dart';
import 'package:flutter/material.dart';
import 'package:flutter_screenutil/flutter_screenutil.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:hooks_riverpod/hooks_riverpod.dart';
import '../../support/default_error_text.dart';

/// 留言是不是「我的」只看伺服器算的 `isMine`，不比對任何 ID。
///
/// postId 刻意用非數字字串（'post-abc'），讓 _startStream 的
/// int.tryParse(postId) 失敗、提早 return，測試不會啟動真的 SSE 連線
/// （CommentStreamService 是純 template，不吃這個 fixture 也無妨測到的目標
/// bug：留言列表 GET 回應的暱稱 fallback 比對邏輯）。
class _CommentsAdapter implements HttpClientAdapter {
  Completer<ResponseBody>? pendingMutation;
  final List<RequestOptions> mutations = [];

  @override
  Future<ResponseBody> fetch(
    RequestOptions options,
    Stream<Uint8List>? requestStream,
    Future<void>? cancelFuture,
  ) async {
    if (options.method == 'POST') {
      mutations.add(options);
      if (pendingMutation != null) return pendingMutation!.future;
    }
    final body = switch (options.path) {
      '/public/posts/post-abc/comments' => {
        'code': 0,
        'data': {
          'records': [
            {
              // 刻意不帶 userName/username/author/name 等欄位，逼近
              // _userNameOf 走到「這是我的留言，用我的暱稱顯示」的 fallback
              // 分支（原本的 bug 2 現場）。
              'id': 'cm-1',
              'isMine': true, // 伺服器算的「我的留言」
              'content': '這是我自己的留言',
              'likeCount': 0,
            },
          ],
        },
      },
      _ => {'code': 0, 'data': <Object>[]},
    };
    return ResponseBody.fromString(
      jsonEncode(body),
      200,
      headers: {
        Headers.contentTypeHeader: [Headers.jsonContentType],
      },
    );
  }

  @override
  void close({bool force = false}) {}
}

class _ActiveSession extends Session {
  @override
  Future<SessionModel?> build() async => const SessionModel(
    userId: 'account-3001', // raw accountId，刻意跟 buyer_id 不同
    nickname: 'Iris',
  );
}

void main() {
  if (!F.isInitialized) F.init(Flavor.dev);

  testWidgets('自己留言的暱稱 fallback 只看 isMine', (tester) async {
    final dio = Dio(BaseOptions(baseUrl: 'https://example.test'))
      ..httpClientAdapter = _CommentsAdapter();
    addTearDown(dio.close);

    final container = ProviderContainer(
      overrides: [
        dioProvider.overrideWithValue(dio),
        authRepositoryProvider.overrideWith((_) => _profileRepository()),
        sessionProvider.overrideWith(_ActiveSession.new),
      ],
    );
    addTearDown(container.dispose);
    // sessionProvider 是 async provider；widget initState 內是同步讀
    // ref.read(sessionProvider).value，若這時還在 AsyncLoading 會直接讀到
    // null（產品環境下 session 早在 app 啟動時就 resolve 過，不會遇到；獨立
    // widget test 裡要先手動等它 resolve 一次，避免測試本身的 race 蓋掉真正
    // 要測的 isMine 判斷）。
    await container.read(sessionProvider.future);

    await HttpOverrides.runZoned(
      () => tester.pumpWidget(
        UncontrolledProviderScope(
          container: container,
          child: ScreenUtilInit(
            designSize: const Size(390, 844),
            enableScaleWH: () => false,
            enableScaleText: () => false,
            builder: (_, __) => const MaterialApp(
              home: Scaffold(
                // postOwnerName 刻意給一個跟 session 暱稱不同的值：
                // _userNameOf 最後一層 fallback 是
                // `postOwnerName ?? _myDisplayName ?? 'User'`，若不給
                // postOwnerName，即使 isMine 判斷是錯的，也會
                // 因為 _myDisplayName 直接落到這層 fallback 而巧合顯示對的
                // 暱稱，蓋掉真正要測的比對邏輯。
                body: CommentsBottomSheet(
                  postId: 'post-abc',
                  postOwnerName: '商家',
                ),
              ),
            ),
          ),
        ),
      ),
      createHttpClient: (_) {
        throw StateError('External network is disabled in widget tests');
      },
    );

    await tester.pump();
    await tester.pump(const Duration(milliseconds: 100));
    await tester.pump();

    expect(find.text('Iris'), findsOneWidget);
    expect(find.text('商家'), findsNothing);
    expect(find.text('這是我自己的留言'), findsOneWidget);
  });

  testWidgets('傳送回覆期間不能取消或切換對象，重複點擊只新增一筆', (tester) async {
    final adapter = _CommentsAdapter();
    await _pumpComments(tester, adapter);
    await tester.tap(find.text('回覆').first);
    await tester.pump();
    final input = find.byKey(const ValueKey('comments-panel-input'));
    final send = find.byKey(const ValueKey('send-comment-action'));
    await tester.enterText(input, '完成後留在原討論串');
    await tester.pump();
    adapter.pendingMutation = Completer<ResponseBody>();
    await tester.tap(send);
    await _waitForMutation(tester, adapter, 1);
    await tester.tap(send);
    await tester.tap(find.byKey(const ValueKey('cancel-comment-reply')));
    await tester.pump();
    expect(find.text('回覆 @Iris'), findsOneWidget);
    expect(adapter.mutations, hasLength(1));
    expect(adapter.mutations.single.path, '/app/posts/comments/cm-1/replies');
    adapter.pendingMutation!.complete(
      _response({'id': 'cm-new', 'content': '完成後留在原討論串'}),
    );
    await tester.pumpAndSettle();
    expect(tester.takeException(), isNull);
    expect(tester.widget<TextField>(input).controller!.text, isEmpty);
    expect(find.byKey(const ValueKey('comment-reply-context')), findsNothing);
    expect(find.text('完成後留在原討論串'), findsOneWidget);
  });

  testWidgets('留言IME送出防重複、失敗保留草稿並能重新送出', (tester) async {
    final adapter = _CommentsAdapter();
    await _pumpComments(tester, adapter);
    final input = find.byKey(const ValueKey('comments-panel-input'));
    final send = find.byKey(const ValueKey('send-comment-action'));
    await tester.enterText(input, '網路恢復後重試');
    await tester.pump();
    adapter.pendingMutation = Completer<ResponseBody>();
    await tester.testTextInput.receiveAction(TextInputAction.send);
    await _waitForMutation(tester, adapter, 1);
    await tester.tap(send);
    expect(adapter.mutations, hasLength(1));
    adapter.pendingMutation!.complete(ResponseBody.fromString('{}', 500));
    await tester.pumpAndSettle();
    expect(tester.widget<TextField>(input).controller!.text, '網路恢復後重試');
    expect(find.text('傳送失敗，請稍後再試'), findsOneWidget);
    expect(tester.widget<HkFrostedIconButton>(send).loading, isFalse);
    adapter.pendingMutation = Completer<ResponseBody>();
    await tester.tap(send);
    await _waitForMutation(tester, adapter, 2);
    adapter.pendingMutation!.complete(
      _response({'id': 'cm-new', 'content': '網路恢復後重試'}),
    );
    await tester.pumpAndSettle();
    expect(adapter.mutations, hasLength(2));
    expect(tester.widget<TextField>(input).controller!.text, isEmpty);
    expect(find.text('網路恢復後重試'), findsOneWidget);
    await tester.pump(const Duration(seconds: 3));
    await tester.pumpAndSettle();
    expect(tester.takeException(), isNull);
  });

  testWidgets('留言送出後關閉頁面，延遲回應不再操作已卸載元件', (tester) async {
    final adapter = _CommentsAdapter();
    await _pumpComments(tester, adapter);
    await tester.enterText(find.byType(TextField), '離頁中的請求');
    await tester.pump();
    adapter.pendingMutation = Completer<ResponseBody>();
    await tester.tap(find.byKey(const ValueKey('send-comment-action')));
    await _waitForMutation(tester, adapter, 1);
    expect(adapter.mutations, hasLength(1));
    await tester.pumpWidget(const SizedBox.shrink());
    adapter.pendingMutation!.complete(_response({'id': 'cm-new'}));
    await tester.pumpAndSettle();
    expect(tester.takeException(), isNull);
  });

  testWidgets('回覆失敗只送原本的一筆請求，不會改新增頂層留言', (tester) async {
    final adapter = _CommentsAdapter();
    await _pumpComments(tester, adapter);
    await tester.tap(find.text('回覆').first);
    await tester.enterText(find.byType(TextField), '保留回覆內容');
    await tester.pump();
    adapter.pendingMutation = Completer<ResponseBody>();
    await tester.tap(find.byKey(const ValueKey('send-comment-action')));
    await _waitForMutation(tester, adapter, 1);
    adapter.pendingMutation!.complete(ResponseBody.fromString('{}', 500));
    await tester.pumpAndSettle();
    expect(adapter.mutations, hasLength(1));
    expect(adapter.mutations.single.path, '/app/posts/comments/cm-1/replies');
    expect(find.text('回覆 @Iris'), findsOneWidget);
    expect(
      tester.widget<TextField>(find.byType(TextField)).controller!.text,
      '保留回覆內容',
    );
    expect(find.text('傳送失敗，請稍後再試'), findsOneWidget);
    await tester.pump(const Duration(seconds: 3));
    await tester.pumpAndSettle();
    expect(tester.takeException(), isNull);
  });
}

ResponseBody _response(Map<String, dynamic> data) => ResponseBody.fromString(
  jsonEncode({'code': 0, 'data': data}),
  200,
  headers: {
    Headers.contentTypeHeader: [Headers.jsonContentType],
  },
);

Future<void> _waitForMutation(
  WidgetTester tester,
  _CommentsAdapter adapter,
  int count,
) async {
  for (var i = 0; i < 20 && adapter.mutations.length < count; i++) {
    await tester.pump(const Duration(milliseconds: 20));
  }
  expect(adapter.mutations, hasLength(count));
}

Future<void> _pumpComments(
  WidgetTester tester,
  _CommentsAdapter adapter,
) async {
  final dio = Dio(BaseOptions(baseUrl: 'https://example.test'))
    ..httpClientAdapter = adapter;
  addTearDown(dio.close);
  final container = ProviderContainer(
    overrides: [
      dioProvider.overrideWithValue(dio),
      authRepositoryProvider.overrideWith((_) => _profileRepository()),
      sessionProvider.overrideWith(_ActiveSession.new),
    ],
  );
  addTearDown(container.dispose);
  await container.read(sessionProvider.future);
  await HttpOverrides.runZoned(
    () => tester.pumpWidget(
      UncontrolledProviderScope(
        container: container,
        child: ScreenUtilInit(
          designSize: const Size(390, 844),
          enableScaleWH: () => false,
          enableScaleText: () => false,
          builder: (_, __) => MaterialApp(
            theme: AppTheme.light,
            home: const Scaffold(body: CommentsBottomSheet(postId: 'post-abc')),
          ),
        ),
      ),
    ),
    createHttpClient: (_) => throw StateError('External network disabled'),
  );
  await tester.pumpAndSettle();
}

AuthRepository _profileRepository() => AuthRepository(
  IdentityAppServiceClient(FakeTransportBuilder().build()),
  BuyerAppServiceClient(
    FakeTransportBuilder()
        .unary(
          BuyerAppService.getProfile,
          (_, _) => buyer.GetProfileResponse(
            profile: buyer.Profile(id: 'buyer-7002', displayName: 'Iris'),
          ),
        )
        .build(),
  ),
  defaults: zhDefaultErrorText,
);
