import 'shared/widgets/hk_temperature_tag.dart';
import 'features/product/presentation/widgets/product_temperature_tags.dart';
import 'features/post/presentation/widgets/page_header.dart';
import 'package:flutter/material.dart';
import 'package:hawkr_seller/shared/widgets/hk_icon.dart';
import 'package:flutter/foundation.dart';
import 'package:flutter/semantics.dart';
import 'package:flutter_localizations/flutter_localizations.dart';
import 'package:flutter_screenutil/flutter_screenutil.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import 'core/layout/app_media_query_policy.dart';
import 'core/theme/app_colors.dart';
import 'core/theme/app_theme.dart';
import 'core/theme/app_typography.dart';
import 'core/theme/hk_tokens.dart';
import 'features/order/presentation/order_card_widgets/pill_button.dart';
import 'shared/icons/app_icons.dart';
import 'shared/widgets/app_dropdown.dart';
import 'shared/widgets/hk_async_state_panel.dart';
import 'shared/widgets/hk_button_label.dart';
import 'shared/widgets/hk_choice_chip.dart';
import 'shared/widgets/hk_content_card.dart';
import 'shared/widgets/hk_frosted_card_surface.dart';
import 'shared/widgets/hk_frosted_icon_button.dart';
import 'shared/widgets/hk_glass_app_bar.dart';
import 'shared/widgets/hk_media_picker_layout.dart';
import 'shared/widgets/hk_page_section.dart';
import 'shared/widgets/hk_round_action_button.dart';
import 'shared/widgets/hk_status_badge.dart';
import 'shared/widgets/fruit_spinner.dart';
import 'shared/widgets/green_header_bar.dart';
import 'shared/widgets/linked_product_card.dart';
import 'shared/widgets/step_indicator.dart';
import 'shared/widgets/temperature_tag.dart';

/// Runs real seller widgets without app bootstrap, authentication, or APIs.
void main() {
  WidgetsFlutterBinding.ensureInitialized();
  runApp(SellerComponentCatalogApp.fromUri(Uri.base));
}

const sellerCatalogComponents = <String, String>{
  'foundations': '基礎樣式 Foundations',
  'chrome': '頁首與安全區域 Chrome',
  'buttons': '按鈕 Buttons',
  'inputs': '表單 Inputs',
  'chips': '篩選與狀態 Chips',
  'cards': '卡片 Cards',
  'states': '載入與回饋 States',
  'commerce': '賣家營運 Commerce',
};

const _catalogSearchTerms = <String, String>{
  'chrome':
      'GreenHeaderBar HkGlassAppBar HkMediaPickerLayout 安全區域 狀態列 標題 副標題 返回 相片 圖庫',
  'buttons':
      'HkButtonLabel HkRoundActionButton HkFrostedIconButton 霧面 送出 儲存 更多',
};

class SellerComponentCatalogApp extends StatelessWidget {
  const SellerComponentCatalogApp({
    super.key,
    this.initialComponent = 'foundations',
    this.textScale = 1,
    this.embedded = false,
  });

  factory SellerComponentCatalogApp.fromUri(Uri uri) {
    final requested = uri.queryParameters['component'];
    final scale = double.tryParse(uri.queryParameters['textScale'] ?? '1');
    return SellerComponentCatalogApp(
      initialComponent: sellerCatalogComponents.containsKey(requested)
          ? requested!
          : 'foundations',
      textScale: scale != null && scale.isFinite ? scale.clamp(0.8, 1.4) : 1,
      embedded: uri.queryParameters['embedded'] == 'true',
    );
  }

  final String initialComponent;
  final double textScale;
  final bool embedded;

