import 'dart:io';

import 'package:flutter_test/flutter_test.dart';

void main() {
  test('protected issue-97 call sites do not construct ad-hoc clients', () {
    final protectedSources = [
      File(
        'lib/features/product/data/product_upload_service.dart',
      ).readAsStringSync(),
      File(
        'lib/features/notification/data/push/device_registrar.dart',
      ).readAsStringSync(),
    ];

    for (final source in protectedSources) {
      expect(source, isNot(contains('Dio(')));
      expect(source, isNot(contains("headers: {'Authorization'")));
      expect(source, isNot(contains('updateToken(')));
    }

    final certification = [
      'lib/features/seller/presentation/seller_certification_upload_screen.dart',
      'lib/features/seller/presentation/kyc_wizard_steps.dart',
      'lib/features/seller/presentation/kyc_wizard_widgets.dart',
    ].map((path) => File(path).readAsStringSync()).join('\n');
    expect(certification, isNot(contains('HttpClient().getUrl')));
    expect(certification, isNot(contains('dioProvider')));
    expect(certification, contains('protectedMediaControllerProvider'));

    final protectedMediaService = File(
      'lib/features/media/data/protected_media_remote_service.dart',
    ).readAsStringSync();
    expect(protectedMediaService, isNot(contains('Dio(')));
    expect(protectedMediaService, contains('ref.watch(dioProvider)'));

    final session = File(
      'lib/core/session/session_provider.dart',
    ).readAsStringSync();
    expect(session, isNot(contains('ProductUploadService.updateToken')));
    expect(
      session,
      isNot(contains('PushNotificationManager.instance.updateToken')),
    );
  });
}
