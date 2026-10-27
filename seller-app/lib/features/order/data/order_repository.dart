import 'package:dio/dio.dart';
import 'package:riverpod_annotation/riverpod_annotation.dart';

import '../../../config/flavor.dart';
import '../../../core/exceptions/app_exception.dart';
import '../../../core/networking/api_request_helper.dart';
import '../../../core/networking/dio_provider.dart';
import '../../seller/data/current_store.dart';
import '../../../shared/utils/hk_log.dart';
import 'order_model.dart';

part 'order_repository.g.dart';

class OrderRepository with ApiRequestHelper {
  OrderRepository(this._dio, this._ref);
  final Dio _dio;
  final Ref _ref;

  @override
  Dio get dio => _dio;

  // ── 訂單列表 ──────────────────────────────────────────

  Future<List<OrderSummary>> fetchMatchedOrders({
    int page = 1,
    int pageSize = 10,
    String? status,
  }) async {
    HKLog.logD(
      '[OrderRepository] fetchMatchedOrders page=$page pageSize=$pageSize status=$status',
    );
    final resp = await apiGet(
      '/api/v1/app/orders',
      queryParams: {
        'view': 'seller',
        'page': '$page',
        'page_size': '$pageSize',
        if (status != null) 'status': status,
      },
    );
    HKLog.logD(
      '[OrderRepository] fetchMatchedOrders resp code=${resp['code']} message=${resp['message']} dataType=${resp['data']?.runtimeType}',
    );
    return _parseOrderList(resp);
  }

  Future<List<OrderSummary>> fetchMatchedWithItemOrders({
    int page = 1,
    int pageSize = 10,
    String? status,
  }) async {
    final resp = await apiGet(
      '/api/v1/app/orders',
      queryParams: {
        'view': 'seller',
        'page': '$page',
        'page_size': '$pageSize',
        if (status != null) 'status': status,
      },
    );
    return _parseOrderList(resp);
  }

  Future<List<OrderSummary>> fetchOpeningOrders({
    int page = 1,
    int pageSize = 10,
  }) async {
    final resp = await apiGet(
      '/api/v1/app/orders',
      queryParams: {
        'view': 'seller',
        'page': '$page',
        'page_size': '$pageSize',
        'status': '10,20', // PAID + PENDING_SHIPMENT (待出貨)
      },
    );
    return _parseOrderList(resp);
  }

  // ── 訂單詳情 ──────────────────────────────────────────

  Future<OrderSummary> fetchMatchedOrderDetails({
    required String orderSn,
  }) async {
    final resp = await apiGet(
      '/api/v1/app/orders/$orderSn',
      queryParams: {'view': 'seller'},
    );
    final data = resp['data'];
    if (data is Map<String, dynamic>) return OrderSummary.fromJson(data);
    return OrderSummary.fromJson(resp);
  }

  /// 取得訂單原始 JSON（用於抓 paymentMethod, shippingNote 等未建模欄位）
  Future<Map<String, dynamic>> fetchRawOrderJson(String orderSn) async {
    final resp = await apiGet(
      '/api/v1/app/orders/$orderSn',
      queryParams: {'view': 'seller'},
    );
    final data = resp['data'];
    return data is Map<String, dynamic> ? data : resp;
  }

  /// 查詢訂單付款歷程，含切換付款方式關閉的舊交易與重複付款紀錄。
  ///
  /// #2501：付款歷程一般路徑已收斂為買家專用；賣家必須帶 view=seller，後端以現任
  /// store_id 驗自己店家的訂單歸屬並遮蔽買家 PII。少了這個參數會落入買家路徑而回 404。
  Future<List<Map<String, dynamic>>> fetchPaymentHistory(String orderSn) async {
    final resp = await apiGet(
      '/api/v1/app/payments/orders/$orderSn/history',
      queryParams: {'view': 'seller'},
    );
    final data = resp['data'];
    final records = data is Map ? data['records'] : data;
    if (records is List) {
      return records
          .whereType<Map>()
          .map((m) => Map<String, dynamic>.from(m))
          .toList(growable: false);
    }
    return const [];
  }

