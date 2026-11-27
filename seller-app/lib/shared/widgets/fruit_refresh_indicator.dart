import 'dart:math';

import 'package:flutter/material.dart';
import '../icons/app_icons.dart';


/// 20 種水果對應的圖示與主題色（30% 透明度圓形底色）
const _fruitData = <({IconData icon, Color color})>[
  (icon: AppIcons.ecoOutlined, color: Color(0x4DFF3B30)),
  (icon: AppIcons.localFloristOutlined, color: Color(0x4DFF453A)),
  (icon: AppIcons.spaOutlined, color: Color(0x4DFF5752)),
  (icon: AppIcons.grassOutlined, color: Color(0x4D34C759)),
  (icon: AppIcons.yardOutlined, color: Color(0x4DFFCC00)),
  (icon: AppIcons.ecoOutlined, color: Color(0x4DFFD60A)),
  (icon: AppIcons.localFloristOutlined, color: Color(0x4DFFC000)),
  (icon: AppIcons.spaOutlined, color: Color(0x4D30D158)),
  (icon: AppIcons.grassOutlined, color: Color(0x4DFF9500)),
  (icon: AppIcons.yardOutlined, color: Color(0x4DFF9F0A)),
  (icon: AppIcons.ecoOutlined, color: Color(0x4DFFAA33)),
  (icon: AppIcons.localFloristOutlined, color: Color(0x4DFFE433)),
  (icon: AppIcons.spaOutlined, color: Color(0x4DFFDE1A)),
  (icon: AppIcons.grassOutlined, color: Color(0x4D66D464)),
  (icon: AppIcons.yardOutlined, color: Color(0x4DFF4F64)),
  (icon: AppIcons.ecoOutlined, color: Color(0x4D28CD41)),
  (icon: AppIcons.localFloristOutlined, color: Color(0x4D5856D6)),
  (icon: AppIcons.spaOutlined, color: Color(0x4DA2D729)),
  (icon: AppIcons.grassOutlined, color: Color(0x4DFF2D55)),
  (icon: AppIcons.yardOutlined, color: Color(0x4DFF375F)),
];


/// 水果圓形容器的直徑（px）
const _kCircleSize = 45.0;

/// 水果 SVG 圖示的寬高（px），比圓容器略小以留出內邊距
const _kFruitSize = 32.0;

/// 觸發刷新所需的最小下拉距離（px）；progress = pixels / _kThreshold
const _kThreshold = 60.0;

/// 下拉時整個內容區塊最多向下位移的距離（px），為水果騰出顯示空間
const _kContentShift = 60.0;

/// 超過閾值後仍可繼續下拉的額外距離（px），提供「拉滿」的視覺彈性回饋
const _kOverpull = 45.0;

enum _RefreshPhase { idle, dragging, triggered, refreshing, canceling, closing }


class FruitRefreshIndicator extends StatefulWidget {
  const FruitRefreshIndicator({
    super.key,
    required this.child,
    required this.onRefresh,
  });

  final Widget child;
  final Future<void> Function() onRefresh;

  @override
  State<FruitRefreshIndicator> createState() => _FruitRefreshIndicatorState();
}

