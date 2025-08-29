// CLIENT/ui/desktop/rosewire_desktop.dart
import 'dart:async';
import 'dart:io';
import 'dart:convert';
import 'dart:ui';
import 'package:flutter/material.dart';
import 'package:path_provider/path_provider.dart';
import 'package:shared_preferences/shared_preferences.dart';

import '../../services/ssh_chat_service.dart';
import 'search/search_panel.dart';
import 'transfers/transfers_panel.dart';
import 'library/library_panel.dart';
import 'chat/chat_panel.dart';
import 'network/network_panel.dart';
import 'settings/settings_panel.dart';
import 'about/about_panel.dart';

// Shared colors
const rosePink = Color(0xFFEA4C89);
const rosePurple = Color(0xFF6C3483);
const roseWhite = Colors.white;
const roseGray = Color(0xFF22232A);
const roseGreen = Color(0xFF26C281);
const roseOrange = Colors.orange;
const roseRed = Colors.redAccent;


class RoseWireDesktop extends StatefulWidget {
  final String nickname;
  final String keyPath;

  const RoseWireDesktop({super.key, required this.nickname, required this.keyPath});

  @override
  State<RoseWireDesktop> createState() => _RoseWireDesktopState();
}

class _RoseWireDesktopState extends State<RoseWireDesktop> {
  int _selectedPanelIndex = 3;

  late final SshChatService _sshChatService;
  String? _libraryFolder;
  List<File> _libraryFiles = [];
  String _serverHost = 'rosewire.rosevines.network';
  String _currentUserAddress = '';
  String? _versionWarning;
  Timer? _shareRefreshTimer;

  ConnectionStatus _connectionStatus = ConnectionStatus.connected;
  StreamSubscription? _connectionStatusSubscription;
  StreamSubscription? _identitySubscription;


  late List<Widget> _panels = [];

  @override
  void initState() {
    super.initState();
    _sshChatService = SshChatService();

    _connectionStatusSubscription = _sshChatService.connectionStatus.listen((status) {
      if (mounted) {
        setState(() {
          _connectionStatus = status;
        });
        if (status == ConnectionStatus.connected) {
          _shareLibraryToServer();
        }
      }
    });

    _identitySubscription = _sshChatService.identity.listen((identity) {
      if (mounted) {
        setState(() {
          _currentUserAddress = identity;
        });
        // Rebuild panels that depend on the address
        _buildPanels();
      }
    });

    _sshChatService.versionStatus.listen((status) {
      if (status.startsWith("Warning:") && mounted) {
        setState(() {
          _versionWarning = status;
        });
      }
    });

    _buildPanels();
    _initializeConnection();
  }

  void _buildPanels() {
    // Use the confirmed address if available, otherwise construct a temporary one.
    final address = _currentUserAddress.isNotEmpty ? _currentUserAddress : '@${widget.nickname}@$_serverHost';
    setState(() {
      _panels = [
        SearchPanel(chatService: _sshChatService),
        TransfersPanel(chatService: _sshChatService),
        LibraryPanel(
          nickname: widget.nickname,
          onLibraryChanged: _handleLibraryChanged,
        ),
        ChatPanel(
          nickname: widget.nickname,
          chatService: _sshChatService,
          currentUserAddress: address,
        ),
        NetworkPanel(chatService: _sshChatService),
        const SettingsPanel(),
        const AboutPanel(),
      ];
    });
  }

  Future<void> _initializeConnection() async {
    final prefs = await SharedPreferences.getInstance();
    _serverHost = prefs.getString('rosewire_server') ?? 'rosewire.rosevines.network';

    _buildPanels();

    await _sshChatService.connect(
      nickname: widget.nickname,
      keyPath: widget.keyPath,
      host: _serverHost,
    );

    // Trigger initial data fetches now that the connection is live.
    _sshChatService.fetchTopFiles();
    _sshChatService.requestStats();

    await _restoreLibraryAndShare();

    _shareRefreshTimer?.cancel();
    _shareRefreshTimer = Timer.periodic(const Duration(minutes: 10), (_) {
      if (mounted && _connectionStatus == ConnectionStatus.connected) {
        print("Refreshing file share to keep it alive...");
        _shareLibraryToServer();
      }
    });
  }

  Future<void> _restoreLibraryAndShare() async {
    try {
      final dir = await getApplicationSupportDirectory();
      final configFile = File('${dir.path}/${widget.nickname}_rosewire_library.json');
      if (await configFile.exists()) {
        final config = jsonDecode(await configFile.readAsString());
        final folderPath = config["folderPath"] as String?;
        if (folderPath != null && folderPath.isNotEmpty) {
          final files = await Directory(folderPath)
              .list()
              .where((f) => f is File)
              .toList();
          _handleLibraryChanged(folderPath, files.cast<File>());
        }
      }
    } catch (e) {
      print("Could not restore library on desktop: $e");
    }
  }

