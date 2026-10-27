import 'package:hawkr_seller/features/seller/data/current_store.dart';
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

const _replyAuthor = '屏東產地直送生鮮測試商店';
const _sheetKey = ValueKey('seller-post-comments-sheet-42');
final _sheet = find.byKey(_sheetKey);
Finder get _input =>
    find.descendant(of: _sheet, matching: find.byType(TextField));

Future<_Repo> _pumpFeed(
  WidgetTester tester, {
  required Size size,
  required double scale,
}) async {
  tester.view.devicePixelRatio = 1;
  tester.view.physicalSize = size;
  addTearDown(tester.view.resetDevicePixelRatio);
  addTearDown(tester.view.resetPhysicalSize);
  final repo = _Repo();
  when(() => repo.listMyPosts()).thenAnswer(
    (_) async => [
      const Post(
        id: 42,
        sellerId: 7,
        contentType: 'TEXT',
        content: '產地直送',
        commentCount: 2,
      ),
    ],
  );
  when(() => repo.listCommentsRaw(42)).thenAnswer(
    (_) async => [
      {
        'id': 1,
        'isMine': false,
        'userName': '測試留言者',
        'content': '請問本週還能出貨嗎？',
        'createdAt': '2025-01-01T10:30:00Z',
      },
    ],
  );
  when(() => repo.listCommentReplies(1)).thenAnswer(
    (_) async => [
      {
        'id': 2,
        'parentId': 1,
        'isMine': false,
        'userName': _replyAuthor,
        'content': '可以，會依訂單順序安排出貨。',
        'createdAt': '2025-01-02T10:30:00Z',
      },
    ],
  );
  await tester.pumpWidget(
    ProviderScope(
      overrides: [
        postRepositoryProvider.overrideWithValue(repo),
        sellerStoreIdProvider.overrideWithValue('7'),
      ],
      child: ScreenUtilInit(
        designSize: const Size(414, 896),
        enableScaleWH: () => false,
        enableScaleText: () => false,
        minTextAdapt: false,
        builder: (_, _) => MaterialApp(
          theme: AppTheme.light,
          builder: (context, child) => MediaQuery(
            data: MediaQuery.of(context).copyWith(
              textScaler: TextScaler.linear(scale),
              padding: size.width > size.height
                  ? const EdgeInsets.fromLTRB(59, 0, 59, 21)
                  : const EdgeInsets.only(top: 62, bottom: 34),
            ),
            child: child!,
          ),
          home: const PageHomeFeedScreen(initialIndex: 0, sellerName: '測試商店'),
        ),
      ),
    ),
  );
  await tester.pumpAndSettle();
  return repo;
}

Future<void> _openComments(WidgetTester tester) async {
  await tester.tap(find.bySemanticsLabel(RegExp('^留言，共')));
  await tester.pumpAndSettle();
  expect(_sheet, findsOneWidget);
  expect(_input, findsOneWidget);
}

Future<void> _closeComments(WidgetTester tester) async {
  final close = find.byKey(const ValueKey('seller-post-close-comments-42'));
  await tester.ensureVisible(close);
  await tester.pumpAndSettle();
  expect(close.hitTestable(), findsOneWidget);
  await tester.tap(close);
  await tester.pumpAndSettle();
  expect(_sheet, findsNothing);
}

