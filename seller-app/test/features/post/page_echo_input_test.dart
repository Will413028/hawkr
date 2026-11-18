import 'package:hawkr_seller/core/session/my_store.dart';
import 'package:hawkr_seller/features/seller/data/current_store.dart';
import 'package:hawkr_seller/shared/widgets/seller_avatar_widget.dart';
import 'dart:async';
import 'dart:io';
import 'dart:ui' as ui;

import 'package:hawkr_api/gen/hawkr/social/v1alpha1/social.pbenum.dart';
import 'package:hawkr_seller/config/flavor.dart';
import 'package:hawkr_seller/features/report/data/store_block_rules.dart';
import 'package:hawkr_seller/core/theme/app_theme.dart';
import 'package:hawkr_seller/features/post/data/post_comment.dart';
import 'package:hawkr_seller/features/post/data/post_repository.dart';
import 'package:hawkr_seller/features/post/presentation/widgets/page_echo_tab.dart';
import 'package:hawkr_seller/features/seller/data/seller_repository.dart';
import 'package:hawkr_seller/shared/icons/app_icons.dart';
import 'package:hawkr_seller/shared/widgets/app_bottom_navigation_bar.dart';
import 'package:hawkr_seller/shared/widgets/hk_frosted_icon_button.dart';
import 'package:hawkr_seller/shared/widgets/seller_floating_navigation_layout.dart';
import 'package:dio/dio.dart';
import 'package:flutter/material.dart';
import 'package:flutter/rendering.dart';
import 'package:flutter/services.dart';
import 'package:flutter_screenutil/flutter_screenutil.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:hooks_riverpod/hooks_riverpod.dart';
import 'package:mocktail/mocktail.dart';
import 'package:shared_preferences/shared_preferences.dart';

import '../../support/my_store_fixtures.dart';
import 'post_comment_fixtures.dart';

class _MockPostRepository extends Mock implements PostRepository {}

class _RecordingAdapter implements HttpClientAdapter {
  final requests = <String>[];

  @override
  Future<ResponseBody> fetch(
    RequestOptions options,
    Stream<Uint8List>? requestStream,
    Future<void>? cancelFuture,
  ) async {
    requests.add('${options.method} ${options.path}');
    return ResponseBody.fromString('{"code":0,"data":{}}', 200);
  }

  @override
  void close({bool force = false}) {}
}

Finder _iconButton(String tooltip) => find.byWidgetPredicate(
  (widget) => widget is IconButton && widget.tooltip == tooltip,
);

const _logoTemplate = 'https://img.example.test/logo/{preset}.jpg';
late _RecordingAdapter _lastAdapter;

class _StoreHolder extends Notifier<MyStore?> {
  @override
  MyStore? build() => null;

  void set(MyStore? store) => state = store;
}

final _lateStore = NotifierProvider<_StoreHolder, MyStore?>(_StoreHolder.new);

const _captureKey = ValueKey('echo-input-capture');

