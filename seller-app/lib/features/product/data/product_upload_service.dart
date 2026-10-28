import 'dart:async';

import 'package:flutter/foundation.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:uuid/uuid.dart';

import 'package:dio/dio.dart';
import '../../../core/analytics/analytics_events.dart';
import '../../../core/networking/dio_provider.dart';
import '../../../shared/utils/secret_redaction.dart';
import '../../media/data/media_repository.dart';
import 'hashtag_extractor.dart';

/// 商品上傳任務狀態
enum UploadStatus { pending, uploading, success, failed }

/// SKU / 商品樂觀鎖版本衝突的業務碼。
///
/// SSOT 是後端 `docs/代碼管理（CodeMeta）/錯誤碼.md` 的
/// `DELEGATION_RESOURCE_VERSION_CONFLICT`（訊息「資料已被更新，請重新載入後再試」）。
///
/// dev 實測（2026-08-22）：`PUT /app/products/{id}` 帶過期的 `skus[].expectedVersion`
/// 回 **HTTP 409 ＋ envelope `{"code":9266,...}`**。判定一律讀 body 的 `code`，不看 HTTP
/// 狀態碼——409 是 Gateway 對「未登錄業務碼」的塌縮預設值，同一個 409 底下還有其他碼。
const int kVersionConflictBizCode = 9266;

/// 商品上傳任務
class ProductUploadTask {
  final String id;
  final String productName;
  final DateTime createdAt;
  UploadStatus status;
  String? errorMessage;

  /// 後端回傳的業務碼（僅失敗時有值）。
  ///
  /// 存在的理由：呼叫端要對 [kVersionConflictBizCode] 給出「重新載入」這個**可行的下一步**，
  /// 而不是彈一個吐司讓使用者用同一份過期版本一再重試（那會永遠失敗）。
  /// 用訊息字串比對是不可靠的——訊息由後端 CodeMeta 決定、隨時可改。
  int? errorCode;
  double progress; // 0.0 - 1.0

  /// 給使用者看的錯誤訊息。
  ///
  /// [errorMessage] 是 `e.toString()`，對 `Exception('商品需先下架後再編輯')` 會得到
  /// **「Exception: 商品需先下架後再編輯」**——`Exception:` 這五個字對賣家沒有意義，
  /// 只是把 Dart 的型別名字漏到畫面上。這裡剝掉它，讓後端的中文訊息乾淨呈現。
  String? get displayMessage {
    final m = errorMessage?.trim();
    if (m == null || m.isEmpty) return null;
    for (final prefix in const ['Exception: ', 'Exception:']) {
      if (m.startsWith(prefix)) return m.substring(prefix.length).trim();
    }
    return m;
  }

  ProductUploadTask({
    required this.id,
    required this.productName,
    required this.createdAt,
    this.status = UploadStatus.pending,
    this.errorMessage,
    this.errorCode,
    this.progress = 0.0,
  });
}

/// 商品上傳資料
class ProductUploadData {
  final List<Uint8List> images;
  final String? productId; // 新 API: 商品 TSID（編輯時使用）
  final Object? skuId; // 舊 API: SKU ID（向後相容）
  final String productName;
  final String category; // L1 名稱 (如「水果」)
  final String subCategory; // L2 名稱 (如「柚子」) 或 L3 名稱
  final int? catalogId; // L3 分類 ID（正式 cat_id 或 candidate_id）
  final bool isCandidateCategory;
  // 完整分類資訊
  final int? l1Id;
  final String? l1Name;
  final int? l2Id;
  final String? l2Name;
  final String? l3Name; // L3 分類名稱
  final String? origin;
  final String? description;
  final List<Map<String, dynamic>>? descriptionBlocks;
  final List<String> descriptionStagedTokens;
  final bool isEditMode;
  final Map<String, dynamic>? existingProduct;
  final List<SpecData> specs;
  final String sellerId;

  ProductUploadData({
    required this.images,
    this.productId,
    this.skuId,
    required this.productName,
    required this.category,
    required this.subCategory,
    this.catalogId,
    this.isCandidateCategory = false,
    this.l1Id,
    this.l1Name,
    this.l2Id,
    this.l2Name,
    this.l3Name,
    this.origin,
    this.description,
    this.descriptionBlocks,
    this.descriptionStagedTokens = const [],
    this.isEditMode = false,
    this.existingProduct,
    required this.specs,
    required this.sellerId,
  });
}