  @Deprecated(
    'Legacy endpoint /order/order/reservation/status does not exist in gateway',
  )
  Future<String?> fetchReservationStatus(String orderSn) async {
    final url = '${F.appFlavor.appApi}/order/order/reservation/status/$orderSn';
    HKLog.logD('[OrderRepository] fetchReservationStatus $url');
    try {
      final res = await _dio.get(
        url,
        options: Options(validateStatus: (s) => true, headers: _jsonHeaders),
      );
      if ((res.statusCode ?? 0) != 200) return null;
      final decoded = res.data;
      if (decoded is Map<String, dynamic>) {
        final s =
            decoded['reservationStatus'] ??
            decoded['status'] ??
            decoded['statusName'] ??
            decoded['statusDesc'] ??
            decoded['state'];
        if (s != null) return s.toString();
        final c = decoded['code'] ?? decoded['statusCode'];
        if (c != null) return c.toString();
      }
      if (decoded is String) return decoded;
    } catch (_) {}
    return null;
  }

  @Deprecated(
    'Legacy endpoint /order/order/reservation/status does not exist in gateway',
  )
  Future<void> setReservationStatus({
    required String orderSn,
    required String status,
  }) async {
    final url = '${F.appFlavor.appApi}/order/order/reservation/status/$orderSn';
    final res = await _dio.post(
      url,
      queryParameters: {'status': status},
      options: Options(validateStatus: (s) => true, headers: _jsonHeaders),
    );
    if ((res.statusCode ?? 0) != 200) throw Exception('HTTP ${res.statusCode}');
    final body = res.data is Map<String, dynamic>
        ? res.data as Map<String, dynamic>
        : null;
    if (body == null) throw Exception('無效的伺服器回應');
    final code = int.tryParse(body['code']?.toString() ?? '') ?? -1;
    if (code != 0) {
      final msg = body['message']?.toString();
      throw Exception((msg == null || msg.isEmpty) ? '更新訂單狀態失敗' : msg);
    }
  }

  // ── 出貨工作日 ──────────────────────────────────────────

  /// 取得明日起 7 個工作天（排除週日與國定假日）
  /// GET /app/shipping_business_days
  Future<List<BusinessDay>> fetchShippingBusinessDays() async {
    final resp = await apiGet('/api/v1/app/shipping_business_days');
    if (resp['code'] == 0 && resp['data'] is Map<String, dynamic>) {
      final data = resp['data'] as Map<String, dynamic>;
      final list = data['businessDays'] as List<dynamic>? ?? [];
      return list
          .whereType<Map>()
          .map(
            (e) => BusinessDay(
              date: (e['date'] ?? '').toString(),
              dayOfWeek: (e['dayOfWeek'] ?? '').toString(),
            ),
          )
          .toList(growable: false);
    }
    return const [];
  }

  // ── 訂單操作 ──────────────────────────────────────────

  /// 超商確認出貨
  /// POST /app/shipments/{order_no}/confirm
  /// 前置條件：超商物流（SEVEN_ELEVEN_STORE_PICKUP）且已完成取號
  /// [stagedTokens] 選填，出貨照片的 stagedToken
  ///
  /// 注意：OpenAPI spec 寫的 /app/seller/shipments/{order_no}/confirm 後端
  /// 還沒實作（hit 404）。實際 deploy 的是這條舊路徑 /app/shipments/...，
  /// 但舊路徑 backend schema 可能不收 stagedTokens（B-148 issue 追蹤）。
  /// 等 backend 把 spec 寫的 /seller/ 路徑實作好或舊路徑補 stagedTokens
  /// 之後再切回正式路徑。
  Future<Map<String, dynamic>> confirmShipment(
    String orderNo, {
    List<String>? stagedTokens,
  }) {
    final body = <String, dynamic>{};
    if (stagedTokens != null && stagedTokens.isNotEmpty) {
      body['stagedTokens'] = stagedTokens;
    }
    return apiPost('/api/v1/app/shipments/$orderNo/confirm', body: body);
  }

