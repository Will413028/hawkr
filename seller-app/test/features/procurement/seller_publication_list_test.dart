import 'dart:async';

import 'package:hawkr_seller/core/router/app_router.dart';
import 'package:hawkr_seller/features/procurement/data/procurement_error.dart';
import 'package:hawkr_seller/features/procurement/data/seller_quote_model.dart';
import 'package:hawkr_seller/features/procurement/data/seller_quote_repository.dart';
import 'package:hawkr_seller/features/procurement/presentation/seller_publication_list_controller.dart';
import 'package:hawkr_seller/features/procurement/presentation/seller_publication_list_screen.dart';
import 'package:hawkr_seller/shared/widgets/hk_empty_state.dart';
import 'package:hawkr_seller/shared/widgets/hk_status_badge.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_screenutil/flutter_screenutil.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:go_router/go_router.dart';
import 'package:mocktail/mocktail.dart';

class _MockRepo extends Mock implements SellerQuoteRepository {}

final _now = DateTime(2026, 9, 14, 15, 30);

const _emptyStateKey = ValueKey('seller-publications-empty-state');
const _deadlineBadgeKey = ValueKey('seller-publication-deadline-badge');
const _statusBadgeKey = ValueKey('seller-publication-response-status-badge');
const _draftBadgeKey = ValueKey('seller-publication-draft-badge');

SellerPublicationListItem _row({
  String publicationId = 'pub-1',
  String item = '高麗菜',
  String itemLabel = '梨山高麗菜',
  String quantity = '200',
  String unit = 'BOX',
  String deliveryCity = '臺北市',
  String deliveryDate = '2026-09-20',
  String responseDeadline = '2026-09-18',
  String latestResponseStatus = '',
  bool hasDraft = false,
}) {
  return SellerPublicationListItem(
    summary: SellerPublicationSummary(
      publication: SellerFacingPublication(
        publicationId: publicationId,
        publicationVersion: '4',
        inviteId: 'inv-1',
        lane: 'PLATFORM',
        sellerFacingPayloadHash: 'hash-4',
        deliveryState: 'SENT',
        publishedAt: '2026-09-13T02:00:00Z',
      ),
      sellerFacingPayload: SellerFacingPayload(
        item: item,
        itemLabel: itemLabel,
        quantity: quantity,
        unit: unit,
        packaging: '紙箱',
        deliveryCity: deliveryCity,
        deliveryDate: deliveryDate,
        responseDeadline: responseDeadline,
      ),
    ),
    latestResponseStatus: latestResponseStatus,
    hasDraft: hasDraft,
  );
}

String? _badgeLabel(WidgetTester tester, Key key) {
  final found = tester.widgetList<HkStatusBadge>(find.byKey(key));
  return found.isEmpty ? null : found.first.label;
}

