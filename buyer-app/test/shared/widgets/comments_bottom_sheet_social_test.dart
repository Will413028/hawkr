import 'package:flutter/material.dart';
import 'package:flutter_screenutil/flutter_screenutil.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:go_router/go_router.dart';
import 'package:hawkr_api/gen/hawkr/social/v1alpha1/social.pb.dart' as pb;
import 'package:hawkr_api/transport.dart' show Code, ConnectException;
import 'package:hawkr_buyer/config/flavor.dart';
import 'package:hawkr_buyer/core/networking/connect_provider.dart';
import 'package:hawkr_buyer/core/session/session_model.dart';
import 'package:hawkr_buyer/core/session/session_provider.dart';
import 'package:hawkr_buyer/core/theme/app_theme.dart';
import 'package:hawkr_buyer/shared/widgets/comments_bottom_sheet.dart';
import 'package:hooks_riverpod/hooks_riverpod.dart';

import '../../features/social/social_fakes.dart';

class _GuestSession extends Session {
  @override
  Future<SessionModel?> build() async => null;
}

class _ActiveSession extends Session {
  @override
  Future<SessionModel?> build() async =>
      const SessionModel(userId: 'account-3001', nickname: 'Iris');
}

late ProviderContainer _container;

Future<void> _pump(
  WidgetTester tester, {
  required FakeSocialPublic public,
  FakeSocialApp? app,
  bool guest = false,
  bool viewerCanComment = true,
  pb.Actor? viewerActor,
  bool switchable = false,
}) async {
  final container = ProviderContainer(
    overrides: [
      socialPublicClientProvider.overrideWithValue(public.client()),
      socialAppClientProvider.overrideWithValue(
        (app ?? FakeSocialApp()).client(),
      ),
      sessionProvider.overrideWith(
        switchable
            ? SwitchableSession.new
            : guest
            ? _GuestSession.new
            : _ActiveSession.new,
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
                  body: CommentsBottomSheet(
                    postId: '101',
                    viewerCanComment: viewerCanComment,
                    viewerActor: viewerActor,
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
  await tester.pumpAndSettle();
}

String? _hint(WidgetTester tester) => tester
    .widget<TextField>(find.byKey(const ValueKey('comments-panel-input')))
    .decoration
    ?.hintText;

void main() {
  if (!F.isInitialized) F.init(Flavor.dev);

  testWidgets('刪除入口只看 viewer_can_delete', (tester) async {
    final public = FakeSocialPublic(
      comments: [
        fakeComment('c1', canDelete: true),
        fakeComment('c2', canDelete: false),
      ],
    );
    await _pump(tester, public: public);

    expect(find.byKey(const ValueKey('comment-delete-c1')), findsOneWidget);
    expect(find.byKey(const ValueKey('comment-delete-c2')), findsNothing);
  });

  testWidgets('刪除：確認後送出 DeleteComment（帶 etag），只移除那一則，不重讀', (tester) async {
    final public = FakeSocialPublic(
      comments: [
        fakeComment('c1', body: '要刪的', canDelete: true),
        fakeComment('c2', body: '留下的'),
      ],
    );
    final app = FakeSocialApp();
    await _pump(tester, public: public, app: app);

    await tester.tap(find.byKey(const ValueKey('comment-delete-c1')));
    await tester.pumpAndSettle();
    expect(find.text('確定要刪除這則留言嗎？'), findsOneWidget);
    await tester.tap(find.text('刪除').last);
    await tester.pumpAndSettle();

    expect(app.deleted.single.commentId, 'c1');
    expect(app.deleted.single.etag, 'etag-c1');
    expect(public.listCommentsCalls, 1);
    expect(find.text('要刪的'), findsNothing);
    expect(find.text('留下的'), findsOneWidget);
  });

  testWidgets('刪除有回覆的頂層留言：變成已刪除佔位，已展開的回覆與其他留言不動', (tester) async {
    final public = FakeSocialPublic(
      comments: [
        fakeComment('c1', canDelete: true, replyCount: 1),
        fakeComment('c2', body: '另一則', replyCount: 1),
      ],
      replies: {
        'c1': [fakeComment('r1', body: 'c1 的回覆')],
        'c2': [fakeComment('r2', body: 'c2 的回覆')],
      },
    );
    await _pump(tester, public: public, app: FakeSocialApp());
    await tester.ensureVisible(
      find.byKey(const ValueKey('comment-more-replies-c2')),
    );
    await tester.tap(find.byKey(const ValueKey('comment-more-replies-c2')));
    await tester.pumpAndSettle();
    expect(find.text('c2 的回覆'), findsOneWidget);

    await tester.tap(find.byKey(const ValueKey('comment-delete-c1')));
    await tester.pumpAndSettle();
    await tester.tap(find.text('刪除').last);
    await tester.pumpAndSettle();

    expect(
      find.byKey(const ValueKey('comment-placeholder-c1')),
      findsOneWidget,
    );
    expect(find.text('c2 的回覆'), findsOneWidget);
    expect(public.listCommentsCalls, 2);
  });

  testWidgets('回覆入口只看 viewer_can_reply', (tester) async {
    final public = FakeSocialPublic(
      comments: [
        fakeComment('c1', canReply: true),
        fakeComment('c2', canReply: false),
      ],
    );
    await _pump(tester, public: public);

    expect(find.byKey(const ValueKey('comment-reply-c1')), findsOneWidget);
    expect(find.byKey(const ValueKey('comment-reply-c2')), findsNothing);
  });

  testWidgets('viewer_can_comment 是 false：沒有留言框', (tester) async {
    await _pump(tester, public: FakeSocialPublic(), viewerCanComment: false);
    expect(find.byKey(const ValueKey('comments-panel-input')), findsNothing);
  });

  testWidgets('訪客：顯示「登入後留言」，點了經確認後導向登入', (tester) async {
    await _pump(
      tester,
      guest: true,
      public: FakeSocialPublic(),
      viewerCanComment: false,
    );
    expect(find.byKey(const ValueKey('comments-panel-input')), findsNothing);
    expect(find.text('登入後留言'), findsOneWidget);
    await tester.tap(find.text('登入後留言'));
    await tester.pumpAndSettle();
    await tester.tap(find.text('前往登入'));
    await tester.pumpAndSettle();
    expect(find.text('登入頁'), findsOneWidget);
  });

  testWidgets('已登入但 viewer_can_comment 是 false（被封鎖）：沒有留言框也沒有登入提示', (
    tester,
  ) async {
    await _pump(tester, public: FakeSocialPublic(), viewerCanComment: false);
    expect(find.byKey(const ValueKey('comments-panel-input')), findsNothing);
    expect(find.text('登入後留言'), findsNothing);
  });

  testWidgets('viewer_can_comment 是 true：有留言框', (tester) async {
    await _pump(tester, public: FakeSocialPublic());
    expect(find.byKey(const ValueKey('comments-panel-input')), findsOneWidget);
  });

  testWidgets('以店家身分：留言與回覆的提示帶店名（C2、C3）', (tester) async {
    final public = FakeSocialPublic(
      comments: [fakeComment('c1', canReply: true)],
    );
    await _pump(tester, public: public, viewerActor: storeActor('好店'));

    expect(_hint(tester), '以「好店」身分留言…');
    await tester.tap(find.byKey(const ValueKey('comment-reply-c1')));
    await tester.pump();
    expect(_hint(tester), '以「好店」身分回覆…');
  });

  testWidgets('以買家身分：提示不帶店名', (tester) async {
    final public = FakeSocialPublic(
      comments: [fakeComment('c1', canReply: true)],
    );
    await _pump(tester, public: public, viewerActor: buyerActor('Iris'));

    expect(_hint(tester), '參與討論…');
    await tester.tap(find.byKey(const ValueKey('comment-reply-c1')));
    await tester.pump();
    expect(_hint(tester), '寫下回覆…');
  });

  testWidgets('回覆：送出 parent_id 與 reply_to_id，留言內容不經過 isMine', (tester) async {
    final public = FakeSocialPublic(
      comments: [fakeComment('c1', canReply: true)],
    );
    final app = FakeSocialApp();
    await _pump(tester, public: public, app: app);

    await tester.tap(find.byKey(const ValueKey('comment-reply-c1')));
    await tester.pump();
    await tester.enterText(
      find.byKey(const ValueKey('comments-panel-input')),
      '謝謝光臨',
    );
    await tester.pump();
    await tester.tap(find.byKey(const ValueKey('send-comment-action')));
    await tester.pumpAndSettle();

    final request = app.created.single;
    expect(request.postId, '101');
    expect(request.body, '謝謝光臨');
    expect(request.parentId, 'c1');
    expect(request.replyToId, 'c1');
    expect(request.requestId, isNotEmpty);
  });

  testWidgets('下拉重讀留言（沒有即時推送）', (tester) async {
    final public = FakeSocialPublic(comments: [fakeComment('c1')]);
    await _pump(tester, public: public);
    expect(public.listCommentsCalls, 1);

    await tester.fling(find.text('留言內文'), const Offset(0, 300), 1000);
    await tester.pumpAndSettle();

    expect(public.listCommentsCalls, 2);
  });

  testWidgets('已刪除與被隱藏的項目只顯示佔位文字，沒有動作', (tester) async {
    final public = FakeSocialPublic(
      comments: [
        fakeComment(
          'd',
          state: pb.CommentState.COMMENT_STATE_DELETED,
          canDelete: true,
          canReply: true,
        ),
        fakeComment('h', state: pb.CommentState.COMMENT_STATE_HIDDEN),
      ],
    );
    await _pump(tester, public: public);

    expect(find.text('此留言已刪除'), findsOneWidget);
    expect(find.text('此留言已被隱藏'), findsOneWidget);
    expect(find.byKey(const ValueKey('comment-delete-d')), findsNothing);
    expect(find.byKey(const ValueKey('comment-reply-d')), findsNothing);
  });

  testWidgets('打開時不另外讀 GetPost：留言權限與身分由呼叫端帶入', (tester) async {
    final public = FakeSocialPublic(comments: [fakeComment('c1')]);
    await _pump(tester, public: public, viewerActor: storeActor('好店'));

    expect(public.getPostCalls, 0);
    expect(find.byKey(const ValueKey('comments-panel-input')), findsOneWidget);
    expect(_hint(tester), '以「好店」身分留言…');
  });

  testWidgets('打開後登入：重讀 GetPost 與留言，出現留言框與身分提示', (tester) async {
    final public = FakeSocialPublic(
      post: fakePost(viewerCanComment: true, viewerActor: buyerActor('Iris')),
      comments: [fakeComment('c1')],
    );
    await _pump(
      tester,
      public: public,
      switchable: true,
      viewerCanComment: false,
    );
    expect(find.text('登入後留言'), findsOneWidget);
    expect(find.byKey(const ValueKey('comments-panel-input')), findsNothing);
    expect(public.getPostCalls, 0);
    expect(public.listCommentsCalls, 1);

    public.comments = [fakeComment('c1').deepCopy()..viewerLiked = true];
    (_container.read(sessionProvider.notifier) as SwitchableSession).signIn();
    await tester.pumpAndSettle();

    expect(public.getPostCalls, 1);
    expect(public.listCommentsCalls, 2);
    expect(find.byKey(const ValueKey('comments-panel-input')), findsOneWidget);
    expect(find.text('登入後留言'), findsNothing);
    expect(_hint(tester), '參與討論…');
  });

  testWidgets('留言讀取失敗：顯示錯誤與重試，不是「還沒有留言」；重試後讀到', (tester) async {
    var fail = true;
    final public = FakeSocialPublic(
      commentsResponder: (_) {
        if (fail) throw ConnectException(Code.unavailable, 'down');
        return pb.ListCommentsResponse(comments: [fakeComment('c1')]);
      },
    );
    await _pump(tester, public: public);

    expect(find.byKey(const ValueKey('comments-load-error')), findsOneWidget);
    expect(find.text('載入失敗，請稍後再試'), findsOneWidget);
    expect(find.byKey(const ValueKey('comments-empty-state')), findsNothing);

    fail = false;
    await tester.tap(find.byKey(const ValueKey('comments-load-retry')));
    await tester.pumpAndSettle();

    expect(find.byKey(const ValueKey('comments-load-error')), findsNothing);
    expect(find.text('留言內文'), findsOneWidget);
  });

  testWidgets('「查看更多留言」帶上一頁的 page_token，下一頁接在後面', (tester) async {
    final public = FakeSocialPublic(
      commentsResponder: (req) => req.pageToken.isEmpty
          ? pb.ListCommentsResponse(
              comments: [fakeComment('c1', body: '第一頁')],
              nextPageToken: 'page-2',
            )
          : pb.ListCommentsResponse(comments: [fakeComment('c2', body: '第二頁')]),
    );
    await _pump(tester, public: public);
    expect(find.text('第二頁'), findsNothing);

    await tester.tap(find.byKey(const ValueKey('comments-load-more')));
    await tester.pumpAndSettle();

    expect(public.listCommentsRequests.last.pageToken, 'page-2');
    expect(find.text('第一頁'), findsOneWidget);
    expect(find.text('第二頁'), findsOneWidget);
    expect(find.byKey(const ValueKey('comments-load-more')), findsNothing);
  });

  testWidgets('「查看更多回覆」帶 parent_id 與 page_token，回覆接在後面', (tester) async {
    final public = FakeSocialPublic(
      commentsResponder: (req) {
        if (!req.hasParentId()) {
          return pb.ListCommentsResponse(
            comments: [fakeComment('c1', replyCount: 2)],
          );
        }
        return req.pageToken.isEmpty
            ? pb.ListCommentsResponse(
                comments: [fakeComment('r1', body: '回覆一')],
                nextPageToken: 'rp-2',
              )
            : pb.ListCommentsResponse(
                comments: [fakeComment('r2', body: '回覆二')],
              );
      },
    );
    await _pump(tester, public: public);

    await tester.tap(find.byKey(const ValueKey('comment-more-replies-c1')));
    await tester.pumpAndSettle();

    final replyRequests = public.listCommentsRequests.where(
      (r) => r.hasParentId(),
    );
    expect(replyRequests.map((r) => r.parentId), everyElement('c1'));
    expect(replyRequests.map((r) => r.pageToken), ['', 'rp-2']);
    expect(find.text('回覆一'), findsOneWidget);
    expect(find.text('回覆二'), findsOneWidget);
  });
}
