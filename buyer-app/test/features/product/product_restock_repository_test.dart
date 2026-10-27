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
      'restock mutations ${fail ? 'fail closed' : 'send the exact product ID'}',
      () async {
        final ids = <String>[];
        final transport = FakeTransportBuilder()
            .unary(BuyerAppService.subscribeRestock, (request, _) {
              ids.add(request.productId);
              if (fail) throw ConnectException(Code.unavailable, 'offline');
              return buyer.SubscribeRestockResponse();
            })
            .unary(BuyerAppService.unsubscribeRestock, (request, _) {
              ids.add(request.productId);
              if (fail) throw ConnectException(Code.unavailable, 'offline');
              return buyer.UnsubscribeRestockResponse();
            })
            .build();
        final dio = Dio();
        addTearDown(dio.close);
        final repo = ProductRepository(dio, BuyerAppServiceClient(transport));

        expect(await repo.subscribeRestock('bad-id'), isFalse);
        expect(await repo.unsubscribeRestock('bad-id'), isFalse);
        expect(await repo.subscribeRestock('871936508549696815'), !fail);
        expect(await repo.unsubscribeRestock('871936508549696815'), !fail);
        expect(ids, ['871936508549696815', '871936508549696815']);
      },
    );
  }
}