/// SKU 規格資料（對應後端 CreateSkuInput / UpdateSkuInput）
class SpecData {
  /// 既有 SKU 的後端 id（新增的規格為 null）。
  ///
  /// **不送它，編輯就不是編輯**：`PUT /app/products/{id}` 對沒有 id 的 SKU 一律走
  /// INSERT，並把不在保留清單裡的舊 SKU 實體刪除（product-service `updateSkus`）。
  /// 於是每次存檔都是「全刪既有 SKU、全插一批新 TSID」——樂觀鎖 CAS 從不執行，
  /// 而 cart_items.sku_id / order_items.sku_id 會靜默指向一個已不存在的列
  /// （product_sku 沒有任何 FK 指向它，所以資料庫不會攔）。
  final String? skuId;

  /// 這筆 SKU 讀取當下的樂觀鎖版本（來自 GET 詳情的 `skus[].version`）。
  ///
  /// 必須與 [skuId] 成對送出：後端只在 UPDATE 分支讀 expectedVersion，
  /// 沒有 id 就走 INSERT ⇒ 單獨送 expectedVersion 會被**靜默丟棄**（HTTP 200、無 log）。
  final String? skuVersion;

  final String grade; // → skuName
  final int price; // → price (TWD 整數字串)
  /// PayUni #758: SKU 特價（0 或 null = 無特價，編輯時不送 = 後端清空）
  final int? salePrice;
  final int stock; // → stockQty
  final double? weight; // → weight
  final String
  unit; // → unit (WeightUnit value: 3301=KG, 3302=CATTY, 3303=GRAM)
  final double? lengthCm; // → lengthCm
  final double? widthCm; // → widthCm
  final double? heightCm; // → heightCm
  final List<Map<String, dynamic>>
  shippingMethods; // → shippingMethods (SKU 層級)

  SpecData({
    this.skuId,
    this.skuVersion,
    required this.grade,
    required this.price,
    this.salePrice,
    required this.stock,
    this.weight,
    this.unit = '3301', // WeightUnit.kg.value
    this.lengthCm,
    this.widthCm,
    this.heightCm,
    this.shippingMethods = const [],
  });

  static Map<String, String> shippingMethodJson({
    required String type,
    required String feePayer,
    required String freeThreshold,
  }) => {'type': type, 'feePayer': feePayer, 'freeThreshold': freeThreshold};

  /// 轉換為後端 SKU API 格式 (camelCase)
  ///
  /// PayUni #758 規則：
  /// - 永遠送 price
  /// - salePrice 有效（> 0 且 < price）時送 salePrice
  /// - salePrice 為 null / 0 / >= price 時 **不送** salePrice 欄位
  ///   （編輯模式下不送即視為清空特價）
  ///
  /// id / expectedVersion 規則：
  /// - [skuId] 空 = 這是新增的規格，兩個欄位都不送（維持 INSERT 語意）
  /// - [skuId] 非空才送 `id`；`expectedVersion` 只在 id 也在時才有意義，
  ///   單獨送會被後端靜默丟棄，所以兩者綁在同一個條件下
  Map<String, dynamic> toSkuJson() {
    final hasSale = salePrice != null && salePrice! > 0 && salePrice! < price;
    final id = skuId?.trim() ?? '';
    final version = skuVersion?.trim() ?? '';
    return {
      if (id.isNotEmpty) 'id': id,
      if (id.isNotEmpty && version.isNotEmpty) 'expectedVersion': version,
      'skuName': grade,
      'weight': weight != null ? weight.toString() : '0',
      'unit': unit,
      'price': price.toString(),
      if (hasSale) 'salePrice': salePrice!.toString(),
      'stockQty': stock.toString(),
      if (lengthCm != null && lengthCm! > 0)
        'lengthCm': lengthCm!.toStringAsFixed(0),
      if (widthCm != null && widthCm! > 0)
        'widthCm': widthCm!.toStringAsFixed(0),
      if (heightCm != null && heightCm! > 0)
        'heightCm': heightCm!.toStringAsFixed(0),
      if (shippingMethods.isNotEmpty) 'shippingMethods': shippingMethods,
    };
  }

  /// 向後相容別名
  Map<String, dynamic> toVariantJson() => toSkuJson();
}

