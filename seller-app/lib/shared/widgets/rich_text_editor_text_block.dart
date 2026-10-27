import 'package:hawkr_seller/core/theme/app_colors.dart';
import 'package:hawkr_seller/shared/icons/app_icon_colors.dart';
import 'package:hawkr_seller/shared/icons/app_icons.dart';
import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_quill/flutter_quill.dart';
import 'package:flutter_screenutil/flutter_screenutil.dart';

import '../../config/image_config.dart';
import '../../features/product/data/hashtag_extractor.dart';
import 'hashtag_autocomplete_popover.dart';
import 'optimized_image.dart';

/// 區塊類型（text = 行內富文本 / image / table / list）
enum BlockType { text, image, table, list }

/// 表格 cell：內含 runs（與 02 §3.6 一致；賣家 app 內部編輯用純文字單一 run）。
class TableCellData {
  String text;
  TableCellData(this.text);
}

/// 文字區塊樣式
class TextBlockStyle {
  final bool isBold;
  final bool isItalic;
  final Color color;
  final double fontSize;
  final FontWeight fontWeight;
  final TextAlign alignment;

  const TextBlockStyle({
    this.isBold = false,
    this.isItalic = false,
    this.color = Colors.black,
    this.fontSize = 14.0,
    this.fontWeight = FontWeight.w400,
    this.alignment = TextAlign.left,
  });

  TextBlockStyle copyWith({
    bool? isBold,
    bool? isItalic,
    Color? color,
    double? fontSize,
    FontWeight? fontWeight,
    TextAlign? alignment,
  }) {
    return TextBlockStyle(
      isBold: isBold ?? this.isBold,
      isItalic: isItalic ?? this.isItalic,
      color: color ?? this.color,
      fontSize: fontSize ?? this.fontSize,
      fontWeight: fontWeight ?? this.fontWeight,
      alignment: alignment ?? this.alignment,
    );
  }

  TextBlockStyle copy() {
    return TextBlockStyle(
      isBold: isBold,
      isItalic: isItalic,
      color: color,
      fontSize: fontSize,
      fontWeight: fontWeight,
      alignment: alignment,
    );
  }
}

/// 編輯器區塊資料（文字或圖片）
class TextBlockData {
  final String id;
  final BlockType type;
  String text;
  TextBlockStyle style;

  /// 文字 block 的行內富文本 controller（inline runs 存這裡；由父層建立/釋放）。
  QuillController? quillController;

  // 圖片相關
  Uint8List? imageBytes;
  String? imageUrl;
  String? objectKey;
  String? stagedToken;
  bool isUploading;

  // 表格相關（type=table）
  List<List<TableCellData>>? tableCells;
  bool tableWithHeadings;

  // 清單相關（type=list）：項目純文字（賣家 app 內部編輯用純文字單一 run）
  List<String>? listItems;
  bool listOrdered;

  TextBlockData({
    required this.id,
    required this.text,
    this.type = BlockType.text,
    this.style = const TextBlockStyle(),
    this.quillController,
    this.imageBytes,
    this.imageUrl,
    this.objectKey,
    this.stagedToken,
    this.isUploading = false,
    this.tableCells,
    this.tableWithHeadings = false,
    this.listItems,
    this.listOrdered = false,
  });

  TextBlockData copy() {
    QuillController? qc;
    if (type == BlockType.text && quillController != null) {
      qc = QuillController(
        document: Document.fromDelta(quillController!.document.toDelta()),
        selection: const TextSelection.collapsed(offset: 0),
      );
    }
    return TextBlockData(
      id: id,
      text: text,
      type: type,
      style: style.copy(),
      quillController: qc,
      imageBytes: imageBytes,
      imageUrl: imageUrl,
      objectKey: objectKey,
      stagedToken: stagedToken,
      isUploading: isUploading,
      tableCells: tableCells
          ?.map((row) => row.map((c) => TableCellData(c.text)).toList())
          .toList(),
      tableWithHeadings: tableWithHeadings,
      listItems: listItems?.toList(),
      listOrdered: listOrdered,
    );
  }

  void disposeControllers() {
    quillController?.dispose();
    quillController = null;
  }
}

