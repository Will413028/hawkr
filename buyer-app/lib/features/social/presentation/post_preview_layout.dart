/// 貼文預覽與貼文詳情共用的固定 logical-pixel 幾何。
///
/// 這些值刻意不使用螢幕比例縮放，避免不同入口、裝置高度或 SafeArea
/// 讓預覽卡片與卡片內建互動列的間距漂移。
abstract final class PostPreviewLayout {
  static const double previewHorizontalInset = 8;
  static const double previewTopInset = 8;
  static const double previewToActionBarGap = 0;

  static const double panelHorizontalInset = 0;
  static const double panelBottomInset = 0;

  static const double actionBarHorizontalInset = previewHorizontalInset;
  static const double actionBarBottomInset = 12;
  static const double actionBarHeight = 64;
}
