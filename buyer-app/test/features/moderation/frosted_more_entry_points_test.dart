import 'package:hawkr_buyer/l10n/l10n.dart';
import 'dart:convert';
import 'dart:typed_data';

import 'package:hawkr_api/gen/hawkr/common/v1alpha1/store_card.pb.dart';
import 'package:hawkr_api/gen/hawkr/social/v1alpha1/social.pb.dart' as pb;
import 'package:hawkr_api/testing.dart';
import 'package:hawkr_buyer/config/flavor.dart';
import 'package:hawkr_buyer/core/networking/connect_provider.dart';
import 'package:hawkr_buyer/core/networking/dio_provider.dart';
import 'package:hawkr_buyer/core/session/session_model.dart';
import 'package:hawkr_buyer/core/session/session_provider.dart';
import 'package:hawkr_buyer/core/theme/app_theme.dart';
import 'package:hawkr_api/gen/hawkr/moderation/v1alpha1/moderation.pbenum.dart';
import 'package:hawkr_buyer/features/moderation/data/block_target.dart';
import '../review/review_fakes.dart';
import 'package:hawkr_buyer/features/moderation/presentation/block_confirm_dialog.dart';
import 'package:hawkr_buyer/features/moderation/presentation/report_sheet.dart';
import 'package:hawkr_buyer/features/product/presentation/merchant_item_detail_screen.dart';
import 'package:hawkr_buyer/features/social/presentation/merchant_posts_feed_screen.dart';
import 'package:hawkr_buyer/features/social/presentation/post_detail_screen.dart';
import 'package:dio/dio.dart';
import 'package:hawkr_buyer/features/seller/data/seller_repository.dart';
import '../../features/social/social_fakes.dart';
import '../../support/fake_seller_repository.dart';
import 'package:flutter/material.dart';
import 'package:flutter_screenutil/flutter_screenutil.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:go_router/go_router.dart';
import 'package:hooks_riverpod/hooks_riverpod.dart';

class _Session extends Session {
  _Session(this.loggedIn);
  final bool loggedIn;
  @override
  Future<SessionModel?> build() async => loggedIn
      ? const SessionModel(userId: 'buyer-account', nickname: '測試買家')
      : null;
}

FakeSocialPublic _social() {
  final post = pb.Post(
    id: '101',
    store: StoreCard(id: '7', name: '測試品牌'),
    body: '檢查操作',
  );
  return FakeSocialPublic(post: post, posts: [post]);
}

class _Adapter implements HttpClientAdapter {
  final List<RequestOptions> mutations = [];
  @override
  Future<ResponseBody> fetch(
    RequestOptions options,
    Stream<Uint8List>? requestStream,
    Future<void>? cancelFuture,
  ) async {
    if (options.method != 'GET') mutations.add(options);
    final Object data = switch (options.path) {
      '/public/products/55' => {'id': '55', 'skus': <Object>[]},
      _ => <String, Object>{},
    };
    return ResponseBody.fromString(
      jsonEncode({'code': 0, 'data': data}),
      200,
      headers: {
        Headers.contentTypeHeader: [Headers.jsonContentType],
      },
    );
  }

  @override
  void close({bool force = false}) {}
}

