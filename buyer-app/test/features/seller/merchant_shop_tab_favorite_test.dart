import 'dart:async';
import 'dart:convert';
import 'dart:typed_data';

import 'package:hawkr_buyer/core/networking/dio_provider.dart';
import 'package:hawkr_buyer/core/session/session_model.dart';
import 'package:hawkr_buyer/core/session/session_provider.dart';
import 'package:hawkr_buyer/features/seller/presentation/widgets/merchant_shop_tab.dart';
import 'package:hawkr_buyer/features/visit/data/visit_repository.dart';
import 'package:dio/dio.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:hooks_riverpod/hooks_riverpod.dart';

class _SignedInSession extends Session {
  @override
  Future<SessionModel?> build() async =>
      const SessionModel(userId: 'buyer-1', nickname: 'Test Buyer');
}

/// 批次檢查停在 [pendingCheck]，讓測試在請求進行中操作收藏。
class _PendingFavoritesRepository extends Fake implements VisitRepository {
  final pendingCheck = Completer<Set<int>>();
  final writes = <String>[];
  var checks = 0;

  @override
  Future<Set<int>> batchCheckFavorites(Iterable<int> productIds) {
    checks++;
    return pendingCheck.future;
  }

  @override
  Future<void> addFavorite(int productId) async => writes.add('+$productId');

  @override
  Future<void> removeFavorite(int productId) async => writes.add('-$productId');
}

class _EmptyAdapter implements HttpClientAdapter {
  @override
  Future<ResponseBody> fetch(
    RequestOptions options,
    Stream<Uint8List>? requestStream,
    Future<void>? cancelFuture,
  ) async => ResponseBody.fromString(
    jsonEncode(const {'code': 0, 'message': 'success', 'data': {}}),
    200,
    headers: {
      Headers.contentTypeHeader: [Headers.jsonContentType],
    },
  );

  @override
  void close({bool force = false}) {}
}

void main() {
  testWidgets(
    'a favorite check still in flight does not undo the buyer\'s local change',
    (tester) async {
      tester.view.devicePixelRatio = 1;
      tester.view.physicalSize = const Size(390, 900);
      addTearDown(tester.view.resetDevicePixelRatio);
      addTearDown(tester.view.resetPhysicalSize);
      final dio = Dio(BaseOptions(baseUrl: 'https://test.local'))
        ..httpClientAdapter = _EmptyAdapter();
      addTearDown(dio.close);
      final favorites = _PendingFavoritesRepository();
      final container = ProviderContainer(
        overrides: [
          dioProvider.overrideWithValue(dio),
          sessionProvider.overrideWith(_SignedInSession.new),
          visitRepositoryProvider.overrideWithValue(favorites),
        ],
      );
      addTearDown(container.dispose);
      await container.read(sessionProvider.future);

      await tester.pumpWidget(
        UncontrolledProviderScope(
          container: container,
          child: Consumer(
            builder: (context, ref, _) => MaterialApp(
              home: Scaffold(
                body: MerchantShopTab(
                  ref: ref,
                  merchantId: '2683382637',
                  merchantName: '南畝王生技',
                  prefetchedProducts: const [
                    {'id': '1', 'name': '高麗菜', 'priceMin': '500'},
                    {'id': '2', 'name': '紅豆', 'priceMin': '195'},
                  ],
                  prefetchedReady: true,
                ),
              ),
            ),
          ),
        ),
      );
      await tester.pump();

      MerchantShopCard card(String sku) => tester.widget<MerchantShopCard>(
        find.byWidgetPredicate((w) => w is MerchantShopCard && w.sku == sku),
      );

      // 檢查還沒回來：使用者先收藏再取消收藏商品 1。
      expect(favorites.checks, 1);
      card('1').onToggleFavorite();
      await tester.pump();
      expect(card('1').isFav, isTrue);
      card('1').onToggleFavorite();
      await tester.pump();
      expect(card('1').isFav, isFalse);
      expect(favorites.writes, ['+1', '-1']);

      // 回應是在使用者取消之前讀到的伺服器狀態：1、2 都有收藏。
      favorites.pendingCheck.complete({1, 2});
      await tester.pump();
      await tester.pump();

      expect(card('1').isFav, isFalse);
      expect(card('2').isFav, isTrue);
      expect(tester.takeException(), isNull);
    },
  );
}
