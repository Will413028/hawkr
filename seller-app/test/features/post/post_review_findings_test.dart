import 'dart:async' show runZonedGuarded;
import 'dart:convert' show base64Decode;
import 'dart:io';

import 'package:extended_text_field/extended_text_field.dart';
import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_screenutil/flutter_screenutil.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:go_router/go_router.dart';
import 'package:hawkr_api/gen/hawkr/seller/v1alpha1/seller.pbenum.dart';
import 'package:hawkr_api/gen/hawkr/social/v1alpha1/social.pbenum.dart';
import 'package:hawkr_api/transport.dart';
import 'package:hawkr_seller/config/flavor.dart';
import 'package:hawkr_seller/core/l10n/ui_locale.dart';
import 'package:hawkr_seller/core/session/my_store.dart';
import 'package:hawkr_seller/core/theme/app_theme.dart';
import 'package:hawkr_seller/features/post/data/post_model.dart';
import 'package:hawkr_seller/features/post/data/post_repository.dart';
import 'package:hawkr_seller/features/post/presentation/post_compose_screen.dart';
import 'package:hawkr_seller/features/post/presentation/post_controller.dart';
import 'package:hawkr_seller/features/post/presentation/post_edit_screen.dart';
import 'package:hawkr_seller/features/product/data/product_repository.dart';
import 'package:hawkr_seller/features/seller/data/current_store.dart';
import 'package:hawkr_seller/shared/icons/app_icons.dart';
import 'package:hawkr_seller/shared/utils/floating_toast.dart';
import 'package:hawkr_seller/shared/widgets/linked_product_card.dart';
import 'package:hooks_riverpod/hooks_riverpod.dart';
import 'package:mocktail/mocktail.dart';

import '../../support/my_store_fixtures.dart';

class _Repo extends Mock implements PostRepository {}

class _Products extends Mock implements ProductRepository {}

const _linked = PostLinkedProduct(id: '900', name: '蘋果', price: 120);

Post _post({int id = 42, String etag = 'e1', PostLinkedProduct? linked}) =>
    Post(
      id: id,
      etag: etag,
      content: '貼文內容',
      status: PostStatus.POST_STATUS_DRAFT,
      linkedProduct: linked,
    );

MyStore _store(List<SellerCapability>? capabilities) => MyStore(
  myStoreResponse(
    id: '7',
    storeCode: 'FRESH88',
    capabilities: capabilities,
    publicListing: publicListingMissing(),
  ),
);

Future<void> _pumpScreen(
  WidgetTester tester, {
  required _Repo repo,
  required Widget screen,
  List<Map<String, dynamic>> products = const [],
}) async {
  tester.view.devicePixelRatio = 1;
  tester.view.physicalSize = const Size(402, 874);
  addTearDown(tester.view.resetDevicePixelRatio);
  addTearDown(tester.view.resetPhysicalSize);
  addTearDown(FloatingToast.dismiss);
  final productRepo = _Products();
  when(() => productRepo.getMyProducts()).thenAnswer(
    (_) async => {
      'data': {'records': products},
    },
  );
  final router = GoRouter(
    routes: [
      GoRoute(path: '/', builder: (_, _) => const Scaffold()),
      GoRoute(path: '/screen', builder: (_, _) => screen),
    ],
  );
  addTearDown(router.dispose);
  await tester.pumpWidget(
    ProviderScope(
      overrides: [
        postRepositoryProvider.overrideWithValue(repo),
        productRepositoryProvider.overrideWithValue(productRepo),
        sellerStoreIdProvider.overrideWithValue('7'),
        defaultErrorTextProvider.overrideWithValue(
          (code) => 'ERR-${code.name}',
        ),
      ],
      child: ScreenUtilInit(
        designSize: const Size(414, 896),
        enableScaleWH: () => false,
        enableScaleText: () => false,
        builder: (_, _) =>
            MaterialApp.router(theme: AppTheme.light, routerConfig: router),
      ),
    ),
  );
  unawaited(router.push('/screen'));
  await tester.pumpAndSettle();
}

void unawaited(Future<void> _) {}