/// 商品上傳服務 - 後台處理上傳任務
///
/// 使用新版 Product Service API（POST /api/v1/merchant/products）
/// 一次 API 呼叫建立商品，包含圖片（staged tokens）、規格、認證等
class ProductUploadService {
  static final ProductUploadService _instance =
      ProductUploadService._internal();
  factory ProductUploadService() => _instance;
  ProductUploadService._internal();

  static const _uuid = Uuid();

  Dio? _dio;
  MediaRepository? _mediaRepo;

  /// Product gateway calls and media-session calls share the central app Dio,
  /// so 401 logout and 9240 suspension behavior stay consistent.
  ///
  /// [MediaRepository] still creates an explicit unauthenticated Dio only for
  /// the OSS presigned PUT boundary.
  void useHttpClient(Dio dio) {
    if (identical(_dio, dio)) return;
    _dio = dio;
    _mediaRepo = MediaRepository(dio);
  }

  Dio get _httpClient {
    final dio = _dio;
    if (dio == null) {
      throw StateError(
        'ProductUploadService requires productUploadServiceProvider',
      );
    }
    return dio;
  }

  MediaRepository get _mediaRepository {
    final repository = _mediaRepo;
    if (repository == null) {
      throw StateError(
        'ProductUploadService requires productUploadServiceProvider',
      );
    }
    return repository;
  }

  /// POST /api/v1/app/products
  Future<Map<String, dynamic>> _createProductV2({
    required String idempotencyKey,
    required String name,
    required String categoryId,
    String? origin,
    String? description,
    List<Map<String, dynamic>>? descriptionBlocks,
    List<Map<String, dynamic>>? images,
    List<Map<String, dynamic>>? skus,
    String? status,
    List<String>? hashtags,
  }) async {
    Map<String, dynamic>? descriptionInput;
    if (descriptionBlocks != null && descriptionBlocks.isNotEmpty) {
      descriptionInput = {'version': '1.0', 'blocks': descriptionBlocks};
    } else if (description != null && description.isNotEmpty) {
      descriptionInput = {
        'version': '1.0',
        'blocks': [
          {'type': 'text', 'content': description},
        ],
      };
    }

    try {
      final resp = await _httpClient.post(
        '/api/v1/app/products',
        data: {
          'idempotencyKey': idempotencyKey,
          'name': name,
          'categoryId': categoryId,
          if (origin != null && origin.isNotEmpty) 'origin': origin,
          if (descriptionInput != null) 'description': descriptionInput,
          if (images != null && images.isNotEmpty) 'images': images,
          if (skus != null && skus.isNotEmpty) 'skus': skus,
          if (status != null) 'status': status,
          if (hashtags != null) 'hashtags': hashtags,
        },
        options: Options(validateStatus: (s) => true),
      );
      return _parseResponse(resp);
    } catch (_) {
      return {'code': -1, 'message': '網路連線失敗，請稍後再試'};
    }
  }

  /// PUT /api/v1/app/products/{id}
  Future<Map<String, dynamic>> _updateProductV2(
    String productId, {
    String? idempotencyKey,
    String? name,
    String? categoryId,
    String? origin,
    String? description,
    List<Map<String, dynamic>>? descriptionBlocks,
    List<Map<String, dynamic>>? images,
    List<Map<String, dynamic>>? skus,
    String? status,
    List<String>? hashtags,
    bool updateHashtags = false,
  }) async {
    Map<String, dynamic>? descriptionInput;
    if (descriptionBlocks != null && descriptionBlocks.isNotEmpty) {
      descriptionInput = {'version': '1.0', 'blocks': descriptionBlocks};
    } else if (description != null && description.isNotEmpty) {
      descriptionInput = {
        'version': '1.0',
        'blocks': [
          {'type': 'text', 'content': description},
        ],
      };
    }

    try {
      final resp = await _httpClient.put(
        '/api/v1/app/products/$productId',
        data: {
          if (idempotencyKey != null) 'idempotencyKey': idempotencyKey,
          if (name != null) 'name': name,
          if (categoryId != null) 'categoryId': categoryId,
          if (origin != null) 'origin': origin,
          if (descriptionInput != null) 'description': descriptionInput,
          if (images != null) 'images': images,
          if (skus != null) 'skus': skus,
          if (status != null) 'status': status,
          if (hashtags != null) 'hashtags': hashtags,
          // updateHashtags 用 explicit boolean，false 也要傳，避免後端解讀為「不變更」
          // 但 spec 預設 false=不變更，所以只在 true 時帶（true=全量取代）
          if (updateHashtags) 'updateHashtags': true,
        },
        options: Options(validateStatus: (s) => true),
      );
      return _parseResponse(resp);
    } catch (_) {
      return {'code': -1, 'message': '網路連線失敗，請稍後再試'};
    }
  }