  @override
  Widget build(BuildContext context) => ScreenUtilInit(
    designSize: const Size(414, 896),
    enableScaleWH: () => false,
    enableScaleText: () => false,
    minTextAdapt: false,
    splitScreenMode: true,
    builder: (context, child) => MaterialApp(
      title: 'Hawkr · 賣家元件庫',
      debugShowCheckedModeBanner: false,
      theme: AppTheme.light,
      locale: const Locale('zh', 'TW'),
      supportedLocales: const [Locale('zh', 'TW'), Locale('en', 'US')],
      localizationsDelegates: GlobalMaterialLocalizations.delegates,
      builder: (context, child) => MediaQuery(
        data: AppMediaQueryPolicy.apply(
          MediaQuery.of(
            context,
          ).copyWith(textScaler: TextScaler.linear(textScale)),
        ),
        child: child!,
      ),
      home: _CatalogPage(
        initialComponent: initialComponent,
        embedded: embedded,
      ),
    ),
  );
}

class _CatalogPage extends StatefulWidget {
  const _CatalogPage({required this.initialComponent, required this.embedded});

  final String initialComponent;
  final bool embedded;

  @override
  State<_CatalogPage> createState() => _CatalogPageState();
}

class _CatalogPageState extends State<_CatalogPage> {
  SemanticsHandle? _semanticsHandle;
  late String _component =
      sellerCatalogComponents.containsKey(widget.initialComponent)
      ? widget.initialComponent
      : 'foundations';
  String _query = '';

  @override
  void initState() {
    super.initState();
    // Expose real Flutter controls for Storybook's browser inspection.
    if (kIsWeb) {
      _semanticsHandle = SemanticsBinding.instance.ensureSemantics();
    }
  }

  @override
  void dispose() {
    _semanticsHandle?.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    final matches = sellerCatalogComponents.entries.where(
      (entry) =>
          '${entry.key} ${entry.value} ${_catalogSearchTerms[entry.key] ?? ''}'
              .toLowerCase()
              .contains(_query.toLowerCase()),
    );
    return Scaffold(
      body: SafeArea(
        child: Align(
          alignment: Alignment.topCenter,
          child: ConstrainedBox(
            constraints: const BoxConstraints(maxWidth: 1120),
            child: SingleChildScrollView(
              key: const ValueKey('catalog-scroll'),
              padding: const EdgeInsets.all(HkSpace.lg),
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.stretch,
                children: [
                  if (!widget.embedded) ...[
                    Text('Hawkr · 賣家元件庫', style: AppTypography.display),
                    const SizedBox(height: HkSpace.sm),
                    const Text('正式元件、固定範例資料。可切換分類並操作元件狀態。'),
                    const SizedBox(height: HkSpace.lg),
                    TextField(
                      key: const ValueKey('catalog-search'),
                      decoration: const InputDecoration(
                        labelText: '搜尋元件',
                        hintText: '按鈕、cards、states…',
                      ),
                      onChanged: (value) => setState(() => _query = value),
                    ),
                    const SizedBox(height: HkSpace.sm),
                    Wrap(
                      spacing: HkSpace.sm,
                      runSpacing: HkSpace.xs,
                      children: [
                        for (final entry in matches)
                          HkChoiceChip(
                            key: ValueKey('nav-${entry.key}'),
                            label: entry.value,
                            selected: _component == entry.key,
                            onTap: () => setState(() => _component = entry.key),
                          ),
                      ],
                    ),
                    if (matches.isEmpty) const Text('沒有符合的元件，請換個關鍵字。'),
                    const Divider(height: HkSpace.section),
                  ],
                  Semantics(
                    header: true,
                    child: Text(
                      sellerCatalogComponents[_component]!,
                      style: AppTypography.h1,
                    ),
                  ),
                  const SizedBox(height: HkSpace.lg),
                  _CatalogExamples(
                    key: ValueKey(_component),
                    component: _component,
                  ),
                ],
              ),
            ),
          ),
        ),
      ),
    );
  }
}

class _CatalogExamples extends StatefulWidget {
  const _CatalogExamples({super.key, required this.component});

  final String component;

  @override
  State<_CatalogExamples> createState() => _CatalogExamplesState();
}

class _CatalogExamplesState extends State<_CatalogExamples> {
  final _formKey = GlobalKey<FormState>();
  int _quantity = 1;
  int _actions = 0;
  int _step = 1;
  int _photoIndex = 0;
  String _selection = '全部';
  String _temperature = '冷藏';
  String? _category;
  bool _enabled = true;
  bool _saved = false;