Future<_MockPostRepository> _pumpEcho(
  WidgetTester tester, {
  Size size = const Size(390, 844),
  bool nested = false,
  bool ownReply = false,
  bool lateStore = false,
  double textScale = 1,
  StoreForumEntry? message,
  List<StoreForumEntry>? replies,
  bool expectRootText = true,
  bool canBlock = true,
}) async {
  tester.view.devicePixelRatio = 1;
  tester.view.physicalSize = size;
  tester.view.viewPadding = const FakeViewPadding(bottom: 34);
  tester.view.padding = const FakeViewPadding(bottom: 34);
  addTearDown(tester.view.resetDevicePixelRatio);
  addTearDown(tester.view.resetPhysicalSize);
  addTearDown(tester.view.resetViewInsets);
  addTearDown(tester.view.resetViewPadding);
  addTearDown(tester.view.resetPadding);

  final repository = _MockPostRepository();
  final adapter = _RecordingAdapter();
  final ownReplies =
      replies ??
      [
        if (ownReply)
          testForumEntry(
            id: '99',
            parentId: '42',
            isMine: true,
            author: '',
            body: '原本的回覆',
            canEdit: true,
            canDelete: true,
          ),
      ];
  final root =
      message ??
      testForumEntry(
        id: '42',
        author: '小明',
        body: '請問何時出貨?',
        replyCount: ownReplies.length,
      );
  when(() => repository.listForumEntries('7')).thenAnswer((_) async => [root]);
  when(
    () => repository.listForumEntries('7', parentId: '42'),
  ).thenAnswer((_) async => ownReplies);
  _lastAdapter = adapter;
  SharedPreferences.setMockInitialValues({});
  await tester.pumpWidget(
    ProviderScope(
      overrides: [
        postRepositoryProvider.overrideWithValue(repository),
        canBlockBuyersProvider.overrideWithValue(canBlock),
        if (ownReply && lateStore)
          myStoreSnapshotProvider.overrideWith((ref) => ref.watch(_lateStore))
        else if (ownReply)
          myStoreSnapshotProvider.overrideWithValue(
            MyStore(myStoreResponse(id: '7', logoTemplate: _logoTemplate)),
          ),
        sellerRepositoryProvider.overrideWith(
          (ref) => SellerRepository(Dio()..httpClientAdapter = adapter, ref),
        ),
      ],
      child: ScreenUtilInit(
        designSize: const Size(390, 844),
        enableScaleWH: () => false,
        enableScaleText: () => false,
        builder: (_, _) => MaterialApp(
          theme: AppTheme.light,
          builder: (context, child) => MediaQuery(
            data: MediaQuery.of(
              context,
            ).copyWith(textScaler: TextScaler.linear(textScale)),
            child: child!,
          ),
          home: RepaintBoundary(
            key: _captureKey,
            child: Scaffold(
              body: SellerFloatingNavigationLayout(
                page: _EchoPage(nested: nested),
                navigation: AppBottomNavigationBar(
                  currentIndex: NavIndex.category,
                  onTap: (_) {},
                ),
              ),
            ),
          ),
        ),
      ),
    ),
  );
  await tester.pumpAndSettle();
  // The production navigation has a 900 ms route-settling timer.
  await tester.pump(const Duration(seconds: 1));
  if (expectRootText) expect(find.text(root.body), findsOneWidget);
  return repository;
}

class _EchoPage extends StatelessWidget {
  const _EchoPage({required this.nested});

  final bool nested;

  @override
  Widget build(BuildContext context) {
    if (!nested) {
      return const Scaffold(
        body: PageEchoTab(sellerId: '7', sellerDisplayName: '測試商家'),
      );
    }
    return DefaultTabController(
      length: 1,
      child: Scaffold(
        body: NestedScrollView(
          headerSliverBuilder: (context, innerBoxIsScrolled) => [
            SliverOverlapAbsorber(
              handle: NestedScrollView.sliverOverlapAbsorberHandleFor(context),
              sliver: const SliverAppBar(
                pinned: true,
                title: Text('測試商家'),
                bottom: TabBar(tabs: [Tab(text: '留言板')]),
              ),
            ),
          ],
          body: TabBarView(
            children: [
              Builder(
                builder: (context) => PageEchoTab(
                  sellerId: '7',
                  sellerDisplayName: '測試商家',
                  outerHandle: NestedScrollView.sliverOverlapAbsorberHandleFor(
                    context,
                  ),
                ),
              ),
            ],
          ),
        ),
      ),
    );
  }
}

// Optional local evidence; no golden files or machine paths enter the repo.
Future<void> _capture(WidgetTester tester, String state) async {
  final prefix = Platform.environment['HK_ECHO_CAPTURE'];
  if (prefix == null) return;
  await tester.runAsync(() async {
    final boundary = tester.renderObject<RenderRepaintBoundary>(
      find.byKey(_captureKey),
    );
    final image = await boundary.toImage(pixelRatio: 1);
    try {
      final data = await image.toByteData(format: ui.ImageByteFormat.png);
      await File('$prefix-$state.png').writeAsBytes(data!.buffer.asUint8List());
    } finally {
      image.dispose();
    }
  });
}

