import 'dart:ui';
import 'package:flutter/material.dart';

void main() {
  runApp(const RoseWireApp());
}

final rosePink = Color(0xFFEA4C89);
final rosePurple = Color(0xFF6C3483);
final roseWhite = Colors.white;
final roseGray = Color(0xFF22232A);
final roseGreen = Color(0xFF26C281);

class RoseWireApp extends StatelessWidget {
  const RoseWireApp({super.key});

  @override
  Widget build(BuildContext context) {
    return MaterialApp(
      title: 'RoseWire',
      debugShowCheckedModeBanner: false,
      theme: ThemeData(
        brightness: Brightness.dark,
        colorScheme: ColorScheme.dark(
          primary: rosePink,
          secondary: rosePurple,
          background: roseGray,
        ),
        fontFamily: 'Segoe UI',
        useMaterial3: true,
      ),
      home: const RoseWireHome(),
    );
  }
}

class RoseWireHome extends StatefulWidget {
  const RoseWireHome({super.key});

  @override
  State<RoseWireHome> createState() => _RoseWireHomeState();
}

class _RoseWireHomeState extends State<RoseWireHome> {
  int _selectedIndex = 0;
  final searchController = TextEditingController();

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      body: Row(
        children: [
          NavigationRail(
            backgroundColor: roseGray.withOpacity(0.95),
            selectedIndex: _selectedIndex,
            onDestinationSelected: (idx) => setState(() => _selectedIndex = idx),
            labelType: NavigationRailLabelType.all,
            leading: Padding(
              padding: const EdgeInsets.all(12.0),
              child: CircleAvatar(
                backgroundColor: rosePink,
                radius: 22,
                child: Icon(Icons.cable_rounded, color: roseWhite, size: 26),
              ),
            ),
            destinations: [
              NavigationRailDestination(
                icon: Icon(Icons.search),
                selectedIcon: Icon(Icons.search, color: rosePink),
                label: Text('Search'),
              ),
              NavigationRailDestination(
                icon: Icon(Icons.swap_vertical_circle),
                selectedIcon: Icon(Icons.swap_vertical_circle, color: rosePink),
                label: Text('Transfers'),
              ),
              NavigationRailDestination(
                icon: Icon(Icons.library_music),
                selectedIcon: Icon(Icons.library_music, color: rosePink),
                label: Text('Library'),
              ),
              NavigationRailDestination(
                icon: Icon(Icons.lock),
                selectedIcon: Icon(Icons.lock, color: rosePink),
                label: Text('SSH'),
              ),
            ],
            trailing: Padding(
              padding: const EdgeInsets.only(bottom: 16.0),
              child: Column(
                mainAxisSize: MainAxisSize.min,
                children: [
                  Icon(Icons.settings, color: roseWhite.withOpacity(0.5)),
                  SizedBox(height: 12),
                  Icon(Icons.info_outline, color: roseWhite.withOpacity(0.5)),
                ],
              ),
            ),
          ),
          Expanded(
            child: Container(
              decoration: BoxDecoration(
                gradient: LinearGradient(
                  colors: [roseGray, rosePurple.withOpacity(0.3)],
                  begin: Alignment.topLeft,
                  end: Alignment.bottomRight,
                ),
              ),
              child: Center(
                child: ConstrainedBox(
                  constraints: BoxConstraints(maxWidth: 900, maxHeight: 700),
                  child: ClipRRect(
                    borderRadius: BorderRadius.circular(32),
                    child: BackdropFilter(
                      filter: ImageFilter.blur(sigmaX: 16, sigmaY: 16),
                      child: Container(
                        decoration: BoxDecoration(
                          color: Colors.black.withOpacity(0.3),
                          boxShadow: [
                            BoxShadow(
                              color: rosePurple.withOpacity(0.15),
                              blurRadius: 24,
                              offset: Offset(0, 8),
                            ),
                          ],
                        ),
                        child: Column(
                          crossAxisAlignment: CrossAxisAlignment.stretch,
                          children: [
                            RoseWireHeader(),
                            Expanded(child: _buildPanel(_selectedIndex)),
                            RoseWireStatusBar(),
                          ],
                        ),
                      ),
                    ),
                  ),
                ),
              ),
            ),
          ),
        ],
      ),
    );
  }

  Widget _buildPanel(int idx) {
    switch (idx) {
      case 0:
        return _SearchPanel(controller: searchController);
      case 1:
        return _TransfersPanel();
      case 2:
        return _LibraryPanel();
      case 3:
        return _SSHPanel();
      default:
        return Center(child: Text('Unknown panel'));
    }
  }
}