/// 富文本編輯器結果
///
/// [content] 純文字預覽（用於顯示摘要）
/// [blocks] 結構化區塊（供 API description 欄位使用，含 stagedToken）
/// [stagedTokens] 所有圖片的 stagedToken（供 FinalizeImages 使用）
class RichTextEditorResult {
  final String content;
  final List<Map<String, dynamic>> blocks;
  final List<String> stagedTokens;

  const RichTextEditorResult({
    required this.content,
    required this.blocks,
    required this.stagedTokens,
  });
}

/// 線性區塊 Widget
///
/// 左側拖曳把手 + 中央內容（文字或圖片） + 右側刪除鈕（選中時）
class TextBlockWidget extends StatefulWidget {
  const TextBlockWidget({
    super.key,
    required this.data,
    required this.index,
    required this.isSelected,
    required this.onTap,
    required this.onTextChanged,
    required this.onDelete,
  });

  final TextBlockData data;
  final int index;
  final bool isSelected;
  final VoidCallback onTap;
  final ValueChanged<String> onTextChanged;
  final VoidCallback onDelete;

  @override
  State<TextBlockWidget> createState() => _TextBlockWidgetState();
}

class _TextBlockWidgetState extends State<TextBlockWidget> {
  /// 表格目前聚焦的格子（刪「這一列/欄」的錨點；null = 未聚焦，刪尾）。
  int? _tableFocusRow;
  int? _tableFocusCol;

  /// 表格結構版本：列/欄增刪或格內展開後 +1，讓 cell 的 ValueKey 全數更新
  /// —— TextFormField 的 initialValue 只在首次 build 生效，結構變更後若
  /// 沿用同 key 會顯示舊值。
  int _tableGen = 0;

  final FocusNode _focusNode = FocusNode();
  final ScrollController _editorScroll = ScrollController();

  // Hashtag autocomplete tagging-mode 狀態（caret 在 #xxx 中間就會 non-null）
  HashtagTaggingMode? _taggingMode;

  QuillController? get _qc => widget.data.quillController;

  @override
  void initState() {
    super.initState();
    _qc?.addListener(_onQuillChanged);
    _focusNode.addListener(_onFocusChanged);
  }

  @override
  void didUpdateWidget(TextBlockWidget oldWidget) {
    super.didUpdateWidget(oldWidget);
    // undo/redo 會換掉 controller instance → 重新掛 listener
    if (oldWidget.data.quillController != widget.data.quillController) {
      oldWidget.data.quillController?.removeListener(_onQuillChanged);
      widget.data.quillController?.addListener(_onQuillChanged);
    }
    if (widget.data.type == BlockType.text &&
        widget.isSelected &&
        !oldWidget.isSelected) {
      _focusNode.requestFocus();
    }
  }

  @override
  void dispose() {
    // controller 由父層擁有並釋放，這裡只移除 listener
    _qc?.removeListener(_onQuillChanged);
    _focusNode.removeListener(_onFocusChanged);
    _focusNode.dispose();
    _editorScroll.dispose();
    super.dispose();
  }

  void _onQuillChanged() {
    final plain = _qc?.document.toPlainText() ?? '';
    widget.data.text = plain;
    widget.onTextChanged(plain);
    _recomputeTaggingMode();
  }

  void _onFocusChanged() {
    if (_focusNode.hasFocus && !widget.isSelected) {
      widget.onTap();
    }
    if (!_focusNode.hasFocus) {
      _setTaggingMode(null);
    } else {
      _recomputeTaggingMode();
    }
  }

  void _recomputeTaggingMode() {
    final qc = _qc;
    if (qc == null || !_focusNode.hasFocus) {
      _setTaggingMode(null);
      return;
    }
    final sel = qc.selection;
    if (!sel.isValid || !sel.isCollapsed) {
      _setTaggingMode(null);
      return;
    }
    final caret = sel.baseOffset;
    final text = qc.document.toPlainText();
    if (caret < 0 || caret > text.length) {
      _setTaggingMode(null);
      return;
    }
    final before = text.substring(0, caret);
    _setTaggingMode(HashtagExtractor.detectTaggingMode(before));
  }

