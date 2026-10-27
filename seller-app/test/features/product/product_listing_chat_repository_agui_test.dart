/// 整合層：`ProductListingChatRepository.stream` 透過 dio 的
/// `ResponseType.stream` 拿到真正的 AG-UI SSE bytes 後，必須吐出正確對應的
/// `SseEvent` 序列——這是 `decodeAgUiSse` 真的接進 `stream()`（而不只是單元
/// 測試裡自己呼叫）的證據。
///
/// 這支測試釘的正是現行 bug 形狀：舊 parser 把所有 frame 都標成
/// `type: 'message'`（因為它找不到 `event:` 行），`ProductListingChatController`
/// 的 switch 沒有 `message` 這個 case，`draft_patch` 因此永遠不會 merge 進
/// 草稿。修好後應該拿到 `type: 'draft_patch'`。
///
/// 依既有 repo 慣例（見 `test/core/networking/dio_auth_interceptor_skip_test.dart`）：
/// 用自訂 `HttpClientAdapter` 攔截 Dio，不需 mockito。這裡額外把 fixture
/// bytes 切成多段餵出，模擬真實串流分批到達。
library;

import 'dart:convert';
import 'dart:io';
import 'dart:typed_data';

import 'package:hawkr_seller/features/product/data/product_listing_chat_repository.dart';
import 'package:dio/dio.dart';
import 'package:flutter_test/flutter_test.dart';

List<int> _fixtureBytes(String name) =>
    File('test/fixtures/agui/$name').readAsBytesSync();

/// 把整份 fixture bytes 切成固定大小的段，模擬真實 TCP 串流分批到達
/// （刻意選一個跟任何 JSON 邊界都對不齊的大小）。
const _segmentSize = 37;

class _SegmentedSseAdapter implements HttpClientAdapter {
  _SegmentedSseAdapter(this.bytes);

  final List<int> bytes;

  @override
  Future<ResponseBody> fetch(
    RequestOptions options,
    Stream<Uint8List>? requestStream,
    Future<void>? cancelFuture,
  ) async {
    final segments = <Uint8List>[];
    for (var i = 0; i < bytes.length; i += _segmentSize) {
      final end = (i + _segmentSize < bytes.length)
          ? i + _segmentSize
          : bytes.length;
      segments.add(Uint8List.fromList(bytes.sublist(i, end)));
    }
    return ResponseBody(
      Stream<Uint8List>.fromIterable(segments),
      200,
      headers: {
        Headers.contentTypeHeader: ['text/event-stream'],
      },
    );
  }

  @override
  void close({bool force = false}) {}
}

Dio _dioWith(HttpClientAdapter adapter) =>
    Dio(BaseOptions(baseUrl: 'https://test.local'))
      ..httpClientAdapter = adapter;

