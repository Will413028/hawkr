import 'package:flutter/material.dart';
import 'package:flutter_screenutil/flutter_screenutil.dart';
import 'package:flutter_test/flutter_test.dart';
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

class _ActiveSession extends Session {
  @override
  Future<SessionModel?> build() async =>
      const SessionModel(userId: 'account-3001', nickname: 'Iris');
}

Future<void> _pump(
  WidgetTester tester, {
  required FakeSocialPublic public,
  FakeSocialApp? app,
}) async {
  final container = ProviderContainer(
    overrides: [
      socialPublicClientProvider.overrideWithValue(public.client()),
      socialAppClientProvider.overrideWithValue(
        (app ?? FakeSocialApp()).client(),
      ),
      sessionProvider.overrideWith(_ActiveSession.new),
    ],
  );
  addTearDown(container.dispose);
  await container.read(sessionProvider.future);
  await tester.pumpWidget(
    UncontrolledProviderScope(
      container: container,
      child: ScreenUtilInit(
        designSize: const Size(390, 844),
        enableScaleWH: () => false,
        enableScaleText: () => false,
        builder: (_, _) => MaterialApp(
          theme: AppTheme.light,
          home: const Scaffold(body: CommentsBottomSheet(postId: '101')),
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

  testWidgets('刪除：確認後送出 DeleteComment（帶 etag）並重讀留言', (tester) async {
    final public = FakeSocialPublic(
      comments: [fakeComment('c1', canDelete: true)],
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
    await _pump(
      tester,
      public: FakeSocialPublic(post: fakePost(viewerCanComment: false)),
    );
    expect(find.byKey(const ValueKey('comments-panel-input')), findsNothing);
  });

  testWidgets('viewer_can_comment 是 true：有留言框', (tester) async {
    await _pump(
      tester,
      public: FakeSocialPublic(post: fakePost(viewerCanComment: true)),
    );
    expect(find.byKey(const ValueKey('comments-panel-input')), findsOneWidget);
  });

  testWidgets('以店家身分：留言與回覆的提示帶店名（C2、C3）', (tester) async {
    final public = FakeSocialPublic(
      post: fakePost(viewerActor: storeActor('好店')),
      comments: [fakeComment('c1', canReply: true)],
    );
    await _pump(tester, public: public);

    expect(_hint(tester), '以「好店」身分留言…');
    await tester.tap(find.byKey(const ValueKey('comment-reply-c1')));
    await tester.pump();
    expect(_hint(tester), '以「好店」身分回覆…');
  });

  testWidgets('以買家身分：提示不帶店名', (tester) async {
    final public = FakeSocialPublic(
      post: fakePost(viewerActor: buyerActor('Iris')),
      comments: [fakeComment('c1', canReply: true)],
    );
    await _pump(tester, public: public);

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

  testWidgets('貼文讀不到（不公開）時留言仍讀得到，但不能留言', (tester) async {
    final public = FakeSocialPublic(
      getPostError: ConnectException(Code.notFound, 'gone'),
      comments: [fakeComment('c1')],
    );
    await _pump(tester, public: public);

    expect(find.text('留言內文'), findsOneWidget);
    expect(find.byKey(const ValueKey('comments-panel-input')), findsNothing);
  });
}
