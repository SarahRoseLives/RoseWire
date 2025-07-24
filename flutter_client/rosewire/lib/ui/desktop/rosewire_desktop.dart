import 'dart:io';
import 'dart:ui';
import 'package:flutter/material.dart';
import '../../services/ssh_chat_service.dart';
import 'search/search_panel.dart';
import 'transfers/transfers_panel.dart';
import 'library/library_panel.dart';
import 'chat/chat_panel.dart';
import 'settings/settings_panel.dart';
import 'about/about_panel.dart';

// Shared colors
const rosePink = Color(0xFFEA4C89);
const rosePurple = Color(0xFF6C3483);
const roseWhite = Colors.white;
const roseGray = Color(0xFF22232A);
const roseGreen = Color(0xFF26C281);

class RoseWireDesktop extends StatefulWidget {
  final String nickname;
  final String keyPath;

  const RoseWireDesktop({super.key, required this.nickname, required this.keyPath});

  @override
  State<RoseWireDesktop> createState() => _RoseWireDesktopState();
}

class _RoseWireDesktopState extends State<RoseWireDesktop> {
  int _selectedIndex = 3; // Default to chat panel
  late final SshChatService _sshChatService;
  String? _libraryFolder;
  List<File> _libraryFiles = [];

  late final List<Widget> _panels;

  @override
  void initState() {
    super.initState();
    _sshChatService = SshChatService();
    _sshChatService.connect(
      nickname: widget.nickname,
      keyPath: widget.keyPath,
    );

    _panels = [
      SearchPanel(),
      TransfersPanel(),
      LibraryPanel(
        nickname: widget.nickname,
        onLibraryChanged: _handleLibraryChanged,
      ),
      ChatPanel(
        nickname: widget.nickname,
        chatService: _sshChatService,
      ),
      SettingsPanel(),
      AboutPanel(),
    ];
  }

  void _handleLibraryChanged(String folderPath, List<File> files) {
    _libraryFolder = folderPath;
    _libraryFiles = files;
    _shareLibraryToServer();
  }

  void _shareLibraryToServer() {
    // Format: /share file:size:isDir|file2:size2:isDir2 ...
    final payload = _libraryFiles.map((file) {
      final name = file.path.split(Platform.pathSeparator).last.replaceAll('|', '_');
      final size = file.lengthSync();
      return "$name:$size:false";
    }).join("|");
    final command = "/share $payload";
    _sshChatService.sendMessage(command);
  }

  @override
  void dispose() {
    _sshChatService.dispose();
    super.dispose();
  }

  final List<NavigationRailDestination> _destinations = const [
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
      icon: Icon(Icons.chat),
      selectedIcon: Icon(Icons.chat, color: rosePink),
      label: Text('Chat'),
    ),
    NavigationRailDestination(
      icon: Icon(Icons.settings),
      selectedIcon: Icon(Icons.settings, color: rosePink),
      label: Text('Settings'),
    ),
    NavigationRailDestination(
      icon: Icon(Icons.info_outline),
      selectedIcon: Icon(Icons.info_outline, color: rosePink),
      label: Text('About'),
    ),
  ];

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
            destinations: _destinations,
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
                            _RoseWireHeader(nickname: widget.nickname),
                            Expanded(child: _panels[_selectedIndex]),
                            _RoseWireStatusBar(nickname: widget.nickname),
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
}

class _RoseWireHeader extends StatelessWidget {
  final String nickname;
  const _RoseWireHeader({required this.nickname});
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
          Text(
            nickname,
            style: TextStyle(
              color: roseWhite,
              fontWeight: FontWeight.bold,
              fontSize: 16,
            ),
          ),
          SizedBox(width: 10),
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

class _RoseWireStatusBar extends StatelessWidget {
  final String nickname;
  const _RoseWireStatusBar({required this.nickname});
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
            "Connected via SSH as $nickname",
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