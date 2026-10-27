import 'package:flutter/material.dart';

class DraggableReplyWrapper extends StatefulWidget {
  const DraggableReplyWrapper({
    super.key,
    required this.child,
    required this.enable,
    this.onSwipeRightReply,
    this.maxShift = 72,
    this.triggerShift = 28,
  });
  final Widget child;
  final bool enable;
  final VoidCallback? onSwipeRightReply;
  final double maxShift;
  final double triggerShift;
  @override
  State<DraggableReplyWrapper> createState() => _DraggableReplyWrapperState();
}

class _DraggableReplyWrapperState extends State<DraggableReplyWrapper> {
  double _dx = 0;
  bool _dragging = false;
  bool _fired = false;

  void _reset() {
    setState(() {
      _dragging = false;
      _dx = 0;
    });
  }

  @override
  Widget build(BuildContext context) {
    return TweenAnimationBuilder<double>(
      tween: Tween<double>(begin: 0, end: _dx),
      duration: _dragging ? Duration.zero : const Duration(milliseconds: 160),
      builder: (context, value, child) =>
          Transform.translate(offset: Offset(value, 0), child: child),
      child: GestureDetector(
        onHorizontalDragStart: widget.enable
            ? (_) {
                setState(() {
                  _dragging = true;
                  _fired = false;
                });
              }
            : null,
        onHorizontalDragUpdate: widget.enable
            ? (d) {
                final dd = d.primaryDelta ?? 0;
                if (dd <= 0) return; // only right swipe
                double next = _dx + dd;
                if (next > widget.maxShift) next = widget.maxShift;
                setState(() {
                  _dx = next;
                });
                if (!_fired && _dx >= widget.triggerShift) {
                  widget.onSwipeRightReply?.call();
                  _fired = true;
                }
              }
            : null,
        onHorizontalDragEnd: widget.enable ? (_) => _reset() : null,
        onHorizontalDragCancel: widget.enable ? () => _reset() : null,
        child: widget.child,
      ),
    );
  }
}
