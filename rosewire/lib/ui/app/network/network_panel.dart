import 'package:flutter/material.dart';
import '../../../services/ssh_chat_service.dart';

class NetworkPanelMobile extends StatelessWidget {
  final SshChatService chatService;
  const NetworkPanelMobile({super.key, required this.chatService});

  @override
  Widget build(BuildContext context) {
    return Center(child: Text("NetworkPanel (Android UI)"));
  }
}