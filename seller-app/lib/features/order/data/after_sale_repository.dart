import 'package:dio/dio.dart';
import 'package:riverpod_annotation/riverpod_annotation.dart';
import '../../../core/networking/dio_provider.dart';
import '../../../core/exceptions/app_exception.dart';
import 'after_sale_model.dart';

part 'after_sale_repository.g.dart';

@riverpod
AfterSaleRepository afterSaleRepository(Ref ref) =>
    AfterSaleRepository(ref.watch(dioProvider));

/// Resolve API 的回傳結果
class ResolveResult {
  final String caseNo;
  final String status;
  final bool needRefundPage;
  final String returnShipmentNo; // 退貨物流單號（7-11退貨代碼 / 黑貓託運單號）

  const ResolveResult({
    required this.caseNo,
    required this.status,
    required this.needRefundPage,
    required this.returnShipmentNo,
  });
}

String _msg(Map<String, dynamic> body, String fallback) =>
    (body['message'] ?? fallback).toString();

class AfterSaleRepository {
  final Dio _dio;
  AfterSaleRepository(this._dio);

  /// GET /app/seller/after_sales — 賣家售後列表
  Future<({List<AfterSaleCase> records, int total})> getAfterSaleList({
    int page = 1,
    int pageSize = 20,
    String? status,
  }) async {
    try {
      final params = <String, dynamic>{'page': page, 'page_size': pageSize};
      if (status != null) params['status'] = status;

      final resp = await _dio.get<Map<String, dynamic>>(
        '/api/v1/app/seller/after_sales',
        queryParameters: params,
      );
      final body = resp.data!;
      if (body['code'] != 0) {
        throw ServerException(message: _msg(body, '查詢售後列表失敗'), statusCode: 0);
      }
      final data = body['data'] as Map<String, dynamic>? ?? {};
      final rawList = data['records'] as List? ?? [];
      final records = rawList
          .whereType<Map>()
          .map((e) => AfterSaleCase.fromJson(Map<String, dynamic>.from(e)))
          .toList();
      final total =
          int.tryParse(data['total']?.toString() ?? '') ?? records.length;
      return (records: records, total: total);
    } on DioException catch (e) {
      if (e.response?.statusCode == 401) {
        throw const UnauthorizedException(message: '未授權');
      }
      // 404 = API 尚未部署，回傳空列表
      if (e.response?.statusCode == 404) {
        return (records: <AfterSaleCase>[], total: 0);
      }
      if (e.response?.statusCode == 404) {
        throw const ServerException(message: '售後服務功能即將上線', statusCode: 404);
      }
      throw NetworkException(message: e.message ?? '網路錯誤');
    }
  }

  /// GET /app/seller/after_sales/:case_no — 賣家售後詳情（後端用 caseNo 當 path）
  Future<AfterSaleCase> getAfterSaleDetail(String caseNo) async {
    try {
      final resp = await _dio.get<Map<String, dynamic>>(
        '/api/v1/app/seller/after_sales/$caseNo',
      );
      final body = resp.data!;
      if (body['code'] != 0) {
        throw ServerException(message: _msg(body, '查詢售後詳情失敗'), statusCode: 0);
      }
      return AfterSaleCase.fromJson(
        body['data'] as Map<String, dynamic>? ?? {},
      );
    } on DioException catch (e) {
      if (e.response?.statusCode == 401) {
        throw const UnauthorizedException(message: '未授權');
      }
      if (e.response?.statusCode == 404) {
        throw const ServerException(message: '售後服務功能即將上線', statusCode: 404);
      }
      throw NetworkException(message: e.message ?? '網路錯誤');
    }
  }

