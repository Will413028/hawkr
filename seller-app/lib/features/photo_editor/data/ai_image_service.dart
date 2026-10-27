import 'dart:convert';
import 'dart:typed_data';

import 'package:dio/dio.dart';
import 'package:flutter_image_compress/flutter_image_compress.dart';
import 'package:riverpod_annotation/riverpod_annotation.dart';

import '../../../core/networking/dio_provider.dart';
import '../../../shared/utils/hk_log.dart';
import 'ai_style_model.dart';

part 'ai_image_service.g.dart';

/// 當週 AI 修圖額度用盡時拋出（envelope code 4291）。
class AiQuotaExceededException implements Exception {
  const AiQuotaExceededException(this.message);
  final String message;
  @override
  String toString() => message;
}

/// AI 修圖業務碼。SSOT 是後端 `docs/代碼管理（CodeMeta）/錯誤碼.md` 的
/// 「AI 修圖業務碼（9250~9259）」；這裡只鏡射需要分流的碼值，**文案一律用後端
/// 回傳的 message，不在 App 端複製一份**，否則兩邊會各改各的。
abstract final class AiTransformCode {
  /// 模型該次沒產出圖片。後端已同步重試過一次，使用者再送一次仍可能成功。
  static const int noImage = 9250;

  /// 照片或描述未通過安全性審查——重送同一張圖不會有不同結果。
  static const int safetyBlocked = 9251;

  /// 產出的圖與受版權保護內容過於相似——同上。
  static const int recitation = 9252;

  /// 上游 AI 服務暫時不可用，稍後可再試。
  static const int providerUnavailable = 9253;

  /// 本週額度用完。走 HTTP 429，但 **HTTP 429 不是判準**——Gateway 的限流器也回
  /// 429（envelope code 為 429），只看 status 會把「請求太頻繁」誤判成「額度用
  /// 完」（#2272）。
  static const int quotaExceeded = 4291;

  static const Set<int> known = {
    noImage,
    safetyBlocked,
    recitation,
    providerUnavailable,
  };

  /// 值得原封不動再送一次的碼；其餘是對內容本身的確定性判定，重試無用。
  static const Set<int> retryable = {noImage, providerUnavailable};
}

/// AI 修圖失敗。
///
/// [retryable] 表示「同一張圖原封不動再送一次」是否可能成功，UI 據此決定要引導
/// 重試還是請賣家換照片／改描述——不要再去比對 [message] 字串。
class AiTransformException implements Exception {
  const AiTransformException(this.message, {this.code, this.retryable = false});

  final String message;

  /// 後端業務碼（9250~9253）；判不出來時為 null。
  final int? code;
  final bool retryable;

  @override
  String toString() => message;
}

/// 從 transform 回應判斷是否額度用盡。判準**只有 envelope code 4291**。
///
/// 曾經是「HTTP 429 或 code 4291」，於是 Gateway 限流器的 429（envelope code
/// 429、訊息是英文的 rate limit exceeded）被誤判成額度用完，UI 直接把按鈕鎖成
/// 「本週已用完」並去重抓風格（#2272）。限流走 [rateLimitedFrom]。
/// 是則回傳對應例外（訊息優先取後端 messageZh），否則回傳 null。
AiQuotaExceededException? quotaExceededFrom(int? statusCode, Object? data) {
  if (!(data is Map && data['code'] == AiTransformCode.quotaExceeded)) {
    return null;
  }
  final msg = data['messageZh'] ?? data['message'];
  return AiQuotaExceededException(
    (msg is String && msg.isNotEmpty) ? msg : '本週 AI 美化次數已用完，下週一重置',
  );
}

/// 從 transform 回應判斷是否被限流（HTTP 429 但不是額度用完）。
///
/// 訊息刻意不取後端的——Gateway 限流器產生的是英文 `rate limit exceeded`，
/// 不是寫給賣家看的字。回傳 null 表示不是限流。
AiTransformException? rateLimitedFrom(int? statusCode, Object? data) {
  if (statusCode != 429) return null;
  if (data is Map && data['code'] == AiTransformCode.quotaExceeded) return null;
  return const AiTransformException(
    '操作太頻繁，請稍候幾秒再試',
    code: 429,
    retryable: true,
  );
}