void main() {
  if (!F.isInitialized) F.init(Flavor.dev);
  for (final surface in ['商家貼文', '貼文詳情', '商品評價']) {
    for (final loggedIn in [true, false]) {
      testWidgets(
        '$surface 更多在320px與大字級可點擊，${loggedIn ? '檢舉封鎖對象正確且取消無副作用' : '訪客登入提示可取消'}',
        (tester) async {
          tester.view.devicePixelRatio = 1;
          tester.view.physicalSize = const Size(320, 844);
          addTearDown(tester.view.resetDevicePixelRatio);
          addTearDown(tester.view.resetPhysicalSize);
          final adapter = _Adapter();
          final dio = Dio(BaseOptions(baseUrl: 'https://example.test'))
            ..httpClientAdapter = adapter;
          addTearDown(dio.close);
          final container = ProviderContainer(
            overrides: [
              dioProvider.overrideWithValue(dio),
              socialPublicClientProvider.overrideWithValue(_social().client()),
              reviewPublicClientProvider.overrideWithValue(
                FakeReviewPublic(
                  pages: [
                    [
                      reviewRecord(
                        'review-12',
                        content: '這是評價內容',
                        authorName: '測試顧客',
                      ),
                    ],
                  ],
                ).client(),
              ),
              connectAuthTransportProvider.overrideWith(
                (_) => FakeTransportBuilder().build(),
              ),
              sellerRepositoryProvider.overrideWithValue(
                fakeSellerRepository(),
              ),
              sessionProvider.overrideWith(() => _Session(loggedIn)),
            ],
          );
          addTearDown(container.dispose);
          await container.read(sessionProvider.future);
          final router = GoRouter(
            routes: [
              GoRoute(
                path: '/',
                builder: (_, __) => switch (surface) {
                  '商品評價' => const MerchantItemDetailScreen(
                    skuId: '55',
                    merchantId: '7',
                    title: '測試商品',
                  ),
                  '貼文詳情' => const PostDetailScreen(postId: 101),
                  _ => const MerchantPostsFeedScreen(merchantId: '7'),
                },
              ),
              GoRoute(
                path: '/login',
                builder: (_, __) => const Scaffold(body: Text('登入頁')),
              ),
            ],
          );
          addTearDown(router.dispose);
          await tester.pumpWidget(
            UncontrolledProviderScope(
              container: container,
              child: ScreenUtilInit(
                designSize: const Size(390, 844),
                enableScaleWH: () => false,
                enableScaleText: () => false,
                builder: (_, __) => MaterialApp.router(
                  localizationsDelegates:
                      AppLocalizations.localizationsDelegates,
                  supportedLocales: AppLocalizations.supportedLocales,
                  theme: AppTheme.light,
                  routerConfig: router,
                  builder: (context, child) => MediaQuery(
                    data: MediaQuery.of(
                      context,
                    ).copyWith(textScaler: const TextScaler.linear(1.4)),
                    child: child!,
                  ),
                ),
              ),
            ),
          );
          for (var i = 0; i < 10; i++) {
            await tester.pump(const Duration(milliseconds: 100));
          }
          final more = find.byTooltip(surface == '商品評價' ? '更多評價操作' : '更多貼文操作');
          await tester.ensureVisible(more);
          await tester.pumpAndSettle();
          expect(tester.getSize(more), const Size.square(48));
          final mutationsBefore = adapter.mutations.length;
          await tester.tap(more);
          await tester.pumpAndSettle();
          if (!loggedIn) {
            expect(find.text('需要登入'), findsOneWidget);
            await tester.tap(find.text('取消'));
            await tester.pumpAndSettle();
            expect(find.text('需要登入'), findsNothing);
            expect(more, findsOneWidget);
          } else {
            expect(find.text('檢舉'), findsOneWidget);
            await tester.tap(find.text('檢舉'));
            await tester.pumpAndSettle();
            final report = tester.widget<ReportSheetWidget>(
              find.byType(ReportSheetWidget),
            );
            final reportTarget = report.target;
            if (surface == '商品評價') {
              // 評價的檢舉只送參照：種類與評價 ID。
              expect(reportTarget.id, 'review-12');
              expect(
                reportTarget.kind,
                ReportTargetKind.REPORT_TARGET_KIND_REVIEW,
              );
            } else {
              expect(reportTarget.id, '101');
              expect(
                reportTarget.kind,
                ReportTargetKind.REPORT_TARGET_KIND_POST,
              );
            }
            router.pop();
            await tester.pumpAndSettle();
            await tester.tap(more);
            await tester.pumpAndSettle();
            if (surface == '商品評價') {
              // 契約只能封鎖評價的回覆店家，不能封鎖寫評價的買家，所以評價入口沒有封鎖選項。
              expect(find.text('封鎖此使用者'), findsNothing);
              await tester.tapAt(const Offset(10, 10));
              await tester.pumpAndSettle();
            } else {
              await tester.tap(find.text('封鎖此使用者'));
              await tester.pumpAndSettle();
              final block = tester.widget<BlockConfirmDialog>(
                find.byType(BlockConfirmDialog),
              );
              final blockTarget = block.target;
              expect(blockTarget, isA<StoreBlockTarget>());
              expect((blockTarget as StoreBlockTarget).storeId, '7');
              expect(blockTarget.confirmText, blockStoreConfirmText);
              await tester.tap(find.text('取消'));
              await tester.pumpAndSettle();
              expect(find.byType(BlockConfirmDialog), findsNothing);
            }
          }
          expect(adapter.mutations.length, mutationsBefore);
          expect(tester.takeException(), isNull);
        },
      );
    }
  }
}
