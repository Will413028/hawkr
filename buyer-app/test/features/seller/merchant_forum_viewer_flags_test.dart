import 'dart:async';

import 'package:flutter/material.dart';
import 'package:flutter_screenutil/flutter_screenutil.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:hawkr_api/transport.dart' show Code, ConnectException;
import 'package:hawkr_buyer/config/flavor.dart';
import 'package:hawkr_buyer/core/networking/connect_provider.dart';
import 'package:hawkr_buyer/core/session/session_model.dart';
import 'package:hawkr_buyer/core/session/session_provider.dart';
import 'package:hawkr_buyer/core/theme/app_theme.dart';
import 'package:go_router/go_router.dart';
import 'package:hawkr_buyer/features/seller/presentation/widgets/merchant_chat_tab.dart';
import 'package:hawkr_buyer/shared/icons/app_icons.dart';
import 'package:hawkr_buyer/shared/widgets/hk_frosted_icon_button.dart';
import 'package:hooks_riverpod/hooks_riverpod.dart';

import '../social/social_fakes.dart';

/// 討論區（`ListForumEntries` 等）的顯示只看伺服器給的 `viewer_can_*`，沒有任何比對 ID 的擁有權邏輯；
/// 討論區沒有按讚（舊 client 拿討論區 id 打貼文留言的讚端點，從未生效）。
FakeSocialPublic _forum({bool canComment = true}) => FakeSocialPublic(
  forumViewerCanComment: canComment,
  forumEntries: [
    fakeForumEntry('c-1', body: '可編輯與刪除的留言', canEdit: true, canDelete: true),
    fakeForumEntry('c-2', body: '只能回覆的留言', canReply: true),
    fakeForumEntry('c-3', body: '什麼都不能做的留言'),
  ],
);

class _TestSession extends Session {
  _TestSession(this.loggedIn);
  final bool loggedIn;
  @override
  Future<SessionModel?> build() async => loggedIn
      ? const SessionModel(userId: 'account-3001', nickname: 'Iris')
      : null;
}

late ProviderContainer _container;

Future<void> _pump(
  WidgetTester tester,
  FakeSocialPublic public, {
  FakeSocialApp? app,
  bool loggedIn = true,
  bool switchable = false,
}) async {
  final container = ProviderContainer(
    overrides: [
      socialPublicClientProvider.overrideWithValue(public.client()),
      socialAppClientProvider.overrideWithValue(
        (app ?? FakeSocialApp()).client(),
      ),
      sessionProvider.overrideWith(
        () => switchable ? SwitchableSession() : _TestSession(loggedIn),
      ),
    ],
  );
  addTearDown(container.dispose);
  _container = container;
  await container.read(sessionProvider.future);
  await tester.pumpWidget(
    UncontrolledProviderScope(
      container: container,
      child: ScreenUtilInit(
        designSize: const Size(390, 844),
        enableScaleWH: () => false,
        enableScaleText: () => false,
        builder: (_, _) => MaterialApp.router(
          theme: AppTheme.light,
          routerConfig: GoRouter(
            routes: [
              GoRoute(
                path: '/',
                builder: (_, _) => Scaffold(
                  body: Consumer(
                    builder: (context, ref, _) =>
                        MerchantChatTab(merchantId: '7', merchantName: '南崁王生技'),
                  ),
                ),
              ),
              GoRoute(
                path: '/login',
                builder: (_, _) => const Scaffold(body: Text('登入頁')),
              ),
            ],
          ),
        ),
      ),
    ),
  );
  await tester.pump();
  await tester.pump(const Duration(milliseconds: 100));
  await tester.pump();
}