  void _handleLibraryChanged(String folderPath, List<File> files) {
    setState(() {
      _libraryFolder = folderPath;
      _libraryFiles = files;
    });

    _sshChatService.setLibraryPath(folderPath);
    _shareLibraryToServer();
  }

  void _shareLibraryToServer() {
    if (_libraryFiles.isEmpty) return;
    final filesPayload = _libraryFiles.map((file) {
      final name = file.path.split(Platform.pathSeparator).last;
      final size = file.lengthSync();
      return {
        "Name": name,
        "Size": size,
        "IsDir": false,
      };
    }).toList();
    _sshChatService.shareFiles(filesPayload);
    print("Shared ${filesPayload.length} files with the network.");
  }

  @override
  void dispose() {
    _shareRefreshTimer?.cancel();
    _connectionStatusSubscription?.cancel();
    _identitySubscription?.cancel();
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
      icon: Icon(Icons.cloud),
      selectedIcon: Icon(Icons.cloud, color: rosePink),
      label: Text('Network'),
    ),
    NavigationRailDestination(
      icon: Icon(Icons.settings),
      selectedIcon: Icon(Icons.settings, color: rosePink),
      label: Text('Settings'),
    ),
  ];

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      body: Row(
        children: [
          NavigationRail(
            backgroundColor: roseGray.withOpacity(0.95),
            selectedIndex: _selectedPanelIndex.clamp(0, _destinations.length - 1),
            onDestinationSelected: (idx) => setState(() => _selectedPanelIndex = idx),
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
            trailing: Expanded(
              child: Align(
                alignment: Alignment.bottomCenter,
                child: Padding(
                  padding: const EdgeInsets.only(bottom: 20.0),
                  child: IconButton(
                    icon: const Icon(Icons.info_outline),
                    tooltip: "About",
                    onPressed: () => setState(() => _selectedPanelIndex = 6),
                  ),
                ),
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
                  constraints: const BoxConstraints(maxWidth: 850, maxHeight: 600),
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
                              offset: const Offset(0, 8),
                            ),
                          ],
                        ),
                        child: Column(
                          crossAxisAlignment: CrossAxisAlignment.stretch,
                          children: [
                            _RoseWireHeader(
                              nickname: widget.nickname,
                            ),
                            if (_versionWarning != null)
                              Container(
                                color: Colors.orange[800],
                                padding: const EdgeInsets.symmetric(horizontal: 16, vertical: 8),
                                child: Text(
                                  _versionWarning!,
                                  textAlign: TextAlign.center,
                                  style: const TextStyle(color: Colors.white, fontWeight: FontWeight.bold),
                                ),
                              ),
                            Expanded(
                              child: _panels.isEmpty
                                  ? Center(child: CircularProgressIndicator())
                                  : IndexedStack(
                                      index: _selectedPanelIndex,
                                      children: _panels,
                                    ),
                            ),
                            _RoseWireStatusBar(
                              nickname: widget.nickname,
                              status: _connectionStatus,
                            ),
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
          const Text(
            'RoseWire',
            style: TextStyle(
              fontSize: 34,
              fontWeight: FontWeight.bold,
              color: rosePink,
              letterSpacing: 2,
              fontFamily: 'Segoe UI',
            ),
          ),
          const SizedBox(width: 18),
          Container(
            padding: const EdgeInsets.symmetric(horizontal: 14, vertical: 4),
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
          const Spacer(),
          Text(
            nickname,
            style: const TextStyle(
              color: roseWhite,
              fontWeight: FontWeight.bold,
              fontSize: 16,
            ),
          ),
        ],
      ),
    );
  }
}

class _RoseWireStatusBar extends StatelessWidget {
  final String nickname;
  final ConnectionStatus status;

  const _RoseWireStatusBar({required this.nickname, required this.status});

  @override
  Widget build(BuildContext context) {
    IconData icon;
    Color color;
    String text;

    switch (status) {
      case ConnectionStatus.connected:
        icon = Icons.lock;
        color = roseGreen;
        text = "Connected via SSH as $nickname";
        break;
      case ConnectionStatus.reconnecting:
        icon = Icons.sync_problem;
        color = roseOrange;
        text = "Connection lost. Reconnecting...";
        break;
      case ConnectionStatus.disconnected:
        icon = Icons.cloud_off;
        color = roseRed;
        text = "Offline. Could not connect to the server.";
        break;
    }

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
          Icon(icon, size: 16, color: color),
          const SizedBox(width: 8),
          Text(
            text,
            style: TextStyle(
              color: color,
              fontWeight: FontWeight.bold,
              fontSize: 14,
            ),
          ),
          const Spacer(),
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