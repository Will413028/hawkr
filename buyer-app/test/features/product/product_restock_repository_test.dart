import 'package:hawkr_api/gen/hawkr/buyer/v1alpha1/buyer.connect.client.dart';
import 'package:hawkr_api/gen/hawkr/buyer/v1alpha1/buyer.connect.spec.dart';
import 'package:hawkr_api/gen/hawkr/buyer/v1alpha1/buyer.pb.dart' as buyer;
import 'package:hawkr_api/testing.dart';
import 'package:hawkr_api/transport.dart' show Code, ConnectException;
import 'package:hawkr_buyer/features/product/data/product_repository.dart';
import 'package:dio/dio.dart';
import 'package:flutter_test/flutter_test.dart';

void main() {
  for (final fail in [false, true]) {
    test(
      'restock mutations ${fail ? 'fail closed' : 'send the exact product and SKU IDs'}',
      () async {
        final ids = <String>[];
        final transport = FakeTransportBuilder()
            .unary(BuyerAppService.subscribeRestock, (request, _) {
              ids.add(request.productId);
              ids.add(request.skuId);
              if (fail) throw ConnectException(Code.unavailable, 'offline');
              return buyer.SubscribeRestockResponse();
            })
            .unary(BuyerAppService.unsubscribeRestock, (request, _) {
              ids.add(request.skuId);
              if (fail) throw ConnectException(Code.unavailable, 'offline');
              return buyer.UnsubscribeRestockResponse();
            })
            .build();
        final dio = Dio();
        addTearDown(dio.close);
        final repo = ProductRepository(dio, BuyerAppServiceClient(transport));

        const product = '0190a1b2-c3d4-7e5f-8a6b-7c8d9e0f1a2b';
        const sku = '0190a1b2-c3d4-7e5f-8a6b-7c8d9e0f1a2c';
        expect(await repo.subscribeRestock('bad-id', sku), isFalse);
        expect(
          await repo.subscribeRestock(product, '871936508549696815'),
          isFalse,
        );
        expect(await repo.unsubscribeRestock('bad-id'), isFalse);
        expect(await repo.unsubscribeRestock(''), isFalse);
        expect(await repo.subscribeRestock(product, sku), !fail);
        expect(await repo.unsubscribeRestock(sku), !fail);
        expect(ids, [product, sku, sku]);
      },
    );
  }
}
