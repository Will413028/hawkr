import 'package:dio/dio.dart';
import 'package:riverpod_annotation/riverpod_annotation.dart';

import '../../../core/networking/api_request_helper.dart';
import '../../../core/networking/dio_provider.dart';
import '../../../shared/utils/hk_log.dart';
import 'customer_model.dart';
import '../../coupon/data/coupon_model.dart';

part 'customer_repository.g.dart';

class CustomerRepository with ApiRequestHelper {
  CustomerRepository(this._dio);
  final Dio _dio;

  @override
  Dio get dio => _dio;

  // ─── 顧客列表 ────────────────────────────────────────────────────────────

  /// GET /app/sellers/me/customers
  Future<List<Customer>> getCustomers({
    String? customerType, // 'normal' / 'vip' / 'blocklist' / null (全部)
    String? sortBy,
    String sortOrder = 'desc',
    String? keyword,
    int? tagId,
    int page = 1,
    int pageSize = 20,
  }) async {
    try {
      final q = <String, String>{
        'page': '$page',
        'page_size': '$pageSize',
        'sort_order': sortOrder,
      };
      if (keyword != null && keyword.isNotEmpty) q['keyword'] = keyword;
      if (customerType != null && customerType.isNotEmpty) {
        q['customer_type'] = customerType;
      }
      if (sortBy != null && sortBy.isNotEmpty) q['sort_by'] = sortBy;
      if (tagId != null) q['tag_id'] = '$tagId';

      final result = await apiGet(
        '/api/v1/app/sellers/me/customers',
        queryParams: q,
      );
      if (result['code'] == 0) {
        final data = result['data'];
        final List<dynamic> list = data is List
            ? data
            : (data?['list'] ?? data?['customers'] ?? []);
        return list
            .map((j) => _parseCustomer(j as Map<String, dynamic>))
            .toList();
      }
      throw Exception(result['message'] ?? 'Failed to load customers');
    } catch (e) {
      HKLog.logE('CustomerRepository.getCustomers', e);
      rethrow;
    }
  }

  // ─── 顧客詳情 ────────────────────────────────────────────────────────────

  /// GET /app/sellers/me/customers/{buyerId}
  Future<Customer?> getCustomerByAccountId(String accountId) async {
    try {
      final result = await apiGet(
        '/api/v1/app/sellers/me/customers/$accountId',
      );
      if (result['code'] == 0 && result['data'] is Map<String, dynamic>) {
        return _parseCustomer(result['data'] as Map<String, dynamic>);
      }
      return null;
    } catch (e) {
      HKLog.logE('CustomerRepository.getCustomerByAccountId', e);
      return null;
    }
  }

  // ─── 買家基本資訊（不限 CRM 客戶）─────────────────────────────────────────

  /// GET /app/sellers/me/customers/{buyerId}/meta
  /// 透過 buyer-service 查詢，不需要買家在 CRM 中
  Future<({String nickname, String avatarUrl})?> getBuyerMeta(
    String accountId,
  ) async {
    try {
      final result = await apiGet(
        '/api/v1/app/sellers/me/customers/$accountId/meta',
      );
      if (result['code'] == 0 && result['data'] is Map<String, dynamic>) {
        final data = result['data'] as Map<String, dynamic>;
        return (
          nickname: (data['nickname'] ?? '').toString(),
          avatarUrl: (data['avatarUrl'] ?? '').toString(),
        );
      }
      return null;
    } catch (e) {
      HKLog.logE('CustomerRepository.getBuyerMeta', e);
      return null;
    }
  }

  // ─── 更新顧客分類 ────────────────────────────────────────────────────────

  /// PATCH /app/sellers/me/customers/{buyerId}
  /// [customerType] = 'normal' | 'vip' | 'blocklist'
  Future<void> updateCustomerType(String buyerId, String customerType) async {
    try {
      final result = await apiPatch(
        '/api/v1/app/sellers/me/customers/$buyerId',
        body: {'customerType': customerType},
      );
      if (result['code'] != 0) {
        throw Exception(result['message'] ?? 'Update customer type failed');
      }
    } catch (e) {
      HKLog.logE('CustomerRepository.updateCustomerType', e);
      rethrow;
    }
  }

  // ─── 顧客標籤 ────────────────────────────────────────────────────────────

  /// GET /app/sellers/me/customers/{buyerId}/tags
  Future<List<CustomerTag>> getCustomerTags(String buyerId) async {
    try {
      final result = await apiGet(
        '/api/v1/app/sellers/me/customers/$buyerId/tags',
      );
      if (result['code'] == 0) {
        final data = result['data'];
        final List<dynamic> list = data is List
            ? data
            : (data?['list'] ?? data?['tags'] ?? []);
        return list.map((j) => _parseTag(j as Map<String, dynamic>)).toList();
      }
      return [];
    } catch (e) {
      HKLog.logE('CustomerRepository.getCustomerTags', e);
      rethrow;
    }
  }

