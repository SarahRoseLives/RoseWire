import 'dart:async';
import 'package:flutter/material.dart';
import '../../../services/ssh_chat_service.dart';
import '../../../models/search_result.dart';
import '../../../theme_manager.dart'; // Corrected import

// Helper function to get an icon based on the file extension.
IconData _getIconForFile(String fileName) {
  final extension = fileName.contains('.') ? fileName.split('.').last.toLowerCase() : '';
  switch (extension) {
    // Audio
    case 'mp3':
    case 'wav':
    case 'aac':
    case 'flac':
    case 'ogg':
    case 'm4a':
      return Icons.music_note;
    // Video
    case 'mp4':
    case 'mov':
    case 'avi':
    case 'mkv':
    case 'webm':
      return Icons.movie;
    // Image
    case 'jpg':
    case 'jpeg':
    case 'png':
    case 'gif':
    case 'bmp':
    case 'webp':
      return Icons.image;
    // Archive
    case 'zip':
    case 'rar':
    case '7z':
    case 'tar':
    case 'gz':
      return Icons.archive;
    // Document
    case 'pdf':
      return Icons.picture_as_pdf;
    case 'doc':
    case 'docx':
      return Icons.description; // Generic document icon
    case 'xls':
    case 'xlsx':
      return Icons.grid_on; // Spreadsheet icon
    case 'ppt':
    case 'pptx':
      return Icons.slideshow; // Presentation icon
    // Code/Text
    case 'txt':
    case 'md':
    case 'log':
      return Icons.article;
    case 'json':
    case 'xml':
    case 'html':
    case 'css':
    case 'js':
    case 'dart':
    case 'py':
    case 'java':
    case 'c':
    case 'cpp':
    case 'sh':
      return Icons.code;
    // Default
    default:
      return Icons.insert_drive_file;
  }
}


class SearchPanelMobile extends StatefulWidget {
  final SshChatService chatService;
  const SearchPanelMobile({super.key, required this.chatService});

  @override
  State<SearchPanelMobile> createState() => _SearchPanelMobileState();
}

class _SearchPanelMobileState extends State<SearchPanelMobile> {
  final _searchController = TextEditingController();
  StreamSubscription? _searchSubscription;
  List<SearchResult> _results = [];
  bool _isLoading = true;
  bool _hasSearched = false;

  @override
  void initState() {
    super.initState();
    _searchSubscription = widget.chatService.searchResults.listen((results) {
      if (mounted) {
        setState(() {
          _results = results;
          _isLoading = false;
        });
      }
    });

    widget.chatService.fetchTopFiles();
  }

  @override
  void dispose() {
    _searchController.dispose();
    _searchSubscription?.cancel();
    super.dispose();
  }

  void _performSearch() {
    final query = _searchController.text.trim();
    FocusScope.of(context).unfocus();
    if (query.isEmpty) {
        setState(() {
            _isLoading = true;
            _hasSearched = false;
            _results = [];
        });
        widget.chatService.fetchTopFiles();
        return;
    }
    setState(() {
      _isLoading = true;
      _hasSearched = true;
      _results = [];
    });
    widget.chatService.searchFiles(query);
  }

  void _downloadFile(SearchResult item) {
    widget.chatService.downloadFile(item.fileName, item.size, item.peer);
    ScaffoldMessenger.of(context).showSnackBar(
      SnackBar(
        backgroundColor: Colors.green[600],
        content: Text(
          "Started download: ${item.fileName}",
          style: const TextStyle(color: Colors.white),
        ),
        duration: const Duration(seconds: 3),
      ),
    );
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      backgroundColor: Theme.of(context).scaffoldBackgroundColor,
      body: Column(
        children: [
          _buildSearchBar(),
          Expanded(child: _buildBody()),
        ],
      ),
    );
  }

  Widget _buildSearchBar() {
    final theme = Theme.of(context);
    return Padding(
      padding: const EdgeInsets.all(12.0),
      child: Row(
        children: [
          Expanded(
            child: TextField(
              controller: _searchController,
              decoration: InputDecoration(
                hintText: "Search for files...",
                hintStyle: TextStyle(color: theme.colorScheme.onSurface.withOpacity(0.5)),
                filled: true,
                fillColor: theme.colorScheme.surface,
                prefixIcon: Icon(Icons.search, color: theme.colorScheme.primary),
                border: OutlineInputBorder(
                  borderRadius: BorderRadius.circular(20),
                  borderSide: BorderSide.none,
                ),
                contentPadding: const EdgeInsets.symmetric(vertical: 0, horizontal: 16),
              ),
              style: TextStyle(color: theme.colorScheme.onSurface, fontSize: 15),
              onSubmitted: (_) => _performSearch(),
            ),
          ),
          const SizedBox(width: 8),
          IconButton(
            icon: Icon(Icons.search, color: theme.colorScheme.primary, size: 30),
            onPressed: _performSearch,
          ),
        ],
      ),
    );
  }

  Widget _buildBody() {
    final theme = Theme.of(context);
    if (_isLoading) {
      return const Center(child: CircularProgressIndicator());
    }

    if (_results.isEmpty) {
      return Center(
        child: Text(
          _hasSearched
              ? 'No results found for your query.'
              : 'No shared files available on the network.',
          textAlign: TextAlign.center,
          style: TextStyle(color: theme.colorScheme.onSurface.withOpacity(0.7), fontSize: 16),
        ),
      );
    }

    return ListView.builder(
      padding: const EdgeInsets.symmetric(horizontal: 8.0),
      itemCount: _results.length,
      itemBuilder: (context, idx) {
        final item = _results[idx];
        return Card(
          elevation: 3,
          margin: const EdgeInsets.symmetric(vertical: 6),
          color: theme.colorScheme.surface,
          shape: RoundedRectangleBorder(
            borderRadius: BorderRadius.circular(12),
          ),
          child: ListTile(
            leading: CircleAvatar(
              backgroundColor: theme.colorScheme.primary,
              child: Icon(_getIconForFile(item.fileName), color: theme.colorScheme.onPrimary),
            ),
            title: Text(
              item.fileName,
              style: TextStyle(color: theme.colorScheme.onSurface, fontWeight: FontWeight.bold),
            ),
            subtitle: Text(
              "Size: ${item.formattedSize} • From: ${item.peer}",
              style: TextStyle(color: theme.colorScheme.onSurface.withOpacity(0.7)),
            ),
            trailing: IconButton(
              icon: const Icon(Icons.download_for_offline, color: statusGreen),
              onPressed: () => _downloadFile(item),
              tooltip: "Download File",
            ),
          ),
        );
      },
    );
  }
}