import 'package:flutter_test/flutter_test.dart';
import 'package:hawkr_api/gen/hawkr/assistant/v1alpha1/assistant.connect.client.dart';
import 'package:hawkr_api/gen/hawkr/assistant/v1alpha1/assistant.connect.spec.dart';
import 'package:hawkr_api/gen/hawkr/assistant/v1alpha1/assistant.pb.dart' as pb;
import 'package:hawkr_api/gen/hawkr/assistant/v1alpha1/assistant.pbenum.dart';
import 'package:hawkr_api/testing.dart';
import 'package:hawkr_api/transport.dart';
import 'package:hawkr_buyer/features/ai_assistant/data/ai_assistant_repository.dart';

import '../../support/default_error_text.dart';

/// 伺服器端的一輪：每 10 秒一個文字事件，共 150 秒，最後是 finished；每條串流最長 100 秒就正常結束。
/// [watches] 記下每次訂閱帶的 cursor。
class _Server {
  _Server({this.duplicateOnResume = false, this.failures = const []});

  static const eventSeconds = 10;
  static const lifetimeSeconds = 100;
  static const turnSeconds = 150;

  final bool duplicateOnResume;
  final List<ConnectException> failures;
  final watches = <String>[];
  final sends = <pb.SendAssistantMessageRequest>[];

  int _failed = 0;

  pb.WatchAssistantTurnResponse _text(int second) =>
      pb.WatchAssistantTurnResponse(
        cursor: 'c$second',
        text: pb.AssistantTextDelta(text: '[$second]'),
      );

  Stream<pb.WatchAssistantTurnResponse> _watch(
    pb.WatchAssistantTurnRequest r,
  ) async* {
    watches.add(r.cursor);
    if (_failed < failures.length) throw failures[_failed++];
    final from = r.cursor.isEmpty ? 0 : int.parse(r.cursor.substring(1));
    final start = r.cursor.isEmpty ? 0 : from;
    // 接續的連線從 cursor 之後開始；本條串流的 100 秒從這一刻重新計。
    if (duplicateOnResume && r.cursor.isNotEmpty) yield _text(from);
    var elapsed = 0;
    for (var t = start + eventSeconds; t <= turnSeconds; t += eventSeconds) {
      elapsed += eventSeconds;
      if (elapsed > lifetimeSeconds) return;
      yield _text(t);
    }
    yield pb.WatchAssistantTurnResponse(
      cursor: 'end',
      finished: pb.AssistantFinished(sessionId: 'sess-1'),
    );
  }

  AiAssistantRepository repository({List<Duration>? delays}) =>
      AiAssistantRepository(
        AssistantAppServiceClient(
          FakeTransportBuilder()
              .unary(AssistantAppService.sendAssistantMessage, (req, _) {
                sends.add(req);
                return pb.SendAssistantMessageResponse(
                  turnId: 'turn-1',
                  sessionId: 'sess-1',
                );
              })
              .server(
                AssistantAppService.watchAssistantTurn,
                (req, _) => _watch(req),
              )
              .build(),
        ),
        defaults: zhDefaultErrorText,
        delay: (d) async => delays?.add(d),
        mint: () => 'req-1',
      );
}

Future<List<AiTurnEvent>> _collect(AiAssistantRepository repo) => repo
    .streamChat(topic: AssistantTopic.ASSISTANT_TOPIC_CHAT, message: 'hi')
    .toList();

String _texts(List<AiTurnEvent> events) =>
    events.whereType<AiTextDelta>().map((e) => e.text).join();

void main() {
  final expected = [for (var t = 10; t <= 150; t += 10) '[$t]'].join();

  test('150 秒的一輪在 100 秒被截斷後以 cursor 接續，不漏不重', () async {
    final server = _Server();
    final events = await _collect(server.repository());

    expect(_texts(events), expected);
    expect(events.last, isA<AiTurnFinished>());
    expect((events.last as AiTurnFinished).sessionId, 'sess-1');
    expect(server.watches, ['', 'c100']);
  });

  test('接續時伺服器重送已處理的事件：只處理一次', () async {
    final server = _Server(duplicateOnResume: true);
    final events = await _collect(server.repository());

    expect(_texts(events), expected);
  });

  test('送出帶 client_request_id、topic 與 session', () async {
    final server = _Server();
    await _collect(server.repository());

    expect(server.sends.single.clientRequestId, 'req-1');
    expect(server.sends.single.topic, AssistantTopic.ASSISTANT_TOPIC_CHAT);
    expect(server.sends.single.message, 'hi');
  });

  test('暫時性斷線退避後以同一個 cursor 接續', () async {
    final server = _Server(
      failures: [ConnectException(Code.unavailable, 'blip')],
    );
    final delays = <Duration>[];
    final events = await _collect(server.repository(delays: delays));

    expect(_texts(events), expected);
    expect(server.watches, ['', '', 'c100']);
    expect(delays, [const Duration(seconds: 1)]);
  });

  test('not_found（輪次不存在）不重試，直接丟出', () async {
    final server = _Server(failures: [ConnectException(Code.notFound, 'gone')]);

    await expectLater(
      _collect(server.repository()),
      throwsA(
        isA<ConnectException>().having((e) => e.code, 'code', Code.notFound),
      ),
    );
    expect(server.watches, ['']);
  });

  test('連續失敗達上限後丟出', () async {
    final server = _Server(
      failures: [
        for (var i = 0; i < 3; i++) ConnectException(Code.unavailable, 'down'),
      ],
    );

    await expectLater(
      _collect(server.repository()),
      throwsA(isA<ConnectException>()),
    );
    expect(server.watches.length, 3);
  });
}
