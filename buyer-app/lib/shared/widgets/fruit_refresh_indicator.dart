import 'dart:math';

import 'package:flutter/material.dart';
import 'package:flutter_screenutil/flutter_screenutil.dart';
import 'package:hawkr_buyer/shared/icons/app_icons.dart';
import 'package:hawkr_buyer/shared/widgets/hk_brand_loader.dart';

/// 6 種葉形圖示 + 20 組活力主色調（30% 透明度作為清透的圓形底色）
const _fruitIcons = <IconData>[
  AppIcons.ecoOutlined,
  AppIcons.spaOutlined,
  AppIcons.localFloristOutlined,
  AppIcons.grassOutlined,
  AppIcons.yardOutlined,
  AppIcons.energySavingsLeafOutlined,
];

const _fruitColors = <Color>[
  Color(0x4DFF3B30), // 鮮亮苹果紅
  Color(0x4DFF453A), // 活力深紅
  Color(0x4DFF5752), // 柔和果紅
  Color(0x4D34C759), // 青苹果綠
  Color(0x4DFFCC00), // 柠檬亮黃
  Color(0x4DFFD60A), // 鲜榨鲜黃
  Color(0x4DFFC000), // 暖金黃
  Color(0x4D30D158), // 青柠绿
  Color(0x4DFF9500), // 蜜橘橙
  Color(0x4DFF9F0A), // 活力亮橙
  Color(0x4DFFAA33), // 浅橘橙
  Color(0x4DFFE433), // 香蕉暖黄
  Color(0x4DFFDE1A), // 成熟焦黄
  Color(0x4D66D464), // 芭乐果绿
  Color(0x4DFF4F64), // 西瓜浅红
  Color(0x4D28CD41), // 西瓜皮绿
  Color(0x4D5856D6), // 蓝莓靛蓝
  Color(0x4DA2D729), // 牛油果浅绿
  Color(0x4DFF2D55), // 草莓鲜粉红
  Color(0x4DFF375F), // 草莓玫红
];

const _kCircleSize = 48.0;
const _kFruitSize = 32.0;
const _kThreshold = 80.0;

enum _RefreshPhase { idle, dragging, triggered, refreshing, canceling, closing }

class FruitRefreshIndicator extends StatefulWidget {
  const FruitRefreshIndicator({
    super.key,
    required this.child,
    required this.onRefresh,
    this.topOffset = 0,
  });

  final Widget child;
  final Future<void> Function() onRefresh;

  /// 水果動畫的額外頂部偏移（px）。
  /// 在 NestedScrollView 中使用時傳入 outerHandle.layoutExtent，
  /// 使水果出現在可視內容區而非 header 後方。
  final double topOffset;

  @override
  State<FruitRefreshIndicator> createState() => _FruitRefreshIndicatorState();
}

