import 'package:dio/dio.dart';
import 'package:riverpod_annotation/riverpod_annotation.dart';

import '../../../core/codemeta/code_meta.dart';
import '../../../core/codemeta/code_meta_display.dart';
import '../../../core/networking/dio_provider.dart';
import '../../../shared/utils/hk_log.dart';

part 'payuni_logistics_service.g.dart';

/// PAYUNi 物流查詢服務
///
/// 透過後端代理呼叫 PAYUNi API，避免在前端暴露敏感金鑰
/// 後端端點: POST /api/v1/app/logistics
///
/// 支援查詢：
/// - 7-ELEVEN 超商物流狀態
/// - 黑貓宅配物流狀態
class PayuniLogisticsService {
  PayuniLogisticsService(this._dio);
  final Dio _dio;

  /// 物流類型
  static const String lgsTypeB2C = 'B2C'; // 超商大宗寄倉
  static const String lgsTypeC2C = 'C2C'; // 超商店到店
  static const String lgsTypeHOME = 'HOME'; // 黑貓宅配
  static const String lgsTypeC2B = 'C2B'; // 超商退貨便

  /// 查詢物流狀態
  ///
  /// [shipTradeNo] UNi 物流序號 (B2C/C2C/HOME 使用)
  /// [lgsType] 物流型態: B2C, C2C, HOME, C2B
  /// [tradeType] 宅配類別: 1=正物流, 2=逆物流 (僅 HOME 使用)
  /// [returnOdno] 退貨便編號 (C2B 使用，12碼)
  Future<LogisticsQueryResult> queryLogistics({
    String? shipTradeNo,
    required String lgsType,
    int? tradeType,
    String? returnOdno,
  }) async {
    try {
      HKLog.logD('[PayuniLogistics] Querying logistics: $lgsType');

      // 建立請求 body
      final Map<String, dynamic> body = {'lgsType': lgsType};

      if (shipTradeNo != null && shipTradeNo.isNotEmpty) {
        body['shipTradeNo'] = shipTradeNo;
      }

      if (lgsType == lgsTypeHOME && tradeType != null) {
        body['tradeType'] = tradeType;
      }

      if (lgsType == lgsTypeC2B && returnOdno != null) {
        body['returnOdno'] = returnOdno;
      }

      // 發送請求
      final response = await _dio.post(
        '/api/v1/app/logistics',
        data: body,
        options: Options(
          validateStatus: (s) => true,
          receiveTimeout: const Duration(seconds: 30),
        ),
      );

      if ((response.statusCode ?? 0) != 200) {
        return LogisticsQueryResult.error('HTTP Error: ${response.statusCode}');
      }

      final Map<String, dynamic> responseData;
      if (response.data is Map<String, dynamic>) {
        responseData = response.data as Map<String, dynamic>;
      } else {
        return LogisticsQueryResult.error('回應格式錯誤');
      }

      final code = responseData['code'];
      if (code != 0 && code != null) {
        final message = responseData['message'] ?? '查詢失敗';
        return LogisticsQueryResult.error(message.toString());
      }

      final data = responseData['data'];
      if (data == null) {
        return LogisticsQueryResult.error('回傳資料為空');
      }

      // 檢查後端返回的 status
      final status = data['status']?.toString() ?? '';
      if (status != 'SUCCESS' && status.isNotEmpty) {
        return LogisticsQueryResult.error(
          data['message']?.toString() ?? '查詢失敗',
        );
      }

      return LogisticsQueryResult.success(LogisticsInfo.fromMap(data));
    } catch (e) {
      HKLog.logW('[PayuniLogistics] Query error: $e');
      return LogisticsQueryResult.error('查詢發生錯誤: $e');
    }
  }
}

@riverpod
PayuniLogisticsService payuniLogisticsService(Ref ref) {
  return PayuniLogisticsService(ref.watch(dioProvider));
}

/// 物流查詢結果
class LogisticsQueryResult {
  final bool success;
  final String? errorMessage;
  final LogisticsInfo? data;

  LogisticsQueryResult._({required this.success, this.errorMessage, this.data});

  factory LogisticsQueryResult.success(LogisticsInfo data) {
    return LogisticsQueryResult._(success: true, data: data);
  }

  factory LogisticsQueryResult.error(String message) {
    return LogisticsQueryResult._(success: false, errorMessage: message);
  }
}

/// 物流資訊
class LogisticsInfo {
  final String status;
  final String message;
  final String merID;
  final String partnerId;
  final String merTradeNo;
  final String tradeNo;
  final String shipTradeNo;
  final String odno; // 出貨單編號/宅配託運單號
  final int goodsType; // 1=常溫, 2=冷凍, 3=冷藏
  final String lgsType; // B2C, C2C, HOME
  final int shipType; // 1=7-ELEVEN, 2=黑貓宅配
  final int serviceType; // 1=取貨付款, 3=取貨不付款
  final int shipAmt; // 代收金額
  final String consignee; // 收件人 (隱碼)
  final String consigneeMobile; // 收件人手機 (隱碼)
  final int shipStatus; // 物流貨態狀態碼
  final int? pickupStoreType; // 關轉門市類型
  final String shipStatusDesc; // 貨態說明
  final String shipStatusTime; // 貨態日期
  final int? progressPercent; // 進度百分比 0-100；null = 未知（無對應流程的進度表）