void main() {
  setUpAll(() async {
    if (!F.isInitialized) F.init(Flavor.dev);
    // Opt-in local capture can load the device's CJK font instead of Ahem.
    final fontPath = Platform.environment['HK_ECHO_FONT'];
    if (fontPath != null) {
      final bytes = ByteData.sublistView(await File(fontPath).readAsBytes());
      for (final family in ['Roboto', 'Ahem']) {
        await (FontLoader(family)..addFont(Future.value(bytes))).load();
      }
      await (FontLoader(
        'MaterialIcons',
      )..addFont(rootBundle.load('fonts/MaterialIcons-Regular.otf'))).load();
    }
  });

  testWidgets('reply has one empty composer without an automatic mention', (
    tester,
  ) async {
    await _pumpEcho(tester);
    await _capture(tester, 'idle');
    await tester.tap(find.text('回覆'));
    await tester.pumpAndSettle();
    await _capture(tester, 'reply');

    expect(find.byType(TextField), findsOneWidget);
    final field = tester.widget<TextField>(find.byType(TextField));
    expect(field.controller!.text, isEmpty);
    expect(field.readOnly, isFalse);
    expect(field.minLines, 1);
    expect(field.maxLines, 4);
    expect(field.keyboardType, TextInputType.multiline);
    expect(field.decoration!.hintText, '回覆 小明...');

    await tester.enterText(find.byType(TextField), '明天出貨，謝謝您');
    await tester.pumpAndSettle();
    expect(
      find.descendant(
        of: _iconButton('送出留言'),
        matching: find.byIcon(AppIcons.send),
      ),
      findsOneWidget,
    );
    expect(tester.widget<IconButton>(_iconButton('送出留言')).onPressed, isNotNull);
    await _capture(tester, 'ready');
    await tester.tap(find.text('請問何時出貨?'));
    await tester.pump();
    expect(field.focusNode!.hasFocus, isFalse);
  });

  testWidgets(
    'idle composer is read only and excludes inherited field chrome',
    (tester) async {
      await _pumpEcho(tester);
      final decorator = tester.widget<InputDecorator>(
        find.byType(InputDecorator),
      );
      final decoration = decorator.decoration;
      expect(decoration.filled, isFalse);
      for (final border in [
        decoration.border,
        decoration.enabledBorder,
        decoration.focusedBorder,
        decoration.disabledBorder,
        decoration.errorBorder,
        decoration.focusedErrorBorder,
      ]) {
        expect(border, InputBorder.none);
      }
      final field = tester.widget<TextField>(find.byType(TextField));
      expect(field.readOnly, isTrue);
      expect(field.decoration!.hintText, '請選擇留言回覆');
      expect(tester.widget<IconButton>(_iconButton('送出留言')).onPressed, isNull);
      await tester.tap(find.byType(TextField));
      await tester.pump();
      expect(tester.testTextInput.isVisible, isFalse);
    },
  );

  for (final failure in [
    StateError('network unavailable'),
    StateError('aborted'),
  ]) {
    testWidgets('${failure.message} keeps draft and target for one retry', (
      tester,
    ) async {
      final semantics = tester.ensureSemantics();
      try {
        final repository = await _pumpEcho(tester);
        var response = Completer<StoreForumEntry>();
        when(
          () => repository.replyForumEntry(
            '7',
            parentId: '42',
            body: '明天出貨\n謝謝您',
          ),
        ).thenAnswer((_) => response.future);

        await tester.tap(find.text('回覆'));
        await tester.pumpAndSettle();
        final fieldFinder = find.byType(TextField);
        await tester.enterText(fieldFinder, ' 明天出貨\n謝謝您 ');
        await tester.pump();

        // The send control belongs to the field's tap region, so pointer-down
        // must not close the keyboard and move the button before pointer-up.
        final sendGesture = await tester.startGesture(
          tester.getCenter(find.byTooltip('送出留言')),
        );
        await tester.pump();
        expect(
          tester.widget<TextField>(fieldFinder).focusNode!.hasFocus,
          isTrue,
        );
        await sendGesture.up();
        // Two rapid taps and an IME action can arrive before the next frame.
        await tester.tap(find.byTooltip('送出留言'));
        await tester.testTextInput.receiveAction(TextInputAction.send);
        await tester.pump();
        verify(
          () => repository.replyForumEntry(
            '7',
            parentId: '42',
            body: '明天出貨\n謝謝您',
          ),
        ).called(1);
        expect(find.bySemanticsLabel('留言傳送中'), findsOneWidget);
        expect(tester.widget<TextField>(fieldFinder).readOnly, isTrue);
        expect(
          tester.widget<IconButton>(_iconButton('送出留言')).onPressed,
          isNull,
        );

        response.completeError(failure);
        await tester.pump();
        await tester.pump(const Duration(milliseconds: 350));
        final failedField = tester.widget<TextField>(fieldFinder);
        expect(failedField.controller!.text, ' 明天出貨\n謝謝您 ');
        expect(failedField.readOnly, isFalse);
        expect(failedField.decoration!.hintText, '回覆 小明...');
        expect(find.byTooltip('取消回覆'), findsOneWidget);

        response = Completer<StoreForumEntry>();
        await tester.tap(find.byTooltip('送出留言'));
        await tester.pump();
        verify(
          () => repository.replyForumEntry(
            '7',
            parentId: '42',
            body: '明天出貨\n謝謝您',
          ),
        ).called(1);
        response.complete(testForumEntry(id: '50', parentId: '42'));
        await tester.pumpAndSettle();
        final sentField = tester.widget<TextField>(fieldFinder);
        expect(sentField.controller!.text, isEmpty);
        expect(sentField.readOnly, isTrue);
        expect(sentField.decoration!.hintText, '請選擇留言回覆');
        expect(find.byTooltip('取消回覆'), findsNothing);
        // Let the failure toast's removal timer finish before unmounting.
        await tester.pump(const Duration(seconds: 3));
        expect(tester.takeException(), isNull);
      } finally {
        semantics.dispose();
      }
    });
  }

  testWidgets('cancelling a reply clears its draft and restores idle state', (
    tester,
  ) async {
    final repository = await _pumpEcho(tester);
    await tester.tap(find.text('回覆'));
    await tester.pumpAndSettle();
    await tester.enterText(find.byType(TextField), '尚未送出的草稿');
    await tester.pump();
    await tester.tap(find.byTooltip('取消回覆'));
    await tester.pumpAndSettle();
    final field = tester.widget<TextField>(find.byType(TextField));
    expect(field.controller!.text, isEmpty);
    expect(field.readOnly, isTrue);
    expect(field.focusNode!.hasFocus, isFalse);
    expect(field.decoration!.hintText, '請選擇留言回覆');
    verifyNever(
      () => repository.replyForumEntry(
        any(),
        parentId: any(named: 'parentId'),
        body: any(named: 'body'),
      ),
    );
  });

  testWidgets(
    'editing updates only the selected reply through the same composer',
    (tester) async {
      final repository = await _pumpEcho(
        tester,
        ownReply: true,
        size: const Size(320, 568),
        textScale: 1.4,
      );
      var response = Completer<StoreForumEntry>();
      when(
        () =>
            repository.updateForumEntry('99', etag: 'etag-99', body: '改為後天出貨'),
      ).thenAnswer((_) => response.future);
      Future<void> openEdit() async {
        await tester.tap(find.byIcon(AppIcons.moreHoriz).last);
        await tester.pumpAndSettle();
        await tester.tap(find.text('編輯留言'));
        await tester.pumpAndSettle();
      }

      await openEdit();
      expect(find.byType(TextField), findsOneWidget);
      expect(
        tester.widget<TextField>(find.byType(TextField)).controller!.text,
        '原本的回覆',
      );
      await tester.enterText(find.byType(TextField), '取消的編輯');
      await tester.tap(find.byTooltip('取消編輯'));
      await tester.pumpAndSettle();
      expect(
        tester.widget<TextField>(find.byType(TextField)).controller!.text,
        isEmpty,
      );

      await openEdit();
      await tester.enterText(find.byType(TextField), '改為後天出貨');
      await tester.pump();
      await tester.tap(find.byTooltip('儲存留言'));
      await tester.tap(find.byTooltip('儲存留言'));
      await tester.testTextInput.receiveAction(TextInputAction.send);
      await tester.pump();
      verify(
        () =>
            repository.updateForumEntry('99', etag: 'etag-99', body: '改為後天出貨'),
      ).called(1);
      expect(tester.widget<TextField>(find.byType(TextField)).readOnly, isTrue);
      expect(tester.widget<IconButton>(_iconButton('取消編輯')).onPressed, isNull);
      response.completeError(StateError('stale'));
      await tester.pump();
      expect(
        tester.widget<TextField>(find.byType(TextField)).controller!.text,
        '改為後天出貨',
      );
      expect(find.byTooltip('取消編輯'), findsOneWidget);

      response = Completer<StoreForumEntry>();
      await tester.tap(find.byTooltip('儲存留言'));
      await tester.pump();
      response.complete(testForumEntry(id: '99', isMine: true));
      await tester.pumpAndSettle();
      final field = tester.widget<TextField>(find.byType(TextField));
      expect(field.controller!.text, isEmpty);
      expect(field.readOnly, isTrue);
      expect(field.decoration!.hintText, '請選擇留言回覆');
      verify(
        () =>
            repository.updateForumEntry('99', etag: 'etag-99', body: '改為後天出貨'),
      ).called(1);
      verifyNever(
        () => repository.replyForumEntry(
          any(),
          parentId: any(named: 'parentId'),
          body: any(named: 'body'),
        ),
      );
      await tester.pump(const Duration(seconds: 3));
      expect(tester.takeException(), isNull);
    },
  );

  testWidgets('comment authors are never resolved through a profile request', (
    tester,
  ) async {
    await _pumpEcho(
      tester,
      message: testForumEntry(
        id: '42',
        author: '小明',
        body: '請問何時出貨?',
        replyCount: 2,
      ),
      replies: [
        testForumEntry(
          id: '99',
          parentId: '42',
          isMine: true,
          author: '店家',
          body: '原本的回覆',
          canEdit: true,
          canDelete: true,
        ),
        testForumEntry(id: '100', parentId: '42', author: '小華', body: '買家追問'),
      ],
    );

    expect(_lastAdapter.requests, isEmpty);
    expect(find.text('小明'), findsOneWidget);
    expect(find.text('小華'), findsOneWidget);
    expect(find.text('用戶31'), findsNothing);
  });

  testWidgets('own reply uses the store snapshot and never loads a profile', (
    tester,
  ) async {
    await _pumpEcho(tester, ownReply: true);

    expect(_lastAdapter.requests, isEmpty);
    final prefs = await SharedPreferences.getInstance();
    expect(prefs.getKeys(), isEmpty);
    expect(
      find.byWidgetPredicate(
        (w) =>
            w is SellerAvatarWidget &&
            w.imageUrl == 'https://img.example.test/logo/avatar.jpg',
      ),
      findsOneWidget,
    );
  });

  testWidgets('own reply avatar appears when the store snapshot arrives late', (
    tester,
  ) async {
    await _pumpEcho(tester, ownReply: true, lateStore: true);
    final avatar = find.byWidgetPredicate(
      (w) =>
          w is SellerAvatarWidget &&
          w.imageUrl == 'https://img.example.test/logo/avatar.jpg',
    );
    expect(avatar, findsNothing);

    ProviderScope.containerOf(tester.element(find.byType(PageEchoTab)))
        .read(_lateStore.notifier)
        .set(MyStore(myStoreResponse(id: '7', logoTemplate: _logoTemplate)));
    await tester.pumpAndSettle();

    expect(avatar, findsOneWidget);
  });

  testWidgets('switching edit to reply sends only the selected parent', (
    tester,
  ) async {
    final repository = await _pumpEcho(tester, ownReply: true);
    when(
      () => repository.replyForumEntry('7', parentId: '42', body: '改成回覆'),
    ).thenAnswer((_) async => testForumEntry(id: '51', parentId: '42'));
    await tester.tap(find.byTooltip('留言更多操作').last);
    await tester.pumpAndSettle();
    await tester.tap(find.text('編輯留言'));
    await tester.pumpAndSettle();
    await tester.tap(find.text('回覆').first);
    await tester.pumpAndSettle();
    final field = tester.widget<TextField>(find.byType(TextField));
    expect(field.controller!.text, isEmpty);
    expect(find.byTooltip('取消編輯'), findsNothing);
    await tester.enterText(find.byType(TextField), '改成回覆');
    await tester.pump();
    await tester.tap(find.byTooltip('送出留言'));
    await tester.pumpAndSettle();
    verify(
      () => repository.replyForumEntry('7', parentId: '42', body: '改成回覆'),
    ).called(1);
    verifyNever(
      () => repository.updateForumEntry(
        any(),
        etag: any(named: 'etag'),
        body: any(named: 'body'),
      ),
    );
  });

  testWidgets(
    'own delete fails safely then retry clears the removed edit target',
    (tester) async {
      final repository = await _pumpEcho(tester, ownReply: true);
      var response = Completer<void>();
      when(
        () => repository.deleteForumEntry('99', etag: 'etag-99'),
      ).thenAnswer((_) => response.future);
      await tester.tap(find.byTooltip('留言更多操作').last);
      await tester.pumpAndSettle();
      await tester.tap(find.text('編輯留言'));
      await tester.pumpAndSettle();
      Future<void> delete() async {
        await tester.tap(find.byTooltip('留言更多操作').last);
        await tester.pumpAndSettle();
        await tester.tap(find.text('刪除留言'));
        await tester.pumpAndSettle();
        await tester.tap(find.text('刪除'));
        await tester.pump();
        await tester.pump(const Duration(milliseconds: 350));
      }

      await delete();
      await tester.tap(find.byTooltip('留言更多操作').last);
      await tester.pump();
      verify(
        () => repository.deleteForumEntry('99', etag: 'etag-99'),
      ).called(1);
      expect(find.text('刪除留言'), findsNothing);
      response.completeError(StateError('offline'));
      await tester.pumpAndSettle();
      expect(find.byTooltip('取消編輯'), findsOneWidget);
      expect(tester.takeException(), isNull);
      response = Completer<void>();
      await delete();
      when(
        () => repository.listForumEntries('7', parentId: '42'),
      ).thenAnswer((_) async => []);
      response.complete();
      await tester.pumpAndSettle();
      expect(find.text('原本的回覆'), findsNothing);
      expect(find.byTooltip('取消編輯'), findsNothing);
      expect(
        tester.widget<TextField>(find.byType(TextField)).controller!.text,
        isEmpty,
      );
      verify(
        () => repository.deleteForumEntry('99', etag: 'etag-99'),
      ).called(1);
      await tester.pump(const Duration(seconds: 3));
      expect(tester.takeException(), isNull);
    },
  );

  testWidgets('other comment offers report without block or owner actions', (
    tester,
  ) async {
    final repository = await _pumpEcho(tester);
    await tester.tap(find.byTooltip('留言更多操作'));
    await tester.pumpAndSettle();
    expect(find.text('檢舉留言'), findsOneWidget);
    expect(find.text('封鎖此人'), findsNothing);
    expect(find.text('編輯留言'), findsNothing);
    expect(find.text('刪除留言'), findsNothing);
    await tester.tap(find.text('檢舉留言'));
    await tester.pumpAndSettle();
    expect(find.text('送出檢舉'), findsOneWidget);
    await tester.tapAt(const Offset(10, 30));
    await tester.pumpAndSettle();
    verifyNever(
      () => repository.deleteForumEntry(any(), etag: any(named: 'etag')),
    );
    expect(tester.takeException(), isNull);
  });

  testWidgets('forum shows no like control', (tester) async {
    await _pumpEcho(tester, ownReply: true);
    expect(find.byIcon(AppIcons.favorite), findsNothing);
    expect(find.byIcon(AppIcons.favoriteBorder), findsNothing);
    expect(find.textContaining('喜歡'), findsNothing);
    expect(find.byTooltip('喜歡'), findsNothing);
  });

  testWidgets('reply, edit and delete follow viewer_can_* (false case)', (
    tester,
  ) async {
    await _pumpEcho(
      tester,
      message: testForumEntry(
        id: '42',
        author: '小明',
        body: '請問何時出貨?',
        canReply: false,
        replyCount: 1,
      ),
      replies: [
        testForumEntry(
          id: '99',
          parentId: '42',
          isMine: true,
          author: '店家',
          body: '原本的回覆',
          canReply: false,
        ),
      ],
    );
    expect(find.text('回覆'), findsNothing);
    // 自己的回覆沒有 viewer_can_edit／viewer_can_delete：沒有編輯與刪除入口。
    // 買家的頂層留言仍有檢舉入口，所以只剩一顆更多按鈕。
    expect(find.byTooltip('留言更多操作'), findsOneWidget);
    await tester.tap(find.byTooltip('留言更多操作'));
    await tester.pumpAndSettle();
    expect(find.text('編輯留言'), findsNothing);
    expect(find.text('刪除留言'), findsNothing);
    expect(find.text('檢舉留言'), findsOneWidget);
  });

  testWidgets('other entry offers delete only when viewer_can_delete', (
    tester,
  ) async {
    await _pumpEcho(
      tester,
      message: testForumEntry(
        id: '42',
        author: '小明',
        body: '請問何時出貨?',
        canDelete: true,
      ),
    );
    await tester.tap(find.byTooltip('留言更多操作'));
    await tester.pumpAndSettle();
    expect(find.text('刪除留言'), findsOneWidget);
    expect(find.text('檢舉留言'), findsOneWidget);
  });

  testWidgets('deleting a forum entry asks first and cancel sends nothing', (
    tester,
  ) async {
    final repository = await _pumpEcho(tester, ownReply: true);
    await tester.tap(find.byTooltip('留言更多操作').last);
    await tester.pumpAndSettle();
    await tester.tap(find.text('刪除留言'));
    await tester.pumpAndSettle();
    expect(find.text('確定要刪除此留言嗎？'), findsOneWidget);
    await tester.tap(find.text('取消'));
    await tester.pumpAndSettle();
    verifyNever(
      () => repository.deleteForumEntry(any(), etag: any(named: 'etag')),
    );
  });

  testWidgets(
    'a submit that starts while the delete dialog is open blocks it',
    (tester) async {
      final repository = await _pumpEcho(tester, ownReply: true);
      final update = Completer<StoreForumEntry>();
      when(
        () => repository.updateForumEntry('99', etag: 'etag-99', body: '改好了'),
      ).thenAnswer((_) => update.future);
      await tester.tap(find.byIcon(AppIcons.moreHoriz).last);
      await tester.pumpAndSettle();
      await tester.tap(find.text('編輯留言'));
      await tester.pumpAndSettle();
      await tester.enterText(find.byType(TextField), '改好了');
      await tester.pump();
      await tester.tap(find.byTooltip('留言更多操作').last);
      await tester.pumpAndSettle();
      await tester.tap(find.text('刪除留言'));
      await tester.pumpAndSettle();
      expect(find.text('確定要刪除此留言嗎？'), findsOneWidget);
      // 平台的送出事件不經過被對話框蓋住的按鈕。
      await tester.showKeyboard(find.byType(TextField));
      await tester.testTextInput.receiveAction(TextInputAction.send);
      await tester.pump();
      verify(
        () => repository.updateForumEntry('99', etag: 'etag-99', body: '改好了'),
      ).called(1);
      await tester.tap(find.text('刪除'));
      await tester.pump();
      await tester.pump(const Duration(milliseconds: 350));
      verifyNever(
        () => repository.deleteForumEntry(any(), etag: any(named: 'etag')),
      );
      update.complete(testForumEntry(id: '99', isMine: true));
      await tester.pumpAndSettle();
      await tester.pump(const Duration(seconds: 3));
    },
  );

  testWidgets('deleted and hidden placeholders show approved text only', (
    tester,
  ) async {
    await _pumpEcho(
      tester,
      expectRootText: false,
      message: testForumEntry(
        id: '42',
        state: CommentState.COMMENT_STATE_DELETED,
        replyCount: 1,
      ),
      replies: [testForumEntry(id: '99', parentId: '42', body: '還在的回覆')],
    );
    expect(find.text('此留言已刪除'), findsOneWidget);
    expect(find.text('還在的回覆'), findsOneWidget);
  });

  for (final fail in [false, true]) {
    testWidgets(
      'pending echo ${fail ? 'failure' : 'success'} after leaving is safe',
      (tester) async {
        final repository = await _pumpEcho(tester);
        final response = Completer<StoreForumEntry>();
        when(
          () => repository.replyForumEntry('7', parentId: '42', body: '離頁'),
        ).thenAnswer((_) => response.future);
        await tester.tap(find.text('回覆'));
        await tester.pumpAndSettle();
        await tester.enterText(find.byType(TextField), '離頁');
        await tester.pump();
        await tester.tap(find.byTooltip('送出留言'));
        await tester.pumpWidget(const SizedBox.shrink());
        if (fail) {
          response.completeError(StateError('offline'));
        } else {
          response.complete(testForumEntry(id: '52', parentId: '42'));
        }
        await tester.pumpAndSettle();
        expect(tester.takeException(), isNull);
      },
    );
  }

  for (final width in [320.0, 402.0, 440.0]) {
    for (final scale in [1.0, 1.4]) {
      testWidgets('echo metadata and actions fit $width/$scale', (
        tester,
      ) async {
        await _pumpEcho(
          tester,
          size: Size(width, 874),
          textScale: scale,
          message: testForumEntry(
            id: '42',
            author: '需要完整閱讀的長名稱買家',
            body: '請問何時出貨?',
            replyCount: 2,
            createTime: DateTime(2024, 1, 1, 10),
          ),
          replies: [
            for (final id in ['98', '99'])
              testForumEntry(
                id: id,
                parentId: '42',
                author: '長名稱回覆使用者$id',
                body: '留言回覆內容$id',
                createTime: DateTime(2024, 1, 1, 10),
              ),
          ],
        );
        for (final text in tester.widgetList<Text>(find.text('2024/01/01'))) {
          final paragraph = tester.renderObject<RenderParagraph>(
            find.byWidget(text),
          );
          expect(paragraph.didExceedMaxLines, isFalse);
          expect(
            tester.getRect(find.byWidget(text)).right,
            lessThanOrEqualTo(width - 16),
          );
        }
        for (final action in tester.widgetList<HkFrostedIconButton>(
          find.byType(HkFrostedIconButton),
        )) {
          expect(tester.getSize(find.byWidget(action)), const Size.square(48));
        }
        final reply = find
            .ancestor(
              of: find.text('回覆').first,
              matching: find.byType(GestureDetector),
            )
            .first;
        expect(tester.getSize(reply).height, greaterThanOrEqualTo(48));
        expect(tester.takeException(), isNull);
      });
    }
  }

  for (final size in [
    const Size(320, 568),
    const Size(402, 874),
    const Size(440, 956),
  ]) {
    for (final textScale in [1.0, 1.4]) {
      testWidgets(
        '${size.width.toInt()}px/$textScale composer stays above navigation and keyboard',
        (tester) async {
          await _pumpEcho(
            tester,
            size: size,
            nested: true,
            textScale: textScale,
          );
          await tester.tap(find.text('回覆'));
          await tester.pumpAndSettle();
          await tester.enterText(
            find.byType(TextField),
            '第一行\n第二行\n第三行\n第四行\n第五行',
          );
          await tester.pumpAndSettle();
          final fieldFinder = find.byType(TextField);
          final originalField = tester.widget<TextField>(fieldFinder);
          final sendFinder = find.byTooltip('送出留言');
          final navFinder = find.byKey(
            const ValueKey('seller-bottom-nav-surface'),
          );
          expect(
            tester.getRect(fieldFinder).bottom,
            lessThan(tester.getRect(navFinder).top),
          );
          expect(
            tester.getRect(sendFinder).bottom,
            lessThan(tester.getRect(navFinder).top),
          );

          tester.view.viewInsets = const FakeViewPadding(bottom: 300);
          tester.view.padding = FakeViewPadding.zero;
          await tester.pumpAndSettle();
          await _capture(tester, 'keyboard-${size.width.toInt()}-$textScale');
          expect(find.byType(TextField), findsOneWidget);
          final keyboardField = tester.widget<TextField>(fieldFinder);
          expect(keyboardField.controller, same(originalField.controller));
          expect(keyboardField.focusNode, same(originalField.focusNode));
          expect(keyboardField.focusNode!.hasFocus, isTrue);
          expect(keyboardField.controller!.text, '第一行\n第二行\n第三行\n第四行\n第五行');
          if (size.width == 320) expect(keyboardField.maxLines, lessThan(4));
          final navRect = tester.getRect(navFinder);
          expect(navRect.bottom, lessThanOrEqualTo(size.height - 300));
          expect(tester.getRect(fieldFinder).bottom, lessThan(navRect.top));
          expect(tester.getRect(sendFinder).bottom, lessThan(navRect.top));
          expect(tester.getRect(fieldFinder).left, greaterThanOrEqualTo(0));
          expect(
            tester.getRect(sendFinder).right,
            lessThanOrEqualTo(size.width),
          );
          expect(tester.takeException(), isNull);
          tester.view.viewInsets = FakeViewPadding.zero;
          tester.view.padding = const FakeViewPadding(bottom: 34);
          await tester.pumpAndSettle();
          final restoredField = tester.widget<TextField>(fieldFinder);
          expect(restoredField.maxLines, 4);
          expect(restoredField.controller, same(originalField.controller));
          expect(restoredField.controller!.text, '第一行\n第二行\n第三行\n第四行\n第五行');
          expect(tester.takeException(), isNull);
        },
      );
    }
  }

  for (final c in [
    (
      name: 'buyer author deletable',
      buyer: true,
      del: true,
      mine: false,
      show: true,
    ),
    (name: 'store author', buyer: false, del: true, mine: false, show: false),
    (name: 'not deletable', buyer: true, del: false, mine: false, show: false),
    (
      name: 'no content-contract capability',
      buyer: true,
      del: true,
      mine: false,
      show: false,
    ),
  ]) {
    testWidgets('forum block item: ${c.name}', (tester) async {
      await _pumpEcho(
        tester,
        message: testForumEntry(
          id: '42',
          author: '小明',
          body: '請問何時出貨?',
          authorIsBuyer: c.buyer,
          canDelete: c.del,
        ),
        canBlock: c.name != 'no content-contract capability',
      );
      await tester.tap(find.byTooltip('留言更多操作'));
      await tester.pumpAndSettle();
      expect(find.text('檢舉留言'), findsOneWidget);
      expect(find.text('封鎖此人'), c.show ? findsOneWidget : findsNothing);
    });
  }
}
