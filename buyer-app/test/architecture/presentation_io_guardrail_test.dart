import 'dart:io';

import 'package:flutter_test/flutter_test.dart';

import '../../tool/architecture_check.dart';

void main() {
  test(
    'presentation I/O violations cannot grow beyond the owned ratchet',
    () async {
      final result = await checkPresentationIo(
        repositoryRoot: Directory.current,
      );

      expect(result.errors, isEmpty, reason: result.errors.join('\n'));
      expect(result.checkedFiles, greaterThan(0));
    },
  );

  test('Connect 的 client 與 transport 是 I/O，proto 的訊息型別不是', () {
    const rule = PresentationIoRule.connect;
    expect(
      rule.matches("import '../../../core/networking/connect_provider.dart';"),
      isTrue,
    );
    expect(rule.matches("import 'package:hawkr_api/transport.dart';"), isTrue);
    expect(
      rule.matches(
        "import 'package:hawkr_api/gen/hawkr/content/v1alpha1/"
        "content.connect.client.dart';",
      ),
      isTrue,
    );
    expect(
      rule.matches(
        "import 'package:hawkr_api/gen/hawkr/content/v1alpha1/"
        "content.pb.dart' as api;",
      ),
      isFalse,
    );
    expect(
      rule.matches("import 'package:hawkr_api/images.dart' as api;"),
      isFalse,
    );
  });
}