void main() {
  String? pushedQuoteLocation;

  Future<void> pump(WidgetTester tester, SellerQuoteRepository repo) async {
    pushedQuoteLocation = null;
    final router = GoRouter(
      initialLocation: ProcurementRoutes.sellerPublications,
      routes: [
        GoRoute(
          path: ProcurementRoutes.sellerPublications,
          builder: (_, _) => SellerPublicationListScreen(now: _now),
        ),
        GoRoute(
          path: ProcurementRoutes.sellerQuote,
          builder: (_, state) {
            pushedQuoteLocation = state.uri.toString();
            return const Scaffold(body: Text('quote-form-stub'));
          },
        ),
      ],
    );

    await tester.pumpWidget(
      ProviderScope(
        overrides: [sellerQuoteRepositoryProvider.overrideWithValue(repo)],
        child: ScreenUtilInit(
          designSize: const Size(414, 896),
          builder: (_, _) => MaterialApp.router(routerConfig: router),
        ),
      ),
    );
  }

  _MockRepo repoReturning(List<SellerPublicationListItem> rows) {
    final repo = _MockRepo();
    when(repo.listPublications).thenAnswer((_) async => rows);
    return repo;
  }

  group('清單三態', () {
    testWidgets('loading：清單回來之前不先閃一次空狀態', (tester) async {
      final completer = Completer<List<SellerPublicationListItem>>();
      final repo = _MockRepo();
      when(repo.listPublications).thenAnswer((_) => completer.future);

      await pump(tester, repo);
      await tester.pump();

      expect(find.byType(CircularProgressIndicator), findsOneWidget);
      expect(find.byKey(_emptyStateKey), findsNothing);

      completer.complete([_row()]);
      await tester.pumpAndSettle();
      expect(find.byType(CircularProgressIndicator), findsNothing);
    });

    testWidgets('error：顯示失敗原因，不假裝成空清單', (tester) async {
      final repo = _MockRepo();
      when(
        repo.listPublications,
      ).thenThrow(const ProcurementException(message: '目前無法使用回價功能，請稍後再試。'));

      await pump(tester, repo);
      await tester.pumpAndSettle();

      expect(find.textContaining('目前無法使用回價功能'), findsOneWidget);
      expect(find.byKey(_emptyStateKey), findsNothing);
      expect(find.text('重新載入'), findsOneWidget);
    });

    testWidgets('error → 重新載入：再打一次端點並畫出清單', (tester) async {
      var calls = 0;
      final repo = _MockRepo();
      when(repo.listPublications).thenAnswer((_) async {
        calls++;
        if (calls == 1) {
          throw const ProcurementException(message: '暫時取不到詢價清單');
        }
        return [_row()];
      });

      await pump(tester, repo);
      await tester.pumpAndSettle();
      expect(find.textContaining('暫時取不到詢價清單'), findsOneWidget);

      await tester.tap(find.text('重新載入'));
      await tester.pumpAndSettle();

      expect(calls, 2);
      expect(find.text('梨山高麗菜'), findsOneWidget);
    });

    testWidgets('empty：誠實的空狀態，不是一片空白也不是無限轉圈', (tester) async {
      await pump(tester, repoReturning(const []));
      await tester.pumpAndSettle();

      expect(find.byType(CircularProgressIndicator), findsNothing);
      final empty = tester.widget<HkEmptyState>(find.byKey(_emptyStateKey));
      expect(empty.message, '目前沒有等你回價的詢價');
      expect(empty.description, isNotNull);
    });
  });

  group('清單列', () {
    testWidgets('顯示品項與需求內容', (tester) async {
      await pump(tester, repoReturning([_row()]));
      await tester.pumpAndSettle();

      expect(find.text('梨山高麗菜'), findsOneWidget);
      expect(find.text('需求數量 200 BOX'), findsOneWidget);
      expect(find.text('交貨縣市 臺北市'), findsOneWidget);
      expect(find.text('希望交貨日 2026-09-20'), findsOneWidget);
      expect(find.text('回覆期限 2026-09-18'), findsOneWidget);
      expect(find.textContaining('共 1 則'), findsOneWidget);
    });

    testWidgets('沒有品項也畫得出來，不會變成一張沒有抬頭的空卡', (tester) async {
      await pump(
        tester,
        repoReturning([_row(item: '', itemLabel: '', quantity: '', unit: '')]),
      );
      await tester.pumpAndSettle();

      expect(find.text('未填品項的詢價'), findsOneWidget);
      expect(find.textContaining('需求數量'), findsNothing);
    });

    testWidgets('點一列導到既有的回價表單，並帶上 publicationId', (tester) async {
      await pump(tester, repoReturning([_row(publicationId: 'pub-7')]));
      await tester.pumpAndSettle();

      await tester.tap(find.text('梨山高麗菜'));
      await tester.pumpAndSettle();

      expect(
        pushedQuoteLocation,
        '${ProcurementRoutes.sellerQuote}?publicationId=pub-7',
      );
      expect(find.text('quote-form-stub'), findsOneWidget);
    });

    testWidgets('缺識別碼的那一列點不開，而且明講點不開', (tester) async {
      await pump(tester, repoReturning([_row(publicationId: '')]));
      await tester.pumpAndSettle();

      expect(find.textContaining('缺少識別碼'), findsOneWidget);

      await tester.tap(find.text('梨山高麗菜'));
      await tester.pumpAndSettle();

      expect(pushedQuoteLocation, isNull);
    });
  });

  group('處理進度', () {
    testWidgets('⛔ 空字串是「還沒回價」，不是「未知」也不是不顯示', (tester) async {
      await pump(tester, repoReturning([_row()]));
      await tester.pumpAndSettle();

      expect(_badgeLabel(tester, _statusBadgeKey), '還沒回價');
      expect(find.textContaining('未知'), findsNothing);
      expect(find.text('—'), findsNothing);
      expect(find.byKey(_draftBadgeKey), findsNothing);
    });

    testWidgets('已提交 ACTIVE 就顯示已回價', (tester) async {
      await pump(tester, repoReturning([_row(latestResponseStatus: 'ACTIVE')]));
      await tester.pumpAndSettle();

      expect(_badgeLabel(tester, _statusBadgeKey), '已回價');
      expect(find.byKey(_draftBadgeKey), findsNothing);
    });

    testWidgets('🔴 已提交＋又存了新草稿：兩個標籤同時出現，不是二選一', (tester) async {
      await pump(
        tester,
        repoReturning([_row(latestResponseStatus: 'ACTIVE', hasDraft: true)]),
      );
      await tester.pumpAndSettle();

      expect(_badgeLabel(tester, _statusBadgeKey), '已回價');
      expect(_badgeLabel(tester, _draftBadgeKey), '草稿未送出');
    });

    testWidgets('還沒回價＋有草稿：也是兩個標籤，草稿不會蓋掉「還沒回價」', (tester) async {
      await pump(tester, repoReturning([_row(hasDraft: true)]));
      await tester.pumpAndSettle();

      expect(_badgeLabel(tester, _statusBadgeKey), '還沒回價');
      expect(_badgeLabel(tester, _draftBadgeKey), '草稿未送出');
    });

    testWidgets('四個值域各有自己的賣家視角文案，不用 CodeMeta 的營運 label', (tester) async {
      const cases = <String, String>{
        'ACTIVE': '已回價',
        'SUPERSEDED': '已被新版取代',
        'WITHDRAWN': '已撤回',
        'EXPIRED': '報價已過期',
      };
      for (final entry in cases.entries) {
        await pump(
          tester,
          repoReturning([_row(latestResponseStatus: entry.key)]),
        );
        await tester.pumpAndSettle();
        expect(
          _badgeLabel(tester, _statusBadgeKey),
          entry.value,
          reason: '${entry.key} 的文案不對',
        );
      }
    });

    testWidgets('🔴 認不得的新 code 仍然是「已回價」，不得退回「還沒回價」', (tester) async {
      await pump(
        tester,
        repoReturning([_row(latestResponseStatus: 'SOMETHING_NEW')]),
      );
      await tester.pumpAndSettle();

      expect(_badgeLabel(tester, _statusBadgeKey), '已回價（請更新 App 看狀態）');
    });

    testWidgets('抬頭數還沒回價的則數，草稿不算回過價', (tester) async {
      await pump(
        tester,
        repoReturning([
          _row(publicationId: 'pub-1', latestResponseStatus: 'ACTIVE'),
          _row(publicationId: 'pub-2', hasDraft: true),
          _row(publicationId: 'pub-3'),
        ]),
      );
      await tester.pumpAndSettle();

      expect(find.text('共 3 則，其中 2 則還沒回價。'), findsOneWidget);
    });

    testWidgets('全部回過價就不再說還剩幾則', (tester) async {
      await pump(
        tester,
        repoReturning([
          _row(publicationId: 'pub-1', latestResponseStatus: 'ACTIVE'),
          _row(publicationId: 'pub-2', latestResponseStatus: 'WITHDRAWN'),
        ]),
      );
      await tester.pumpAndSettle();

      expect(find.text('共 2 則，都已經回過價了。'), findsOneWidget);
    });
  });

  group('回覆期限', () {
    Future<String?> badgeLabel(WidgetTester tester, String deadline) async {
      await pump(tester, repoReturning([_row(responseDeadline: deadline)]));
      await tester.pumpAndSettle();
      return _badgeLabel(tester, _deadlineBadgeKey);
    }

    testWidgets('過期看得出來', (tester) async {
      expect(await badgeLabel(tester, '2026-09-13'), '已過回覆期限');
    });

    testWidgets('期限當天是「今天截止」不是「已過期」', (tester) async {
      expect(await badgeLabel(tester, '2026-09-14'), '今天截止');
    });

    testWidgets('還沒到就顯示剩幾天', (tester) async {
      expect(await badgeLabel(tester, '2026-09-18'), '剩 4 天');
    });

    testWidgets('⛔ 舊資料沒有期限：整個不顯示，不補假日期也不當成已過期', (tester) async {
      expect(await badgeLabel(tester, ''), isNull);
    });

    testWidgets('⛔ 沒有期限時連「回覆期限」那一行都不出現', (tester) async {
      await pump(tester, repoReturning([_row(responseDeadline: '')]));
      await tester.pumpAndSettle();

      expect(find.textContaining('回覆期限'), findsNothing);
      expect(find.textContaining('已過回覆期限'), findsNothing);
      expect(find.byKey(_deadlineBadgeKey), findsNothing);
      expect(find.text('梨山高麗菜'), findsOneWidget);
      expect(_badgeLabel(tester, _statusBadgeKey), '還沒回價');
    });
  });

  group('classifyResponseDeadline', () {
    test('空字串與認不得的字串一律 unknown，不猜日期', () {
      for (final raw in const [
        '',
        '   ',
        'null',
        '待確認',
        '2026-13-45',
        '2026-02-31',
      ]) {
        final result = classifyResponseDeadline(raw, now: _now);
        expect(
          result.state,
          SellerPublicationDeadlineState.unknown,
          reason: '「$raw」被判成了 ${result.state}',
        );
        expect(result.isKnown, isFalse);
        expect(result.date, isNull);
      }
    });

    test('昨天＝已過期，今天＝當天，明天以後＝剩幾天', () {
      expect(
        classifyResponseDeadline('2026-09-13', now: _now).state,
        SellerPublicationDeadlineState.overdue,
      );

      final today = classifyResponseDeadline('2026-09-14', now: _now);
      expect(today.state, SellerPublicationDeadlineState.dueToday);
      expect(today.daysLeft, 0);
      expect(today.date, DateTime(2026, 9, 14));

      final soon = classifyResponseDeadline('2026-09-15', now: _now);
      expect(soon.state, SellerPublicationDeadlineState.upcoming);
      expect(soon.daysLeft, 1);
    });

    test('跨月與跨年也照日曆算', () {
      expect(classifyResponseDeadline('2026-10-01', now: _now).daysLeft, 17);
      expect(classifyResponseDeadline('2027-01-01', now: _now).daysLeft, 109);
    });

    test('帶時間的 RFC3339 也吃得下，判的仍然是「哪一天」', () {
      final result = classifyResponseDeadline(
        '2026-09-14T23:59:00Z',
        now: _now,
      );
      expect(result.isKnown, isTrue);
      expect(result.state, isNot(SellerPublicationDeadlineState.unknown));
    });
  });
}
