import 'package:flutter/material.dart';
import '../../../services/ssh_chat_service.dart';

class ChatPanelMobile extends StatelessWidget {
  final SshChatService chatService;
  final String nickname;
  const ChatPanelMobile({super.key, required this.chatService, required this.nickname});

  @override
  Widget build(BuildContext context) {
    return Center(child: Text("ChatPanel (Android UI)"));
  }
}