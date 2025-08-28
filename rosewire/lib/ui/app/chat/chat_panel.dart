// CLIENT/ui/app/chat/chat_panel.dart
import 'dart:async';
import 'package:flutter/material.dart';
import '../../../services/ssh_chat_service.dart';

class ChatPanelMobile extends StatefulWidget {
  final SshChatService chatService;
  final String nickname;
  // The full address of the currently logged-in user (e.g., @user@instance.com)
  final String currentUserAddress;

  const ChatPanelMobile({
    super.key,
    required this.chatService,
    required this.nickname,
    required this.currentUserAddress,
  });

  @override
  State<ChatPanelMobile> createState() => _ChatPanelMobileState();
}

class _ChatPanelMobileState extends State<ChatPanelMobile> {
  final TextEditingController _chatController = TextEditingController();
  final ScrollController _scrollController = ScrollController();
  final List<_ChatMessage> _messages = [];
  StreamSubscription? _messageSubscription;

  @override
  void initState() {
    super.initState();
    _messageSubscription = widget.chatService.messages.listen(_onMessageReceived);
  }

  void _onMessageReceived(Map<String, dynamic> message) {
    final type = message['type'] as String;
    final payload = message['payload'] as Map<String, dynamic>;

    switch (type) {
      case 'chat_broadcast':
      case 'system_broadcast':
        _handleChatMessage(type, payload);
        break;
      default:
        return;
    }
  }

  void _handleChatMessage(String type, Map<String, dynamic> payload) {
    late final _ChatMessage newMessage;

    switch (type) {
      case 'chat_broadcast':
        final fullAddress = payload['nickname'] as String?;
        if (fullAddress == null || fullAddress.isEmpty) return;

        final text = payload['text'] as String;
        newMessage = _ChatMessage(fullAddress, text, isMe: fullAddress == widget.currentUserAddress);
        break;
      case 'system_broadcast':
        final text = payload['text'] as String;
        newMessage = _ChatMessage("System", text, isMe: false, isSystem: true);
        break;
    }

    if (mounted) {
      setState(() {
        _messages.add(newMessage);
      });
      _scrollToBottom();
    }
  }

  void _sendMessage() {
    final text = _chatController.text.trim();
    if (text.isEmpty) return;
    widget.chatService.sendMessage(text);
    _chatController.clear();
  }

  void _scrollToBottom() {
    Future.delayed(const Duration(milliseconds: 100), () {
      if (_scrollController.hasClients) {
        _scrollController.animateTo(
          _scrollController.position.maxScrollExtent,
          duration: const Duration(milliseconds: 300),
          curve: Curves.easeOut,
        );
      }
    });
  }

  @override
  void dispose() {
    _messageSubscription?.cancel();
    _chatController.dispose();
    _scrollController.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      backgroundColor: Colors.grey[900],
      body: Column(
        children: [
          Expanded(
            child: ListView.builder(
              controller: _scrollController,
              padding: const EdgeInsets.all(8.0),
              itemCount: _messages.length,
              itemBuilder: (context, idx) {
                final msg = _messages[idx];

                if (msg.isSystem) {
                  return Padding(
                    padding: const EdgeInsets.symmetric(vertical: 8.0),
                    child: Text(
                      msg.text,
                      textAlign: TextAlign.center,
                      style: TextStyle(
                        color: Colors.white.withOpacity(0.6),
                        fontStyle: FontStyle.italic,
                      ),
                    ),
                  );
                }

                final isMe = msg.isMe;

                return Container(
                  margin: const EdgeInsets.symmetric(vertical: 6, horizontal: 12),
                  alignment: isMe ? Alignment.centerRight : Alignment.centerLeft,
                  child: Container(
                    padding: const EdgeInsets.symmetric(horizontal: 16, vertical: 10),
                    decoration: BoxDecoration(
                      color: isMe ? Colors.pinkAccent : Colors.grey[800],
                      borderRadius: BorderRadius.circular(14),
                    ),
                    child: Column(
                      crossAxisAlignment: CrossAxisAlignment.start,
                      children: [
                        if (!isMe)
                          Padding(
                            padding: const EdgeInsets.only(bottom: 4.0),
                            child: Text(
                              msg.fullAddress,
                              style: const TextStyle(
                                color: Colors.pinkAccent,
                                fontWeight: FontWeight.bold,
                                fontSize: 13,
                              ),
                            ),
                          ),
                        Text(
                          msg.text,
                          style: const TextStyle(
                            color: Colors.white,
                            fontSize: 15,
                          ),
                        ),
                      ],
                    ),
                  ),
                );
              },
            ),
          ),
          _buildChatInput(),
        ],
      ),
    );
  }

  Widget _buildChatInput() {
    return Container(
      padding: const EdgeInsets.all(8.0),
      decoration: BoxDecoration(
        color: Colors.grey[850],
        border: Border(
          top: BorderSide(color: Colors.pinkAccent.withOpacity(0.5), width: 1),
        ),
      ),
      child: Row(
        children: [
          Expanded(
            child: TextField(
              controller: _chatController,
              decoration: InputDecoration(
                hintText: "Type a message...",
                hintStyle: TextStyle(color: Colors.white.withOpacity(0.5)),
                filled: true,
                fillColor: Colors.grey[800],
                border: OutlineInputBorder(
                  borderRadius: BorderRadius.circular(20),
                  borderSide: BorderSide.none,
                ),
                contentPadding: const EdgeInsets.symmetric(vertical: 10, horizontal: 16),
              ),
              style: const TextStyle(color: Colors.white, fontSize: 15),
              onSubmitted: (_) => _sendMessage(),
            ),
          ),
          const SizedBox(width: 8),
          IconButton(
            icon: const Icon(Icons.send, color: Colors.pinkAccent),
            onPressed: _sendMessage,
          ),
        ],
      ),
    );
  }
}

class _ChatMessage {
  final String fullAddress;
  final String text;
  final bool isMe;
  final bool isSystem;

  _ChatMessage(this.fullAddress, this.text, {required this.isMe, this.isSystem = false});
}