  /// POST /app/sellers/me/customers/{buyerId}/tags
  Future<void> addCustomerTags(String buyerId, List<String> tagNames) async {
    try {
      final result = await apiPost(
        '/api/v1/app/sellers/me/customers/$buyerId/tags',
        body: {'tagNames': tagNames},
      );
      if (result['code'] != 0) {
        throw Exception(result['message'] ?? 'Add customer tags failed');
      }
    } catch (e) {
      HKLog.logE('CustomerRepository.addCustomerTags', e);
      rethrow;
    }
  }

  /// DELETE /app/sellers/me/customers/{buyerId}/tags/{tagId}
  Future<void> removeCustomerTag(String buyerId, int tagId) async {
    try {
      final result = await apiDelete(
        '/api/v1/app/sellers/me/customers/$buyerId/tags/$tagId',
      );
      if (result['code'] != 0) {
        throw Exception(result['message'] ?? 'Remove customer tag failed');
      }
    } catch (e) {
      HKLog.logE('CustomerRepository.removeCustomerTag', e);
      rethrow;
    }
  }

  // ─── 商家標籤 CRUD ───────────────────────────────────────────────────────

  /// GET /app/sellers/me/tags
  Future<List<CustomerTag>> getSellerTags() async {
    try {
      final result = await apiGet('/api/v1/app/sellers/me/tags');
      if (result['code'] == 0) {
        final data = result['data'];
        final List<dynamic> list = data is List
            ? data
            : (data?['list'] ?? data?['tags'] ?? []);
        return list.map((j) => _parseTag(j as Map<String, dynamic>)).toList();
      }
      return [];
    } catch (e) {
      HKLog.logE('CustomerRepository.getSellerTags', e);
      rethrow;
    }
  }

  /// POST /app/sellers/me/tags
  Future<CustomerTag> createSellerTag(String name) async {
    try {
      final result = await apiPost(
        '/api/v1/app/sellers/me/tags',
        body: {'name': name},
      );
      if (result['code'] == 0 && result['data'] is Map<String, dynamic>) {
        return _parseTag(result['data'] as Map<String, dynamic>);
      }
      throw Exception(result['message'] ?? 'Create tag failed');
    } catch (e) {
      HKLog.logE('CustomerRepository.createSellerTag', e);
      rethrow;
    }
  }

  /// PATCH /app/sellers/me/tags/{tagId}
  Future<void> updateSellerTag(int tagId, String name) async {
    try {
      final result = await apiPatch(
        '/api/v1/app/sellers/me/tags/$tagId',
        body: {'name': name},
      );
      if (result['code'] != 0) {
        throw Exception(result['message'] ?? 'Update tag failed');
      }
    } catch (e) {
      HKLog.logE('CustomerRepository.updateSellerTag', e);
      rethrow;
    }
  }

  /// DELETE /app/sellers/me/tags/{tagId}
  Future<void> deleteSellerTag(int tagId) async {
    try {
      final result = await apiDelete('/api/v1/app/sellers/me/tags/$tagId');
      if (result['code'] != 0) {
        throw Exception(result['message'] ?? 'Delete tag failed');
      }
    } catch (e) {
      HKLog.logE('CustomerRepository.deleteSellerTag', e);
      rethrow;
    }
  }

  // ─── 顧客訂單 / 消費 / 優惠券 ───────────────────────────────────────────

  /// GET /app/sellers/me/customers/{buyerId}/orders — 顧客消費紀錄
  Future<({List<OrderRecord> orders, List<SpendingRecord> spending})>
  getCustomerOrders(String customerId) async {
    try {
      final result = await apiGet(
        '/api/v1/app/sellers/me/customers/$customerId/orders',
        queryParams: {'page_size': '50'},
      );
      if (result['code'] == 0) {
        final data = result['data'];
        final List<dynamic> list = data is List
            ? data
            : (data?['records'] ?? data?['list'] ?? data?['orders'] ?? []);
        final orders = list
            .map((j) => _parseOrderRecord(j as Map<String, dynamic>))
            .toList();
        final spending = list
            .map((j) => _parseSpendingRecord(j as Map<String, dynamic>))
            .toList();
        return (orders: orders, spending: spending);
      }
      throw Exception(result['message']);
    } catch (e) {
      HKLog.logE('CustomerRepository.getCustomerOrders', e);
      rethrow;
    }
  }

