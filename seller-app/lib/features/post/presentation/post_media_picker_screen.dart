import 'package:hawkr_seller/shared/widgets/hk_button_label.dart';
import 'package:hawkr_seller/core/theme/app_colors.dart';
import 'package:hawkr_seller/shared/icons/app_icon_colors.dart';
import 'package:hawkr_seller/shared/icons/app_icons.dart';
import 'package:hawkr_seller/shared/widgets/hk_glass_app_bar.dart';
import 'package:hawkr_seller/shared/widgets/hk_media_picker_layout.dart';
import '../../../shared/utils/platform_io.dart';
import 'dart:typed_data';

import 'package:flutter/material.dart';
import 'package:go_router/go_router.dart';
import 'package:image_picker/image_picker.dart';
import 'package:palette_generator_plus/palette_generator_plus.dart';
import 'package:photo_manager/photo_manager.dart';
import 'package:photo_manager_image_provider/photo_manager_image_provider.dart';

import '../../../shared/utils/floating_toast.dart';
import '../../../shared/utils/image_format_detector.dart';
import '../../../shared/widgets/truck_loading_overlay.dart';
import '../../photo_editor/presentation/photo_editor_args.dart';
import 'package:flutter_screenutil/flutter_screenutil.dart';

/// A lightweight in-app media picker similar to Instagram step 1.
/// - Top: preview of the currently selected media
/// - Bottom: recent gallery grid (first tile = camera)
/// - Top-right: "下一步" goes to PostComposeScreen for now
class PostMediaPickerScreen extends StatefulWidget {
  final String title;
  const PostMediaPickerScreen({super.key, this.title = 'New Post'});

  @override
  State<PostMediaPickerScreen> createState() => _PostMediaPickerScreenState();
}

class _PostMediaPickerScreenState extends State<PostMediaPickerScreen> {
  final ImagePicker _picker = ImagePicker();

  // Album and assets
  List<AssetEntity> _assets = [];
  bool _loading = true;
  String? _error;
  // Helpers: media compression
  Future<Uint8List?> _readImageBytes(File file) async {
    try {
      return await file.readAsBytes();
    } catch (_) {
      return null;
    }
  }

  /// 直接取得 PhotoKit 原始資料。`AssetEntity.file` 在 iOS 是壓縮後檔案，
  /// GIF 可能在到達格式偵測前就被轉成單幀 JPEG，因此不可作為 fallback。
  Future<Uint8List?> _readOriginalAssetBytes(AssetEntity asset) async {
    try {
      final bytes = await asset.originBytes;
      return bytes == null || bytes.isEmpty ? null : bytes;
    } catch (_) {
      return null;
    }
  }

  Future<bool> _isGifAsset(AssetEntity asset) async {
    try {
      final mimeType = (await asset.mimeTypeAsync)?.trim().toLowerCase();
      if (mimeType == 'image/gif') return true;
    } catch (_) {}

    final title = asset.title?.trim().toLowerCase();
    return title?.endsWith('.gif') ?? false;
  }

  /// GIF 必須讀 PhotoKit 原始資料以保留所有影格；其他格式沿用
  /// `AssetEntity.file` 的平台轉碼，避免 HEIC 壓縮成 JPEG 後仍沿用 HEIC MIME。
  Future<Uint8List?> _readAssetImageBytes(AssetEntity asset) async {
    if (await _isGifAsset(asset)) {
      return _readOriginalAssetBytes(asset);
    }
    final file = await asset.file;
    return file == null ? null : _readImageBytes(file);
  }

  AssetEntity? _selected;
  File? _capturedFile; // if taken from camera
  Color _dominantColor = AppColors.textEmphasis;

  Future<void> _updateDominantColor() async {
    ImageProvider? provider;
    if (_capturedFile != null) {
      provider = FileImage(_capturedFile!);
    } else if (_selected != null) {
      provider = AssetEntityImageProvider(
        _selected!,
        isOriginal: false,
        thumbnailSize: const ThumbnailSize.square(50),
      );
    }
    if (provider == null) return;
    try {
      final palette = await PaletteGenerator.fromImageProvider(
        provider,
        size: const Size(50, 50),
        maximumColorCount: 4,
        quantizer: const LegacyQuantizer(),
      );
      if (mounted) {
        setState(
          () => _dominantColor =
              palette.dominantColor?.color ??
              palette.colors.firstOrNull ??
              AppColors.textEmphasis,
        );
      }
    } catch (_) {}
  }

  @override
  void initState() {
    super.initState();
    // Defer heavy PhotoManager calls to after first frame to reduce iOS crash risk
    WidgetsBinding.instance.addPostFrameCallback((_) => _loadAssets());
  }