  Map<String, dynamic> _parseResponse(Response<dynamic> resp) {
    final statusCode = resp.statusCode ?? -1;
    if (statusCode >= 200 && statusCode < 300) {
      final data = resp.data;
      if (data == null) return {'code': 0, 'message': 'OK'};
      if (data is Map<String, dynamic>) return data;
      if (data is Map) return Map<String, dynamic>.from(data);
      return {'code': 0, 'data': data};
    }
    final data = resp.data;
    if (data is Map<String, dynamic>) {
      return {
        'code': data['code'] ?? statusCode,
        'message': data['message'] ?? 'HTTP $statusCode',
        'httpStatus': statusCode,
        ...data,
      };
    }
    return {'code': -1, 'message': 'HTTP $statusCode'};
  }

  final List<ProductUploadTask> _tasks = [];
  final _taskStreamController =
      StreamController<List<ProductUploadTask>>.broadcast();

  /// 監聽任務狀態變化
  Stream<List<ProductUploadTask>> get taskStream =>
      _taskStreamController.stream;

  /// 取得所有任務
  List<ProductUploadTask> get tasks => List.unmodifiable(_tasks);

  /// 取得進行中的任務數量
  int get pendingCount => _tasks
      .where(
        (t) =>
            t.status == UploadStatus.uploading ||
            t.status == UploadStatus.pending,
      )
      .length;

  /// 檢查指定商品名稱是否正在上傳中
  bool isUploading(String productName) {
    return _tasks.any(
      (t) =>
          t.productName == productName &&
          (t.status == UploadStatus.uploading ||
              t.status == UploadStatus.pending),
    );
  }

  /// 取得指定商品名稱的上傳進度 (0.0 - 1.0)，如果不在上傳中則返回 null
  double? getUploadProgress(String productName) {
    final task = _tasks
        .where(
          (t) =>
              t.productName == productName &&
              (t.status == UploadStatus.uploading ||
                  t.status == UploadStatus.pending),
        )
        .firstOrNull;
    return task?.progress;
  }

  /// 提交上傳任務，返回最終 task（含 success/failed 結果）。
  ///
  /// caller 可選擇 `await` 等候結果再決定 UI 動作（顯示錯誤 / pop 頁面），
  /// 也可選擇 fire-and-forget；taskStream 仍會即時推送狀態變化給 page_shop_tab
  /// 等背景監聽者。
  Future<ProductUploadTask> submitUpload(
    ProductUploadData data, {
    AnalyticsEvents? analytics,
  }) async {
    final taskId = DateTime.now().millisecondsSinceEpoch.toString();
    final task = ProductUploadTask(
      id: taskId,
      productName: data.productName,
      createdAt: DateTime.now(),
      status: UploadStatus.uploading,
      progress: 0.0,
    );

    _tasks.insert(0, task);
    _notifyListeners();

    await _executeUpload(task, data, analytics);
    return task;
  }

  /// 執行上傳（後台）
  Future<void> _executeUpload(
    ProductUploadTask task,
    ProductUploadData data,
    AnalyticsEvents? analytics,
  ) async {
    try {
      if (data.isEditMode) {
        await _executeEditMode(task, data, analytics);
      } else {
        await _executeCreateMode(task, data, analytics);
      }

      task.status = UploadStatus.success;
      task.progress = 1.0;
      debugPrint('[UploadService] Task ${task.id} completed successfully');
    } catch (e) {
      final safeError = redactSensitiveText(e.toString());
      task.status = UploadStatus.failed;
      task.errorMessage = safeError;
      debugPrint('[UploadService] Task ${task.id} failed: $safeError');
    }

    _notifyListeners();
  }