  /// 反查賣家收到的某筆訂單評論（B-145 完成前的 fallback）。
  ///
  /// `/app/orders/{orderNo}` detail 沒回傳 review 欄位，所以走
  /// `/app/sellers/me/reviews` 列表，依 `order_item_id` 比對找出對應評論。
  /// 後端 review entity 沒有 orderNo 欄位，所以 caller 必須提供該訂單所有
  /// OrderItem id 的集合。
  ///
  /// 內部分頁掃（最多 [maxPages] 頁），找到第一筆 match 就回傳。
  /// 沒找到回 null（買家還沒評論）。
  Future<Map<String, dynamic>?> findReviewByOrderItemIds(
    Set<String> orderItemIds, {
    int maxPages = 5,
    int pageSize = 50,
  }) async {
    if (orderItemIds.isEmpty) return null;
    for (var page = 1; page <= maxPages; page++) {
      final resp = await apiGet(
        '/api/v1/app/sellers/me/reviews',
        queryParams: {'page': '$page', 'size': '$pageSize'},
      );
      if ((resp['code'] as int? ?? -1) != 0) return null;
      final data = resp['data'];
      final list =
          (data is Map ? (data['records'] ?? data['list']) : null) as List? ??
          const [];
      for (final raw in list) {
        if (raw is! Map) continue;
        // 後端這支吐的是 camelCase（`gateway/internal/handler/order_review.go`
        // 的 `OrderReviewRecord.OrderItemID` tag 為 `orderItemId`）。這裡曾寫成
        // snake_case，取到的永遠是空字串，於是比對永不成立、賣家在訂單詳情
        // 一直看不到買家的評論——而且完全無聲，沒有錯誤也沒有空狀態提示。
        final id = (raw['orderItemId'] ?? '').toString().trim();
        if (id.isNotEmpty && orderItemIds.contains(id)) {
          return Map<String, dynamic>.from(raw);
        }
      }
      if (list.length < pageSize) break; // 沒滿一頁 = 沒更多
    }
    return null;
  }

  /// 賣家取消訂單
  /// POST /app/orders/{order_no}/seller_cancellations
  Future<CancelOrderResult> cancelOrder(
    String orderNo, {
    String reason = 'OTHER',
  }) async {
    final res = await apiPost(
      '/api/v1/app/orders/$orderNo/seller_cancellations',
      body: {'reason': reason.isNotEmpty ? reason : 'OTHER'},
    );
    final code = res['code'] as int? ?? -1;
    if (code != 0) {
      final msg = res['message']?.toString().trim() ?? '取消訂單失敗';
      throw Exception(msg);
    }
    final data = res['data'] as Map<String, dynamic>? ?? {};
    return CancelOrderResult(
      orderNo: (data['orderNo'] ?? orderNo).toString(),
      refundAmount: (data['refundAmount'] ?? '0').toString(),
      needRefundPage: data['needRefundPage'] == true,
    );
  }

  /// 呼叫黑貓預約取件
  /// POST /app/shipments/{order_no}/call_tcat
  /// [isContact] 電話聯絡 Y/N, [isTrolley] 需要推車 Y/N, [memo] 備註
  Future<CallTcatResult> callTcat(
    String orderNo, {
    String? isContact,
    String? isTrolley,
    String? memo,
  }) async {
    final body = <String, dynamic>{};
    if (isContact != null) body['isContact'] = isContact;
    if (isTrolley != null) body['isTrolley'] = isTrolley;
    if (memo != null && memo.isNotEmpty) body['memo'] = memo;
    final resp = await apiPost(
      '/api/v1/app/shipments/$orderNo/call_tcat',
      body: body,
    );
    if (resp['code'] == 0 && resp['data'] is Map<String, dynamic>) {
      final data = resp['data'] as Map<String, dynamic>;
      return CallTcatResult(
        callTime: (data['callTime'] ?? '').toString(),
        shipmentNo: (data['shipmentNo'] ?? '').toString(),
      );
    }
    final code = resp['code'] is int ? resp['code'] as int : -1;
    final msg = (resp['message'] ?? '呼叫黑貓取件失敗').toString();
    throw ApiException(code: code, message: msg);
  }

