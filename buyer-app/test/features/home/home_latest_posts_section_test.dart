import 'package:hawkr_buyer/features/home/data/home_post_preview_repository.dart';
import 'package:hawkr_buyer/features/home/presentation/home_widget/home_latest_posts_section.dart';
import 'package:hawkr_buyer/features/social/data/social_model.dart';
import 'package:hawkr_buyer/shared/widgets/hk_post_card_surface.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:go_router/go_router.dart';
import 'package:hooks_riverpod/hooks_riverpod.dart';

class _FakeHomePostPreviewRepository implements HomePostPreviewRepository {
  _FakeHomePostPreviewRepository(this.handler);

  final Future<List<Post>> Function() handler;

  @override
  Future<List<Post>> fetchLatest({required int limit}) => handler();
}

void main() {
  testWidgets(
    'preview stays bounded and overflow-free at 320px and 1.4x text',
    (tester) async {
      await _setViewport(tester, 320);
      final semantics = tester.ensureSemantics();
      final repository = _FakeHomePostPreviewRepository(
        () async => [_post(1), _post(2), _post(3)],
      );

      await tester.pumpWidget(_testApp(repository: repository, textScale: 1.4));
      await tester.pump();

      expect(find.byKey(const ValueKey('home-post-1')), findsOneWidget);
      expect(find.byKey(const ValueKey('home-post-2')), findsOneWidget);
      expect(find.byKey(const ValueKey('home-post-3')), findsNothing);
      expect(find.byType(HkPostCardSurface), findsNWidgets(2));
      expect(find.byKey(const ValueKey('home-post-surface-1')), findsOneWidget);
      expect(tester.takeException(), isNull);
      await expectLater(tester, meetsGuideline(labeledTapTargetGuideline));
      semantics.dispose();
    },
  );

  testWidgets('post failure is isolated and the explicit retry can recover', (
    tester,
  ) async {
    await _setViewport(tester, 390);
    var attempt = 0;
    final repository = _FakeHomePostPreviewRepository(() async {
      attempt++;
      if (attempt == 1) throw StateError('social unavailable');
      return [_post(9)];
    });

    await tester.pumpWidget(
      _testApp(
        repository: repository,
        trailing: const SliverToBoxAdapter(
          child: Text('商品區仍可使用', key: ValueKey('product-sentinel')),
        ),
      ),
    );
    await tester.pump();

    expect(find.byKey(const ValueKey('home-posts-error')), findsOneWidget);
    expect(find.byKey(const ValueKey('product-sentinel')), findsOneWidget);

    await tester.tap(find.byKey(const ValueKey('home-posts-retry')));
    await tester.pump();
    await tester.pump();

    expect(find.byKey(const ValueKey('home-post-9')), findsOneWidget);
    expect(find.byKey(const ValueKey('product-sentinel')), findsOneWidget);
    expect(tester.takeException(), isNull);
  });

  testWidgets('preview exports the hero tag expected by the social feed', (
    tester,
  ) async {
    await _setViewport(tester, 390);
    final repository = _FakeHomePostPreviewRepository(() async => [_post(21)]);

    await tester.pumpWidget(_testApp(repository: repository));
    await tester.pump();

    final hero = tester.widget<Hero>(find.byType(Hero));
    expect(hero.tag, 'post-image-21');
    expect(tester.takeException(), isNull);
  });

  testWidgets('merchant post keeps the established social-feed navigation', (
    tester,
  ) async {
    await _setViewport(tester, 390);
    final repository = _FakeHomePostPreviewRepository(() async => [_post(11)]);
    final router = GoRouter(
      initialLocation: '/',
      routes: [
        GoRoute(
          path: '/',
          builder: (_, __) => const Scaffold(
            body: CustomScrollView(slivers: [HomeLatestPostsSection()]),
          ),
        ),
        GoRoute(
          path: '/social/posts',
          builder: (_, state) {
            final extra = state.extra! as Map<String, dynamic>;
            return Scaffold(
              body: Text(
                '${extra['merchantId']}|${extra['initialPostId']}|'
                '${extra['includeGlobalFeed']}',
              ),
            );
          },
        ),
      ],
    );
    addTearDown(router.dispose);

    await tester.pumpWidget(
      ProviderScope(
        overrides: [
          homePostPreviewRepositoryProvider.overrideWithValue(repository),
        ],
        child: MaterialApp.router(routerConfig: router),
      ),
    );
    await tester.pump();

    await tester.tap(find.byKey(const ValueKey('home-post-11')));
    await tester.pumpAndSettle();

    expect(find.text('7|11|true'), findsOneWidget);
  });
}

Widget _testApp({
  required HomePostPreviewRepository repository,
  double textScale = 1,
  Widget trailing = const SliverToBoxAdapter(child: SizedBox.shrink()),
}) {
  return ProviderScope(
    overrides: [
      homePostPreviewRepositoryProvider.overrideWithValue(repository),
    ],
    child: MaterialApp(
      builder: (context, child) => MediaQuery(
        data: MediaQuery.of(
          context,
        ).copyWith(textScaler: TextScaler.linear(textScale)),
        child: child!,
      ),
      home: Scaffold(
        body: CustomScrollView(
          slivers: [const HomeLatestPostsSection(), trailing],
        ),
      ),
    ),
  );
}

Future<void> _setViewport(WidgetTester tester, double width) async {
  tester.view.devicePixelRatio = 1;
  tester.view.physicalSize = Size(width, 844);
  addTearDown(tester.view.resetDevicePixelRatio);
  addTearDown(tester.view.resetPhysicalSize);
}

Post _post(int id) => Post(
  id: id,
  content: '這是一段足以在小螢幕換行的商家最新貼文內容 $id',
  merchantId: 7,
  merchantName: '南崁王生技長名稱',
  likeCount: 12,
  commentCount: 3,
  createdAt: '2026-09-10T12:00:00+08:00',
);
