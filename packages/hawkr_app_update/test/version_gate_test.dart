import 'dart:async';
import 'dart:io';

import 'package:hawkr_api/gen/hawkr/content/v1alpha1/content.connect.client.dart';
import 'package:hawkr_api/gen/hawkr/content/v1alpha1/content.connect.spec.dart';
import 'package:hawkr_api/gen/hawkr/content/v1alpha1/content.pb.dart';
import 'package:hawkr_api/testing.dart';
import 'package:hawkr_api/transport.dart' show Code, ConnectException;
import 'package:hawkr_app_update/hawkr_app_update.dart';
import 'package:test/test.dart';

/// 假的 CheckAppVersion：記下每次請求，回應由 [respond] 決定（丟出例外就是失敗）。
class _Backend {
  final requests = <CheckAppVersionRequest>[];
  FutureOr<CheckAppVersionResponse> Function(FakeHandlerContext) respond =
      (_) => CheckAppVersionResponse(
        updateRequired: true,
        minimumVersion: '2.0.0',
        storeUrl: 'itms-apps://apps.apple.com/app/id1',
        noticeTitle: '請更新',
        noticeBody: '此版本已停止支援',
      );

  ContentPublicServiceClient client() {
    final transport = FakeTransportBuilder().unary(
      ContentPublicService.checkAppVersion,
      (req, ctx) {
        requests.add(req);
        return respond(ctx);
      },
    ).build();
    return ContentPublicServiceClient(transport);
  }
}

void main() {
  late _Backend backend;
  late DateTime now;
  late List<String> logs;

  VersionGate gate({bool iOS = true, String version = '1.0.0'}) => VersionGate(
    client: backend.client(),
    appType: AppType.APP_TYPE_SELLER,
    isIOS: () => iOS,
    clientVersion: () => version,
    log: logs.add,
    now: () => now,
  );

  setUp(() {
    backend = _Backend();
    now = DateTime.utc(2026, 9, 27);
    logs = [];
  });

  test('冷啟動送出 App 的類型與版本，結果是後端的回應', () async {
    final g = gate();
    final res = await g.checkOnStartup();
    expect(backend.requests.single.appType, AppType.APP_TYPE_SELLER);
    expect(backend.requests.single.clientVersion, '1.0.0');
    expect(res.updateRequired, isTrue);
    expect(res.minimumVersion, '2.0.0');
    expect(res.storeUrl, 'itms-apps://apps.apple.com/app/id1');
    expect(res.noticeTitle, '請更新');
    expect(res.noticeBody, '此版本已停止支援');
    expect(res.isFrozen, isTrue);
    expect(g.result, same(res));
  });

  test('不是 iOS、或讀不到自己的版本時不檢查，一律放行', () async {
    expect((await gate(iOS: false).checkOnStartup()).updateRequired, isFalse);
    expect((await gate(version: '').checkOnStartup()).updateRequired, isFalse);
    expect(backend.requests, isEmpty);
    expect(logs, hasLength(1));
  });

  // 設計書 54 §7.6 ②：任何失敗都放行。後端故障時若判成「需要更新」，所有使用者會被鎖在沒辦法自救的畫面。
  for (final (name, error) in <(String, Object)>[
    ('伺服器錯誤', ConnectException(Code.internal, 'boom')),
    ('參數錯誤', ConnectException(Code.invalidArgument, 'bad version')),
    ('連線失敗', const SocketException('down')),
  ]) {
    test('$name → 放行', () async {
      backend.respond = (_) => throw error;
      expect((await gate().checkOnStartup()).updateRequired, isFalse);
      expect(logs, hasLength(1));
    });
  }

  test('逾時 → 約 3 秒放行，不卡住啟動', () async {
    // fake transport 不會因為 signal 中止；handler 自己等 signal，模擬真的 transport。
    backend.respond = (ctx) async => throw await ctx.signal.future;
    final stopwatch = Stopwatch()..start();
    expect((await gate().checkOnStartup()).updateRequired, isFalse);
    expect(stopwatch.elapsed, lessThan(const Duration(seconds: 5)));
  });

  test('失敗時保留上一次的結果；失敗不更新節流的錨點，回到前景立刻重試', () async {
    final g = gate();
    await g.checkOnStartup();
    now = now.add(versionGateRecheckInterval);
    backend.respond = (_) => throw Exception('服務暫時無法使用');
    expect((await g.checkOnResume()).updateRequired, isTrue);

    backend.respond = (_) => CheckAppVersionResponse(updateRequired: false);
    now = now.add(const Duration(seconds: 1));
    expect((await g.checkOnResume()).updateRequired, isFalse);
    expect(backend.requests, hasLength(3));
  });

  test('回到前景：距上次成功未滿節流間隔就不檢查；冷啟動不看節流', () async {
    final g = gate();
    await g.checkOnStartup();
    now = now.add(versionGateRecheckInterval - const Duration(seconds: 1));
    await g.checkOnResume();
    expect(backend.requests, hasLength(1));

    await g.checkOnStartup();
    expect(backend.requests, hasLength(2));

    now = now.add(versionGateRecheckInterval);
    await g.checkOnResume();
    expect(backend.requests, hasLength(3));
  });
}