  void _recordAction() => setState(() => _actions++);

  Widget _section(String title, Widget child, {String? description}) => Padding(
    padding: const EdgeInsets.only(bottom: HkSpace.xxl),
    child: HkPageSection(title: title, description: description, child: child),
  );

  Widget _stack(List<Widget> children) => Column(
    crossAxisAlignment: CrossAxisAlignment.stretch,
    children: children,
  );

  @override
  Widget build(BuildContext context) => switch (widget.component) {
    'chrome' => _chrome(),
    'buttons' => _buttons(),
    'inputs' => _inputs(),
    'chips' => _chips(),
    'cards' => _cards(),
    'states' => _states(),
    'commerce' => _commerce(),
    _ => _foundations(),
  };

  Widget _chrome() => _stack([
    _section(
      'GreenHeaderBar · 頂部安全區域',
      RepaintBoundary(
        key: const ValueKey('catalog-safe-header-preview'),
        child: ColoredBox(
          color: AppColors.bgPrimary,
          child: MediaQuery(
            data: MediaQuery.of(context).copyWith(
              padding: const EdgeInsets.only(top: 62),
              viewPadding: const EdgeInsets.only(top: 62),
            ),
            child: const GreenHeaderBar(
              key: ValueKey('catalog-safe-header'),
              title: '訂單與錢包頁首',
              showBackButton: false,
              includeTopSafeArea: true,
            ),
          ),
        ),
      ),
      description: '模擬頂部 62px 安全區＋56px 工具列，共 118px。整片由同一玻璃表面繪製，邊界沒有另外補色。',
    ),
    _section(
      'GreenHeaderBar · 長標題與操作',
      SizedBox(
        width: double.infinity,
        child: GreenHeaderBar(
          key: const ValueKey('catalog-green-header'),
          title: '商品管理與配送設定完整資訊',
          subtitle: '季節蔬菜箱・桃園配送服務',
          onBackPressed: _recordAction,
          trailing: [
            HeaderTextButton(
              key: const ValueKey('catalog-green-save'),
              text: '儲存',
              onPressed: _recordAction,
            ),
          ],
        ),
      ),
      description: '正式共用頁首，長標題與副標題須保留返回和儲存的操作空間。',
    ),
    _section(
      'HkGlassAppBar · 功能頁',
      SizedBox(
        width: double.infinity,
        height: 200,
        child: Scaffold(
          key: const ValueKey('catalog-glass-scaffold'),
          appBar: HkGlassAppBar(
            key: const ValueKey('catalog-glass-app-bar'),
            title: const Text('訂單詳細資訊與出貨進度追蹤'),
            subtitle: '配送地址與收件人聯絡資訊',
            onBackPressed: _recordAction,
            actions: [
              HeaderTextButton(
                key: const ValueKey('catalog-glass-save'),
                text: '儲存',
                onPressed: _recordAction,
              ),
            ],
          ),
          body: const Center(child: Text('功能頁內容範例')),
        ),
      ),
      description: '直接放入有高度限制的 Scaffold.appBar，檢查 1.4 倍文字仍完整容納副標題。',
    ),
    Text('頁首操作次數：$_actions', key: const ValueKey('chrome-action-count')),
    const SizedBox(height: HkSpace.xxl),
    _section(
      'HkMediaPickerLayout · 相片與圖庫',
      _mediaPicker(),
      description: '正式選圖版面依可用高度分配預覽與圖庫，橫向並排。色塊為本機範例，可點選切換預覽。',
    ),
    _section(
      'PageHeader · 商店資訊與圓形操作',
      _storeHeader(),
      description:
          '使用正式商店頁首，依長地點、獎項、訂閱人數及字級計算高度；圓形圖示維持原尺寸，外層保留 48px 操作範圍。資料為本機範例。',
    ),
  ]);

