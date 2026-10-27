import 'package:hawkr_seller/config/flavor.dart';
import 'package:hawkr_seller/core/codemeta/code_meta.dart';
import 'package:hawkr_seller/core/exceptions/app_exception.dart';
import 'package:hawkr_seller/core/theme/app_theme.dart';
import 'package:hawkr_seller/features/post/data/post_model.dart';
import 'package:hawkr_seller/features/post/data/post_repository.dart';
import 'package:hawkr_seller/features/post/presentation/page_home_feed_screen.dart';
import 'package:hawkr_seller/shared/utils/floating_toast.dart';
import 'package:flutter/material.dart';
import 'package:flutter_screenutil/flutter_screenutil.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:hooks_riverpod/hooks_riverpod.dart';
import 'package:mocktail/mocktail.dart';

class _MockPostRepository extends Mock implements PostRepository {}

void main() {
  setUpAll(() {
    if (!F.isInitialized) F.init(Flavor.dev);
  });

  testWidgets('failed publish shows the actionable API message and keeps the '
      'status', (tester) async {
    tester.view.physicalSize = const Size(390, 844);
    tester.view.devicePixelRatio = 1;
    addTearDown(tester.view.resetPhysicalSize);
    addTearDown(tester.view.resetDevicePixelRatio);
    addTearDown(FloatingToast.dismiss);

    final repository = _MockPostRepository();
    when(() => repository.listMyPosts()).thenAnswer(
      (_) async => [
        Post(
          id: 42,
          sellerId: 7,
          content: '測試貼文',
          imageKeys: const ['posts/a.jpg'],
          publishStatus: PostPublishStatus.draft.value,
        ),
      ],
    );
    when(() => repository.listCommentsRaw(42)).thenAnswer((_) async => []);
    when(
      () => repository.publishPostOrThrow(42),
    ).thenThrow(const ApiException(code: 9001, message: '目前無法發布'));

    await tester.pumpWidget(
      ProviderScope(
        overrides: [postRepositoryProvider.overrideWithValue(repository)],
        child: ScreenUtilInit(
          designSize: const Size(390, 844),
          builder: (_, _) => MaterialApp(
            theme: AppTheme.light,
            home: const PageHomeFeedScreen(sellerName: '測試店家'),
          ),
        ),
      ),
    );
    await tester.pumpAndSettle();

    await tester.tap(find.byKey(const ValueKey('seller-post-publish-42')));
    await tester.pumpAndSettle();
    await tester.tap(find.text('發布').last);
    await tester.pump();
    await tester.pump();

    expect(find.text('發布失敗：目前無法發布'), findsOneWidget);
    expect(find.text('未發布'), findsOneWidget);
    verify(() => repository.listMyPosts()).called(1);

    await tester.pump(const Duration(seconds: 3));
  });
}
