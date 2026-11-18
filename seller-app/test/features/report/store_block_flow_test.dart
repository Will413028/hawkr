import 'package:flutter/material.dart';
import 'package:flutter_screenutil/flutter_screenutil.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:hawkr_api/gen/hawkr/common/v1alpha1/image.pb.dart' as common;
import 'package:hawkr_api/gen/hawkr/seller/v1alpha1/seller.connect.client.dart';
import 'package:hawkr_api/gen/hawkr/seller/v1alpha1/seller.connect.spec.dart';
import 'package:hawkr_api/gen/hawkr/seller/v1alpha1/seller.pb.dart' as pb;
import 'package:hawkr_api/testing.dart';
import 'package:hooks_riverpod/hooks_riverpod.dart';
import 'package:hawkr_seller/config/flavor.dart';
import 'package:hawkr_seller/core/theme/app_theme.dart';
import 'package:hawkr_seller/features/report/data/store_block_repository.dart';
import 'package:hawkr_seller/shared/models/buyer_card_info.dart';
import 'package:hawkr_seller/features/report/presentation/block_list_screen.dart';
import 'package:hawkr_seller/features/report/presentation/widgets/block_actions.dart';

const _c6 =
    '封鎖後，對方將無法在你的貼文與討論區留言、按讚，也無法傳送訊息給你；對方仍看得到你公開的內容，評價不受影響。你可以隨時在「封鎖名單」解除封鎖。';

Future<void> _pump(
  WidgetTester tester,
  FakeTransportBuilder transport,
  Widget home,
) async {
  tester.view.devicePixelRatio = 1;
  tester.view.physicalSize = const Size(390, 844);
  addTearDown(tester.view.resetPhysicalSize);
  addTearDown(tester.view.resetDevicePixelRatio);
  await tester.pumpWidget(
    ProviderScope(
      overrides: [
        storeBlockRepositoryProvider.overrideWithValue(
          StoreBlockRepository(SellerAppServiceClient(transport.build())),
        ),
      ],
      child: ScreenUtilInit(
        designSize: const Size(390, 844),
        enableScaleWH: () => false,
        enableScaleText: () => false,
        builder: (_, _) => MaterialApp(theme: AppTheme.light, home: home),
      ),
    ),
  );
  await tester.pumpAndSettle();
}

class _Host extends ConsumerWidget {
  const _Host({this.commentId, this.forumEntryId, this.name = '小明'});

  final String name;
  final String? commentId;
  final String? forumEntryId;

  @override
  Widget build(BuildContext context, WidgetRef ref) => Scaffold(
    body: Center(
      child: TextButton(
        onPressed: () => blockBuyerWithConfirm(
          context,
          ref,
          buyerName: name,
          commentId: commentId,
          forumEntryId: forumEntryId,
        ),
        child: const Text('go'),
      ),
    ),
  );
}

class _ThrowingRepo implements StoreBlockRepository {
  @override
  Future<void> blockByComment(String commentId) async =>
      throw StateError('boom');

  @override
  dynamic noSuchMethod(Invocation invocation) => throw UnimplementedError();
}

pb.BlockedBuyer _blocked(String id, String name, {bool deleted = false}) =>
    pb.BlockedBuyer(
      buyer: pb.BuyerCard(
        id: id,
        deleted: deleted,
        displayName: name,
        avatar: common.Image(),
      ),
    );

