import 'package:dio/dio.dart';
import 'package:riverpod_annotation/riverpod_annotation.dart';
import '../../../core/networking/dio_provider.dart';
import '../../../core/exceptions/app_exception.dart';
import 'logistics_track.dart';
import 'order_summary.dart' as legacy;
import 'order_model.dart';

part 'order_repository.g.dart';

@riverpod
OrderRepository orderRepository(Ref ref) =>
    OrderRepository(ref.watch(dioProvider));

class OrderRepository {
  final Dio _dio;
  OrderRepository(this._dio);

  Future<({List<Order> records, int total})> getOrders({
    String? status,
    int page = 1,
    int pageSize = 10,
  }) async {
    try {
      final queryParams = <String, dynamic>{
        'page': '$page',
        'page_size': '$pageSize',
      };
      if (status != null && status.isNotEmpty) {
        queryParams['status'] = status;
      }
      final resp = await _dio.get<Map<String, dynamic>>(
        '/app/orders',
        queryParameters: queryParams,
      );
      final body = resp.data!;
      if (body['code'] != 0) {
        throw ServerException((body['message'] ?? '請求失敗').toString());
      }
      final data = body['data'];
      final list = (data is Map ? data['records'] : data) as List? ?? [];
      final total = data is Map
          ? int.tryParse(data['total']?.toString() ?? '') ?? 0
          : list.length;
      final records = list.map((e) {
        final m = Map<String, dynamic>.from(e as Map);
        // 49 §6.1 成對契約：gateway `OrderListItemResponse` 只回 logoKey（裸
        // objectKey，SSOT）+ logoUrl（Gateway 組的顯示 URL）。`Order.sellerAvatarUrl`
        // 綁的 JSON key 是 logoUrl，顯示端 OssImage 是 dual-mode，所以這裡把裸 key
        // 覆蓋進去以取得 preset 版而非 /i/original/。其餘八個舊候選（avatar / logo /
        // sellerLogo / snake 系）wire 上都不存在，已移除。
        final logoKey = (m['logoKey'] ?? '').toString();
        if (logoKey.isNotEmpty) m['logoUrl'] = logoKey;
        return Order.fromJson(m);
      }).toList();
      return (records: records, total: total);
    } on DioException catch (e) {
      if (e.response?.statusCode == 401) throw const UnauthorizedException();
      throw const NetworkException();
    }
  }

  Future<Order> getOrderDetail(String orderSn) async {
    try {
      final resp = await _dio.get<Map<String, dynamic>>('/app/orders/$orderSn');
      final body = resp.data!;
      if (body['code'] != 0) {
        throw ServerException((body['message'] ?? '請求失敗').toString());
      }
      return Order.fromJson(body['data'] as Map<String, dynamic>);
    } on DioException catch (e) {
      if (e.response?.statusCode == 401) throw const UnauthorizedException();
      throw const NetworkException();
    }
  }

  /// GET /app/orders/{order_no}/logistics — 買家物流追蹤。
  ///
  /// 🔴 後端會**即時**向 provider 同步當前狀態再回傳，所以這支比讀 order 上的
  /// `shippingStatus` 快照新。Ownership 由 shipping-service 以 order_no 對應的
  /// buyer 驗證，前端不需要（也不得）帶任何身分參數。
  ///
  /// ⚠️ 沒有物流單的訂單（尚未出貨、或建單還沒成功）後端回非零業務碼，
  /// 這裡照常丟 [ServerException]——呼叫端要先判斷「這張單有沒有物流」再打。
  Future<LogisticsTrack> getOrderLogistics(String orderNo) async {
    try {
      final resp = await _dio.get<Map<String, dynamic>>(
        '/app/orders/$orderNo/logistics',
      );
      final body = resp.data!;
      if (body['code'] != 0) {
        throw ServerException((body['message'] ?? '查詢物流狀態失敗').toString());
      }
      final data = body['data'];
      if (data is! Map) throw const ServerException('查詢物流狀態失敗');
      return LogisticsTrack.fromWire(orderNo, Map<String, dynamic>.from(data));
    } on DioException catch (e) {
      if (e.response?.statusCode == 401) throw const UnauthorizedException();
      throw const NetworkException();
    }
  }

