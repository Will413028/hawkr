import 'dart:convert';

/// 語意事件（App 內部詞彙），name 例：text / done / error / tool_result /
/// draft_patch / step_detail。由 [AgUiSseDecoder] 依 AG-UI `type` 分派產生，
/// 供 repository 層直接轉為既有的 `AiSseEvent`。
///
/// [terminal] 只在事件由 `RUN_FINISHED` / `RUN_ERROR` 產生時為 true。加這
/// 個欄位是因為 [name] 對 `TOOL_CALL_RESULT`／`CUSTOM` 而言是**後端可控的
/// 自由字串**（`TOOL_CALL_RESULT` 的 name 來自 `TOOL_CALL_START.toolCallName`；
/// `CUSTOM` 的 name 來自 `CUSTOM.name`）——目前 assistant lane 硬編
/// `tool_result`／`draft_patch`／`step_detail`，但同一份 encoder 的其他
/// lane 已經在用像 `get_my_orders` 這樣的工具名（見 gateway
/// `agui_golden_test.go`）。呼叫端不可用 `name == 'done' || name ==
/// 'error'` 字串比對來判斷串流是否該終止：哪天有個工具或 CUSTOM 事件剛好
/// 叫 `done` 或 `error`，字串比對會把它誤判成控制流訊號，串流被靜默提前
/// 截斷——這正是這次改動要根除的失敗形狀（舊 parser 讀不到事件），不該
/// 由控制流邏輯自己重新引入一個同語意陷阱。判斷是否終止一律看
/// [terminal]，不要看 [name]。
class AgUiEvent {
  const AgUiEvent({
    required this.name,
    required this.data,
    this.terminal = false,
  });
  final String name;
  final String data;
  final bool terminal;
}

/// 同步、增量的 AG-UI SSE 解碼器。可單獨測試，不碰 IO。
///
/// gateway 已把 AI SSE framing 從 `event: <name>\ndata: <json>` 換成純
/// `data: <json>`，事件類型改成靠 JSON `type` 欄位分派——這個 class 就是
/// 對應表的唯一實作，欄位取自 gateway `ag_ui_events.go` / `ai_assistant_stream.go`。
///
/// 一個 decoder 實例對應一次串流呼叫；`toolCallId → toolCallName` 對照表
/// 是 instance field，不得做成 static/global，否則會跨請求互相污染。
class AgUiSseDecoder {
  final Map<String, String> _toolCallNames = <String, String>{};

  String _buffer = '';
  final List<String> _pendingDataLines = <String>[];
  bool _terminated = false;

  /// 收到 `RUN_FINISHED` / `RUN_ERROR` 後為 true；之後再餵任何東西都不再
  /// 產生事件。
  bool get terminated => _terminated;

  /// 餵入新的一段文字（已經是 utf8 decode 後的字串），回傳這段文字觸發的
  /// 語意事件（可能是空 list）。
  List<AgUiEvent> addChunk(String text) {
    if (_terminated) return const <AgUiEvent>[];

    _buffer += text;
    final events = <AgUiEvent>[];

    while (true) {
      final idx = _buffer.indexOf('\n');
      if (idx == -1) break;
      final line = _buffer.substring(0, idx);
      _buffer = _buffer.substring(idx + 1);

      _processLine(line, events);
      if (_terminated) break;
    }

    return events;
  }

  /// 處理串流結束時沒有結尾空行的殘餘 block（例如最後一個 `data:` 行後面
  /// 沒有再送一個空行就斷線)。
  List<AgUiEvent> flush() {
    if (_terminated) return const <AgUiEvent>[];

    final events = <AgUiEvent>[];
    if (_buffer.isNotEmpty) {
      final remaining = _buffer;
      _buffer = '';
      _processLine(remaining, events);
    }
    _finalizeBlock(events);
    return events;
  }