  /// 超商出貨單列印（取得交貨便代碼）
  /// POST /app/shipments/{order_no}/print_cvs_label
  /// 回傳 { odno: 出貨單編號(8碼), validationNo: C2C驗證碼(4碼) }
  /// [stagedTokens] 選填，有傳才更新照片
  Future<CvsLabelResult> printCvsLabel(
    String orderNo, {
    List<String>? stagedTokens,
  }) async {
    final now = DateTime.now().add(const Duration(days: 1));
    final shipDate =
        '${now.year}-${now.month.toString().padLeft(2, '0')}-${now.day.toString().padLeft(2, '0')}';
    final body = <String, dynamic>{'shipDate': shipDate};
    if (stagedTokens != null && stagedTokens.isNotEmpty) {
      body['stagedTokens'] = stagedTokens;
    }
    final resp = await apiPost(
      '/api/v1/app/shipments/$orderNo/print_cvs_label',
      body: body,
    );
    if (resp['code'] == 0 && resp['data'] is Map<String, dynamic>) {
      final data = resp['data'] as Map<String, dynamic>;
      return CvsLabelResult(
        shipmentNo: (data['shipmentNo'] ?? '').toString(),
        partnerId: (data['partnerId'] ?? '').toString(),
        odno: (data['odno'] ?? '').toString(),
        validationNo: (data['validationNo'] ?? '').toString(),
        webhookReceived: data['webhookReceived'] == true,
      );
    }
    final code = resp['code'] is int ? resp['code'] as int : -1;
    final msg = (resp['message'] ?? '超商出貨失敗').toString();
    throw ApiException(code: code, message: msg);
  }

  /// 產宅配託運單 PDF
  ///
  /// POST /app/shipments/{order_no}/generate_home_label
  /// [shipDate] 格式 yyyyMMdd（如 "20260325"）
  /// [stagedTokens] 選填，有傳才更新照片
  /// 回傳 { labelUrl, shipmentNo }
  Future<HomeLabelResult> generateHomeLabel(
    String orderNo, {
    required String shipDate,
    List<String>? stagedTokens,
  }) async {
    final body = <String, dynamic>{'shipDate': shipDate};
    if (stagedTokens != null && stagedTokens.isNotEmpty) {
      body['stagedTokens'] = stagedTokens;
    }
    final resp = await apiPost(
      '/api/v1/app/shipments/$orderNo/generate_home_label',
      body: body,
    );
    if (resp['code'] == 0 && resp['data'] is Map<String, dynamic>) {
      final data = resp['data'] as Map<String, dynamic>;
      return HomeLabelResult(
        labelUrl: (data['labelUrl'] ?? data['label_url'] ?? '').toString(),
        shipmentNo: (data['shipmentNo'] ?? '').toString(),
      );
    }
    final code = resp['code'] is int ? resp['code'] as int : -1;
    final msg = (resp['message'] ?? '產生託運單失敗').toString();
    throw ApiException(code: code, message: msg);
  }

  @Deprecated(
    'Legacy endpoint /order/order/merchant/claim does not exist in gateway',
  )
  Future<Map<String, dynamic>> claimOrder({
    required String orderSn,
    String? sellerId,
    String? sellerName,
  }) async {
    final mid = sellerId ?? _ref.read(sellerStoreIdProvider);
    if (mid == null) throw Exception('Missing sellerId');
    final mname =
        (sellerName ?? _ref.read(myStoreSnapshotProvider)?.storeName ?? '')
            .trim();
    final uri = Uri.parse('${F.appFlavor.appApi}/order/order/merchant/claim')
        .replace(
          queryParameters: {
            'orderSn': orderSn,
            'merchantId': mid,
            if (mname.isNotEmpty) 'sellerName': mname,
          },
        );
    final res = await _dio.post(
      uri.toString(),
      options: Options(validateStatus: (s) => true, headers: _jsonHeaders),
    );
    HKLog.logD('[OrderRepository] claimOrder $uri -> ${res.statusCode}');
    if ((res.statusCode ?? 0) < 200 || (res.statusCode ?? 0) >= 300) {
      throw Exception('HTTP ${res.statusCode}');
    }
    final body = res.data is Map<String, dynamic>
        ? res.data as Map<String, dynamic>
        : null;
    if (body == null) return {'code': -1, 'message': 'Invalid response'};
    final code = int.tryParse(body['code']?.toString() ?? '') ?? -1;
    final msg = body['message']?.toString();
    if (code == 0) return body;
    if (code == 409) throw Exception('409 already claimed');
    if (code == 404) throw Exception('404 not found');
    if (code == 400) throw Exception('400 bad request');
    throw Exception('Server code $code: ${msg ?? 'Unknown error'}');
  }

