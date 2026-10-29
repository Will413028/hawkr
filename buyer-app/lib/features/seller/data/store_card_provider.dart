import 'dart:async';

import 'package:hawkr_api/gen/hawkr/common/v1alpha1/store_card.pb.dart';
import 'package:riverpod_annotation/riverpod_annotation.dart';

import 'seller_repository.dart';

part 'store_card_provider.g.dart';

/// 把同一個同步區段（例如同一次 build）裡要的店家卡片合成一次 BatchGetStores。
class StoreCardLoader {
  StoreCardLoader(this._repository);

  final SellerRepository _repository;
  final Map<String, Completer<StoreCard?>> _pending = {};
  bool _flushScheduled = false;

  /// 沒有卡片（從沒公開過、不存在）時是 null；讀取失敗時丟出錯誤（呼叫端與 provider 把它當作「沒讀到」，
  /// 不是「沒有卡片」，錯誤不會被當成 null 快取）。
  Future<StoreCard?> load(String storeId) {
    final id = storeId.trim();
    if (id.isEmpty) return Future.value();
    final waiting = _pending[id];
    if (waiting != null) return waiting.future;
    final completer = Completer<StoreCard?>();
    _pending[id] = completer;
    if (!_flushScheduled) {
      _flushScheduled = true;
      scheduleMicrotask(_flush);
    }
    return completer.future;
  }

  Future<void> _flush() async {
    final batch = Map<String, Completer<StoreCard?>>.of(_pending);
    _pending.clear();
    _flushScheduled = false;
    try {
      final cards = await _repository.batchGetStores(batch.keys);
      for (final MapEntry(:key, :value) in batch.entries) {
        value.complete(cards[key]);
      }
    } catch (error, stackTrace) {
      for (final completer in batch.values) {
        completer.completeError(error, stackTrace);
      }
    }
  }
}

@riverpod
StoreCardLoader storeCardLoader(Ref ref) =>
    StoreCardLoader(ref.watch(sellerRepositoryProvider));

/// 一家店的卡片，同一個畫面同時要的店家共用一次批次讀取。
///
/// 沒有卡片是資料 null；讀取失敗是 AsyncError（不快取成 null），`ref.invalidate` 或下一次重新建立時重讀。
@riverpod
Future<StoreCard?> storeCard(Ref ref, String storeId) =>
    ref.watch(storeCardLoaderProvider).load(storeId);
