import 'package:hawkr_seller/shared/widgets/hk_button_label.dart';
import 'package:hawkr_seller/shared/icons/app_icon_colors.dart';
import 'package:hawkr_seller/shared/icons/app_icons.dart';
import 'dart:convert';

import 'package:flutter/material.dart';
import 'package:flutter_quill/flutter_quill.dart';
import 'package:flutter_screenutil/flutter_screenutil.dart';
import 'package:hooks_riverpod/hooks_riverpod.dart';
import 'package:image_picker/image_picker.dart';

import '../../core/theme/app_colors.dart';
import '../../features/media/data/media_repository.dart';
import '../../features/product/data/hashtag_extractor.dart';
import 'app_confirm_dialog.dart';
import 'pasted_table_parser.dart';
import 'rich_text_editor_text_block.dart';
import 'rich_text_editor_toolbar.dart';
import 'rich_text_runs_delta.dart';
import '../utils/floating_toast.dart';

/// 商品說明富文本編輯器
/// 線性文件式排列，區塊可拖曳排序
class RichTextEditor extends ConsumerStatefulWidget {
  const RichTextEditor({
    super.key,
    this.initialContent,
    this.initialBlocks,
    this.maxLength = 3000,
    this.onSave,
  });

  final String? initialContent;

  /// 結構化初始區塊（編輯模式回填用）
  /// 格式：[{'type':'text','content':'...'}, {'type':'image','imageUrl':'...','imageKey':'...'}]
  /// （契約：`DescriptionBlockResponse`，gateway/internal/handler/product.go）
  final List<Map<String, dynamic>>? initialBlocks;
  final int maxLength;
  final ValueChanged<RichTextEditorResult>? onSave;

  @override
  ConsumerState<RichTextEditor> createState() => _RichTextEditorState();
}

class _RichTextEditorState extends ConsumerState<RichTextEditor> {
  final List<TextBlockData> _blocks = [];
  final ScrollController _scrollController = ScrollController();

  int? _selectedBlockIndex;

  // Undo/Redo 堆疊
  final List<List<TextBlockData>> _undoStack = [];
  final List<List<TextBlockData>> _redoStack = [];

  @override
  void initState() {
    super.initState();
    if (widget.initialBlocks != null && widget.initialBlocks!.isNotEmpty) {
      // 從結構化區塊回填（編輯模式）
      for (final block in widget.initialBlocks!) {
        final type = block['type']?.toString() ?? 'text';
        if (type == 'image') {
          _blocks.add(
            TextBlockData(
              id: _generateId(),
              text: '',
              type: BlockType.image,
              imageUrl: block['imageUrl']?.toString(),
              objectKey: block['imageKey']?.toString(),
            ),
          );
        } else if (type == 'table') {
          final parsed = _parseTableCells(block['tableJson']);
          if (parsed != null) {
            _blocks.add(
              TextBlockData(
                id: _generateId(),
                text: '',
                type: BlockType.table,
                tableCells: parsed.$1,
                tableWithHeadings: parsed.$2,
              ),
            );
          }
        } else if (type == 'list') {
          final parsed = _parseListItems(block['listJson']);
          if (parsed != null) {
            _blocks.add(
              TextBlockData(
                id: _generateId(),
                text: '',
                type: BlockType.list,
                listItems: parsed.$1,
                listOrdered: parsed.$2,
              ),
            );
          }
        } else {
          final runs = block['runs'];
          final content = (block['content'] ?? block['text'] ?? '').toString();
          final hasRuns = runs is List && runs.isNotEmpty;
          if (!hasRuns && content.isEmpty) continue;
          final styleMap = block['style'] as Map<String, dynamic>? ?? {};
          final style = _parseBlockStyle(styleMap);
          // 新資料走 runs；legacy content 把整塊樣式降為單一 run（不遺失）
          final effectiveRuns = hasRuns
              ? runs
              : [
                  <String, dynamic>{
                    'text': content,
                    if (style.isBold) 'bold': true,
                    if (style.isItalic) 'italic': true,
                    if (style.color != AppColors.textEmphasis)
                      'color': _colorToHex(style.color),
                  },
                ];
          _blocks.add(
            TextBlockData(
              id: _generateId(),
              text: hasRuns ? '' : content,
              style: style,
              quillController: _newTextController(effectiveRuns),
            ),
          );
        }
      }
      // 解析完畢後，為缺少 imageUrl 的圖片區塊取得簽名 URL
      _resolveImageUrls();
    } else if (widget.initialContent != null &&
        widget.initialContent!.isNotEmpty) {
      final content = widget.initialContent!.trim();
      if (content.isNotEmpty &&
          content != '{}' &&
          content != '[]' &&
          content != 'null') {
        _blocks.add(
          TextBlockData(
            id: _generateId(),
            text: widget.initialContent!,
            quillController: _newTextController([
              {'text': widget.initialContent!},
            ]),
          ),
        );
      }
    }
  }

