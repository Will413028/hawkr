class ParsedProductDescription {
  const ParsedProductDescription({required this.text, required this.blocks});

  final String text;
  final List<Map<String, dynamic>> blocks;
}

ParsedProductDescription parseProductDescription(dynamic raw) {
  if (raw is String) {
    return ParsedProductDescription(text: raw.trim(), blocks: const []);
  }

  if (raw is! Map) {
    return const ParsedProductDescription(text: '', blocks: []);
  }

  final description = Map<String, dynamic>.from(raw);
  final blocks = <Map<String, dynamic>>[];
  final rawBlocks = description['blocks'];
  if (rawBlocks is List) {
    for (final block in rawBlocks) {
      if (block is Map) {
        blocks.add(Map<String, dynamic>.from(block));
      }
    }
  }

  if (blocks.isNotEmpty) {
    return ParsedProductDescription(
      text: blocks.map(_blockText).where((text) => text.isNotEmpty).join('\n'),
      blocks: blocks,
    );
  }

  return ParsedProductDescription(
    text:
        _normalizeText(description['text']) ??
        _normalizeText(description['content']) ??
        '',
    blocks: const [],
  );
}

String _blockText(Map<String, dynamic> block) {
  return _normalizeText(block['content']) ??
      _normalizeText(block['text']) ??
      '';
}

String? _normalizeText(dynamic raw) {
  final text = (raw ?? '').toString().trim();
  return text.isEmpty ? null : text;
}
