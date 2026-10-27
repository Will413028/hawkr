/// 整合層：`AiAssistantRepository.streamChat` 透過 dio 的
/// `ResponseType.stream` 拿到真正的 AG-UI SSE bytes 後，必須吐出正確對應
/// 的 `AiSseEvent` 序列——這是 `decodeAgUiSse` 真的接進 `_startStream`（而不
/// 只是單元測試裡自己呼叫）的證據。
///
/// 依既有 repo 慣例（見 `test/features/social/social_repository_submit_report_test.dart`）：
/// 用自訂 `HttpClientAdapter` 攔截 Dio，不需 mockito。這裡額外把 fixture
/// bytes 切成多段餵出，模擬真實串流分批到達。
library;

import 'dart:convert';
import 'dart:io';
import 'dart:typed_data';

import 'package:hawkr_buyer/features/ai_assistant/data/ai_assistant_repository.dart';
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
              'text 事件不該是 terminal（若 repository 沒有正確傳遞 event.terminal，'
              '這裡會被錯改成 false 也測不出來，但下面 done 那條會抓到）',
        );
      }
      expect(events.first.data.startsWith('哈'), isTrue);

      expect(events.last.type, 'done');
      expect(
        events.last.terminal,
        isTrue,
        reason:
            'repository 必須把 decodeAgUiSse 產生的 AgUiEvent.terminal 傳遞到 '
            'AiSseEvent.terminal，不能寫死 false（否則 controller 永遠不會存 '
            'sessionId、RUN_ERROR 的錯誤文案也不會顯示）',
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
            'repository 必須把 event.terminal 如實傳遞，不能寫死 false（否則 '
            'controller 判斷不到終止，錯誤文案永遠不會顯示）',
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
        reason: '撞名的 tool 事件不是真正的 RUN_FINISHED，terminal 必須是 false',
      );
      expect(
        events[1].type,
        'text',
        reason: '證明 TOOL_CALL_RESULT（toolCallName=done）之後的 frame 沒有被截斷',
      );
      expect(events[1].terminal, isFalse);
      expect(events[2].type, 'done');
      expect(
        events[2].terminal,
        isTrue,
        reason: '只有真正的 RUN_FINISHED 才該是 terminal，repository 不能把它寫死 false',
      );
      final doneData = jsonDecode(events[2].data) as Map<String, dynamic>;
      expect(doneData['sessionId'], 's1');
    });
  });
}
