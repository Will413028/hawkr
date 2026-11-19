import 'dart:async';

import 'package:flutter/material.dart';
import 'package:flutter_screenutil/flutter_screenutil.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:go_router/go_router.dart';
import 'package:hawkr_api/gen/hawkr/social/v1alpha1/social.pb.dart' as pb;
import 'package:hawkr_api/transport.dart' show Code, ConnectException;
import 'package:hawkr_buyer/config/flavor.dart';
import 'package:hawkr_buyer/core/networking/connect_provider.dart';
import 'package:hawkr_buyer/core/session/session_provider.dart';
import 'package:hawkr_buyer/core/theme/app_theme.dart';
import 'package:hawkr_buyer/features/social/data/social_model.dart';
import 'package:hawkr_buyer/features/social/presentation/comments_notifier.dart';
import 'package:hawkr_buyer/features/social/presentation/post_entity_store.dart';
import 'package:hawkr_buyer/shared/widgets/comments_bottom_sheet.dart';
import 'package:hooks_riverpod/hooks_riverpod.dart';

import '../../features/social/social_fakes.dart';

late ProviderContainer _container;

SwitchableSession get _session =>
    _container.read(sessionProvider.notifier) as SwitchableSession;

Future<void> _pump(
  WidgetTester tester,
  FakeSocialPublic public, {
  bool viewerCanComment = true,
}) async {
  final container = ProviderContainer(
    overrides: [
      socialPublicClientProvider.overrideWithValue(public.client()),
      socialAppClientProvider.overrideWithValue(FakeSocialApp().client()),
      sessionProvider.overrideWith(SwitchableSession.new),
    ],
  );
  addTearDown(container.dispose);
  _container = container;
  await container.read(sessionProvider.future);
  container.read(postEntityStoreProvider.notifier).seed([
    Post(id: 101, viewerCanComment: viewerCanComment),
  ]);
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
                builder: (_, _) =>
                    Scaffold(body: CommentsBottomSheet(postId: 101)),
              ),
            ],
          ),
        ),
      ),
    ),
  );
  await tester.pumpAndSettle();
}

const _input = ValueKey('comments-panel-input');

