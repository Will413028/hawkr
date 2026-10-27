import 'dart:io';

/// `/photo-editor` route 的 `extra` 契約。category 決定 AI 分頁載入商品或
/// 貼文的預設風格清單（見 image_editor_screen.dart / image_editor_ai_tab.dart）。
class PhotoEditorArgs {
  const PhotoEditorArgs({required this.file, required this.category});

  final File file;

  /// 'product' | 'post'
  final String category;
}
