/// 商品詳情單一卡面的固定 logical-pixel 幾何。
///
/// 圖片、內容與固定購買列都收在同一張卡內，避免 SafeArea、裝置高度或
/// ScreenUtil 讓外框位置漂移。
abstract final class ProductDetailPreviewLayout {
  static const double previewHorizontalInset = 8;
  static const double previewTopInset = 8;
  static const double purchaseBottomInset = 12;
}