void main() {
  if (!F.isInitialized) F.init(Flavor.dev);

  testWidgets('編輯與刪除選單只看 viewer_can_edit、viewer_can_delete', (tester) async {
    await _pump(tester, _forum());

    // 只有 c-1 顯示 ⋯ 選單。
    expect(find.byIcon(AppIcons.moreHoriz), findsOneWidget);
    await tester.tap(find.byTooltip('更多留言操作'));
    await tester.pumpAndSettle();
    expect(find.text('編輯留言'), findsOneWidget);
    expect(find.text('刪除留言'), findsOneWidget);
  });

  testWidgets('回覆入口只看 viewer_can_reply', (tester) async {
    await _pump(tester, _forum());

    expect(find.text('回覆'), findsOneWidget);
  });

  testWidgets('新增頂層留言的輸入欄只看 viewer_can_comment', (tester) async {
    await _pump(tester, _forum(canComment: false));
    expect(find.byKey(const Key('merchant-comment-input')), findsNothing);
  });

  testWidgets('viewer_can_comment 為 true 時有輸入欄', (tester) async {
    await _pump(tester, _forum());
    expect(find.byKey(const Key('merchant-comment-input')), findsOneWidget);
  });

  testWidgets('訪客：顯示「登入後留言」，點了經確認後導向登入', (tester) async {
    await _pump(tester, _forum(canComment: false), loggedIn: false);
    expect(find.byKey(const Key('merchant-comment-input')), findsNothing);
    expect(find.text('登入後留言'), findsOneWidget);
    await tester.tap(find.text('登入後留言'));
    await tester.pumpAndSettle();
    await tester.tap(find.text('前往登入'));
    await tester.pumpAndSettle();
    expect(find.text('登入頁'), findsOneWidget);
  });

  testWidgets('已登入但 viewer_can_comment 為 false（被封鎖）：沒有輸入欄也沒有登入提示', (
    tester,
  ) async {
    await _pump(tester, _forum(canComment: false));
    expect(find.byKey(const Key('merchant-comment-input')), findsNothing);
    expect(find.text('登入後留言'), findsNothing);
  });

  testWidgets('討論區沒有按讚的控制項', (tester) async {
    await _pump(tester, _forum());

    expect(find.byIcon(AppIcons.favorite), findsNothing);
    expect(find.byIcon(AppIcons.favoriteBorder), findsNothing);
    expect(find.textContaining('按讚'), findsNothing);
    expect(find.byTooltip('按讚'), findsNothing);
  });

  testWidgets('回覆送出 CreateForumEntry（parent_id 為被回覆的項目）', (tester) async {
    final app = FakeSocialApp();
    await _pump(tester, _forum(), app: app);

    await tester.tap(find.text('回覆'));
    await tester.pump();
    await tester.enterText(
      find.byKey(const Key('merchant-comment-input')),
      '這是給另一人的回覆',
    );
    await tester.pump();
    await tester.tap(find.byKey(const Key('merchant-comment-submit')));
    await tester.pumpAndSettle();

    final request = app.forumCreated.single;
    expect(request.storeId, '7');
    expect(request.parentId, 'c-2');
    expect(request.replyToId, 'c-2');
    expect(request.body, '這是給另一人的回覆');
    expect(request.requestId, isNotEmpty);
  });

  testWidgets('回覆某則回覆：parent_id 是所屬的頂層項目，reply_to_id 是被回覆的那一則', (tester) async {
    final app = FakeSocialApp();
    final public = FakeSocialPublic(
      forumEntries: [fakeForumEntry('top', canReply: true, replyCount: 1)],
      forumReplies: {
        'top': [fakeForumEntry('r-1', canReply: true)..parentId = 'top'],
      },
    );
    await _pump(tester, public, app: app);

    await tester.tap(find.text('回覆').last);
    await tester.pump();
    await tester.enterText(
      find.byKey(const Key('merchant-comment-input')),
      '回給回覆',
    );
    await tester.pump();
    await tester.tap(find.byKey(const Key('merchant-comment-submit')));
    await tester.pumpAndSettle();

    final request = app.forumCreated.single;
    expect(request.parentId, 'top');
    expect(request.replyToId, 'r-1');
  });

  testWidgets('以店家身分留言與回覆：輸入框提示帶店名（C2、C3）', (tester) async {
    final public = FakeSocialPublic(
      forumViewerActor: storeActor('好店'),
      forumEntries: [fakeForumEntry('c-2', canReply: true)],
    );
    await _pump(tester, public);

    String? hint() => tester
        .widget<TextField>(find.byKey(const Key('merchant-comment-input')))
        .decoration
        ?.hintText;
    expect(hint(), '以「好店」身分留言…');
    await tester.tap(find.text('回覆'));
    await tester.pump();
    expect(hint(), '以「好店」身分回覆…');
  });

  testWidgets('打開後登入：重讀討論區，登入提示換成輸入欄', (tester) async {
    final public = FakeSocialPublic(
      forumViewerCanComment: false,
      forumEntries: [fakeForumEntry('c-3')],
    );
    await _pump(tester, public, switchable: true);
    expect(find.text('登入後留言'), findsOneWidget);
    expect(find.byKey(const Key('merchant-comment-input')), findsNothing);
    final before = public.listForumCalls;

    public.forumViewerCanComment = true;
    (_container.read(sessionProvider.notifier) as SwitchableSession).signIn();
    await tester.pumpAndSettle();

    expect(public.listForumCalls, greaterThan(before));
    expect(find.text('登入後留言'), findsNothing);
    expect(find.byKey(const Key('merchant-comment-input')), findsOneWidget);
  });

  testWidgets('編輯送出 UpdateForumEntry（帶 etag 與 body 的 update_mask）', (
    tester,
  ) async {
    final app = FakeSocialApp();
    await _pump(tester, _forum(), app: app);

    await tester.tap(find.byTooltip('更多留言操作'));
    await tester.pumpAndSettle();
    await tester.tap(find.text('編輯留言'));
    await tester.pumpAndSettle();
    await tester.enterText(
      find.byKey(const Key('merchant-comment-input')),
      '改過的內容',
    );
    await tester.pump();
    await tester.tap(find.byKey(const Key('merchant-comment-submit')));
    await tester.pumpAndSettle();

    final request = app.forumUpdated.single;
    expect(request.entry.id, 'c-1');
    expect(request.entry.etag, 'etag-c-1');
    expect(request.entry.body, '改過的內容');
    expect(request.updateMask.paths, ['body']);
    await tester.pump(const Duration(seconds: 3));
    await tester.pumpAndSettle();
  });

  testWidgets('刪除必須經確認，送出 DeleteForumEntry（帶 etag）', (tester) async {
    final app = FakeSocialApp();
    await _pump(tester, _forum(), app: app);

    await tester.tap(find.byTooltip('更多留言操作'));
    await tester.pumpAndSettle();
    await tester.tap(find.text('刪除留言'));
    await tester.pumpAndSettle();
    await tester.tap(find.text('取消'));
    await tester.pumpAndSettle();
    expect(app.forumDeleted, isEmpty);

    await tester.tap(find.byTooltip('更多留言操作'));
    await tester.pumpAndSettle();
    await tester.tap(find.text('刪除留言'));
    await tester.pumpAndSettle();
    await tester.tap(find.text('刪除'));
    await tester.pumpAndSettle();

    expect(app.forumDeleted.single.entryId, 'c-1');
    expect(app.forumDeleted.single.etag, 'etag-c-1');
    await tester.pump(const Duration(seconds: 3));
    await tester.pumpAndSettle();
  });

  testWidgets('送出失敗：保留草稿與回覆模式，且只送一次', (tester) async {
    final app = FakeSocialApp()..pending = Completer<void>();
    await _pump(tester, _forum(), app: app);

    await tester.tap(find.text('回覆'));
    await tester.pump();
    final input = find.byKey(const Key('merchant-comment-input'));
    final submit = find.byKey(const Key('merchant-comment-submit'));
    await tester.enterText(input, '保留這份草稿');
    await tester.pump();
    await tester.tap(submit);
    await tester.pump();
    await tester.tap(submit);
    await tester.pump();
    expect(app.forumCreated, hasLength(1));
    expect(tester.widget<HkFrostedIconButton>(submit).loading, isTrue);

    app.pending!.completeError(ConnectException(Code.unavailable, 'down'));
    await tester.pumpAndSettle();
    expect(tester.widget<TextField>(input).controller!.text, '保留這份草稿');
    expect(
      find.byKey(const Key('merchant-comment-mode-banner')),
      findsOneWidget,
    );
    expect(tester.widget<HkFrostedIconButton>(submit).loading, isFalse);
    await tester.pump(const Duration(seconds: 3));
    await tester.pumpAndSettle();
    expect(tester.takeException(), isNull);
  });
}
