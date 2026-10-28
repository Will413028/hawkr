import 'package:flutter/foundation.dart' show kIsWeb;
import 'package:flutter/material.dart';
import 'package:flutter/semantics.dart' show SemanticsHandle;
import 'package:flutter_screenutil/flutter_screenutil.dart';

import '../core/layout/app_media_query_policy.dart';
import '../core/layout/app_text_size.dart';
import '../features/cart/presentation/cart_typography.dart';
import '../core/theme/app_colors.dart';
import '../core/theme/app_theme.dart';
import '../core/theme/app_typography.dart';
import '../core/theme/hk_tokens.dart';
import '../features/address/data/address_model.dart';
import '../features/cart/data/cart_model.dart';
import '../features/cart/presentation/widgets/merchant_group.dart';
import '../features/cart/presentation/widgets/merchant_group_card.dart';
import '../features/cart/presentation/widgets/cart_checkout_action.dart';
import '../features/cart/presentation/widgets/cart_top_bar.dart';
import '../features/cart/presentation/widgets/shipping_section.dart';
import '../features/cart/presentation/widgets/payment_details_section.dart';
import '../features/cart/presentation/widgets/delivery_notes_section.dart';
import '../features/cart/presentation/widgets/address_picker_sheet.dart';
import '../shared/widgets/hk_icon.dart';
import '../shared/icons/app_icons.dart';
import '../shared/widgets/hk_empty_state.dart';
import '../shared/widgets/hk_button_label.dart';
import '../shared/widgets/hk_choice_chip.dart';
import '../shared/widgets/hk_frosted_card_surface.dart';
import '../shared/widgets/hk_frosted_icon_button.dart';
import '../shared/widgets/hk_post_card_surface.dart';
import '../shared/widgets/hk_round_action_button.dart';
import '../shared/widgets/hk_secondary_cta_tabs.dart';
import '../shared/widgets/notification_count_badge.dart';
import '../shared/widgets/product_card.dart';
import '../shared/widgets/quantity_stepper.dart';
import '../shared/widgets/hk_brand_loader.dart';
import '../shared/widgets/hk_skeleton_shimmer.dart';

/// These IDs are also used by the seller catalog and Web Storybook iframe stories.
const buyerCatalogSections = <String, String>{
  'foundations': '基礎樣式',
  'buttons': '按鈕',
  'inputs': '輸入欄位',
  'chips': '選擇與標籤',
  'cards': '卡片表面',
  'states': '載入與回饋',
  'commerce': '商品與數量',
  'cart': '購物車',
};

const _searchTerms = <String, String>{
  'foundations': 'AppColors AppTypography HkSpace HkRadius 色彩 字級 間距 圓角',
  'buttons':
      'HkButtonLabel HkRoundActionButton HkFrostedIconButton FilledButton OutlinedButton TextButton CTA 霧面 送出 儲存 更多',
  'inputs': 'TextFormField TextField 表單 驗證 停用',
  'chips': 'HkChoiceChip HkSecondaryCtaTabs 篩選 頁籤 單選',
  'cards': 'HkFrostedCardSurface HkPostCardSurface Card 玻璃 貼文',
  'states':
      'HkBrandLoader HkSkeletonShimmer HkEmptyState LinearProgressIndicator 載入 骨架屏 空狀態 錯誤',
  'commerce': 'ProductCard QuantityStepper NotificationCountBadge 商品 數量 缺貨 徽章',
  'cart':
      'MerchantGroupCard CartProductTile ShippingSection PaymentDetailsSection DeliveryNotesSection CartCheckoutAction 購物車 地址 配送 結帳',
};

class BuyerComponentCatalogApp extends StatefulWidget {
  const BuyerComponentCatalogApp({super.key, this.initialUri});

  final Uri? initialUri;

  @override
  State<BuyerComponentCatalogApp> createState() =>
      _BuyerComponentCatalogAppState();
}

class _BuyerComponentCatalogAppState extends State<BuyerComponentCatalogApp> {
  SemanticsHandle? _semanticsHandle;
  late final bool _embedded;
  late String _section;
  late double _textScale;
  late AppTextSize _textSize;
  String _query = '';

