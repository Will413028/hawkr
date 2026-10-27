/// 整合層：`AiAssistantRepository.streamChat` 透過 dio 的
/// `ResponseType.stream` 拿到真正的 AG-UI SSE bytes 後，必須吐出正確對應
/// 的 `AiSseEvent` 序列——這是 `decodeAgUiSse` 真的接進 `_startStream`（而不
/// 只是單元測試裡自己呼叫）的證據。
///
/// 依既有 repo 慣例（見 `test/core/networking/dio_auth_interceptor_skip_test.dart`）：
/// 用自訂 `HttpClientAdapter` 攔截 Dio，不需 mockito。這裡額外把 fixture
/// bytes 切成多段餵出，模擬真實串流分批到達。
library;

import 'dart:convert';
import 'dart:io';
import 'dart:typed_data';

import 'package:hawkr_seller/features/ai_assistant/data/ai_assistant_repository.dart';
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
  group('AiAssistantRepository.streamChat：AG-UI JSON type 分派', () {
    test('assistant_text_done：吐出 4 個 text + 1 個 done', () async {
      final dio = _dioWith(
        _SegmentedSseAdapter(_fixtureBytes('assistant_text_done.sse')),
      );
      addTearDown(dio.close);

      final events = await AiAssistantRepository(
        dio,
      ).streamChat(message: '哈囉').toList();

      expect(
        events.length,
        5,
        reason:
            '應恰好 4 個 text + 1 個 done，實際 ${events.length} 個：'
            '${events.map((e) => e.type).toList()}',
      );
      for (final e in events.take(4)) {
        expect(e.type, 'text');
        expect(
          e.terminal,
          isFalse,
          reason:
              'terminal 必須是 repository 從 AgUiEvent 原樣轉發的值，不能被 '
              '`AiSseEvent(type: event.name, data: event.data)` 這種漏帶欄位 '
              '的寫法悄悄變回預設值 false 而測不出來（本條看 text 該是 false）',
        );
      }
      expect(events.first.data.startsWith('哈'), isTrue);

      expect(events.last.type, 'done');
      expect(
        events.last.terminal,
        isTrue,
        reason:
            '真正的 RUN_FINISHED 事件必須是 terminal，若 repository 沒有轉發 '
            'event.terminal（永遠預設 false），controller 新的終止判斷永遠等 '
            '不到這個訊號——sessionId 存不下、多輪失憶的原形就是這樣來的',
      );
      final doneData = jsonDecode(events.last.data) as Map<String, dynamic>;
      expect(doneData['sessionId'], '11c3bef0-492b-4ff1-87a1-a67e70c3825d');
    });

    test('assistant_run_error：吐出 1 個 error 並關閉串流', () async {
      final dio = _dioWith(
        _SegmentedSseAdapter(_fixtureBytes('assistant_run_error.sse')),
      );
      addTearDown(dio.close);

      final events = await AiAssistantRepository(
        dio,
      ).streamChat(message: '哈囉').toList();

      expect(events.length, 1, reason: '應恰好 1 個 error 且串流已關閉');
      expect(events.single.type, 'error');
      expect(
        events.single.terminal,
        isTrue,
        reason:
            'RUN_ERROR 必須是 terminal，若 repository 沒有轉發 event.terminal '
            '（永遠預設 false），controller 新的終止判斷永遠等不到這個訊號，'
            '錯誤訊息永遠不會顯示',
      );
      final data = jsonDecode(events.single.data) as Map<String, dynamic>;
      expect(data['message'], 'message is required');
      expect(data['code'], '3');
    });

    test('toolCallName 剛好叫 done：streamChat 吐出的 AiSseEvent 序列沒有被提前截斷', () async {
      // 手寫（非 fixture）：TOOL_CALL_START 的 toolCallName 就叫 "done"，
      // 對應的 TOOL_CALL_RESULT 因此也會產生 name=="done" 的 AgUiEvent，
      // 但它不是 RUN_FINISHED，terminal 該是 false——這是這一輪要修的
      // 缺陷：舊寫法用 event.name=='done' 判斷終止，會在這裡誤判提前關閉
      // controller。
      const sseText =
          'data: {"type":"TOOL_CALL_START","toolCallId":"tc-1","toolCallName":"done"}\n'
          '\n'
          'data: {"type":"TOOL_CALL_RESULT","toolCallId":"tc-1","content":"{\\"ok\\":true}"}\n'
          '\n'
          'data: {"type":"TEXT_MESSAGE_CONTENT","delta":"之後還有內容"}\n'
          '\n'
          'data: {"type":"RUN_FINISHED","result":{"sessionId":"s1"}}\n'
          '\n';

      final dio = _dioWith(_SegmentedSseAdapter(utf8.encode(sseText)));
      addTearDown(dio.close);

      final events = await AiAssistantRepository(
        dio,
      ).streamChat(message: '哈囉').toList();

      expect(
        events.length,
        3,
        reason:
            '應該拿到：toolCallName=done 的事件、後面的 text、最後 RUN_FINISHED '
            '的 done；若在 toolCallName=done 那裡被誤判提前終止就只會有 1 個，'
            '實際 ${events.length} 個：${events.map((e) => e.type).toList()}',
      );

      expect(events[0].type, 'done');
      expect(
        events[0].terminal,
        isFalse,
        reason:
            'toolCallName=done 不是真正的 RUN_FINISHED，terminal 必須是 '
            'false——若 repository 沒轉發 event.terminal，這條斷言測不出漏轉發，'
            '因為預設值剛好也是 false，所以必須靠下面「真正的 done」那條 '
            'isTrue 斷言來夾擊',
      );
      expect(
        events[1].type,
        'text',
        reason: '證明 TOOL_CALL_RESULT（toolCallName=done）之後的 frame 沒有被截斷',
      );
      expect(events[2].type, 'done');
      expect(
        events[2].terminal,
        isTrue,
        reason:
            '只有真正的 RUN_FINISHED 才該是 terminal；若 repository 沒有轉發 '
            'event.terminal（永遠預設 false），這裡會轉紅',
      );
      final doneData = jsonDecode(events[2].data) as Map<String, dynamic>;
      expect(doneData['sessionId'], 's1');
    });
  });
}