  /// 從 runs 建 QuillController（inline runs 存 controller）。
  QuillController _newTextController(List<dynamic> runs) {
    return QuillController(
      document: Document.fromDelta(RunsDeltaAdapter.runsToDelta(runs)),
      selection: const TextSelection.collapsed(offset: 0),
      // 強制走純文字貼上路徑（不讓外部 HTML 搶先攔截），並攔截貼上偵測表格。
      // flutter_quill 11 的貼上 hook 仍標 @experimental，此為刻意使用。
      config: QuillControllerConfig(
        // ignore: experimental_member_use
        clipboardConfig: QuillClipboardConfig(
          // ignore: experimental_member_use
          enableExternalRichPaste: false,
          // ignore: experimental_member_use
          onPlainTextPaste: _handlePastedTextForTable,
        ),
      ),
    );
  }

  /// 解析 block-level style（fontSize / fontWeight / alignment）；bold/italic/color
  /// 對新資料只是 legacy 承接用（新資料走 run 級）。
  TextBlockStyle _parseBlockStyle(Map<String, dynamic> styleMap) {
    final fontSize = styleMap['fontSize'] is num
        ? (styleMap['fontSize'] as num).toDouble()
        : styleMap['fontSize'] is String
        ? (double.tryParse(
                RegExp(
                      r'(\d+(?:\.\d+)?)',
                    ).firstMatch(styleMap['fontSize'] as String)?.group(1) ??
                    '',
              ) ??
              14.0)
        : 14.0;
    FontWeight fontWeight = FontWeight.w400;
    if (styleMap['fontWeight'] is num) {
      final wVal = (styleMap['fontWeight'] as num).toInt();
      final wIndex = (wVal ~/ 100) - 1;
      if (wIndex >= 0 && wIndex < FontWeight.values.length) {
        fontWeight = FontWeight.values[wIndex];
      }
    }
    final alignStr = styleMap['alignment']?.toString() ?? 'left';
    final alignment = alignStr == 'center'
        ? TextAlign.center
        : alignStr == 'right'
        ? TextAlign.right
        : TextAlign.left;
    Color color = AppColors.textEmphasis;
    final colorHex = styleMap['color']?.toString();
    if (colorHex != null && colorHex.startsWith('#') && colorHex.length == 7) {
      color = Color(int.parse('FF${colorHex.substring(1)}', radix: 16));
    }
    return TextBlockStyle(
      isBold: styleMap['bold'] == true,
      isItalic: styleMap['italic'] == true,
      fontSize: fontSize,
      fontWeight: fontWeight,
      alignment: alignment,
      color: color,
    );
  }

  /// tableJson → (cells, withHeadings)；cell 內只取純文字（賣家 app 表格編輯純文字）。
  (List<List<TableCellData>>, bool)? _parseTableCells(dynamic tableJson) {
    if (tableJson is! String || tableJson.isEmpty) return null;
    try {
      final model = jsonDecode(tableJson) as Map<String, dynamic>;
      final rows = model['rows'];
      if (rows is! List || rows.isEmpty) return null;
      final cells = <List<TableCellData>>[];
      for (final row in rows) {
        if (row is! List) continue;
        final rowCells = <TableCellData>[];
        for (final cell in row) {
          final runs = (cell is Map && cell['runs'] is List)
              ? cell['runs'] as List
              : const [];
          final text = runs
              .whereType<Map>()
              .map((r) => r['text']?.toString() ?? '')
              .join();
          rowCells.add(TableCellData(text));
        }
        cells.add(rowCells);
      }
      if (cells.isEmpty) return null;
      return (cells, model['withHeadings'] == true);
    } catch (_) {
      return null;
    }
  }

  /// listJson → (items, ordered)；item 內只取純文字（賣家 app 清單編輯純文字）。
  (List<String>, bool)? _parseListItems(dynamic listJson) {
    if (listJson is! String || listJson.isEmpty) return null;
    try {
      final model = jsonDecode(listJson) as Map<String, dynamic>;
      final items = model['items'];
      if (items is! List || items.isEmpty) return null;
      final texts = <String>[];
      for (final item in items) {
        final runs = (item is Map && item['runs'] is List)
            ? item['runs'] as List
            : const [];
        final text = runs
            .whereType<Map>()
            .map((r) => r['text']?.toString() ?? '')
            .join();
        texts.add(text);
      }
      if (texts.isEmpty) return null;
      return (texts, model['ordered'] == true);
    } catch (_) {
      return null;
    }
  }

  String _colorToHex(Color c) =>
      '#${c.toARGB32().toRadixString(16).padLeft(8, '0').substring(2).toUpperCase()}';

  /// 為圖片區塊解析可顯示的 CDN URL
  void _resolveImageUrls() {
    for (final block in _blocks) {
      if (block.type != BlockType.image) continue;
      final key = block.objectKey;
      if (key != null && key.isNotEmpty) {
        final url = MediaRepository.resolveUrl(key);
        if (url != null) {
          block.imageUrl = url;
        }
      } else if (block.imageUrl == null ||
          !block.imageUrl!.startsWith('http')) {
        final fallbackKey = block.imageUrl;
        if (fallbackKey != null && fallbackKey.isNotEmpty) {
          final url = MediaRepository.resolveUrl(fallbackKey);
          if (url != null) {
            block.imageUrl = url;
          }
        }
      }
    }
  }