class _FruitRefreshIndicatorState extends State<FruitRefreshIndicator>
        with
        TickerProviderStateMixin {
  final _random = Random();

  IconData _currentFruit = _fruitData[0].icon;

  Color _currentColor = _fruitData[0].color;

  _RefreshPhase _phase = _RefreshPhase.idle;


  late final AnimationController _pullAnim = AnimationController(vsync: this);

  late final AnimationController _spinCtrl = AnimationController(
    vsync: this,
    duration: const Duration(milliseconds: 800),
  );

  late final AnimationController _snapCtrl = AnimationController(
    vsync: this,
    duration: const Duration(milliseconds: 400),
  );

  @override
  void dispose() {
    _pullAnim.dispose();
    _spinCtrl.dispose();
    _snapCtrl.dispose();
    super.dispose();
  }

  /// 從資料表隨機挑選一種水果，更新 [_currentFruit] 和 [_currentColor]。
  /// 每次使用者開始下拉時呼叫，確保每次刷新都有新驚喜。
  void _pickRandomFruit() {
    final pick = _fruitData[_random.nextInt(_fruitData.length)];
    _currentFruit = pick.icon;
    _currentColor = pick.color;
  }


  /// 以 easeOutCubic 曲線在 400ms 內將下拉進度動畫至 [target]。
  /// 回傳 Future，可用 await 等待動畫完成。
  Future<void> _animatePullTo(double target) {
    final begin = _pullAnim.value;
    _snapCtrl.reset();

    final anim = Tween<double>(
      begin: begin,
      end: target,
    ).animate(CurvedAnimation(parent: _snapCtrl, curve: Curves.easeOutCubic));

    anim.addListener(() => _pullAnim.value = anim.value);

    return _snapCtrl.forward();
  }


  /// 由 RefreshIndicator 在使用者放手且 progress ≥ 1.0 時觸發。
  /// 執行順序：切換到 refreshing 態 → 呼叫 onRefresh → 動畫收起 → 回到 idle
  Future<void> _handleRefresh() async {
    setState(() => _phase = _RefreshPhase.refreshing);

    if (!_spinCtrl.isAnimating) _spinCtrl.repeat();

    await widget.onRefresh();

    if (!mounted) return;

    setState(() => _phase = _RefreshPhase.closing);
    _spinCtrl.stop();

    await _animatePullTo(0);

    _spinCtrl.reset();

    if (mounted) setState(() => _phase = _RefreshPhase.idle);
  }


  /// 使用者在 progress < 1.0 時放手，水果以動畫回彈歸零。
  /// 只在 dragging 階段有效，防止其他階段誤觸。
  void _startCanceling() {
    if (_phase != _RefreshPhase.dragging) return;

    setState(() => _phase = _RefreshPhase.canceling);

    _animatePullTo(0).then((_) {
      _spinCtrl.stop();
      _spinCtrl.reset();
      if (mounted) setState(() => _phase = _RefreshPhase.idle);
    });
  }


  /// 監聽滾動通知，將捲軸位移轉換為水果動畫進度。
  bool _handleScrollNotification(ScrollNotification n) {
    if (n is ScrollEndNotification) {
      _startCanceling();
      return false;
    }

    if (n is! ScrollUpdateNotification) return false;

    final pixels = n.metrics.pixels;

    if (pixels < 0) {
      final maxProgress = 1.0 + _kOverpull / _kThreshold;

      final progress = (-pixels / _kThreshold).clamp(0.0, maxProgress);

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
        _spinCtrl.repeat();
      } else if (progress >= 1.0 && _phase == _RefreshPhase.dragging) {
        setState(() => _phase = _RefreshPhase.triggered);
      } else if (progress < 1.0 && _phase == _RefreshPhase.triggered) {
        setState(() => _phase = _RefreshPhase.dragging);
      }
    }

    return false;
  }


  @override
  Widget build(BuildContext context) {
    return NotificationListener<ScrollNotification>(
      onNotification: _handleScrollNotification,
      child: Stack(
        children: [
          AnimatedBuilder(
            animation: _pullAnim,
            builder: (_, child) => Transform.translate(
              offset: Offset(
                0,
                _pullAnim.value.clamp(0.0, 1.0) * _kContentShift,
              ),
              child: child,
            ),
            child: RefreshIndicator(
              onRefresh: _handleRefresh,
              displacement: 60,
              edgeOffset: -200,
              color: Colors.transparent,
              backgroundColor: Colors.transparent,
              strokeWidth: 0,
              child: widget.child,
            ),
          ),

          AnimatedBuilder(
            animation: Listenable.merge([_pullAnim, _spinCtrl]),
            builder: (_, _) {
              if (_pullAnim.value == 0) return const SizedBox.shrink();
              return _buildFruit(_pullAnim.value);
            },
          ),
        ],
      ),
    );
  }


  /// 根據當前下拉進度 [progress] 建構水果圓形圖示。
  Widget _buildFruit(double progress) {

    final maxProgress = 1.0 + _kOverpull / _kThreshold;

    final clampedProgress = progress.clamp(0.0, maxProgress);

    final pulledPx = clampedProgress * _kThreshold;

    final scale = clampedProgress.clamp(0.0, 1.0);

    final opacity = clampedProgress.clamp(0.0, 1.0);

    final topPos = pulledPx / 2 - _kCircleSize / 2;


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
                  child: ExcludeSemantics(
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
      ),
    );
  }
}
