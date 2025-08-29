// CLIENT/ui/app/rosewire_app.dart
import 'dart:async';
import 'dart:convert';
import 'dart:io';
import 'package:flutter/material.dart';
import 'package:path_provider/path_provider.dart';
import 'package:shared_preferences/shared_preferences.dart';
import '../../services/ssh_chat_service.dart';
import 'login_panel.dart';
import 'chat/chat_panel.dart';
import 'library/library_panel.dart';
import 'transfers/transfers_panel.dart';
import 'search/search_panel.dart';
import 'network/network_panel.dart';
import 'settings/settings_panel.dart';
import 'about/about_panel.dart';

class RoseWireAppMobile extends StatefulWidget {
  const RoseWireAppMobile({super.key});

  @override
  State<RoseWireAppMobile> createState() => _RoseWireAppMobileState();
}

class _RoseWireAppMobileState extends State<RoseWireAppMobile> {
  bool _loggedIn = false;
  String? _nickname;
  String? _keyPath;
  int _selectedTab = 3;
  String? _libraryPath;
  String _serverHost = 'rosewire.rosevines.network';
  String _currentUserAddress = '';

  String? _versionWarning;
  StreamSubscription? _versionSubscription;
  StreamSubscription? _identitySubscription;
  Timer? _shareRefreshTimer;

  ConnectionStatus _connectionStatus = ConnectionStatus.connected;
  StreamSubscription? _connectionStatusSubscription;

  late final SshChatService _chatService = SshChatService();

  @override
  void dispose() {
    _versionSubscription?.cancel();
    _identitySubscription?.cancel();
    _shareRefreshTimer?.cancel();
    _connectionStatusSubscription?.cancel();
    _chatService.dispose();
    super.dispose();
  }

  void _onLogin(String nickname, String keyPath) {
    setState(() {
      _loggedIn = true;
      _nickname = nickname;
      _keyPath = keyPath;
    });
    _initializeServicesAfterLogin(nickname, keyPath);
  }

  Future<File> _getLibraryConfigFile(String nickname) async {
    final dir = await getApplicationSupportDirectory();
    return File('${dir.path}/${nickname}_rosewire_library.json');
  }

  Future<void> _reshareLibraryFiles() async {
    if (_libraryPath == null) return;

    final dir = Directory(_libraryPath!);
    if (await dir.exists()) {
      final files = await dir.list().where((f) => f is File).cast<File>().toList();
      final filesPayload = files.map((file) {
        return {
          "Name": file.path.split('/').last,
          "Size": file.lengthSync(),
          "IsDir": false,
        };
      }).toList();
      _chatService.shareFiles(filesPayload);
      print("Shared ${filesPayload.length} files with the network.");
    }
  }

  Future<void> _initializeServicesAfterLogin(String nickname, String keyPath) async {
    _connectionStatusSubscription = _chatService.connectionStatus.listen((status) {
      if (mounted) {
        setState(() {
          _connectionStatus = status;
        });
        if (status == ConnectionStatus.connected) {
          _reshareLibraryFiles();
        }
      }
    });

    _versionSubscription = _chatService.versionStatus.listen((status) {
      if (status.startsWith("Warning:") && mounted) {
        setState(() => _versionWarning = status);
      } else if (mounted) {
        setState(() => _versionWarning = null);
      }
    });

    _identitySubscription = _chatService.identity.listen((identity) {
      if (mounted) {
        setState(() {
          _currentUserAddress = identity;
        });
      }
    });

    final prefs = await SharedPreferences.getInstance();
    _serverHost = prefs.getString('rosewire_server') ?? 'rosewire.rosevines.network';

    String? loadedPath;
    try {
      final configFile = await _getLibraryConfigFile(nickname);
      if (await configFile.exists()) {
        final config = jsonDecode(await configFile.readAsString());
        loadedPath = config["folderPath"] as String?;
      }
    } catch (e) {
      print("Could not restore library path config: $e");
    }

    if (loadedPath == null || loadedPath.isEmpty) {
        final Directory? appDir = await getExternalStorageDirectory();
        if (appDir != null) {
            loadedPath = appDir.path;
        } else {
            print("ERROR: Could not get external storage directory.");
            return;
        }
    }

    await _chatService.setLibraryPath(loadedPath);
    setState(() {
      _libraryPath = loadedPath;
    });

    await _chatService.connect(
      nickname: nickname,
      keyPath: keyPath,
      host: _serverHost,
    );

    await _reshareLibraryFiles();

    _shareRefreshTimer?.cancel();
    _shareRefreshTimer = Timer.periodic(const Duration(minutes: 10), (_) {
      if (_loggedIn && _connectionStatus == ConnectionStatus.connected) {
        print("Refreshing file share to keep it alive...");
        _reshareLibraryFiles();
      }
    });
  }

