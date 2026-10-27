import 'dart:async';

import 'package:hawkr_api/gen/hawkr/seller/v1alpha1/seller.pbenum.dart';
import 'package:hawkr_seller/config/flavor.dart';
import 'package:hawkr_seller/core/codemeta/code_meta.dart';
import 'package:hawkr_seller/core/exceptions/app_exception.dart';
import 'package:hawkr_seller/core/session/my_store.dart';
import 'package:hawkr_seller/core/theme/app_theme.dart';
import 'package:hawkr_seller/shared/icons/app_icons.dart';
import 'package:hawkr_seller/features/post/data/post_model.dart';
import 'package:hawkr_seller/features/post/data/post_publish.dart';
import 'package:hawkr_seller/features/post/data/post_repository.dart';
import 'package:hawkr_seller/features/post/presentation/post_controller.dart';
import 'package:hawkr_seller/features/post/presentation/page_home_feed_screen.dart';
import 'package:hawkr_seller/features/seller/data/current_store.dart';
import 'package:hawkr_seller/shared/utils/floating_toast.dart';
import 'package:flutter/material.dart';
import 'package:flutter_screenutil/flutter_screenutil.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:hooks_riverpod/hooks_riverpod.dart';
import 'package:mocktail/mocktail.dart';

import '../../support/my_store_fixtures.dart';

class _Repo extends Mock implements PostRepository {}

final _draft = PostPublishStatus.draft.value;
final _published = PostPublishStatus.published.value;
final _unpublished = PostPublishStatus.unpublished.value;

Post _post({
  int id = 42,
  String? publishStatus,
  int? hiddenAt,
  DateTime? scheduledPublishAt,
  String content = '貼文內容',
  List<String> imageKeys = const ['posts/a.jpg'],
}) => Post(
  id: id,
  sellerId: 7,
  contentType: 'IMAGE',
  content: content,
  imageKeys: imageKeys,
  likeCount: 5,
  publishStatus: publishStatus,
  hiddenAt: hiddenAt,
  scheduledPublishAt: scheduledPublishAt,
);

MyStore _store({List<SellerCapability>? capabilities}) => MyStore(
  myStoreResponse(
    id: '7',
    storeCode: 'FRESH88',
    capabilities: capabilities,
    publicListing: publicListingMissing(),
  ),
);

Future<_Repo> _pump(
  WidgetTester tester,
  List<Post> Function() posts, {
  MyStore? store,
  int initialIndex = 0,
  int? initialPostId,
}) async {
  tester.view.physicalSize = const Size(390, 844);
  tester.view.devicePixelRatio = 1;
  addTearDown(tester.view.resetPhysicalSize);
  addTearDown(tester.view.resetDevicePixelRatio);
  addTearDown(FloatingToast.dismiss);
  final repo = _Repo();
  when(() => repo.listMyPosts()).thenAnswer((_) async => posts());
  when(() => repo.listCommentsRaw(any())).thenAnswer((_) async => []);
  when(() => repo.listCommentReplies(any())).thenAnswer((_) async => []);
  await tester.pumpWidget(
    ProviderScope(
      overrides: [
        postRepositoryProvider.overrideWithValue(repo),
        myStoreSnapshotProvider.overrideWithValue(store ?? _store()),
      ],
      child: ScreenUtilInit(
        designSize: const Size(390, 844),
        enableScaleWH: () => false,
        enableScaleText: () => false,
        builder: (_, _) => MaterialApp(
          theme: AppTheme.light,
          home: PageHomeFeedScreen(
            initialIndex: initialIndex,
            initialPostId: initialPostId,
            sellerName: '測試店家',
          ),
        ),
      ),
    ),
  );
  await tester.pumpAndSettle();
  return repo;
}

Finder get _publishButton =>
    find.byKey(const ValueKey('seller-post-publish-42'));

Finder get _publishOutlined =>
    find.descendant(of: _publishButton, matching: find.byType(OutlinedButton));