  @override
  void dispose() {
    for (final b in _blocks) {
      b.disposeControllers();
    }
    _scrollController.dispose();
    super.dispose();
  }

  static int _idCounter = 0;
  String _generateId() {
    _idCounter++;
    return '${DateTime.now().millisecondsSinceEpoch}_$_idCounter';
  }

  void _saveState() {
    _undoStack.add(_blocks.map((b) => b.copy()).toList());
    _redoStack.clear();
    if (_undoStack.length > 50) {
      _undoStack.removeAt(0);
    }
  }

  void _undo() {
    if (_undoStack.isEmpty) return;
    _redoStack.add(_blocks.map((b) => b.copy()).toList());
    setState(() {
      _blocks.clear();
      _blocks.addAll(_undoStack.removeLast());
      _selectedBlockIndex = null;
    });
  }

  void _redo() {
    if (_redoStack.isEmpty) return;
    _undoStack.add(_blocks.map((b) => b.copy()).toList());
    setState(() {
      _blocks.clear();
      _blocks.addAll(_redoStack.removeLast());
      _selectedBlockIndex = null;
    });
  }

  int get _totalLength {
    int total = 0;
    for (final block in _blocks) {
      total += block.text.length;
    }
    return total;
  }

  /// 從所有 text block 抽 hashtag、合併 dedup 後的數量（給 top bar 計數器用）。
  /// 內容以 QuillController 即時純文字為準（runs 載入的 block `text` 為空、
  /// 打字也不回寫 text——只讀 text 會恆為 0，issue #1246）。
  int get _totalHashtagCount {
    final buf = StringBuffer();
    for (final b in _blocks) {
      if (b.type != BlockType.text) continue;
      final live = b.quillController?.document.toPlainText() ?? b.text;
      if (live.trim().isEmpty) continue;
      buf.write(live);
      buf.write('\n');
    }
    return HashtagExtractor.countInText(buf.toString());
  }

  /// 建立含純文字內容的 text block（混合貼上分離的敘述行用）。
  TextBlockData _newTextBlock(String text) {
    return TextBlockData(
      id: _generateId(),
      text: text,
      quillController: _newTextController([
        {'text': text},
      ]),
    );
  }

  void _addTextBlock() {
    _saveState();
    setState(() {
      final newBlock = TextBlockData(
        id: _generateId(),
        text: '',
        quillController: _newTextController([]),
      );
      _blocks.add(newBlock);
      _selectedBlockIndex = _blocks.length - 1;
    });
    _scrollToBottom();
  }

  void _insertTable() {
    _saveState();
    setState(() {
      final cells = List.generate(
        2,
        (_) => List.generate(2, (_) => TableCellData('')),
      );
      final block = TextBlockData(
        id: _generateId(),
        text: '',
        type: BlockType.table,
        tableCells: cells,
        tableWithHeadings: true,
      );
      _blocks.add(block);
      _selectedBlockIndex = _blocks.length - 1;
    });
    _scrollToBottom();
  }

  /// 貼上純文字時偵測是否為 Excel / Google Sheet 的 tab 分隔表格。
  ///
  /// 是表格 → 彈確認對話框，確認後插入表格 block 並回傳 `''`（抑制文字貼入
  /// 當前段落）；取消 / 非表格 → 回傳 `null`（走預設純文字貼上）。
  Future<String?> _handlePastedTextForTable(String plainText) async {
    final detected = detectPastedTable(plainText);
    if (detected == null) return null;

    final confirmed = await showAppConfirmDialog(
      context,
      title: '偵測到表格',
      message: '是否將貼上內容插入為表格？\n（表格內文字不套用粗體 / 顏色等樣式）',
      confirmText: '插入表格',
      cancelText: '貼成文字',
      confirmTextColor: AppColors.primary,
    );
    if (!confirmed) return null;

    _insertTableFromMatrix(
      detected.rows,
      leadingText: detected.leadingText,
      trailingText: detected.trailingText,
    );
    if (detected.truncated) {
      _snack('表格超過上限（欄 10 / 列 50 / 單格 200 字）已截斷');
    }
    return '';
  }

  /// 依二維字串矩陣建立表格 block，插在目前聚焦 block 之後。
  /// [leadingText]/[trailingText]：混合內容剪貼簿分離出的敘述行，各自
  /// 成獨立段落插在表格前/後（issue #1246，不吞進表格）。
  void _insertTableFromMatrix(
    List<List<String>> matrix, {
    String leadingText = '',
    String trailingText = '',
  }) {
    _saveState();
    setState(() {
      final cells = matrix
          .map((row) => row.map((t) => TableCellData(t)).toList())
          .toList();
      var at = _selectedBlockIndex == null
          ? _blocks.length
          : (_selectedBlockIndex! + 1).clamp(0, _blocks.length);
      if (leadingText.isNotEmpty) {
        _blocks.insert(at, _newTextBlock(leadingText));
        at++;
      }
      final block = TextBlockData(
        id: _generateId(),
        text: '',
        type: BlockType.table,
        tableCells: cells,
        tableWithHeadings: true,
      );
      _blocks.insert(at, block);
      _selectedBlockIndex = at;
      if (trailingText.isNotEmpty) {
        _blocks.insert(at + 1, _newTextBlock(trailingText));
      }
    });
    _scrollToBottom();
  }