  Future<void> _loadAssets() async {
    setState(() {
      _loading = true;
      _error = null;
    });
    final PermissionState ps = await PhotoManager.requestPermissionExtend();
    // 處理 iOS 14+ 限制存取權限：hasAccess 表示至少有部分存取權限
    if (!ps.hasAccess) {
      setState(() {
        _error = '需要相簿權限才能選取媒體';
        _loading = false;
      });
      return;
    }

    // 如果是限制存取，顯示提示但繼續載入可用的照片
    if (ps == PermissionState.limited) {
      // 可選：顯示提示讓用戶知道只能看到部分照片
      debugPrint(
        '[PhotoPicker] Limited access - only selected photos are available',
      );
    }

    try {
      final List<AssetPathEntity> paths = await PhotoManager.getAssetPathList(
        onlyAll: true,
        type: RequestType.image, // images only
      );
      if (paths.isEmpty) {
        setState(() {
          _assets = [];
          _selected = null;
          _loading = false;
        });
        return;
      }
      // Load more photos - increased from 60 to 500
      final recent = paths.first;
      final List<AssetEntity> assets = await recent.getAssetListPaged(
        page: 0,
        size: 500,
      );
      setState(() {
        _assets = assets;
        _selected = assets.isNotEmpty ? assets.first : null;
        _loading = false;
      });
      _updateDominantColor();
    } catch (_) {
      setState(() {
        _error = '讀取相簿失敗，請重試';
        _loading = false;
      });
    }
  }

  Future<void> _openCamera() async {
    final XFile? taken = await _picker.pickImage(source: ImageSource.camera);
    if (taken != null) {
      setState(() {
        _capturedFile = File(taken.path);
        _selected = null; // using captured file for preview
      });
      _updateDominantColor();
    }
  }

  Widget _preview() {
    if (_capturedFile != null) {
      return _dominantColorPreview(
        child: Image.file(
          _capturedFile!,
          fit: BoxFit.contain,
          width: double.infinity,
          height: double.infinity,
        ),
      );
    }
    final AssetEntity? sel = _selected;
    if (sel == null) {
      return const Center(child: Text('沒有可預覽的媒體'));
    }
    return _dominantColorPreview(
      child: AssetEntityImage(
        sel,
        isOriginal: false,
        thumbnailSize: const ThumbnailSize.square(600),
        thumbnailFormat: ThumbnailFormat.jpeg,
        fit: BoxFit.contain,
        loadingBuilder: (context, child, progress) {
          if (progress == null) return child;
          return const Center(child: CircularProgressIndicator());
        },
      ),
    );
  }

  /// IG 風格預覽：主色背景 + 前景完整顯示
  Widget _dominantColorPreview({required Widget child}) {
    return Container(
      color: AppColors.bgPrimary,
      child: AnimatedContainer(
        duration: const Duration(milliseconds: 300),
        color: _dominantColor.withValues(alpha: 0.45),
        child: Center(child: child),
      ),
    );
  }

  Future<void> _next() async {
    // Prepare preview and source media bytes for upload.
    Uint8List? previewBytes;
    Uint8List? mediaBytes;
    String contentType = 'image';

    if (_capturedFile != null) {
      // Camera capture (image)
      try {
        TruckLoadingOverlay.show(context);
        mediaBytes = await _readImageBytes(_capturedFile!);
      } finally {
        if (mounted) TruckLoadingOverlay.hide(context);
      }
      previewBytes = mediaBytes;
      contentType = 'image';
    } else if (_selected != null) {
      contentType = 'image';
      try {
        TruckLoadingOverlay.show(context);
        mediaBytes = await _readAssetImageBytes(_selected!);
      } finally {
        if (mounted) TruckLoadingOverlay.hide(context);
      }
      if (!mounted) return;
      if (mediaBytes == null) {
        FloatingToast.error(context, '無法取得相簿原始圖片，請確認 iCloud 已下載後重試');
        return;
      }
      // 使用完整圖片作為預覽，確保進入編輯器時有完整圖片資料
      previewBytes = mediaBytes;
    }

    if (!mounted) return;
    if (mediaBytes == null && previewBytes == null) {
      // ScaffoldMessenger.of(context).showSnackBar(const SnackBar(content: Text('尚未選擇媒體')));
      FloatingToast.error(context, '尚未選擇媒體');
      return;
    }

    // Open compose; if published, bubble the created result back to caller
    final created = await context.push<Map<String, dynamic>>(
      '/post/compose',
      extra: {
        'imagePreview': previewBytes,
        'mediaBytes': mediaBytes,
        'contentType': contentType,
      },
    );
    if (!mounted) return;
    if (created != null) {
      context.pop(created);
    }
  }

  Future<(Uint8List? preview, Uint8List? media, String contentType)>
  _prepareMediaForAi() async {
    Uint8List? previewBytes;
    Uint8List? mediaBytes;
    String contentType = 'image';

    if (_capturedFile != null) {
      mediaBytes = await _readImageBytes(_capturedFile!);
      previewBytes = mediaBytes;
      contentType = 'image';
    } else if (_selected != null) {
      contentType = 'image';
      mediaBytes = await _readAssetImageBytes(_selected!);
      previewBytes ??= await _selected!.thumbnailDataWithSize(
        const ThumbnailSize.square(600),
      );
      previewBytes ??= mediaBytes;
    }
    return (previewBytes, mediaBytes, contentType);
  }

