import 'package:hawkr_buyer/core/theme/app_typography.dart';
import 'package:hawkr_buyer/features/address/data/address_model.dart';
import 'package:hawkr_buyer/features/address/data/store_pickup_model.dart';
import 'package:hawkr_buyer/features/address/presentation/widgets/address_selection_widgets.dart';
import 'package:hawkr_buyer/shared/widgets/hk_icon.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

void main() {
  testWidgets('empty picker matches the web address sheet hierarchy', (
    tester,
  ) async {
    var closeCount = 0;
    var addCount = 0;
    await _pump(
      tester,
      child: AddressPickerPanel(
        addresses: const [],
        selectedId: null,
        onSelect: (_) async => true,
        onAdd: () => addCount += 1,
        storePickups: const [],
        selectedStorePickupId: null,
        onStorePickupSelect: (_) async => true,
        onAddStorePickup: () {},
        onClose: () => closeCount += 1,
      ),
    );

    expect(find.text('物流設定'), findsOneWidget);
    expect(
      find.byKey(const ValueKey('address-picker-drag-handle')),
      findsOneWidget,
    );
    expect(find.bySemanticsLabel('關閉'), findsNothing);
    expect(find.text('還沒有宅配地址'), findsOneWidget);
    expect(find.text('先新增一筆地址，結帳時就能一鍵帶入。'), findsOneWidget);
    expect(find.text('新增宅配地址'), findsOneWidget);
    expect(find.text('取消'), findsNothing);

    await tester.tap(find.text('新增宅配地址'));
    await tester.pump();
    expect(closeCount, 1);
    expect(addCount, 1);
    expect(tester.takeException(), isNull);
  });

  testWidgets('saved address follows the web row content order and selects', (
    tester,
  ) async {
    const address = Address(
      etag: 'v1',
      id: 1,
      receiverName: '王小明',
      receiverPhone: '0912345678',
      townCode: '63000040',
      addressDetail: '民生東路一段1號',
      isDefault: true,
    );
    var selected = false;
    var closed = false;
    await _pump(
      tester,
      child: AddressPickerPanel(
        addresses: const [address],
        selectedId: address.id,
        onSelect: (_) async {
          selected = true;
          return true;
        },
        onAdd: () {},
        storePickups: const [],
        selectedStorePickupId: null,
        onStorePickupSelect: (_) async => true,
        onAddStorePickup: () {},
        onClose: () => closed = true,
      ),
    );

    expect(find.text('王小明'), findsOneWidget);
    expect(find.text('預設'), findsOneWidget);
    expect(find.text('0912345678'), findsOneWidget);
    expect(find.text('臺北市中山區民生東路一段1號'), findsOneWidget);
    expect(
      tester.getTopLeft(find.text('0912345678')).dy,
      lessThan(tester.getTopLeft(find.text('臺北市中山區民生東路一段1號')).dy),
    );

    await tester.tap(find.text('王小明'));
    await tester.pump();
    expect(selected, isTrue);
    expect(closed, isTrue);
    expect(tester.takeException(), isNull);
  });

  testWidgets('management empty state uses the website copy and primary CTA', (
    tester,
  ) async {
    var tapped = false;
    await _pump(
      tester,
      child: Column(
        mainAxisSize: MainAxisSize.min,
        children: [
          const AddressSectionHeading(),
          AddressEmptyState(
            title: '還沒有宅配地址',
            description: '先新增至少一筆地址，結帳時就能一鍵帶入。',
            actionLabel: '新增第一筆地址',
            onAction: () => tapped = true,
          ),
        ],
      ),
    );

    expect(find.text('宅配地址'), findsOneWidget);
    expect(find.text('結帳選擇宅配到家時，會自動帶入這個預設地址。'), findsOneWidget);
    expect(find.text('還沒有宅配地址'), findsOneWidget);
    final action = find.byKey(const ValueKey('address-empty-primary-action'));
    expect(tester.getSize(action).height, greaterThanOrEqualTo(48));
    await tester.tap(action);
    expect(tapped, isTrue);
    expect(tester.takeException(), isNull);
  });

  testWidgets('management card keeps edit and delete as separate actions', (
    tester,
  ) async {
    const address = Address(
      etag: 'v1',
      id: 2,
      receiverName: '非常長的收件人名稱',
      receiverPhone: '0987654321',
      townCode: '65000010',
      addressDetail: '文化路二段一百二十三號十二樓',
    );
    var edited = false;
    var deleted = false;
    final semantics = tester.ensureSemantics();
    try {
      await _pump(
        tester,
        width: 320,
        textScale: 2,
        child: Padding(
          padding: const EdgeInsets.all(16),
          child: AddressChoiceCard(
            address: address,
            selected: false,
            onTap: () {},
            onEdit: () => edited = true,
            onDelete: () => deleted = true,
          ),
        ),
      );

      final editAction = find.bySemanticsLabel('編輯地址');
      final deleteAction = find.bySemanticsLabel('刪除地址');
      expect(editAction, findsOneWidget);
      expect(deleteAction, findsOneWidget);
      expect(tester.getSize(editAction).width, greaterThanOrEqualTo(48));
      expect(tester.getSize(deleteAction).height, greaterThanOrEqualTo(48));
      await tester.tap(editAction);
      await tester.tap(deleteAction);
      expect(edited, isTrue);
      expect(deleted, isTrue);
      expect(tester.takeException(), isNull);
    } finally {
      semantics.dispose();
    }
  });

  testWidgets('empty picker stays safe at 320px and 200 percent text', (
    tester,
  ) async {
    await _pump(
      tester,
      width: 320,
      textScale: 2,
      child: AddressPickerPanel(
        addresses: const [],
        selectedId: null,
        onSelect: (_) async => true,
        onAdd: () {},
        storePickups: const [],
        selectedStorePickupId: null,
        onStorePickupSelect: (_) async => true,
        onAddStorePickup: () {},
        onClose: () {},
      ),
    );

    expect(find.byKey(const ValueKey('address-picker-panel')), findsOneWidget);
    expect(tester.takeException(), isNull);
  });

  testWidgets('address sections expand independently from a compact summary', (
    tester,
  ) async {
    final semantics = tester.ensureSemantics();
    try {
      await _pump(
        tester,
        width: 320,
        textScale: 1.4,
        child: const _CollapsibleSectionHarness(),
      );

      expect(find.text('常用門市 · 123456'), findsOneWidget);
      expect(find.text('取貨門市內容'), findsNothing);
      final collapsed = find.bySemanticsLabel(
        '7-11 取貨門市，1 筆，常用門市 · 123456，點擊展開',
      );
      expect(collapsed, findsOneWidget);
      expect(tester.getSize(collapsed).height, greaterThanOrEqualTo(48));

      await tester.tap(collapsed);
      await tester.pump(const Duration(milliseconds: 220));

      expect(find.text('取貨門市內容'), findsOneWidget);
      expect(
        find.bySemanticsLabel('7-11 取貨門市，1 筆，常用門市 · 123456，點擊收合'),
        findsOneWidget,
      );
      expect(tester.takeException(), isNull);
    } finally {
      semantics.dispose();
    }
  });

  testWidgets('7-11 card exposes store, recipient, default and actions', (
    tester,
  ) async {
    const pickup = StorePickup(
      id: 'pickup-1',
      receiverName: '王小明',
      receiverPhone: '0912345678',
      storeCode: '123456',
      storeName: '統一超商民生門市',
      storeAddress: '臺北市中山區民生東路一段1號',
      isDefault: true,
    );
    var edited = false;
    var deleted = false;
    await _pump(
      tester,
      width: 320,
      textScale: 1.4,
      child: Padding(
        padding: const EdgeInsets.all(16),
        child: StorePickupChoiceCard(
          pickup: pickup,
          selected: true,
          onTap: () {},
          onEdit: () => edited = true,
          onDelete: () => deleted = true,
        ),
      ),
    );

    expect(find.text('統一超商民生門市'), findsOneWidget);
    expect(find.text('預設'), findsOneWidget);
    expect(find.text('王小明 · 0912345678'), findsOneWidget);
    expect(find.text('7-11 店號 123456'), findsOneWidget);
    expect(find.text('臺北市中山區民生東路一段1號'), findsOneWidget);
    await tester.tap(find.bySemanticsLabel('編輯門市'));
    await tester.tap(find.bySemanticsLabel('刪除門市'));
    expect(edited, isTrue);
    expect(deleted, isTrue);
    expect(tester.takeException(), isNull);
  });

  testWidgets('logistics picker can select and add a 7-11 pickup store', (
    tester,
  ) async {
    const pickup = StorePickup(
      id: 'pickup-1',
      receiverName: '王小明',
      receiverPhone: '0912345678',
      storeCode: '123456',
      storeName: '統一超商民生門市',
      storeAddress: '臺北市中山區民生東路一段1號',
      isDefault: true,
    );
    var selected = false;
    var closed = false;
    var addCount = 0;
    await _pump(
      tester,
      child: AddressPickerPanel(
        addresses: const [],
        selectedId: null,
        onSelect: (_) async => true,
        onAdd: () {},
        storePickups: const [pickup],
        selectedStorePickupId: pickup.id,
        onStorePickupSelect: (_) async {
          selected = true;
          return true;
        },
        onAddStorePickup: () => addCount += 1,
        onClose: () => closed = true,
      ),
    );

    await tester.tap(find.byKey(const ValueKey('logistics-store-pickup-tab')));
    await tester.pump();

    expect(find.text('統一超商民生門市'), findsOneWidget);
    expect(find.text('新增 7-11 取貨門市'), findsOneWidget);
    await tester.tap(find.text('統一超商民生門市'));
    await tester.pump();
    expect(selected, isTrue);
    expect(closed, isTrue);

    closed = false;
    selected = false;
    await _pump(
      tester,
      child: AddressPickerPanel(
        addresses: const [],
        selectedId: null,
        onSelect: (_) async => true,
        onAdd: () {},
        storePickups: const [],
        selectedStorePickupId: null,
        onStorePickupSelect: (_) async => true,
        onAddStorePickup: () => addCount += 1,
        onClose: () => closed = true,
      ),
    );
    await tester.tap(find.byKey(const ValueKey('logistics-store-pickup-tab')));
    await tester.pump();
    expect(find.text('還沒有 7-11 取貨門市'), findsOneWidget);
    await tester.tap(find.text('新增 7-11 取貨門市'));
    await tester.pump();
    expect(closed, isTrue);
    expect(addCount, 1);
    expect(tester.takeException(), isNull);
  });
}