class _FruitRefreshIndicatorState extends State<FruitRefreshIndicator>
    with TickerProviderStateMixin {
  final _random = Random();
  IconData _currentFruit = _fruitIcons[0];
  Color _currentColor = _fruitColors[0];

  _RefreshPhase _phase = _RefreshPhase.idle;

  // ── 動畫控制器 ─────────────────────────────────────────────

  // 手勢進度 0→1，直接賦值同步手勢，松手後 animateTo() 緩動
  late final AnimationController _pullAnim = AnimationController(vsync: this);

  // 順時針自轉（出現時持續）
  late final AnimationController _spinCtrl = AnimationController(
    vsync: this,
    duration: const Duration(milliseconds: 800),
  );

  @override
  void dispose() {
    _pullAnim.dispose();
    _spinCtrl.dispose();
    super.dispose();
  }

  void _pickRandomFruit() {
    _currentFruit = _fruitIcons[_random.nextInt(_fruitIcons.length)];
    _currentColor = _fruitColors[_random.nextInt(_fruitColors.length)];
  }

  // ── 刷新回調 ────────────────────────────────────────────────

  Future<void> _handleRefresh() async {
    setState(() => _phase = _RefreshPhase.refreshing);
    if (!_spinCtrl.isAnimating) _spinCtrl.repeat();

    await widget.onRefresh();

    if (!mounted) return;
    setState(() => _phase = _RefreshPhase.closing);
    _spinCtrl.stop();
    await _pullAnim.animateTo(
      0,
      duration: const Duration(milliseconds: 250),
      curve: Curves.easeInOut,
    );
    _spinCtrl.reset();
    if (mounted) setState(() => _phase = _RefreshPhase.idle);
  }

  // ── 未達閾值松手：立即開始收起 ─────────────────────────────

  void _startCanceling() {
    if (_phase != _RefreshPhase.dragging) return;
    setState(() => _phase = _RefreshPhase.canceling);
    _pullAnim
        .animateTo(
          0,
          duration: const Duration(milliseconds: 200),
          curve: Curves.easeInOut,
        )
        .then((_) {
          _spinCtrl.stop();
          _spinCtrl.reset();
          if (mounted) setState(() => _phase = _RefreshPhase.idle);
        });
  }

  // ── ScrollNotification 處理 ─────────────────────────────────

  bool _handleScrollNotification(ScrollNotification n) {
    if (n is ScrollEndNotification) {
      _startCanceling();
      return false;
    }

    if (n is! ScrollUpdateNotification) return false;

    final pixels = n.metrics.pixels;

    if (pixels < 0) {
      final progress = (-pixels / _kThreshold).clamp(0.0, 1.0);

      if (_phase == _RefreshPhase.dragging ||
          _phase == _RefreshPhase.triggered) {
        _pullAnim.value = progress;
      }

      if (_phase == _RefreshPhase.idle && -pixels > 5) {
        setState(() {
          _phase = _RefreshPhase.dragging;
          _pickRandomFruit();
        });
        _pullAnim.value = progress;
        _spinCtrl.repeat(); // 下拉出现时就开始顺时针自转
      } else if (progress >= 1.0 && _phase == _RefreshPhase.dragging) {
        setState(() => _phase = _RefreshPhase.triggered);
      } else if (progress < 1.0 && _phase == _RefreshPhase.triggered) {
        setState(() => _phase = _RefreshPhase.dragging);
      }
    }

    return false;
  }

  // ── Build ───────────────────────────────────────────────────

  @override
  Widget build(BuildContext context) {
    return NotificationListener<ScrollNotification>(
      onNotification: _handleScrollNotification,
      child: Stack(
        children: [
          RefreshIndicator(
            onRefresh: _handleRefresh,
            displacement: 60,
            edgeOffset: -200,
            color: Colors.transparent,
            backgroundColor: Colors.transparent,
            strokeWidth: 0,
            child: widget.child,
          ),
          // AnimatedBuilder 只重建水果 Widget，不觸發整棵樹 rebuild
          AnimatedBuilder(
            animation: Listenable.merge([_pullAnim, _spinCtrl]),
            builder: (_, __) {
              if (_pullAnim.value == 0) {
                return const SizedBox.shrink();
              }
              return _buildFruit(_pullAnim.value);
            },
          ),
        ],
      ),
    );
  }

  // ── 水果 Widget ─────────────────────────────────────────────

  Widget _buildFruit(double progress) {
    // 從小到大漸變出现
    final scale = progress.clamp(0.0, 1.0);
    final opacity = progress.clamp(0.0, 1.0);

    // 计算跟随下拉空白区域的相对位置
    final pulledDistance = progress * _kThreshold;
    // 基础位置：拉满时 (80px) → 中心在 40px，向上偏 12px → 距顶 4px
    // 加上 topOffset 使水果落在可视内容区（NestedScrollView 中 header 之下）
    final topPos = widget.topOffset + (pulledDistance - _kCircleSize) / 2 - 12;

    return Positioned(
      top: topPos,
      left: 0,
      right: 0,
      child: Center(
        child: Opacity(
          opacity: opacity,
          child: Transform.scale(
            scale: scale,
            child: Container(
              width: _kCircleSize,
              height: _kCircleSize,
              decoration: BoxDecoration(
                color: _currentColor,
                shape: BoxShape.circle,
                // 同色系光暈：讓圓形在白色背景上更有立體感
                boxShadow: [
                  BoxShadow(
                    color: _currentColor.withValues(alpha: 0.55),
                    blurRadius: 14,
                    spreadRadius: 2,
                  ),
                ],
              ),
              child: Center(
                child: RotationTransition(
                  turns: _spinCtrl,
                  child: Icon(
                    _currentFruit,
                    size: _kFruitSize,
                    color: _currentColor.withValues(alpha: 1),
                  ),
                ),
              ),
            ),
          ),
        ),
      ),
    );
  }
}

