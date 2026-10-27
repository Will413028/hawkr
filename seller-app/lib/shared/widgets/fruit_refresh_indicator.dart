// dart:math 提供 Random，用於每次下拉隨機挑選水果
import 'dart:math';

import 'package:flutter/material.dart';
import '../icons/app_icons.dart';

// ─────────────────────────────────────────────────────────────
// 水果素材資料表
//
// Dart 的「record」語法：({ IconData icon, Color color }) 是一個
// 匿名具名欄位的 record 型別，可同時攜帶圖示與配套顏色。
//
// 顏色值格式：0x4D______
//   0x4D = 十六進位的 77，即 alpha ≈ 30%，讓圓形底色保持清透感。
// ─────────────────────────────────────────────────────────────

/// 20 種水果對應的圖示與主題色（30% 透明度圓形底色）
const _fruitData = <({IconData icon, Color color})>[
  (icon: AppIcons.ecoOutlined, color: Color(0x4DFF3B30)), // 鮮亮蘋果紅
  (icon: AppIcons.localFloristOutlined, color: Color(0x4DFF453A)), // 活力深紅
  (icon: AppIcons.spaOutlined, color: Color(0x4DFF5752)), // 柔和果紅
  (icon: AppIcons.grassOutlined, color: Color(0x4D34C759)), // 青蘋果綠
  (icon: AppIcons.yardOutlined, color: Color(0x4DFFCC00)), // 檸檬亮黃
  (icon: AppIcons.ecoOutlined, color: Color(0x4DFFD60A)), // 鮮榨鮮黃
  (icon: AppIcons.localFloristOutlined, color: Color(0x4DFFC000)), // 暖金黃
  (icon: AppIcons.spaOutlined, color: Color(0x4D30D158)), // 青檸綠
  (icon: AppIcons.grassOutlined, color: Color(0x4DFF9500)), // 蜜橘橙
  (icon: AppIcons.yardOutlined, color: Color(0x4DFF9F0A)), // 活力亮橙
  (icon: AppIcons.ecoOutlined, color: Color(0x4DFFAA33)), // 淺橘橙
  (icon: AppIcons.localFloristOutlined, color: Color(0x4DFFE433)), // 香蕉暖黃
  (icon: AppIcons.spaOutlined, color: Color(0x4DFFDE1A)), // 成熟焦黃
  (icon: AppIcons.grassOutlined, color: Color(0x4D66D464)), // 芭樂果綠
  (icon: AppIcons.yardOutlined, color: Color(0x4DFF4F64)), // 西瓜淺紅
  (icon: AppIcons.ecoOutlined, color: Color(0x4D28CD41)), // 西瓜皮綠
  (icon: AppIcons.localFloristOutlined, color: Color(0x4D5856D6)), // 藍莓靛藍
  (icon: AppIcons.spaOutlined, color: Color(0x4DA2D729)), // 酪梨淺綠
  (icon: AppIcons.grassOutlined, color: Color(0x4DFF2D55)), // 草莓鮮粉紅
  (icon: AppIcons.yardOutlined, color: Color(0x4DFF375F)), // 草莓玫紅
];

// ─────────────────────────────────────────────────────────────
// 佈局常數
// ─────────────────────────────────────────────────────────────

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

// ─────────────────────────────────────────────────────────────
// 刷新狀態機
//
// 六個狀態描述整個下拉刷新的生命週期：
//
//   idle       → 靜止，沒有任何下拉行為
//   dragging   → 使用者正在下拉，progress < 1.0（未達閾值）
//   triggered  → progress ≥ 1.0，已達閾值，等待使用者放手
//   refreshing → 使用者放手後，正在執行 onRefresh 回調
//   canceling  → 未達閾值就放手，水果正在以動畫回彈歸零
//   closing    → 刷新完成，水果正在以動畫收起歸零
// ─────────────────────────────────────────────────────────────
enum _RefreshPhase { idle, dragging, triggered, refreshing, canceling, closing }