  // 超商相關
  final String? storeID; // 取件門市代碼
  final String? storeName; // 取件門市名稱
  final String? validationNo; // 驗證碼 (C2C)

  // 黑貓宅配相關
  final String? fileNo; // 託運單PDF檔序號
  final int? tradeType; // 1=正物流, 2=逆物流
  final String? consigneeAddress; // 收件人地址

  // 狀態標記
  final bool isFinalStatus; // 是否為最終狀態
  final bool isErrorStatus; // 是否為異常狀態

  LogisticsInfo({
    required this.status,
    required this.message,
    required this.merID,
    required this.partnerId,
    required this.merTradeNo,
    required this.tradeNo,
    required this.shipTradeNo,
    required this.odno,
    required this.goodsType,
    required this.lgsType,
    required this.shipType,
    required this.serviceType,
    required this.shipAmt,
    required this.consignee,
    required this.consigneeMobile,
    required this.shipStatus,
    this.pickupStoreType,
    required this.shipStatusDesc,
    required this.shipStatusTime,
    this.progressPercent,
    this.storeID,
    this.storeName,
    this.validationNo,
    this.fileNo,
    this.tradeType,
    this.consigneeAddress,
    this.isFinalStatus = false,
    this.isErrorStatus = false,
  });

  factory LogisticsInfo.fromMap(Map<String, dynamic> map) {
    final shipStatus =
        _parseInt(map['shipStatus']) ?? _parseInt(map['ShipStatus']) ?? 0;
    final lgsType = (map['lgsType'] ?? map['LgsType'] ?? '').toString();

    return LogisticsInfo(
      status: (map['status'] ?? map['Status'] ?? '').toString(),
      message: (map['message'] ?? map['Message'] ?? '').toString(),
      merID: (map['merID'] ?? map['MerID'] ?? '').toString(),
      partnerId: (map['partnerId'] ?? map['PartnerId'] ?? '').toString(),
      merTradeNo: (map['merTradeNo'] ?? map['MerTradeNo'] ?? '').toString(),
      tradeNo: (map['tradeNo'] ?? map['TradeNo'] ?? '').toString(),
      shipTradeNo: (map['shipTradeNo'] ?? map['ShipTradeNo'] ?? '').toString(),
      odno: (map['odno'] ?? map['Odno'] ?? '').toString(),
      goodsType:
          _parseInt(map['goodsType']) ?? _parseInt(map['GoodsType']) ?? 0,
      lgsType: lgsType,
      shipType: _parseInt(map['shipType']) ?? _parseInt(map['ShipType']) ?? 0,
      serviceType:
          _parseInt(map['serviceType']) ?? _parseInt(map['ServiceType']) ?? 0,
      shipAmt: _parseInt(map['shipAmt']) ?? _parseInt(map['ShipAmt']) ?? 0,
      consignee: (map['consignee'] ?? map['Consignee'] ?? '').toString(),
      consigneeMobile: (map['consigneeMobile'] ?? map['ConsigneeMobile'] ?? '')
          .toString(),
      shipStatus: shipStatus,
      pickupStoreType:
          _parseInt(map['pickupStoreType']) ??
          _parseInt(map['PickupStoreType']),
      shipStatusDesc: (map['shipStatusDesc'] ?? map['ShipStatusDesc'] ?? '')
          .toString(),
      shipStatusTime: (map['shipStatusTime'] ?? map['ShipStatusTime'] ?? '')
          .toString(),
      // 後端沒給、且本 App 無對應進度表（B2C／C2B／未知 lgsType）時留 null，
      // 由呈現層決定怎麼顯示「進度未知」，不得借用別條流程的百分比。
      progressPercent:
          _parseInt(map['progressPercent']) ??
          LogisticsStatusCode.getProgressPercent(shipStatus, lgsType),
      storeID: map['storeID']?.toString() ?? map['StoreID']?.toString(),
      storeName: map['storeName']?.toString() ?? map['StoreName']?.toString(),
      validationNo:
          map['validationNo']?.toString() ?? map['ValidationNo']?.toString(),
      fileNo: map['fileNo']?.toString() ?? map['FileNo']?.toString(),
      tradeType: _parseInt(map['tradeType']) ?? _parseInt(map['TradeType']),
      consigneeAddress:
          map['consigneeAddress']?.toString() ??
          map['ConsigneeAddress']?.toString(),
      isFinalStatus:
          map['isFinalStatus'] == true ||
          LogisticsStatusCode.isFinalStatus(shipStatus),
      isErrorStatus:
          map['isErrorStatus'] == true ||
          LogisticsStatusCode.isErrorStatus(shipStatus),
    );
  }

  static int? _parseInt(dynamic value) {
    if (value == null) return null;
    if (value is int) return value;
    if (value is String) return int.tryParse(value);
    return null;
  }

