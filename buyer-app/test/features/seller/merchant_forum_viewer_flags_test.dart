import 'dart:async';

import 'package:flutter/material.dart';
import 'package:flutter_screenutil/flutter_screenutil.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:hawkr_api/transport.dart' show Code, ConnectException;
import 'package:hawkr_buyer/config/flavor.dart';
import 'package:hawkr_buyer/core/networking/connect_provider.dart';
import 'package:hawkr_buyer/core/theme/app_theme.dart';
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

Future<void> _pump(
  WidgetTester tester,
  FakeSocialPublic public, {
  FakeSocialApp? app,
}) async {
  final container = ProviderContainer(
    overrides: [
      socialPublicClientProvider.overrideWithValue(public.client()),
      socialAppClientProvider.overrideWithValue(
        (app ?? FakeSocialApp()).client(),
      ),
    ],
  );
  addTearDown(container.dispose);
  await tester.pumpWidget(
    UncontrolledProviderScope(
      container: container,
      child: ScreenUtilInit(
        designSize: const Size(390, 844),
        enableScaleWH: () => false,
        enableScaleText: () => false,
        builder: (_, _) => MaterialApp(
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
    expect(request.body, '這是給另一人的回覆');
    expect(request.requestId, isNotEmpty);
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