// ─────────────────────────────────────────────────────────────
// FruitRefreshIndicator
//
// 自訂下拉刷新元件，以隨機水果 SVG 替代系統的 CircularProgressIndicator。
//
// 用法：
//   FruitRefreshIndicator(
//     onRefresh: () async { await loadData(); },
//     child: ListView(...),
//   )
// ─────────────────────────────────────────────────────────────

class FruitRefreshIndicator extends StatefulWidget {
  const FruitRefreshIndicator({
    super.key,
    required this.child, // 被包裹的可滾動元件（ListView、CustomScrollView 等）
    required this.onRefresh, // 刷新回調，必須回傳 Future<void>，完成後動畫才會收起
  });

  final Widget child;
  final Future<void> Function() onRefresh;

  @override
  State<FruitRefreshIndicator> createState() => _FruitRefreshIndicatorState();
}

class _FruitRefreshIndicatorState extends State<FruitRefreshIndicator>
        // TickerProviderStateMixin 允許同一個 State 同時持有多個 AnimationController
        // （若只有一個 controller 應使用更輕量的 SingleTickerProviderStateMixin）
        with
        TickerProviderStateMixin {
  // 亂數產生器，每次下拉從 _fruitData 隨機取一個水果
  final _random = Random();

  // 當前顯示的水果 SVG 路徑，初始值為資料表第一筆（蘋果）
  IconData _currentFruit = _fruitData[0].icon;

  // 當前水果對應的主題色，用於圓形底色和陰影
  Color _currentColor = _fruitData[0].color;

  // 當前刷新階段，驅動所有條件判斷
  _RefreshPhase _phase = _RefreshPhase.idle;

  // ── 動畫控制器 ─────────────────────────────────────────────
  //
  // Flutter 的 AnimationController 本身就是一個 0.0～1.0 的數值驅動器。
  // 搭配 vsync: this 讓動畫與螢幕刷新率同步，避免在後台浪費 CPU。

  // 下拉進度動畫（0.0 = 完全收起，1.0 = 達到閾值，最大可到 1+overpull/threshold）
  // upperBound 未指定，預設為 1.0；但我們透過直接賦值 value 突破上限，
  // 所以這裡不設定 duration（只做直接賦值，不使用 animateTo）
  late final AnimationController _pullAnim = AnimationController(vsync: this);

  // 水果自轉動畫：800ms 轉一圈，下拉開始就 repeat()，刷新完成後 stop()+reset()
  late final AnimationController _spinCtrl = AnimationController(
    vsync: this,
    duration: const Duration(milliseconds: 800),
  );

  // 回彈／收起動畫：400ms，搭配 easeOutCubic 曲線模擬橡皮筋彈回感
  // 用途：(1) 未達閾值放手時回彈到 0；(2) 刷新完成後收起到 0
  late final AnimationController _snapCtrl = AnimationController(
    vsync: this,
    duration: const Duration(milliseconds: 400),
  );

  @override
  void dispose() {
    // 元件從樹中移除時，必須手動釋放所有 AnimationController，
    // 否則 vsync ticker 會持續佔用資源並造成記憶體洩漏
    _pullAnim.dispose();
    _spinCtrl.dispose();
    _snapCtrl.dispose();
    super.dispose();
  }

  /// 從資料表隨機挑選一種水果，更新 [_currentFruit] 和 [_currentColor]。
  /// 每次使用者開始下拉時呼叫，確保每次刷新都有新驚喜。
  void _pickRandomFruit() {
    // nextInt(n) 回傳 [0, n) 的隨機整數，剛好對應資料表的合法索引
    final pick = _fruitData[_random.nextInt(_fruitData.length)];
    _currentFruit = pick.icon;
    _currentColor = pick.color;
  }

  // ── 通用回彈動畫 ────────────────────────────────────────────
  //
  // 將 _pullAnim 從當前值動畫至 target（通常為 0.0）。
  // 做法：建立一個 Tween 動畫，在每個影格把值「橋接」回 _pullAnim.value，
  // 再用 _snapCtrl 驅動整個過程。
  //
  // 為何不直接用 _pullAnim.animateTo()？
  //   因為 _pullAnim 沒有設定 duration，animateTo 需要 duration 才能運作。
  //   改用獨立的 _snapCtrl 管理時間，Tween 負責數值映射，兩者職責分離。

  /// 以 easeOutCubic 曲線在 400ms 內將下拉進度動畫至 [target]。
  /// 回傳 Future，可用 await 等待動畫完成。
  Future<void> _animatePullTo(double target) {
    final begin = _pullAnim.value; // 記錄動畫起始值（當前進度）
    _snapCtrl.reset(); // 將 _snapCtrl 歸零，確保從頭播放

    // Tween：定義數值從 begin 到 target 的線性插值範圍
    // CurvedAnimation：將 _snapCtrl 的線性進度套用 easeOutCubic 曲線
    //   easeOutCubic = 開頭快、末尾慢，模擬橡皮筋彈回的減速感
    final anim = Tween<double>(
      begin: begin,
      end: target,
    ).animate(CurvedAnimation(parent: _snapCtrl, curve: Curves.easeOutCubic));

    // 每個動畫影格都把 anim 的當前插值同步回 _pullAnim.value，
    // 這樣 AnimatedBuilder 監聽 _pullAnim 就能即時更新畫面
    anim.addListener(() => _pullAnim.value = anim.value);

    // forward() 開始播放 _snapCtrl（0.0 → 1.0），並回傳 TickerFuture
    // TickerFuture 實作了 Future，因此可直接 return 給呼叫方 await
    return _snapCtrl.forward();
  }

  // ── 刷新回調 ────────────────────────────────────────────────

  /// 由 RefreshIndicator 在使用者放手且 progress ≥ 1.0 時觸發。
  /// 執行順序：切換到 refreshing 態 → 呼叫 onRefresh → 動畫收起 → 回到 idle
  Future<void> _handleRefresh() async {
    // 切換到「刷新中」狀態
    setState(() => _phase = _RefreshPhase.refreshing);

    // 確保自轉動畫持續播放（dragging 階段已啟動，這裡作保底）
    if (!_spinCtrl.isAnimating) _spinCtrl.repeat();

    // 執行外部傳入的刷新邏輯（API 請求等），等待完成
    await widget.onRefresh();

    // onRefresh 完成後，元件可能已被銷毀（例如頁面已 pop），需提前返回
    if (!mounted) return;

    // 切換到「收起中」狀態，停止自轉
    setState(() => _phase = _RefreshPhase.closing);
    _spinCtrl.stop();

    // 以 easeOutCubic 動畫將進度從當前值收回到 0
    await _animatePullTo(0);

    // 動畫結束後將自轉重置到初始角度（避免下次出現時有殘留旋轉）
    _spinCtrl.reset();

    // 確認元件仍掛載後，才安全地呼叫 setState 回到 idle
    if (mounted) setState(() => _phase = _RefreshPhase.idle);
  }

  // ── 未達閾值松手：easeOutCubic 回彈 ─────────────────────────

  /// 使用者在 progress < 1.0 時放手，水果以動畫回彈歸零。
  /// 只在 dragging 階段有效，防止其他階段誤觸。
  void _startCanceling() {
    // 如果不是 dragging 態（例如已經 triggered 或正在 refreshing），直接忽略
    if (_phase != _RefreshPhase.dragging) return;

    setState(() => _phase = _RefreshPhase.canceling);

    // 動畫回彈到 0，完成後清理動畫狀態並回到 idle
    _animatePullTo(0).then((_) {
      _spinCtrl.stop(); // 停止自轉
      _spinCtrl.reset(); // 重置自轉角度
      if (mounted) setState(() => _phase = _RefreshPhase.idle);
    });
  }

  // ── ScrollNotification 處理 ─────────────────────────────────
  //
  // NotificationListener 會在子樹的任何 ScrollView 發生滾動事件時觸發。
  // 回傳 false 表示「不阻止事件繼續向上冒泡」，確保父層也能收到。

  /// 監聽滾動通知，將捲軸位移轉換為水果動畫進度。
  bool _handleScrollNotification(ScrollNotification n) {
    // ── 放手事件（手指離開螢幕）──
    if (n is ScrollEndNotification) {
      // 放手時若未達閾值，觸發回彈；若已達閾值則由 RefreshIndicator 接管
      _startCanceling();
      return false;
    }

    // 只處理滾動更新事件，其他通知（ScrollStartNotification 等）直接忽略
    if (n is! ScrollUpdateNotification) return false;

    // pixels > 0 表示向下捲動（正常捲動）；pixels < 0 表示向上超出頂部（下拉）
    final pixels = n.metrics.pixels;

    if (pixels < 0) {
      // ── 計算下拉進度 ──
      //
      // progress = 下拉距離 / 閾值
      //   = 0.0：尚未下拉
      //   = 1.0：恰好到達觸發閾值
      //   > 1.0：超過閾值（overpull 區間，最大到 maxProgress）
      //
      // maxProgress = 1.0 + _kOverpull / _kThreshold
      //   例如 threshold=80, overpull=50 → maxProgress = 1.625
      //   表示最多可拉到 80+50=130px，給使用者「拉滿了」的回饋
      final maxProgress = 1.0 + _kOverpull / _kThreshold;

      // -pixels：將負值轉正（下拉距離絕對值），再除以閾值得到進度比例
      // clamp：確保進度不超過允許的最大值
      final progress = (-pixels / _kThreshold).clamp(0.0, maxProgress);

      // dragging 和 triggered 狀態下，即時把滾動進度同步給動畫
      // （直接賦值而非動畫，讓水果跟手指移動完全同步）
      if (_phase == _RefreshPhase.dragging ||
          _phase == _RefreshPhase.triggered) {
        _pullAnim.value = progress;
      }

      // ── 狀態轉換邏輯 ──

      if (_phase == _RefreshPhase.idle && -pixels > 5) {
        // 從靜止進入下拉：超過 5px 才開始（避免微小抖動誤觸）
        setState(() {
          _phase = _RefreshPhase.dragging;
          _pickRandomFruit(); // 下拉開始時隨機換一個水果
        });
        _pullAnim.value = progress;
        _spinCtrl.repeat(); // 水果出現的同時開始自轉
      } else if (progress >= 1.0 && _phase == _RefreshPhase.dragging) {
        // 下拉超過閾值：切換到 triggered（等待放手觸發刷新）
        setState(() => _phase = _RefreshPhase.triggered);
      } else if (progress < 1.0 && _phase == _RefreshPhase.triggered) {
        // 已 triggered 但使用者又往回拉（縮回閾值以內）：退回 dragging
        setState(() => _phase = _RefreshPhase.dragging);
      }
    }

    return false;
  }

  // ── Build ───────────────────────────────────────────────────

  @override
  Widget build(BuildContext context) {
    // NotificationListener：攔截子樹的 ScrollNotification
    // 不使用 GestureDetector，因為後者會與 ScrollView 的手勢競爭
    return NotificationListener<ScrollNotification>(
      onNotification: _handleScrollNotification,
      child: Stack(
        // Stack 讓水果浮層疊在內容上方，無需調整內容的 padding
        children: [
          // ① 內容區：隨下拉向下位移，為水果騰出顯示空間
          //
          // AnimatedBuilder 只監聽 _pullAnim，只重建自身（content 子樹不重建），
          // 比直接在 build() 裡讀取 _pullAnim.value 更高效。
          AnimatedBuilder(
            animation: _pullAnim,
            builder: (_, child) => Transform.translate(
              offset: Offset(
                0,
                // progress 超過 1.0（overpull 段）不再繼續位移，
                // 只在 0.0 ～ 1.0 範圍內線性放大位移量，最大 _kContentShift px
                _pullAnim.value.clamp(0.0, 1.0) * _kContentShift,
              ),
              child: child,
            ),
            // child 放在 AnimatedBuilder 外層，避免每個影格重新 build
            child: RefreshIndicator(
              onRefresh: _handleRefresh,
              // displacement：原生 loading 圓圈距頂部的距離，設大讓它隱藏到可視區外
              displacement: 60,
              // edgeOffset：負值讓原生指示器更早觸發，但視覺上已被水果取代
              edgeOffset: -200,
              // 以下三個屬性讓原生 RefreshIndicator 視覺上完全透明（隱藏原生圓圈）
              color: Colors.transparent,
              backgroundColor: Colors.transparent,
              strokeWidth: 0,
              child: widget.child,
            ),
          ),

          // ② 水果浮層：同時監聽 _pullAnim 和 _spinCtrl，任一變化都重建
          //
          // Listenable.merge 合併多個 Listenable，讓 AnimatedBuilder 同時響應兩者
          AnimatedBuilder(
            animation: Listenable.merge([_pullAnim, _spinCtrl]),
            builder: (_, __) {
              // progress 為 0 時（完全收起）不渲染任何東西，節省繪製成本
              if (_pullAnim.value == 0) return const SizedBox.shrink();
              return _buildFruit(_pullAnim.value);
            },
          ),
        ],
      ),
    );
  }

  // ── 水果 Widget ─────────────────────────────────────────────

  /// 根據當前下拉進度 [progress] 建構水果圓形圖示。
  Widget _buildFruit(double progress) {
    // ── 計算視覺參數 ──

    // overpull 段的最大進度上限（與 _handleScrollNotification 保持一致）
    final maxProgress = 1.0 + _kOverpull / _kThreshold;

    // clamp 防止 progress 超出合法範圍（理論上不應發生，但作為防禦）
    final clampedProgress = progress.clamp(0.0, maxProgress);

    // 將進度反算回像素距離，用來計算水果的垂直位置
    final pulledPx = clampedProgress * _kThreshold;

    // scale：進度 0→1 時水果從無到全尺寸縮放出現；超過 1.0（overpull）保持全尺寸
    final scale = clampedProgress.clamp(0.0, 1.0);

    // opacity：與 scale 同步，0→1 逐漸淡入；overpull 段保持完全不透明
    final opacity = clampedProgress.clamp(0.0, 1.0);

    // ── 計算垂直位置 ──
    //
    // 目標：水果垂直居中於「被位移騰出的空間」裡
    //
    // 被騰出的空間高度 ≈ pulledPx（內容往下移了多少，頂部就空出多少）
    // 空間中心 = pulledPx / 2
    // 水果圓心需對齊空間中心，所以 top = 空間中心 - 圓形半徑
    final topPos = pulledPx / 2 - _kCircleSize / 2;

    // ── 組裝 Widget 樹 ──

    return Positioned(
      top: topPos, // 動態計算的垂直位置
      left: 0, // 水平方向撐滿，配合內部 Center 實現水平居中
      right: 0,
      child: Center(
        child: Opacity(
          opacity: opacity, // 淡入效果
          child: Transform.scale(
            scale: scale, // 縮放出現效果
            child: Container(
              width: _kCircleSize,
              height: _kCircleSize,
              decoration: BoxDecoration(
                color: _currentColor, // 水果主題色（30% 透明度）作為底色
                shape: BoxShape.circle, // 圓形
                boxShadow: [
                  BoxShadow(
                    // 陰影顏色取自同一主題色，但調高 alpha 至 55%，增加立體感
                    color: _currentColor.withValues(alpha: 0.55),
                    blurRadius: 14, // 模糊半徑，值越大陰影越柔和
                    spreadRadius: 2, // 擴散半徑，讓陰影比容器略大一圈
                  ),
                ],
              ),
              child: Center(
                child: RotationTransition(
                  // turns：_spinCtrl 的值每增加 1.0 就旋轉一整圈（360°）
                  // repeat() 讓它持續從 0→1→0→1... 無限循環
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