  /// POST /app/seller/after_sales/:case_no/resolve — 提交決策
  /// 回傳 ResolveResultResponse: { caseNo, status, needRefundPage, returnShipmentNo }
  Future<ResolveResult> resolve(
    String caseNo, {
    required String resolutionType,
    String? returnShippingMethod,
    String? refundAmount,
    String? couponAmount,
    String? couponMinSpend,
    String? couponMessage,
    String? rejectReason,
    String? rejectMessage,
  }) async {
    try {
      final data = <String, dynamic>{'resolutionType': resolutionType};
      if (returnShippingMethod != null) {
        data['returnShippingMethod'] = returnShippingMethod;
      }
      if (refundAmount != null) data['refundAmount'] = refundAmount;
      if (couponAmount != null) data['couponAmount'] = couponAmount;
      if (couponMinSpend != null) data['couponMinSpend'] = couponMinSpend;
      if (couponMessage != null) data['couponMessage'] = couponMessage;
      if (rejectReason != null) data['rejectReason'] = rejectReason;
      if (rejectMessage != null) data['rejectMessage'] = rejectMessage;

      final resp = await _dio.post<Map<String, dynamic>>(
        '/api/v1/app/seller/after_sales/$caseNo/resolve',
        data: data,
      );
      final body = resp.data!;
      if (body['code'] != 0) {
        throw ServerException(message: _msg(body, '提交決策失敗'), statusCode: 0);
      }
      final d = body['data'] as Map<String, dynamic>? ?? {};
      return ResolveResult(
        caseNo: (d['caseNo'] ?? '').toString(),
        status: (d['status'] ?? '').toString(),
        needRefundPage: d['needRefundPage'] == true,
        returnShipmentNo: (d['returnShipmentNo'] ?? '').toString(),
      );
    } on DioException catch (e) {
      if (e.response?.statusCode == 401) {
        throw const UnauthorizedException(message: '未授權');
      }
      if (e.response?.statusCode == 404) {
        throw const ServerException(message: '售後服務功能即將上線', statusCode: 404);
      }
      // 透傳後端 envelope message（業務碼如 9025 補償券發放失敗 / 9011 退貨建單失敗 /
      // 9006 退款金額超限 / 9007 優惠券規則違反 等，後端已帶具體原因），而非 Dio 的 HTTP
      // status 文字 —— 否則賣家只看到「Http status error [409]」無法得知業務失敗原因（#1150）。
      final body = e.response?.data;
      if (body is Map<String, dynamic> && body['message'] != null) {
        throw ServerException(
          message: _msg(body, '提交決策失敗'),
          statusCode: e.response?.statusCode ?? 0,
        );
      }
      throw NetworkException(message: e.message ?? '網路錯誤');
    }
  }

  /// POST /app/seller/after_sales/:case_no/confirm_receipt — 確認收貨／驗貨後退款
  Future<void> confirmReceipt(
    String caseNo, {
    String? actualRefundAmount,
    String? deductionReason,
    String? inspectionNote,
    List<String>? inspectionPhotoTokens,
  }) async {
    try {
      final data = <String, dynamic>{};
      if (actualRefundAmount != null && actualRefundAmount.isNotEmpty) {
        data['actualRefundAmount'] = actualRefundAmount;
      }
      if (deductionReason != null && deductionReason.isNotEmpty) {
        data['deductionReason'] = deductionReason;
      }
      if (inspectionNote != null && inspectionNote.isNotEmpty) {
        data['inspectionNote'] = inspectionNote;
      }
      if (inspectionPhotoTokens != null && inspectionPhotoTokens.isNotEmpty) {
        data['inspectionPhotoTokens'] = inspectionPhotoTokens;
      }

      final resp = await _dio.post<Map<String, dynamic>>(
        '/api/v1/app/seller/after_sales/$caseNo/confirm_receipt',
        data: data.isEmpty ? null : data,
      );
      final body = resp.data!;
      if (body['code'] != 0) {
        throw ServerException(message: _msg(body, '確認收貨失敗'), statusCode: 0);
      }
    } on DioException catch (e) {
      if (e.response?.statusCode == 401) {
        throw const UnauthorizedException(message: '未授權');
      }
      if (e.response?.statusCode == 404) {
        throw const ServerException(message: '售後服務功能即將上線', statusCode: 404);
      }
      throw NetworkException(message: e.message ?? '網路錯誤');
    }
  }

  /// POST /app/orders/:order_no/seller_cancellations — 賣家取消出貨
  Future<({String orderNo, String refundAmount, bool needRefundPage})>
  cancelShipment(
    String orderNo, {
    required String reason,
    String? remark,
  }) async {
    try {
      final data = <String, dynamic>{'reason': reason};
      if (remark != null && remark.isNotEmpty) data['remark'] = remark;

      final resp = await _dio.post<Map<String, dynamic>>(
        '/api/v1/app/orders/$orderNo/seller_cancellations',
        data: data,
      );
      final body = resp.data!;
      if (body['code'] != 0) {
        final code = body['code'];
        if (code == 9008) {
          throw const ServerException(message: '訂單狀態不允許取消出貨', statusCode: 400);
        }
        if (code == 9009) {
          throw const ServerException(message: '該訂單已有取消出貨申請', statusCode: 400);
        }
        throw ServerException(message: _msg(body, '取消出貨失敗'), statusCode: 0);
      }
      final d = body['data'] as Map<String, dynamic>? ?? {};
      return (
        orderNo: (d['orderNo'] ?? orderNo).toString(),
        refundAmount: (d['refundAmount'] ?? '0').toString(),
        needRefundPage: d['needRefundPage'] == true,
      );
    } on DioException catch (e) {
      if (e.response?.statusCode == 401) {
        throw const UnauthorizedException(message: '未授權');
      }
      if (e.response?.statusCode == 404) {
        throw const ServerException(message: '售後服務功能即將上線', statusCode: 404);
      }
      throw NetworkException(message: e.message ?? '網路錯誤');
    }
  }
}
