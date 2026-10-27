import 'package:hawkr_buyer/features/notification/data/push/background_handler.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:shared_preferences/shared_preferences.dart';

void main() {
  TestWidgetsFlutterBinding.ensureInitialized();

  test(
    'clearFcmProcessedIds removes the dedup key but leaves other keys',
    () async {
      SharedPreferences.setMockInitialValues({
        fcmProcessedIdsKey: ['msg-1', 'msg-2'],
        'receipt_photo_HK-1': 'https://local.test/a.jpg',
      });

      await clearFcmProcessedIds();

      final preferences = await SharedPreferences.getInstance();
      expect(preferences.getStringList(fcmProcessedIdsKey), isNull);
      expect(
        preferences.getString('receipt_photo_HK-1'),
        'https://local.test/a.jpg',
      );
    },
  );
}
