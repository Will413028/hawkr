import 'package:flutter_test/flutter_test.dart';
import 'package:hawkr_app_update/app_version_utils.dart';
import 'package:package_info_plus/package_info_plus.dart';

void _mock(String version, String build) {
  PackageInfo.setMockInitialValues(
    appName: 'hawkr',
    packageName: 'app.hawkr',
    version: version,
    buildNumber: build,
    buildSignature: '',
  );
}

void main() {
  TestWidgetsFlutterBinding.ensureInitialized();

  test('fetchAppVersion 回顯示字串：版本加版次，版次去掉前綴 0', () async {
    _mock('1.0.3', '007');
    expect(await fetchAppVersion(), '1.0.3(7)');
  });

  test('fetchAppVersion 沒有版次時只回 semver', () async {
    _mock('1.0.3', '');
    expect(await fetchAppVersion(), '1.0.3');
  });

  test('fetchAppVersion 版次不是數字時照原樣保留', () async {
    _mock('1.0.3', 'abc');
    expect(await fetchAppVersion(), '1.0.3(abc)');
  });

  test('fetchAppVersion 版本為空時回空字串', () async {
    _mock('', '7');
    expect(await fetchAppVersion(), '');
  });

  test('fetchRawAppVersion 回 bare semver，不含版次', () async {
    _mock('1.0.3', '7');
    expect(await fetchRawAppVersion(), '1.0.3');
  });

  test('fetchRawAppVersion 去掉前後空白', () async {
    _mock(' 1.0.3 ', '7');
    expect(await fetchRawAppVersion(), '1.0.3');
  });
}
