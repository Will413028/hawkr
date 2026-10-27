import 'dart:async';

import 'package:hawkr_seller/core/theme/app_theme.dart';
import 'package:hawkr_seller/features/post/data/post_repository.dart';
import 'package:hawkr_seller/shared/widgets/comments_bottom_sheet.dart';
import 'package:flutter/material.dart';
import 'package:flutter_screenutil/flutter_screenutil.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:hooks_riverpod/hooks_riverpod.dart';
import 'package:mocktail/mocktail.dart';

class _MockPostRepository extends Mock implements PostRepository {}

void main() {
  late _MockPostRepository repository;

  setUp(() {
    repository = _MockPostRepository();
    when(() => repository.listCommentsRaw(42)).thenAnswer(
      (_) async => <Map<String, dynamic>>[
        <String, dynamic>{
          'id': 1,
          'isMine': false,
          'userName': '小明',
          'content': '第一則留言',
          'likeCount': 2,
          'liked': false,
        },
        <String, dynamic>{
          'id': 2,
          'parentId': 1,
          'isMine': false,
          'userName': '小華',
          'content': '第一則回覆',
          'likeCount': 1,
          'liked': true,
        },
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
          builder: (_, __) => MaterialApp(
            theme: AppTheme.light,
            home: Scaffold(
              body: CommentsBottomSheet(postId: 42, postRepository: repository),
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
      final response = Completer<Map<String, dynamic>?>();
      when(
        () => repository.commentOnPostRaw(42, content: '測試留言'),
      ).thenAnswer((_) => response.future);
      await pumpSheet(tester);
      await tester.enterText(find.byType(TextField), '測試留言');
      await tester.pump();
      await tester.tap(find.byKey(const ValueKey('send-comment-action')));
      await tester.tap(find.byKey(const ValueKey('send-comment-action')));
      await tester.pump();
      verify(() => repository.commentOnPostRaw(42, content: '測試留言')).called(1);
      expect(find.byType(CircularProgressIndicator), findsOneWidget);
      response.complete(null);
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
    final response = Completer<Map<String, dynamic>?>();
    when(
      () => repository.replyToComment(1, postId: 42, content: '回覆內容'),
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
    response.complete({'id': 3, 'parentId': 1, 'content': '回覆內容'});
    await tester.pumpAndSettle();
    expect(
      tester.widget<TextField>(find.byType(TextField)).controller!.text,
      isEmpty,
    );
    expect(find.byKey(const ValueKey('cancel-comment-reply')), findsNothing);
    verify(
      () => repository.replyToComment(1, postId: 42, content: '回覆內容'),
    ).called(1);
    expect(tester.takeException(), isNull);
  });

  testWidgets('thrown send error retains draft and allows retry', (
    tester,
  ) async {
    when(
      () => repository.commentOnPostRaw(42, content: '網路重試'),
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
        final response = Completer<Map<String, dynamic>?>();
        if (replying) {
          when(
            () => repository.replyToComment(1, postId: 42, content: '離頁'),
          ).thenAnswer((_) => response.future);
        } else {
          when(
            () => repository.commentOnPostRaw(42, content: '離頁'),
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
        response.complete({'id': 3, 'content': '離頁'});
        await tester.pumpAndSettle();
        expect(tester.takeException(), isNull);
      },
    );
  }

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
