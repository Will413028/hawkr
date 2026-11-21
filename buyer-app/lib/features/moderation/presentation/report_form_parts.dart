import 'dart:io';

import 'package:flutter/material.dart';
import 'package:flutter_screenutil/flutter_screenutil.dart';
import 'package:hawkr_buyer/core/theme/app_colors.dart';
import 'package:hawkr_buyer/shared/icons/app_icon_colors.dart';
import 'package:hawkr_buyer/shared/icons/app_icons.dart';
import 'package:image_picker/image_picker.dart';

import '../../media/data/media_repository.dart';

InputDecoration reportFieldDecoration(String hint) => InputDecoration(
  hintText: hint,
  hintStyle: const TextStyle(fontSize: 14, color: AppColors.textDisabled),
  filled: true,
  fillColor: AppColors.gray100,
  contentPadding: EdgeInsets.symmetric(horizontal: 12.w, vertical: 12.h),
  border: OutlineInputBorder(
    borderRadius: BorderRadius.circular(8.r),
    borderSide: BorderSide.none,
  ),
);

Widget reportFieldLabel(String text, {bool required = false}) => RichText(
  text: TextSpan(
    style: const TextStyle(
      fontSize: 16,
      fontWeight: FontWeight.w700,
      color: AppColors.textPrimary,
    ),
    children: [
      TextSpan(text: text),
      if (required)
        const TextSpan(
          text: ' *',
          style: TextStyle(color: AppColors.danger, fontSize: 16),
        ),
    ],
  ),
);

Future<File?> pickReportPhoto() async {
  final picked = await ImagePicker().pickImage(
    source: ImageSource.gallery,
    maxWidth: 1024,
    imageQuality: 85,
  );
  return picked == null ? null : File(picked.path);
}

/// 把照片逐張上傳成暫存憑證；[cache] 以路徑記住已上傳的，失敗重送不重傳。
Future<List<String>> stageReportEvidence(
  MediaRepository media,
  List<File> photos,
  Map<String, String> cache,
) async {
  final tokens = <String>[];
  for (final file in photos) {
    var token = cache[file.path];
    if (token == null) {
      token = (await media.uploadImage(file: file)).stagedToken;
      cache[file.path] = token;
    }
    tokens.add(token);
  }
  return tokens;
}

class ReportPhotoRow extends StatelessWidget {
  const ReportPhotoRow({
    super.key,
    required this.photos,
    required this.maxCount,
    required this.disabled,
    required this.addKey,
    required this.onAdd,
    required this.onRemove,
  });

  final List<File> photos;
  final int maxCount;
  final bool disabled;
  final Key addKey;
  final VoidCallback onAdd;
  final void Function(int index) onRemove;

  @override
  Widget build(BuildContext context) {
    final size = 80.w;
    return SingleChildScrollView(
      scrollDirection: Axis.horizontal,
      child: Row(
        children: [
          for (var i = 0; i < photos.length; i++) ...[
            SizedBox.square(
              dimension: size,
              child: Stack(
                children: [
                  Positioned.fill(
                    child: ClipRRect(
                      borderRadius: BorderRadius.circular(16.r),
                      child: Image.file(
                        photos[i],
                        fit: BoxFit.cover,
                        cacheWidth: 256,
                        errorBuilder: (_, _, _) => const ColoredBox(
                          color: AppColors.bgPrimary,
                          child: Center(
                            child: Icon(
                              AppIcons.imageNotSupportedOutlined,
                              color: AppIconColors.muted,
                            ),
                          ),
                        ),
                      ),
                    ),
                  ),
                  Positioned(
                    top: 0,
                    right: 0,
                    child: GestureDetector(
                      onTap: disabled ? null : () => onRemove(i),
                      child: Container(
                        width: 20.w,
                        height: 20.w,
                        decoration: const BoxDecoration(
                          color: AppColors.primary,
                          shape: BoxShape.circle,
                        ),
                        child: Icon(
                          AppIcons.close,
                          size: 14.sp,
                          color: AppIconColors.onBrand,
                        ),
                      ),
                    ),
                  ),
                ],
              ),
            ),
            SizedBox(width: 10.w),
          ],
          if (photos.length < maxCount)
            GestureDetector(
              key: addKey,
              onTap: disabled ? null : onAdd,
              child: SizedBox.square(
                dimension: size,
                child: DecoratedBox(
                  decoration: BoxDecoration(
                    borderRadius: BorderRadius.circular(16.r),
                    border: Border.all(
                      color: AppColors.textPrimary.withValues(alpha: 0.3),
                      width: 1.5.w,
                    ),
                  ),
                  child: Icon(
                    AppIcons.add,
                    size: 24.sp,
                    color: AppIconColors.defaultColor.withValues(alpha: 0.5),
                  ),
                ),
              ),
            ),
        ],
      ),
    );
  }
}
