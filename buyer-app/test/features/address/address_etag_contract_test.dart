import 'package:hawkr_api/gen/hawkr/buyer/v1alpha1/buyer.connect.client.dart';
import 'package:hawkr_api/gen/hawkr/buyer/v1alpha1/buyer.connect.spec.dart';
import 'package:hawkr_api/gen/hawkr/buyer/v1alpha1/buyer.pb.dart' as buyer;
import 'package:hawkr_api/gen/hawkr/common/v1alpha1/postal_address.pb.dart'
    as common;
import 'package:hawkr_api/testing.dart';
import 'package:hawkr_api/transport.dart' show Code, ConnectException;
import 'package:hawkr_buyer/core/exceptions/app_exception.dart';
import 'package:hawkr_buyer/features/address/data/address_model.dart';
import 'package:hawkr_buyer/features/address/data/address_repository.dart';
import 'package:flutter_test/flutter_test.dart';
import '../../support/default_error_text.dart';

void main() {
  test(
    'stale form etag reaches the write and the conflict reaches the caller',
    () async {
      buyer.UpdateAddressRequest? sent;
      var readCount = 0;
      final client = BuyerAppServiceClient(
        FakeTransportBuilder()
            .unary(BuyerAppService.getAddress, (_, _) {
              readCount++;
              return buyer.GetAddressResponse(
                address: buyer.Address(id: '12', etag: 'v2'),
              );
            })
            .unary(BuyerAppService.updateAddress, (request, _) {
              sent = request;
              throw ConnectException(Code.aborted, '版本衝突');
            })
            .build(),
      );
      await expectLater(
        AddressRepository(
          client,
          defaults: zhDefaultErrorText,
        ).updateAddress(12, etag: 'v1', receiverName: '甲'),
        throwsA(isA<ServerException>()),
      );
      expect(readCount, 0);
      expect(sent?.address.etag, 'v1');
    },
  );

  test(
    'address call canceled: internal default text, no hand-written sentence',
    () async {
      final client = BuyerAppServiceClient(
        FakeTransportBuilder()
            .unary(
              BuyerAppService.updateAddress,
              (_, _) => throw ConnectException(Code.canceled, 'DEV-ONLY'),
            )
            .build(),
      );
      await expectLater(
        AddressRepository(
          client,
          defaults: zhDefaultErrorText,
        ).updateAddress(12, etag: 'v1', receiverName: '甲'),
        throwsA(
          isA<ServerException>().having(
            (e) => e.message,
            'message',
            zhDefaultErrorText(Code.internal),
          ),
        ),
      );
    },
  );

  test(
    'the address list requests the per-buyer limit and follows next_page_token',
    () async {
      final requests = <buyer.ListAddressesRequest>[];
      final client = BuyerAppServiceClient(
        FakeTransportBuilder().unary(BuyerAppService.listAddresses, (
          request,
          _,
        ) {
          requests.add(request);
          return request.pageToken.isEmpty
              ? buyer.ListAddressesResponse(
                  addresses: [buyer.Address(id: '1', etag: 'v1')],
                  nextPageToken: 'p2',
                )
              : buyer.ListAddressesResponse(
                  addresses: [buyer.Address(id: '2', etag: 'v1')],
                );
        }).build(),
      );
      final addresses = await AddressRepository(
        client,
        defaults: zhDefaultErrorText,
      ).getAddresses();
      expect(requests, hasLength(2));
      expect(requests.map((r) => r.pageSize), [20, 20]);
      expect(requests.map((r) => r.pageToken), ['', 'p2']);
      expect(addresses.map((a) => a.id), [1, 2]);
    },
  );

  test('a complete first page is read once', () async {
    var reads = 0;
    final client = BuyerAppServiceClient(
      FakeTransportBuilder().unary(BuyerAppService.listAddresses, (_, _) {
        reads++;
        return buyer.ListAddressesResponse(
          addresses: [buyer.Address(id: '1', etag: 'v1')],
        );
      }).build(),
    );
    await AddressRepository(
      client,
      defaults: zhDefaultErrorText,
    ).getAddresses();
    expect(reads, 1);
  });

  group('update_mask lists only the fields the caller edited', () {
    Future<buyer.UpdateAddressRequest> sentUpdate({
      String? receiverName,
      String? townCode,
      String? addressDetail,
    }) async {
      buyer.UpdateAddressRequest? sent;
      final client = BuyerAppServiceClient(
        FakeTransportBuilder().unary(BuyerAppService.updateAddress, (
          request,
          _,
        ) {
          sent = request;
          return buyer.UpdateAddressResponse();
        }).build(),
      );
      await AddressRepository(
        client,
        defaults: zhDefaultErrorText,
      ).updateAddress(
        12,
        etag: 'v1',
        receiverName: receiverName,
        townCode: townCode,
        addressDetail: addressDetail,
      );
      return sent!;
    }

    test('editing only the receiver sends exactly receiver_name', () async {
      final sent = await sentUpdate(receiverName: '甲');
      expect(sent.updateMask.paths, ['receiver_name']);
    });

    test('editing the address sends exactly postal_address', () async {
      final sent = await sentUpdate(townCode: '63000050', addressDetail: '一號');
      expect(sent.updateMask.paths, ['postal_address']);
    });
  });

  group('postal_address follows the town-code contract', () {
    test('update replaces postal_address as one mask path', () async {
      buyer.UpdateAddressRequest? sent;
      final client = BuyerAppServiceClient(
        FakeTransportBuilder().unary(BuyerAppService.updateAddress, (
          request,
          _,
        ) {
          sent = request;
          return buyer.UpdateAddressResponse();
        }).build(),
      );
      await AddressRepository(
        client,
        defaults: zhDefaultErrorText,
      ).updateAddress(
        12,
        etag: 'v1',
        townCode: '63000050',
        addressDetail: '一號',
      );
      expect(sent?.updateMask.paths, ['postal_address']);
      expect(sent?.address.postalAddress.townCode, '63000050');
      expect(sent?.address.postalAddress.addressDetail, '一號');
    });

    test(
      'an empty town code or a half update never reaches the write',
      () async {
        var writes = 0;
        final client = BuyerAppServiceClient(
          FakeTransportBuilder().unary(BuyerAppService.updateAddress, (_, _) {
            writes++;
            return buyer.UpdateAddressResponse();
          }).build(),
        );
        final repository = AddressRepository(
          client,
          defaults: zhDefaultErrorText,
        );
        await expectLater(
          repository.updateAddress(
            12,
            etag: 'v1',
            townCode: '',
            addressDetail: '一號',
          ),
          throwsA(isA<ServerException>()),
        );
        await expectLater(
          repository.updateAddress(12, etag: 'v1', townCode: '63000050'),
          throwsA(isA<ServerException>()),
        );
        expect(writes, 0);
      },
    );

    test('create sends and the list maps the code with its detail', () async {
      buyer.CreateAddressRequest? sent;
      final client = BuyerAppServiceClient(
        FakeTransportBuilder()
            .unary(BuyerAppService.createAddress, (request, _) {
              sent = request;
              return buyer.CreateAddressResponse();
            })
            .unary(
              BuyerAppService.listAddresses,
              (_, _) => buyer.ListAddressesResponse(
                addresses: [
                  buyer.Address(
                    id: '1',
                    postalAddress: common.PostalAddress(
                      townCode: '63000050',
                      addressDetail: '一號',
                    ),
                  ),
                ],
              ),
            )
            .build(),
      );
      final repository = AddressRepository(
        client,
        defaults: zhDefaultErrorText,
      );
      await repository.addAddress(
        receiverName: '甲',
        receiverPhone: '0912345678',
        townCode: '63000050',
        addressDetail: '一號',
      );
      expect(sent?.address.postalAddress.townCode, '63000050');
      expect(sent?.address.postalAddress.addressDetail, '一號');
      final addresses = await repository.getAddresses();
      expect(addresses.single.townCode, '63000050');
      expect(addresses.single.fullAddress, '臺北市中正區一號');
    });
  });
}