class RoseWireHeader extends StatelessWidget {
  @override
  Widget build(BuildContext context) {
    return Container(
      height: 72,
      padding: const EdgeInsets.symmetric(horizontal: 32),
      decoration: BoxDecoration(
        color: Colors.black.withOpacity(0.30),
        border: Border(
          bottom: BorderSide(
            color: rosePurple.withOpacity(0.6),
            width: 2,
          ),
        ),
      ),
      child: Row(
        children: [
          Text(
            'RoseWire',
            style: TextStyle(
              fontSize: 34,
              fontWeight: FontWeight.bold,
              color: rosePink,
              letterSpacing: 2,
              fontFamily: 'Segoe UI',
            ),
          ),
          SizedBox(width: 18),
          Container(
            padding: EdgeInsets.symmetric(horizontal: 14, vertical: 4),
            decoration: BoxDecoration(
              color: rosePurple.withOpacity(0.2),
              borderRadius: BorderRadius.circular(8),
            ),
            child: Text(
              'powered by SSH',
              style: TextStyle(
                color: roseWhite.withOpacity(0.8),
                fontWeight: FontWeight.w600,
                fontSize: 14,
              ),
            ),
          ),
          Spacer(),
          Icon(Icons.search_rounded, color: roseWhite.withOpacity(0.3)),
          SizedBox(width: 10),
          Icon(Icons.cloud, color: roseWhite.withOpacity(0.3)),
          SizedBox(width: 10),
          Icon(Icons.settings, color: roseWhite.withOpacity(0.3)),
        ],
      ),
    );
  }
}

class RoseWireStatusBar extends StatelessWidget {
  @override
  Widget build(BuildContext context) {
    return Container(
      height: 32,
      padding: const EdgeInsets.symmetric(horizontal: 24),
      decoration: BoxDecoration(
        color: Colors.black.withOpacity(0.30),
        border: Border(
          top: BorderSide(
            color: rosePurple.withOpacity(0.5),
            width: 2,
          ),
        ),
      ),
      child: Row(
        children: [
          Icon(Icons.lock, size: 16, color: roseGreen),
          SizedBox(width: 8),
          Text(
            "Connected via SSH",
            style: TextStyle(
              color: roseGreen,
              fontWeight: FontWeight.bold,
              fontSize: 14,
            ),
          ),
          Spacer(),
          Text(
            "RoseWire 2.0 - Modern Edition",
            style: TextStyle(
              color: roseWhite.withOpacity(0.8),
              fontSize: 13,
            ),
          ),
        ],
      ),
    );
  }
}

class _SearchPanel extends StatefulWidget {
  final TextEditingController controller;
  const _SearchPanel({required this.controller});
  @override
  State<_SearchPanel> createState() => _SearchPanelState();
}