  /// GET /app/sellers/me/customers/{buyerId}/coupons — 顧客優惠券列表
  Future<List<Coupon>> getCustomerCoupons(String customerId) async {
    try {
      final result = await apiGet(
        '/api/v1/app/sellers/me/customers/$customerId/coupons',
      );
      if (result['code'] == 0) {
        final data = result['data'];
        final List<dynamic> list = data is List
            ? data
            : (data?['records'] ?? data?['list'] ?? data?['coupons'] ?? []);
        return list
            .map((j) => _parseCoupon(j as Map<String, dynamic>))
            .toList();
      }
      return [];
    } catch (e) {
      HKLog.logE('CustomerRepository.getCustomerCoupons', e);
      rethrow;
    }
  }

  // ─── 解析輔助 ────────────────────────────────────────────────────────────

  CustomerTag _parseTag(Map<String, dynamic> j) {
    return CustomerTag(
      id: _parseInt(j['id'] ?? j['tagId'] ?? 0),
      name: j['name']?.toString() ?? '',
    );
  }

  /// 解析顧客列表 (`GET /customers`) 與顧客詳情 (`GET /customers/{id}`) 的單筆
  /// 客戶。兩支端點的欄位名一致，都是 camelCase（列表在後端把 CRM 家族統一成
  /// camelCase 時跟上，與本次改動同批上線）。
  ///
  /// **刻意不留舊名 / 猜測性 fallback**：契約只有一個來源，對不上就該讓測試炸
  /// 掉，而不是靜默拿到別的值——`customer_account_id ?? account_id ?? id` 這種
  /// chain 正是列表切 camel 時讓客戶 ID 靜默變成 crm row 主鍵的原因。
  Customer _parseCustomer(Map<String, dynamic> j) {
    // customerType: 'normal' / 'vip' / 'blocklist'
    final ct = (j['customerType'] ?? '').toString().toLowerCase();

    final List<CustomerTag> tags = [];
    if (j['tags'] is List) {
      for (final t in j['tags'] as List) {
        if (t is Map<String, dynamic>) {
          tags.add(_parseTag(t));
        } else if (t is String) {
          tags.add(CustomerTag(id: 0, name: t));
        }
      }
    }

    final isVip = ct == 'vip';
    final isBlacklisted = ct == 'blocklist';

    // 後端欄位是 frequentCategories（詳情才有；列表不回這個欄位）。
    // 原本讀的 `frequentItems` 後端從來沒有回過，客戶詳情的「常買品項」區塊
    // 因此永遠是空的、被 isEmpty 判斷整段隱藏。
    final frequentItems = <String>[];
    if (j['frequentCategories'] is List) {
      frequentItems.addAll(
        (j['frequentCategories'] as List).map((e) => e.toString()),
      );
    }

    return Customer(
      // 客戶識別碼，值語意是 buyer_id（2026-07-27 rekey 後端回應改用
      // camelCase `buyerId`；不接受 `id`——那是 crm row 主鍵，別人的 ID）。
      id: (j['buyerId'] ?? '').toString(),
      name: j['nickname'] ?? '未知',
      phone: j['phone'] ?? '',
      totalSpent: _parseInt(j['totalSpent'] ?? 0),
      visitCount: _parseInt(j['orderCount'] ?? 0),
      couponCount: _parseInt(j['couponCount'] ?? 0),
      isVip: isVip,
      isBlacklisted: isBlacklisted,
      customerType: ct.isEmpty ? 'normal' : ct,
      avatarUrl: j['avatarUrl'],
      address: j['address']?.toString(),
      email: j['email']?.toString(),
      frequentItems: frequentItems,
      tags: tags,
    );
  }