  /// 取得寄件類型文字
  String get goodsTypeText {
    switch (goodsType) {
      case 1:
        return ShippingTemperature.ambient.label;
      case 2:
        return ShippingTemperature.frozen.label;
      case 3:
        return ShippingTemperature.chilled.label;
      default:
        return '未知';
    }
  }

  /// 取得物流商名稱
  String get shipTypeText {
    switch (shipType) {
      case 1:
        return ShippingMethod.sevenElevenStorePickup.label;
      case 2:
        return ShippingMethod.taqbin.label;
      default:
        return '未知';
    }
  }

  /// 取得取件方式文字
  String get serviceTypeText {
    switch (serviceType) {
      case 1:
        return '取貨付款';
      case 3:
        return '取貨不付款';
      case 4:
        return '退貨付款';
      case 5:
        return '退貨不付款';
      default:
        return '未知';
    }
  }

  /// 取得物流類型文字
  String get lgsTypeText {
    switch (lgsType) {
      case 'B2C':
        return '大宗寄倉';
      case 'C2C':
        return '店到店';
      case 'HOME':
        return '宅配到府';
      case 'C2B':
        return '退貨便';
      default:
        return lgsType;
    }
  }

  String get shipStatusLabel {
    if (shipStatusDesc.isNotEmpty) return shipStatusDesc;
    final raw = shipStatus.toString();
    final label = CodeMetaDisplay.shippingStatus(raw, fallback: '');
    if (label.isNotEmpty && label != raw) return label;
    return LogisticsStatusCode.getStatusText(shipStatus);
  }
}

/// 物流貨態狀態碼對照表
class LogisticsStatusCode {
  static const Map<int, String> statusMap = {
    // 7-ELEVEN 超商
    0: '初始狀態',
    1: '已取號',
    2: '已列印',
    3: '已出貨',
    10: '配送中',
    11: '到店',
    12: '取貨完成',
    13: '退貨中',
    14: '退貨完成',
    20: '逾期未取',
    21: '遺失',
    81: '門市關轉',
    99: '異常',

    // 黑貓宅配
    30: '已收件',
    31: '配送中',
    32: '配達完成',
    33: '無法投遞',
    34: '退回中',
    35: '退回完成',
  };

  /// 取得狀態說明
  static String getStatusText(int code) {
    return statusMap[code] ?? '狀態碼: $code';
  }

  /// 判斷是否為最終狀態
  static bool isFinalStatus(int code) {
    return [12, 14, 20, 21, 32, 35].contains(code);
  }

  /// 判斷是否為異常狀態
  static bool isErrorStatus(int code) {
    return [20, 21, 33, 99].contains(code);
  }

  /// 取得進度百分比 (0-100)；`lgsType` 不是本 App 支援的兩條流程時回 `null`。
  ///
  /// 🔴 原本是 `if (lgsType == 'HOME') {宅配} else {超商}`，於是 `B2C`（大宗寄倉）、
  /// `C2B`（退貨便）與空字串全部被套上**超商 C2C 的百分比**。改成逐值 switch ＋
  /// 明確 `null`（未知），呼叫端負責決定「未知進度」怎麼呈現。
  ///
  /// ⚠️ 這個算式在 `hawkr-website/lib/seller/shipment.ts:progressPercentFor`
  /// 有**逐值相同的第二份**，兩份之間沒有任何 gate 比對（65A §12.2）。改這裡時
  /// 必須同步改那一份，否則兩份各自演化而無人知道。
  static int? getProgressPercent(int code, String lgsType) {
    switch (lgsType) {
      case PayuniLogisticsService.lgsTypeHOME:
        return _homeProgressPercent(code);
      case PayuniLogisticsService.lgsTypeC2C:
        return _cvsProgressPercent(code);
      // B2C（大宗寄倉）／C2B（退貨便）流程與 C2C 不同，本 App 沒有對應的進度表；
      // 回 null 而不是借用 C2C 的百分比。
      case PayuniLogisticsService.lgsTypeB2C:
      case PayuniLogisticsService.lgsTypeC2B:
        return null;
      default:
        return null;
    }
  }

  /// 黑貓宅配進度（`lgsType == HOME`）。
  static int _homeProgressPercent(int code) {
    switch (code) {
      case 0:
        return 0;
      case 1:
      case 2:
        return 15;
      case 30:
        return 30;
      case 31:
        return 60;
      case 32:
        return 100;
      case 33:
      case 34:
        return 80;
      case 35:
        return 100;
      default:
        return 50;
    }
  }

  /// 超商店到店進度（`lgsType == C2C`）。
  static int _cvsProgressPercent(int code) {
    switch (code) {
      case 0:
        return 0;
      case 1:
        return 10;
      case 2:
        return 20;
      case 3:
        return 35;
      case 10:
        return 50;
      case 11:
        return 80;
      case 12:
        return 100;
      case 13:
        return 85;
      case 14:
        return 100;
      case 20:
      case 21:
      case 99:
        return 100;
      default:
        return 50;
    }
  }
}