  void _insertList({required bool ordered}) {
    _saveState();
    setState(() {
      final block = TextBlockData(
        id: _generateId(),
        text: '',
        type: BlockType.list,
        listItems: [''],
        listOrdered: ordered,
      );
      _blocks.add(block);
      _selectedBlockIndex = _blocks.length - 1;
    });
    _scrollToBottom();
  }

  void _deleteBlock(int index) {
    if (index < 0 || index >= _blocks.length) return;
    _saveState();
    setState(() {
      _blocks.removeAt(index);
      _selectedBlockIndex = null;
    });
  }

  void _updateBlockText(int index, String text) {
    if (index < 0 || index >= _blocks.length) return;
    // Mutate directly without setState — the child's TextEditingController
    // is the source of truth during editing. Calling setState here would
    // trigger a rebuild that re-syncs the controller in didUpdateWidget,
    // causing setState-during-build errors.
    _blocks[index].text = text;
  }

  void _updateBlockStyle(int index, TextBlockStyle style) {
    if (index < 0 || index >= _blocks.length) return;
    _saveState();
    setState(() {
      _blocks[index].style = style;
    });
  }

  /// 目前聚焦的文字 block 的 QuillController（inline 格式套用對象）。
  QuillController? get _focusedTextController {
    final i = _selectedBlockIndex;
    if (i == null || i < 0 || i >= _blocks.length) return null;
    final b = _blocks[i];
    return b.type == BlockType.text ? b.quillController : null;
  }

  bool _selectionHasAttribute(Attribute attribute) {
    return _focusedTextController?.getSelectionStyle().attributes.containsKey(
          attribute.key,
        ) ??
        false;
  }

  /// 對選取範圍 toggle 行內屬性（粗體/斜體/底線）。
  void _toggleInlineAttr(Attribute attr) {
    final qc = _focusedTextController;
    if (qc == null) return;
    final has = qc.getSelectionStyle().attributes.containsKey(attr.key);
    qc.formatSelection(has ? Attribute.clone(attr, null) : attr);
    setState(() {});
  }

  /// 對選取範圍套用行內顏色（hex = '#RRGGBB'）。
  void _applyInlineColor(String hex) {
    _focusedTextController?.formatSelection(ColorAttribute(hex));
    setState(() {});
  }

  /// 對選取範圍套用 / 移除行內連結。
  Future<void> _applyInlineLink() async {
    final qc = _focusedTextController;
    if (qc == null) return;
    if (qc.selection.isCollapsed) {
      _snack('請先選取要加連結的文字');
      return;
    }
    final url = await _promptUrl();
    if (url == null) return;
    if (url.isEmpty) {
      qc.formatSelection(Attribute.clone(Attribute.link, null));
    } else if (RegExp(r'^https?://', caseSensitive: false).hasMatch(url)) {
      qc.formatSelection(LinkAttribute(url));
    } else {
      _snack('僅接受 http:// 或 https:// 開頭的網址');
    }
  }

  Future<String?> _promptUrl() async {
    final controller = TextEditingController();
    final result = await showDialog<String>(
      context: context,
      builder: (ctx) => AlertDialog(
        title: const Text('連結網址'),
        content: TextField(
          controller: controller,
          autofocus: true,
          keyboardType: TextInputType.url,
          decoration: const InputDecoration(hintText: 'https://...（留空移除連結）'),
        ),
        actions: [
          TextButton(
            onPressed: () => Navigator.pop(ctx),
            child: const HkButtonLabel('取消'),
          ),
          TextButton(
            onPressed: () => Navigator.pop(ctx, controller.text.trim()),
            child: const HkButtonLabel('確定'),
          ),
        ],
      ),
    );
    controller.dispose();
    return result;
  }

  void _snack(String msg) {
    if (!mounted) return;
    // 站內 design system 統一走 FloatingToast（原生 SnackBar 樣式不符，
    // issue #1246 錯誤提示規範）
    FloatingToast.show(context, msg, isSuccess: false);
  }

  void _onReorder(int oldIndex, int newIndex) {
    _saveState();
    setState(() {
      final block = _blocks.removeAt(oldIndex);
      _blocks.insert(newIndex, block);
      // 更新選中索引
      if (_selectedBlockIndex == oldIndex) {
        _selectedBlockIndex = newIndex;
      } else if (_selectedBlockIndex != null) {
        if (oldIndex < _selectedBlockIndex! &&
            newIndex >= _selectedBlockIndex!) {
          _selectedBlockIndex = _selectedBlockIndex! - 1;
        } else if (oldIndex > _selectedBlockIndex! &&
            newIndex <= _selectedBlockIndex!) {
          _selectedBlockIndex = _selectedBlockIndex! + 1;
        }
      }
    });
  }