void main() {
  group('ProductListingChatRepository.stream：AG-UI JSON type 分派', () {
    test(
      'listing_draft_patch：draft_patch 事件的 type 是 draft_patch，不是 message',
      () async {
        final dio = _dioWith(
          _SegmentedSseAdapter(_fixtureBytes('listing_draft_patch.sse')),
        );
        addTearDown(dio.close);

        final events = await ProductListingChatRepository(
          dio,
        ).stream(message: '幫我上架大甲芋頭').toList();

        final draftPatches = events.where((e) => e.type == 'draft_patch');
        expect(
          draftPatches.length,
          1,
          reason:
              '應恰好 1 個 type==draft_patch，實際事件 type：'
              '${events.map((e) => e.type).toList()}',
        );
        expect(
          events.where((e) => e.type == 'message'),
          isEmpty,
          reason:
              '這正是現行 bug 的形狀：舊 parser 找不到 event: 行時會把所有 frame '
              '標成 message，導致 controller 的 switch 沒有對應 case',
        );

        expect(
          draftPatches.single.terminal,
          isFalse,
          reason:
              'CUSTOM draft_patch 不是控制流訊號，terminal 必須是 false——若 '
              'repository 沒有轉發 event.terminal（永遠預設 false），這條斷言 '
              '測不出漏轉發，必須靠下面「真正的 done」那條 isTrue 斷言來夾擊',
        );

        final patch =
            jsonDecode(draftPatches.single.data) as Map<String, dynamic>;
        expect(patch['skus'], isA<List>());
        final skus = patch['skus'] as List;
        expect(
          (skus[0] as Map<String, dynamic>)['skuName'],
          '5 公斤裝一箱',
          reason: '巢狀（陣列包物件）不該被扁平化或吃掉',
        );
        expect(patch['tags'], contains('友善栽培'));

        expect(events.where((e) => e.type == 'text').length, 4);
        final doneEvents = events.where((e) => e.type == 'done').toList();
        expect(doneEvents.length, 1);
        expect(
          doneEvents.single.terminal,
          isTrue,
          reason:
              '真正的 RUN_FINISHED 事件必須是 terminal，若 repository 沒有轉發 '
              'event.terminal（永遠預設 false），controller 新的終止判斷永遠 '
              '等不到這個訊號——sessionId 存不下、多輪失憶的原形就是這樣來的',
        );
      },
    );

    test('resp.data 為 null（例如被攔截器清空）：吐出 type=error/data=empty stream '
        '且 terminal=true', () async {
      final dio = Dio(BaseOptions(baseUrl: 'https://test.local'))
        ..httpClientAdapter = _SegmentedSseAdapter(
          _fixtureBytes('assistant_text_done.sse'),
        )
        ..interceptors.add(
          InterceptorsWrapper(
            onResponse: (response, handler) {
              // 模擬 resp.data?.stream == null 這個分支——這是唯一能
              // 透過真實 dio 流程觸發它的方式：ResponseType.stream 底下
              // Dio 只在 statusOk 時才會設 response.data，且
              // ResponseBody.stream 本身不可為 null，所以要製造出
              // `resp.data == null` 只能靠攔截器主動清空。
              response.data = null;
              handler.next(response);
            },
          ),
        );
      addTearDown(dio.close);

      final events = await ProductListingChatRepository(
        dio,
      ).stream(message: '幫我上架高麗菜').toList();

      expect(events.length, 1, reason: '應恰好 1 個 error 事件');
      expect(events.single.type, 'error');
      expect(events.single.data, 'empty stream');
      expect(
        events.single.terminal,
        isTrue,
        reason:
            '這次呼叫已經沒有任何後續事件會來，語意上等同 RUN_ERROR；若 '
            '這裡不是 terminal:true，controller 新的終止判斷（見 '
            'product_listing_chat_controller.dart 的 case \'error\'：'
            'if (event.terminal)）會把這個錯誤完全忽略，state.error 永遠 '
            '不會被設定',
      );
    });

    // 覆蓋深度不對稱：AI 助手側（ai_assistant_repository_agui_test.dart）
    // 有一條「toolCallName 剛好叫 done」的撞名 repository 層測試，上架助手
    // 這支 repository 卻沒有對應的撞名測試——撞名保護只分散在 decoder 層
    // 與 controller 層（後者用 fake repository，跨不過
    // `ProductListingChatRepository.stream()` 這條接線）。這條補上：手寫
    // 一個 CUSTOM 事件 name 剛好叫 "error"，斷言它 terminal==false、後面
    // 的 text 沒被截斷、真正的 RUN_FINISHED 才是 terminal。
    test('CUSTOM 的 name 剛好叫 error：type==error 但 terminal==false，'
        '後面的 text 沒有被提前截斷', () async {
      const sseText =
          'data: {"type":"CUSTOM","name":"error","value":'
          '{"note":"not a real error"}}\n'
          '\n'
          'data: {"type":"TEXT_MESSAGE_CONTENT","delta":"還有更多"}\n'
          '\n'
          'data: {"type":"RUN_FINISHED","result":{"sessionId":"s1"}}\n'
          '\n';

      final dio = _dioWith(_SegmentedSseAdapter(utf8.encode(sseText)));
      addTearDown(dio.close);

      final events = await ProductListingChatRepository(
        dio,
      ).stream(message: '幫我上架高麗菜').toList();

      expect(
        events.length,
        3,
        reason:
            '應該拿到：CUSTOM name=error 事件、後面的 text、最後 '
            'RUN_FINISHED 的 done；若被誤判提前終止就只會有 1 個，實際 '
            '${events.length} 個：${events.map((e) => e.type).toList()}',
      );

      expect(events[0].type, 'error');
      expect(
        events[0].terminal,
        isFalse,
        reason: 'CUSTOM 即使 name 叫 error，也不是控制流訊號',
      );

      expect(
        events[1].type,
        'text',
        reason: '證明 CUSTOM（name=error）之後的 frame 確實還被解析到，沒有提前結束串流',
      );

      expect(events[2].type, 'done');
      expect(
        events[2].terminal,
        isTrue,
        reason: '只有 RUN_FINISHED 產生的這個 done 才該是 terminal',
      );
    });
  });
}
