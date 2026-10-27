import 'package:hawkr_buyer/core/theme/app_colors.dart';
import 'package:hawkr_buyer/shared/icons/app_icon_colors.dart';
import 'package:hawkr_buyer/shared/icons/app_icons.dart';
import 'package:flutter/material.dart';
import 'package:flutter_screenutil/flutter_screenutil.dart';
import 'package:video_player/video_player.dart';
import 'package:hawkr_buyer/shared/widgets/hk_brand_loader.dart';

class PostVideoPlayer extends StatefulWidget {
  const PostVideoPlayer({
    super.key,
    required this.url,
    this.autoPlay = true,
    this.loop = true,
    this.muted = true,
  });
  final String url;
  final bool autoPlay;
  final bool loop;
  final bool muted;

  @override
  State<PostVideoPlayer> createState() => _PostVideoPlayerState();
}

class _PostVideoPlayerState extends State<PostVideoPlayer> {
  VideoPlayerController? _controller;
  bool _initializing = true;
  bool _hasError = false;

  @override
  void initState() {
    super.initState();
    _init();
  }

  Future<void> _init() async {
    try {
      final uri = Uri.parse(widget.url);
      final c = VideoPlayerController.networkUrl(uri);
      setState(() {
        _controller = c;
      });
      await c.initialize();
      if (widget.loop) c.setLooping(true);
      if (widget.muted) c.setVolume(0);
      if (widget.autoPlay) c.play();
      if (!mounted) return;
      setState(() {
        _initializing = false;
      });
    } catch (_) {
      if (!mounted) return;
      setState(() {
        _hasError = true;
        _initializing = false;
      });
    }
  }

  @override
  void dispose() {
    _controller?.dispose();
    super.dispose();
  }

  void _toggle() {
    final c = _controller;
    if (c == null) return;
    if (c.value.isPlaying) {
      c.pause();
    } else {
      c.play();
    }
    setState(() {});
  }

  @override
  Widget build(BuildContext context) {
    if (_hasError) {
      return Container(
        color: AppColors.dividerLight,
        child: const Center(child: Icon(AppIcons.brokenImageOutlined)),
      );
    }
    final c = _controller;
    if (c == null || _initializing) {
      return const Center(child: HkBrandLoader(size: HkBrandLoader.page));
    }
    return Stack(
      alignment: Alignment.center,
      children: [
        FittedBox(
          fit: BoxFit.cover,
          clipBehavior: Clip.hardEdge,
          child: SizedBox(
            width: c.value.size.width,
            height: c.value.size.height,
            child: VideoPlayer(c),
          ),
        ),
        Positioned.fill(
          child: Material(
            color: Colors.transparent,
            child: InkWell(onTap: _toggle),
          ),
        ),
        if (!c.value.isPlaying)
          Container(
            decoration: BoxDecoration(
              color: AppColors.textDisabled,
              borderRadius: BorderRadius.circular(32.r),
            ),
            padding: EdgeInsets.all(8.w),
            child: Icon(
              AppIcons.playArrowRounded,
              size: 48.sp,
              color: AppIconColors.onBrand,
            ),
          ),
      ],
    );
  }
}