void main() {
  setUpAll(() {
    if (!F.isInitialized) F.init(Flavor.dev);
  });

  group('status badges', () {
    for (final (status, hiddenAt, label) in [
      (_draft, null, '未發布'),
      (_unpublished, null, '已下架'),
      (_published, 1760000000, '平台下架'),
    ]) {
      testWidgets('$label 的徽章', (tester) async {
        await _pump(
          tester,
          () => [_post(publishStatus: status, hiddenAt: hiddenAt)],
        );
        expect(find.text(label), findsOneWidget);
        expect(
          find.byKey(const ValueKey('seller-post-status-42')),
          findsOneWidget,
        );
      });
    }

    testWidgets('已發布不顯示徽章', (tester) async {
      await _pump(tester, () => [_post(publishStatus: _published)]);
      expect(find.byKey(const ValueKey('seller-post-status-42')), findsNothing);
    });

    testWidgets('平台下架加上已下架（4403）只顯示平台下架', (tester) async {
      await _pump(
        tester,
        () => [_post(publishStatus: _unpublished, hiddenAt: 1760000000)],
      );
      expect(find.text('平台下架'), findsOneWidget);
      expect(find.text('已下架'), findsNothing);
      expect(find.text('未發布'), findsNothing);
    });

    testWidgets('有排程的未發布貼文顯示排程時間', (tester) async {
      // UTC 01:05 = 臺北 09:05；顯示不跟著裝置時區。
      final at = DateTime.utc(2026, 10, 20, 1, 5);
      await _pump(
        tester,
        () => [_post(publishStatus: _draft, scheduledPublishAt: at)],
      );
      expect(find.text('未發布'), findsOneWidget);
      expect(find.text('排程於 2026/10/20 09:05 發布'), findsOneWidget);
    });

    testWidgets('沒有排程時不顯示排程時間', (tester) async {
      await _pump(tester, () => [_post(publishStatus: _draft)]);
      expect(find.textContaining('排程於'), findsNothing);
    });
  });

  testWidgets('沒有任何按讚入口：雙擊、愛心都不存在，數字只顯示', (tester) async {
    await _pump(tester, () => [_post(publishStatus: _published)]);

    expect(find.bySemanticsLabel(RegExp('按讚')), findsNothing);
    expect(
      find.byWidgetPredicate(
        (widget) => widget is Icon && widget.icon == AppIcons.favorite,
      ),
      findsNothing,
    );
    final media = find.byKey(const ValueKey('seller-post-media-42'));
    await tester.tap(media);
    await tester.pump(const Duration(milliseconds: 50));
    await tester.tap(media);
    await tester.pumpAndSettle();
    expect(find.text('5'), findsOneWidget);
    expect(tester.takeException(), isNull);
  });

  group('share', () {
    testWidgets('已發布且店家可公開才顯示分享', (tester) async {
      await _pump(tester, () => [_post(publishStatus: _published)]);
      expect(find.bySemanticsLabel('分享貼文'), findsOneWidget);
    });

    for (final (name, status, hiddenAt) in [
      ('未發布', '4401', null),
      ('已下架', '4403', null),
      ('平台下架', '4402', 1760000000),
    ]) {
      testWidgets('$name 的貼文不顯示分享', (tester) async {
        await _pump(
          tester,
          () => [_post(publishStatus: status, hiddenAt: hiddenAt)],
        );
        expect(find.bySemanticsLabel('分享貼文'), findsNothing);
      });
    }

    testWidgets('店家還不能公開時分享停用並說明原因', (tester) async {
      await _pump(
        tester,
        () => [_post(publishStatus: _published)],
        store: MyStore(
          myStoreResponse(
            id: '7',
            storeCode: 'FRESH88',
            publicListing: publicListingMissing([
              SellerRequirement.SELLER_REQUIREMENT_KYC_APPROVED,
            ]),
          ),
        ),
      );
      await tester.tap(find.bySemanticsLabel('分享貼文'));
      await tester.pumpAndSettle();
      expect(find.text('店家公開後才能分享店面連結'), findsOneWidget);
    });

    testWidgets('店家還不能公開且貼文未發布時不顯示分享', (tester) async {
      await _pump(
        tester,
        () => [_post(publishStatus: _draft)],
        store: MyStore(
          myStoreResponse(
            id: '7',
            storeCode: 'FRESH88',
            publicListing: publicListingMissing([
              SellerRequirement.SELLER_REQUIREMENT_KYC_APPROVED,
            ]),
          ),
        ),
      );
      expect(find.bySemanticsLabel('分享貼文'), findsNothing);
    });
  });

  group('publish control', () {
    for (final (status, label) in [
      (_draft, '發布'),
      (_unpublished, '發布'),
      (_published, '下架'),
    ]) {
      testWidgets('狀態 $status 的按鈕是「$label」', (tester) async {
        await _pump(tester, () => [_post(publishStatus: status)]);
        expect(
          find.descendant(of: _publishButton, matching: find.text(label)),
          findsOneWidget,
        );
      });
    }

    testWidgets('缺值的貼文沒有發布按鈕', (tester) async {
      await _pump(tester, () => [_post()]);
      expect(_publishButton, findsNothing);
    });

    testWidgets('發布：確認後送出、重讀列表，不做樂觀更新', (tester) async {
      var status = _draft;
      final repo = await _pump(tester, () => [_post(publishStatus: status)]);
      when(() => repo.publishPostOrThrow(42)).thenAnswer((_) async {
        status = _published;
      });

      await tester.tap(_publishButton);
      await tester.pumpAndSettle();
      expect(find.text('發布這則貼文？發布後買家會在動態牆看到它。'), findsOneWidget);
      expect(find.text('這則貼文已排程，現在發布會取消排程。'), findsNothing);
      verifyNever(() => repo.publishPostOrThrow(any()));

      await tester.tap(find.text('發布').last);
      await tester.pumpAndSettle();

      verify(() => repo.publishPostOrThrow(42)).called(1);
      verify(() => repo.listMyPosts()).called(2);
      expect(find.text('未發布'), findsNothing);
      expect(
        find.descendant(of: _publishButton, matching: find.text('下架')),
        findsOneWidget,
      );
    });

    testWidgets('取消確認不送出', (tester) async {
      final repo = await _pump(tester, () => [_post(publishStatus: _draft)]);
      await tester.tap(_publishButton);
      await tester.pumpAndSettle();
      await tester.tap(find.text('取消'));
      await tester.pumpAndSettle();
      verifyNever(() => repo.publishPostOrThrow(any()));
    });

    testWidgets('有排程的貼文發布確認多一句取消排程', (tester) async {
      await _pump(
        tester,
        () => [
          _post(
            publishStatus: _draft,
            scheduledPublishAt: DateTime.utc(2026, 10, 20, 1, 5),
          ),
        ],
      );
      await tester.tap(_publishButton);
      await tester.pumpAndSettle();
      expect(find.text('發布這則貼文？發布後買家會在動態牆看到它。'), findsOneWidget);
      expect(find.text('這則貼文已排程，現在發布會取消排程。'), findsOneWidget);
    });

    testWidgets('下架：確認文案與送出', (tester) async {
      var status = _published;
      final repo = await _pump(tester, () => [_post(publishStatus: status)]);
      when(() => repo.unpublishPostOrThrow(42)).thenAnswer((_) async {
        status = _unpublished;
      });

      await tester.tap(_publishButton);
      await tester.pumpAndSettle();
      expect(
        find.text('下架這則貼文？內容會保留，但買家不再看得到。下架後可以再次發布，但無法改回「未發布」。'),
        findsOneWidget,
      );
      await tester.tap(find.text('下架').last);
      await tester.pumpAndSettle();

      verify(() => repo.unpublishPostOrThrow(42)).called(1);
      expect(find.text('已下架'), findsOneWidget);
    });

    testWidgets('送出中顯示進行中文字並停用', (tester) async {
      final repo = await _pump(tester, () => [_post(publishStatus: _draft)]);
      final request = Completer<void>();
      when(() => repo.publishPostOrThrow(42)).thenAnswer((_) => request.future);

      await tester.tap(_publishButton);
      await tester.pumpAndSettle();
      await tester.tap(find.text('發布').last);
      await tester.pump();
      expect(
        find.descendant(of: _publishButton, matching: find.text('發布中…')),
        findsOneWidget,
      );
      verify(() => repo.publishPostOrThrow(42)).called(1);

      request.complete();
      await tester.pumpAndSettle();
    });

    testWidgets('下架失敗顯示伺服器原因、狀態不變', (tester) async {
      final repo = await _pump(
        tester,
        () => [_post(publishStatus: _published)],
      );
      when(
        () => repo.unpublishPostOrThrow(42),
      ).thenThrow(const ClientException(message: '貼文已被平台下架', statusCode: 409));

      await tester.tap(_publishButton);
      await tester.pumpAndSettle();
      await tester.tap(find.text('下架').last);
      await tester.pump();
      await tester.pump();

      expect(find.text('下架失敗：貼文已被平台下架'), findsOneWidget);
      verify(() => repo.listMyPosts()).called(1);
      expect(
        find.descendant(of: _publishButton, matching: find.text('下架')),
        findsOneWidget,
      );
      await tester.pump(const Duration(seconds: 3));
    });

    testWidgets('發布成功但重讀列表失敗：留著舊列表、不顯示發布失敗、發布與下架停用', (tester) async {
      final repo = await _pump(tester, () => [_post(publishStatus: _draft)]);
      when(() => repo.publishPostOrThrow(42)).thenAnswer((_) async {});
      // 第一次讀（build）成功，發布後重讀失敗。
      when(
        () => repo.listMyPosts(),
      ).thenThrow(const ClientException(message: '讀取失敗', statusCode: 500));

      await tester.tap(_publishButton);
      await tester.pumpAndSettle();
      await tester.tap(find.text('發布').last);
      await tester.pumpAndSettle();

      verify(() => repo.publishPostOrThrow(42)).called(1);
      expect(find.textContaining('發布失敗'), findsNothing);
      expect(find.textContaining('Error:'), findsNothing);
      // 舊列表還在，重讀失敗的原因用既有的 toast 交代。
      expect(find.text('貼文內容'), findsOneWidget);
      expect(find.text('讀取失敗'), findsOneWidget);
      // 舊列表上的狀態不可信：按鈕還在但停用，點了不再送出。
      expect(_publishButton, findsOneWidget);
      expect(tester.widget<OutlinedButton>(_publishOutlined).onPressed, isNull);
      await tester.tap(_publishButton, warnIfMissed: false);
      await tester.pumpAndSettle();
      verifyNever(() => repo.unpublishPostOrThrow(any()));
      await tester.pump(const Duration(seconds: 3));
    });

    testWidgets('重讀失敗後下一次重讀成功：按鈕依新狀態恢復', (tester) async {
      var status = _draft;
      final repo = await _pump(tester, () => [_post(publishStatus: status)]);
      when(() => repo.publishPostOrThrow(42)).thenAnswer((_) async {
        status = _published;
      });
      var fail = true;
      when(() => repo.listMyPosts()).thenAnswer((_) async {
        if (fail) {
          throw const ClientException(message: '讀取失敗', statusCode: 500);
        }
        return [_post(publishStatus: status)];
      });
      await tester.tap(_publishButton);
      await tester.pumpAndSettle();
      await tester.tap(find.text('發布').last);
      await tester.pumpAndSettle();
      expect(tester.widget<OutlinedButton>(_publishOutlined).onPressed, isNull);

      fail = false;
      final container = ProviderScope.containerOf(
        tester.element(find.byType(PageHomeFeedScreen)),
      );
      await container.read(postListProvider.notifier).silentRefresh();
      await tester.pumpAndSettle();

      expect(
        find.descendant(of: _publishButton, matching: find.text('下架')),
        findsOneWidget,
      );
      expect(
        tester.widget<OutlinedButton>(_publishOutlined).onPressed,
        isNotNull,
      );
      await tester.pump(const Duration(seconds: 3));
    });

    testWidgets('沒有照片的草稿按發布：顯示提示、不開確認、不送出', (tester) async {
      final repo = await _pump(
        tester,
        () => [_post(publishStatus: _draft, imageKeys: const [])],
      );

      await tester.tap(_publishButton);
      await tester.pumpAndSettle();

      expect(find.textContaining('請先補上照片'), findsOneWidget);
      expect(find.textContaining('請先編輯貼文並加入至少 1 張照片，再回來發布。'), findsOneWidget);
      expect(find.text(kPostPublishConfirm), findsNothing);
      verifyNever(() => repo.publishPostOrThrow(any()));
      await tester.pump(const Duration(seconds: 3));
    });

    testWidgets('平台下架時發布停用並說明原因', (tester) async {
      final repo = await _pump(
        tester,
        () => [_post(publishStatus: _draft, hiddenAt: 1760000000)],
      );

      await tester.tap(_publishButton);
      await tester.pumpAndSettle();

      expect(find.text('已被平台下架，發布後買家仍看不到'), findsOneWidget);
      expect(find.text('發布這則貼文？發布後買家會在動態牆看到它。'), findsNothing);
      verifyNever(() => repo.publishPostOrThrow(any()));
    });

    testWidgets('停權（沒有 MARKETING）時沒有發布按鈕', (tester) async {
      await _pump(
        tester,
        () => [_post(publishStatus: _draft)],
        store: _store(capabilities: suspendedCapabilities),
      );
      expect(_publishButton, findsNothing);
    });

    testWidgets('停權時下架仍在（CONTENT_CONTRACT）', (tester) async {
      await _pump(
        tester,
        () => [_post(publishStatus: _published)],
        store: _store(capabilities: suspendedCapabilities),
      );
      expect(
        find.descendant(of: _publishButton, matching: find.text('下架')),
        findsOneWidget,
      );
    });
  });

  group('開到指定的貼文', () {
    final posts = [
      _post(id: 41, publishStatus: _published, content: '第一篇貼文'),
      _post(id: 42, publishStatus: _draft, content: '第二篇貼文'),
      _post(id: 43, publishStatus: _published, content: '第三篇貼文'),
    ];

    testWidgets('postId 對到清單裡的那一篇（含未發布的）', (tester) async {
      await _pump(tester, () => posts, initialPostId: 43);
      expect(find.text('第三篇貼文'), findsOneWidget);
      expect(find.text('第一篇貼文'), findsNothing);
    });

    testWidgets('找不到時停在第一篇', (tester) async {
      await _pump(tester, () => posts, initialPostId: 999);
      expect(find.text('第一篇貼文'), findsOneWidget);
    });

    test('feedIndexForPost', () {
      expect(feedIndexForPost(posts, 42), 1);
      expect(feedIndexForPost(posts, 999), 0);
      expect(feedIndexForPost(posts, null), 0);
    });
  });
}