/// 從 `POST /app/image_transformations` 的 `data` 取出圖片 base64。
///
/// 契約鍵**只有 `imageBase64`**：handler 是
/// `response.OK(c, gin.H{"imageBase64": ..., "mimeType": ..., "weeklyUsage": ...,
/// "weeklyLimit": ...})`（gateway/internal/handler/ai.go）——四個鍵、全 camelCase。
/// 這裡曾經多讀一個 `image_base64` 候選鍵，後端從來沒吐過它；留著只會讓下一個人
/// 以為契約有兩種寫法，於是繼續往上疊候選鍵。
String? transformedImageBase64(Object? responseData) {
  if (responseData is! Map) return null;
  final value = responseData['imageBase64'];
  return (value is String && value.isNotEmpty) ? value : null;
}

/// AI 圖片轉換服務
///
/// 透過後端代理呼叫 Google Gemini API，避免在前端暴露 API Key
/// 後端端點: POST /api/v1/app/image_transformations
class AiImageService {
  AiImageService(this._dio);
  final Dio _dio;

  /// 從後端取得 AI 風格列表 + 使用量（依 category 過濾）
  Future<AiStylesResponse?> fetchStyles({String category = 'product'}) async {
    try {
      HKLog.logD('[AiImageService] Fetching AI styles (category=$category)...');
      final response = await _dio.get(
        '/api/v1/app/ai_styles',
        queryParameters: {'category': category},
      );

      if ((response.statusCode ?? 0) < 200 ||
          (response.statusCode ?? 0) >= 300) {
        HKLog.logW('[AiImageService] HTTP Error: ${response.statusCode}');
        return null;
      }

      final Map<String, dynamic> data;
      final responseBody = response.data;
      if (responseBody is Map<String, dynamic>) {
        data = responseBody;
      } else {
        return null;
      }

      final code = data['code'];
      if (code != 0 && code != null) {
        HKLog.logW('[AiImageService] API Error: ${data['message']}');
        return null;
      }

      final responseData = data['data'];
      if (responseData == null) return null;

      return AiStylesResponse.fromJson(responseData as Map<String, dynamic>);
    } catch (e, stackTrace) {
      HKLog.logE('[AiImageService] fetchStyles failed', e, stackTrace);
      return null;
    }
  }