class _SearchPanelState extends State<_SearchPanel> {
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
                  controller: widget.controller,
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
            child: AnimatedList(
              initialItemCount: searchResults.length,
              itemBuilder: (context, idx, animation) {
                final item = searchResults[idx];
                return SlideTransition(
                  position: animation.drive(Tween(
                    begin: Offset(1, 0),
                    end: Offset(0, 0),
                  )),
                  child: Card(
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

class _TransfersPanel extends StatelessWidget {
  final List<Map<String, dynamic>> transfers = [
    {
      "title": "RosePetal.flac",
      "progress": 0.73,
      "speed": "1.2 MB/s",
      "status": "Downloading",
      "user": "audioenthusiast"
    },
    {
      "title": "PinkNoise.wav",
      "progress": 1.0,
      "speed": "Done",
      "status": "Complete",
      "user": "sshshare"
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
            "Active Transfers",
            style: TextStyle(
              fontSize: 18,
              color: roseWhite,
              fontWeight: FontWeight.w600,
            ),
          ),
          SizedBox(height: 18),
          Expanded(
            child: ListView.builder(
              itemCount: transfers.length,
              itemBuilder: (context, idx) {
                final item = transfers[idx];
                return Card(
                  elevation: 3,
                  margin: EdgeInsets.symmetric(vertical: 8),
                  color: roseGray.withOpacity(0.85),
                  shape: RoundedRectangleBorder(
                    borderRadius: BorderRadius.circular(16),
                    side: BorderSide(
                      color: roseGreen.withOpacity(0.2),
                      width: 1.2,
                    ),
                  ),
                  child: ListTile(
                    leading: Icon(
                      item["progress"] == 1.0 ? Icons.check_circle : Icons.downloading,
                      color: item["progress"] == 1.0 ? roseGreen : rosePink,
                    ),
                    title: Text(item["title"] ?? "", style: TextStyle(color: roseWhite, fontWeight: FontWeight.bold)),
                    subtitle: Text("${item["status"] ?? ""} • ${item["speed"] ?? ""}", style: TextStyle(color: roseWhite.withOpacity(0.7))),
                    trailing: SizedBox(
                      width: 120,
                      child: Column(
                        mainAxisAlignment: MainAxisAlignment.center,
                        children: [
                          LinearProgressIndicator(
                            value: item["progress"] as double,
                            color: roseGreen,
                            backgroundColor: rosePink.withOpacity(0.2),
                            minHeight: 8,
                            borderRadius: BorderRadius.circular(8),
                          ),
                          SizedBox(height: 6),
                          Text(item["user"] ?? "", style: TextStyle(color: rosePink, fontWeight: FontWeight.bold)),
                        ],
                      ),
                    ),
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

class _LibraryPanel extends StatelessWidget {
  final List<Map<String, String>> library = [
    {
      "title": "Synthwave - Rose.mp3",
      "artist": "RetroSynth",
      "album": "Neon Nights",
      "length": "3:42",
      "bitrate": "320 kbps",
      "type": "MP3"
    },
    {
      "title": "RosePetal.flac",
      "artist": "ChillBeats",
      "album": "Petals",
      "length": "5:11",
      "bitrate": "Lossless",
      "type": "FLAC"
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
            "My Library",
            style: TextStyle(
              fontSize: 18,
              color: roseWhite,
              fontWeight: FontWeight.w600,
            ),
          ),
          SizedBox(height: 18),
          Expanded(
            child: ListView.builder(
              itemCount: library.length,
              itemBuilder: (context, idx) {
                final item = library[idx];
                return Card(
                  elevation: 3,
                  margin: EdgeInsets.symmetric(vertical: 8),
                  color: roseGray.withOpacity(0.85),
                  shape: RoundedRectangleBorder(
                    borderRadius: BorderRadius.circular(16),
                    side: BorderSide(
                      color: rosePurple.withOpacity(0.2),
                      width: 1.2,
                    ),
                  ),
                  child: ListTile(
                    leading: Icon(Icons.music_note, color: rosePink),
                    title: Text(item["title"] ?? "", style: TextStyle(color: roseWhite, fontWeight: FontWeight.bold)),
                    subtitle: Text(
                      "${item["artist"] ?? ""} • ${item["album"] ?? ""} • ${item["length"] ?? ""} • ${item["bitrate"] ?? ""}",
                      style: TextStyle(color: roseWhite.withOpacity(0.7)),
                    ),
                    trailing: Text(item["type"] ?? "", style: TextStyle(color: rosePink, fontWeight: FontWeight.bold)),
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

class _SSHPanel extends StatelessWidget {
  @override
  Widget build(BuildContext context) {
    return Padding(
      padding: const EdgeInsets.symmetric(horizontal: 32, vertical: 18),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Text(
            "SSH Connection",
            style: TextStyle(
              fontSize: 18,
              color: roseWhite,
              fontWeight: FontWeight.w600,
            ),
          ),
          SizedBox(height: 16),
          Container(
            width: double.infinity,
            height: 120,
            padding: const EdgeInsets.all(14),
            decoration: BoxDecoration(
              color: Colors.black,
              borderRadius: BorderRadius.circular(12),
              border: Border.all(color: rosePurple.withOpacity(0.4), width: 2),
            ),
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text("rose@rosewire:~\$ ssh rosewire", style: TextStyle(fontFamily: "Consolas", color: roseGreen)),
                SizedBox(height: 6),
                Text("Welcome to RoseWire SSH Music Sharing!", style: TextStyle(fontFamily: "Consolas", color: roseWhite)),
                Text("Type \"help\" for a list of commands.", style: TextStyle(fontFamily: "Consolas", color: roseWhite)),
                SizedBox(height: 10),
                Text("rose@rosewire:~\$ ", style: TextStyle(fontFamily: "Consolas", color: roseGreen)),
              ],
            ),
          ),
          SizedBox(height: 18),
          Row(
            children: [
              Text("Host:", style: TextStyle(color: roseWhite, fontSize: 14)),
              SizedBox(width: 8),
              SizedBox(
                width: 160,
                child: TextField(
                  decoration: InputDecoration(
                    filled: true,
                    fillColor: roseGray.withOpacity(0.7),
                    border: OutlineInputBorder(
                      borderRadius: BorderRadius.circular(8),
                      borderSide: BorderSide.none,
                    ),
                    contentPadding: EdgeInsets.symmetric(vertical: 0, horizontal: 10),
                  ),
                  style: TextStyle(color: roseWhite, fontSize: 14),
                ),
              ),
              SizedBox(width: 12),
              Text("Port:", style: TextStyle(color: roseWhite, fontSize: 14)),
              SizedBox(width: 8),
              SizedBox(
                width: 60,
                child: TextField(
                  decoration: InputDecoration(
                    filled: true,
                    fillColor: roseGray.withOpacity(0.7),
                    border: OutlineInputBorder(
                      borderRadius: BorderRadius.circular(8),
                      borderSide: BorderSide.none,
                    ),
                    contentPadding: EdgeInsets.symmetric(vertical: 0, horizontal: 10),
                  ),
                  style: TextStyle(color: roseWhite, fontSize: 14),
                ),
              ),
              SizedBox(width: 12),
              ElevatedButton.icon(
                icon: Icon(Icons.lock_open),
                label: Text("Connect"),
                style: ElevatedButton.styleFrom(
                  backgroundColor: roseGreen,
                  foregroundColor: roseWhite,
                  padding: EdgeInsets.symmetric(horizontal: 18, vertical: 10),
                  shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(8)),
                ),
                onPressed: () {},
              ),
            ],
          ),
          SizedBox(height: 18),
          Row(
            children: [
              Icon(Icons.info_outline_rounded, color: roseWhite.withOpacity(0.7)),
              SizedBox(width: 8),
              Expanded(
                child: Text(
                  "RoseWire uses SSH for secure peer-to-peer music sharing.\nConfigure your SSH settings to connect.",
                  style: TextStyle(
                    fontSize: 13,
                    color: roseWhite.withOpacity(0.8),
                  ),
                ),
              ),
            ],
          ),
        ],
      ),
    );
  }
}