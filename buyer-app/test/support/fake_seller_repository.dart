import 'package:hawkr_api/gen/hawkr/common/v1alpha1/store_card.pb.dart';
import 'package:hawkr_buyer/features/seller/data/seller_repository.dart';
import 'package:mocktail/mocktail.dart';

class _MockSellerRepository extends Mock implements SellerRepository {}

/// 只會回 [cards] 的 SellerRepository。[batches] 記下每次 BatchGetStores 的 id。
SellerRepository fakeSellerRepository({
  Iterable<StoreCard> cards = const [],
  List<List<String>>? batches,
}) {
  final repository = _MockSellerRepository();
  final byId = {for (final card in cards) card.id: card};
  when(() => repository.batchGetStores(any())).thenAnswer((invocation) async {
    final ids = (invocation.positionalArguments.first as Iterable<String>)
        .toList();
    batches?.add(ids);
    return {
      for (final id in ids)
        if (byId.containsKey(id)) id: byId[id]!,
    };
  });
  when(() => repository.recordProductView(any())).thenAnswer((_) async {});
  return repository;
}
