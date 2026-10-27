import 'dart:async';
import 'dart:convert';
import 'dart:io';
import 'dart:typed_data';

import 'package:hawkr_api/gen/hawkr/buyer/v1alpha1/buyer.connect.client.dart';
import 'package:hawkr_api/gen/hawkr/buyer/v1alpha1/buyer.connect.spec.dart';
import 'package:hawkr_api/gen/hawkr/buyer/v1alpha1/buyer.pb.dart' as buyer;
import 'package:hawkr_api/gen/hawkr/identity/v1alpha1/identity.connect.client.dart';
import 'package:hawkr_api/testing.dart';
import 'package:hawkr_buyer/config/flavor.dart';
import 'package:hawkr_buyer/features/auth/data/auth_repository.dart';
import 'package:hawkr_buyer/core/networking/dio_provider.dart';
import 'package:hawkr_buyer/core/session/session_model.dart';
import 'package:hawkr_buyer/core/session/session_provider.dart';
import 'package:hawkr_buyer/features/seller/presentation/widgets/merchant_chat_tab.dart';
import 'package:hawkr_buyer/shared/icons/app_icons.dart';
import 'package:hawkr_buyer/core/theme/app_theme.dart';
import 'package:hawkr_buyer/shared/widgets/hk_frosted_icon_button.dart';
import 'package:flutter/gestures.dart';
import 'package:dio/dio.dart';
import 'package:flutter/material.dart';
import 'package:flutter_screenutil/flutter_screenutil.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:hooks_riverpod/hooks_riverpod.dart';
import '../../support/default_error_text.dart';