  Future<void> _insertImage() async {
    final picker = ImagePicker();
    // 描述圖不壓縮：保留原始解析度，避免商品故事圖 / 內嵌文字變糊（#1248）。
    final XFile? image = await picker.pickImage(source: ImageSource.gallery);
    if (image == null) return;

    final bytes = await image.readAsBytes();

    _saveState();
    final blockId = _generateId();
    final newBlock = TextBlockData(
      id: blockId,
      text: '',
      type: BlockType.image,
      imageBytes: bytes,
      isUploading: true,
    );

    setState(() {
      _blocks.add(newBlock);
      _selectedBlockIndex = _blocks.length - 1;
    });

    _scrollToBottom();

    // 後台上傳
    try {
      final ts = DateTime.now().millisecondsSinceEpoch;
      final result = await ref
          .read(mediaRepositoryProvider)
          .uploadWithSession(
            bytes: bytes,
            filename: 'desc_$ts.jpg',
            compress: false,
          );

      if (!mounted) return;
      final idx = _blocks.indexWhere((b) => b.id == blockId);
      if (idx < 0) return;

      setState(() {
        _blocks[idx].isUploading = false;
        _blocks[idx].imageUrl = result.url.isNotEmpty
            ? result.url
            : result.fileUrl;
        _blocks[idx].objectKey = result.objectKey;
        _blocks[idx].stagedToken = result.stagedToken;
        debugPrint(
          '[Editor] Image uploaded: ${result.objectKey}, stagedToken=${result.stagedToken != null}',
        );
      });
    } catch (e) {
      debugPrint('[Editor] Image upload failed: $e');
      if (!mounted) return;
      final idx = _blocks.indexWhere((b) => b.id == blockId);
      if (idx >= 0) {
        setState(() {
          // 移除失敗的 block：本地 bytes 會讓圖「看起來已插入」，但無
          // stagedToken/objectKey 存檔時會被靜默丟棄——假象比失敗更糟
          //（issue #1246 錯誤提示規範）。
          _blocks.removeAt(idx);
          if (_selectedBlockIndex != null &&
              _selectedBlockIndex! >= _blocks.length) {
            _selectedBlockIndex = _blocks.isEmpty ? null : _blocks.length - 1;
          }
        });
      }
      _snack('圖片上傳失敗，請確認網路後再試');
    }
  }

  void _scrollToBottom() {
    WidgetsBinding.instance.addPostFrameCallback((_) {
      if (_scrollController.hasClients) {
        _scrollController.animateTo(
          _scrollController.position.maxScrollExtent,
          duration: const Duration(milliseconds: 300),
          curve: Curves.easeOut,
        );
      }
    });
  }

  void _onCanvasTap() {
    setState(() {
      _selectedBlockIndex = null;
    });
    FocusScope.of(context).unfocus();
  }

  RichTextEditorResult _exportResult() {
    // 純文字預覽（用於摘要顯示）
    final content = _blocks
        .map((b) {
          if (b.type == BlockType.image) {
            final url = b.imageUrl ?? b.objectKey ?? '';
            return url.isNotEmpty ? '![image]($url)' : '';
          }
          return b.text;
        })
        .where((s) => s.isNotEmpty)
        .join('\n\n');

    // 結構化區塊（供 API description 欄位使用）
    final blocks = <Map<String, dynamic>>[];
    for (final b in _blocks) {
      if (b.type == BlockType.image) {
        if (b.stagedToken != null || b.objectKey != null) {
          // 寫入契約：`DescriptionBlockInput`（gateway/internal/handler/seller_product.go）
          // 的圖片鍵是 `imageKey`（原 `objectKey`）。
          blocks.add({
            'type': 'image',
            if (b.stagedToken != null) 'stagedToken': b.stagedToken,
            if (b.objectKey != null) 'imageKey': b.objectKey,
          });
        }
      } else if (b.type == BlockType.table) {
        final tableJson = _tableCellsToJson(b);
        if (tableJson != null) {
          blocks.add({'type': 'table', 'tableJson': tableJson});
        }
      } else if (b.type == BlockType.list) {
        final listJson = _listItemsToJson(b);
        if (listJson != null) {
          blocks.add({'type': 'list', 'listJson': listJson});
        }
      } else {
        // text block：inline runs 從 QuillController 讀取（deltaToRuns）
        final qc = b.quillController;
        if (qc == null) continue;
        final runs = RunsDeltaAdapter.deltaToRuns(qc.document.toDelta());
        if (runs.isEmpty) continue;
        final s = b.style;
        // block-level style 只保留 fontSize(標題,px 字串) / fontWeight / alignment；
        // bold/italic/color/連結為 run 級（已在 runs 內）
        final styleMap = <String, dynamic>{
          if (s.fontSize != 14.0) 'fontSize': '${s.fontSize.toInt()}px',
          if (s.fontWeight != FontWeight.w400) 'fontWeight': s.fontWeight.value,
          if (s.alignment != TextAlign.left) 'alignment': s.alignment.name,
        };
        blocks.add({
          'type': 'text',
          'runs': runs,
          if (styleMap.isNotEmpty) 'style': styleMap,
        });
      }
    }

    // 所有圖片的 stagedToken
    final stagedTokens = _blocks
        .where((b) => b.type == BlockType.image && b.stagedToken != null)
        .map((b) => b.stagedToken!)
        .toList();

    return RichTextEditorResult(
      content: content,
      blocks: blocks,
      stagedTokens: stagedTokens,
    );
  }