void main() {
  setUpAll(() {
    if (!F.isInitialized) F.init(Flavor.dev);
  });

  for (final byComment in [true, false]) {
    testWidgets(
      'BlockBuyer carries ${byComment ? 'comment_id' : 'forum_entry_id'} after the C6 confirm',
      (tester) async {
        final requests = <pb.BlockBuyerRequest>[];
        final transport = FakeTransportBuilder().unary(
          SellerAppService.blockBuyer,
          (request, _) {
            requests.add(request);
            return pb.BlockBuyerResponse(changed: true);
          },
        );
        await _pump(
          tester,
          transport,
          byComment
              ? const _Host(commentId: 'c1')
              : const _Host(forumEntryId: 'f1'),
        );
        await tester.tap(find.text('go'));
        await tester.pumpAndSettle();
        expect(find.text('封鎖「小明」？'), findsOneWidget);
        expect(find.text(_c6), findsOneWidget);
        expect(requests, isEmpty);
        await tester.tap(find.text('封鎖'));
        await tester.pumpAndSettle();
        expect(requests, hasLength(1));
        final r = requests.single;
        if (byComment) {
          expect(r.whichSource(), pb.BlockBuyerRequest_Source.commentId);
          expect(r.commentId, 'c1');
        } else {
          expect(r.whichSource(), pb.BlockBuyerRequest_Source.forumEntryId);
          expect(r.forumEntryId, 'f1');
        }
        expect(find.text('已封鎖「小明」'), findsOneWidget);
        await tester.pump(const Duration(seconds: 5));
      },
    );
  }

  testWidgets(
    'block list pages with ListBlockedBuyers and unblocks by buyer_id',
    (tester) async {
      final listTokens = <String>[];
      final unblocked = <String>[];
      final transport = FakeTransportBuilder()
          .unary(SellerAppService.listBlockedBuyers, (request, _) {
            listTokens.add(request.pageToken);
            return request.pageToken.isEmpty
                ? pb.ListBlockedBuyersResponse(
                    blockedBuyers: [
                      for (var i = 0; i < 12; i++) _blocked('b$i', '買家$i'),
                    ],
                    nextPageToken: 'next',
                  )
                : pb.ListBlockedBuyersResponse(
                    blockedBuyers: [_blocked('b99', '買家99')],
                  );
          })
          .unary(SellerAppService.unblockBuyer, (request, _) {
            unblocked.add(request.buyerId);
            return pb.UnblockBuyerResponse(changed: true);
          });
      await _pump(tester, transport, const BlockListScreen());
      expect(listTokens, ['']);
      expect(find.text('買家0'), findsOneWidget);
      await tester.tap(find.text('解除封鎖').first);
      await tester.pumpAndSettle();
      await tester.tap(find.text('解除封鎖').last);
      await tester.pumpAndSettle();
      expect(unblocked, ['b0']);
      expect(find.text('買家0'), findsNothing);
      await tester.drag(find.byType(ListView), const Offset(0, -3000));
      await tester.pumpAndSettle();
      expect(listTokens, ['', 'next']);
      expect(find.text('買家99'), findsOneWidget);
    },
  );

  testWidgets('block list shows deleted buyer label', (tester) async {
    final transport = FakeTransportBuilder().unary(
      SellerAppService.listBlockedBuyers,
      (_, _) => pb.ListBlockedBuyersResponse(
        blockedBuyers: [_blocked('b1', '', deleted: true)],
      ),
    );
    await _pump(tester, transport, const BlockListScreen());
    expect(find.text(deletedBuyerLabel), findsOneWidget);
  });

  testWidgets('blank buyer name uses the shared fallback in title and toast', (
    tester,
  ) async {
    final transport = FakeTransportBuilder().unary(
      SellerAppService.blockBuyer,
      (_, _) => pb.BlockBuyerResponse(changed: true),
    );
    await _pump(tester, transport, const _Host(commentId: 'c1', name: ''));
    await tester.tap(find.text('go'));
    await tester.pumpAndSettle();
    expect(find.text('封鎖「此人」？'), findsOneWidget);
    await tester.tap(find.text('封鎖'));
    await tester.pumpAndSettle();
    expect(find.text('已封鎖「此人」'), findsOneWidget);
    await tester.pump(const Duration(seconds: 5));
  });

  testWidgets('non-Exception failure propagates instead of becoming a toast', (
    tester,
  ) async {
    Future<void>? pending;
    await tester.pumpWidget(
      ProviderScope(
        overrides: [
          storeBlockRepositoryProvider.overrideWithValue(_ThrowingRepo()),
        ],
        child: ScreenUtilInit(
          designSize: const Size(390, 844),
          builder: (_, _) => MaterialApp(
            theme: AppTheme.light,
            home: Consumer(
              builder: (context, ref, _) => Scaffold(
                body: TextButton(
                  onPressed: () => pending = blockBuyerWithConfirm(
                    context,
                    ref,
                    buyerName: '小明',
                    commentId: 'c1',
                  ),
                  child: const Text('go'),
                ),
              ),
            ),
          ),
        ),
      ),
    );
    await tester.tap(find.text('go'));
    final result = expectLater(pending!, throwsStateError);
    await tester.pumpAndSettle();
    await tester.tap(find.text('封鎖'));
    await tester.pumpAndSettle();
    await result;
    expect(find.text('已封鎖「小明」'), findsNothing);
  });

  testWidgets('empty first page with a token loads the next page', (
    tester,
  ) async {
    final tokens = <String>[];
    final transport = FakeTransportBuilder().unary(
      SellerAppService.listBlockedBuyers,
      (request, _) {
        tokens.add(request.pageToken);
        return request.pageToken.isEmpty
            ? pb.ListBlockedBuyersResponse(nextPageToken: 'next')
            : pb.ListBlockedBuyersResponse(
                blockedBuyers: [_blocked('b1', '買家1')],
              );
      },
    );
    await _pump(tester, transport, const BlockListScreen());
    expect(tokens, ['', 'next']);
    expect(find.text('買家1'), findsOneWidget);
    expect(
      find.byKey(const ValueKey('seller-block-list-empty-state')),
      findsNothing,
    );
  });

  testWidgets('short first page that does not scroll loads the next page', (
    tester,
  ) async {
    final tokens = <String>[];
    final transport = FakeTransportBuilder().unary(
      SellerAppService.listBlockedBuyers,
      (request, _) {
        tokens.add(request.pageToken);
        return request.pageToken.isEmpty
            ? pb.ListBlockedBuyersResponse(
                blockedBuyers: [_blocked('b1', '買家1')],
                nextPageToken: 'next',
              )
            : pb.ListBlockedBuyersResponse(
                blockedBuyers: [_blocked('b2', '買家2')],
              );
      },
    );
    await _pump(tester, transport, const BlockListScreen());
    expect(tokens, ['', 'next']);
    expect(find.text('買家2'), findsOneWidget);
  });
}
