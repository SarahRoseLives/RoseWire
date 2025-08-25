import 'dart:convert';
import 'dart:io';
import 'package:flutter/material.dart';
import 'package:path_provider/path_provider.dart';
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
  int _selectedTab = 3; // Default to Chat
  String? _libraryPath;

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
    String? loadedPath;

    // 1. Try to restore the user's previously selected library path.
    try {
      final configFile = await _getLibraryConfigFile(nickname);
      if (await configFile.exists()) {
        final config = jsonDecode(await configFile.readAsString());
        loadedPath = config["folderPath"] as String?;
      }
    } catch (e) {
      print("Could not restore library path config: $e");
    }

    // --- START FIX ---
    // 2. If no path was restored, create and set a safe, default path.
    if (loadedPath == null || loadedPath.isEmpty) {
        // getExternalStorageDirectory() returns a path like /storage/emulated/0/Android/data/com.example.rosewire/files
        // This directory is guaranteed to be writable without special permissions.
        final Directory? appDir = await getExternalStorageDirectory();
        if (appDir != null) {
            loadedPath = appDir.path;
            print("No saved path found. Using default app directory: $loadedPath");
        } else {
            print("ERROR: Could not get external storage directory.");
            // Handle error case, maybe show a message to the user
            return;
        }
    }
    // --- END FIX ---

    // 3. Set the now-guaranteed-to-be-valid path on the service and in the state.
    await _chatService.setLibraryPath(loadedPath);
    setState(() {
      _libraryPath = loadedPath;
    });
    print("Library path initialized to: $_libraryPath");

    // 4. Connect to the server.
    await _chatService.connect(nickname: nickname, keyPath: keyPath);

    // 5. Automatically share files from the initialized library path.
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
      print("Automatically shared ${filesPayload.length} files from initialized library.");
    }
  }

  void _handleLibraryChanged(String folderPath, List<Map<String, dynamic>> files) {
    _chatService.setLibraryPath(folderPath);
    _chatService.shareFiles(files);
    setState(() {
      _libraryPath = folderPath;
    });
    print("Library changed by user. Path set and ${files.length} files shared.");
  }

  List<Widget> get _tabs => [
        SearchPanelMobile(chatService: _chatService),
        TransfersPanelMobile(chatService: _chatService),
        LibraryPanelMobile(
          nickname: _nickname ?? '',
          chatService: _chatService,
          onLibraryChanged: _handleLibraryChanged,
          initialPath: _libraryPath,
        ),
        ChatPanelMobile(chatService: _chatService, nickname: _nickname ?? ''),
        NetworkPanelMobile(chatService: _chatService),
        const SettingsPanelMobile(),
        const AboutPanelMobile(),
      ];

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
