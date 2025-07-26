import 'package:flutter/material.dart';
import '../../../services/ssh_chat_service.dart';

class TransfersPanelMobile extends StatelessWidget {
  final SshChatService chatService;
  const TransfersPanelMobile({super.key, required this.chatService});

  @override
  Widget build(BuildContext context) {
    return Center(child: Text("TransfersPanel (Android UI)"));
  }
}