// ─────────────────────────────────────────────────────────────
// FruitRefreshListView
//
// 封裝「下拉刷新 + 觸底加載更多 + ListView」三層組合，
// 避免各頁面重複手寫 FruitRefreshIndicator > NotificationListener > ListView.builder。
//
// 基本用法（純刷新，無分頁）：
//   FruitRefreshListView(
//     itemCount: items.length,
//     itemBuilder: (_, i) => ItemCard(item: items[i]),
//     onRefresh: controller.refresh,
//   )
//
// 帶分頁：
//   FruitRefreshListView(
//     itemCount: items.length,
//     itemBuilder: (_, i) => ItemCard(item: items[i]),
//     onRefresh: controller.refresh,
//     onLoadMore: controller.loadMore,
//     hasMore: controller.hasMore,
//   )
// ─────────────────────────────────────────────────────────────

class FruitRefreshListView extends StatelessWidget {
  const FruitRefreshListView({
    super.key,
    required this.itemCount,
    required this.itemBuilder,
    required this.onRefresh,
    this.onLoadMore,
    this.hasMore = false,
    this.padding,
    this.loadMoreWidget,
    this.scrollController,
  });

  /// 列表項數量（不含 load-more 指示器）
  final int itemCount;

  /// 列表項 Builder
  final IndexedWidgetBuilder itemBuilder;

  /// 下拉刷新回調
  final Future<void> Function() onRefresh;

  /// 觸底加載更多回調（為 null 時禁用分頁）
  final VoidCallback? onLoadMore;

  /// 是否還有更多數據，控制是否顯示 load-more 指示器
  final bool hasMore;

  /// ListView 的 padding，默認上下左右各 0
  final EdgeInsetsGeometry? padding;

  /// 自定義 load-more 指示器，預設使用品牌 HkBrandLoader
  final Widget? loadMoreWidget;

  /// 外部傳入的 ScrollController（需要監聽滾動位置時使用）
  final ScrollController? scrollController;

  static const double _loadMoreThreshold = 200;

  @override
  Widget build(BuildContext context) {
    return FruitRefreshIndicator(
      onRefresh: onRefresh,
      child: NotificationListener<ScrollNotification>(
        onNotification: (scroll) {
          if (onLoadMore != null &&
              hasMore &&
              scroll is ScrollEndNotification &&
              scroll.metrics.extentAfter < _loadMoreThreshold) {
            onLoadMore!();
          }
          return false;
        },
        child: ListView.builder(
          controller: scrollController,
          padding: padding ?? EdgeInsets.zero,
          itemCount: itemCount + (hasMore && onLoadMore != null ? 1 : 0),
          itemBuilder: (context, index) {
            if (index >= itemCount) {
              return Padding(
                padding: EdgeInsets.all(16.w),
                child:
                    loadMoreWidget ??
                    const Center(
                      child: HkBrandLoader(size: HkBrandLoader.inline),
                    ),
              );
            }
            return itemBuilder(context, index);
          },
        ),
      ),
    );
  }
}
