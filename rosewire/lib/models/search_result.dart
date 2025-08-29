// CLIENT/models/search_result.dart
class SearchResult {
  final String fileName;
  final int size; // in bytes
  final String peer;
  final String hash;

  SearchResult(
      {required this.fileName,
      required this.size,
      required this.peer,
      required this.hash});

  String get formattedSize {
    if (size < 1024) return "$size B";
    if (size < 1024 * 1024) return "${(size / 1024).toStringAsFixed(2)} KB";
    if (size < 1024 * 1024 * 1024) {
      return "${(size / (1024 * 1024)).toStringAsFixed(2)} MB";
    }
    if (size < 1024 * 1024 * 1024 * 1024) {
      return "${(size / (1024 * 1024 * 1024)).toStringAsFixed(2)} GB";
    }
    return "${(size / (1024 * 1024 * 1024 * 1024)).toStringAsFixed(2)} TB";
  }
}