class _CollapsibleSectionHarness extends StatefulWidget {
  const _CollapsibleSectionHarness();

  @override
  State<_CollapsibleSectionHarness> createState() =>
      _CollapsibleSectionHarnessState();
}

class _CollapsibleSectionHarnessState
    extends State<_CollapsibleSectionHarness> {
  bool expanded = false;

  @override
  Widget build(BuildContext context) {
    return Padding(
      padding: const EdgeInsets.all(16),
      child: AddressCollapsibleSection(
        title: '7-11 取貨門市',
        description: '結帳時自動帶入預設門市。',
        summary: '常用門市 · 123456',
        count: 1,
        leading: const HkIcon(HkIconName.store),
        expanded: expanded,
        onToggle: () => setState(() => expanded = !expanded),
        child: const Text('取貨門市內容'),
      ),
    );
  }
}

Future<void> _pump(
  WidgetTester tester, {
  required Widget child,
  double width = 390,
  double textScale = 1,
}) async {
  await tester.pumpWidget(
    MaterialApp(
      theme: ThemeData(
        useMaterial3: true,
        colorScheme: ColorScheme.fromSeed(seedColor: const Color(0xFFB8441F)),
        textTheme: AppTypography.textTheme,
        splashFactory: NoSplash.splashFactory,
      ),
      home: MediaQuery(
        data: MediaQueryData(
          size: Size(width, 844),
          textScaler: TextScaler.linear(textScale),
          disableAnimations: true,
        ),
        child: Scaffold(
          body: Align(alignment: Alignment.bottomCenter, child: child),
        ),
      ),
    ),
  );
  await tester.pump();
}
