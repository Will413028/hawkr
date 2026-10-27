import 'dart:typed_data';

import 'package:hawkr_seller/config/flavor.dart';
import 'package:hawkr_seller/core/theme/app_theme.dart';
import 'package:hawkr_seller/features/post/data/post_publish.dart';
import 'package:hawkr_seller/features/post/data/post_repository.dart';
import 'package:hawkr_seller/features/post/presentation/post_compose_screen.dart';
import 'package:hawkr_seller/features/product/data/product_repository.dart';
import 'package:hawkr_seller/features/seller/data/current_store.dart';
import 'package:hawkr_seller/shared/utils/floating_toast.dart';
import 'package:flutter/material.dart';
import 'package:flutter_screenutil/flutter_screenutil.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:hooks_riverpod/hooks_riverpod.dart';
import 'package:mocktail/mocktail.dart';

class _Repo extends Mock implements PostRepository {}

class _Products extends Mock implements ProductRepository {}

Future<_Repo> _pump(WidgetTester tester, {Uint8List? mediaBytes}) async {
  tester.view.devicePixelRatio = 1;
  tester.view.physicalSize = const Size(402, 874);
  addTearDown(tester.view.resetDevicePixelRatio);
  addTearDown(tester.view.resetPhysicalSize);
  addTearDown(FloatingToast.dismiss);
  final repo = _Repo();
  final products = _Products();
  when(() => products.getMyProducts()).thenAnswer(
    (_) async => {
      'data': {'records': <Object>[]},
    },
  );
  await tester.pumpWidget(
    ProviderScope(
      overrides: [
        postRepositoryProvider.overrideWithValue(repo),
        productRepositoryProvider.overrideWithValue(products),
        sellerStoreIdProvider.overrideWithValue('7'),
      ],
      child: ScreenUtilInit(
        designSize: const Size(414, 896),
        enableScaleWH: () => false,
        enableScaleText: () => false,
        builder: (_, _) => MaterialApp(
          theme: AppTheme.light,
          home: PostComposeScreen(contentType: 'image', mediaBytes: mediaBytes),
        ),
      ),
    ),
  );
  await tester.pumpAndSettle();
  return repo;
}

void main() {
  setUpAll(() {
    if (!F.isInitialized) F.init(Flavor.dev);
    registerFallbackValue(Uint8List(0));
  });

  testWidgets('沒有照片：儲存鈕可按，按了提示補照片且不送出', (tester) async {
    final repo = await _pump(tester);

    final save = find.widgetWithText(TextButton, '儲存');
    expect(tester.widget<TextButton>(save).onPressed, isNotNull);
    await tester.tap(save);
    await tester.pump();

    expect(find.textContaining(kPostImageRequiredTitle), findsOneWidget);
    expect(find.textContaining('每則貼文至少需要 1 張照片才能發布。'), findsOneWidget);
    verifyNever(
      () => repo.uploadMedia(
        sellerId: any(named: 'sellerId'),
        bytes: any(named: 'bytes'),
        filename: any(named: 'filename'),
        mimeType: any(named: 'mimeType'),
      ),
    );
    verifyNever(
      () => repo.createPost(
        sellerId: any(named: 'sellerId'),
        title: any(named: 'title'),
        content: any(named: 'content'),
        contentType: any(named: 'contentType'),
        imageUrl: any(named: 'imageUrl'),
        linkedProductId: any(named: 'linkedProductId'),
        tags: any(named: 'tags'),
      ),
    );
    await tester.pump(const Duration(seconds: 3));
  });
}