  /// 新增模式 — 使用新版 Product Service API
  ///
  /// 流程：
  /// 1. 並行上傳圖片 → 取得 staged tokens
  /// 2. 單次 POST /api/v1/merchant/products 建立商品
  ///    （後端自動 finalize 圖片、建立規格、設定認證）
  Future<void> _executeCreateMode(
    ProductUploadTask task,
    ProductUploadData data,
    AnalyticsEvents? analytics,
  ) async {
    // 進度分配：圖片上傳 50% + API 呼叫 50%
    final hasImages = data.images.isNotEmpty;

    // ── Step 1: 並行上傳圖片 → staged tokens ──
    List<Map<String, dynamic>> imageInputs = [];

    if (hasImages) {
      debugPrint(
        '[Upload] Uploading ${data.images.length} images via session flow...',
      );
      final ts = DateTime.now().millisecondsSinceEpoch;

      final files = data.images
          .asMap()
          .entries
          .map((e) => (bytes: e.value, filename: 'product_${ts}_${e.key}.jpg'))
          .toList();

      final results = await _mediaRepository.uploadBatch(
        files: files,
        compress: true,
      );

      debugPrint(
        '[Upload] uploadBatch returned ${results.length} results (of ${data.images.length} files)',
      );

      // 收集成功上傳的結果
      for (int i = 0; i < results.length; i++) {
        final r = results[i];
        debugPrint(
          '[Upload] result[$i]: hasStagedToken='
          '${r.stagedToken?.isNotEmpty == true}, hasObjectKey='
          '${r.objectKey.isNotEmpty}, hasUrl=${r.url.isNotEmpty}',
        );
        if (r.stagedToken != null && r.stagedToken!.isNotEmpty) {
          imageInputs.add({'stagedToken': r.stagedToken});
        }
      }

      debugPrint(
        '[Upload] ${imageInputs.length}/${data.images.length} images with valid stagedTokens',
      );
      if (imageInputs.isEmpty && data.images.isNotEmpty) {
        debugPrint(
          '[Upload] WARNING: No stagedTokens obtained! Images will NOT be saved.',
        );
      }
      task.progress = 0.5;
      _notifyListeners();
    }

    // ── Step 2: 建立 SKU 列表 ──
    final skus = data.specs.map((s) => s.toSkuJson()).toList();

    // ── Step 3: 決定分類 ID ──
    // 優先使用 catalogId（L3 分類），否則嘗試 l2Id、l1Id
    final categoryId = data.catalogId ?? data.l2Id ?? data.l1Id ?? 0;
    if (categoryId == 0) {
      throw Exception('缺少商品分類 ID');
    }

    // ── Step 4: 組裝並呼叫新 API ──
    final idempotencyKey = _uuid.v4();
    debugPrint(
      '[Upload] Creating product via new API (idempotency=$idempotencyKey)',
    );

    task.progress = 0.6;
    _notifyListeners();

    // 從 description（rich text）抽 hashtag；空 list 也送，讓後端知道是空 (對齊 issue #759)
    final hashtags = data.descriptionBlocks != null
        ? HashtagExtractor.extractFromBlocks(data.descriptionBlocks)
        : HashtagExtractor.extractFromText(data.description ?? '');
    debugPrint('[Upload] extracted ${hashtags.length} hashtags: $hashtags');

    final result = await _createProductV2(
      idempotencyKey: idempotencyKey,
      name: data.productName,
      categoryId: categoryId.toString(),
      origin: data.origin,
      description: data.description,
      descriptionBlocks: data.descriptionBlocks,
      images: imageInputs,
      skus: skus,
      status: '4002', // PENDING_REVIEW — 提交審核
      hashtags: hashtags,
    );

    task.progress = 0.9;
    _notifyListeners();

    debugPrint(
      '[Upload] createProductV2 response code=${result['code']}, images sent=${imageInputs.length}',
    );
    final responseData = result['data'] as Map<String, dynamic>?;
    if (responseData != null) {
      debugPrint(
        '[Upload] Response contains gallery=${responseData['gallery'] != null}, '
        'images=${responseData['images'] != null}',
      );
    }

    if (result['code'] != 0) {
      final msg = result['message'] ?? '商品建立失敗';
      throw Exception(msg);
    }

    final productId = result['data']?['id']?.toString() ?? '';
    debugPrint('[Upload] Product created successfully: $productId');
    if (analytics != null && productId.isNotEmpty) {
      unawaited(
        analytics.createProduct(productId, categoryId: categoryId.toString()),
      );
    }

    task.progress = 1.0;
    _notifyListeners();
  }

