import 'package:hawkr_api/gen/hawkr/buyer/v1alpha1/buyer.connect.client.dart';
import 'package:hawkr_api/gen/hawkr/buyer/v1alpha1/buyer.connect.spec.dart';
import 'package:hawkr_api/gen/hawkr/buyer/v1alpha1/buyer.pb.dart' as buyer;
import 'package:hawkr_api/testing.dart';
import 'package:hawkr_buyer/features/follow/data/follow_repository.dart';
import 'package:dio/dio.dart';
import 'package:flutter_test/flutter_test.dart';

void main() {
  test(
    'followStore sends the exact store ID through the generated client',
    () async {
      buyer.FollowStoreRequest? received;
      final transport = FakeTransportBuilder().unary(
        BuyerAppService.followStore,
        (request, _) {
          received = request;
          return buyer.FollowStoreResponse();
        },
      ).build();
      final dio = Dio();
      addTearDown(dio.close);

      await FollowRepository(
        dio,
        BuyerAppServiceClient(transport),
      ).follow(2675237784);

      expect(received?.storeId, '2675237784');
    },
  );
}
