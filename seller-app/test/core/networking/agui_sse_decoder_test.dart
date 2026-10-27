/// gateway 已把 AI SSE framing 從 `event: <name>\ndata: <json>` 換成純
/// `data: <json>`，事件類型改成靠 JSON `type` 欄位分派。這支測試釘的是
/// [AgUiSseDecoder] 對 AG-UI wire 契約的解碼行為。
///
/// fixture 全部是對 dev 實打擷取的 AG-UI SSE bytes（見
/// `test/fixtures/agui/`；`listing_draft_patch.sse` 補的是上架助手的巢狀
/// `CUSTOM.value`（`draft_patch`）、`procurement_keepalive_steps.sse` 補的
/// 是採購串流的巢狀 `CUSTOM.value`（`step_detail`）與真實 keepalive
/// `: ping`），只有「手寫邊界案例」那個 group 例外——那幾條測的是 framing
/// 邊界（沒有空白的 `data:`、非 JSON、非 Map、缺 `type`、純空白行當 block
/// 分隔），不需要真實回應形狀。
library;

import 'dart:convert';
import 'dart:io';

import 'package:hawkr_seller/core/networking/agui_sse.dart';
import 'package:flutter_test/flutter_test.dart';

List<int> _fixtureBytes(String name) =>
    File('test/fixtures/agui/$name').readAsBytesSync();

List<AgUiEvent> _decodeAll(List<int> bytes) {
  final decoder = AgUiSseDecoder();
  final events = <AgUiEvent>[
    ...decoder.addChunk(utf8.decode(bytes)),
    ...decoder.flush(),
  ];
  return events;
}