  // ── 內部工具 ──────────────────────────────────────────

  static List<OrderSummary> _parseOrderList(Map<String, dynamic> resp) {
    final data = resp['data'];
    List<dynamic>? list;
    if (data is Map<String, dynamic>) {
      list =
          data['records'] as List<dynamic>? ??
          data['list'] as List<dynamic>? ??
          data['rows'] as List<dynamic>?;
    }
    if (list == null && data is List<dynamic>) list = data;
    list ??=
        resp['records'] as List<dynamic>? ??
        resp['list'] as List<dynamic>? ??
        resp['rows'] as List<dynamic>?;
    if (list == null) return const [];
    return list
        .whereType<Map>()
        .map((e) => OrderSummary.fromJson(e.cast<String, dynamic>()))
        .toList(growable: false);
  }

  // 身分由 dioProvider 的 auth interceptor 帶（Authorization: Bearer）。
  static const Map<String, dynamic> _jsonHeaders = {
    'Accept': 'application/json',
    'Content-Type': 'application/json',
  };
}

/// 呼叫黑貓取件結果（call_tcat 回傳）
class CallTcatResult {
  const CallTcatResult({required this.callTime, required this.shipmentNo});

  /// 預約取件時間
  final String callTime;

  /// 物流單號
  final String shipmentNo;
}

/// 超商出貨單結果（print_cvs_label 回傳）
class CvsLabelResult {
  const CvsLabelResult({
    required this.shipmentNo,
    required this.partnerId,
    required this.odno,
    required this.validationNo,
    required this.webhookReceived,
  });

  /// 物流單號
  final String shipmentNo;

  /// 合作夥伴 ID
  final String partnerId;

  /// 出貨單編號（8 碼）
  final String odno;

  /// C2C 驗證碼（4 碼）
  final String validationNo;

  /// Webhook 是否已接收
  final bool webhookReceived;

  /// 組合顯示用代碼
  String get displayCode => '$odno$validationNo';
}

/// 宅配託運單結果（generate_home_label 回傳）
class HomeLabelResult {
  const HomeLabelResult({required this.labelUrl, required this.shipmentNo});

  /// 託運單 PDF URL
  final String labelUrl;

  /// 物流單號
  final String shipmentNo;
}

/// 賣家取消訂單結果（seller_cancellations 回傳）
class CancelOrderResult {
  const CancelOrderResult({
    required this.orderNo,
    required this.refundAmount,
    required this.needRefundPage,
  });

  /// 訂單編號
  final String orderNo;

  /// 退款金額（字串，如 "255"）
  final String refundAmount;

  /// 是否需要跳轉退款頁
  final bool needRefundPage;

  /// 退款金額數值（0 表示無退款）
  int get refundAmountInt => int.tryParse(refundAmount) ?? 0;
}

/// 出貨工作日（shipping_business_days 回傳）
class BusinessDay {
  const BusinessDay({required this.date, required this.dayOfWeek});

  /// 日期（yyyy-MM-dd）
  final String date;

  /// 星期幾（一、二、三...）
  final String dayOfWeek;

  /// 解析為 DateTime
  DateTime? toDateTime() => DateTime.tryParse(date);
}

@riverpod
OrderRepository orderRepository(Ref ref) {
  return OrderRepository(ref.watch(dioProvider), ref);
}

/// 依 orderNo 抓 OrderSummary（keepAlive 快取,用於售後卡片內嵌訂單資訊）
@Riverpod(keepAlive: true)
Future<OrderSummary?> orderByNo(Ref ref, String orderNo) async {
  if (orderNo.isEmpty) return null;
  try {
    return await ref
        .read(orderRepositoryProvider)
        .fetchMatchedOrderDetails(orderSn: orderNo);
  } catch (_) {
    return null;
  }
}
