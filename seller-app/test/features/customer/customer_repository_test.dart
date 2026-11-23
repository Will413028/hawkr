import 'package:flutter_test/flutter_test.dart';
import 'package:hawkr_api/gen/hawkr/crm/v1alpha1/crm.connect.client.dart';
import 'package:hawkr_api/gen/hawkr/crm/v1alpha1/crm.connect.spec.dart';
import 'package:hawkr_api/gen/hawkr/crm/v1alpha1/crm.pb.dart' as pb;
import 'package:hawkr_api/gen/hawkr/crm/v1alpha1/crm.pbenum.dart';
import 'package:hawkr_api/testing.dart';
import 'package:hawkr_api/transport.dart' show Code, ConnectException;
import 'package:hawkr_seller/features/customer/data/customer_repository.dart';

CustomerRepository _repo(
  FakeTransportBuilder transport, {
  String? store = '7',
}) => CustomerRepository(
  CrmAppServiceClient(transport.build()),
  storeId: () => store,
);

pb.Customer _customer(String id, {CustomerType? type, List<pb.Tag>? tags}) =>
    pb.Customer(
      buyerId: id,
      type: type ?? CustomerType.CUSTOMER_TYPE_NORMAL,
      orderCount: 3,
      display: pb.CustomerDisplay(nickname: '買家$id'),
      tags: tags,
    )..mergeFromProto3Json({'totalSpent': '1500'});