void main() {
  setUpAll(() {
    if (!F.isInitialized) F.init(Flavor.dev);
    registerFallbackValue(Uint8List(0));
  });

  group('F4 編輯貼文不會悄悄解除關聯商品', () {
    testWidgets('只改內文：商品清單裡沒有該商品時 updateProduct 為 false', (tester) async {
      final repo = _Repo();
      final post = _post(linked: _linked);
      when(() => repo.listMyPosts()).thenAnswer((_) async => [post]);
      when(
        () => repo.updatePost(
          any(),
          etag: any(named: 'etag'),
          content: any(named: 'content'),
          imageStagedToken: any(named: 'imageStagedToken'),
          productId: any(named: 'productId'),
          updateProduct: any(named: 'updateProduct'),
          hashtags: any(named: 'hashtags'),
        ),
      ).thenAnswer((_) async => post);
      await _pumpScreen(
        tester,
        repo: repo,
        screen: PostEditScreen(post: post),
      );

      await tester.tap(find.widgetWithText(TextButton, '儲存'));
      await tester.pumpAndSettle();

      final call = verify(
        () => repo.updatePost(
          42,
          etag: 'e1',
          content: any(named: 'content'),
          imageStagedToken: any(named: 'imageStagedToken'),
          productId: captureAny(named: 'productId'),
          updateProduct: captureAny(named: 'updateProduct'),
          hashtags: any(named: 'hashtags'),
        ),
      )..called(1);
      expect(call.captured, [false, '900']);
      await tester.pump(const Duration(seconds: 3));
    });
  });

  group('F7 非 Connect 例外仍有回饋', () {
    testWidgets('編輯儲存', (tester) async {
      final repo = _Repo();
      final post = _post();
      when(() => repo.listMyPosts()).thenAnswer((_) async => [post]);
      when(
        () => repo.updatePost(
          any(),
          etag: any(named: 'etag'),
          content: any(named: 'content'),
          imageStagedToken: any(named: 'imageStagedToken'),
          productId: any(named: 'productId'),
          updateProduct: any(named: 'updateProduct'),
          hashtags: any(named: 'hashtags'),
        ),
      ).thenThrow(Exception('boom'));
      await _pumpScreen(
        tester,
        repo: repo,
        screen: PostEditScreen(post: post),
      );

      await tester.tap(find.widgetWithText(TextButton, '儲存'));
      await tester.pump();
      await tester.pump();

      expect(tester.takeException(), isNull);
      expect(find.byType(PostEditScreen), findsOneWidget);
      expect(find.text('ERR-unknown'), findsOneWidget);
      await tester.pump(const Duration(seconds: 3));
    });

    testWidgets('新增貼文', (tester) async {
      final repo = _Repo();
      when(
        () => repo.uploadMedia(
          sellerId: any(named: 'sellerId'),
          bytes: any(named: 'bytes'),
          filename: any(named: 'filename'),
          mimeType: any(named: 'mimeType'),
        ),
      ).thenAnswer((_) async => 'tok');
      when(
        () => repo.createPost(
          content: any(named: 'content'),
          imageStagedToken: any(named: 'imageStagedToken'),
          productId: any(named: 'productId'),
          hashtags: any(named: 'hashtags'),
        ),
      ).thenThrow(Exception('boom'));
      await _pumpScreen(
        tester,
        repo: repo,
        screen: PostComposeScreen(
          contentType: 'image',
          mediaBytes: Uint8List.fromList([0xFF, 0xD8, 0xFF, 0xE0, 0, 0]),
        ),
      );

      final input = find.byType(ExtendedTextField);
      expect(input, findsOneWidget);
      await tester.tap(input);
      await tester.pump();
      tester.testTextInput.enterText('新貼文');
      await tester.pump();
      await tester.tap(find.widgetWithText(TextButton, '儲存'));
      await tester.pump();
      await tester.pump();

      expect(tester.takeException(), isNull);
      expect(find.byType(PostComposeScreen), findsOneWidget);
      expect(find.text('ERR-unknown'), findsOneWidget);
      final save = find.widgetWithText(TextButton, '儲存');
      expect(tester.widget<TextButton>(save).onPressed, isNotNull);
      await tester.pump(const Duration(seconds: 3));
    });
  });

  group('PostList controller', () {
    ProviderContainer container(_Repo repo) {
      final c = ProviderContainer(
        overrides: [postRepositoryProvider.overrideWithValue(repo)],
      );
      addTearDown(c.dispose);
      return c;
    }

    test('F5 publish 遇到 aborted：重讀列表並把錯誤往上丟', () async {
      final repo = _Repo();
      var etag = 'old';
      when(
        () => repo.listMyPosts(),
      ).thenAnswer((_) async => [_post(etag: etag)]);
      when(() => repo.getMyPost(42)).thenAnswer((_) async => _post(etag: etag));
      when(
        () => repo.publishPostOrThrow(42, etag: any(named: 'etag')),
      ).thenAnswer((invocation) async {
        if (invocation.namedArguments[#etag] == 'old') {
          etag = 'new';
          throw ConnectException(Code.aborted, 'stale');
        }
      });
      final c = container(repo);
      await c.read(postListProvider.future);

      await expectLater(
        c.read(postListProvider.notifier).setPublished(42, publish: true),
        throwsA(isA<ConnectException>()),
      );
      expect(c.read(postListProvider).value!.single.etag, 'new');

      await c.read(postListProvider.notifier).setPublished(42, publish: true);
      verify(() => repo.publishPostOrThrow(42, etag: 'new')).called(1);
    });

    test('F5 updatePost 遇到 aborted：重讀列表', () async {
      final repo = _Repo();
      var etag = 'old';
      when(
        () => repo.listMyPosts(),
      ).thenAnswer((_) async => [_post(etag: etag)]);
      when(() => repo.getMyPost(42)).thenAnswer((_) async => _post(etag: etag));
      when(
        () => repo.updatePost(
          any(),
          etag: any(named: 'etag'),
          content: any(named: 'content'),
          imageStagedToken: any(named: 'imageStagedToken'),
          productId: any(named: 'productId'),
          updateProduct: any(named: 'updateProduct'),
          hashtags: any(named: 'hashtags'),
        ),
      ).thenAnswer((_) async {
        etag = 'new';
        throw ConnectException(Code.aborted, 'stale');
      });
      final c = container(repo);
      final posts = await c.read(postListProvider.future);

      await expectLater(
        c
            .read(postListProvider.notifier)
            .updatePost(posts.single, content: 'x'),
        throwsA(isA<ConnectException>()),
      );
      expect(c.read(postListProvider).value!.single.etag, 'new');
    });

    test('F6 貼文不在已載入的列表：向伺服器取 etag 再送出，不丟 StateError', () async {
      final repo = _Repo();
      when(() => repo.listMyPosts()).thenAnswer((_) async => []);
      when(
        () => repo.getMyPost(42),
      ).thenAnswer((_) async => _post(etag: 'fresh'));
      when(
        () => repo.unpublishPostOrThrow(42, etag: any(named: 'etag')),
      ).thenAnswer((_) async {});
      final c = container(repo);
      await c.read(postListProvider.future);

      await c.read(postListProvider.notifier).setPublished(42, publish: false);

      verify(() => repo.unpublishPostOrThrow(42, etag: 'fresh')).called(1);
    });
  });

  group('F9 canMarketPostsProvider', () {
    ProviderContainer withStore(MyStore? store) {
      final c = ProviderContainer(
        overrides: [myStoreSnapshotProvider.overrideWithValue(store)],
      );
      addTearDown(c.dispose);
      return c;
    }

    test('能力未讀到時不下結論；停權時為 false；一般店家為 true', () {
      expect(withStore(null).read(canMarketPostsProvider), isTrue);
      expect(
        withStore(_store(suspendedCapabilities)).read(canMarketPostsProvider),
        isFalse,
      );
      expect(withStore(_store(null)).read(canMarketPostsProvider), isTrue);
    });
  });

  group('Round 2', () {
    final updates = <Map<Symbol, dynamic>>[];

    setUp(updates.clear);

    void stubUpdate(_Repo repo, Future<Post> Function(Invocation) answer) {
      when(
        () => repo.updatePost(
          any(),
          etag: any(named: 'etag'),
          content: any(named: 'content'),
          imageStagedToken: any(named: 'imageStagedToken'),
          productId: any(named: 'productId'),
          updateProduct: any(named: 'updateProduct'),
          hashtags: any(named: 'hashtags'),
        ),
      ).thenAnswer((invocation) {
        updates.add(invocation.namedArguments);
        return answer(invocation);
      });
    }

    Finder contentField() => find.byType(ExtendedTextField);

    Future<void> typeContent(WidgetTester tester, String text) async {
      await tester.tap(contentField());
      await tester.pump();
      tester.testTextInput.enterText(text);
      await tester.pump();
    }

    Future<void> save(WidgetTester tester) async {
      await tester.tap(find.widgetWithText(TextButton, '儲存'));
      await tester.pumpAndSettle();
    }

    testWidgets('R2-2 aborted：載入伺服器最新內容，第二次儲存不會帶舊文字配新 etag', (tester) async {
      final repo = _Repo();
      final post = _post();
      final latest = post.copyWith(etag: 'e2', content: '別人改過的內容');
      when(() => repo.listMyPosts()).thenAnswer((_) async => [post]);
      when(() => repo.getMyPost(42)).thenAnswer((_) async => latest);
      var calls = 0;
      stubUpdate(repo, (_) async {
        calls++;
        if (calls == 1) throw ConnectException(Code.aborted, 'stale');
        return latest;
      });
      await _pumpScreen(
        tester,
        repo: repo,
        screen: PostEditScreen(post: post),
      );

      await typeContent(tester, '我的修改');
      await save(tester);

      expect(find.textContaining('已在其他地方更新'), findsOneWidget);
      expect(
        tester.widget<ExtendedTextField>(contentField()).controller!.text,
        '別人改過的內容',
      );
      await tester.pump(const Duration(seconds: 3));

      await save(tester);
      expect(updates.map((u) => [u[#etag], u[#content]]), [
        ['e1', '我的修改'],
        ['e2', '別人改過的內容'],
      ]);
      await tester.pump(const Duration(seconds: 3));
    });

    testWidgets('R2-3 商品變更與目前載入的貼文比較', (tester) async {
      final repo = _Repo();
      final post = _post();
      final latest = post.copyWith(etag: 'e2', linkedProduct: _linked);
      when(() => repo.listMyPosts()).thenAnswer((_) async => [post]);
      when(() => repo.getMyPost(42)).thenAnswer((_) async => latest);
      var calls = 0;
      stubUpdate(repo, (_) async {
        calls++;
        if (calls == 1) throw ConnectException(Code.aborted, 'stale');
        return latest;
      });
      await _pumpScreen(
        tester,
        repo: repo,
        screen: PostEditScreen(post: post),
        products: const [],
      );

      await save(tester);
      await tester.pump(const Duration(seconds: 3));
      await save(tester);

      expect(
        [updates.last[#productId], updates.last[#updateProduct]],
        ['900', false],
      );
      await tester.pump(const Duration(seconds: 3));
    });

    testWidgets('R2-4 目前關聯商品不在商品清單也看得到，且可取消關聯', (tester) async {
      final repo = _Repo();
      final post = _post(linked: _linked);
      when(() => repo.listMyPosts()).thenAnswer((_) async => [post]);
      stubUpdate(repo, (_) async => post);
      await _pumpScreen(
        tester,
        repo: repo,
        screen: PostEditScreen(post: post),
      );

      expect(find.byType(LinkedProductCard), findsOneWidget);
      expect(find.text('蘋果'), findsOneWidget);
      await tester.tap(find.byKey(const ValueKey('post-edit-unlink-product')));
      await tester.pump();
      expect(find.byType(LinkedProductCard), findsNothing);

      await save(tester);
      expect(
        [updates.single[#productId], updates.single[#updateProduct]],
        [null, true],
      );
      await tester.pump(const Duration(seconds: 3));
    });

    testWidgets('R2-7 程式錯誤不變成 toast，儲存鍵恢復可按', (tester) async {
      final repo = _Repo();
      final post = _post();
      when(() => repo.listMyPosts()).thenAnswer((_) async => [post]);
      stubUpdate(repo, (_) async => throw StateError('bug'));
      await _pumpScreen(
        tester,
        repo: repo,
        screen: PostEditScreen(post: post),
      );

      final errors = <Object>[];
      await runZonedGuarded(() async {
        await tester.tap(find.widgetWithText(TextButton, '儲存'));
        await tester.pumpAndSettle();
      }, (error, _) => errors.add(error));

      expect(errors.single, isA<StateError>());
      expect(find.text('ERR-unknown'), findsNothing);
      final button = find.widgetWithText(TextButton, '儲存');
      expect(tester.widget<TextButton>(button).onPressed, isNotNull);
    });

    testWidgets('R2-8 重試儲存不會重新上傳已暫存的圖片', (tester) async {
      final dir = Directory.systemTemp.createTempSync('post_edit_pick');
      addTearDown(() => dir.deleteSync(recursive: true));
      final file = File('${dir.path}/pick.png')
        ..writeAsBytesSync(
          base64Decode(
            'iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mNk+M9QDwADhgGAWjR9awAAAABJRU5ErkJggg==',
          ),
        );
      final messenger = tester.binding.defaultBinaryMessenger;
      messenger.setMockMethodCallHandler(
        const MethodChannel('plugins.flutter.io/image_picker'),
        (call) async => file.path,
      );
      addTearDown(
        () => messenger.setMockMethodCallHandler(
          const MethodChannel('plugins.flutter.io/image_picker'),
          null,
        ),
      );
      final repo = _Repo();
      final post = _post();
      when(() => repo.listMyPosts()).thenAnswer((_) async => [post]);
      when(
        () => repo.uploadMedia(
          sellerId: any(named: 'sellerId'),
          bytes: any(named: 'bytes'),
          filename: any(named: 'filename'),
          mimeType: any(named: 'mimeType'),
        ),
      ).thenAnswer((_) async => 'tok');
      var calls = 0;
      stubUpdate(repo, (_) async {
        calls++;
        if (calls == 1) throw ConnectException(Code.unavailable, 'down');
        return post;
      });
      await _pumpScreen(
        tester,
        repo: repo,
        screen: PostEditScreen(post: post),
      );

      await tester.runAsync(() async {
        await tester.tap(find.byIcon(AppIcons.permMediaOutlined));
        await Future<void>.delayed(const Duration(milliseconds: 300));
      });
      await tester.pumpAndSettle();
      await save(tester);
      await tester.pump(const Duration(seconds: 3));
      await save(tester);

      verify(
        () => repo.uploadMedia(
          sellerId: any(named: 'sellerId'),
          bytes: any(named: 'bytes'),
          filename: any(named: 'filename'),
          mimeType: any(named: 'mimeType'),
        ),
      ).called(1);
      expect(updates.map((u) => u[#imageStagedToken]), ['tok', 'tok']);
      await tester.pump(const Duration(seconds: 3));
    });

    test('R2-5 衝突後重讀失敗不被吞掉', () async {
      final repo = _Repo();
      when(() => repo.listMyPosts()).thenAnswer((_) async => [_post()]);
      when(
        () => repo.getMyPost(42),
      ).thenAnswer((_) async => throw ConnectException(Code.unavailable, 'x'));
      when(
        () => repo.updatePost(
          any(),
          etag: any(named: 'etag'),
          content: any(named: 'content'),
          imageStagedToken: any(named: 'imageStagedToken'),
          productId: any(named: 'productId'),
          updateProduct: any(named: 'updateProduct'),
          hashtags: any(named: 'hashtags'),
        ),
      ).thenAnswer((_) async => throw ConnectException(Code.aborted, 'stale'));
      final c = ProviderContainer(
        overrides: [postRepositoryProvider.overrideWithValue(repo)],
      );
      addTearDown(c.dispose);
      final posts = await c.read(postListProvider.future);

      await expectLater(
        c
            .read(postListProvider.notifier)
            .updatePost(posts.single, content: 'x'),
        throwsA(
          isA<ConnectException>().having(
            (e) => e.code,
            'code',
            Code.unavailable,
          ),
        ),
      );
    });
  });
}