  void _setTaggingMode(HashtagTaggingMode? next) {
    if (next == null && _taggingMode == null) return;
    if (next != null &&
        _taggingMode != null &&
        next.hashAt == _taggingMode!.hashAt &&
        next.prefix == _taggingMode!.prefix) {
      return;
    }
    setState(() => _taggingMode = next);
  }

  /// 用戶選了一個 hashtag → 取代 `#prefix` 為 `#name `（走 Quill replaceText）
  void _onSelectHashtag(String name) {
    final mode = _taggingMode;
    final qc = _qc;
    if (mode == null || qc == null) return;
    setState(() => _taggingMode = null);
    final prefixEnd = mode.hashAt + 1 + mode.prefix.length;
    final inserted = '#$name ';
    qc.replaceText(
      mode.hashAt,
      prefixEnd - mode.hashAt,
      inserted,
      TextSelection.collapsed(offset: mode.hashAt + inserted.length),
    );
    if (!_focusNode.hasFocus) _focusNode.requestFocus();
  }

  @override
  Widget build(BuildContext context) {
    return GestureDetector(
      onTap: () {
        widget.onTap();
        if (widget.data.type == BlockType.text) {
          _focusNode.requestFocus();
        }
      },
      child: Container(
        padding: EdgeInsets.symmetric(vertical: 4.h),
        color: widget.isSelected ? AppColors.primarySoft : Colors.transparent,
        child: Row(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            // 拖曳把手
            ReorderableDragStartListener(
              index: widget.index,
              child: Padding(
                padding: EdgeInsets.only(left: 4.w, top: 8.h, right: 4.w),
                child: Icon(
                  AppIcons.dragIndicator,
                  color: widget.isSelected
                      ? AppIconColors.secondary
                      : AppIconColors.disabled,
                  size: 20.sp,
                ),
              ),
            ),
            // 內容區
            Expanded(
              child: widget.data.type == BlockType.image
                  ? _buildImageContent()
                  : widget.data.type == BlockType.table
                  ? _buildTableContent()
                  : widget.data.type == BlockType.list
                  ? _buildListContent()
                  : _buildTextContent(),
            ),
            // 刪除按鈕（選中時）
            if (widget.isSelected)
              GestureDetector(
                onTap: widget.onDelete,
                child: Padding(
                  padding: EdgeInsets.only(top: 8.h, right: 8.w),
                  child: Icon(
                    AppIcons.close,
                    color: AppIconColors.danger,
                    size: 20.sp,
                  ),
                ),
              ),
          ],
        ),
      ),
    );
  }

  Widget _buildTextContent() {
    final qc = widget.data.quillController;
    if (qc == null) return const SizedBox.shrink();
    return Padding(
      padding: EdgeInsets.symmetric(horizontal: 8.w, vertical: 4.h),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.stretch,
        children: [
          // 行內富文本編輯器（flutter_quill）：粗體/斜體/底線/顏色/連結由上方
          // 工具列對「目前選取範圍」套用，inline runs 存 QuillController。
          // hashtag 偵測/插入改讀 QuillController（見 _onQuillChanged / _onSelectHashtag）。
          QuillEditor(
            controller: qc,
            focusNode: _focusNode,
            scrollController: _editorScroll,
            config: QuillEditorConfig(
              scrollable: false,
              expands: false,
              autoFocus: false,
              placeholder: '在此輸入文字...',
              padding: EdgeInsets.symmetric(vertical: 8.h),
            ),
          ),
          if (_taggingMode != null)
            HashtagAutocompletePopover(
              prefix: _taggingMode!.prefix,
              onSelect: _onSelectHashtag,
            ),
        ],
      ),
    );
  }

  /// 格內貼上表格資料（tab/多行）→ 試算表語意：從錨點格 (r,c) 逐格填入、
  /// 列/欄不足自動擴增（上限 欄10/列50/格200 字，對齊商品描述表格規範），
  /// 覆蓋既有內容（Notion 行為，issue #1246）。
  void _expandGridIntoCells(
    int anchorRow,
    int anchorCol,
    List<List<String>> grid,
  ) {
    const maxRows = 50, maxCols = 10, maxCellLen = 200;
    final rows = widget.data.tableCells;
    if (rows == null || rows.isEmpty || grid.isEmpty) return;
    setState(() {
      final gridCols = grid
          .map((r) => r.length)
          .fold<int>(0, (a, b) => a > b ? a : b);
      final needRows = (anchorRow + grid.length).clamp(0, maxRows);
      var colCount = rows
          .map((r) => r.length)
          .fold<int>(0, (a, b) => a > b ? a : b);
      final needCols = (anchorCol + gridCols).clamp(0, maxCols);
      while (rows.length < needRows) {
        rows.add(List.generate(colCount, (_) => TableCellData('')));
      }
      if (needCols > colCount) {
        for (final row in rows) {
          while (row.length < needCols) {
            row.add(TableCellData(''));
          }
        }
        colCount = needCols;
      }
      for (var i = 0; i < grid.length; i++) {
        final tr = anchorRow + i;
        if (tr >= rows.length || tr >= maxRows) break;
        for (var j = 0; j < grid[i].length; j++) {
          final tc = anchorCol + j;
          if (tc >= rows[tr].length || tc >= maxCols) break;
          var v = grid[i][j];
          if (v.length > maxCellLen) v = v.substring(0, maxCellLen);
          rows[tr][tc].text = v;
        }
      }
      _tableGen++; // 讓所有 cell 以新 initialValue 重建
    });
  }

  /// 表格編輯 UI：逐格純文字輸入 + 加/刪列欄（橫向捲動）。
  Widget _buildTableContent() {
    final cells = widget.data.tableCells;
    if (cells == null || cells.isEmpty) return const SizedBox.shrink();
    final colCount = cells
        .map((r) => r.length)
        .fold<int>(0, (a, b) => a > b ? a : b);
    return Padding(
      padding: EdgeInsets.symmetric(horizontal: 8.w, vertical: 8.h),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.stretch,
        children: [
          // 列/欄增減控制
          Wrap(
            spacing: 8.w,
            children: [
              _tableCtrlBtn('加列', AppIcons.add, () {
                setState(() {
                  widget.data.tableCells!.add(
                    List.generate(colCount, (_) => TableCellData('')),
                  );
                  _tableGen++;
                });
              }),
              _tableCtrlBtn(
                _tableFocusRow != null ? '刪此列' : '刪列',
                AppIcons.remove,
                () {
                  final rows = widget.data.tableCells!;
                  if (rows.length > 1) {
                    setState(() {
                      final at =
                          (_tableFocusRow != null &&
                              _tableFocusRow! < rows.length)
                          ? _tableFocusRow!
                          : rows.length - 1;
                      rows.removeAt(at);
                      _tableFocusRow = null;
                      _tableFocusCol = null;
                      _tableGen++;
                    });
                  }
                },
              ),
              _tableCtrlBtn('加欄', AppIcons.add, () {
                setState(() {
                  for (final row in widget.data.tableCells!) {
                    row.add(TableCellData(''));
                  }
                  _tableGen++;
                });
              }),
              _tableCtrlBtn(
                _tableFocusCol != null ? '刪此欄' : '刪欄',
                AppIcons.remove,
                () {
                  if (colCount > 1) {
                    setState(() {
                      final at =
                          (_tableFocusCol != null && _tableFocusCol! < colCount)
                          ? _tableFocusCol!
                          : colCount - 1;
                      for (final row in widget.data.tableCells!) {
                        if (at < row.length) row.removeAt(at);
                      }
                      _tableFocusRow = null;
                      _tableFocusCol = null;
                      _tableGen++;
                    });
                  }
                },
              ),
              _tableCtrlBtn(
                widget.data.tableWithHeadings ? '標題列✓' : '標題列',
                AppIcons.title,
                () => setState(
                  () => widget.data.tableWithHeadings =
                      !widget.data.tableWithHeadings,
                ),
              ),
            ],
          ),
          SizedBox(height: 8.h),
          SingleChildScrollView(
            scrollDirection: Axis.horizontal,
            child: Table(
              border: TableBorder.all(color: const Color(0xFFD0D0D0), width: 1),
              defaultColumnWidth: FixedColumnWidth(120.w),
              children: [
                for (var r = 0; r < cells.length; r++)
                  TableRow(
                    decoration: (widget.data.tableWithHeadings && r == 0)
                        ? const BoxDecoration(color: AppColors.gray100)
                        : null,
                    children: [
                      for (var c = 0; c < colCount; c++)
                        Padding(
                          padding: EdgeInsets.symmetric(
                            horizontal: 8.w,
                            vertical: 4.h,
                          ),
                          child: TextFormField(
                            key: ValueKey(
                              'cell-${widget.data.id}-$r-$c-$_tableGen',
                            ),
                            initialValue: c < cells[r].length
                                ? cells[r][c].text
                                : '',
                            maxLines: null,
                            onTap: () {
                              _tableFocusRow = r;
                              _tableFocusCol = c;
                              // 只更新刪列/欄按鈕文案，不重建 cell
                              setState(() {});
                            },
                            inputFormatters: [
                              _CellPasteExpandFormatter(
                                onGridPasted: (grid) =>
                                    _expandGridIntoCells(r, c, grid),
                              ),
                            ],
                            style: TextStyle(
                              fontSize: 14.sp,
                              fontWeight:
                                  (widget.data.tableWithHeadings && r == 0)
                                  ? FontWeight.bold
                                  : FontWeight.w400,
                              color: AppColors.textEmphasis,
                            ),
                            decoration: const InputDecoration(
                              isDense: true,
                              border: InputBorder.none,
                              contentPadding: EdgeInsets.zero,
                            ),
                            onChanged: (v) {
                              if (c < cells[r].length) cells[r][c].text = v;
                            },
                          ),
                        ),
                    ],
                  ),
              ],
            ),
          ),
        ],
      ),
    );
  }

  /// 清單編輯 UI：逐項純文字輸入 + 加/刪項 + 有序/無序切換。
  Widget _buildListContent() {
    final items = widget.data.listItems;
    if (items == null) return const SizedBox.shrink();
    final ordered = widget.data.listOrdered;
    return Padding(
      padding: EdgeInsets.symmetric(horizontal: 8.w, vertical: 8.h),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.stretch,
        children: [
          // 加/刪項 + 有序切換
          Wrap(
            spacing: 8.w,
            children: [
              _tableCtrlBtn('加項', AppIcons.add, () {
                setState(() => widget.data.listItems!.add(''));
              }),
              _tableCtrlBtn('刪項', AppIcons.remove, () {
                if (widget.data.listItems!.length > 1) {
                  setState(() => widget.data.listItems!.removeLast());
                }
              }),
              _tableCtrlBtn(
                ordered ? '編號清單✓' : '編號清單',
                ordered
                    ? AppIcons.formatListNumbered
                    : AppIcons.formatListBulleted,
                () => setState(
                  () => widget.data.listOrdered = !widget.data.listOrdered,
                ),
              ),
            ],
          ),
          SizedBox(height: 8.h),
          for (var i = 0; i < items.length; i++)
            Padding(
              padding: EdgeInsets.symmetric(vertical: 4.h),
              child: Row(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  SizedBox(
                    width: ordered ? 22.w : 16.w,
                    child: Text(
                      ordered ? '${i + 1}.' : '•',
                      textAlign: ordered ? TextAlign.right : TextAlign.left,
                      style: TextStyle(
                        fontSize: 14.sp,
                        height: 1.4,
                        color: AppColors.ink500,
                      ),
                    ),
                  ),
                  SizedBox(width: 6.w),
                  Expanded(
                    child: TextFormField(
                      key: ValueKey('listitem-${widget.data.id}-$i'),
                      initialValue: items[i],
                      maxLines: null,
                      style: TextStyle(
                        fontSize: 14.sp,
                        height: 1.4,
                        color: AppColors.textEmphasis,
                      ),
                      decoration: InputDecoration(
                        isDense: true,
                        hintText: '清單項目',
                        hintStyle: TextStyle(
                          fontSize: 14.sp,
                          color: AppColors.gray300,
                        ),
                        border: InputBorder.none,
                        contentPadding: EdgeInsets.zero,
                      ),
                      onChanged: (v) {
                        if (i < widget.data.listItems!.length) {
                          widget.data.listItems![i] = v;
                        }
                      },
                    ),
                  ),
                ],
              ),
            ),
        ],
      ),
    );
  }

  Widget _tableCtrlBtn(String label, IconData icon, VoidCallback onTap) {
    return GestureDetector(
      onTap: onTap,
      child: Container(
        padding: EdgeInsets.symmetric(horizontal: 8.w, vertical: 4.h),
        decoration: BoxDecoration(
          color: AppColors.ink50,
          borderRadius: BorderRadius.circular(6.r),
        ),
        child: Row(
          mainAxisSize: MainAxisSize.min,
          children: [
            Icon(icon, size: 14.sp, color: AppIconColors.secondary),
            SizedBox(width: 2.w),
            Text(
              label,
              style: TextStyle(fontSize: 12.sp, color: AppColors.ink500),
            ),
          ],
        ),
      ),
    );
  }

  Widget _buildImageContent() {
    final data = widget.data;
    return Padding(
      padding: EdgeInsets.symmetric(horizontal: 8.w, vertical: 8.h),
      child: ClipRRect(
        borderRadius: BorderRadius.circular(8.r),
        child: Stack(
          children: [
            if (data.imageBytes != null)
              Image.memory(
                data.imageBytes!,
                width: double.infinity,
                fit: BoxFit.fitWidth,
              )
            else if (data.imageUrl != null && data.imageUrl!.isNotEmpty)
              OptimizedNetworkImage(
                imageUrl: data.imageUrl!,
                scene: ImageScene.descriptionImage,
                width: double.infinity,
                fit: BoxFit.fitWidth,
                errorWidget: _buildImagePlaceholder('載入失敗'),
              )
            else
              _buildImagePlaceholder('無圖片'),
            if (data.isUploading)
              Positioned.fill(
                child: Container(
                  color: AppColors.textDisabled,
                  child: Center(
                    child: SizedBox(
                      width: 28.w,
                      height: 28.w,
                      child: const CircularProgressIndicator(
                        strokeWidth: 3,
                        valueColor: AlwaysStoppedAnimation<Color>(Colors.white),
                      ),
                    ),
                  ),
                ),
              ),
          ],
        ),
      ),
    );
  }

  Widget _buildImagePlaceholder(String label) {
    return Container(
      width: double.infinity,
      height: 120.h,
      color: AppColors.gray100,
      child: Center(
        child: Text(
          label,
          style: TextStyle(color: AppColors.gray300, fontSize: 14.sp),
        ),
      ),
    );
  }
}

