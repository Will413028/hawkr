import 'package:hawkr_api/gen/hawkr/imageai/v1alpha1/imageai.pb.dart' as pb;
import 'package:hawkr_api/images.dart';

/// `ListAiStyles` 的一筆風格。縮圖是伺服器給的 `Image`（uri_template），顯示端取 [ImagePreset.thumb] 的網址。
class AiStyle {
  const AiStyle({
    required this.id,
    required this.name,
    required this.label,
    required this.prompt,
    this.thumbnailUrl = '',
  });

  factory AiStyle.fromProto(pb.AiStyle s) => AiStyle(
    id: s.id,
    name: s.name,
    label: s.label,
    prompt: s.prompt,
    thumbnailUrl: s.hasThumbnail() ? s.thumbnail.uri(ImagePreset.thumb) : '',
  );

  final String id;
  final String name;
  final String label;
  final String prompt;
  final String thumbnailUrl;
}

/// `ListAiStyles` 的回應：風格（依顯示順序）、沒選風格時的提示詞與本週用量。
class AiStyles {
  const AiStyles({
    this.styles = const [],
    this.defaultPrompt = '',
    this.weeklyUsage = 0,
    this.weeklyLimit = 0,
  });

  final List<AiStyle> styles;
  final String defaultPrompt;
  final int weeklyUsage;
  final int weeklyLimit;
}
