import 'dart:async';
import 'package:flutter/material.dart';
import '../../../services/ssh_chat_service.dart';
import '../../../models/search_result.dart'; // <-- CORRECTED IMPORT
import '../rosewire_desktop.dart';

// ** The SearchResult class has been REMOVED from this file **

class SearchPanel extends StatefulWidget {
  const SearchPanel({super.key});

  @override
  State<SearchPanel> createState() => _SearchPanelState();
}

class _SearchPanelState extends State<SearchPanel> {
  final searchController = TextEditingController();

  // Example: Replace with actual search results from your stream/controller as appropriate
  final List<Map<String, String>> searchResults = [
    {
      "title": "Synthwave - Rose.mp3",
      "size": "4.1 MB",
      "type": "MP3",
      "bitrate": "320 kbps",
      "user": "musicfan01",
      "host": "rose.ssh.net",
    },
    {
      "title": "RosePetal.flac",
      "size": "19.7 MB",
      "type": "FLAC",
      "bitrate": "Lossless",
      "user": "audioenthusiast",
      "host": "rose.ssh.net",
    },
    {
      "title": "PinkNoise.wav",
      "size": "11.4 MB",
      "type": "WAV",
      "bitrate": "1411 kbps",
      "user": "sshshare",
      "host": "rosewire.ssh.net",
    },
  ];

  @override
  Widget build(BuildContext context) {
    return Padding(
      padding: const EdgeInsets.symmetric(horizontal: 32, vertical: 18),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Text(
            "Search for music, podcasts, or files",
            style: TextStyle(
              fontSize: 18,
              color: roseWhite,
              fontWeight: FontWeight.w600,
            ),
          ),
          SizedBox(height: 12),
          Row(
            children: [
              Expanded(
                child: TextField(
                  controller: searchController,
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
                    prefixIcon: Icon(Icons.search, color: rosePink),
                    contentPadding: EdgeInsets.symmetric(vertical: 0, horizontal: 16),
                  ),
                  style: TextStyle(color: roseWhite, fontSize: 15),
                  onSubmitted: (text) {
                    // Add your search logic here
                  },
                ),
              ),
              SizedBox(width: 16),
              ElevatedButton.icon(
                icon: Icon(Icons.search),
                label: Text("Search"),
                style: ElevatedButton.styleFrom(
                  backgroundColor: rosePink,
                  foregroundColor: roseWhite,
                  padding: EdgeInsets.symmetric(horizontal: 24, vertical: 14),
                  textStyle: TextStyle(fontSize: 15, fontWeight: FontWeight.bold),
                  shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(10)),
                  elevation: 0,
                ),
                onPressed: () {},
              ),
            ],
          ),
          SizedBox(height: 20),
          Expanded(
            child: ListView.builder(
              itemCount: searchResults.length,
              itemBuilder: (context, idx) {
                final item = searchResults[idx];
                return Card(
                  elevation: 4,
                  margin: EdgeInsets.symmetric(vertical: 8),
                  color: roseGray.withOpacity(0.85),
                  shape: RoundedRectangleBorder(
                    borderRadius: BorderRadius.circular(16),
                    side: BorderSide(
                      color: rosePink.withOpacity(0.2),
                      width: 1.2,
                    ),
                  ),
                  child: ListTile(
                    leading: CircleAvatar(
                      backgroundColor: rosePink,
                      child: Icon(Icons.music_note, color: roseWhite),
                    ),
                    title: Text(item["title"] ?? "", style: TextStyle(color: roseWhite, fontWeight: FontWeight.bold, fontSize: 16)),
                    subtitle: Text(
                      "${item["size"] ?? ""} • ${item["type"] ?? ""} • ${item["bitrate"] ?? ""}",
                      style: TextStyle(color: roseWhite.withOpacity(0.7)),
                    ),
                    trailing: Column(
                      mainAxisAlignment: MainAxisAlignment.center,
                      children: [
                        Text(item["user"] ?? "", style: TextStyle(color: rosePink, fontWeight: FontWeight.bold)),
                        Text(item["host"] ?? "", style: TextStyle(color: roseWhite.withOpacity(0.6))),
                      ],
                    ),
                    onTap: () {},
                  ),
                );
              },
            ),
          ),
        ],
      ),
    );
  }
}