  /// 表格 cells → tableJson（cell 純文字 → 單一 run）。
  String? _tableCellsToJson(TextBlockData b) {
    final cells = b.tableCells;
    if (cells == null || cells.isEmpty) return null;
    final rows = cells
        .map(
          (row) => row
              .map(
                (c) => {
                  'runs': c.text.isEmpty
                      ? <Map<String, dynamic>>[]
                      : [
                          {'text': c.text},
                        ],
                },
              )
              .toList(),
        )
        .toList();
    return jsonEncode({'withHeadings': b.tableWithHeadings, 'rows': rows});
  }

  /// 清單 items → listJson（item 純文字 → 單一 run；空 item 丟棄，全空回 null）。
  String? _listItemsToJson(TextBlockData b) {
    final items = b.listItems;
    if (items == null) return null;
    final nonEmpty = items.where((t) => t.trim().isNotEmpty).toList();
    if (nonEmpty.isEmpty) return null;
    final jsonItems = nonEmpty
        .map(
          (t) => {
            'runs': [
              {'text': t},
            ],
          },
        )
        .toList();
    return jsonEncode({'ordered': b.listOrdered, 'items': jsonItems});
  }

  void _onSave() {
    // ── Debug: 記錄每個 block 的完整資料 ──
    debugPrint('╔══════════════════════════════════════════════════');
    debugPrint('║ [RichTextEditor] 儲存 — 共 ${_blocks.length} 個區塊');
    debugPrint('╠══════════════════════════════════════════════════');
    for (int i = 0; i < _blocks.length; i++) {
      final b = _blocks[i];
      if (b.type == BlockType.image) {
        debugPrint('║ [$i] IMAGE');
        debugPrint('║     url: ${b.imageUrl ?? "(null)"}');
        debugPrint('║     objectKey: ${b.objectKey ?? "(null)"}');
        debugPrint('║     stagedToken: ${b.stagedToken != null ? "✓" : "✗"}');
        debugPrint('║     uploading: ${b.isUploading}');
      } else {
        final s = b.style;
        debugPrint(
          '║ [$i] TEXT: "${b.text.length > 50 ? '${b.text.substring(0, 50)}...' : b.text}"',
        );
        debugPrint('║     bold: ${s.isBold}, italic: ${s.isItalic}');
        debugPrint(
          '║     fontSize: ${s.fontSize}, fontWeight: ${s.fontWeight}',
        );
        debugPrint(
          '║     color: #${s.color.toARGB32().toRadixString(16).padLeft(8, '0').toUpperCase()}',
        );
        debugPrint('║     alignment: ${s.alignment.name}');
      }
    }
    debugPrint('╠══════════════════════════════════════════════════');

    final result = _exportResult();

    debugPrint(
      '║ [Export] content preview: "${result.content.length > 80 ? '${result.content.substring(0, 80)}...' : result.content}"',
    );
    debugPrint('║ [Export] blocks JSON:');
    for (int i = 0; i < result.blocks.length; i++) {
      debugPrint('║   [$i] ${result.blocks[i]}');
    }
    debugPrint('║ [Export] stagedTokens: ${result.stagedTokens.length} 個');
    debugPrint('╚══════════════════════════════════════════════════');

    widget.onSave?.call(result);
    Navigator.of(context).pop(result);
  }