  void _handleLibraryChanged(String folderPath, List<Map<String, dynamic>> files) {
    _chatService.setLibraryPath(folderPath);
    _chatService.shareFiles(files);
    setState(() {
      _libraryPath = folderPath;
    });
  }

  List<Widget> get _tabs {
    return [
      SearchPanelMobile(chatService: _chatService),
      TransfersPanelMobile(chatService: _chatService),
      LibraryPanelMobile(
        nickname: _nickname ?? '',
        chatService: _chatService,
        onLibraryChanged: _handleLibraryChanged,
        initialPath: _libraryPath,
      ),
      ChatPanelMobile(
        chatService: _chatService,
        nickname: _nickname ?? '',
        currentUserAddress: _currentUserAddress,
      ),
      NetworkPanelMobile(chatService: _chatService),
      const SettingsPanelMobile(),
      const AboutPanelMobile(),
    ];
  }

  PreferredSizeWidget? _buildStatusBanners() {
    final banners = <Widget>[];

    final connectionBanner = _buildConnectionStatusBanner();
    if (connectionBanner != null) {
      banners.add(connectionBanner);
    } else {
      final versionBanner = _buildVersionWarningBanner();
      if (versionBanner != null) {
        banners.add(versionBanner);
      }
    }

    if (banners.isEmpty) return null;

    return PreferredSize(
      preferredSize: Size.fromHeight(banners.length * 48.0),
      child: Column(children: banners),
    );
  }

  Widget? _buildVersionWarningBanner() {
    if (_versionWarning == null) return null;
    return Container(
      height: 48,
      color: Colors.orange[800],
      padding: const EdgeInsets.all(8.0),
      child: Center(
        child: Text(
          _versionWarning!,
          textAlign: TextAlign.center,
          style: const TextStyle(color: Colors.white, fontWeight: FontWeight.bold),
        ),
      ),
    );
  }

  Widget? _buildConnectionStatusBanner() {
    String? text;
    Color? color;
    switch (_connectionStatus) {
      case ConnectionStatus.connected:
        return null;
      case ConnectionStatus.reconnecting:
        text = "Connection lost. Reconnecting...";
        color = Colors.orange[800];
        break;
      case ConnectionStatus.disconnected:
        text = "Offline. Could not connect to server.";
        color = Colors.red[800];
        break;
    }

    return Container(
      height: 48,
      color: color,
      padding: const EdgeInsets.all(8.0),
      child: Center(
        child: Text(
          text,
          textAlign: TextAlign.center,
          style: const TextStyle(color: Colors.white, fontWeight: FontWeight.bold),
        ),
      ),
    );
  }

  @override
  Widget build(BuildContext context) {
    if (!_loggedIn) {
      return LoginPanelMobile(onLogin: _onLogin);
    }
    final theme = Theme.of(context);

    return Scaffold(
      appBar: AppBar(
        title: const Text('RoseWire'),
        backgroundColor: theme.colorScheme.primary,
        titleTextStyle: TextStyle(
          color: theme.colorScheme.onPrimary,
          fontSize: 20,
          fontWeight: FontWeight.bold,
        ),
        bottom: _buildStatusBanners(),
      ),
      body: IndexedStack(
        index: _selectedTab,
        children: _tabs,
      ),
      bottomNavigationBar: BottomNavigationBar(
        currentIndex: _selectedTab,
        onTap: (idx) => setState(() => _selectedTab = idx),
        selectedItemColor: theme.colorScheme.primary,
        unselectedItemColor: Colors.grey[400],
        backgroundColor: theme.colorScheme.surface,
        type: BottomNavigationBarType.fixed,
        items: const [
          BottomNavigationBarItem(icon: Icon(Icons.search), label: 'Search'),
          BottomNavigationBarItem(icon: Icon(Icons.swap_vertical_circle), label: 'Transfers'),
          BottomNavigationBarItem(icon: Icon(Icons.library_music), label: 'Library'),
          BottomNavigationBarItem(icon: Icon(Icons.chat), label: 'Chat'),
          BottomNavigationBarItem(icon: Icon(Icons.cloud), label: 'Network'),
          BottomNavigationBarItem(icon: Icon(Icons.settings), label: 'Settings'),
        ],
      ),
    );
  }
}