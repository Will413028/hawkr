import 'package:dio/dio.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:hawkr_buyer/config/flavor.dart';
import 'package:hawkr_buyer/features/chat/data/chat_repository.dart';

class _CapturingAdapter implements HttpClientAdapter {
  final seen = <Map<String, dynamic>>[];

  @override
  Future<ResponseBody> fetch(
    RequestOptions options,
    Stream<List<int>>? requestStream,
    Future<void>? cancelFuture,
  ) async {
    seen.add(Map.of(options.headers));
    return ResponseBody.fromString(
      '[]',
      200,
      headers: {
        Headers.contentTypeHeader: [Headers.jsonContentType],
      },
    );
  }

  @override
  void close({bool force = false}) {}
}

void main() {
  TestWidgetsFlutterBinding.ensureInitialized();
  setUpAll(() {
    if (!F.isInitialized) F.init(Flavor.dev);
  });

  Future<(ChatRepository, _CapturingAdapter)> make(String? token) async {
    final adapter = _CapturingAdapter();
    final repo = ChatRepository.withDio(
      sessionToken: () async => token,
      accountId: () => 'acc-1',
      dio: Dio(BaseOptions(validateStatus: (_) => true))
        ..httpClientAdapter = adapter,
    );
    addTearDown(repo.dispose);
    return (repo, adapter);
  }

  test('聊天 HTTP 請求帶 session token 的 Bearer', () async {
    final (repo, adapter) = await make('live');
    await repo.getConversations();

    expect(adapter.seen.single['Authorization'], 'Bearer live');
  });

  test('訪客沒有 token：不送出請求', () async {
    final (repo, adapter) = await make(null);

    await expectLater(repo.getConversations(), throwsException);
    expect(adapter.seen, isEmpty);
  });
}