  /// 編輯模式 — 使用新版 Product Service API（PUT /api/v1/merchant/products/{pid}）
  Future<void> _executeEditMode(
    ProductUploadTask task,
    ProductUploadData data,
    AnalyticsEvents? analytics,
  ) async {
    // 從多個來源解析 productId
    final productId =
        data.productId ??
        data.existingProduct?['id']?.toString() ??
        data.skuId?.toString();
    if (productId == null || productId.isEmpty) {
      throw Exception('缺少商品 ID，無法更新');
    }

    // 1. 上傳新圖片
    List<Map<String, dynamic>> imageInputs = [];
    if (data.images.isNotEmpty) {
      final ts = DateTime.now().millisecondsSinceEpoch;
      final files = data.images
          .asMap()
          .entries
          .map((e) => (bytes: e.value, filename: 'product_${ts}_${e.key}.jpg'))
          .toList();

      final results = await _mediaRepository.uploadBatch(
        files: files,
        compress: true,
      );

      for (final r in results) {
        if (r.stagedToken != null && r.stagedToken!.isNotEmpty) {
          imageInputs.add({'stagedToken': r.stagedToken});
        }
      }
      task.progress = 0.4;
      _notifyListeners();
    }

    // 2. 組裝更新請求
    // 候選分類的 id 來自 category_candidate 表，與 category 表 id 空間衝突，
    // 直接作為 category_id 傳送會導致後端映射到錯誤的正式分類。
    // 候選分類未審核前，保留原商品的 category_id 不變。
    int? categoryId;
    if (data.isCandidateCategory) {
      final ep = data.existingProduct;
      final origId =
          ep?['category_id'] ?? ep?['categoryId'] ?? ep?['catalogId'];
      categoryId = origId is int
          ? origId
          : int.tryParse(origId?.toString() ?? '');
      debugPrint(
        '[Upload] isCandidateCategory=true, preserving original category_id=$categoryId',
      );
    } else {
      categoryId = data.catalogId ?? data.l2Id ?? data.l1Id;
    }
    final skus = data.specs.map((s) => s.toSkuJson()).toList();

    final idempotencyKey = _uuid.v4();

    task.progress = 0.6;
    _notifyListeners();

    // 編輯模式：同樣從 description 抽 hashtag；updateHashtags=true 讓後端全量取代既有
    final hashtags = data.descriptionBlocks != null
        ? HashtagExtractor.extractFromBlocks(data.descriptionBlocks)
        : HashtagExtractor.extractFromText(data.description ?? '');
    debugPrint(
      '[Upload] extracted ${hashtags.length} hashtags for update: $hashtags',
    );

    final result = await _updateProductV2(
      productId,
      idempotencyKey: idempotencyKey,
      name: data.productName,
      categoryId: categoryId?.toString(),
      origin: data.origin,
      description: data.description,
      descriptionBlocks: data.descriptionBlocks,
      images: imageInputs.isNotEmpty ? imageInputs : null,
      skus: skus,
      hashtags: hashtags,
      updateHashtags: true,
    );

    if (result['code'] != 0) {
      // PayUni #758: code=3019 表示商品在 ACTIVE / PENDING_REVIEW，需先下架；保留後端 message
      final rawMsg = result['message'];
      final code = result['code'];
      // 業務碼往上帶：呼叫端要靠它分辨「版本衝突」與其他失敗，兩者的下一步不同。
      task.errorCode = code is int
          ? code
          : int.tryParse(code?.toString() ?? '');
      final fallback = code == 3019 ? '商品需先下架後再編輯' : '商品更新失敗';
      throw Exception(rawMsg ?? fallback);
    }

    debugPrint('[Upload] Product updated successfully: $productId');
    if (analytics != null) {
      unawaited(analytics.updateProduct(productId));
    }

    task.progress = 1.0;
    _notifyListeners();
  }

  void _notifyListeners() {
    _taskStreamController.add(List.unmodifiable(_tasks));
  }

  /// 清除已完成的任務
  void clearCompletedTasks() {
    _tasks.removeWhere(
      (t) =>
          t.status == UploadStatus.success || t.status == UploadStatus.failed,
    );
    _notifyListeners();
  }

  void dispose() {
    _taskStreamController.close();
  }
}

/// Authenticated ProductUploadService entry point.
///
/// UI code that can start an upload must read this provider instead of using
/// the singleton factory directly. Passive task-stream consumers may continue
/// observing the singleton's in-memory queue.
final productUploadServiceProvider = Provider<ProductUploadService>((ref) {
  final service = ProductUploadService();
  service.useHttpClient(ref.watch(dioProvider));
  return service;
});
