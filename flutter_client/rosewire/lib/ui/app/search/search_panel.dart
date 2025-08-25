import 'dart:async';
import 'package:flutter/material.dart';
import '../../../services/ssh_chat_service.dart';
import '../../../models/search_result.dart';

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
    // Subscribe to the stream of search results from the service.
    _searchSubscription = widget.chatService.searchResults.listen((results) {
      if (mounted) {
        setState(() {
          _results = results;
          _isLoading = false;
        });
      }
    });

    // Fetch the top files when the panel is first displayed.
    widget.chatService.fetchTopFiles();
  }

  @override
  void dispose() {
    _searchController.dispose();
    _searchSubscription?.cancel();
    super.dispose();
  }

  /// Triggers a file search via the chat service.
  void _performSearch() {
    final query = _searchController.text.trim();
    // Hide keyboard
    FocusScope.of(context).unfocus();
    if (query.isEmpty) {
        // If the search is cleared, fetch the top files again.
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

  /// Initiates a file download and shows a confirmation snackbar.
  void _downloadFile(SearchResult item) {
    widget.chatService.downloadFile(item.fileName, item.size, item.peer);
    // Show a confirmation message to the user.
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
      backgroundColor: Colors.grey[900],
      body: Column(
        children: [
          _buildSearchBar(),
          Expanded(child: _buildBody()),
        ],
      ),
    );
  }

  /// Builds the search input field and button.
  Widget _buildSearchBar() {
    return Padding(
      padding: const EdgeInsets.all(12.0),
      child: Row(
        children: [
          Expanded(
            child: TextField(
              controller: _searchController,
              decoration: InputDecoration(
                hintText: "Search for files...",
                hintStyle: TextStyle(color: Colors.white.withOpacity(0.5)),
                filled: true,
                fillColor: Colors.grey[800],
                prefixIcon: const Icon(Icons.search, color: Colors.pinkAccent),
                border: OutlineInputBorder(
                  borderRadius: BorderRadius.circular(20),
                  borderSide: BorderSide.none,
                ),
                contentPadding: const EdgeInsets.symmetric(vertical: 0, horizontal: 16),
              ),
              style: const TextStyle(color: Colors.white, fontSize: 15),
              onSubmitted: (_) => _performSearch(),
            ),
          ),
          const SizedBox(width: 8),
          IconButton(
            icon: const Icon(Icons.search, color: Colors.pinkAccent, size: 30),
            onPressed: _performSearch,
          ),
        ],
      ),
    );
  }

  /// Builds the main content area: loading indicator, results list, or empty state message.
  Widget _buildBody() {
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
          style: const TextStyle(color: Colors.white70, fontSize: 16),
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
          color: Colors.grey[850],
          shape: RoundedRectangleBorder(
            borderRadius: BorderRadius.circular(12),
          ),
          child: ListTile(
            leading: const CircleAvatar(
              backgroundColor: Colors.pinkAccent,
              child: Icon(Icons.music_note, color: Colors.white),
            ),
            title: Text(
              item.fileName,
              style: const TextStyle(color: Colors.white, fontWeight: FontWeight.bold),
            ),
            subtitle: Text(
              "Size: ${item.formattedSize} • From: ${item.peer}",
              style: TextStyle(color: Colors.white.withOpacity(0.7)),
            ),
            trailing: IconButton(
              icon: const Icon(Icons.download_for_offline, color: Colors.greenAccent),
              onPressed: () => _downloadFile(item),
              tooltip: "Download File",
            ),
          ),
        );
      },
    );
  }
}
