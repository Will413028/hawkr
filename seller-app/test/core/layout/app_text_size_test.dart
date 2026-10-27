import 'package:hawkr_seller/core/layout/app_text_size.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:shared_preferences/shared_preferences.dart';

void main() {
  TestWidgetsFlutterBinding.ensureInitialized();

  test('storage values map to the three supported preferences', () {
    expect(AppTextSize.fromStorage('small'), AppTextSize.small);
    expect(AppTextSize.fromStorage('standard'), AppTextSize.standard);
    expect(AppTextSize.fromStorage('large'), AppTextSize.large);
    expect(AppTextSize.fromStorage(null), AppTextSize.standard);
    expect(AppTextSize.fromStorage('unexpected'), AppTextSize.standard);
  });

  test('restores the saved preference', () async {
    SharedPreferences.setMockInitialValues({
      AppTextSizeController.storageKey: AppTextSize.large.name,
    });
    final container = ProviderContainer();
    addTearDown(container.dispose);

    await container.read(appTextSizeProvider.notifier).restore();

    expect(container.read(appTextSizeProvider), AppTextSize.large);
  });

  test('invalid persisted data fails open to standard', () async {
    SharedPreferences.setMockInitialValues({
      AppTextSizeController.storageKey: 'extra-large',
    });
    final container = ProviderContainer();
    addTearDown(container.dispose);

    await container.read(appTextSizeProvider.notifier).restore();

    expect(container.read(appTextSizeProvider), AppTextSize.standard);
  });

  test('applies immediately and persists future launches', () async {
    SharedPreferences.setMockInitialValues({});
    final container = ProviderContainer();
    addTearDown(container.dispose);

    final save = container
        .read(appTextSizeProvider.notifier)
        .setTextSize(AppTextSize.small);

    expect(container.read(appTextSizeProvider), AppTextSize.small);
    expect(await save, isTrue);

    final preferences = await SharedPreferences.getInstance();
    expect(
      preferences.getString(AppTextSizeController.storageKey),
      AppTextSize.small.name,
    );
  });

  test('rapid changes persist the latest preference', () async {
    SharedPreferences.setMockInitialValues({});
    final container = ProviderContainer();
    addTearDown(container.dispose);
    final controller = container.read(appTextSizeProvider.notifier);

    final first = controller.setTextSize(AppTextSize.small);
    final second = controller.setTextSize(AppTextSize.large);
    await Future.wait([first, second]);

    expect(container.read(appTextSizeProvider), AppTextSize.large);
    final preferences = await SharedPreferences.getInstance();
    expect(
      preferences.getString(AppTextSizeController.storageKey),
      AppTextSize.large.name,
    );
  });
}