/// 攔截表格儲存格的「貼上表格資料」：單次輸入變更引入 tab 或換行時，
/// 視為貼上試算表資料 → 吞掉本次變更、交由 [onGridPasted] 從錨點格展開
/// （issue #1246 對齊 Notion）。一般打字（單字元、無 tab/換行）不受影響。
class _CellPasteExpandFormatter extends TextInputFormatter {
  _CellPasteExpandFormatter({required this.onGridPasted});

  final void Function(List<List<String>> grid) onGridPasted;

  @override
  TextEditingValue formatEditUpdate(
    TextEditingValue oldValue,
    TextEditingValue newValue,
  ) {
    final inserted = newValue.text.length - oldValue.text.length;
    // 只攔「一次插入多字元且含 tab/換行」的貼上；避免誤攔輸入法組字
    if (inserted > 1 &&
        (newValue.text.contains('\t') || newValue.text.contains('\n')) &&
        !oldValue.text.contains('\t') &&
        !oldValue.text.contains('\n')) {
      // 從差異推出貼入的原始片段：以 selection 起點對齊（簡化：整段 newText
      // 拆格。貼上場景 old 幾乎總是空格或短文字，整段解析即符合預期）。
      final lines = newValue.text
          .replaceAll('\r\n', '\n')
          .replaceAll('\r', '\n')
          .split('\n')
          .where((l) => l.isNotEmpty)
          .toList();
      if (lines.isNotEmpty) {
        final grid = lines.map((l) => l.split('\t')).toList();
        // 至少要形成 2 格以上才算表格資料，否則放行原生貼上
        final cellsTotal = grid.fold<int>(0, (a, r) => a + r.length);
        if (cellsTotal >= 2) {
          WidgetsBinding.instance.addPostFrameCallback((_) {
            onGridPasted(grid);
          });
          return oldValue; // 吞掉本次插入，由展開流程填格
        }
      }
    }
    return newValue;
  }
}