  Widget _storeHeader() => LayoutBuilder(
    builder: (context, constraints) => ProviderScope(
      child: MediaQuery(
        data: MediaQuery.of(context).copyWith(
          size: Size(constraints.maxWidth, MediaQuery.sizeOf(context).height),
          padding: EdgeInsets.zero,
          viewPadding: EdgeInsets.zero,
        ),
        child: Builder(
          builder: (context) {
            const location = '測試農產專區友善農園路十二鄰十八號';
            const award = '年度友善耕作優良農產品獎項';
            final spec = PageHeaderLayoutSpec.of(context);
            return SizedBox(
              key: const ValueKey('catalog-store-header'),
              height: spec.expandedProfileHeightFor(
                hasAward: true,
                awardText: award,
                fansCount: 12000,
                displayedLocation: location,
                description: '',
              ),
              child: PageHeader(
                sellerName: '範例生鮮友善農產直送範例商店',
                location: location,
                prize: award,
                fansCount: 12000,
                onBackPressed: _recordAction,
                onChat: _recordAction,
              ),
            );
          },
        ),
      ),
    ),
  );

  Widget _mediaPicker() {
    const colors = [
      AppColors.primarySoft,
      AppColors.warningSoft,
      AppColors.surfaceMuted,
      AppColors.neutralCream,
    ];
    return SizedBox(
      width: double.infinity,
      height: MediaQuery.orientationOf(context) == Orientation.landscape
          ? 260
          : 440,
      child: HkMediaPickerLayout(
        key: const ValueKey('catalog-media-picker'),
        preview: ColoredBox(
          color: colors[_photoIndex % colors.length],
          child: Center(
            child: Text('範例照片 ${_photoIndex + 1}', style: AppTypography.body),
          ),
        ),
        grid: GridView.builder(
          padding: const EdgeInsets.all(HkSpace.xs),
          gridDelegate: const SliverGridDelegateWithFixedCrossAxisCount(
            crossAxisCount: 4,
            crossAxisSpacing: HkSpace.xs,
            mainAxisSpacing: HkSpace.xs,
          ),
          itemCount: 8,
          itemBuilder: (context, index) => Semantics(
            button: true,
            selected: index == _photoIndex,
            label: '選擇範例照片 ${index + 1}',
            child: Material(
              color: colors[index % colors.length],
              shape: RoundedRectangleBorder(
                side: BorderSide(
                  color: index == _photoIndex
                      ? AppColors.actionPrimary
                      : Colors.transparent,
                  width: 2,
                ),
              ),
              child: InkWell(
                key: ValueKey('catalog-photo-$index'),
                onTap: () => setState(() => _photoIndex = index),
                child: ExcludeSemantics(
                  child: Center(child: Text('${index + 1}')),
                ),
              ),
            ),
          ),
        ),
      ),
    );
  }