  @override
  Widget build(BuildContext context) {
    final topPadding = MediaQuery.of(context).padding.top;
    final bottomPadding = MediaQuery.of(context).viewInsets.bottom;

    // QuillEditor 需要 FlutterQuillLocalizations；用 Localizations.override 在
    // 編輯器子樹局部注入（繼承 app 既有 delegates + 加 Quill），免動 MaterialApp。
    return Localizations.override(
      context: context,
      delegates: const [FlutterQuillLocalizations.delegate],
      child: Scaffold(
        backgroundColor: AppColors.bgPrimary,
        resizeToAvoidBottomInset: false,
        body: Column(
          children: [
            // Status bar
            Container(height: topPadding, color: AppColors.primary),
            // Top bar
            Container(
              height: 56.h,
              padding: EdgeInsets.symmetric(horizontal: 16.w),
              decoration: const BoxDecoration(color: AppColors.primary),
              child: Row(
                mainAxisAlignment: MainAxisAlignment.spaceBetween,
                children: [
                  GestureDetector(
                    onTap: _onSave,
                    child: Icon(
                      AppIcons.chevronLeft,
                      color: AppIconColors.onBrand,
                      size: 28.sp,
                    ),
                  ),
                  Row(
                    mainAxisSize: MainAxisSize.min,
                    children: [
                      Text(
                        '商品說明',
                        style: TextStyle(
                          fontSize: 18.sp,
                          fontWeight: FontWeight.w700,
                          color: Colors.white,
                        ),
                      ),
                      SizedBox(width: 12.w),
                      // hashtag 計數器（# n/10）；達上限變橘
                      Builder(
                        builder: (_) {
                          final n = _totalHashtagCount;
                          final maxN = HashtagExtractor.maxCount;
                          final hit = n >= maxN;
                          return Container(
                            padding: EdgeInsets.symmetric(
                              horizontal: 8.w,
                              vertical: 4.h,
                            ),
                            decoration: BoxDecoration(
                              color: hit
                                  ? const Color(0x33FFFFFF)
                                  : const Color(0x1AFFFFFF),
                              borderRadius: BorderRadius.circular(12.r),
                            ),
                            child: Text(
                              '# $n/$maxN',
                              style: TextStyle(
                                fontSize: 12.sp,
                                fontWeight: FontWeight.w700,
                                color: Colors.white,
                              ),
                            ),
                          );
                        },
                      ),
                    ],
                  ),
                  GestureDetector(
                    onTap: _onSave,
                    child: Text(
                      '儲存',
                      style: TextStyle(
                        fontSize: 16.sp,
                        fontWeight: FontWeight.w700,
                        color: Colors.white,
                      ),
                    ),
                  ),
                ],
              ),
            ),
            // 文件區域
            Expanded(
              child: GestureDetector(
                onTap: _onCanvasTap,
                behavior: HitTestBehavior.opaque,
                child: _blocks.isEmpty ? _buildEmptyState() : _buildBlockList(),
              ),
            ),
            // Toolbar
            EditorToolbar(
              onAddBlock: _addTextBlock,
              onBold: _focusedTextController != null
                  ? () => _toggleInlineAttr(Attribute.bold)
                  : null,
              onItalic: _focusedTextController != null
                  ? () => _toggleInlineAttr(Attribute.italic)
                  : null,
              onUnderline: _focusedTextController != null
                  ? () => _toggleInlineAttr(Attribute.underline)
                  : null,
              onLink: _focusedTextController != null ? _applyInlineLink : null,
              onColorPicker: _focusedTextController != null
                  ? () => _showColorPicker(_selectedBlockIndex!)
                  : null,
              onAlignLeft: _selectedBlockIndex != null
                  ? () => _updateBlockStyle(
                      _selectedBlockIndex!,
                      _blocks[_selectedBlockIndex!].style.copyWith(
                        alignment: TextAlign.left,
                      ),
                    )
                  : null,
              onAlignCenter: _selectedBlockIndex != null
                  ? () => _updateBlockStyle(
                      _selectedBlockIndex!,
                      _blocks[_selectedBlockIndex!].style.copyWith(
                        alignment: TextAlign.center,
                      ),
                    )
                  : null,
              onAlignRight: _selectedBlockIndex != null
                  ? () => _updateBlockStyle(
                      _selectedBlockIndex!,
                      _blocks[_selectedBlockIndex!].style.copyWith(
                        alignment: TextAlign.right,
                      ),
                    )
                  : null,
              onTextStyle: _selectedBlockIndex != null
                  ? () => _showTextStylePicker(_selectedBlockIndex!)
                  : null,
              onInsertImage: _insertImage,
              onInsertTable: _insertTable,
              onInsertBulletList: () => _insertList(ordered: false),
              onInsertNumberedList: () => _insertList(ordered: true),
              onUndo: _undoStack.isNotEmpty ? _undo : null,
              onRedo: _redoStack.isNotEmpty ? _redo : null,
              isBoldActive: _selectionHasAttribute(Attribute.bold),
              isItalicActive: _selectionHasAttribute(Attribute.italic),
              isUnderlineActive: _selectionHasAttribute(Attribute.underline),
              isLinkActive: _selectionHasAttribute(Attribute.link),
              isColorActive: _selectionHasAttribute(Attribute.color),
              currentLength: _totalLength,
              maxLength: widget.maxLength,
              selectedStyle: _selectedBlockIndex != null
                  ? _blocks[_selectedBlockIndex!].style
                  : null,
              bottomPadding: bottomPadding,
            ),
          ],
        ),
      ),
    );
  }

  Widget _buildEmptyState() {
    return Center(
      child: Column(
        mainAxisSize: MainAxisSize.min,
        children: [
          Icon(AppIcons.textFields, size: 48.sp, color: AppIconColors.disabled),
          SizedBox(height: 12.h),
          Text(
            '點擊下方「+ 區塊」新增文字',
            style: TextStyle(fontSize: 14.sp, color: AppColors.gray300),
          ),
        ],
      ),
    );
  }

