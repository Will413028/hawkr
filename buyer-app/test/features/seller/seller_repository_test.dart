import 'package:dio/dio.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:hawkr_api/gen/hawkr/common/v1alpha1/store_card.pb.dart';
import 'package:hawkr_api/gen/hawkr/seller/v1alpha1/seller.connect.client.dart';
import 'package:hawkr_api/gen/hawkr/seller/v1alpha1/seller.connect.spec.dart';
import 'package:hawkr_api/gen/hawkr/seller/v1alpha1/seller.pb.dart' as seller;
import 'package:hawkr_api/testing.dart';
import 'package:hawkr_api/transport.dart' show Code, ConnectException;
import 'package:hawkr_buyer/features/follow/data/follow_repository.dart';
import 'package:hawkr_buyer/features/seller/data/seller_repository.dart';
import 'package:mocktail/mocktail.dart';

class _MockFollowRepository extends Mock implements FollowRepository {}

SellerRepository _repository(
  FakeTransportBuilder builder, {
  FakeTransportBuilder? withSession,
}) {
  final dio = Dio();
  addTearDown(dio.close);
  return SellerRepository(
    dio,
    _MockFollowRepository(),
    SellerPublicServiceClient(builder.build()),
    SellerPublicServiceClient((withSession ?? builder).build()),
  );
}

void main() {
  test('getStore reads by store code', () async {
    seller.GetStoreRequest? received;
    final repository = _repository(
      FakeTransportBuilder().unary(SellerPublicService.getStore, (request, _) {
        received = request;
        return seller.GetStoreResponse(
          store: seller.PublicStore(id: '9', storeCode: 'jinhu'),
        );
      }),
    );

    final response = await repository.getStore('jinhu');

    expect(received?.storeCode, 'jinhu');
    expect(response.store.id, '9');
  });

  test('getStore lets a not found error through', () async {
    final repository = _repository(
      FakeTransportBuilder().unary(
        SellerPublicService.getStore,
        (_, __) => throw ConnectException(Code.notFound, 'missing'),
      ),
    );

    await expectLater(
      repository.getStore('gone'),
      throwsA(
        isA<ConnectException>().having((e) => e.code, 'code', Code.notFound),
      ),
    );
  });

  test('batchGetStores dedupes ids, drops blanks and splits at 100', () async {
    final requests = <List<String>>[];
    final repository = _repository(
      FakeTransportBuilder().unary(SellerPublicService.batchGetStores, (
        request,
        _,
      ) {
        requests.add(List.of(request.ids));
        return seller.BatchGetStoresResponse(
          stores: [for (final id in request.ids) StoreCard(id: id)],
        );
      }),
    );
    final ids = [for (var i = 1; i <= 101; i++) '$i', '1', ' ', ''];

    final cards = await repository.batchGetStores(ids);

    expect(requests.map((r) => r.length), [100, 1]);
    expect(cards.length, 101);
    expect(cards['101']?.id, '101');
  });

  test('batchGetStores with no ids sends nothing', () async {
    final repository = _repository(FakeTransportBuilder());

    expect(await repository.batchGetStores(const ['', ' ']), isEmpty);
  });

  test('recordProductView sends the product id', () async {
    seller.RecordProductViewRequest? received;
    final repository = _repository(
      FakeTransportBuilder(),
      withSession: FakeTransportBuilder().unary(
        SellerPublicService.recordProductView,
        (request, _) {
          received = request;
          return seller.RecordProductViewResponse();
        },
      ),
    );

    await repository.recordProductView('88');

    expect(received?.productId, '88');
  });

  test('recordProductView swallows resource_exhausted', () async {
    final repository = _repository(
      FakeTransportBuilder(),
      withSession: FakeTransportBuilder().unary(
        SellerPublicService.recordProductView,
        (_, __) => throw ConnectException(Code.resourceExhausted, 'slow down'),
      ),
    );

    await repository.recordProductView('88');
  });

  test('recordProductView ignores a blank product id', () async {
    final repository = _repository(FakeTransportBuilder());

    await repository.recordProductView(' ');
  });
}
