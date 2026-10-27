import 'package:hawkr_buyer/shared/widgets/hk_placeholder.dart';
import 'package:hawkr_api/gen/hawkr/buyer/v1alpha1/buyer.pbenum.dart';
import 'package:hawkr_buyer/core/theme/app_colors.dart';
import 'package:hawkr_buyer/core/theme/app_typography.dart';
import 'package:hawkr_buyer/features/settings/presentation/widgets/profile_edit_widgets.dart';
import 'package:hawkr_buyer/shared/widgets/hk_icon.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

void main() {
  testWidgets(
    'profile text field edits inline and readonly state is explicit',
    (tester) async {
      final nameController = TextEditingController(text: 'Cora Chou');
      final emailController = TextEditingController(text: 'carl@example.com');
      addTearDown(nameController.dispose);
      addTearDown(emailController.dispose);

      await _pumpSurface(
        tester,
        width: 390,
        child: Form(
          child: Column(
            children: [
              ProfileTextField(
                label: '用戶名稱',
                controller: nameController,
                required: true,
              ),
              ProfileTextField(
                label: '登入信箱',
                controller: emailController,
                helperText: '登入信箱目前無法在此變更',
                readOnly: true,
              ),
            ],
          ),
        ),
      );

      await tester.enterText(
        find.byKey(const ValueKey('profile-field-用戶名稱')),
        '新的名稱',
      );
      await tester.pump();
      expect(nameController.text, '新的名稱');
      expect(find.text('登入信箱目前無法在此變更'), findsOneWidget);
      expect(find.textContaining('用戶名稱', findRichText: true), findsOneWidget);
      expect(
        tester
            .widget<InputDecorator>(
              find.descendant(
                of: find.byKey(const ValueKey('profile-field-用戶名稱')),
                matching: find.byType(InputDecorator),
              ),
            )
            .decoration
            .labelText,
        isNull,
      );
      expect(
        tester
            .widget<EditableText>(
              find.descendant(
                of: find.byKey(const ValueKey('profile-field-登入信箱')),
                matching: find.byType(EditableText),
              ),
            )
            .readOnly,
        isTrue,
      );
    },
  );

  testWidgets('selection field exposes one labeled 48dp action', (
    tester,
  ) async {
    var tapped = false;
    final semantics = tester.ensureSemantics();
    try {
      await _pumpSurface(
        tester,
        width: 390,
        child: ProfileSelectionField(
          label: '生日',
          value: '1990-01-01',
          placeholder: '選擇日期',
          onTap: () => tapped = true,
        ),
      );

      final action = find.bySemanticsLabel('生日，1990-01-01');
      expect(tester.getSize(action).height, greaterThanOrEqualTo(48));
      await tester.tap(action);
      await tester.pump();
      expect(tapped, isTrue);
    } finally {
      semantics.dispose();
    }
  });

  testWidgets('profile dropdown opens the shared dropdown and selects a city', (
    tester,
  ) async {
    var selected = '';
    final semantics = tester.ensureSemantics();
    try {
      await _pumpSurface(
        tester,
        width: 390,
        textScale: 1,
        child: ProfileDropdownField<String>(
          label: '縣市',
          value: null,
          placeholder: '選擇縣市',
          items: const ['臺北市', '新北市'],
          itemLabel: (city) => city,
          icon: HkIconName.location,
          showSearch: true,
          onSelected: (value) => selected = value,
        ),
      );

      await tester.tap(find.bySemanticsLabel('縣市，選擇縣市'));
      await tester.pump();
      expect(find.text('臺北市'), findsOneWidget);

      await tester.tap(find.text('臺北市'));
      await tester.pump();
      expect(selected, '臺北市');
    } finally {
      semantics.dispose();
    }
  });

  testWidgets(
    'profile dropdown without a value shows the placeholder style and marks the null entry selected',
    (tester) async {
      final semantics = tester.ensureSemantics();
      try {
        await _pumpSurface(
          tester,
          width: 390,
          textScale: 1,
          child: ProfileDropdownField<Gender?>(
            label: '性別',
            value: null,
            placeholder: '尚未設定',
            items: const [null, Gender.GENDER_MALE],
            itemLabel: (g) => g == null ? '尚未設定' : '男性',
            onSelected: (_) {},
          ),
        );

        final surface = find.byKey(const ValueKey('profile-dropdown-性別'));
        final label = tester.widget<Text>(
          find.descendant(of: surface, matching: find.text('尚未設定')),
        );
        expect(label.style?.color, AppColors.textHint);

        await tester.tap(find.bySemanticsLabel('性別，尚未設定'));
        await tester.pump();
        bool selectedAt(int i) => tester
            .getSemantics(find.byKey(ValueKey('app-dropdown-item-$i')))
            .getSemanticsData()
            .flagsCollection
            .isSelected
            .toBoolOrNull()!;
        expect(selectedAt(0), isTrue);
        expect(selectedAt(1), isFalse);
      } finally {
        semantics.dispose();
      }
    },
  );

  testWidgets('profile dropdown shows a value that is not in the items', (
    tester,
  ) async {
    await _pumpSurface(
      tester,
      width: 390,
      textScale: 1,
      child: ProfileDropdownField<String>(
        label: '縣市',
        value: '某縣',
        placeholder: '選擇縣市',
        items: const ['臺北市', '新北市'],
        itemLabel: (city) => city,
        onSelected: (_) {},
      ),
    );

    final surface = find.byKey(const ValueKey('profile-dropdown-縣市'));
    final label = tester.widget<Text>(
      find.descendant(of: surface, matching: find.text('某縣')),
    );
    expect(label.style?.color, AppColors.textEmphasis);
    expect(find.text('選擇縣市'), findsNothing);
  });

  testWidgets('text and selection field icons share one trailing column', (
    tester,
  ) async {
    final jobController = TextEditingController();
    addTearDown(jobController.dispose);

    await _pumpSurface(
      tester,
      width: 390,
      textScale: 1,
      child: Column(
        children: [
          ProfileSelectionField(
            label: '生日',
            value: '',
            placeholder: '選擇日期',
            icon: HkIconName.calendar,
            onTap: () {},
          ),
          const SizedBox(height: 12),
          ProfileTextField(
            label: '職業',
            controller: jobController,
            icon: HkIconName.document,
          ),
        ],
      ),
    );

    Finder iconIn(Key fieldKey, HkIconName name) => find.descendant(
      of: find.byKey(fieldKey),
      matching: find.byWidgetPredicate(
        (widget) => widget is HkIcon && widget.name == name,
      ),
    );

    final calendarX = tester
        .getCenter(
          iconIn(const ValueKey('profile-selection-生日'), HkIconName.calendar),
        )
        .dx;
    final jobX = tester
        .getCenter(
          iconIn(const ValueKey('profile-field-職業'), HkIconName.document),
        )
        .dx;

    expect((calendarX - jobX).abs(), lessThan(1));
    expect(tester.takeException(), isNull);
  });

  for (final width in <double>[320, 390, 768]) {
    testWidgets('profile card adapts at ${width.toInt()}px', (tester) async {
      await _pumpSurface(
        tester,
        width: width,
        child: ProfileSectionCard(
          title: '其他個人資料',
          description: '選填資料可協助提供更符合所在地的內容。',
          children: [
            ProfileResponsivePair(
              first: ProfileSelectionField(
                label: '性別',
                value: '',
                placeholder: '尚未設定',
                onTap: () {},
              ),
              second: ProfileSelectionField(
                label: '生日',
                value: '',
                placeholder: '選擇日期',
                onTap: () {},
              ),
            ),
          ],
        ),
      );

      final first = tester.getTopLeft(
        find.byKey(const ValueKey('profile-selection-性別')),
      );
      final second = tester.getTopLeft(
        find.byKey(const ValueKey('profile-selection-生日')),
      );
      if (width >= 768) {
        expect((first.dy - second.dy).abs(), lessThan(1));
      } else {
        expect(second.dy, greaterThan(first.dy));
      }
      expect(tester.takeException(), isNull);
    });
  }

  testWidgets('profile card stacks safely at 200 percent text scale', (
    tester,
  ) async {
    await _pumpSurface(
      tester,
      width: 320,
      textScale: 2,
      child: ProfileSectionCard(
        title: '其他個人資料',
        description: '選填資料可協助提供更符合所在地的內容。',
        children: [
          ProfileResponsivePair(
            first: ProfileSelectionField(
              label: '性別',
              value: '女性',
              placeholder: '尚未設定',
              onTap: () {},
            ),
            second: ProfileSelectionField(
              label: '生日',
              value: '1990-01-01',
              placeholder: '選擇日期',
              onTap: () {},
            ),
          ),
        ],
      ),
    );

    expect(find.text('女性'), findsOneWidget);
    expect(find.text('1990-01-01'), findsOneWidget);
    expect(tester.takeException(), isNull);
  });

  testWidgets('avatar editor has a clear accessible action', (tester) async {
    var tapped = false;
    final semantics = tester.ensureSemantics();
    try {
      await _pumpSurface(
        tester,
        width: 390,
        child: ProfileAvatarEditor(
          avatar: null,
          onPressed: () => tapped = true,
        ),
      );

      final action = find.bySemanticsLabel(RegExp('更換個人頭像'));
      expect(tester.getSize(action).height, greaterThanOrEqualTo(48));
      expect(find.text('更換照片'), findsNothing);
      expect(
        find.descendant(
          of: find.byKey(const ValueKey('profile-avatar-preview')),
          matching: find.byType(HkPlaceholder),
        ),
        findsOneWidget,
      );
      await tester.tap(action);
      await tester.pump();
      expect(tapped, isTrue);
    } finally {
      semantics.dispose();
    }
  });

  testWidgets('avatar identity card adapts at large text scale', (
    tester,
  ) async {
    await _pumpSurface(
      tester,
      width: 320,
      textScale: 2,
      child: ProfileAvatarEditor(
        avatar: null,
        displayName: 'Cora Chou',
        onPressed: () {},
      ),
    );

    final avatar = tester.getRect(
      find.byKey(const ValueKey('profile-avatar-preview')),
    );
    final identity = tester.getRect(
      find.byKey(const ValueKey('profile-avatar-identity')),
    );
    expect(identity.top, greaterThan(avatar.bottom));
    expect(find.text('Cora Chou'), findsOneWidget);
    expect(tester.takeException(), isNull);
  });

  testWidgets('profile identity and input use compact form density', (
    tester,
  ) async {
    final controller = TextEditingController(text: 'Cora Chou');
    addTearDown(controller.dispose);

    await _pumpSurface(
      tester,
      width: 390,
      textScale: 1,
      child: Column(
        children: [
          ProfileAvatarEditor(
            avatar: null,
            displayName: 'Cora Chou',
            onPressed: () {},
          ),
          const SizedBox(height: 16),
          ProfileTextField(
            label: '用戶名稱',
            controller: controller,
            maxLength: 30,
          ),
        ],
      ),
    );

    expect(
      tester.getSize(find.byType(ProfileAvatarEditor)).height,
      lessThan(130),
    );
    expect(
      tester.getSize(find.byKey(const ValueKey('profile-field-用戶名稱'))).height,
      lessThan(56),
    );
    expect(find.textContaining('/30'), findsNothing);
    expect(tester.takeException(), isNull);
  });

  testWidgets('sticky save bar exposes one accessible primary action', (
    tester,
  ) async {
    var saved = false;
    final semantics = tester.ensureSemantics();
    try {
      await _pumpSurface(
        tester,
        width: 390,
        textScale: 1,
        child: ProfileSaveBar(
          enabled: true,
          busy: false,
          onPressed: () => saved = true,
        ),
      );

      final action = find.bySemanticsLabel('儲存個人資料變更');
      expect(tester.getSize(action).height, greaterThanOrEqualTo(48));
      await tester.tap(action);
      await tester.pump();
      expect(saved, isTrue);
      expect(tester.takeException(), isNull);
    } finally {
      semantics.dispose();
    }
  });

  testWidgets('required section and save bar remain safe at 200 percent text', (
    tester,
  ) async {
    final controller = TextEditingController(text: 'Cora Chou');
    addTearDown(controller.dispose);

    await _pumpSurface(
      tester,
      width: 320,
      textScale: 2,
      child: Column(
        children: [
          ProfileSectionCard(
            title: '帳號與聯絡方式',
            description: '必填資料會用於帳號辨識與重要通知。',
            badgeLabel: '必填',
            children: [
              ProfileTextField(
                label: '用戶名稱',
                controller: controller,
                required: true,
              ),
            ],
          ),
          const SizedBox(height: 16),
          ProfileSaveBar(enabled: true, busy: false, onPressed: () {}),
        ],
      ),
    );

    expect(find.text('必填'), findsOneWidget);
    expect(find.text('儲存變更'), findsOneWidget);
    expect(tester.takeException(), isNull);
  });

  testWidgets('save bar does not consume the scaffold body height', (
    tester,
  ) async {
    tester.view.devicePixelRatio = 1;
    tester.view.physicalSize = const Size(390, 844);
    addTearDown(tester.view.resetDevicePixelRatio);
    addTearDown(tester.view.resetPhysicalSize);

    await tester.pumpWidget(
      MaterialApp(
        theme: ThemeData(
          useMaterial3: true,
          scaffoldBackgroundColor: AppColors.bgPrimary,
          textTheme: AppTypography.textTheme,
        ),
        home: Scaffold(
          body: const SizedBox.expand(key: ValueKey('profile-scaffold-body')),
          bottomNavigationBar: ProfileSaveBar(
            enabled: true,
            busy: false,
            onPressed: () {},
          ),
        ),
      ),
    );
    await tester.pump();

    expect(tester.getSize(find.byType(ProfileSaveBar)).height, lessThan(90));
    expect(
      tester
          .getSize(find.byKey(const ValueKey('profile-scaffold-body')))
          .height,
      greaterThan(680),
    );
    expect(tester.takeException(), isNull);
  });
}

Future<void> _pumpSurface(
  WidgetTester tester, {
  required double width,
  required Widget child,
  double textScale = 1.4,
}) async {
  tester.view.devicePixelRatio = 1;
  tester.view.physicalSize = Size(width, 1000);
  addTearDown(tester.view.resetDevicePixelRatio);
  addTearDown(tester.view.resetPhysicalSize);

  await tester.pumpWidget(
    MaterialApp(
      theme: ThemeData(
        useMaterial3: true,
        splashFactory: NoSplash.splashFactory,
        scaffoldBackgroundColor: AppColors.bgPrimary,
        textTheme: AppTypography.textTheme,
      ),
      home: MediaQuery(
        data: MediaQueryData(
          size: Size(width, 1000),
          textScaler: TextScaler.linear(textScale),
        ),
        child: Scaffold(
          body: SingleChildScrollView(
            child: Padding(padding: const EdgeInsets.all(16), child: child),
          ),
        ),
      ),
    ),
  );
  await tester.pump();
}
