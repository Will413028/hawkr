import 'dart:io';
import 'dart:typed_data';

import 'package:flutter/material.dart';
import 'package:image_picker/image_picker.dart';

import '../../../../core/theme/app_colors.dart';
import '../../../../core/theme/hk_tokens.dart';
import '../../../../shared/icons/app_icon_colors.dart';
import '../../../../shared/utils/floating_toast.dart';
import '../../../../shared/utils/hk_log.dart';
import '../../../../shared/widgets/hk_icon.dart';

const evidenceUploadFailedNotice = '上傳失敗，請再試一次';
const _pickFailedNotice = '選取圖片失敗';

/// 選好的一張佐證圖片（位元組與檔名）。
typedef PickedEvidence = ({Uint8List bytes, String filename});

/// 從相機或相簿選一張佐證圖片；取消時回 null。
typedef EvidencePicker = Future<PickedEvidence?> Function(BuildContext context);

class EvidencePhoto {
  EvidencePhoto(this.id, this.bytes);

  final int id;
  final Uint8List bytes;

  /// 上傳完成才有；沒有 token 也沒有 [error] 的照片還在上傳。
  String? token;

  /// 伺服器指出這張的 token 不能用（過期或被拒）的說明；有值表示要移除後重新新增。
  String? error;
}

Future<PickedEvidence?> pickEvidenceFromDevice(BuildContext context) async {
  final source = await showModalBottomSheet<ImageSource>(
    context: context,
    builder: (sheetContext) => SafeArea(
      child: Column(
        mainAxisSize: MainAxisSize.min,
        children: [
          ListTile(
            leading: const HkIcon(HkIconName.photoAdd),
            title: const Text('拍照'),
            onTap: () => Navigator.pop(sheetContext, ImageSource.camera),
          ),
          ListTile(
            leading: const HkIcon(HkIconName.photoAdd),
            title: const Text('從相簿選擇'),
            onTap: () => Navigator.pop(sheetContext, ImageSource.gallery),
          ),
          ListTile(
            leading: const HkIcon(HkIconName.close),
            title: const Text('取消'),
            onTap: () => Navigator.pop(sheetContext),
          ),
        ],
      ),
    ),
  );
  if (source == null) return null;
  try {
    final image = await ImagePicker().pickImage(
      source: source,
      imageQuality: 80,
    );
    if (image == null) return null;
    final bytes = await File(image.path).readAsBytes();
    return (
      bytes: bytes,
      filename: 'appeal_${DateTime.now().millisecondsSinceEpoch}.jpg',
    );
  } catch (error) {
    HKLog.logW('[EvidencePicker] pick image failed: $error');
    if (context.mounted) FloatingToast.error(context, _pickFailedNotice);
    return null;
  }
}

class EvidencePhotoTile extends StatelessWidget {
  const EvidencePhotoTile({
    super.key,
    required this.photo,
    required this.onRemove,
  });

  final EvidencePhoto photo;
  final VoidCallback? onRemove;

  /// 縮圖的邊長（沒有對應的共用尺寸，只有這個表單用）。
  static const _size = 72.0;
  static const _spinner = 20.0;
  static const _removeIcon = 14.0;

  @override
  Widget build(BuildContext context) {
    final failed = photo.error != null;
    final uploading = photo.token == null && !failed;
    return SizedBox.square(
      dimension: _size,
      child: Stack(
        fit: StackFit.expand,
        children: [
          ClipRRect(
            borderRadius: BorderRadius.circular(HkRadius.sm),
            child: Image.memory(photo.bytes, fit: BoxFit.cover),
          ),
          if (uploading)
            ColoredBox(
              color: AppColors.textEmphasis.withValues(alpha: 0.4),
              child: const Center(
                child: SizedBox.square(
                  dimension: _spinner,
                  child: CircularProgressIndicator(strokeWidth: 2),
                ),
              ),
            ),
          if (failed)
            Semantics(
              label: photo.error,
              child: DecoratedBox(
                decoration: BoxDecoration(
                  color: AppColors.danger.withValues(alpha: 0.35),
                  borderRadius: BorderRadius.circular(HkRadius.sm),
                  border: Border.all(color: AppColors.danger),
                ),
              ),
            ),
          if (onRemove != null)
            Positioned(
              top: 0,
              right: 0,
              child: Semantics(
                button: true,
                label: '移除照片',
                child: InkWell(
                  onTap: onRemove,
                  child: const Padding(
                    padding: EdgeInsets.all(HkSpace.xs),
                    child: DecoratedBox(
                      decoration: BoxDecoration(
                        color: AppColors.textEmphasis,
                        shape: BoxShape.circle,
                      ),
                      child: Padding(
                        padding: EdgeInsets.all(HkSpace.xs / 2),
                        child: HkIcon(
                          HkIconName.close,
                          size: _removeIcon,
                          color: AppIconColors.onBrand,
                        ),
                      ),
                    ),
                  ),
                ),
              ),
            ),
        ],
      ),
    );
  }
}

class AddEvidencePhotoTile extends StatelessWidget {
  const AddEvidencePhotoTile({super.key, required this.onTap});

  final VoidCallback? onTap;

  @override
  Widget build(BuildContext context) {
    return Semantics(
      button: true,
      label: '新增照片',
      child: InkWell(
        onTap: onTap,
        borderRadius: BorderRadius.circular(HkRadius.sm),
        child: Container(
          width: EvidencePhotoTile._size,
          height: EvidencePhotoTile._size,
          decoration: BoxDecoration(
            borderRadius: BorderRadius.circular(HkRadius.sm),
            border: Border.all(color: AppColors.border),
          ),
          child: const Center(
            child: HkIcon(HkIconName.photoAdd, size: HkControl.icon),
          ),
        ),
      ),
    );
  }
}