void main() {
  if (!F.isInitialized) F.init(Flavor.dev);

  testWidgets('R2-3 換觀看者後重讀失敗：清掉前一位的留言與權限，顯示錯誤與重試', (tester) async {
    var calls = 0;
    final public = FakeSocialPublic(
      post: fakePost(viewerCanComment: false),
      commentsResponder: (_) {
        if (++calls > 1) throw ConnectException(Code.unavailable, 'down');
        return pb.ListCommentsResponse(
          comments: [fakeComment('c1', body: '前一位看得到', canDelete: true)],
        );
      },
    );
    await _pump(tester, public);
    expect(find.text('前一位看得到'), findsOneWidget);
    expect(find.byKey(_input), findsOneWidget);
    expect(find.byKey(const ValueKey('comment-delete-c1')), findsOneWidget);

    _session.signIn();
    await tester.pumpAndSettle();

    expect(find.byKey(const ValueKey('comments-load-error')), findsOneWidget);
    expect(find.text('載入失敗，請稍後再試'), findsOneWidget);
    expect(find.text('前一位看得到'), findsNothing);
    expect(find.byKey(const ValueKey('comment-delete-c1')), findsNothing);
    expect(find.byKey(_input), findsNothing);
  });

  testWidgets('R2-5 換觀看者前發出的回覆晚回來：丟棄，且「查看更多回覆」不卡在載入中', (tester) async {
    final oldReplies = Completer<pb.ListCommentsResponse>();
    var replyCalls = 0;
    final public = FakeSocialPublic(
      commentsResponder: (req) {
        if (!req.hasParentId()) {
          return pb.ListCommentsResponse(
            comments: [fakeComment('c1', replyCount: 1)],
          );
        }
        if (++replyCalls == 1) return oldReplies.future;
        return pb.ListCommentsResponse(
          comments: [fakeComment('r2', body: '新觀看者的回覆')],
        );
      },
    );
    await _pump(tester, public);
    await tester.tap(find.byKey(const ValueKey('comment-more-replies-c1')));
    await tester.pump();
    expect(replyCalls, 1);

    _session.signIn();
    await tester.pumpAndSettle();
    oldReplies.complete(
      pb.ListCommentsResponse(comments: [fakeComment('r1', body: '舊的回覆')]),
    );
    await tester.pumpAndSettle();

    expect(find.text('舊的回覆'), findsNothing);

    await tester.tap(find.byKey(const ValueKey('comment-more-replies-c1')));
    await tester.pumpAndSettle();

    expect(replyCalls, 2);
    expect(find.text('新觀看者的回覆'), findsOneWidget);
  });

  testWidgets('R2-6 連續換觀看者：較舊的 GetPost 晚回來不蓋掉新的留言權限', (tester) async {
    final oldPost = Completer<pb.Post>();
    var postCalls = 0;
    final public = FakeSocialPublic(
      comments: [fakeComment('c1')],
      getPostResponder: () {
        if (++postCalls == 1) return oldPost.future;
        return fakePost(viewerCanComment: false);
      },
    );
    await _pump(tester, public, viewerCanComment: false);

    _session.signIn();
    await tester.pump();
    _session.switchTo('account-3002');
    await tester.pumpAndSettle();
    oldPost.complete(fakePost(viewerCanComment: true));
    await tester.pumpAndSettle();

    expect(postCalls, 2);
    expect(find.byKey(_input), findsNothing);
  });

  testWidgets('R2-7 下拉重讀一併重讀 GetPost 的留言權限', (tester) async {
    final public = FakeSocialPublic(
      post: fakePost(viewerCanComment: true),
      comments: [fakeComment('c1')],
    );
    await _pump(tester, public, viewerCanComment: false);
    expect(public.getPostCalls, 0);
    expect(find.byKey(_input), findsNothing);

    await tester.fling(find.text('留言內文'), const Offset(0, 300), 1000);
    await tester.pumpAndSettle();

    expect(public.getPostCalls, 1);
    expect(find.byKey(_input), findsOneWidget);
  });

  testWidgets('R2-7 讀取失敗後重試一併重讀 GetPost 的留言權限', (tester) async {
    var fail = true;
    final public = FakeSocialPublic(
      post: fakePost(viewerCanComment: true),
      commentsResponder: (_) {
        if (fail) throw ConnectException(Code.unavailable, 'down');
        return pb.ListCommentsResponse(comments: [fakeComment('c1')]);
      },
    );
    await _pump(tester, public, viewerCanComment: false);
    expect(public.getPostCalls, 0);

    fail = false;
    await tester.tap(find.byKey(const ValueKey('comments-load-retry')));
    await tester.pumpAndSettle();

    expect(public.getPostCalls, 1);
    expect(find.byKey(_input), findsOneWidget);
  });

  testWidgets('換觀看者前發出的分頁晚回來：丟棄，不接到新觀看者的列表', (tester) async {
    final oldPage = Completer<pb.ListCommentsResponse>();
    var calls = 0;
    final public = FakeSocialPublic(
      commentsResponder: (req) {
        calls++;
        if (calls == 1) {
          return pb.ListCommentsResponse(
            comments: [fakeComment('c1', body: '第一頁')],
            nextPageToken: 'p2',
          );
        }
        if (req.pageToken == 'p2') return oldPage.future;
        return pb.ListCommentsResponse(
          comments: [fakeComment('n1', body: '新觀看者')],
        );
      },
    );
    await _pump(tester, public);
    await tester.tap(find.byKey(const ValueKey('comments-load-more')));
    await tester.pump();

    _session.signIn();
    await tester.pumpAndSettle();
    oldPage.complete(
      pb.ListCommentsResponse(comments: [fakeComment('c2', body: '舊的第二頁')]),
    );
    await tester.pumpAndSettle();

    expect(find.text('舊的第二頁'), findsNothing);
    expect(find.text('新觀看者'), findsOneWidget);
  });

  testWidgets('R3-5 重讀失敗：留言框與已載入的留言都留著', (tester) async {
    var fail = false;
    final public = FakeSocialPublic(
      commentsResponder: (_) {
        if (fail) throw ConnectException(Code.unavailable, 'down');
        return pb.ListCommentsResponse(
          comments: [fakeComment('c1', body: '已載入的留言')],
        );
      },
    );
    await _pump(tester, public);

    fail = true;
    final ok = await _container.read(commentsProvider(101).notifier).reload();
    await tester.pumpAndSettle();

    expect(ok, isFalse);
    expect(find.text('已載入的留言'), findsOneWidget);
    expect(find.byKey(_input), findsOneWidget);
    expect(find.byKey(const ValueKey('comments-load-error')), findsNothing);
  });

  testWidgets('換觀看者後貼文實體重讀：新觀看者的留言權限回來，輸入框出現', (tester) async {
    final public = FakeSocialPublic(
      post: fakePost(viewerCanComment: true),
      comments: [fakeComment('c1')],
    );
    await _pump(tester, public, viewerCanComment: false);
    expect(public.getPostCalls, 0);
    expect(find.byKey(_input), findsNothing);

    _session.signIn();
    await tester.pumpAndSettle();

    expect(public.getPostCalls, 1);
    expect(find.byKey(_input), findsOneWidget);
  });
}
