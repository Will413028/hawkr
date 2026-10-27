import 'package:dio/dio.dart';
import 'package:riverpod_annotation/riverpod_annotation.dart';

import '../../../core/networking/agui_sse.dart';
import '../../../core/networking/dio_provider.dart';
import 'product_listing_model.dart';

part 'product_listing_chat_repository.g.dart';

/// SSE client：連線到 `/api/v1/app/sellers/me/assistant/product-listing/chat`
/// 並 yield 解析後的 [SseEvent]（type: text / draft_patch / tool_result / done / error）。
class ProductListingChatRepository {
  ProductListingChatRepository(this._dio);

  final Dio _dio;

  static const _endpoint =
      '/api/v1/app/sellers/me/assistant/product-listing/chat';

  Stream<SseEvent> stream({
    required String message,
    String? sessionId,
    String? imageUrl,
  }) async* {
    final body = <String, dynamic>{
      'message': message,
      // 2026-08-16: 契約遷移至 camelCase。Gateway 與 ai-service 皆已更新。
      if (sessionId != null && sessionId.isNotEmpty) 'sessionId': sessionId,
      if (imageUrl != null && imageUrl.isNotEmpty) 'imageUrl': imageUrl,
    };

    final resp = await _dio.post<ResponseBody>(
      _endpoint,
      data: body,
      options: Options(
        responseType: ResponseType.stream,
        headers: {'Accept': 'text/event-stream'},
        // SSE streams can run long; disable receive timeout
        receiveTimeout: Duration.zero,
      ),
    );

    final stream = resp.data?.stream;
    if (stream == null) {
      // 沒有 body 可讀＝這次呼叫已經沒有任何後續事件會來，語意上等同於
      // RUN_ERROR：terminal 必須是 true，否則 controller 新的終止判斷
      // （見 product_listing_chat_controller.dart）永遠看不到這個錯誤。
      yield SseEvent(type: 'error', data: 'empty stream', terminal: true);
      return;
    }

    await for (final event in decodeAgUiSse(stream)) {
      yield SseEvent(
        type: event.name,
        data: event.data,
        terminal: event.terminal,
      );
      // 判斷是否終止一律看 event.terminal，不看 event.name——name 對
      // TOOL_CALL_RESULT/CUSTOM 是後端可控的自由字串，字串比對 'done'/
      // 'error' 會被剛好同名的工具或自訂事件誤判成提前終止。
      if (event.terminal) return;
    }
  }
}

@riverpod
ProductListingChatRepository productListingChatRepository(Ref ref) {
  return ProductListingChatRepository(ref.read(dioProvider));
}
