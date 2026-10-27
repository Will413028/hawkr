import 'package:hawkr_seller/shared/widgets/hk_button_label.dart';
import 'package:hawkr_seller/core/theme/app_colors.dart';
import 'package:hawkr_seller/shared/icons/app_icon_colors.dart';
import 'package:hawkr_seller/shared/icons/app_icons.dart';
import 'package:hawkr_seller/shared/widgets/hk_glass_app_bar.dart';
import 'package:hawkr_seller/shared/widgets/hk_media_picker_layout.dart';
import 'dart:typed_data';
import 'package:flutter/material.dart';
import 'package:flutter_screenutil/flutter_screenutil.dart';
import 'package:go_router/go_router.dart';
import 'package:image_picker/image_picker.dart';
import 'package:palette_generator_plus/palette_generator_plus.dart';
import 'package:photo_manager/photo_manager.dart';
import 'package:photo_manager_image_provider/photo_manager_image_provider.dart';
import 'package:video_player/video_player.dart';

import '../../../shared/utils/floating_toast.dart';
import '../../../shared/utils/platform_io.dart';
import '../../../shared/widgets/truck_loading_overlay.dart';

/// 商品媒體選取器：相冊選取或相機拍攝，可推進圖片編輯器（/photo-editor）
class ProductSetupMediaPickerScreen extends StatefulWidget {
  const ProductSetupMediaPickerScreen({super.key});

  @override
  State<ProductSetupMediaPickerScreen> createState() =>
      _ProductSetupMediaPickerScreenState();
}

class _ProductSetupMediaPickerScreenState
    extends State<ProductSetupMediaPickerScreen> {
  final ImagePicker _picker = ImagePicker();

  // Album and assets
  List<AssetEntity> _assets = [];
  bool _loading = true;
  String? _error;

  // Multi-select state
  final Set<int> _selectedIndices = {}; // indices into _assets
  AssetEntity? _previewAsset; // currently previewed asset
  File? _capturedFile; // if taken from camera
  Color _dominantColor = AppColors.textEmphasis;

  Future<void> _updateDominantColor() async {
    ImageProvider? provider;
    if (_capturedFile != null) {
      provider = FileImage(_capturedFile!);
    } else if (_previewAsset != null) {
      provider = AssetEntityImageProvider(
        _previewAsset!,
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
          _previewAsset = null;
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
        _previewAsset = assets.isNotEmpty ? assets.first : null;
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
        _previewAsset = null;
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
    final AssetEntity? sel = _previewAsset;
    if (sel == null) {
      return const Center(child: Text('沒有可預覽的媒體'));
    }
    if (sel.type == AssetType.video) {
      return FutureBuilder<File?>(
        future: sel.file,
        builder: (context, snapshot) {
          if (!snapshot.hasData) {
            return const Center(child: CircularProgressIndicator());
          }
          final file = snapshot.data!;
          return _VideoInlinePlayer(file: file);
        },
      );
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
    // 多選模式：收集所有選中的圖片，直接回傳（不進編輯器）
    if (_selectedIndices.isEmpty && _capturedFile == null) {
      FloatingToast.error(context, '請至少選擇一張照片');
      return;
    }

    if (!mounted) return;
    TruckLoadingOverlay.show(context, message: '準備中...');
    try {
      final List<Uint8List> imageBytesList = [];

      // 相機拍攝的照片
      if (_capturedFile != null) {
        final bytes = await _capturedFile!.readAsBytes();
        imageBytesList.add(bytes);
      }

      // 相簿多選的照片
      for (final idx in _selectedIndices.toList()..sort()) {
        if (idx < 0 || idx >= _assets.length) continue;
        final file = await _assets[idx].file;
        if (file != null) {
          final bytes = await file.readAsBytes();
          imageBytesList.add(bytes);
        }
      }

      if (!mounted) return;
      TruckLoadingOverlay.hide(context);

      if (imageBytesList.isEmpty) {
        FloatingToast.error(context, '無法讀取圖片');
        return;
      }

      context.pop({'images': imageBytesList});
    } catch (e) {
      if (mounted) {
        TruckLoadingOverlay.hide(context);
        FloatingToast.error(context, '讀取圖片失敗');
      }
    }
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      backgroundColor: AppColors.bgPrimary,
      appBar: HkGlassAppBar(
        title: const Text('選擇照片'),
        onBackPressed: () => context.pop(),
        actions: [
          TextButton(
            onPressed: _next,
            child: HkButtonLabel(
              _selectedIndices.isEmpty && _capturedFile == null
                  ? '完成'
                  : '完成(${_selectedIndices.length + (_capturedFile != null ? 1 : 0)})',
            ),
          ),
        ],
      ),
      body: _loading
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
                gridDelegate: const SliverGridDelegateWithFixedCrossAxisCount(
                  crossAxisCount: 4,
                  crossAxisSpacing: 1,
                  mainAxisSpacing: 1,
                ),
                itemCount: _assets.length + 1,
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
                  final assetIdx = index - 1;
                  final asset = _assets[assetIdx];
                  final isChecked = _selectedIndices.contains(assetIdx);
                  return GestureDetector(
                    onTap: () {
                      setState(() {
                        // 切換多選
                        if (isChecked) {
                          _selectedIndices.remove(assetIdx);
                        } else {
                          _selectedIndices.add(assetIdx);
                        }
                        // 更新預覽
                        _capturedFile = null;
                        _previewAsset = asset;
                      });
                      _updateDominantColor();
                    },
                    child: Stack(
                      fit: StackFit.expand,
                      children: [
                        AssetEntityImage(
                          asset,
                          isOriginal: false,
                          thumbnailSize: const ThumbnailSize.square(200),
                          thumbnailFormat: ThumbnailFormat.jpeg,
                          fit: BoxFit.cover,
                          errorBuilder: (context, error, stackTrace) =>
                              Container(color: AppColors.borderSubtle),
                        ),
                        // 選中遮罩
                        if (isChecked)
                          Container(
                            color: AppColors.textEmphasis.withValues(
                              alpha: 0.3,
                            ),
                          ),
                        // 勾選圈圈
                        Positioned(
                          right: 6,
                          top: 6,
                          child: Container(
                            width: 24.w,
                            height: 24.w,
                            decoration: BoxDecoration(
                              shape: BoxShape.circle,
                              color: isChecked
                                  ? AppColors.primary
                                  : AppColors.textEmphasis.withValues(
                                      alpha: 0.3,
                                    ),
                              border: Border.all(
                                color: AppColors.bgCard,
                                width: 2,
                              ),
                            ),
                            child: isChecked
                                ? Center(
                                    child: Text(
                                      '${_selectedIndices.toList().indexOf(assetIdx) + 1}',
                                      style: TextStyle(
                                        color: AppColors.bgCard,
                                        fontSize: 12.sp,
                                        fontWeight: FontWeight.w700,
                                      ),
                                    ),
                                  )
                                : null,
                          ),
                        ),
                      ],
                    ),
                  );
                },
              ),
            ),
    );
  }
}