void main() {
  test(
    'listCustomers sends store, filters, sort, direction and page token',
    () async {
      final requests = <pb.ListCustomersRequest>[];
      final repo = _repo(
        FakeTransportBuilder().unary(CrmAppService.listCustomers, (req, _) {
          requests.add(req);
          return pb.ListCustomersResponse(
            customers: [_customer('1'), _customer('2')],
            nextPageToken: 'next',
          );
        }),
      );

      final page = await repo.listCustomers(
        type: CustomerType.CUSTOMER_TYPE_VIP,
        tagId: 'tag-9',
        sort: CustomerSort.CUSTOMER_SORT_TOTAL_SPENT,
        descending: true,
        pageToken: 'cursor',
      );

      final r = requests.single;
      expect(r.storeId, '7');
      expect(r.hasType(), isTrue);
      expect(r.type, CustomerType.CUSTOMER_TYPE_VIP);
      expect(r.tagId, 'tag-9');
      expect(r.sort, CustomerSort.CUSTOMER_SORT_TOTAL_SPENT);
      expect(r.descending, isTrue);
      expect(r.pageToken, 'cursor');
      expect(page.nextPageToken, 'next');
      expect(page.customers.map((c) => c.id), ['1', '2']);
      expect(page.customers.first.totalSpent, 1500);
      expect(page.customers.first.orderCount, 3);
    },
  );

  test('listCustomers without a type filter leaves type unset', () async {
    final requests = <pb.ListCustomersRequest>[];
    final repo = _repo(
      FakeTransportBuilder().unary(CrmAppService.listCustomers, (req, _) {
        requests.add(req);
        return pb.ListCustomersResponse();
      }),
    );
    await repo.listCustomers();
    expect(requests.single.hasType(), isFalse);
    expect(requests.single.hasTagId(), isFalse);
  });

  test('listAllCustomersWithTag follows next_page_token to the end', () async {
    final tokens = <String>[];
    final repo = _repo(
      FakeTransportBuilder().unary(CrmAppService.listCustomers, (req, _) {
        tokens.add(req.pageToken);
        return switch (req.pageToken) {
          '' => pb.ListCustomersResponse(
            customers: [_customer('1')],
            nextPageToken: 'p2',
          ),
          _ => pb.ListCustomersResponse(customers: [_customer('2')]),
        };
      }),
    );
    final all = await repo.listAllCustomersWithTag('t');
    expect(tokens, ['', 'p2']);
    expect(all.map((c) => c.id), ['1', '2']);
  });

  test(
    'getCustomer maps not_found to null and rethrows other errors',
    () async {
      final notFound = _repo(
        FakeTransportBuilder().unary(CrmAppService.getCustomer, (_, _) {
          throw ConnectException(Code.notFound, 'no');
        }),
      );
      expect(await notFound.getCustomer('1'), isNull);

      final denied = _repo(
        FakeTransportBuilder().unary(CrmAppService.getCustomer, (_, _) {
          throw ConnectException(Code.permissionDenied, 'suspended');
        }),
      );
      await expectLater(
        denied.getCustomer('1'),
        throwsA(
          isA<ConnectException>().having(
            (e) => e.code,
            'code',
            Code.permissionDenied,
          ),
        ),
      );
    },
  );

  test('updateCustomerType sends only type in the update mask', () async {
    final requests = <pb.UpdateCustomerRequest>[];
    final repo = _repo(
      FakeTransportBuilder().unary(CrmAppService.updateCustomer, (req, _) {
        requests.add(req);
        return pb.UpdateCustomerResponse(
          customer: _customer('5', type: CustomerType.CUSTOMER_TYPE_VIP),
        );
      }),
    );
    final updated = await repo.updateCustomerType(
      '5',
      CustomerType.CUSTOMER_TYPE_VIP,
    );
    final r = requests.single;
    expect(r.storeId, '7');
    expect(r.customer.buyerId, '5');
    expect(r.customer.type, CustomerType.CUSTOMER_TYPE_VIP);
    expect(r.updateMask.paths, ['type']);
    expect(updated.isVip, isTrue);
  });

  test(
    'tag operations send string ids and return the updated customer',
    () async {
      final seen = <String>[];
      final repo = _repo(
        FakeTransportBuilder()
            .unary(CrmAppService.addCustomerTags, (req, _) {
              seen.add('add ${req.buyerId} ${req.tagNames.join(',')}');
              return pb.AddCustomerTagsResponse(
                customer: _customer(
                  '5',
                  tags: [pb.Tag(id: 't1', name: '常客')],
                ),
              );
            })
            .unary(CrmAppService.removeCustomerTag, (req, _) {
              seen.add('remove ${req.buyerId} ${req.tagId}');
              return pb.RemoveCustomerTagResponse(customer: _customer('5'));
            })
            .unary(CrmAppService.createTag, (req, _) {
              seen.add('create ${req.name}');
              return pb.CreateTagResponse(
                tag: pb.Tag(id: 't2', name: req.name),
              );
            })
            .unary(CrmAppService.updateTag, (req, _) {
              seen.add(
                'rename ${req.tag.id} ${req.tag.name} ${req.updateMask.paths}',
              );
              return pb.UpdateTagResponse(tag: req.tag);
            })
            .unary(CrmAppService.deleteTag, (req, _) {
              seen.add('delete ${req.tagId}');
              return pb.DeleteTagResponse();
            })
            .unary(CrmAppService.listTags, (req, _) {
              seen.add('list ${req.storeId}');
              return pb.ListTagsResponse(
                tags: [pb.Tag(id: 't1', name: '常客')],
              );
            }),
      );

      final added = await repo.addCustomerTags('5', ['常客']);
      expect(added.tags.single.id, 't1');
      expect((await repo.removeCustomerTag('5', 't1')).tags, isEmpty);
      expect((await repo.createTag('新客')).id, 't2');
      expect((await repo.renameTag('t2', '舊客')).name, '舊客');
      await repo.deleteTag('t2');
      expect((await repo.listTags()).single.name, '常客');
      expect(seen, [
        'add 5 常客',
        'remove 5 t1',
        'create 新客',
        'rename t2 舊客 [name]',
        'delete t2',
        'list 7',
      ]);
    },
  );

  test('a missing store fails before any request', () async {
    final repo = _repo(FakeTransportBuilder(), store: null);
    await expectLater(repo.listCustomers(), throwsStateError);
  });
}