  OrderRecord _parseOrderRecord(Map<String, dynamic> j) {
    String items = '';
    if (j['items'] is List) {
      items = (j['items'] as List)
          .map((item) {
            final productName = (item['productName'] ?? item['name'] ?? '')
                .toString();
            final skuName = (item['skuName'] ?? '').toString();
            if (productName.isNotEmpty && skuName.isNotEmpty) {
              return '$productName ($skuName)';
            }
            return productName.isNotEmpty ? productName : skuName;
          })
          .where((n) => n.isNotEmpty)
          .take(3)
          .join(', ');
    } else if (j['productNames'] is String) {
      items = j['productNames'] as String;
    }

    // Status codes from /app/orders API (2026-03 v2):
    // 0=待付款, 10=已付款, 20=已出貨, 30=配送中, 32=待取貨, 40=已送達, 50=已完成, 60=部分退款, 70=全額退款, 80=已取消
    final statusCode = _parseInt(j['status'] ?? 0);
    final statusStr = (j['status'] ?? '').toString();
    OrderStatus status;
    if (statusCode == 50 || statusStr == 'completed') {
      status = OrderStatus.completed;
    } else if (statusCode == 80 || statusStr == 'cancelled') {
      status = OrderStatus.cancelled;
    } else if (statusCode == 60 || statusCode == 70) {
      status = OrderStatus.completed; // 退款歸類為已完成
    } else if (statusStr == 'issue') {
      status = OrderStatus.issue;
    } else if (statusCode >= 10 || statusStr == 'delivering') {
      status = OrderStatus.delivering;
    } else {
      status = OrderStatus.delivering;
    }

    // Extract review/rating if present (from reviews array or flat fields)
    int rating = 0;
    String review = '';
    if (j['reviews'] is List && (j['reviews'] as List).isNotEmpty) {
      final r = (j['reviews'] as List).first;
      if (r is Map<String, dynamic>) {
        rating = _parseInt(r['rating'] ?? 0);
        review = (r['content'] ?? r['comment'] ?? '').toString();
      }
    } else {
      rating = _parseInt(j['rating'] ?? 0);
      review = (j['review'] ?? j['comment'] ?? j['content'] ?? '').toString();
    }

    return OrderRecord(
      orderId: (j['orderNo'] ?? j['orderSn'] ?? j['orderId'] ?? j['id'] ?? '')
          .toString(),
      date: _formatDate(
        (j['createTime'] ?? j['dateTime'] ?? j['createdAt'] ?? '').toString(),
      ),
      items: items.isNotEmpty ? items : '商品',
      amount: _parseInt(
        j['actualAmount'] ??
            j['totalActualAmount'] ??
            j['totalAmount'] ??
            j['amount'] ??
            0,
      ),
      status: status,
      rating: rating,
      review: review,
    );
  }

  SpendingRecord _parseSpendingRecord(Map<String, dynamic> j) {
    return SpendingRecord(
      date: _formatDate(
        (j['createTime'] ?? j['dateTime'] ?? j['createdAt'] ?? '').toString(),
        format: 'MM/dd',
      ),
      orderId: (j['orderNo'] ?? j['orderSn'] ?? j['orderId'] ?? j['id'] ?? '')
          .toString(),
      amount: _parseInt(
        j['actualAmount'] ??
            j['totalActualAmount'] ??
            j['totalAmount'] ??
            j['amount'] ??
            0,
      ),
      refund: _parseInt(j['refundAmount'] ?? 0),
      discountAmount: _parseInt(
        j['discountAmount'] ?? j['discount_amount'] ?? 0,
      ),
    );
  }

  Coupon _parseCoupon(Map<String, dynamic> m) {
    // Normalise type: handle both integer and string
    String type;
    final rawType = m['type'];
    if (rawType is int) {
      switch (rawType) {
        case 2:
          type = 'DISCOUNT';
        default:
          type = 'CASH';
      }
    } else {
      type = (rawType ?? 'CASH').toString().toUpperCase();
    }

    return Coupon(
      templateId: (m['templateId'] ?? m['template_id'] ?? m['id'] ?? '')
          .toString(),
      instanceId: (m['instanceId'] ?? m['instance_id'] ?? '').toString(),
      title: (m['title'] ?? m['name'] ?? '').toString(),
      type: type,
      status: (m['status'] ?? '').toString(),
      discountAmount:
          m['discountAmount']?.toString() ?? m['discount_amount']?.toString(),
      discountPercent:
          m['discountPercent']?.toString() ?? m['discount_percent']?.toString(),
      thresholdAmount:
          m['thresholdAmount']?.toString() ?? m['threshold_amount']?.toString(),
      maxDiscountAmount: m['maxDiscountAmount']?.toString(),
      startAt: m['startAt']?.toString() ?? m['start_at']?.toString(),
      endAt: m['endAt']?.toString() ?? m['end_at']?.toString(),
      applicableStoreId: m['applicableStoreId']?.toString(),
      sellerName: m['sellerName']?.toString(),
      createdAt: m['createdAt']?.toString(),
    );
  }

  int _parseInt(dynamic v) {
    if (v is int) return v;
    if (v is double) return v.toInt();
    if (v is String) return int.tryParse(v) ?? 0;
    return 0;
  }

  String _formatDate(String raw, {String format = 'yyyy/MM/dd'}) {
    if (raw.isEmpty) return '';
    try {
      final dt = DateTime.tryParse(
        raw.contains('T') ? raw : raw.replaceFirst(' ', 'T'),
      );
      if (dt != null) {
        final local = dt.toLocal();
        if (format == 'MM/dd') {
          return '${local.month.toString().padLeft(2, '0')}/${local.day.toString().padLeft(2, '0')}';
        }
        return '${local.year}/${local.month.toString().padLeft(2, '0')}/${local.day.toString().padLeft(2, '0')}';
      }
    } catch (_) {}
    return raw;
  }
}

@riverpod
CustomerRepository customerRepository(Ref ref) {
  return CustomerRepository(ref.watch(dioProvider));
}