  @override
  void initState() {
    super.initState();
    // Catalog-only: expose real Flutter controls to Storybook/browser tools
    // without first requiring a visitor to activate the accessibility overlay.
    if (kIsWeb) _semanticsHandle = WidgetsBinding.instance.ensureSemantics();
    final params = (widget.initialUri ?? Uri.base).queryParameters;
    _textSize = AppTextSize.fromStorage(params['textSize']);
    final requested = params['component'];
    _section = buyerCatalogSections.containsKey(requested)
        ? requested!
        : 'foundations';
    _textScale = (double.tryParse(params['textScale'] ?? '') ?? 1).clamp(
      AppMediaQueryPolicy.minTextScale,
      AppMediaQueryPolicy.maxTextScale,
    );
    _embedded = params['embedded'] == 'true';
  }

  @override
  void dispose() {
    _semanticsHandle?.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    return ScreenUtilInit(
      designSize: const Size(414, 896),
      enableScaleWH: () => false,
      enableScaleText: () => false,
      minTextAdapt: false,
      splitScreenMode: true,
      builder: (context, child) => MaterialApp(
        title: 'Hawkr · 買家元件庫',
        debugShowCheckedModeBanner: false,
        theme: AppTheme.light,
        builder: (context, child) => MediaQuery(
          data: AppMediaQueryPolicy.apply(
            MediaQuery.of(
              context,
            ).copyWith(textScaler: TextScaler.linear(_textScale)),
            textSize: _textSize,
          ),
          child: child!,
        ),
        home: Scaffold(
          body: SafeArea(
            child: LayoutBuilder(
              builder: (context, constraints) {
                final wide = constraints.maxWidth >= HkBreakpoints.wide;
                final content = _content(context);
                if (_embedded) return content;
                if (wide) {
                  return Row(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      SizedBox(width: 280, child: _navigation(context)),
                      const VerticalDivider(width: 1),
                      Expanded(child: content),
                    ],
                  );
                }
                return CustomScrollView(
                  slivers: [
                    SliverToBoxAdapter(
                      child: _navigation(context, compact: true),
                    ),
                    SliverFillRemaining(hasScrollBody: true, child: content),
                  ],
                );
              },
            ),
          ),
        ),
      ),
    );
  }

  Widget _navigation(BuildContext context, {bool compact = false}) {
    final entries = buyerCatalogSections.entries.where((entry) {
      final search = '${entry.key} ${entry.value} ${_searchTerms[entry.key]}'
          .toLowerCase();
      return search.contains(_query.toLowerCase().trim());
    }).toList();
    final body = Column(
      crossAxisAlignment: CrossAxisAlignment.stretch,
      mainAxisSize: MainAxisSize.min,
      children: [
        Text('Hawkr', style: AppTypography.h1),
        const SizedBox(height: HkSpace.xs),
        Text('買家元件庫', style: AppTypography.metaMuted),
        const SizedBox(height: HkSpace.lg),
        TextField(
          key: const ValueKey('catalog-search'),
          decoration: const InputDecoration(
            labelText: '搜尋元件',
            hintText: '例如 ProductCard',
          ),
          onChanged: (value) => setState(() => _query = value),
        ),
        const SizedBox(height: HkSpace.sm),
        if (entries.isEmpty) const Text('找不到元件，請換個關鍵字。'),
        if (compact)
          Wrap(
            spacing: HkSpace.sm,
            children: [for (final entry in entries) _sectionChip(entry)],
          )
        else
          for (final entry in entries)
            Align(alignment: Alignment.centerLeft, child: _sectionChip(entry)),
        const SizedBox(height: HkSpace.sm),
        Wrap(
          spacing: HkSpace.sm,
          crossAxisAlignment: WrapCrossAlignment.center,
          children: [
            Text('文字比例', style: AppTypography.meta),
            for (final value in [1.0, 1.4])
              HkChoiceChip(
                label: '${value.toStringAsFixed(1)}×',
                selected: _textScale == value,
                onTap: () => setState(() => _textScale = value),
              ),
          ],
        ),
      ],
    );
    final padding = Padding(
      padding: const EdgeInsets.all(HkSpace.lg),
      child: body,
    );
    return compact ? padding : SingleChildScrollView(child: padding);
  }

  Widget _sectionChip(MapEntry<String, String> entry) => HkChoiceChip(
    key: ValueKey('catalog-nav-${entry.key}'),
    label: entry.value,
    selected: _section == entry.key,
    onTap: () => setState(() => _section = entry.key),
  );

  Widget _content(BuildContext context) => SingleChildScrollView(
    key: ValueKey('catalog-scroll-$_section'),
    padding: const EdgeInsets.all(HkSpace.lg),
    child: Align(
      alignment: Alignment.topCenter,
      child: ConstrainedBox(
        constraints: const BoxConstraints(maxWidth: 720),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.stretch,
          children: [
            Text(
              '買家 · ${buyerCatalogSections[_section]}',
              style: _section == 'cart'
                  ? CartTypography.navigationTitle
                  : AppTypography.h1,
            ),
            const SizedBox(height: HkSpace.sm),
            Text(
              '正式元件 · ${_textScale.toStringAsFixed(1)}× 文字',
              style: AppTypography.metaMuted,
            ),
            const SizedBox(height: HkSpace.xxl),
            _CatalogExamples(
              key: ValueKey(_section),
              section: _section,
              textSize: _textSize,
              onTextSizeChanged: (value) => setState(() => _textSize = value),
            ),
          ],
        ),
      ),
    ),
  );
}