/// 2026-08-01 身份稽核回歸測試。
///
/// session 存的是 raw accountId（會員登入 ID），跟討論區留言 API 回傳、已解析的
/// buyer_id 是不同 ID 空間（2026-07-18 起新帳號兩者刻意不同，作為偵測「用錯 ID
/// 空間」bug 的 canary）。fixture 的 accountId／buyerId 刻意用不同數字，避免測
/// 不出真正的比對邏輯（過去大量測試因兩者用同一個值而測不出來）。
class _ForumOwnershipAdapter implements HttpClientAdapter {
  Completer<ResponseBody>? pendingMutation;
  final List<RequestOptions> mutations = [];
  @override
  Future<ResponseBody> fetch(
    RequestOptions options,
    Stream<Uint8List>? requestStream,
    Future<void>? cancelFuture,
  ) async {
    if (options.method == 'POST' ||
        options.method == 'PUT' ||
        options.method == 'DELETE') {
      mutations.add(options);
      if (pendingMutation != null) return pendingMutation!.future;
    }
    final body = switch (options.path) {
      '/public/sellers/7/forum' => {
        'code': 0,
        'data': {
          'records': [
            {
              'id': 'c-1',
              'isMine': true, // 伺服器算的「我的留言」
              'userName': 'Will',
              'content': '我自己的留言',
              'likeCount': 0,
              'replyCount': 0,
            },
            {
              'id': 'c-2',
              'isMine': false, // 別人的留言
              'userName': '路人',
              'content': '別人的留言',
              'likeCount': 0,
              'replyCount': 0,
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
    userId: 'account-1001', // raw accountId，刻意跟 buyer_id 不同
    nickname: 'Will',
  );
}

Future<void> _pumpForum(
  WidgetTester tester,
  _ForumOwnershipAdapter adapter,
) async {
  final dio = Dio(BaseOptions(baseUrl: 'https://example.test'))
    ..httpClientAdapter = adapter;
  addTearDown(dio.close);

  final container = ProviderContainer(
    overrides: [
      dioProvider.overrideWithValue(dio),
      // buyer_id 由 BuyerAppService.GetProfile 的 profile.id 提供（已解析的
      // buyer_id，跟 session 的 raw accountId 刻意不同）。
      authRepositoryProvider.overrideWith((_) => _profileRepository()),
      sessionProvider.overrideWith(_ActiveSession.new),
    ],
  );
  addTearDown(container.dispose);
  // sessionProvider 是 async provider；widget initState 內是同步讀
  // ref.read(sessionProvider).value，若 session 這時還在 AsyncLoading
  // 會直接讀到 null（產品環境下 session 早在 app 啟動時就 resolve 過，
  // 不會遇到；獨立 widget test 裡則要先手動等它 resolve 一次，避免測試
  // 本身的 race 蓋掉真正要測的 buyer_id 比對邏輯）。
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
            home: Scaffold(
              body: Consumer(
                builder: (context, ref, _) => MerchantChatTab(
                  ref: ref,
                  merchantId: '7',
                  merchantName: '南崁王生技',
                ),
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
}

void main() {
  if (!F.isInitialized) F.init(Flavor.dev);

  testWidgets('留言擁有權只看伺服器的 isMine：只有自己的留言顯示編輯/刪除選單', (tester) async {
    await _pumpForum(tester, _ForumOwnershipAdapter());

    expect(find.text('我自己的留言'), findsOneWidget);
    expect(find.text('別人的留言'), findsOneWidget);

    // 只有「我自己的留言」該顯示 ⋯ 選單（編輯/刪除入口）。
    expect(find.byIcon(AppIcons.moreHoriz), findsOneWidget);
  });

  testWidgets('從編輯切換回覆會真的回覆選定留言，不會誤改原留言', (tester) async {
    final adapter = _ForumOwnershipAdapter();
    await _pumpForum(tester, adapter);
    await tester.tap(find.byTooltip('更多留言操作'));
    await tester.pumpAndSettle();
    await tester.tap(find.text('編輯留言'));
    await tester.pumpAndSettle();
    expect(find.byTooltip('儲存留言'), findsOneWidget);
    await tester.tap(find.text('回覆').last);
    await tester.pumpAndSettle();
    expect(find.byTooltip('送出留言'), findsOneWidget);
    final input = find.byKey(const Key('merchant-comment-input'));
    expect(tester.widget<TextField>(input).controller!.text, isEmpty);
    await tester.enterText(input, '這是給另一人的回覆');
    await tester.pump();
    await tester.tap(find.byKey(const Key('merchant-comment-submit')));
    await tester.pumpAndSettle();
    expect(adapter.mutations, hasLength(1));
    expect(adapter.mutations.single.method, 'POST');
    expect(adapter.mutations.single.path, '/app/sellers/forum/c-2/reply');
    expect(tester.takeException(), isNull);
  });

  testWidgets('更多與長按選單可取消，刪除必須經確認且使用正確留言ID', (tester) async {
    final adapter = _ForumOwnershipAdapter();
    await _pumpForum(tester, adapter);
    await tester.tap(find.byTooltip('更多留言操作'));
    await tester.pumpAndSettle();
    expect(find.text('編輯留言'), findsOneWidget);
    expect(find.text('刪除留言'), findsOneWidget);
    await tester.tapAt(const Offset(10, 10));
    await tester.pumpAndSettle();
    expect(find.text('編輯留言'), findsNothing);
    await tester.longPress(find.text('別人的留言'));
    await tester.pumpAndSettle();
    expect(find.text('刪除留言'), findsNothing);
    await tester.longPress(find.text('我自己的留言'));
    await tester.pumpAndSettle();
    await tester.tap(find.text('刪除留言'));
    await tester.pumpAndSettle();
    await tester.tap(find.text('取消'));
    await tester.pumpAndSettle();
    expect(adapter.mutations, isEmpty);
    await tester.tap(find.byTooltip('更多留言操作'));
    await tester.pumpAndSettle();
    await tester.tap(find.text('刪除留言'));
    await tester.pumpAndSettle();
    await tester.tap(find.text('刪除'));
    await tester.pumpAndSettle();
    expect(adapter.mutations, hasLength(1));
    expect(adapter.mutations.single.method, 'DELETE');
    expect(adapter.mutations.single.path, '/app/sellers/forum/c-1');
    await tester.pump(const Duration(seconds: 3));
    await tester.pumpAndSettle();
    expect(tester.takeException(), isNull);
  });
  for (final mode in ['留言', '回覆', '編輯']) {
    testWidgets('$mode 送出只處理一次且失敗保留草稿與模式', (tester) async {
      final adapter = _ForumOwnershipAdapter();
      await _pumpForum(tester, adapter);
      if (mode == '回覆') {
        await tester.tap(find.text('回覆').first);
        await tester.pump();
      } else if (mode == '編輯') {
        final more = find.byTooltip('更多留言操作');
        expect(tester.getSize(more), const Size.square(48));
        await tester.tap(more);
        await tester.pumpAndSettle();
        await tester.tap(find.text('編輯留言'));
        await tester.pumpAndSettle();
      }
      final input = find.byKey(const Key('merchant-comment-input'));
      final submit = find.byKey(const Key('merchant-comment-submit'));
      await tester.enterText(input, '保留這份草稿');
      await tester.pump();
      final focus = tester.widget<TextField>(input).focusNode!;
      final action = tester.widget<HkFrostedIconButton>(submit).onPressed!;
      adapter.pendingMutation = Completer<ResponseBody>();
      addTearDown(() {
        if (!adapter.pendingMutation!.isCompleted) {
          adapter.pendingMutation!.complete(ResponseBody.fromString('{}', 500));
        }
      });
      // Desktop pointer-down must not blur and relocate the composer before up.
      final gesture = await tester.startGesture(
        tester.getCenter(submit),
        kind: PointerDeviceKind.mouse,
      );
      await tester.pump();
      expect(focus.hasFocus, isTrue);
      await gesture.up();
      await tester.pump();
      action(); // A queued duplicate callback must also respect the busy guard.
      await tester.pump();
      for (var i = 0; i < 10 && adapter.mutations.isEmpty; i++) {
        await tester.pump(const Duration(milliseconds: 20));
      }
      expect(adapter.mutations, hasLength(1));
      expect(adapter.mutations.single.path, switch (mode) {
        '編輯' => '/app/sellers/forum/c-1',
        '回覆' => '/app/sellers/forum/c-1/reply',
        _ => '/app/sellers/7/forum',
      });
      expect(tester.widget<TextField>(input).readOnly, isTrue);
      expect(tester.widget<HkFrostedIconButton>(submit).loading, isTrue);
      final moreAction = find.byWidgetPredicate(
        (widget) => widget is HkFrostedIconButton && widget.label == '更多留言操作',
      );
      expect(tester.widget<HkFrostedIconButton>(moreAction).onPressed, isNull);
      await tester.tap(moreAction);
      await tester.pump();
      expect(find.text('刪除留言'), findsNothing);
      adapter.pendingMutation!.complete(ResponseBody.fromString('{}', 500));
      for (
        var i = 0;
        i < 10 && tester.widget<HkFrostedIconButton>(submit).loading;
        i++
      ) {
        await tester.pump(const Duration(milliseconds: 20));
      }
      expect(tester.widget<TextField>(input).controller!.text, '保留這份草稿');
      expect(tester.widget<TextField>(input).readOnly, isFalse);
      expect(tester.widget<HkFrostedIconButton>(submit).loading, isFalse);
      expect(tester.widget<HkFrostedIconButton>(submit).onPressed, isNotNull);
      if (mode != '留言') {
        expect(
          find.byKey(const Key('merchant-comment-mode-banner')),
          findsOneWidget,
        );
      }
      await tester.pump(const Duration(seconds: 3));
      await tester.pumpAndSettle();
      expect(tester.takeException(), isNull);
    });
  }
}

AuthRepository _profileRepository() => AuthRepository(
  IdentityAppServiceClient(FakeTransportBuilder().build()),
  BuyerAppServiceClient(
    FakeTransportBuilder()
        .unary(
          BuyerAppService.getProfile,
          (_, _) => buyer.GetProfileResponse(
            profile: buyer.Profile(id: 'buyer-9001', displayName: 'Will'),
          ),
        )
        .build(),
  ),
  defaults: zhDefaultErrorText,
);
