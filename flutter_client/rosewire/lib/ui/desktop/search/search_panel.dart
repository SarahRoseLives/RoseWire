import 'dart:async';
import 'package:flutter/material.dart';
import '../../../services/ssh_chat_service.dart';
import '../../../models/search_result.dart';
import '../rosewire_desktop.dart';

class SearchPanel extends StatefulWidget {
  final SshChatService chatService;
  const SearchPanel({super.key, required this.chatService});

  @override
  State<SearchPanel> createState() => _SearchPanelState();
}

class _SearchPanelState extends State<SearchPanel> {
  final _searchController = TextEditingController();
  StreamSubscription? _searchSubscription;
  List<SearchResult> _results = [];
  bool _isLoading = false;
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
    if (query.isEmpty) return;
    setState(() {
      _isLoading = true;
      _hasSearched = true;
      _results = [];
    });
    widget.chatService.searchFiles(query);
  }

  void _downloadFile(SearchResult item) {
    widget.chatService.downloadFile(item.fileName, item.size, item.peer);
    ScaffoldMessenger.of(context).showSnackBar(SnackBar(
      content: Text("Started download: ${item.fileName}"),
    ));
  }

  Widget _buildBody() {
    if (_isLoading) {
      return const Center(child: CircularProgressIndicator());
    }
    if (!_hasSearched && _results.isEmpty) {
      return const Center(
        child: Text(
          'Loading top shared files...',
          style: TextStyle(color: roseWhite, fontSize: 16),
        ),
      );
    }
    if (!_hasSearched && _results.isNotEmpty) {
      return ListView.builder(
        itemCount: _results.length,
        itemBuilder: (context, idx) {
          final item = _results[idx];
          return Card(
            elevation: 4,
            margin: const EdgeInsets.symmetric(vertical: 8),
            color: roseGray.withOpacity(0.85),
            shape: RoundedRectangleBorder(
              borderRadius: BorderRadius.circular(16),
              side: BorderSide(
                color: rosePink.withOpacity(0.2),
                width: 1.2,
              ),
            ),
            child: ListTile(
              leading: const CircleAvatar(
                backgroundColor: rosePink,
                child: Icon(Icons.music_note, color: roseWhite),
              ),
              title: Text(item.fileName, style: const TextStyle(color: roseWhite, fontWeight: FontWeight.bold, fontSize: 16)),
              subtitle: Text(
                item.formattedSize,
                style: TextStyle(color: roseWhite.withOpacity(0.7)),
              ),
              trailing: Column(
                mainAxisAlignment: MainAxisAlignment.center,
                children: [
                  Text(item.peer, style: const TextStyle(color: rosePink, fontWeight: FontWeight.bold)),
                  Text("Peer", style: TextStyle(color: roseWhite.withOpacity(0.6), fontSize: 12)),
                ],
              ),
              onTap: () => _downloadFile(item),
            ),
          );
        },
      );
    }
    if (_hasSearched && _results.isEmpty) {
      return const Center(
        child: Text(
          'No results found for your query.',
          style: TextStyle(color: roseWhite, fontSize: 16),
        ),
      );
    }
    return ListView.builder(
      itemCount: _results.length,
      itemBuilder: (context, idx) {
        final item = _results[idx];
        return Card(
          elevation: 4,
          margin: const EdgeInsets.symmetric(vertical: 8),
          color: roseGray.withOpacity(0.85),
          shape: RoundedRectangleBorder(
            borderRadius: BorderRadius.circular(16),
            side: BorderSide(
              color: rosePink.withOpacity(0.2),
              width: 1.2,
            ),
          ),
          child: ListTile(
            leading: const CircleAvatar(
              backgroundColor: rosePink,
              child: Icon(Icons.music_note, color: roseWhite),
            ),
            title: Text(item.fileName, style: const TextStyle(color: roseWhite, fontWeight: FontWeight.bold, fontSize: 16)),
            subtitle: Text(
              item.formattedSize,
              style: TextStyle(color: roseWhite.withOpacity(0.7)),
            ),
            trailing: Column(
              mainAxisAlignment: MainAxisAlignment.center,
              children: [
                Text(item.peer, style: const TextStyle(color: rosePink, fontWeight: FontWeight.bold)),
                Text("Peer", style: TextStyle(color: roseWhite.withOpacity(0.6), fontSize: 12)),
              ],
            ),
            onTap: () => _downloadFile(item),
          ),
        );
      },
    );
  }

  @override
  Widget build(BuildContext context) {
    return Padding(
      padding: const EdgeInsets.symmetric(horizontal: 32, vertical: 18),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          const Text(
            "Search for files on the network",
            style: TextStyle(
              fontSize: 18,
              color: roseWhite,
              fontWeight: FontWeight.w600,
            ),
          ),
          const SizedBox(height: 12),
          Row(
            children: [
              Expanded(
                child: TextField(
                  controller: _searchController,
                  decoration: InputDecoration(
                    hintText: "Type your search...",
                    hintStyle: TextStyle(
                      color: roseWhite.withOpacity(0.4),
                      fontSize: 15,
                    ),
                    filled: true,
                    fillColor: roseGray.withOpacity(0.8),
                    border: OutlineInputBorder(
                      borderRadius: BorderRadius.circular(12),
                      borderSide: BorderSide.none,
                    ),
                    prefixIcon: const Icon(Icons.search, color: rosePink),
                    contentPadding: const EdgeInsets.symmetric(vertical: 0, horizontal: 16),
                  ),
                  style: const TextStyle(color: roseWhite, fontSize: 15),
                  onSubmitted: (_) => _performSearch(),
                ),
              ),
              const SizedBox(width: 16),
              ElevatedButton.icon(
                icon: const Icon(Icons.search),
                label: const Text("Search"),
                style: ElevatedButton.styleFrom(
                  backgroundColor: rosePink,
                  foregroundColor: roseWhite,
                  padding: const EdgeInsets.symmetric(horizontal: 24, vertical: 14),
                  textStyle: const TextStyle(fontSize: 15, fontWeight: FontWeight.bold),
                  shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(10)),
                  elevation: 0,
                ),
                onPressed: _performSearch,
              ),
            ],
          ),
          const SizedBox(height: 20),
          Expanded(child: _buildBody()),
        ],
      ),
    );
  }
}