  void _processLine(String rawLine, List<AgUiEvent> events) {
    final line = rawLine.trimRight(); // 掉尾端 \r，容忍 \r\n。

    if (line.isEmpty) {
      // 空行 = block 結束。
      _finalizeBlock(events);
      return;
    }

    if (!line.startsWith('data:')) {
      // 非 data: 開頭的行一律跳過：keepalive comment `: ping`、殘留的
      // event: / id: / retry: 都走這條。
      return;
    }

    var value = line.substring(5);
    // `data:` 後面至多剝掉一個空白（SSE 規範），不是無條件 substring(6)。
    if (value.startsWith(' ')) {
      value = value.substring(1);
    }
    _pendingDataLines.add(value);
  }

  void _finalizeBlock(List<AgUiEvent> events) {
    if (_pendingDataLines.isEmpty) {
      // block 內沒有任何 data: 行（例如純 `: ping`）→ 整個丟棄。
      return;
    }

    final dataRaw = _pendingDataLines.join('\n');
    _pendingDataLines.clear();

    dynamic decoded;
    try {
      decoded = jsonDecode(dataRaw);
    } catch (_) {
      return; // 不是合法 JSON → 丟棄整個 frame。
    }
    if (decoded is! Map) return; // decode 出來不是 Map → 丟棄。

    final type = decoded['type'];
    if (type is! String) return; // type 不存在或不是 String → 丟棄。

    switch (type) {
      case 'TEXT_MESSAGE_CONTENT':
        final delta = decoded['delta'];
        if (delta is! String) return; // 非 String → 丟棄整個 frame。
        events.add(AgUiEvent(name: 'text', data: delta));
        break;

      case 'RUN_FINISHED':
        final result = decoded['result'];
        events.add(
          AgUiEvent(
            name: 'done',
            data: jsonEncode(result ?? <String, dynamic>{}),
            terminal: true,
          ),
        );
        _terminated = true;
        break;

      case 'RUN_ERROR':
        final errorPayload = <String, dynamic>{};
        final message = decoded['message'];
        final code = decoded['code'];
        if (message != null) errorPayload['message'] = message;
        if (code != null) errorPayload['code'] = code;
        events.add(
          AgUiEvent(
            name: 'error',
            data: jsonEncode(errorPayload),
            terminal: true,
          ),
        );
        _terminated = true;
        break;

      case 'TOOL_CALL_START':
        final toolCallId = decoded['toolCallId'];
        final toolCallName = decoded['toolCallName'];
        if (toolCallId is String && toolCallName is String) {
          _toolCallNames[toolCallId] = toolCallName;
        }
        break; // 不產生事件，只記下對照表。

      case 'TOOL_CALL_RESULT':
        final toolCallId = decoded['toolCallId'];
        final name =
            (toolCallId is String ? _toolCallNames[toolCallId] : null) ??
            'tool_result';
        final content = decoded['content'];
        events.add(
          AgUiEvent(name: name, data: content is String ? content : ''),
        );
        break;

      case 'CUSTOM':
        final name = decoded['name'];
        if (name is! String) return; // 非 String → 丟棄整個 frame。
        events.add(AgUiEvent(name: name, data: jsonEncode(decoded['value'])));
        break;

      default:
        // RUN_STARTED / TEXT_MESSAGE_START / TEXT_MESSAGE_END /
        // TOOL_CALL_END / STEP_STARTED / STEP_FINISHED / STATE_SNAPSHOT
        // 等其餘型別一律忽略。
        break;
    }
  }
}

/// 薄包裝：bytes → utf8.decoder → decoder。terminated 就提早 return，
/// 結束時 flush() 處理沒有結尾空行的殘餘 block。
///
/// 必須用 `bytes.cast<List<int>>().transform(utf8.decoder)`，不可用逐
/// chunk 的 `utf8.decode(chunk, allowMalformed: true)`——後者會把跨 chunk
/// 邊界的多位元組中文字切成替換字元。
Stream<AgUiEvent> decodeAgUiSse(Stream<List<int>> bytes) async* {
  final decoder = AgUiSseDecoder();

  await for (final text in bytes.cast<List<int>>().transform(utf8.decoder)) {
    if (decoder.terminated) return;
    for (final event in decoder.addChunk(text)) {
      yield event;
    }
    if (decoder.terminated) return;
  }

  for (final event in decoder.flush()) {
    yield event;
  }
}
