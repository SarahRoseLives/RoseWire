import 'package:flutter/material.dart';
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

  late final SshChatService _chatService = SshChatService();

  void _onLogin(String nickname, String keyPath) async {
    setState(() {
      _loggedIn = true;
      _nickname = nickname;
      _keyPath = keyPath;
    });
    await _chatService.connect(nickname: nickname, keyPath: keyPath);
  }

  List<Widget> get _tabs => [
        SearchPanelMobile(chatService: _chatService),
        TransfersPanelMobile(chatService: _chatService),
        LibraryPanelMobile(nickname: _nickname ?? '', onLibraryChanged: (a, b) {}),
        ChatPanelMobile(chatService: _chatService, nickname: _nickname ?? ''),
        NetworkPanelMobile(chatService: _chatService),
        SettingsPanelMobile(),
        AboutPanelMobile(),
      ];

  @override
  Widget build(BuildContext context) {
    if (!_loggedIn) {
      return LoginPanelMobile(onLogin: _onLogin);
    }
    return Scaffold(
      appBar: AppBar(
        title: Text('RoseWire'),
        backgroundColor: Colors.pinkAccent,
      ),
      body: _tabs[_selectedTab],
      bottomNavigationBar: BottomNavigationBar(
        currentIndex: _selectedTab,
        onTap: (idx) => setState(() => _selectedTab = idx),
        selectedItemColor: Colors.pinkAccent,
        unselectedItemColor: Colors.grey,
        items: const [
          BottomNavigationBarItem(icon: Icon(Icons.search), label: 'Search'),
          BottomNavigationBarItem(icon: Icon(Icons.swap_vertical_circle), label: 'Transfers'),
          BottomNavigationBarItem(icon: Icon(Icons.library_music), label: 'Library'),
          BottomNavigationBarItem(icon: Icon(Icons.chat), label: 'Chat'),
          BottomNavigationBarItem(icon: Icon(Icons.cloud), label: 'Network'),
          BottomNavigationBarItem(icon: Icon(Icons.settings), label: 'Settings'),
          BottomNavigationBarItem(icon: Icon(Icons.info_outline), label: 'About'),
        ],
        type: BottomNavigationBarType.fixed,
      ),
    );
  }
}