  Future<Order> submitOrder(
    Map<String, dynamic> orderData, {
    Map<String, String>? extraHeaders,
  }) async {
    try {
      final resp = await _dio.post<Map<String, dynamic>>(
        '/app/orders',
        data: orderData,
        options: extraHeaders != null ? Options(headers: extraHeaders) : null,
      );
      final body = resp.data!;
      if (body['code'] != 0) {
        throw ServerException((body['message'] ?? '下單失敗').toString());
      }
      return Order.fromJson(body['data'] as Map<String, dynamic>);
    } on DioException catch (e) {
      if (e.response?.statusCode == 401) throw const UnauthorizedException();
      throw const NetworkException();
    }
  }

  /// Fetch orders with items as raw maps (for lightweight filtering without Freezed models).
  Future<List<Map<String, dynamic>>> getOrdersRaw({int page = 1}) async {
    try {
      final resp = await _dio.get<Map<String, dynamic>>(
        '/app/orders',
        queryParameters: {'page': page, 'page_size': 50},
      );
      final body = resp.data!;
      final data = body['data'];
      List<dynamic> list = [];
      if (data is List) {
        list = data;
      } else if (data is Map) {
        list = data['records'] as List? ?? [];
      }
      return list
          .whereType<Map>()
          .map((e) => Map<String, dynamic>.from(e))
          .toList();
    } catch (_) {
      return <Map<String, dynamic>>[];
    }
  }

  /// POST /app/orders/{order_no}/confirm_receipt — 買家確認收貨
  /// POST /app/orders/{order_no}/confirm_receipt — 確認收貨
  /// [stagedTokens] 為 media-service 預上傳取得的 token，後端會把照片轉正
  /// 並寫入 order.receiptPhotoUrls。沒附照片時可省略。
  Future<bool> confirmReceipt(
    String orderNo, {
    List<String> stagedTokens = const [],
  }) async {
    try {
      final resp = await _dio.post<Map<String, dynamic>>(
        '/app/orders/$orderNo/confirm_receipt',
        data: stagedTokens.isEmpty ? null : {'stagedTokens': stagedTokens},
      );
      final body = resp.data!;
      if (body['code'] != 0) {
        throw ServerException((body['message'] ?? '請求失敗').toString());
      }
      return true;
    } on DioException catch (e) {
      if (e.response?.statusCode == 401) throw const UnauthorizedException();
      throw const NetworkException();
    }
  }

  /// GET /app/orders?status=60,70 — 取得有退款的訂單列表（售後服務列表用）
  Future<List<Order>> getRefundOrders({int page = 1, int pageSize = 20}) async {
    try {
      final resp = await _dio.get<Map<String, dynamic>>(
        '/app/orders',
        queryParameters: {
          'status': '20,30',
          'page': page,
          'page_size': pageSize,
        },
      );
      final body = resp.data!;
      if (body['code'] != 0) {
        throw ServerException((body['message'] ?? '查詢售後訂單失敗').toString());
      }
      final data = body['data'];
      List<dynamic> records = [];
      if (data is List) {
        records = data;
      } else if (data is Map) {
        records = data['records'] as List? ?? [];
      }
      return records
          .whereType<Map>()
          .map((e) => Order.fromJson(Map<String, dynamic>.from(e)))
          .toList();
    } on DioException catch (e) {
      if (e.response?.statusCode == 401) throw const UnauthorizedException();
      throw const NetworkException();
    }
  }

