import 'dart:io';

import 'package:hawkr_seller/core/router/app_router.dart';
import 'package:hawkr_seller/features/seller/presentation/profile_other_options_card.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_screenutil/flutter_screenutil.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:go_router/go_router.dart';

/// 「一個真實賣家，從**哪個畫面**、按**哪顆按鈕**，會看到等他報價的詢價？」
///
/// A14 之前的答案是「沒有」：回價表單與六支端點都做好了、路由也掛上了，但沒有
/// 任何畫面 navigate 過去，`listPublications()` 零消費端——只有手打 URL 帶
/// `publicationId` 才進得去。只有 route 註冊、只有畫面元件，都不算完成。
void main() {
  group('入口：設定 → 營運入口 → 待回覆詢價', () {
    testWidgets('列出「待回覆詢價」，點下去導到 /procurement/publications', (tester) async {
      String? pushedLocation;

      final router = GoRouter(
        initialLocation: '/settings',
        routes: [
          GoRoute(
            path: '/settings',
            builder: (_, _) => Scaffold(
              body: SingleChildScrollView(
                child: ProfileOtherOptionsCard(onShippingTap: () {}),
              ),
            ),
          ),
          GoRoute(
            path: ProcurementRoutes.sellerPublications,
            builder: (_, state) {
              pushedLocation = state.uri.toString();
              return const Scaffold(body: Text('publication-list-stub'));
            },
          ),
        ],
      );

      await tester.pumpWidget(
        ProviderScope(
          child: ScreenUtilInit(
            designSize: const Size(414, 896),
            builder: (_, _) => MaterialApp.router(routerConfig: router),
          ),
        ),
      );
      await tester.pumpAndSettle();

      expect(find.text('待回覆詢價'), findsOneWidget);

      await tester.ensureVisible(find.text('待回覆詢價'));
      await tester.tap(find.text('待回覆詢價'));
      await tester.pumpAndSettle();

      expect(pushedLocation, ProcurementRoutes.sellerPublications);
      expect(find.text('publication-list-stub'), findsOneWidget);
    });
  });

  group('route 已註冊，而且清單真的接到表單', () {
    final routerSource = File(
      'lib/core/router/app_router.dart',
    ).readAsStringSync();

    test('兩條採購線路徑都接到 GoRouter 上', () {
      expect(
        routerSource,
        contains('path: ProcurementRoutes.sellerPublications'),
      );
      expect(routerSource, contains('path: ProcurementRoutes.sellerQuote'));
    });

    test('清單畫面導向回價表單時帶 publicationId', () {
      final listSource = File(
        'lib/features/procurement/presentation/'
        'seller_publication_list_screen.dart',
      ).readAsStringSync();

      expect(listSource, contains('ProcurementRoutes.sellerQuote'));
      expect(listSource, contains('publicationId='));
    });

    test('⛔ 清單不得為了補狀態而逐列打詳情端點', () {
      // 端點 limit 上限 100，逐列打詳情＝一次打開最多 100 次往返。
      // 清單列拿不到草稿／回價狀態是後端契約的形狀，正解是後端補欄位。
      final listSource = File(
        'lib/features/procurement/presentation/'
        'seller_publication_list_screen.dart',
      ).readAsStringSync();
      final controllerSource = File(
        'lib/features/procurement/presentation/'
        'seller_publication_list_controller.dart',
      ).readAsStringSync();

      for (final source in [listSource, controllerSource]) {
        expect(source, isNot(contains('getPublicationDetail')));
        expect(source, isNot(contains('sellerQuoteFormProvider')));
      }
    });

    test('⛔ 入口這條線不下任何寫入命令', () {
      final listSource = File(
        'lib/features/procurement/presentation/'
        'seller_publication_list_screen.dart',
      ).readAsStringSync();

      for (final command in const [
        'saveDraft',
        'submitResponse',
        'withdrawResponse',
        'declinePublication',
      ]) {
        expect(
          listSource,
          isNot(contains(command)),
          reason: '清單是入口，$command 屬於回價表單。',
        );
      }
    });
  });
}
