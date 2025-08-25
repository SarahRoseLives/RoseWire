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
  String _serverHost = 'rosewire.rosevines.network'; // Store the current host

  late final SshChatService _chatService = SshChatService();

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

  Future<void> _initializeServicesAfterLogin(String nickname, String keyPath) async {
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
    }
  }

  void _handleLibraryChanged(String folderPath, List<Map<String, dynamic>> files) {
    _chatService.setLibraryPath(folderPath);
    _chatService.shareFiles(files);
    setState(() {
      _libraryPath = folderPath;
    });
  }

  List<Widget> get _tabs {
    // Construct the full user address to pass to the chat panel
    final currentUserAddress = '@${_nickname ?? ''}@$_serverHost';

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
        currentUserAddress: currentUserAddress,
      ),
      NetworkPanelMobile(chatService: _chatService),
      const SettingsPanelMobile(),
      const AboutPanelMobile(),
    ];
  }

  @override
  Widget build(BuildContext context) {
    if (!_loggedIn) {
      return LoginPanelMobile(onLogin: _onLogin);
    }
    return Scaffold(
      appBar: AppBar(
        title: const Text('RoseWire'),
        backgroundColor: Colors.pinkAccent,
        titleTextStyle: const TextStyle(
          color: Colors.white,
          fontSize: 20,
          fontWeight: FontWeight.bold,
        ),
      ),
      body: IndexedStack(
        index: _selectedTab,
        children: _tabs,
      ),
      bottomNavigationBar: BottomNavigationBar(
        currentIndex: _selectedTab,
        onTap: (idx) => setState(() => _selectedTab = idx),
        selectedItemColor: Colors.pinkAccent,
        unselectedItemColor: Colors.grey[400],
        backgroundColor: Colors.grey[900],
        type: BottomNavigationBarType.fixed,
        items: const [
          BottomNavigationBarItem(icon: Icon(Icons.search), label: 'Search'),
          BottomNavigationBarItem(icon: Icon(Icons.swap_vertical_circle), label: 'Transfers'),
          BottomNavigationBarItem(icon: Icon(Icons.library_music), label: 'Library'),
          BottomNavigationBarItem(icon: Icon(Icons.chat), label: 'Chat'),
          BottomNavigationBarItem(icon: Icon(Icons.cloud), label: 'Network'),
          BottomNavigationBarItem(icon: Icon(Icons.settings), label: 'Settings'),
          BottomNavigationBarItem(icon: Icon(Icons.info_outline), label: 'About'),
        ],
      ),
    );
  }
}