  Widget _foundations() => _stack([
    _section(
      '品牌與語意色',
      Wrap(
        spacing: HkSpace.md,
        runSpacing: HkSpace.md,
        children: [
          for (final token in <(String, Color)>[
            ('actionPrimary', AppColors.actionPrimary),
            ('surfacePage', AppColors.surfacePage),
            ('surfaceCard', AppColors.surfaceCard),
            ('textPrimary', AppColors.textPrimary),
            ('accentFill', AppColors.accentFill),
            ('success', AppColors.success),
            ('warning', AppColors.warning),
            ('error', AppColors.error),
          ])
            SizedBox(
              width: 124,
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Container(
                    height: 48,
                    decoration: BoxDecoration(
                      color: token.$2,
                      borderRadius: BorderRadius.circular(HkRadius.sm),
                      border: Border.all(color: AppColors.borderSoft),
                    ),
                  ),
                  const SizedBox(height: HkSpace.xs),
                  Text(token.$1, style: AppTypography.meta),
                  Text(
                    '#${token.$2.toARGB32().toRadixString(16).substring(2).toUpperCase()}',
                    style: AppTypography.caption,
                  ),
                ],
              ),
            ),
        ],
      ),
      description: '直接引用 AppColors；正式 App 與元件庫共用 AppTheme.light。',
    ),
    _section(
      '文字階層',
      _stack([
        Text('精選品牌，新鮮', style: AppTypography.display),
        Text('營運總覽 · H1', style: AppTypography.h1),
        Text('商品與訂單 · Section', style: AppTypography.sectionTitle),
        Text('為每一份好食材做好準備。', style: AppTypography.body),
        Text('配送與商品資訊 · Meta', style: AppTypography.meta),
        Text('NT\$ 680', style: AppTypography.priceTotal),
      ]),
    ),
    _section(
      '間距、圓角與操作尺寸',
      const Text(
        '間距：4 / 8 / 12 / 16 / 20 / 24 / 32 / 40 / 48 / 64\n'
        '圓角：6 / 8 / 12 / 16 / 24 / 32 / 膠囊\n'
        '最小觸控範圍：48 logical px\n'
        '寬度改變版面，不縮放字體或圖示。文字支援 0.8–1.4 倍。',
      ),
    ),
  ]);

  Widget _buttons() => _stack([
    _section(
      '主要、次要與停用',
      Wrap(
        spacing: HkSpace.md,
        runSpacing: HkSpace.md,
        children: [
          FilledButton(
            key: const ValueKey('primary-action'),
            onPressed: _recordAction,
            child: const HkButtonLabel('儲存商品'),
          ),
          OutlinedButton(
            onPressed: _recordAction,
            child: const HkButtonLabel('預覽'),
          ),
          TextButton(
            onPressed: _recordAction,
            child: const HkButtonLabel('取消'),
          ),
          const FilledButton(onPressed: null, child: HkButtonLabel('尚未完成')),
        ],
      ),
      description: 'Material 按鈕使用正式 AppTheme，文字使用 HkButtonLabel。',
    ),
    _section(
      '長文字',
      FilledButton(
        onPressed: _recordAction,
        child: const HkButtonLabel('儲存商品規格與配送設定並返回商品管理'),
      ),
    ),
    _section(
      '圓形霧面圖示操作',
      Wrap(
        spacing: HkSpace.lg,
        runSpacing: HkSpace.lg,
        children: [
          HkFrostedIconButton(
            key: const ValueKey('catalog-frosted-send'),
            label: '送出留言',
            icon: AppIcons.send,
            onPressed: _recordAction,
          ),
          const HkFrostedIconButton(
            key: ValueKey('catalog-frosted-disabled'),
            label: '送出留言（停用）',
            semanticHint: '輸入留言後即可送出',
            icon: AppIcons.send,
            onPressed: null,
          ),
          HkFrostedIconButton(
            key: const ValueKey('catalog-frosted-loading'),
            label: '送出留言（處理中）',
            icon: AppIcons.send,
            loading: true,
            loadingLabel: '正在送出留言',
            onPressed: _recordAction,
          ),
          HkFrostedIconButton(
            key: const ValueKey('catalog-frosted-more'),
            label: '更多留言操作',
            icon: AppIcons.moreHoriz,
            onPressed: _recordAction,
          ),
          HkFrostedIconButton(
            key: const ValueKey('catalog-frosted-save'),
            label: '儲存編輯',
            icon: AppIcons.check,
            onPressed: _recordAction,
          ),
        ],
      ),
      description: 'HkFrostedIconButton：送出、停用、處理中、更多與儲存共用相同圓形表面。',
    ),
    _section(
      '圓形加減操作',
      Row(
        children: [
          HkRoundActionButton(
            key: const ValueKey('quantity-minus'),
            semanticLabel: '減少數量',
            symbol: HkRoundActionSymbol.minus,
            onPressed: _quantity > 1 ? () => setState(() => _quantity--) : null,
          ),
          Padding(
            padding: const EdgeInsets.symmetric(horizontal: HkSpace.lg),
            child: Text('$_quantity', key: const ValueKey('quantity')),
          ),
          HkRoundActionButton(
            key: const ValueKey('quantity-plus'),
            semanticLabel: '增加數量',
            symbol: HkRoundActionSymbol.plus,
            onPressed: () => setState(() => _quantity++),
          ),
        ],
      ),
    ),
    Text('操作次數：$_actions', key: const ValueKey('action-count')),
  ]);

  Widget _inputs() => Form(
    key: _formKey,
    child: _stack([
      _section(
        '商品名稱',
        TextFormField(
          key: const ValueKey('product-name'),
          decoration: const InputDecoration(
            labelText: '商品名稱',
            hintText: '輸入清楚易懂的商品名稱',
            helperText: '表單內容僅保留於此預覽。',
            helperMaxLines: 2,
          ),
          validator: (value) =>
              value == null || value.trim().isEmpty ? '請填寫商品名稱' : null,
        ),
      ),
      _section(
        '停用與錯誤',
        _stack([
          const TextField(
            enabled: false,
            decoration: InputDecoration(labelText: '系統商品編號', hintText: '建立後產生'),
          ),
          const SizedBox(height: HkSpace.lg),
          const TextField(
            decoration: InputDecoration(labelText: '庫存', errorText: '庫存不可小於 0'),
          ),
        ]),
      ),
      _section(
        'AppDropdown',
        AppDropdown(
          key: const ValueKey('category-dropdown'),
          items: const ['蔬菜', '水果', '冷藏食品'],
          selectedItem: _category,
          placeholder: '選擇商品分類',
          onSelected: (value) => setState(() => _category = value),
        ),
      ),
      CheckboxListTile(
        title: const Text('開放預訂'),
        value: _enabled,
        onChanged: (value) => setState(() => _enabled = value ?? false),
        contentPadding: EdgeInsets.zero,
        controlAffinity: ListTileControlAffinity.leading,
      ),
      FilledButton(
        key: const ValueKey('validate-form'),
        onPressed: () =>
            setState(() => _saved = _formKey.currentState!.validate()),
        child: const HkButtonLabel('檢查表單'),
      ),
      if (_saved) const Text('表單檢查通過', key: ValueKey('form-result')),
    ]),
  );

  Widget _chips() => _stack([
    _section(
      'HkChoiceChip',
      Wrap(
        spacing: HkSpace.sm,
        runSpacing: HkSpace.sm,
        children: [
          for (final label in ['全部', '上架中', '草稿'])
            HkChoiceChip(
              label: label,
              selected: _selection == label,
              onTap: () => setState(() => _selection = label),
            ),
        ],
      ),
    ),
    Text('目前篩選：$_selection', key: const ValueKey('filter-value')),
    const SizedBox(height: HkSpace.lg),
    _section(
      'HkStatusBadge',
      const Wrap(
        spacing: HkSpace.sm,
        runSpacing: HkSpace.md,
        children: [
          HkStatusBadge(label: '草稿'),
          HkStatusBadge(label: '上架中', tone: HkStatusTone.success),
          HkStatusBadge(label: '待出貨', tone: HkStatusTone.warning),
          HkStatusBadge(label: '配送中', tone: HkStatusTone.info),
          HkStatusBadge(label: '審核未通過', tone: HkStatusTone.danger),
        ],
      ),
      description: '每個狀態保留文字，避免只靠顏色辨認。',
    ),
  ]);

  Widget _cards() => _stack([
    _section(
      'HkContentCard · 標準內容',
      const HkContentCard(child: Text('商品說明與營運資料使用清楚、穩定的內容表面。')),
    ),
    _section(
      'HkContentCard · 可操作',
      HkContentCard(
        elevated: true,
        semanticLabel: '查看範例商品',
        onTap: _recordAction,
        child: const HkPageSection(
          title: '季節鮮採蔬菜箱',
          description: '示範資料 · 點擊卡片可驗證互動',
          child: HkStatusBadge(label: '上架中', tone: HkStatusTone.success),
        ),
      ),
    ),
    _section(
      'HkFrostedCardSurface',
      const HkFrostedCardSurface(
        child: Padding(
          padding: EdgeInsets.all(HkSpace.xxl),
          child: Text('現有霧面卡片表面。新內容列表優先使用 HkContentCard。'),
        ),
      ),
    ),
    Text('卡片操作次數：$_actions'),
  ]);

  Widget _states() => _stack([
    _section(
      '載入中 · FruitSpinner',
      const Row(
        children: [
          FruitSpinner(),
          SizedBox(width: HkSpace.lg),
          Expanded(child: Text('正在載入商品…')),
        ],
      ),
    ),
    _section(
      '空狀態',
      HkAsyncStatePanel(
        kind: HkAsyncStateKind.empty,
        title: '尚未建立商品',
        description: '新增第一項商品，讓買家認識你的好食材。',
        icon: HkIconName.goods,
        actionLabel: '新增商品',
        onAction: _recordAction,
      ),
    ),
    _section(
      '錯誤與重試',
      HkAsyncStatePanel(
        kind: HkAsyncStateKind.error,
        title: '暫時無法載入',
        description: '請稍後再試。已填寫的資料不受影響。',
        actionLabel: '重新載入',
        onAction: _recordAction,
      ),
    ),
    _section(
      '離線',
      HkAsyncStatePanel(
        kind: HkAsyncStateKind.offline,
        title: '目前沒有網路連線',
        description: '確認連線後再試一次。',
        actionLabel: '再試一次',
        onAction: _recordAction,
      ),
    ),
    Text('回饋操作次數：$_actions'),
  ]);

  Widget _commerce() => _stack([
    _section(
      '配送溫層標籤 · HkTemperatureTag',
      const ProductTemperatureTags(deliveryTemp: ['3201', '3202', '3203']),
      description: '與 Web 相同的透明圓角 pill；常溫灰、冷藏橘、冷凍藍，標籤隨字級增高。',
    ),
    _section(
      '缺值與未知溫層',
      const Wrap(
        spacing: 4,
        runSpacing: 4,
        children: [
          ProductTemperatureTags(deliveryTemp: null),
          HkTemperatureTag(value: '待確認'),
        ],
      ),
    ),
    _section(
      '配送溫層選擇 · TemperatureTag',
      Wrap(
        spacing: HkSpace.sm,
        runSpacing: HkSpace.sm,
        children: [
          for (final label in ['常溫', '冷藏', '冷凍'])
            TemperatureTag.selectable(
              key: ValueKey('temperature-select-$label'),
              label: label,
              isSelected: _temperature == label,
              onTap: () => setState(() => _temperature = label),
            ),
        ],
      ),
    ),
    Text('選擇溫層：$_temperature', key: const ValueKey('temperature-value')),
    const SizedBox(height: HkSpace.lg),
    _section(
      '關聯商品 · LinkedProductCard',
      LinkedProductCard(
        linkedProduct: const {'name': '季節鮮採蔬菜箱 · 產地直送綜合組', 'price': 680},
        inset: false,
        onTap: _recordAction,
      ),
      description: '固定範例資料；未提供圖片時使用正式元件的缺圖狀態。',
    ),
    _section(
      '訂單操作 · 內容寬度 PillButton',
      SizedBox(
        width: double.infinity,
        child: Wrap(
          key: const ValueKey('catalog-order-actions'),
          spacing: HkSpace.sm,
          runSpacing: HkSpace.sm,
          alignment: WrapAlignment.end,
          crossAxisAlignment: WrapCrossAlignment.center,
          children: [
            PillButton(
              key: const ValueKey('catalog-order-details'),
              label: '細節',
              color: AppColors.primary,
              onTap: _recordAction,
            ),
            PillButton(
              key: const ValueKey('catalog-order-shipping'),
              label: '預約出貨',
              color: AppColors.warning,
              onTap: _recordAction,
            ),
          ],
        ),
      ),
      description: '沿用正式訂單按鈕色彩與圓角；按內容決定寬度，保留 48dp 操作範圍。空間不足才換行，點擊僅更新範例計數。',
    ),
    _section(
      '物流進度 · StepIndicator',
      _stack([
        StepIndicator(steps: StepIndicator.logisticsSteps, currentStep: _step),
        const SizedBox(height: HkSpace.lg),
        OutlinedButton(
          key: const ValueKey('next-step'),
          onPressed: () => setState(() => _step = (_step + 1) % 4),
          child: const HkButtonLabel('切換物流步驟'),
        ),
      ]),
    ),
    Text('營運操作次數：$_actions'),
  ]);
}
