import 'dart:async';

import 'package:hawkr_seller/config/flavor.dart';
import 'package:hawkr_seller/core/theme/app_theme.dart';
import 'package:hawkr_seller/features/post/data/post_model.dart';
import 'package:hawkr_seller/features/post/data/post_repository.dart';
import 'package:hawkr_seller/features/post/presentation/page_home_feed_screen.dart';
import 'package:flutter/material.dart';
import 'package:flutter_screenutil/flutter_screenutil.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:hooks_riverpod/hooks_riverpod.dart';
import 'package:mocktail/mocktail.dart';

class _Repo extends Mock implements PostRepository {}

void main() {
  setUpAll(() {
    if (!F.isInitialized) F.init(Flavor.dev);
  });
  for (final self in [false, true]) {
    testWidgets(
      'feed ${self ? 'own' : 'other'} more keeps ownership, cancels, and retries failed delete',
      (tester) async {
        tester.view.physicalSize = const Size(390, 844);
        tester.view.devicePixelRatio = 1;
        addTearDown(tester.view.resetPhysicalSize);
        addTearDown(tester.view.resetDevicePixelRatio);
        final repo = _Repo();
        var comments = <Map<String, dynamic>>[
          {'id': 1, 'isMine': self, 'userName': '測試留言者', 'content': '留言內容'},
        ];
        when(() => repo.listMyPosts()).thenAnswer(
          (_) async => [
            Post(
              id: 42,
              sellerId: 7,
              contentType: 'IMAGE',
              content: '貼文內容',
              commentCount: 1,
            ),
          ],
        );
        when(() => repo.listCommentsRaw(42)).thenAnswer((_) async => comments);
        when(() => repo.listCommentReplies(1)).thenAnswer((_) async => []);
        var response = Completer<bool>();
        when(() => repo.deleteComment(1)).thenAnswer((_) => response.future);
        await tester.pumpWidget(
          ProviderScope(
            overrides: [postRepositoryProvider.overrideWithValue(repo)],
            child: ScreenUtilInit(
              designSize: const Size(390, 844),
              enableScaleWH: () => false,
              enableScaleText: () => false,
              builder: (_, _) => MaterialApp(
                theme: AppTheme.light,
                home: const PageHomeFeedScreen(
                  initialIndex: 0,
                  sellerName: '測試商家',
                ),
              ),
            ),
          ),
        );
        await tester.pumpAndSettle();
        await tester.tap(find.bySemanticsLabel('留言，共 1 則'));
        await tester.pumpAndSettle();
        final more = find.byTooltip('留言更多操作');
        await tester.ensureVisible(more);
        await tester.pumpAndSettle();
        Future<void> open() async {
          await tester.tap(more);
          await tester.pumpAndSettle();
          expect(find.text('刪除留言'), findsOneWidget);
          expect(find.text('檢舉留言'), self ? findsNothing : findsOneWidget);
          expect(find.text('封鎖此人'), findsNothing);
        }

        await open();
        await tester.tapAt(const Offset(10, 100));
        await tester.pumpAndSettle();
        verifyNever(() => repo.deleteComment(any()));
        await open();
        await tester.tap(find.text('刪除留言'));
        await tester.pump();
        await tester.pump(const Duration(milliseconds: 350));
        // A second menu selection while the API is pending must not duplicate.
        await open();
        await tester.tap(find.text('刪除留言'));
        await tester.pump();
        await tester.pump(const Duration(milliseconds: 350));
        verify(() => repo.deleteComment(1)).called(1);
        response.completeError(StateError('offline'));
        await tester.pumpAndSettle();
        expect(find.text('留言內容'), findsOneWidget);
        expect(tester.takeException(), isNull);
        response = Completer<bool>();
        await open();
        await tester.tap(find.text('刪除留言'));
        await tester.pump();
        comments = [];
        response.complete(true);
        await tester.pumpAndSettle();
        verify(() => repo.deleteComment(1)).called(1);
        expect(find.text('留言內容'), findsNothing);
        await tester.pump(const Duration(seconds: 3));
        expect(tester.takeException(), isNull);
      },
    );
  }
}