class _CatalogExamples extends StatefulWidget {
  const _CatalogExamples({
    super.key,
    required this.section,
    required this.textSize,
    required this.onTextSizeChanged,
  });

  final AppTextSize textSize;
  final ValueChanged<AppTextSize> onTextSizeChanged;

  final String section;

  @override
  State<_CatalogExamples> createState() => _CatalogExamplesState();
}

class _CatalogExamplesState extends State<_CatalogExamples> {
  final _formKey = GlobalKey<FormState>();
  final _cartNotes = TextEditingController();
  Set<int> _cartSelectedSkuIds = {101};
  int _secondCartCount = 1;
  int _shippingIndex = 0;
  int _count = 1;
  int _taps = 0;
  int _tab = 0;
  String _choice = '全部';
  bool _favorite = false;
  bool _retried = false;

  @override
  void dispose() {
    _cartNotes.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) => switch (widget.section) {
    'buttons' => _buttons(),
    'inputs' => _inputs(),
    'chips' => _chips(),
    'cards' => _cards(),
    'states' => _states(),
    'commerce' => _commerce(context),
    'cart' => _cart(context),
    _ => _foundations(),
  };

  Widget _foundations() => _stack([
    _example(
      '色彩角色',
      'core/theme/app_colors.dart · AppColors',
      Wrap(
        spacing: HkSpace.lg,
        runSpacing: HkSpace.lg,
        children: [
          for (final token in <(String, Color)>[
            ('actionPrimary', AppColors.actionPrimary),
            ('brandPrimaryStrong', AppColors.brandPrimaryStrong),
            ('brandSecondary', AppColors.brandSecondary),
            ('surfacePage', AppColors.surfacePage),
            ('bgCard', AppColors.bgCard),
            ('textEmphasis', AppColors.textEmphasis),
            ('success', AppColors.success),
            ('danger', AppColors.danger),
          ])
            SizedBox(
              width: 152,
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Container(
                    height: HkSpace.control,
                    decoration: BoxDecoration(
                      color: token.$2,
                      border: Border.all(color: AppColors.borderSoft),
                      borderRadius: BorderRadius.circular(HkRadius.sm),
                    ),
                  ),
                  const SizedBox(height: HkSpace.sm),
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
    ),
    _example(
      '字級階層',
      'core/theme/app_typography.dart · AppTypography',
      _stack([
        Text('頁面標題 · 20', style: AppTypography.h1),
        Text('區塊標題 · 18', style: AppTypography.sectionTitle),
        Text('精選品牌，好食 · 16', style: AppTypography.body),
        Text('產地與配送資訊 · 14', style: AppTypography.meta),
        Text('補充說明 · 12', style: AppTypography.caption),
        Text('NT\$ 1,280', style: AppTypography.priceTotal),
      ], gap: HkSpace.md),
    ),
    _example(
      '間距與圓角',
      'core/theme/hk_tokens.dart · HkSpace / HkRadius / HkControl',
      _stack([
        Text(
          '4 / 8 / 12 / 16 / 20 / 24 / 32 / 40 / 48 / 64',
          style: AppTypography.meta,
        ),
        Wrap(
          spacing: HkSpace.sm,
          runSpacing: HkSpace.sm,
          children: [
            for (final radius in [
              HkRadius.xs,
              HkRadius.sm,
              HkRadius.md,
              HkRadius.lg,
              HkRadius.xl,
            ])
              Container(
                width: 64,
                height: 64,
                alignment: Alignment.center,
                decoration: BoxDecoration(
                  color: AppColors.surfacePanel,
                  borderRadius: BorderRadius.circular(radius),
                ),
                child: Text('${radius.toInt()}'),
              ),
          ],
        ),
        const Text('最小操作範圍 48 logical px；版面重排，文字不隨螢幕寬度縮小。'),
      ]),
    ),
  ]);

  Widget _buttons() {
    void tap() => setState(() => _taps++);
    return _stack([
      _example(
        '操作層級',
        'core/theme/app_theme.dart · shared/widgets/hk_button_label.dart',
        _stack([
          FilledButton(
            key: const ValueKey('catalog-primary-action'),
            onPressed: tap,
            child: const HkButtonLabel('加入購物車'),
          ),
          ElevatedButton(onPressed: tap, child: const HkButtonLabel('確認選擇')),
          OutlinedButton(onPressed: tap, child: const HkButtonLabel('繼續選購')),
          TextButton(onPressed: tap, child: const HkButtonLabel('查看詳情')),
          const FilledButton(onPressed: null, child: HkButtonLabel('暫時無法購買')),
          FilledButton(
            onPressed: tap,
            child: const HkButtonLabel('動態長文案：確認這次加入購物車的商品與規格'),
          ),
          Text('已操作 $_taps 次', key: const ValueKey('catalog-action-result')),
        ], gap: HkSpace.md),
      ),
      _example(
        '圓形霧面圖示操作',
        'shared/widgets/hk_frosted_icon_button.dart · HkFrostedIconButton',
        _stack([
          const Text('送出、停用、處理中、更多與儲存共用相同圓形表面。'),
          Wrap(
            spacing: HkSpace.lg,
            runSpacing: HkSpace.lg,
            children: [
              HkFrostedIconButton(
                key: const ValueKey('catalog-frosted-send'),
                label: '送出留言',
                icon: AppIcons.send,
                onPressed: tap,
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
                onPressed: tap,
              ),
              HkFrostedIconButton(
                key: const ValueKey('catalog-frosted-more'),
                label: '更多留言操作',
                icon: AppIcons.moreHoriz,
                onPressed: tap,
              ),
              HkFrostedIconButton(
                key: const ValueKey('catalog-frosted-save'),
                label: '儲存編輯',
                icon: AppIcons.check,
                onPressed: tap,
              ),
            ],
          ),
        ]),
      ),
      _example(
        '圓形加減',
        'shared/widgets/hk_round_action_button.dart · HkRoundActionButton',
        Wrap(
          spacing: HkSpace.lg,
          children: [
            HkRoundActionButton(
              semanticLabel: '範例增加',
              symbol: HkRoundActionSymbol.plus,
              onPressed: tap,
            ),
            HkRoundActionButton(
              semanticLabel: '範例減少',
              symbol: HkRoundActionSymbol.minus,
              onPressed: tap,
            ),
            const HkRoundActionButton(
              semanticLabel: '停用增加',
              symbol: HkRoundActionSymbol.plus,
              onPressed: null,
            ),
          ],
        ),
      ),
    ]);
  }

  Widget _inputs() => _stack([
    _example(
      '表單狀態',
      'core/theme/app_theme.dart · TextFormField / TextField',
      Form(
        key: _formKey,
        child: _stack([
          TextFormField(
            key: const ValueKey('catalog-name-input'),
            decoration: const InputDecoration(
              labelText: '收件人姓名',
              hintText: '請輸入姓名',
            ),
            validator: (value) =>
                value == null || value.trim().isEmpty ? '請填寫收件人姓名' : null,
          ),
          const TextField(
            enabled: false,
            decoration: InputDecoration(
              labelText: '停用欄位',
              hintText: '配送方式尚未開放',
            ),
          ),
          const TextField(
            decoration: InputDecoration(labelText: '地址', errorText: '請確認門牌號碼'),
          ),
          FilledButton(
            key: const ValueKey('catalog-validate'),
            onPressed: () => _formKey.currentState!.validate(),
            child: const HkButtonLabel('檢查必填欄位'),
          ),
        ], gap: HkSpace.xxl),
      ),
    ),
  ]);

  Widget _chips() => _stack([
    _example(
      '單選篩選',
      'shared/widgets/hk_choice_chip.dart · HkChoiceChip',
      _stack([
        Wrap(
          spacing: HkSpace.sm,
          children: [
            for (final label in ['全部', '冷藏', '冷凍', '常溫'])
              HkChoiceChip(
                label: label,
                selected: _choice == label,
                onTap: () => setState(() => _choice = label),
              ),
          ],
        ),
        Text('目前篩選：$_choice', key: const ValueKey('catalog-choice-result')),
      ]),
    ),
    _example(
      '次要頁籤',
      'shared/widgets/hk_secondary_cta_tabs.dart · HkSecondaryCtaTabs',
      _stack([
        HkSecondaryCtaTabs(
          labels: const ['商品', '動態'],
          selectedIndex: _tab,
          onSelected: (value) => setState(() => _tab = value),
          itemKeyPrefix: 'catalog-tab',
        ),
        Text(_tab == 0 ? '商品內容範例' : '品牌動態範例'),
      ]),
    ),
  ]);

  Widget _cards() => _stack([
    _example(
      '內容卡片',
      'core/theme/app_theme.dart · CardThemeData',
      Card(
        child: Padding(
          padding: const EdgeInsets.all(HkSpace.lg),
          child: _cardCopy(),
        ),
      ),
    ),
    _example(
      '霧面卡片',
      'shared/widgets/hk_frosted_card_surface.dart · HkFrostedCardSurface',
      HkFrostedCardSurface(
        child: Padding(
          padding: const EdgeInsets.all(HkSpace.lg),
          child: _cardCopy(),
        ),
      ),
    ),
    _example(
      '貼文表面',
      'shared/widgets/hk_post_card_surface.dart · HkPostCardSurface',
      HkPostCardSurface(
        onTap: () => setState(() => _taps++),
        child: Padding(
          padding: const EdgeInsets.all(HkSpace.lg),
          child: _stack([_cardCopy(), Text('已開啟 $_taps 次 · 點按此卡片')]),
        ),
      ),
    ),
  ]);

  Widget _cardCopy() => _stack([
    Text('從產地到餐桌', style: AppTypography.sectionTitle),
    const Text('用清楚的商品資訊，認識食材與品牌。'),
    Text('元件範例內容', style: AppTypography.metaMuted),
  ], gap: HkSpace.sm);

  Widget _states() => _stack([
    _example(
      '載入中',
      'shared/widgets/hk_brand_loader.dart · HkBrandLoader',
      const Column(
        crossAxisAlignment: CrossAxisAlignment.stretch,
        children: [
          Align(
            alignment: Alignment.centerLeft,
            child: HkBrandLoader(
              size: HkBrandLoader.page,
              semanticsLabel: '正在載入商品',
            ),
          ),
          SizedBox(height: HkSpace.lg),
          Text('中性底用品牌四色；實心品牌色底改用 mono，否則葉片會被底色吃掉。'),
          SizedBox(height: HkSpace.sm),
          Row(
            children: [
              ColoredBox(
                color: AppColors.brandPrimaryStrong,
                child: Padding(
                  padding: EdgeInsets.all(HkSpace.sm),
                  child: HkBrandLoader(
                    size: HkBrandLoader.buttonLg,
                    tone: HkBrandLoaderTone.onDark,
                    semanticsLabel: null,
                  ),
                ),
              ),
              SizedBox(width: HkSpace.sm),
              HkBrandLoader(size: HkBrandLoader.inline, semanticsLabel: null),
            ],
          ),
          SizedBox(height: HkSpace.lg),
          LinearProgressIndicator(value: 0.6, semanticsLabel: '載入進度'),
          SizedBox(height: HkSpace.sm),
          Text('正在整理商品資訊…'),
        ],
      ),
    ),
    _example(
      '骨架屏掃光',
      'shared/widgets/hk_skeleton_shimmer.dart · HkSkeletonShimmer',
      const HkSkeletonShimmer(
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.stretch,
          children: [
            SizedBox(
              height: 72,
              child: ColoredBox(color: AppColors.dividerLight),
            ),
            SizedBox(height: HkSpace.sm),
            SizedBox(
              height: 16,
              child: ColoredBox(color: AppColors.dividerLight),
            ),
          ],
        ),
      ),
    ),
    _example(
      '空狀態',
      'shared/widgets/hk_empty_state.dart · HkEmptyState',
      const HkEmptyState(
        icon: HkIconName.emptySearch,
        message: '沒有找到符合的商品',
        description: '試試其他關鍵字，或調整篩選條件。',
        topPadding: HkSpace.sm,
      ),
    ),
    _example(
      '錯誤與重試',
      'AppTheme + Text + OutlinedButton · 元件組合範例',
      _stack([
        Text(
          _retried ? '範例已重試，可以繼續瀏覽。' : '暫時無法載入，請稍後再試。',
          key: const ValueKey('catalog-retry-result'),
        ),
        OutlinedButton(
          key: const ValueKey('catalog-retry'),
          onPressed: () => setState(() => _retried = true),
          child: const HkButtonLabel('重新載入'),
        ),
      ]),
    ),
  ]);

  Widget _commerce(BuildContext context) => _stack([
    _example(
      '購買數量',
      'shared/widgets/quantity_stepper.dart · QuantityStepper',
      _stack([
        Align(
          alignment: Alignment.centerLeft,
          child: QuantityStepper(
            count: _count,
            maxCount: 5,
            onIncrement: () => setState(() => _count++),
            onDecrement: () => setState(() => _count--),
          ),
        ),
        Text(
          '範例數量：$_count / 5',
          key: const ValueKey('catalog-quantity-result'),
        ),
        const Text('最少 1 件、最多 5 件；邊界會停用對應操作。'),
      ]),
    ),
    _example(
      '商品卡片',
      'shared/widgets/product_card.dart · ProductCard',
      _stack([
        const Text('固定範例資料，呈現無圖片 fallback；價格與庫存皆為展示用。'),
        Align(
          alignment: Alignment.centerLeft,
          child: ConstrainedBox(
            constraints: const BoxConstraints(maxWidth: 340),
            child: LayoutBuilder(
              builder: (context, constraints) => SizedBox(
                height:
                    constraints.maxWidth * 3 / 4 +
                    72 +
                    (ProductCard.effectiveTextScale(context) - 1) * 20,
                child: ProductCard(
                  name: '產地直送當季蔬果箱',
                  price: '680',
                  listPriceMin: '780',
                  sellerName: '精選品牌範例',
                  hashtags: const ['當季食材', '常溫'],
                  isFavorited: _favorite,
                  onFavoriteTap: () => setState(() => _favorite = !_favorite),
                  onTap: () => setState(() => _taps++),
                  onAddToCartTap: () => setState(() => _taps++),
                ),
              ),
            ),
          ),
        ),
        Text('收藏：${_favorite ? '已收藏' : '未收藏'} · 商品操作 $_taps 次'),
        Align(
          alignment: Alignment.centerLeft,
          child: ConstrainedBox(
            constraints: const BoxConstraints(maxWidth: 340),
            child: LayoutBuilder(
              builder: (context, constraints) => SizedBox(
                height:
                    constraints.maxWidth * 3 / 4 +
                    72 +
                    (ProductCard.effectiveTextScale(context) - 1) * 20,
                child: const ProductCard(
                  name: '本季限量禮盒',
                  price: '1280',
                  isSoldOut: true,
                ),
              ),
            ),
          ),
        ),
      ]),
    ),
    _example(
      '通知徽章',
      'shared/widgets/notification_count_badge.dart · NotificationCountBadge',
      const Wrap(
        spacing: HkSpace.lg,
        children: [
          SizedBox(width: 40, child: NotificationCountBadge(count: 3)),
          SizedBox(width: 56, child: NotificationCountBadge(count: 128)),
        ],
      ),
    ),
  ]);

  Widget _cart(BuildContext context) {
    const address = Address(
      id: 1,
      etag: 'catalog',
      townCode: '63000050',
      addressDetail: '範例路一段 123 號 8 樓',
      receiverName: '範例收件人',
      receiverPhone: '0912345678',
    );
    final groups = [
      MerchantGroup(
        storeId: 'demo-produce',
        sellerName: '範例蔬果店',
        sellerCity: '台北市',
        items: [
          CartItem(
            id: 1,
            skuId: 101,
            count: _count,
            price: 680,
            title: '產地直送當季蔬果箱',
            specName: '家庭箱／約 3 公斤',
          ),
        ],
      ),
      MerchantGroup(
        storeId: 'demo-seafood',
        sellerName: '範例海鮮店',
        sellerCity: '宜蘭縣',
        items: [
          CartItem(
            id: 2,
            skuId: 202,
            count: _secondCartCount,
            price: 420,
            title: '嚴選冷凍鮮魚',
            specName: '家庭份／約 600 公克',
          ),
          CartItem(
            id: 3,
            skuId: 203,
            count: 1,
            price: 500,
            title: '本季海鮮禮盒',
            purchasable: false,
          ),
        ],
      ),
    ];
    final selectedGroup = groups
        .where(
          (group) => group.items.any(
            (item) => _cartSelectedSkuIds.contains(item.skuId),
          ),
        )
        .firstOrNull;
    final productTotal = groups
        .expand((group) => group.items)
        .where(
          (item) =>
              item.purchasable && _cartSelectedSkuIds.contains(item.skuId),
        )
        .fold<double>(0, (total, item) => total + item.price * item.count);
    final shippingTotal = selectedGroup == null
        ? 0.0
        : (_shippingIndex == 0 ? 120.0 : 80.0);
    void recordAction() => setState(() => _taps++);
    void selectGroup(MerchantGroup group, bool selected) => setState(() {
      _cartSelectedSkuIds = selected
          ? group.items
                .where((item) => item.purchasable)
                .map((item) => item.skuId)
                .toSet()
          : {};
    });
    void selectItem(MerchantGroup group, int skuId, bool selected) =>
        setState(() {
          if (selected) {
            if (selectedGroup?.storeId != group.storeId) {
              _cartSelectedSkuIds.clear();
            }
            _cartSelectedSkuIds.add(skuId);
          } else {
            _cartSelectedSkuIds.remove(skuId);
          }
        });
    return _stack([
      CartTopBar(
        onBack: recordAction,
        onClear: recordAction,
        chatAction: IconButton(
          tooltip: '訊息',
          onPressed: recordAction,
          icon: const Icon(AppIcons.chatBubbleOutline, size: HkControl.icon),
        ),
      ),
      SegmentedButton<AppTextSize>(
        key: const ValueKey('catalog-cart-text-size'),
        showSelectedIcon: false,
        segments: [
          for (final size in AppTextSize.values)
            ButtonSegment(value: size, label: Text(size.label)),
        ],
        selected: {widget.textSize},
        onSelectionChanged: (selection) =>
            widget.onTextSizeChanged(selection.first),
        style: SegmentedButton.styleFrom(
          minimumSize: const Size(0, HkControl.minTap),
          textStyle: CartTypography.button,
        ),
      ),
      Text('固定預覽資料；操作只更新此預覽。', style: CartTypography.supportMuted),
      for (final group in groups)
        MerchantGroupCard(
          group: group,
          checkedSkuIds: _cartSelectedSkuIds,
          skuStock: const {101: 5, 202: 5, 203: 0},
          onGroupCheckChanged: selectGroup,
          onItemCheckChanged: (skuId, selected) =>
              selectItem(group, skuId, selected),
          onQuantityChanged: (item, value) => setState(() {
            if (item.skuId == 101) {
              _count = value;
            } else {
              _secondCartCount = value;
            }
          }),
          onDeleteItem: (_) => recordAction(),
          resolveImageProvider: (_) => null,
          sellerAvatarUrl: (_, {groupLogoUrl}) => null,
          onMerchantTap: recordAction,
        ),
      ShippingSection(
        shippingOptions: const [
          {
            'shippingMethod': '3001',
            'label': '黑貓宅急便（常溫）',
            'deliveryTemp': '3201',
            'fee': '120',
            'estimatedDelivery': '依訂單確認',
          },
          {
            'shippingMethod': '3002',
            'label': '7-11 超商取貨（常溫）',
            'deliveryTemp': '3201',
            'fee': '80',
            'estimatedDelivery': '依訂單確認',
          },
        ],
        selectedShippingIdx: _shippingIndex,
        validateResult: const {'valid': true, 'errors': []},
        validateLoading: false,
        selectedAddress: address,
        userAddresses: const [address],
        storePickups: const [
          {
            'id': 'demo-store',
            'storeName': '範例門市',
            'storeAddress': '台北市中正區範例路 1 號',
            'receiverName': '範例收件人',
            'receiverPhone': '0912345678',
          },
        ],
        selectedStorePickupId: 'demo-store',
        checkedSkuIds: _cartSelectedSkuIds,
        onShippingIdxChanged: (value) => setState(() => _shippingIndex = value),
        onStorePickupSelected: (_) => recordAction(),
        onExpandedGroupsChanged: (_) {},
        onShowAddressPicker: () => showAddressPickerSheet(
          context: context,
          userAddresses: const [address],
          selectedAddressId: 1,
          onAddAddress: recordAction,
        ),
        onOpenStorePickupForm: (_) => recordAction(),
        onAddAddress: recordAction,
        onRetryValidate: recordAction,
      ),
      DeliveryNotesSection(controller: _cartNotes),
      PaymentDetailsSection(
        productTotal: productTotal,
        shippingTotal: shippingTotal,
        couponDiscount: 0,
        grandTotal: productTotal + shippingTotal,
        canSelectCoupon: selectedGroup != null,
        onSelectCoupon: recordAction,
      ),
      CartCheckoutAction(
        onPressed: selectedGroup == null ? null : recordAction,
        storeName: selectedGroup?.sellerName,
        selectedItemCount: _cartSelectedSkuIds.length,
      ),
      Text('預覽操作：$_taps 次', key: const ValueKey('catalog-cart-result')),
    ], gap: HkSpace.sm);
  }

  Widget _example(String title, String source, Widget child) => Column(
    crossAxisAlignment: CrossAxisAlignment.stretch,
    mainAxisSize: MainAxisSize.min,
    children: [
      Text(title, style: AppTypography.sectionTitle),
      const SizedBox(height: HkSpace.xs),
      SelectableText(source, style: AppTypography.caption),
      const SizedBox(height: HkSpace.lg),
      child,
    ],
  );

  Widget _stack(List<Widget> children, {double gap = HkSpace.xxl}) => Column(
    crossAxisAlignment: CrossAxisAlignment.stretch,
    mainAxisSize: MainAxisSize.min,
    children: [
      for (var index = 0; index < children.length; index++) ...[
        if (index > 0) SizedBox(height: gap),
        children[index],
      ],
    ],
  );
}