class _VideoInlinePlayer extends StatefulWidget {
  final File file;
  const _VideoInlinePlayer({required this.file});

  @override
  State<_VideoInlinePlayer> createState() => _VideoInlinePlayerState();
}

class _VideoInlinePlayerState extends State<_VideoInlinePlayer> {
  late final VideoPlayerController _controller;
  bool _ready = false;

  @override
  void initState() {
    super.initState();
    _controller = VideoPlayerController.file(widget.file)
      ..setLooping(true)
      ..initialize().then((_) {
        if (!mounted) return;
        setState(() {
          _ready = true;
        });
      });
  }

  @override
  void dispose() {
    _controller.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    if (!_ready) return const TruckLoadingOverlay();
    return Stack(
      alignment: Alignment.center,
      children: [
        AspectRatio(
          aspectRatio: _controller.value.aspectRatio == 0
              ? 1
              : _controller.value.aspectRatio,
          child: VideoPlayer(_controller),
        ),
        GestureDetector(
          onTap: () {
            setState(() {
              if (_controller.value.isPlaying) {
                _controller.pause();
              } else {
                _controller.play();
              }
            });
          },
          child: Container(
            decoration: const BoxDecoration(
              color: AppColors.textDisabled,
              shape: BoxShape.circle,
            ),
            padding: EdgeInsets.all(8.w),
            child: Icon(
              _controller.value.isPlaying
                  ? AppIcons.pauseCircleFilled
                  : AppIcons.playCircleFill,
              size: 56.sp,
              color: AppIconColors.onBrand,
            ),
          ),
        ),
      ],
    );
  }
}