  Widget _buildBlockList() {
    return ReorderableListView.builder(
      scrollController: _scrollController,
      padding: EdgeInsets.fromLTRB(12.w, 16.h, 12.w, 16.h),
      itemCount: _blocks.length,
      onReorderItem: _onReorder,
      proxyDecorator: (child, index, animation) {
        return Material(
          elevation: 4,
          color: Colors.transparent,
          borderRadius: BorderRadius.circular(12.r),
          child: child,
        );
      },
      itemBuilder: (context, index) {
        final block = _blocks[index];
        final isFirst = index == 0;
        final isLast = index == _blocks.length - 1;

        return Container(
          key: ValueKey(block.id),
          decoration: BoxDecoration(
            color: Colors.white,
            borderRadius: BorderRadius.vertical(
              top: isFirst ? Radius.circular(12.r) : Radius.zero,
              bottom: isLast ? Radius.circular(12.r) : Radius.zero,
            ),
          ),
          child: TextBlockWidget(
            data: block,
            index: index,
            isSelected: _selectedBlockIndex == index,
            onTap: () {
              setState(() => _selectedBlockIndex = index);
            },
            onTextChanged: (text) => _updateBlockText(index, text),
            onDelete: () => _deleteBlock(index),
          ),
        );
      },
    );
  }

  void _showColorPicker(int index) {
    final colors = [
      AppColors.textEmphasis,
      AppColors.textPrimary,
      AppColors.primary,
      AppColors.dangerStrong,
      const Color(0xFF64B5F6),
      const Color(0xFFFFB74D),
      const Color(0xFF81C784),
      const Color(0xFFBA68C8),
    ];

    showModalBottomSheet(
      context: context,
      backgroundColor: Colors.white,
      shape: RoundedRectangleBorder(
        borderRadius: BorderRadius.vertical(top: Radius.circular(16.r)),
      ),
      builder: (ctx) => Container(
        padding: EdgeInsets.all(20.w),
        child: Column(
          mainAxisSize: MainAxisSize.min,
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Text(
              '選擇文字顏色',
              style: TextStyle(
                fontSize: 16.sp,
                fontWeight: FontWeight.w700,
                color: AppColors.textEmphasis,
              ),
            ),
            SizedBox(height: 16.h),
            Wrap(
              spacing: 12.w,
              runSpacing: 12.w,
              children: colors.map((color) {
                final isSelected = _blocks[index].style.color == color;
                return GestureDetector(
                  onTap: () {
                    _applyInlineColor(_colorToHex(color));
                    Navigator.pop(ctx);
                  },
                  child: Container(
                    width: 44.w,
                    height: 44.w,
                    decoration: BoxDecoration(
                      color: color,
                      shape: BoxShape.circle,
                      border: isSelected
                          ? Border.all(color: AppColors.primary, width: 3)
                          : null,
                      boxShadow: [
                        BoxShadow(
                          color: color.withValues(alpha: 0.3),
                          blurRadius: 4.r,
                          offset: Offset(0, 2.h),
                        ),
                      ],
                    ),
                    child: isSelected
                        ? Icon(
                            AppIcons.check,
                            color: AppIconColors.onBrand,
                            size: 20.sp,
                          )
                        : null,
                  ),
                );
              }).toList(),
            ),
            SizedBox(height: MediaQuery.of(context).padding.bottom + 16.h),
          ],
        ),
      ),
    );
  }

  void _showTextStylePicker(int index) {
    showModalBottomSheet(
      context: context,
      backgroundColor: Colors.white,
      shape: RoundedRectangleBorder(
        borderRadius: BorderRadius.vertical(top: Radius.circular(16.r)),
      ),
      builder: (ctx) => Container(
        padding: EdgeInsets.all(20.w),
        child: Column(
          mainAxisSize: MainAxisSize.min,
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Text(
              '選擇文字樣式',
              style: TextStyle(
                fontSize: 16.sp,
                fontWeight: FontWeight.w700,
                color: AppColors.textEmphasis,
              ),
            ),
            SizedBox(height: 16.h),
            _buildStyleOption(ctx, index, '標題', 24.0, FontWeight.w700),
            _buildStyleOption(ctx, index, '副標題', 18.0, FontWeight.w700),
            _buildStyleOption(ctx, index, '內文', 14.0, FontWeight.w400),
            _buildStyleOption(ctx, index, '小字', 12.0, FontWeight.w400),
            SizedBox(height: MediaQuery.of(context).padding.bottom + 16.h),
          ],
        ),
      ),
    );
  }

  Widget _buildStyleOption(
    BuildContext ctx,
    int index,
    String label,
    double fontSize,
    FontWeight fontWeight,
  ) {
    final isSelected = _blocks[index].style.fontSize == fontSize;
    return GestureDetector(
      onTap: () {
        _updateBlockStyle(
          index,
          _blocks[index].style.copyWith(
            fontSize: fontSize,
            fontWeight: fontWeight,
          ),
        );
        Navigator.pop(ctx);
      },
      child: Container(
        width: double.infinity,
        padding: EdgeInsets.symmetric(vertical: 12.h, horizontal: 16.w),
        margin: EdgeInsets.only(bottom: 8.h),
        decoration: BoxDecoration(
          color: isSelected ? const Color(0x1AB8441F) : AppColors.gray100,
          borderRadius: BorderRadius.circular(8.r),
          border: isSelected ? Border.all(color: AppColors.primary) : null,
        ),
        child: Text(
          label,
          style: TextStyle(
            fontSize: fontSize,
            fontWeight: fontWeight,
            color: AppColors.textEmphasis,
          ),
        ),
      ),
    );
  }
}