void main() {
  setUpAll(() {
    if (!F.isInitialized) F.init(Flavor.dev);
  });

  for (final size in [
    const Size(320, 874),
    const Size(402, 874),
    const Size(440, 874),
    const Size(874, 402),
  ]) {
    for (final scale in [1.0, 1.4]) {
      testWidgets(
        'comment sheet reply name, date, and controls fit $size/$scale',
        (tester) async {
          final semantics = tester.ensureSemantics();
          try {
            await _pumpFeed(tester, size: size, scale: scale);
            expect(
              find.text('請問本週還能出貨嗎？'),
              findsNothing,
              reason:
                  'Replies belong to the comments sheet, not the photo window.',
            );
            await _openComments(tester);
            expect(tester.takeException(), isNull);
            final target = find.bySemanticsLabel('回覆 $_replyAuthor');
            await tester.ensureVisible(target);
            await tester.pumpAndSettle();
            final name = find.text(_replyAuthor);
            expect(
              tester.getSize(name).width,
              greaterThanOrEqualTo(50),
              reason: 'Date and menu leave room to identify the author.',
            );
            expect(tester.getSize(target).height, greaterThanOrEqualTo(48));
            expect(target.hitTestable(), findsOneWidget);
            final close = find.byKey(
              const ValueKey('seller-post-close-comments-42'),
            );
            expect(close.hitTestable(), findsOneWidget);
            await tester.tap(target);
            await tester.pumpAndSettle();
            expect(find.text('@$_replyAuthor'), findsOneWidget);
            expect(
              tester.widget<TextField>(_input).focusNode!.hasFocus,
              isTrue,
            );
            final inputRect = tester.getRect(_input);
            expect(
              inputRect.left,
              greaterThanOrEqualTo(size.width > size.height ? 59 : 0),
            );
            expect(
              inputRect.right,
              lessThanOrEqualTo(
                size.width - (size.width > size.height ? 59 : 0),
              ),
            );
            expect(inputRect.bottom, lessThanOrEqualTo(size.height));
            expect(tester.takeException(), isNull);
          } finally {
            semantics.dispose();
          }
        },
      );
    }
  }

  for (final size in [
    const Size(320, 874),
    const Size(402, 874),
    const Size(874, 402),
  ]) {
    testWidgets('sheet reopen preserves draft/reply and guarded retry at $size', (
      tester,
    ) async {
      final repo = await _pumpFeed(tester, size: size, scale: 1.4);
      final mediaBefore = tester.getRect(
        find.byKey(const ValueKey('seller-post-media-42')),
      );
      await _openComments(tester);
      await tester.enterText(_input, '保留這份草稿');
      await tester.pump();
      final decorator = tester.widget<InputDecorator>(
        find.descendant(of: _input, matching: find.byType(InputDecorator)),
      );
      expect(decorator.isFocused, isTrue);
      expect(decorator.decoration.filled, isFalse);
      expect(decorator.decoration.enabledBorder, InputBorder.none);
      expect(decorator.decoration.focusedBorder, InputBorder.none);
      await _closeComments(tester);
      await _openComments(tester);
      expect(tester.widget<TextField>(_input).controller!.text, '保留這份草稿');
      expect(tester.widget<TextField>(_input).focusNode!.hasFocus, isFalse);
      final reply = find.bySemanticsLabel('回覆 $_replyAuthor');
      await tester.ensureVisible(reply);
      await tester.tap(reply);
      await tester.pumpAndSettle();
      expect(tester.widget<TextField>(_input).focusNode!.hasFocus, isTrue);
      final keyboardHeight = size.width > size.height ? 180.0 : 300.0;
      tester.view.viewInsets = FakeViewPadding(bottom: keyboardHeight);
      addTearDown(tester.view.resetViewInsets);
      await tester.pumpAndSettle();
      expect(
        tester.getRect(_input).bottom,
        lessThanOrEqualTo(size.height - keyboardHeight),
      );
      expect(
        tester.getRect(find.byKey(const ValueKey('seller-post-media-42'))),
        mediaBefore,
      );
      expect(tester.takeException(), isNull);
      await _closeComments(tester);
      tester.view.resetViewInsets();
      await tester.pumpAndSettle();
      await _openComments(tester);
      expect(find.text('@$_replyAuthor'), findsOneWidget);
      expect(tester.widget<TextField>(_input).controller!.text, '保留這份草稿');
      expect(tester.widget<TextField>(_input).focusNode!.hasFocus, isTrue);

      var response = Completer<Map<String, dynamic>?>();
      when(
        () => repo.replyToComment(
          1,
          postId: 42,
          content: '@$_replyAuthor 保留這份草稿',
        ),
      ).thenAnswer((_) => response.future);
      // Two platform send events before the disabled field rebuilds still create one request.
      await tester.testTextInput.receiveAction(TextInputAction.send);
      await tester.testTextInput.receiveAction(TextInputAction.send);
      await tester.pump();
      verify(
        () => repo.replyToComment(
          1,
          postId: 42,
          content: '@$_replyAuthor 保留這份草稿',
        ),
      ).called(1);
      expect(tester.widget<TextField>(_input).enabled, isFalse);
      response.completeError(StateError('offline'));
      await tester.pumpAndSettle();
      expect(tester.widget<TextField>(_input).controller!.text, '保留這份草稿');
      expect(tester.widget<TextField>(_input).enabled, isTrue);
      expect(find.text('@$_replyAuthor'), findsOneWidget);
      expect(tester.takeException(), isNull);

      response = Completer<Map<String, dynamic>?>();
      await tester.tap(_input);
      await tester.testTextInput.receiveAction(TextInputAction.send);
      await tester.pump();
      response.complete({'id': 3, 'parentId': 1});
      await tester.pumpAndSettle();
      verify(
        () => repo.replyToComment(
          1,
          postId: 42,
          content: '@$_replyAuthor 保留這份草稿',
        ),
      ).called(1);
      verifyNever(
        () => repo.replyToComment(
          2,
          postId: any(named: 'postId'),
          content: any(named: 'content'),
        ),
      );
      verifyNever(
        () => repo.commentOnPostRaw(any(), content: any(named: 'content')),
      );
      expect(tester.widget<TextField>(_input).controller!.text, isEmpty);
      expect(find.text('@$_replyAuthor'), findsNothing);
      final actions = find.byKey(const ValueKey('seller-post-action-bar-42'));
      expect(
        find.descendant(of: actions, matching: find.text('3')),
        findsOneWidget,
      );

      // Keep the same modal open for a second successful submission. Its
      // callbacks must increment the refreshed count, not the opening value.
      when(
        () => repo.commentOnPostRaw(42, content: '第二則獨立留言'),
      ).thenAnswer((_) async => {'id': 4});
      await tester.enterText(_input, '第二則獨立留言');
      await tester.pump();
      final send = find.bySemanticsLabel('送出留言');
      await tester.ensureVisible(send);
      expect(tester.getSize(send).height, greaterThanOrEqualTo(48));
      await tester.tap(send);
      await tester.pumpAndSettle();
      verify(() => repo.commentOnPostRaw(42, content: '第二則獨立留言')).called(1);
      expect(tester.widget<TextField>(_input).controller!.text, isEmpty);
      expect(
        find.descendant(of: actions, matching: find.text('4')),
        findsOneWidget,
      );
      await _closeComments(tester);
      expect(find.bySemanticsLabel('留言，共 4 則'), findsOneWidget);
      await tester.pump(const Duration(seconds: 3));
      expect(tester.takeException(), isNull);
    });
  }
}