  /// Fetch orders with items, parsed into legacy [OrderSummary] objects.
  /// Mirrors the old `OrderService.fetchAvailableOrders` / `ApiService.orderListWithItem`.
  Future<List<legacy.OrderSummary>> fetchAvailableOrders({
    int page = 1,
    int limit = 20,
    bool withItems = true,
  }) async {
    try {
      final resp = await _dio.get<Map<String, dynamic>>(
        '/app/orders',
        queryParameters: {'page': page, 'page_size': limit},
      );
      final body = resp.data!;
      final data = body['data'];
      List<dynamic> list = [];
      if (data is List) {
        list = data;
      } else if (data is Map) {
        list = data['records'] as List? ?? [];
      }
      if (list.isEmpty) return <legacy.OrderSummary>[];

      String pickStr(Map<String, dynamic> m, List<String> keys) {
        for (final k in keys) {
          final v = m[k];
          if (v == null) continue;
          final s = v.toString();
          if (s.isNotEmpty) return s;
        }
        return '';
      }

      // 以下所有欄名以 gateway `OrderSummaryResponse` 為準
      // （backend-payuni `gateway/internal/handler/order.go:137-186`）。
      // 這個 parser 過去對每個值都寫五到八個候選，實際只有一個存在——其中
      // merchantName / merchantAvatar / price 的候選鏈**整條落空**，商家名、
      // 商家頭像、金額三欄一直是空的。
      String statusOf(Map<String, dynamic> m) => pickStr(m, const ['status']);

      String firstItemName(Map<String, dynamic> m) {
        final items = m['items'];
        if (items is List && items.isNotEmpty && items.first is Map) {
          final fm = Map<String, dynamic>.from(items.first as Map);
          return pickStr(fm, const ['productName']);
        }
        return '';
      }

      String priceOf(Map<String, dynamic> m) {
        final v = m['totalActualAmount'];
        if (v == null) return '';
        return v.toString().replaceAll(RegExp(r'[^0-9.-]'), '');
      }

      String dateOf(Map<String, dynamic> m) => pickStr(m, const ['createTime']);

      final out = <legacy.OrderSummary>[];
      for (final it in list) {
        if (it is! Map) continue;
        final m = Map<String, dynamic>.from(it);
        final status = statusOf(m);
        if (status.trim().isEmpty) continue;
        final id = pickStr(m, const ['orderNo']);
        if (id.isEmpty) continue;

        final merchantId = pickStr(m, const ['storeId']);
        final merchantName = pickStr(m, const ['sellerName']);
        // 49 §6.1 成對契約：裸 key 優先（顯示端 scene 走 buildImageUrl dual-mode）。
        final merchantAvatar = pickStr(m, const ['logoKey', 'logoUrl']);

        final parsedItems = <legacy.OrderItem>[];
        final candLists = [m['items']];
        for (final c in candLists) {
          if (c is List) {
            for (final e in c) {
              if (e is Map) {
                final em = Map<String, dynamic>.from(e);
                String pickStrItem(List<String> keys) {
                  for (final k in keys) {
                    final v = em[k];
                    if (v == null) continue;
                    final s = v.toString();
                    if (s.isNotEmpty) return s;
                  }
                  return '';
                }

                int pickInt(List<String> keys, {int def = 1}) {
                  for (final k in keys) {
                    final v = em[k];
                    if (v == null) continue;
                    if (v is int) return v;
                    if (v is num) return v.toInt();
                    final p = int.tryParse(v.toString());
                    if (p != null) return p;
                  }
                  return def;
                }

                String pickPrice(List<String> keys) {
                  for (final k in keys) {
                    final v = em[k];
                    if (v == null) continue;
                    final cleaned = v.toString().replaceAll(
                      RegExp(r'[^0-9.-]'),
                      '',
                    );
                    if (cleaned.isNotEmpty) return cleaned;
                  }
                  return '';
                }

                // gateway `OrderItemViewResponse`（order.go:195-206）。
                final name = pickStrItem(const ['productName']);
                final qty = pickInt(const ['quantity'], def: 1);
                final price = pickPrice(const ['unitPrice']);
                // 49 號成對契約：優先裸 key。
                final imageUrl = pickStrItem(const ['imageKey', 'imageUrl']);
                final skuId = pickStrItem(const ['skuId']);
                parsedItems.add(
                  legacy.OrderItem(
                    name: name,
                    quantity: qty,
                    price: price,
                    imageUrl: imageUrl.isNotEmpty ? imageUrl : null,
                    skuId: skuId.isNotEmpty ? skuId : null,
                  ),
                );
              }
            }
            if (parsedItems.isNotEmpty) break;
          }
        }

        // 訂單摘要 root 沒有圖片／SKU 欄位，兩者只能來自 items[0]
        // （原本 root 層的八個 / 五個候選 wire 上一個都不存在）。
        final mainImageUrl = parsedItems.isNotEmpty
            ? (parsedItems.first.imageUrl ?? '')
            : '';
        final String? mainSkuId =
            (parsedItems.isNotEmpty &&
                (parsedItems.first.skuId ?? '').isNotEmpty)
            ? parsedItems.first.skuId
            : null;

        out.add(
          legacy.OrderSummary(
            id: id,
            orderSn: id,
            statusCode: pickStr(m, const ['status']),
            title: firstItemName(m),
            subtitle: status.isNotEmpty ? status : 'Matched',
            dateTime: dateOf(m),
            productName: firstItemName(m),
            price: priceOf(m),
            imageUrl: mainImageUrl.isNotEmpty ? mainImageUrl : null,
            mainSkuId: mainSkuId,
            items: parsedItems,
            merchantId: merchantId,
            merchantName: merchantName,
            merchantAvatar: merchantAvatar,
          ),
        );
      }
      return out;
    } on DioException catch (e) {
      if (e.response?.statusCode == 401) throw const UnauthorizedException();
      throw const NetworkException();
    } catch (_) {
      return <legacy.OrderSummary>[];
    }
  }
}
