import 'package:flutter/material.dart';
import '../../../services/ssh_chat_service.dart';

class SearchPanelMobile extends StatelessWidget {
  final SshChatService chatService;
  const SearchPanelMobile({super.key, required this.chatService});

  @override
  Widget build(BuildContext context) {
    return Center(child: Text("SearchPanel (Android UI)"));
  }
}