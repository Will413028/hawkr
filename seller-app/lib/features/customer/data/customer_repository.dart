import 'package:hawkr_api/transport.dart' show Code, ConnectException;
import 'package:hawkr_api/field_mask.dart';
import 'package:hawkr_api/gen/hawkr/crm/v1alpha1/crm.connect.client.dart';
import 'package:hawkr_api/gen/hawkr/crm/v1alpha1/crm.pb.dart' as pb;
import 'package:hawkr_api/gen/hawkr/crm/v1alpha1/crm.pbenum.dart';
import 'package:riverpod_annotation/riverpod_annotation.dart';

import '../../../core/networking/connect_provider.dart';
import '../../seller/data/current_store.dart';
import 'customer_model.dart';

part 'customer_repository.g.dart';

@riverpod
CustomerRepository customerRepository(Ref ref) => CustomerRepository(
  ref.watch(crmClientProvider),
  storeId: () => ref.read(sellerStoreIdProvider),
);

/// 客戶列表的一頁：沒有總數，結束只看 [nextPageToken] 是否為空。
class CustomerPage {
  const CustomerPage({required this.customers, required this.nextPageToken});

  final List<Customer> customers;
  final String nextPageToken;
}

/// 店家的客戶與標籤（`CrmAppService`）。店家 ID 取呼叫時店家的 ID；沒有店家時丟出 [StateError]。
class CustomerRepository {
  CustomerRepository(this._crm, {required String? Function() storeId})
    : _storeId = storeId;

  /// 每頁筆數（API 預設 20，上限 100）。
  static const pageSize = 20;

  final CrmAppServiceClient _crm;
  final String? Function() _storeId;

  String get _store {
    final id = _storeId();
    if (id == null || id.isEmpty) throw StateError('no store');
    return id;
  }

  /// 列出客戶。[type]、[tagId] 不給是不篩選；[sort] 不給是伺服器預設（首筆完成訂單時間）。
  Future<CustomerPage> listCustomers({
    CustomerType? type,
    String? tagId,
    CustomerSort sort = CustomerSort.CUSTOMER_SORT_UNSPECIFIED,
    bool descending = false,
    String pageToken = '',
    int size = pageSize,
  }) async {
    final request = pb.ListCustomersRequest(
      storeId: _store,
      sort: sort,
      descending: descending,
      pageSize: size,
      pageToken: pageToken,
    );
    if (type != null) request.type = type;
    if (tagId != null) request.tagId = tagId;
    final res = await _crm.listCustomers(request);
    return CustomerPage(
      customers: res.customers.map(Customer.fromProto).toList(),
      nextPageToken: res.nextPageToken,
    );
  }

  /// 列出貼了某標籤的全部客戶（逐頁讀到沒有下一頁）；發券對象要完整名單，不能只取第一頁。
  Future<List<Customer>> listAllCustomersWithTag(String tagId) async {
    final all = <Customer>[];
    var token = '';
    do {
      final page = await listCustomers(
        tagId: tagId,
        pageToken: token,
        size: 100,
      );
      all.addAll(page.customers);
      token = page.nextPageToken;
    } while (token.isNotEmpty);
    return all;
  }

  /// 讀一位客戶；這家店沒有這位客戶（含買家已刪除）時回 null，其他失敗照丟。
  Future<Customer?> getCustomer(String buyerId) async {
    try {
      final res = await _crm.getCustomer(
        pb.GetCustomerRequest(storeId: _store, buyerId: buyerId),
      );
      return Customer.fromProto(res.customer);
    } on ConnectException catch (e) {
      if (e.code == Code.notFound) return null;
      rethrow;
    }
  }

  /// 改客戶的分類，回更新後的客戶。
  Future<Customer> updateCustomerType(String buyerId, CustomerType type) async {
    final customer = pb.Customer(buyerId: buyerId, type: type);
    final res = await _crm.updateCustomer(
      pb.UpdateCustomerRequest(
        storeId: _store,
        customer: customer,
        updateMask: fieldMask(customer, ['type']),
      ),
    );
    return Customer.fromProto(res.customer);
  }

  /// 替客戶貼上標籤（名稱不存在的先建立），回貼完之後的客戶。
  Future<Customer> addCustomerTags(String buyerId, List<String> names) async {
    final res = await _crm.addCustomerTags(
      pb.AddCustomerTagsRequest(
        storeId: _store,
        buyerId: buyerId,
        tagNames: names,
      ),
    );
    return Customer.fromProto(res.customer);
  }

  Future<Customer> removeCustomerTag(String buyerId, String tagId) async {
    final res = await _crm.removeCustomerTag(
      pb.RemoveCustomerTagRequest(
        storeId: _store,
        buyerId: buyerId,
        tagId: tagId,
      ),
    );
    return Customer.fromProto(res.customer);
  }

  Future<List<CustomerTag>> listTags() async {
    final res = await _crm.listTags(pb.ListTagsRequest(storeId: _store));
    return res.tags.map(CustomerTag.fromProto).toList();
  }

  Future<CustomerTag> createTag(String name) async {
    final res = await _crm.createTag(
      pb.CreateTagRequest(storeId: _store, name: name),
    );
    return CustomerTag.fromProto(res.tag);
  }

  Future<CustomerTag> renameTag(String tagId, String name) async {
    final tag = pb.Tag(id: tagId, name: name);
    final res = await _crm.updateTag(
      pb.UpdateTagRequest(
        storeId: _store,
        tag: tag,
        updateMask: fieldMask(tag, ['name']),
      ),
    );
    return CustomerTag.fromProto(res.tag);
  }

  Future<void> deleteTag(String tagId) async {
    await _crm.deleteTag(pb.DeleteTagRequest(storeId: _store, tagId: tagId));
  }
}