  Future<void> _openAiEditor() async {
    if (_capturedFile == null && _selected == null) {
      if (!mounted) return;
      FloatingToast.error(context, '請先選擇媒體');
      return;
    }
    TruckLoadingOverlay.show(context);
    Uint8List? mediaBytes;
    String contentType;
    try {
      (_, mediaBytes, contentType) = await _prepareMediaForAi();
    } finally {
      if (mounted) TruckLoadingOverlay.hide(context);
    }
    if (!mounted) return;
    if (contentType != 'image') {
      FloatingToast.error(context, '目前只支援圖片 AI 編輯');
      return;
    }
    if (mediaBytes == null || mediaBytes.isEmpty) {
      FloatingToast.error(context, '無法取得媒體檔案，請重試');
      return;
    }
    if (ImageFormatDetector.detect(mediaBytes).$2 == 'image/gif') {
      FloatingToast.error(context, 'GIF 動圖暫不支援編輯，已保留動圖格式');
      return;
    }

    // 直接帶圖片進 photo-editor，跳過 /seller/photo-tools 中間頁
    final tempFile = File(
      '${Directory.systemTemp.path}/post_edit_${DateTime.now().millisecondsSinceEpoch}.jpg',
    );
    await tempFile.writeAsBytes(mediaBytes);

    if (!mounted) return;
    final edited = await context.push<File?>(
      '/photo-editor',
      extra: PhotoEditorArgs(file: tempFile, category: 'post'),
    );

    // 編輯完成 → 用編輯結果取代當前預覽，使用者直接按「下一步」就能帶著編輯後的圖片進入 compose
    if (edited != null && mounted) {
      setState(() {
        _capturedFile = edited;
        _selected = null;
      });
      _updateDominantColor();
    }
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      backgroundColor: AppColors.bgPrimary,
      appBar: HkGlassAppBar(
        title: Text(widget.title == 'New Post' ? '新增貼文' : widget.title),
        actions: [
          TextButton(onPressed: _next, child: const HkButtonLabel('下一步')),
        ],
      ),
      body: Stack(
        children: [
          Positioned.fill(
            child: _loading
                ? const TruckLoadingOverlay()
                : (_error != null)
                ? Center(child: Text(_error!))
                : HkMediaPickerLayout(
                    preview: Container(
                      color: AppColors.textEmphasis,
                      child: _preview(),
                    ),
                    grid: GridView.builder(
                      padding: EdgeInsets.all(4.w),
                      gridDelegate:
                          const SliverGridDelegateWithFixedCrossAxisCount(
                            crossAxisCount: 4,
                            crossAxisSpacing: 1,
                            mainAxisSpacing: 1,
                          ),
                      itemCount: _assets.length + 1, // +1 for camera tile
                      itemBuilder: (context, index) {
                        if (index == 0) {
                          return InkWell(
                            onTap: _openCamera,
                            child: Container(
                              color: AppColors.textEmphasis,
                              child: const Center(
                                child: Icon(
                                  AppIcons.photoCamera,
                                  color: AppIconColors.onBrand,
                                ),
                              ),
                            ),
                          );
                        }
                        final asset = _assets[index - 1];
                        return GestureDetector(
                          onTap: () {
                            setState(() {
                              _capturedFile = null;
                              _selected = asset;
                            });
                            _updateDominantColor();
                          },
                          child: AssetEntityImage(
                            asset,
                            isOriginal: false,
                            thumbnailSize: const ThumbnailSize.square(150),
                            thumbnailFormat: ThumbnailFormat.jpeg,
                            fit: BoxFit.cover,
                            errorBuilder: (context, error, stackTrace) =>
                                const ColoredBox(color: AppColors.gray100),
                          ),
                        );
                      },
                    ),
                  ),
          ),
          // 浮動 AI 按鈕（參考商店頁的加號樣式）
          Positioned(
            left: 0,
            right: 0,
            bottom: 16,
            child: SafeArea(
              minimum: EdgeInsets.only(bottom: 16.h),
              child: Center(
                child: GestureDetector(
                  onTap: _openAiEditor,
                  child: Container(
                    width: 56.w,
                    height: 56.h,
                    decoration: BoxDecoration(
                      color: AppColors.primary,
                      shape: BoxShape.circle,
                      boxShadow: [
                        BoxShadow(
                          color: AppColors.cardShadow,
                          blurRadius: 8,
                          offset: Offset(0, 2.h),
                        ),
                      ],
                    ),
                    child: Icon(
                      AppIcons.autoFixHigh,
                      color: AppIconColors.onBrand,
                      size: 28.sp,
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
