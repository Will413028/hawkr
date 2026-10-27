import 'dart:io';

import 'package:flutter_test/flutter_test.dart';

void main() {
  final appDelegate = File('ios/Runner/AppDelegate.swift').readAsStringSync();
  final pbxproj = File(
    'ios/Runner.xcodeproj/project.pbxproj',
  ).readAsStringSync();

  group('iOS Firebase wiring when GoogleService-Info.plist is absent', () {
    test(
      'AppDelegate configures Firebase only for complete parsed options',
      () {
        expect(
          appDelegate,
          contains('FirebaseOptions(contentsOfFile: plistPath)'),
        );
        expect(appDelegate, contains('!options.googleAppID.isEmpty'));
        expect(appDelegate, contains('!(options.apiKey ?? "").isEmpty'));
        expect(
          RegExp(r'FirebaseApp\.configure\(\)').allMatches(appDelegate).length,
          1,
        );
      },
    );

    test('the build never writes a placeholder plist', () {
      expect(pbxproj, isNot(contains('<dict/>')));
      expect(pbxproj, contains('略過嵌入'));
    });

    test('the plist is copied into the bundle, not a Resources member', () {
      expect(pbxproj, isNot(contains('GoogleService-Info.plist in Resources')));
      expect(
        pbxproj,
        contains(
          r'DEST=\"${BUILT_PRODUCTS_DIR}/${UNLOCALIZED_RESOURCES_FOLDER_PATH}/GoogleService-Info.plist\"',
        ),
      );
    });
  });
}