void main() {
  group('AgUiSseDecoder：真實 fixture', () {
    test('assistant_text_done：4 個 text + 1 個 done，中間型別被忽略', () {
      final events = _decodeAll(_fixtureBytes('assistant_text_done.sse'));

      // 斷言事件總數：RUN_STARTED/TEXT_MESSAGE_START/TEXT_MESSAGE_END
      // 各出現一次，若沒被忽略，總數就不會是 5。
      expect(
        events.length,
        5,
        reason:
            '應恰好 4 個 text + 1 個 done，實際 ${events.length} 個：'
            '${events.map((e) => e.name).toList()}',
      );

      final textEvents = events.take(4).toList();
      for (final e in textEvents) {
        expect(e.name, 'text');
      }
      expect(
        textEvents.first.data.startsWith('哈'),
        isTrue,
        reason: '第一個 text 的 delta 應以「哈」開頭，實際：${textEvents.first.data}',
      );

      final doneEvent = events.last;
      expect(doneEvent.name, 'done');
      final doneData = jsonDecode(doneEvent.data) as Map<String, dynamic>;
      expect(doneData['sessionId'], '11c3bef0-492b-4ff1-87a1-a67e70c3825d');
    });

    test('assistant_run_error：恰好 1 個 error，message/code 對應 RUN_ERROR', () {
      final events = _decodeAll(_fixtureBytes('assistant_run_error.sse'));

      expect(events.length, 1, reason: '應恰好 1 個 error');
      expect(events.single.name, 'error');
      final data = jsonDecode(events.single.data) as Map<String, dynamic>;
      expect(data['message'], 'message is required');
      expect(data['code'], '3');
    });

    test('assistant_tool_result：1 個 tool_result，content 是原字串沒被再包一層 JSON', () {
      final events = _decodeAll(_fixtureBytes('assistant_tool_result.sse'));

      final toolResults = events.where((e) => e.name == 'tool_result');
      expect(toolResults.length, 1, reason: '應恰好 1 個 tool_result');
      expect(
        toolResults.single.data,
        '{"type":"order_list","orders":[]}',
        reason: 'content 應是原字串，不能被再 jsonEncode 包一層',
      );
    });

    test('assistant_keepalive_ping：`: ping` 不產生事件，序列與無 ping 版本一致', () {
      final withPing = _decodeAll(
        _fixtureBytes('assistant_keepalive_ping.sse'),
      );

      // 手寫「拿掉兩個 `: ping` block」的等價版本，用來比對事件序列。
      const withoutPingText =
          'data: {"type":"RUN_STARTED","threadId":"","runId":"0c1ad480a185e9acf03baa8a03e30f65"}\n'
          '\n'
          'data: {"type":"TEXT_MESSAGE_START","messageId":"0c1ad480a185e9acf03baa8a03e30f65","role":"assistant"}\n'
          '\n'
          'data: {"type":"TEXT_MESSAGE_CONTENT","messageId":"0c1ad480a185e9acf03baa8a03e30f65","delta":"哈"}\n'
          '\n'
          'data: {"type":"RUN_FINISHED","threadId":"","runId":"0c1ad480a185e9acf03baa8a03e30f65","result":{"sessionId":"11c3bef0-492b-4ff1-87a1-a67e70c3825d"}}\n'
          '\n';
      final withoutPing = _decodeAll(utf8.encode(withoutPingText));

      expect(withPing.length, withoutPing.length, reason: 'ping 不應改變事件數量');
      for (var i = 0; i < withPing.length; i++) {
        expect(
          withPing[i].name,
          withoutPing[i].name,
          reason: 'index $i 的 name 應一致',
        );
        expect(
          withPing[i].data,
          withoutPing[i].data,
          reason: 'index $i 的 data 應一致',
        );
      }
      // 對照組本身也要有內容，避免「兩邊都是空 list」這種假陽性。
      expect(withPing, isNotEmpty);
      expect(
        withPing.map((e) => e.name),
        containsAll(<String>['text', 'done']),
      );
    });

    test('assistant_empty_run：只有 1 個 done，零 text（記錄退化行為）', () {
      final events = _decodeAll(_fixtureBytes('assistant_empty_run.sse'));

      expect(events.length, 1);
      expect(events.single.name, 'done');
      expect(events.where((e) => e.name == 'text'), isEmpty);
    });

    test('assistant_text_done：只有最後的 done 事件 terminal == true', () {
      final events = _decodeAll(_fixtureBytes('assistant_text_done.sse'));

      expect(events.last.name, 'done');
      expect(events.last.terminal, isTrue);
      for (final e in events.take(events.length - 1)) {
        expect(
          e.terminal,
          isFalse,
          reason: '${e.name} 不是 RUN_FINISHED/RUN_ERROR 產生的，terminal 該是 false',
        );
      }
    });

    test('assistant_run_error：error 事件 terminal == true', () {
      final events = _decodeAll(_fixtureBytes('assistant_run_error.sse'));

      expect(events.single.name, 'error');
      expect(events.single.terminal, isTrue);
    });

    test('listing_draft_patch：恰好 1 個 draft_patch，巢狀（陣列包物件）沒有被扁平化或吃掉', () {
      final events = _decodeAll(_fixtureBytes('listing_draft_patch.sse'));

      final draftPatches = events.where((e) => e.name == 'draft_patch');
      expect(draftPatches.length, 1, reason: '應恰好 1 個 draft_patch');

      final patch =
          jsonDecode(draftPatches.single.data) as Map<String, dynamic>;
      expect(patch['skus'], isA<List>());
      final skus = patch['skus'] as List;
      expect(
        (skus[0] as Map<String, dynamic>)['skuName'],
        '5 公斤裝一箱',
        reason: 'skus 是陣列包物件，不該被扁平化',
      );
      expect(patch['tags'], contains('友善栽培'), reason: 'tags 陣列不該被吃掉');
    });

    test('listing_draft_patch：同一份 fixture 也要拿到 4 個 text 與 1 個 done', () {
      final events = _decodeAll(_fixtureBytes('listing_draft_patch.sse'));

      expect(
        events.where((e) => e.name == 'text').length,
        4,
        reason:
            '實測 fixture 內恰好 4 個 TEXT_MESSAGE_CONTENT，實際事件：'
            '${events.map((e) => e.name).toList()}',
      );
      expect(events.where((e) => e.name == 'done').length, 1);
    });

    test(
      'procurement_keepalive_steps：9 個 step_detail 全部 terminal == false',
      () {
        final events = _decodeAll(
          _fixtureBytes('procurement_keepalive_steps.sse'),
        );

        final stepDetails = events.where((e) => e.name == 'step_detail');
        expect(stepDetails.length, 9);
        for (final e in stepDetails) {
          expect(
            e.terminal,
            isFalse,
            reason: 'CUSTOM 事件不該是 terminal，即使 name 剛好跟控制流同名也一樣',
          );
        }
      },
    );

    test(
      'procurement_keepalive_steps：CUSTOM step_detail 巢狀 value 完整送達且沒被扁平化',
      () {
        final events = _decodeAll(
          _fixtureBytes('procurement_keepalive_steps.sse'),
        );

        final stepDetails = events
            .where((e) => e.name == 'step_detail')
            .toList();
        expect(stepDetails.length, 9, reason: '應恰好 9 個 step_detail');

        final decodedSteps = stepDetails
            .map((e) => jsonDecode(e.data) as Map<String, dynamic>)
            .toList();

        // 任一筆都要是完整的巢狀 Map，欄位沒有被扁平化或漏接，seq 保持
        // JSON number 型別（不是被轉成字串）。
        final first = decodedSteps.first;
        expect(first.containsKey('turnId'), isTrue);
        expect(first.containsKey('seq'), isTrue);
        expect(first.containsKey('code'), isTrue);
        expect(first.containsKey('status'), isTrue);
        expect(first.containsKey('label'), isTrue);
        expect(first['seq'], isA<int>(), reason: 'seq 不應被轉成字串');

        // status == SUCCEEDED 且真的有 durationMs 的一筆：
        // EXTRACT_SLOTS seq=3，durationMs=11067。
        final succeededWithDuration = decodedSteps.singleWhere(
          (m) => m['code'] == 'EXTRACT_SLOTS' && m['status'] == 'SUCCEEDED',
        );
        expect(succeededWithDuration.containsKey('durationMs'), isTrue);
        expect(succeededWithDuration['durationMs'], 11067);

        // status == RUNNING 的一筆：durationMs 這個 key 不該存在
        // （gateway 的 omitempty 真實行為，不是欄位被清空成 null）。
        final running = decodedSteps.singleWhere(
          (m) => m['code'] == 'UNDERSTAND_INPUT' && m['status'] == 'RUNNING',
        );
        expect(
          running.containsKey('durationMs'),
          isFalse,
          reason: 'RUNNING 狀態不該帶 durationMs（omitempty）',
        );
      },
    );

    test(
      'procurement_keepalive_steps：真實 `: ping` 被靜默忽略，事件總數 = 9 step_detail + 1 text + 1 done',
      () {
        final events = _decodeAll(
          _fixtureBytes('procurement_keepalive_steps.sse'),
        );

        expect(
          events.length,
          11,
          reason:
              '應恰好 11 個事件（9 step_detail + 1 text + 1 done），'
              '實際 ${events.length} 個：${events.map((e) => e.name).toList()}',
        );
        expect(events.where((e) => e.name == 'step_detail').length, 9);
        expect(events.where((e) => e.name == 'text').length, 1);
        expect(events.where((e) => e.name == 'done').length, 1);
        expect(
          events.where((e) => e.name == 'error'),
          isEmpty,
          reason: 'keepalive ping 不該被誤判成錯誤',
        );
      },
    );

    test(
      'procurement_keepalive_steps：STEP_STARTED/STEP_FINISHED/STATE_SNAPSHOT 確實被忽略',
      () {
        final events = _decodeAll(
          _fixtureBytes('procurement_keepalive_steps.sse'),
        );

        const ignoredTypeNames = <String>{
          'STEP_STARTED',
          'STEP_FINISHED',
          'STATE_SNAPSHOT',
        };
        expect(
          events.where((e) => ignoredTypeNames.contains(e.name)),
          isEmpty,
          reason: '這三種 AG-UI type 沒有對應到任何 AgUiEvent.name，不該出現在輸出裡',
        );
      },
    );

    for (final fixtureName in <String>[
      'assistant_text_done.sse',
      'listing_draft_patch.sse',
      'procurement_keepalive_steps.sse',
    ]) {
      test('chunk 邊界地獄：逐 byte 餵與整份一次餵完全相同（$fixtureName）', () async {
        final bytes = _fixtureBytes(fixtureName);

        final wholeStream = Stream<List<int>>.fromIterable(<List<int>>[bytes]);
        final byteByByteStream = Stream<List<int>>.fromIterable(
          bytes.map((b) => <int>[b]),
        );

        final wholeEvents = await decodeAgUiSse(wholeStream).toList();
        final splitEvents = await decodeAgUiSse(byteByByteStream).toList();

        expect(
          splitEvents.length,
          wholeEvents.length,
          reason: '逐 byte 餵不應改變事件數量',
        );
        for (var i = 0; i < wholeEvents.length; i++) {
          expect(
            splitEvents[i].name,
            wholeEvents[i].name,
            reason: 'index $i 的 name 應一致',
          );
          expect(
            splitEvents[i].data,
            wholeEvents[i].data,
            reason:
                'index $i 的 data 應一致（若不同，代表多位元組中文在 chunk '
                '邊界被切壞，或 ping 夾在兩個 frame 中間時邊界判斷壞掉）',
          );
        }
      });
    }

    test('terminated 之後再餵任何東西都不再產生事件', () {
      final decoder = AgUiSseDecoder();
      final bytes = _fixtureBytes('assistant_text_done.sse');
      decoder.addChunk(utf8.decode(bytes));

      expect(decoder.terminated, isTrue);

      final more = decoder.addChunk(
        'data: {"type":"TEXT_MESSAGE_CONTENT","delta":"不該出現"}\n\n',
      );
      expect(more, isEmpty);
      expect(decoder.flush(), isEmpty);
    });
  });

  group('AgUiSseDecoder：手寫邊界案例', () {
    test('data: 後面沒有空白仍能正確解析（不是無條件 substring(6)）', () {
      final decoder = AgUiSseDecoder();
      final events = decoder.addChunk(
        'data:{"type":"TEXT_MESSAGE_CONTENT","delta":"x"}\n\n',
      );

      expect(events.length, 1);
      expect(events.single.name, 'text');
      expect(events.single.data, 'x', reason: '不能被誤剝成 "\'x\\"}"} 之類的殘缺字串');
    });

    test('data: not-json → 不產生事件', () {
      final decoder = AgUiSseDecoder();
      final events = decoder.addChunk('data: not-json\n\n');
      expect(events, isEmpty);
    });

    test('data: [] → decode 出來不是 Map，不產生事件', () {
      final decoder = AgUiSseDecoder();
      final events = decoder.addChunk('data: []\n\n');
      expect(events, isEmpty);
    });

    test('data: {"delta":"x"}（缺 type）→ 不產生事件', () {
      final decoder = AgUiSseDecoder();
      final events = decoder.addChunk('data: {"delta":"x"}\n\n');
      expect(events, isEmpty);
    });

    test('純空白（非空字串）的行視同空行，會結束 block（trimRight() 後為空字串）', () {
      final decoder = AgUiSseDecoder();
      final events = decoder.addChunk(
        'data: {"type":"CUSTOM","name":"a","value":1}\n'
        '   \n'
        'data: {"type":"CUSTOM","name":"b","value":2}\n'
        '\n',
      );

      expect(events.length, 2, reason: '兩個 frame 都要拿到，中間那行只有空白也該被當成 block 分隔');
      expect(events[0].name, 'a');
      expect(events[0].data, '1');
      expect(events[1].name, 'b');
      expect(events[1].data, '2');
    });

    // reviewer 在買家分支上把 `if (!line.startsWith('data:')) return;` 這個
    // 守衛整條拿掉，買家原本的 21 個 decoder 測試全數照樣通過——因為 fixture
    // 裡的 `: ping` 都自成一個 block，守衛沒了之後 `': ping'.substring(5)`
    // 產出的垃圾 'g' 剛好被下游 JSON guard 吃掉，看起來像沒事。這支測試把
    // comment/event:/id:/retry: 行跟真正的 data: 行放進**同一個 block**：
    // 少了守衛時，這些行會被一起 substring(5) 之後 join 成一串非法 JSON，
    // 連同真正的 data: 行都被丟掉。
    test('同一個 block 內混雜 comment / event: / id: / retry: 與 data: 行：'
        '非 data: 行必須被忽略，不能污染同一 block 內真正的 data: 行', () {
      final decoder = AgUiSseDecoder();
      final events = decoder.addChunk(
        ': ping\n'
        'event: legacy\n'
        'id: 42\n'
        'retry: 1000\n'
        'data: {"type":"TEXT_MESSAGE_CONTENT","messageId":"m","delta":"看得到我"}\n'
        '\n',
      );

      expect(
        events.length,
        1,
        reason:
            '守衛拿掉時，comment/event:/id:/retry: 行會跟 data: 行一起被 '
            'join 成非法 JSON，整個 block（含真正的 data:）都會被丟掉，'
            '實際 ${events.length} 個：${events.map((e) => e.name).toList()}',
      );
      expect(events.single.name, 'text');
      expect(events.single.data, '看得到我');
    });

    // assistant_tool_result.sse 裡 toolCallName 剛好就是 "tool_result"，跟
    // fallback 字串一模一樣，查表命中或沒命中兩種情況吐出的事件名都相同，
    // 那條 fixture 測試對查表壞掉／fallback 壞掉沒有判別力。這支測試補上
    // 「查不到對照表」的路徑：沒有前置 TOOL_CALL_START，斷言 fallback 到
    // 'tool_result'。
    test('TOOL_CALL_RESULT 沒有對應的 TOOL_CALL_START（查表 miss）→ '
        'fallback 成 tool_result', () {
      final decoder = AgUiSseDecoder();
      final events = decoder.addChunk(
        'data: {"type":"TOOL_CALL_RESULT","toolCallId":"unknown-id",'
        '"content":"{\\"ok\\":true}"}\n'
        '\n',
      );

      expect(events.length, 1);
      expect(
        events.single.name,
        'tool_result',
        reason: '查表 miss 時應 fallback 成 tool_result，不是留空或崩潰',
      );
      expect(
        events.single.data,
        '{"ok":true}',
        reason: 'content 應是原字串，不能被再包一層',
      );
    });

    // 多行 data: 是 SSE 規格明文規定的 framing（同一個 event 內連續多個
    // data: 行要用 '\n' 接起來），但 7 份真實 fixture 沒有一份用到它，這條
    // 路徑因此完全沒有回歸保護。
    //
    // 分割點刻意選在數字中間（"value":5 + 0}，而不是逗號/大括號這種結構
    // 邊界）：在結構邊界切開時，join('\n') 與 join('') 會得到語意相同的
    // 合法 JSON（已用獨立腳本對照 jsonDecode 輸出實測驗證過），測試對兩種
    // join 方式沒有鑑別力，等於空斷言。攔腰截斷數字則不同：正確的
    // join('\n') 會讓 "value":5 後面接著換行、再接 0}——"value":5 已是完整
    // 數字，後面只能接 , 或 }，不能直接接數字 0，是不合法 JSON，這個
    // frame 該被丟棄（events 為空）；若誤用 join('')，兩半會被焊成合法但
    // 錯誤的 "value":50，frame 不會被丟棄。
    test('同一個 block 內多行 data:，數字被攔腰截斷：正確 join 應讓這個 frame '
        '因不合法 JSON 被丟棄，而不是被 join(\'\') 誤焊成合法但錯誤的數字', () {
      final decoder = AgUiSseDecoder();
      final events = decoder.addChunk(
        'data: {"type":"CUSTOM","name":"n","value":5\n'
        'data: 0}\n'
        '\n',
      );

      expect(
        events,
        isEmpty,
        reason:
            '"value":5 後面接換行再接 0}，join(\'\\n\') 後是不合法 JSON '
            '（缺少 , 或 } 就直接出現數字 0），frame 該整個被丟棄；若程式 '
            '改用 join(\'\')，會焊成合法但錯誤的 "value":50，events 就不會 '
            '是空的，實際 ${events.length} 個',
      );
    });

    // 7 份真實 fixture 全部以 `\n\n` 結尾，且都在 RUN_FINISHED/RUN_ERROR
    // 就已經 `_terminated`，`flush()` 因此從來沒有機會真正做事——這條路徑
    // 完全沒有回歸保護。這條測試故意用 TEXT_MESSAGE_CONTENT（不是
    // RUN_FINISHED/RUN_ERROR），確保測的是 flush() 而不是提早 return。
    test('串流結束時最後一個 data: 沒有接空行就斷線：flush() 仍要把它解析出來', () async {
      final bytes = utf8.encode(
        'data: {"type":"TEXT_MESSAGE_CONTENT","delta":"斷線也要拿到"}\n',
      );
      final events = await decodeAgUiSse(
        Stream<List<int>>.fromIterable(<List<int>>[bytes]),
      ).toList();

      expect(
        events.length,
        1,
        reason: '沒有結尾空行不代表這個 frame 該被丟棄，flush() 該把它解析出來',
      );
      expect(events.single.name, 'text');
      expect(events.single.data, '斷線也要拿到');
    });
  });

  group('AgUiSseDecoder：terminal 旗標不能靠 name 字串比對推導', () {
    test('TOOL_CALL_RESULT 的 toolCallName 剛好叫 done：name==done 但 terminal==false，'
        '後面的 frame 沒有被提前截斷', () {
      final decoder = AgUiSseDecoder();
      final events = decoder.addChunk(
        'data: {"type":"TOOL_CALL_START","toolCallId":"tc-1","toolCallName":"done"}\n'
        '\n'
        'data: {"type":"TOOL_CALL_RESULT","toolCallId":"tc-1","content":"{\\"ok\\":true}"}\n'
        '\n'
        'data: {"type":"TEXT_MESSAGE_CONTENT","delta":"之後還有內容"}\n'
        '\n'
        'data: {"type":"RUN_FINISHED","result":{"sessionId":"s1"}}\n'
        '\n',
      );

      expect(
        events.length,
        3,
        reason:
            '應該拿到：toolCallName=done 的 tool 事件、後面的 text、最後的 '
            'RUN_FINISHED done，若被誤判提前終止就只會有 1 個',
      );

      expect(events[0].name, 'done');
      expect(
        events[0].terminal,
        isFalse,
        reason: 'TOOL_CALL_RESULT 即使 name 叫 done，也不是控制流訊號',
      );

      expect(
        events[1].name,
        'text',
        reason: '證明 TOOL_CALL_RESULT 之後的 frame 確實還被解析到，沒有提前結束串流',
      );

      expect(events[2].name, 'done');
      expect(
        events[2].terminal,
        isTrue,
        reason: '只有 RUN_FINISHED 產生的這個 done 才該是 terminal',
      );
    });

    test('CUSTOM 的 name 剛好叫 error：name==error 但 terminal==false，'
        '後面的 frame 沒有被提前截斷', () {
      final decoder = AgUiSseDecoder();
      final events = decoder.addChunk(
        'data: {"type":"CUSTOM","name":"error","value":{"note":"not a real error"}}\n'
        '\n'
        'data: {"type":"TEXT_MESSAGE_CONTENT","delta":"還有更多"}\n'
        '\n'
        'data: {"type":"RUN_FINISHED","result":{}}\n'
        '\n',
      );

      expect(
        events.length,
        3,
        reason:
            '應該拿到：CUSTOM name=error 事件、後面的 text、最後的 RUN_FINISHED '
            'done，若被誤判提前終止就只會有 1 個',
      );

      expect(events[0].name, 'error');
      expect(
        events[0].terminal,
        isFalse,
        reason: 'CUSTOM 即使 name 叫 error，也不是控制流訊號',
      );

      expect(
        events[1].name,
        'text',
        reason: '證明 CUSTOM 之後的 frame 確實還被解析到，沒有提前結束串流',
      );

      expect(events[2].name, 'done');
      expect(events[2].terminal, isTrue);
    });
  });
}
