import 'dart:async';
import 'package:hawkr_seller/config/flavor.dart';
import 'package:hawkr_seller/core/theme/app_theme.dart';
import 'package:hawkr_seller/features/post/data/post_model.dart';
import 'package:hawkr_seller/features/post/data/post_repository.dart';
import 'package:hawkr_seller/features/post/presentation/widgets/page_home_tab.dart';
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
  for (final size in [
    const Size(320, 874),
    const Size(402, 874),
    const Size(440, 956),
    const Size(874, 402),
  ]) {
    for (final scale in [1.0, 1.4]) {
      testWidgets(
        'grid long-press preview fits $size/$scale with large counts',
        (tester) async {
          final semantics = tester.ensureSemantics();
          try {
            tester.view.devicePixelRatio = 1;
            tester.view.physicalSize = size;
            addTearDown(tester.view.resetDevicePixelRatio);
            addTearDown(tester.view.resetPhysicalSize);
            final repo = _Repo();
            final pendingComments = Completer<List<Comment>>();
            when(() => repo.listMyPostsRaw(page: 1, pageSize: 20)).thenAnswer(
              (_) async => const PostPageResult(
                posts: [
                  {
                    'id': 42,
                    'contentType': 'text',
                    'content': '測試產地直送貼文',
                    'likeCount': 123456789,
                    'commentCount': 987654321,
                  },
                ],
                hasMore: false,
              ),
            );
            when(
              () => repo.listComments(42),
            ).thenAnswer((_) => pendingComments.future);
            await tester.pumpWidget(
              ProviderScope(
                overrides: [postRepositoryProvider.overrideWithValue(repo)],
                child: ScreenUtilInit(
                  designSize: const Size(414, 896),
                  enableScaleWH: () => false,
                  enableScaleText: () => false,
                  minTextAdapt: false,
                  builder: (_, _) => MaterialApp(
                    theme: AppTheme.light,
                    builder: (context, child) => MediaQuery(
                      data: MediaQuery.of(
                        context,
                      ).copyWith(textScaler: TextScaler.linear(scale)),
                      child: child!,
                    ),
                    home: Scaffold(
                      body: PageHomeTab(
                        sellerName: '測試店家',
                        onReloadSeller: () async {},
                      ),
                    ),
                  ),
                ),
              ),
            );
            await tester.pumpAndSettle();
            await tester.longPress(
              find.byKey(const ValueKey('storefront-post-grid-item-42')),
            );
            await tester.pumpAndSettle();
            expect(tester.takeException(), isNull);
            final counts = find.text('99+');
            expect(counts, findsNWidgets(2));
            await tester.ensureVisible(counts.first);
            await tester.pumpAndSettle();
            for (final label in ['共 123456789 個讚', '留言，共 987654321 則']) {
              final target = find.bySemanticsLabel(label);
              expect(target, findsOneWidget);
              expect(tester.getSize(target).height, greaterThanOrEqualTo(48));
              expect(tester.getRect(target).left, greaterThanOrEqualTo(0));
              expect(
                tester.getRect(target).right,
                lessThanOrEqualTo(size.width),
              );
            }
            expect(tester.takeException(), isNull);
          } finally {
            semantics.dispose();
          }
        },
      );
    }
  }
}
