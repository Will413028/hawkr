import 'dart:async';

import 'package:hawkr_api/gen/hawkr/social/v1alpha1/social.pbenum.dart';
import 'package:hawkr_seller/core/theme/app_theme.dart';
import 'package:hawkr_seller/features/post/data/post_comment.dart';
import 'package:hawkr_seller/features/post/data/post_repository.dart';
import 'package:hawkr_seller/shared/widgets/comments_bottom_sheet.dart';
import 'package:flutter/material.dart';
import 'package:flutter_screenutil/flutter_screenutil.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:hooks_riverpod/hooks_riverpod.dart';
import 'package:mocktail/mocktail.dart';

import '../../features/post/post_comment_fixtures.dart';

class _MockPostRepository extends Mock implements PostRepository {}

void main() {
  late _MockPostRepository repository;

  setUp(() {
    repository = _MockPostRepository();
    when(() => repository.listComments(42)).thenAnswer(
      (_) async => [
        testComment(author: '小明', body: '第一則留言', likeCount: 2, replyCount: 1),
      ],
    );
    when(() => repository.listComments(42, parentId: '1')).thenAnswer(
      (_) async => [
        testComment(
          id: '2',
          parentId: '1',
          author: '小華',
          body: '第一則回覆',
          likeCount: 1,
          liked: true,
        ),
      ],
    );
  });

  Future<void> pumpSheet(WidgetTester tester) async {
    tester.view.physicalSize = const Size(390, 844);
    tester.view.devicePixelRatio = 1;
    addTearDown(tester.view.resetPhysicalSize);
    addTearDown(tester.view.resetDevicePixelRatio);

    await tester.pumpWidget(
      ProviderScope(
        child: ScreenUtilInit(
          designSize: const Size(390, 844),
          builder: (_, _) => MaterialApp(
            theme: AppTheme.light,
            home: Scaffold(
              body: CommentsBottomSheet(
                postId: 42,
                postRepository: repository,
                initialCommentCount: 2,
                canComment: true,
              ),
            ),
          ),
        ),
      ),
    );
    await tester.pump();
    await tester.pump();
  }

  testWidgets('comment actions expose stateful labeled 48dp targets', (
    tester,
  ) async {
    final semantics = tester.ensureSemantics();
    await pumpSheet(tester);

    const expectedLabels = <String>[
      '關閉留言',
      '送出留言',
      '回覆 小明 的留言',
      '喜歡 小明 的留言',
      '取消喜歡 小華 的回覆',
    ];
    for (final label in expectedLabels) {
      expect(
        find.bySemanticsLabel(label),
        findsOneWidget,
        reason: 'missing or duplicate semantic label: $label',
      );
      final size = tester.getSize(find.bySemanticsLabel(label));
      expect(size.width, greaterThanOrEqualTo(48), reason: '$label width');
      expect(size.height, greaterThanOrEqualTo(48), reason: '$label height');
    }

    expect(
      tester.getSemantics(find.bySemanticsLabel('送出留言')),
      isSemantics(
        label: '送出留言',
        isButton: true,
        hasEnabledState: true,
        isEnabled: false,
        hasTapAction: false,
      ),
    );
    expect(
      tester.getSemantics(find.bySemanticsLabel('取消喜歡 小華 的回覆')),
      isSemantics(
        label: '取消喜歡 小華 的回覆',
        hasToggledState: true,
        isToggled: true,
        isButton: true,
        hasEnabledState: true,
        isEnabled: true,
        hasTapAction: true,
      ),
    );

    await tester.tap(find.bySemanticsLabel('回覆 小明 的留言'));
    await tester.pump();
    expect(find.bySemanticsLabel('取消回覆'), findsOneWidget);
    final cancelSize = tester.getSize(find.bySemanticsLabel('取消回覆'));
    expect(cancelSize.width, greaterThanOrEqualTo(48));
    expect(cancelSize.height, greaterThanOrEqualTo(48));

    await tester.tap(find.bySemanticsLabel('取消回覆'));
    await tester.pump();
    expect(find.bySemanticsLabel('取消回覆'), findsNothing);
    semantics.dispose();
  });

  testWidgets(
    'pending comment send rejects duplicate taps and retains failed draft',
    (tester) async {
      final response = Completer<PostComment>();
      when(
        () => repository.createComment(42, body: '測試留言'),
      ).thenAnswer((_) => response.future);
      await pumpSheet(tester);
      await tester.enterText(find.byType(TextField), '測試留言');
      await tester.pump();
      await tester.tap(find.byKey(const ValueKey('send-comment-action')));
      await tester.tap(find.byKey(const ValueKey('send-comment-action')));
      await tester.pump();
      verify(() => repository.createComment(42, body: '測試留言')).called(1);
      expect(find.byType(CircularProgressIndicator), findsOneWidget);
      response.completeError(StateError('offline'));
      await tester.pumpAndSettle();
      expect(
        tester.widget<TextField>(find.byType(TextField)).controller!.text,
        '測試留言',
      );
      expect(find.byType(CircularProgressIndicator), findsNothing);
      await tester.pump(const Duration(seconds: 3));
      expect(tester.takeException(), isNull);
    },
  );

  testWidgets('pending reply keeps its target and draft until success', (
    tester,
  ) async {
    final response = Completer<PostComment>();
    when(
      () => repository.createComment(42, body: '回覆內容', parentId: '1'),
    ).thenAnswer((_) => response.future);
    await pumpSheet(tester);
    await tester.tap(find.byKey(const ValueKey('comment-reply-1')));
    await tester.pump();
    await tester.enterText(find.byType(TextField), '回覆內容');
    await tester.pump();
    await tester.tap(find.byKey(const ValueKey('send-comment-action')));
    await tester.pump();
    expect(tester.widget<TextField>(find.byType(TextField)).readOnly, isTrue);
    await tester.tap(find.byKey(const ValueKey('cancel-comment-reply')));
    await tester.tap(find.byKey(const ValueKey('comment-reply-1')));
    await tester.pump();
    expect(
      tester.widget<TextField>(find.byType(TextField)).controller!.text,
      '回覆內容',
    );
    response.complete(testComment(id: '3', parentId: '1'));
    await tester.pumpAndSettle();
    expect(
      tester.widget<TextField>(find.byType(TextField)).controller!.text,
      isEmpty,
    );
    expect(find.byKey(const ValueKey('cancel-comment-reply')), findsNothing);
    verify(
      () => repository.createComment(42, body: '回覆內容', parentId: '1'),
    ).called(1);
    expect(tester.takeException(), isNull);
  });

  testWidgets('thrown send error retains draft and allows retry', (
    tester,
  ) async {
    when(
      () => repository.createComment(42, body: '網路重試'),
    ).thenThrow(StateError('offline'));
    await pumpSheet(tester);
    await tester.enterText(find.byType(TextField), '網路重試');
    await tester.pump();
    await tester.tap(find.byKey(const ValueKey('send-comment-action')));
    await tester.pumpAndSettle();
    expect(tester.takeException(), isNull);
    expect(
      tester.widget<TextField>(find.byType(TextField)).controller!.text,
      '網路重試',
    );
    expect(find.byType(CircularProgressIndicator), findsNothing);
    await tester.pump(const Duration(seconds: 3));
  });

  for (final replying in [false, true]) {
    testWidgets(
      'closing pending ${replying ? 'reply' : 'comment'} ignores completion',
      (tester) async {
        final response = Completer<PostComment>();
        if (replying) {
          when(
            () => repository.createComment(42, body: '離頁', parentId: '1'),
          ).thenAnswer((_) => response.future);
        } else {
          when(
            () => repository.createComment(42, body: '離頁'),
          ).thenAnswer((_) => response.future);
        }
        await pumpSheet(tester);
        if (replying) {
          await tester.tap(find.byKey(const ValueKey('comment-reply-1')));
          await tester.pump();
        }
        await tester.enterText(find.byType(TextField), '離頁');
        await tester.pump();
        await tester.tap(find.byKey(const ValueKey('send-comment-action')));
        await tester.pumpWidget(const SizedBox.shrink());
        response.complete(testComment(id: '3'));
        await tester.pumpAndSettle();
        expect(tester.takeException(), isNull);
      },
    );
  }

  testWidgets('reply control and long-press delete follow viewer_can_*', (
    tester,
  ) async {
    when(() => repository.listComments(42)).thenAnswer(
      (_) async => [
        testComment(id: '1', canReply: false, canDelete: false),
        testComment(id: '3', body: '可管理', canReply: true, canDelete: true),
      ],
    );
    when(
      () => repository.listComments(
        42,
        parentId: any(named: 'parentId', that: isNotNull),
      ),
    ).thenAnswer((_) async => const <PostComment>[]);
    when(
      () => repository.deleteComment('3', etag: 'etag-3'),
    ).thenAnswer((_) async {});
    await pumpSheet(tester);
    expect(find.byKey(const ValueKey('comment-reply-1')), findsNothing);
    expect(find.byKey(const ValueKey('comment-reply-3')), findsOneWidget);

    await tester.longPress(find.text('留言內容'));
    await tester.pumpAndSettle();
    expect(find.text('確定要刪除此留言嗎？\n刪除後將無法復原。'), findsNothing);

    await tester.longPress(find.text('可管理'));
    await tester.pumpAndSettle();
    await tester.tap(find.text('刪除'));
    await tester.pumpAndSettle();
    verify(() => repository.deleteComment('3', etag: 'etag-3')).called(1);
    await tester.pump(const Duration(seconds: 3));
  });

  testWidgets(
    'comments load and show the notice even when buyers cannot see them',
    (tester) async {
      tester.view.physicalSize = const Size(390, 844);
      tester.view.devicePixelRatio = 1;
      addTearDown(tester.view.resetPhysicalSize);
      addTearDown(tester.view.resetDevicePixelRatio);
      await tester.pumpWidget(
        ProviderScope(
          child: ScreenUtilInit(
            designSize: const Size(390, 844),
            builder: (_, _) => MaterialApp(
              theme: AppTheme.light,
              home: Scaffold(
                body: CommentsBottomSheet(
                  postId: 42,
                  postRepository: repository,
                  initialCommentCount: 2,
                  notice: '店家目前不公開，買家看不到這些留言。',
                ),
              ),
            ),
          ),
        ),
      );
      await tester.pump();
      await tester.pump();
      expect(find.text('店家目前不公開，買家看不到這些留言。'), findsOneWidget);
      expect(find.text('第一則留言'), findsOneWidget);
      expect(find.byKey(const ValueKey('send-comment-action')), findsNothing);
      verify(() => repository.listComments(42)).called(1);
    },
  );

  testWidgets('like toggles by viewer_liked and placeholders show text', (
    tester,
  ) async {
    when(() => repository.listComments(42)).thenAnswer(
      (_) async => [
        testComment(id: '1', author: '小明', body: '留言', likeCount: 2),
        testComment(id: '4', state: CommentState.COMMENT_STATE_HIDDEN),
      ],
    );
    when(
      () => repository.listComments(
        42,
        parentId: any(named: 'parentId', that: isNotNull),
      ),
    ).thenAnswer((_) async => const <PostComment>[]);
    when(() => repository.likeComment('1')).thenAnswer((_) async {});
    await pumpSheet(tester);
    expect(find.text('此留言已被隱藏'), findsOneWidget);
    await tester.tap(find.byKey(const ValueKey('comment-like-1')));
    await tester.pumpAndSettle();
    verify(() => repository.likeComment('1')).called(1);
    verifyNever(() => repository.unlikeComment(any()));
    expect(find.text('3'), findsOneWidget);
    when(() => repository.unlikeComment('1')).thenAnswer((_) async {});
    await tester.tap(find.byKey(const ValueKey('comment-like-1')));
    await tester.pumpAndSettle();
    verify(() => repository.unlikeComment('1')).called(1);
    expect(find.text('2'), findsOneWidget);
  });

  testWidgets('a double tap on like sends one request', (tester) async {
    final response = Completer<void>();
    when(() => repository.likeComment('1')).thenAnswer((_) => response.future);
    await pumpSheet(tester);
    await tester.tap(find.byKey(const ValueKey('comment-like-1')));
    await tester.pump();
    await tester.tap(find.byKey(const ValueKey('comment-like-1')));
    await tester.pump();
    verify(() => repository.likeComment('1')).called(1);
    verifyNever(() => repository.unlikeComment(any()));
    response.complete();
    await tester.pumpAndSettle();
  });

  testWidgets('a reply created while replies load is kept', (tester) async {
    final fetch = Completer<List<PostComment>>();
    when(
      () => repository.listComments(42, parentId: '1'),
    ).thenAnswer((_) => fetch.future);
    when(
      () => repository.createComment(
        42,
        body: any(named: 'body'),
        parentId: '1',
      ),
    ).thenAnswer(
      (_) async => testComment(id: '9', parentId: '1', body: '我剛送出的回覆'),
    );
    await pumpSheet(tester);
    await tester.tap(find.byKey(const ValueKey('comment-reply-1')));
    await tester.pump();
    await tester.enterText(find.byType(TextField), '我剛送出的回覆');
    await tester.pump();
    await tester.tap(find.byKey(const ValueKey('send-comment-action')));
    await tester.pump();
    await tester.pump();
    fetch.complete([testComment(id: '2', parentId: '1', body: '伺服器的回覆')]);
    await tester.pumpAndSettle();
    expect(find.text('伺服器的回覆'), findsOneWidget);
    expect(find.text('我剛送出的回覆'), findsOneWidget);
  });

  Future<List<int>> pumpCounting(WidgetTester tester) async {
    final counts = <int>[];
    tester.view.physicalSize = const Size(390, 844);
    tester.view.devicePixelRatio = 1;
    addTearDown(tester.view.resetPhysicalSize);
    addTearDown(tester.view.resetDevicePixelRatio);
    await tester.pumpWidget(
      ProviderScope(
        child: ScreenUtilInit(
          designSize: const Size(390, 844),
          builder: (_, _) => MaterialApp(
            theme: AppTheme.light,
            home: Scaffold(
              body: CommentsBottomSheet(
                postId: 42,
                postRepository: repository,
                initialCommentCount: 5,
                canComment: true,
                onCommentCountChanged: counts.add,
              ),
            ),
          ),
        ),
      ),
    );
    await tester.pump();
    await tester.pump();
    return counts;
  }

  testWidgets('opening reports no count and never reads the post', (
    tester,
  ) async {
    final counts = await pumpCounting(tester);
    expect(counts, isEmpty);
    verifyNever(() => repository.getMyPost(any()));
  });

  testWidgets('a created comment or reply adds one to the opening count', (
    tester,
  ) async {
    when(
      () => repository.createComment(42, body: any(named: 'body')),
    ).thenAnswer((_) async => testComment(id: '8', body: '新留言'));
    when(
      () => repository.createComment(
        42,
        body: any(named: 'body'),
        parentId: '1',
      ),
    ).thenAnswer((_) async => testComment(id: '9', parentId: '1', body: '回'));
    final counts = await pumpCounting(tester);
    await tester.enterText(find.byType(TextField), '新留言');
    await tester.pump();
    await tester.tap(find.byKey(const ValueKey('send-comment-action')));
    await tester.pumpAndSettle();
    expect(counts, [6]);
    await tester.tap(find.byKey(const ValueKey('comment-reply-1')));
    await tester.pump();
    await tester.enterText(find.byType(TextField), '回');
    await tester.pump();
    await tester.tap(find.byKey(const ValueKey('send-comment-action')));
    await tester.pumpAndSettle();
    expect(counts, [6, 7]);
    verifyNever(() => repository.getMyPost(any()));
  });

  testWidgets('a deleted comment takes one off the count', (tester) async {
    when(() => repository.listComments(42)).thenAnswer(
      (_) async => [
        testComment(id: '3', body: '可管理', canReply: true, canDelete: true),
      ],
    );
    when(
      () => repository.listComments(
        42,
        parentId: any(named: 'parentId', that: isNotNull),
      ),
    ).thenAnswer((_) async => const <PostComment>[]);
    when(
      () => repository.deleteComment('3', etag: 'etag-3'),
    ).thenAnswer((_) async {});
    final counts = await pumpCounting(tester);
    await tester.longPress(find.text('可管理'));
    await tester.pumpAndSettle();
    await tester.tap(find.text('刪除'));
    await tester.pumpAndSettle();
    expect(counts, [4]);
    await tester.pump(const Duration(seconds: 3));
  });

  testWidgets('a failed delete leaves the count alone', (tester) async {
    when(() => repository.listComments(42)).thenAnswer(
      (_) async => [
        testComment(id: '3', body: '可管理', canReply: true, canDelete: true),
      ],
    );
    when(
      () => repository.listComments(
        42,
        parentId: any(named: 'parentId', that: isNotNull),
      ),
    ).thenAnswer((_) async => const <PostComment>[]);
    when(
      () => repository.deleteComment('3', etag: 'etag-3'),
    ).thenThrow(Exception('offline'));
    final counts = await pumpCounting(tester);
    await tester.longPress(find.text('可管理'));
    await tester.pumpAndSettle();
    await tester.tap(find.text('刪除'));
    await tester.pumpAndSettle();
    expect(counts, isEmpty);
    await tester.pump(const Duration(seconds: 3));
  });

  testWidgets(
    'a reply sent after a failed reply fetch still loads the thread',
    (tester) async {
      var attempts = 0;
      when(() => repository.listComments(42, parentId: '1')).thenAnswer((
        _,
      ) async {
        attempts++;
        if (attempts == 1) throw Exception('offline');
        return [testComment(id: '2', parentId: '1', body: '伺服器的回覆')];
      });
      when(
        () => repository.createComment(
          42,
          body: any(named: 'body'),
          parentId: '1',
        ),
      ).thenAnswer(
        (_) async => testComment(id: '9', parentId: '1', body: '我剛送出的回覆'),
      );
      await pumpSheet(tester);
      await tester.pumpAndSettle();
      expect(attempts, 1);
      expect(find.text('伺服器的回覆'), findsNothing);
      await tester.tap(find.byKey(const ValueKey('comment-reply-1')));
      await tester.pump();
      await tester.enterText(find.byType(TextField), '我剛送出的回覆');
      await tester.pump();
      await tester.tap(find.byKey(const ValueKey('send-comment-action')));
      await tester.pumpAndSettle();
      expect(attempts, 2);
      expect(find.text('伺服器的回覆'), findsOneWidget);
      expect(find.text('我剛送出的回覆'), findsOneWidget);
      await tester.pump(const Duration(seconds: 3));
    },
  );

  testWidgets('comment sheet passes all three accessibility guidelines', (
    tester,
  ) async {
    final semantics = tester.ensureSemantics();
    await pumpSheet(tester);

    await expectLater(tester, meetsGuideline(androidTapTargetGuideline));
    await expectLater(tester, meetsGuideline(iOSTapTargetGuideline));
    await expectLater(tester, meetsGuideline(labeledTapTargetGuideline));
    semantics.dispose();
  });
}
