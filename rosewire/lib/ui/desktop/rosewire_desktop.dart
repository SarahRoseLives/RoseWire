// CLIENT/ui/desktop/rosewire_desktop.dart
import 'dart:async';
import 'dart:io';
import 'dart:convert';
import 'dart:ui';
import 'package:flutter/material.dart';
import 'package:path_provider/path_provider.dart';
import 'package:shared_preferences/shared_preferences.dart';
import 'package:crypto/crypto.dart';

import '../../services/ssh_chat_service.dart';
import '../../theme_manager.dart';
import 'search/search_panel.dart';
import 'transfers/transfers_panel.dart';
import 'library/library_panel.dart';
import 'chat/chat_panel.dart';
import 'network/network_panel.dart';
import 'settings/settings_panel.dart';
import 'about/about_panel.dart';

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
        // When connection is established (or re-established), share the library.
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

  /// Gets the saved library path from the user's config file.
  Future<String?> _getRestoredLibraryPath() async {
    try {
      final dir = await getApplicationSupportDirectory();
      final configFile = File('${dir.path}/${widget.nickname}_rosewire_library.json');
      if (await configFile.exists()) {
        final config = jsonDecode(await configFile.readAsString());
        return config["folderPath"] as String?;
      }
    } catch (e) {
      print("Could not read library config: $e");
    }
    return null;
  }

  Future<void> _initializeConnection() async {
    // 1. Load server preferences
    final prefs = await SharedPreferences.getInstance();
    _serverHost = prefs.getString('rosewire_server') ?? 'rosewire.rosevines.network';
    _buildPanels(); // Update panels with correct server host

    // 2. Connect to the server FIRST. This creates the internal file service.
    await _sshChatService.connect(
      nickname: widget.nickname,
      keyPath: widget.keyPath,
      host: _serverHost,
    );

    // 3. NOW that the service is connected, restore the library path and set it.
    final restoredPath = await _getRestoredLibraryPath();
    if (restoredPath != null && restoredPath.isNotEmpty) {
      final dir = Directory(restoredPath);
      if (await dir.exists()) {
        final files =
            await dir.list().where((f) => f is File).cast<File>().toList();
        // This single call will update the service's path and the UI state.
        _handleLibraryChanged(restoredPath, files);
      }
    }

    // 4. Trigger initial data fetches now that the connection is live.
    // The initial library share is triggered by the connection status listener.
    _sshChatService.fetchTopFiles();
    _sshChatService.requestStats();

    // 5. Set up the periodic refresh timer.
    _shareRefreshTimer?.cancel();
    _shareRefreshTimer = Timer.periodic(const Duration(minutes: 10), (_) {
      if (mounted && _connectionStatus == ConnectionStatus.connected) {
        print("Refreshing file share to keep it alive...");
        _shareLibraryToServer();
      }
    });
  }


  void _handleLibraryChanged(String folderPath, List<File> files) {
    setState(() {
      _libraryFolder = folderPath;
      _libraryFiles = files;
    });

    _sshChatService.setLibraryPath(folderPath);
    // Share immediately on change, but only if connected.
    if (_connectionStatus == ConnectionStatus.connected) {
      _shareLibraryToServer();
    }
  }

  Future<void> _shareLibraryToServer() async {
    // Do not share if there are no files or if we aren't connected.
    if (_libraryFiles.isEmpty || _connectionStatus != ConnectionStatus.connected) return;

    final filesPayloadFutures = _libraryFiles.map((file) async {
      final name = file.path.split(Platform.pathSeparator).last;
      final size = await file.length();
      final hash = await sha256.bind(file.openRead()).first;
      return {
        "Name": name,
        "Size": size,
        "IsDir": false,
        "Hash": hash.toString(),
      };
    }).toList();

    final filesPayload = await Future.wait(filesPayloadFutures);
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

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final rosePurple = theme.colorScheme.surface.withOpacity(0.5);

    // --- CHANGE: Define contrasting icon color based on theme ---
    final selectedIconColor = theme.colorScheme.primary == Colors.white ? Colors.black : Colors.white;

    final destinations = [
      NavigationRailDestination(
        icon: const Icon(Icons.search),
        selectedIcon: Icon(Icons.search, color: selectedIconColor),
        label: const Text('Search'),
      ),
      NavigationRailDestination(
        icon: const Icon(Icons.swap_vertical_circle),
        selectedIcon:
            Icon(Icons.swap_vertical_circle, color: selectedIconColor),
        label: const Text('Transfers'),
      ),
      NavigationRailDestination(
        icon: const Icon(Icons.library_music),
        selectedIcon:
            Icon(Icons.library_music, color: selectedIconColor),
        label: const Text('Library'),
      ),
      NavigationRailDestination(
        icon: const Icon(Icons.chat),
        selectedIcon: Icon(Icons.chat, color: selectedIconColor),
        label: const Text('Chat'),
      ),
      NavigationRailDestination(
        icon: const Icon(Icons.cloud),
        selectedIcon: Icon(Icons.cloud, color: selectedIconColor),
        label: const Text('Network'),
      ),
      NavigationRailDestination(
        icon: const Icon(Icons.settings),
        selectedIcon: Icon(Icons.settings, color: selectedIconColor),
        label: const Text('Settings'),
      ),
    ];

    return Scaffold(
      body: Row(
        children: [
          NavigationRail(
            backgroundColor: theme.colorScheme.surface.withOpacity(0.5),
            selectedIndex: _selectedPanelIndex.clamp(0, destinations.length - 1),
            onDestinationSelected: (idx) =>
                setState(() => _selectedPanelIndex = idx),
            labelType: NavigationRailLabelType.all,
            leading: Padding(
              padding: const EdgeInsets.all(12.0),
              child: CircleAvatar(
                backgroundColor: theme.colorScheme.primary,
                radius: 22,
                child: Icon(Icons.cable_rounded,
                    color: theme.colorScheme.onPrimary, size: 26),
              ),
            ),
            destinations: destinations,
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
                  colors: [
                    theme.colorScheme.surface,
                    theme.colorScheme.primary.withOpacity(0.1)
                  ],
                  begin: Alignment.topLeft,
                  end: Alignment.bottomRight,
                ),
              ),
              child: Center(
                child: ConstrainedBox(
                  constraints:
                      const BoxConstraints(maxWidth: 850, maxHeight: 600),
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
                                color: statusOrange,
                                padding: const EdgeInsets.symmetric(
                                    horizontal: 16, vertical: 8),
                                child: Text(
                                  _versionWarning!,
                                  textAlign: TextAlign.center,
                                  style: const TextStyle(
                                      color: Colors.white,
                                      fontWeight: FontWeight.bold),
                                ),
                              ),
                            Expanded(
                              child: _panels.isEmpty
                                  ? const Center(child: CircularProgressIndicator())
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
    final theme = Theme.of(context);
    return Container(
      height: 72,
      padding: const EdgeInsets.symmetric(horizontal: 32),
      decoration: BoxDecoration(
        color: Colors.black.withOpacity(0.30),
        border: Border(
          bottom: BorderSide(
            color: theme.colorScheme.surface.withOpacity(0.8),
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
              color: theme.colorScheme.primary,
              letterSpacing: 2,
              fontFamily: 'Segoe UI',
            ),
          ),
          const SizedBox(width: 18),
          Container(
            padding: const EdgeInsets.symmetric(horizontal: 14, vertical: 4),
            decoration: BoxDecoration(
              color: theme.colorScheme.surface.withOpacity(0.2),
              borderRadius: BorderRadius.circular(8),
            ),
            child: Text(
              'powered by SSH',
              style: TextStyle(
                color: theme.colorScheme.onSurface.withOpacity(0.8),
                fontWeight: FontWeight.w600,
                fontSize: 14,
              ),
            ),
          ),
          const Spacer(),
          Text(
            nickname,
            style: TextStyle(
              color: theme.colorScheme.onSurface,
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
    final theme = Theme.of(context);
    IconData icon;
    Color color;
    String text;

    switch (status) {
      case ConnectionStatus.connected:
        icon = Icons.lock;
        color = statusGreen;
        text = "Connected via SSH as $nickname";
        break;
      case ConnectionStatus.reconnecting:
        icon = Icons.sync_problem;
        color = statusOrange;
        text = "Connection lost. Reconnecting...";
        break;
      case ConnectionStatus.disconnected:
        icon = Icons.cloud_off;
        color = statusRed;
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
            color: theme.colorScheme.surface.withOpacity(0.7),
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
              color: theme.colorScheme.onSurface.withOpacity(0.8),
              fontSize: 13,
            ),
          ),
        ],
      ),
    );
  }
}