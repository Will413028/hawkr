import 'package:extended_text_field/extended_text_field.dart';
import 'package:flutter/material.dart';
import 'package:hooks_riverpod/hooks_riverpod.dart';

import '../../../../shared/widgets/hashtag_autocomplete_popover.dart';
import '../../../../shared/widgets/hashtag_span_builder.dart';
import '../../data/post_hashtag_extractor.dart';
import '../../data/post_hashtag_repository.dart';

class PostHashtagTextFormField extends ConsumerStatefulWidget {
  const PostHashtagTextFormField({
    super.key,
    required this.controller,
    this.maxLines,
    this.minLines,
    this.decoration,
    this.style,
    this.validator,
  });

  final TextEditingController controller;
  final int? maxLines;
  final int? minLines;
  final InputDecoration? decoration;
  final TextStyle? style;
  final FormFieldValidator<String>? validator;

  @override
  ConsumerState<PostHashtagTextFormField> createState() =>
      _PostHashtagTextFormFieldState();
}

class _PostHashtagTextFormFieldState
    extends ConsumerState<PostHashtagTextFormField> {
  final FocusNode _focusNode = FocusNode();
  final OverlayPortalController _overlayController = OverlayPortalController();
  final LayerLink _layerLink = LayerLink();
  PostHashtagTaggingMode? _taggingMode;
  // Mirrors the TextField's measured width so the floating popover matches it.
  double _fieldWidth = 0;

  @override
  void initState() {
    super.initState();
    widget.controller.addListener(_recomputeTaggingMode);
    _focusNode.addListener(_onFocusChanged);
  }

  @override
  void didUpdateWidget(covariant PostHashtagTextFormField oldWidget) {
    super.didUpdateWidget(oldWidget);
    if (oldWidget.controller != widget.controller) {
      oldWidget.controller.removeListener(_recomputeTaggingMode);
      widget.controller.addListener(_recomputeTaggingMode);
      _recomputeTaggingMode();
    }
  }

  @override
  void dispose() {
    widget.controller.removeListener(_recomputeTaggingMode);
    _focusNode.removeListener(_onFocusChanged);
    _focusNode.dispose();
    super.dispose();
  }

  void _onFocusChanged() {
    if (!_focusNode.hasFocus) {
      _setTaggingMode(null);
      return;
    }
    _recomputeTaggingMode();
  }

  void _recomputeTaggingMode() {
    if (!_focusNode.hasFocus) {
      _setTaggingMode(null);
      return;
    }
    final selection = widget.controller.selection;
    if (!selection.isValid || !selection.isCollapsed) {
      _setTaggingMode(null);
      return;
    }
    final caret = selection.start;
    final text = widget.controller.text;
    if (caret < 0 || caret > text.length) {
      _setTaggingMode(null);
      return;
    }
    final before = text.substring(0, caret);
    _setTaggingMode(PostHashtagExtractor.detectTaggingMode(before));
  }

  void _setTaggingMode(PostHashtagTaggingMode? next) {
    if (next == null && _taggingMode == null) return;
    if (next != null &&
        _taggingMode != null &&
        next.hashAt == _taggingMode!.hashAt &&
        next.prefix == _taggingMode!.prefix) {
      return;
    }
    if (mounted) setState(() => _taggingMode = next);
    if (next != null) {
      if (!_overlayController.isShowing) _overlayController.show();
    } else {
      if (_overlayController.isShowing) _overlayController.hide();
    }
  }

  void _onSelectHashtag(String name) {
    final mode = _taggingMode;
    if (mode == null) return;

    setState(() => _taggingMode = null);
    if (_overlayController.isShowing) _overlayController.hide();

    final text = widget.controller.text;
    final before = text.substring(0, mode.hashAt);
    final prefixEnd = mode.hashAt + 1 + mode.prefix.length;
    final after = prefixEnd <= text.length ? text.substring(prefixEnd) : '';
    final inserted = '#$name ';
    final nextText = '$before$inserted$after';
    final nextCaret = before.length + inserted.length;

    widget.controller.value = TextEditingValue(
      text: nextText,
      selection: TextSelection.collapsed(offset: nextCaret),
    );
    if (!_focusNode.hasFocus) _focusNode.requestFocus();
  }

  Future<List<HashtagAutocompleteItem>> _search(String prefix) async {
    final items = await ref
        .read(postHashtagRepositoryProvider)
        .search(prefix: prefix, mode: 'prefix', limit: 10);
    return items
        .map(
          (item) =>
              HashtagAutocompleteItem(name: item.name, count: item.postCount),
        )
        .toList(growable: false);
  }

  @override
  Widget build(BuildContext context) {
    return CompositedTransformTarget(
      link: _layerLink,
      child: OverlayPortal(
        controller: _overlayController,
        overlayChildBuilder: (overlayContext) {
          final mode = _taggingMode;
          if (mode == null || _fieldWidth <= 0) {
            return const SizedBox.shrink();
          }
          return CompositedTransformFollower(
            link: _layerLink,
            targetAnchor: Alignment.bottomLeft,
            followerAnchor: Alignment.topLeft,
            offset: const Offset(0, 4),
            showWhenUnlinked: false,
            child: Material(
              type: MaterialType.transparency,
              child: SizedBox(
                width: _fieldWidth,
                child: HashtagAutocompletePopover(
                  prefix: mode.prefix,
                  search: _search,
                  countLabelBuilder: (count) => '$count 篇貼文',
                  onSelect: _onSelectHashtag,
                ),
              ),
            ),
          );
        },
        child: LayoutBuilder(
          builder: (context, constraints) {
            // 在下一個 frame 更新 _fieldWidth，避免在 layout 階段 setState。
            if (_fieldWidth != constraints.maxWidth &&
                constraints.maxWidth.isFinite) {
              final next = constraints.maxWidth;
              WidgetsBinding.instance.addPostFrameCallback((_) {
                if (mounted && _fieldWidth != next) {
                  setState(() => _fieldWidth = next);
                }
              });
            }
            return FormField<String>(
              initialValue: widget.controller.text,
              validator: widget.validator,
              builder: (state) {
                return Column(
                  crossAxisAlignment: CrossAxisAlignment.stretch,
                  mainAxisSize: MainAxisSize.min,
                  children: [
                    ExtendedTextField(
                      controller: widget.controller,
                      focusNode: _focusNode,
                      maxLines: widget.maxLines,
                      minLines: widget.minLines,
                      style: widget.style,
                      specialTextSpanBuilder: HashtagSpanBuilder(),
                      decoration: (widget.decoration ?? const InputDecoration())
                          .copyWith(errorText: state.errorText),
                      onChanged: state.didChange,
                    ),
                  ],
                );
              },
            );
          },
        ),
      ),
    );
  }
}