  /// 使用 AI 轉換圖片
  ///
  /// [imageBytes] 原始圖片的 bytes
  /// [mimeType] 圖片 MIME 類型，預設為 image/jpeg
  /// [prompt] 自訂 prompt（可選，使用預設）
  ///
  /// 成功回傳處理後的圖片 bytes；失敗一律拋例外
  /// （[AiQuotaExceededException] / [AiTransformException]）。
  ///
  /// 舊版失敗時回 null，後端好不容易分類出來的原因在這裡整個被丟掉，UI 只能顯示
  /// 一句「AI 編輯失敗，請重試」——連「這張圖沒過安全審查」和「模型這次沒產圖」
  /// 都分不出來。改成拋例外把 message / code / retryable 帶上去。
  Future<Uint8List> transformWithGemini({
    required Uint8List imageBytes,
    String mimeType = 'image/jpeg',
    String? prompt,
  }) async {
    try {
      HKLog.logD('[AiImageService] Calling AI transform API...');

      // 上傳前壓縮：限制在 1920px、85% 品質，避免 gRPC 訊息過大
      final toUpload = await FlutterImageCompress.compressWithList(
        imageBytes,
        minWidth: 1920,
        minHeight: 1920,
        quality: 85,
        keepExif: false,
        format: CompressFormat.jpeg,
      );
      HKLog.logD(
        '[AiImageService] Compressed ${imageBytes.length} → ${toUpload.length} bytes',
      );

      final response = await _dio.post(
        '/api/v1/app/image_transformations',
        data: FormData.fromMap({
          'image': MultipartFile.fromBytes(
            toUpload,
            filename: 'image_${DateTime.now().millisecondsSinceEpoch}.jpg',
            contentType: DioMediaType.parse('image/jpeg'),
          ),
          if (prompt != null && prompt.trim().isNotEmpty) 'prompt': prompt,
        }),
        options: Options(
          validateStatus: (status) => true,
          receiveTimeout: const Duration(seconds: 120),
        ),
      );

      // 額度用盡：必須在通用 non-2xx 檢查之前攔截，否則會被歸成一般失敗，
      // 丟掉「額度用完」這個 UI 要另外處理（同步用量、鎖按鈕）的語意。
      final quotaEx = quotaExceededFrom(response.statusCode, response.data);
      if (quotaEx != null) throw quotaEx;

      // 限流的 429 共用同一個 HTTP status 但語意相反（等一下就能再送），必須
      // 攔在這裡，否則會落進 _failureFrom 把英文的 rate limit exceeded 顯示出來。
      final rateLimitEx = rateLimitedFrom(response.statusCode, response.data);
      if (rateLimitEx != null) throw rateLimitEx;

      final status = response.statusCode ?? 0;
      final responseBody = response.data;
      final data = responseBody is Map<String, dynamic> ? responseBody : null;
      final rawCode = data?['code'];
      final code = rawCode is int ? rawCode : null;

      if (status < 200 ||
          status >= 300 ||
          (code != null && code != 0 && code != 200)) {
        throw _failureFrom(status, code, data);
      }

      final responseData = data?['data'];
      if (responseData == null) {
        HKLog.logW('[AiImageService] No data in response');
        throw const AiTransformException('這次沒有成功產生圖片，請再試一次', retryable: true);
      }

      final imageBase64 = transformedImageBase64(responseData);
      if (imageBase64 != null) {
        try {
          final result = base64Decode(imageBase64);
          HKLog.logD(
            '[AiImageService] Transform successful, received ${result.length} bytes',
          );
          return result;
        } catch (e) {
          HKLog.logW('[AiImageService] Failed to decode base64: $e');
          throw const AiTransformException('AI 美化結果讀取失敗，請重試', retryable: true);
        }
      }

      // 嘗試從 imageUrl 下載圖片
      final imageUrl = responseData['imageUrl'];
      if (imageUrl is String && imageUrl.isNotEmpty) {
        final imageResponse = await _dio.get<List<int>>(
          imageUrl,
          options: Options(
            validateStatus: (status) => true,
            responseType: ResponseType.bytes,
            receiveTimeout: const Duration(seconds: 30),
          ),
        );
        if ((imageResponse.statusCode ?? 0) == 200 &&
            imageResponse.data != null) {
          final bytes = Uint8List.fromList(imageResponse.data!);
          HKLog.logD(
            '[AiImageService] Downloaded from URL, received ${bytes.length} bytes',
          );
          return bytes;
        }
        HKLog.logW(
          '[AiImageService] Failed to download from URL: ${imageResponse.statusCode}',
        );
        throw const AiTransformException('無法下載 AI 美化結果，請重試', retryable: true);
      }

      HKLog.logW('[AiImageService] No image data in response');
      throw const AiTransformException('這次沒有成功產生圖片，請再試一次', retryable: true);
    } on AiQuotaExceededException {
      rethrow;
    } on AiTransformException {
      rethrow;
    } catch (e, stackTrace) {
      HKLog.logE('[AiImageService] Transform failed', e, stackTrace);
      // 連線／逾時／壓縮失敗：沒有後端回應可分類，一律當暫時性失敗。
      final isTimeout =
          e is DioException &&
          (e.type == DioExceptionType.connectionTimeout ||
              e.type == DioExceptionType.receiveTimeout ||
              e.type == DioExceptionType.sendTimeout);
      throw AiTransformException(
        isTimeout ? 'AI 美化時間過長，請稍後再試一次' : '連線失敗，請確認網路後再試一次',
        retryable: true,
      );
    }
  }

  /// 依業務碼組出可顯示的失敗。文案優先取後端的 messageZh / message
  /// （SSOT 在後端的錯誤碼.md），App 端只在後端沒給時才用泛用句。
  AiTransformException _failureFrom(
    int status,
    int? code,
    Map<String, dynamic>? data,
  ) {
    final backendMessage = data?['messageZh'] ?? data?['message'];
    final message = (backendMessage is String && backendMessage.isNotEmpty)
        ? backendMessage
        : _fallbackMessage(status);
    // 已知業務碼查表；其餘退回 transport 判斷——5xx 屬暫時性，4xx 是請求本身
    // 的問題，重送不會有不同結果。
    final retryable = code != null && AiTransformCode.known.contains(code)
        ? AiTransformCode.retryable.contains(code)
        : status >= 500;
    HKLog.logW(
      '[AiImageService] transform failed: status=$status code=$code msg=$message',
    );
    return AiTransformException(message, code: code, retryable: retryable);
  }

  String _fallbackMessage(int status) {
    if (status == 401 || status == 403) return '登入狀態已過期，請重新登入後再試';
    if (status == 413) return '圖片檔案太大，請換一張較小的圖片';
    if (status >= 500) return 'AI 美化暫時無法使用，請稍後再試';
    return '這次沒有成功產生圖片，請再試一次';
  }
}

@riverpod
AiImageService aiImageService(Ref ref) {
  return AiImageService(ref.watch(dioProvider));
}
