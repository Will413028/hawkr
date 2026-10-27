import 'package:hawkr_buyer/features/order/data/receipt_photo_local_service.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:shared_preferences/shared_preferences.dart';

void main() {
  TestWidgetsFlutterBinding.ensureInitialized();

  test(
    'clearAll removes every receipt photo entry but leaves other keys',
    () async {
      SharedPreferences.setMockInitialValues({
        'receipt_photo_HK-1': 'https://local.test/a.jpg',
        'receipt_photo_HK-2': 'https://local.test/b.jpg',
        'fcm_processed_ids': ['msg-1'],
      });
      final service = SharedPreferencesReceiptPhotoLocalService();

      await service.clearAll();

      final preferences = await SharedPreferences.getInstance();
      expect(await service.read('HK-1'), isNull);
      expect(await service.read('HK-2'), isNull);
      expect(preferences.getStringList('fcm_processed_ids'), ['msg-1']);
    },
  );
}
