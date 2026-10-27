/// 更新請求的 `update_mask`。
library;

import 'package:protobuf/protobuf.dart';
import 'package:protobuf/well_known_types/google/protobuf/field_mask.pb.dart';

/// 建立 [resource]（被更新的資源訊息，例如 `Profile`）的 [FieldMask]。
///
/// [paths] 是 proto 的 snake_case 欄位名。依 API 設計規範（Connect）「更新請求的 update_mask」：路徑只支援最上層的欄位
/// （不支援 `action.url`），唯一的例外是下到 `oneof` 的子訊息（`individual.id_card_no`）；其他子訊息整個換掉，
/// 路徑停在那個欄位，不能再往下。不認得的路徑，或在非 oneof 成員的欄位後面再接子路徑，丟出 [ArgumentError]。
FieldMask fieldMask(GeneratedMessage resource, Iterable<String> paths) {
  final mask = FieldMask();
  for (final path in paths) {
    _check(resource.info_, path);
    mask.paths.add(path);
  }
  return mask;
}

void _check(BuilderInfo root, String path) {
  var info = root;
  final segments = path.split('.');
  for (var i = 0; i < segments.length; i++) {
    final field = info.fieldInfo.values
        .where((f) => f.protoName == segments[i])
        .firstOrNull;
    if (field == null) {
      throw ArgumentError.value(
        path,
        'paths',
        '欄位 ${segments[i]} 不存在於 ${root.qualifiedMessageName}',
      );
    }
    if (i == segments.length - 1) return;
    final next = field.subBuilder;
    if (!field.isGroupOrMessage || field.isRepeated || next == null) {
      throw ArgumentError.value(path, 'paths', '${segments[i]} 不是訊息欄位');
    }
    if (!info.oneofs.containsKey(field.tagNumber)) {
      throw ArgumentError.value(
        path,
        'paths',
        '${segments[i]} 不是 oneof 的子訊息，路徑只能停在它',
      );
    }
    info = next().info_;
  }
}
