// CLIENT/ui/desktop/chat/chat_panel.dart
import 'dart:async';
import 'package:flutter/material.dart';
import '../../../services/ssh_chat_service.dart';
import '../rosewire_desktop.dart';

class ChatPanel extends StatefulWidget {
  final SshChatService chatService;
  final String nickname;
  // The full address of the currently logged-in user (e.g., @user@instance.com)
  final String currentUserAddress;

  const ChatPanel({
    super.key,
    required this.chatService,
    required this.nickname,
    required this.currentUserAddress,
  });

  @override
  State<ChatPanel> createState() => _ChatPanelState();
}

class _ChatPanelState extends State<ChatPanel> {
  final TextEditingController _chatController = TextEditingController();
  final ScrollController _scrollController = ScrollController();
  final List<_ChatMessage> _messages = [];
  StreamSubscription? _messageSubscription;
  late final String _currentUserInstance;

  @override
  void initState() {
    super.initState();
    // Safely extract the instance domain from the current user's address
    final parts = widget.currentUserAddress.split('@');
    _currentUserInstance = parts.length > 2 ? parts.last : '';

    _messageSubscription = widget.chatService.messages.listen(_onMessageReceived);
  }

  void _onMessageReceived(Map<String, dynamic> message) {
    final type = message['type'] as String;
    final payload = message['payload'] as Map<String, dynamic>;

    late final _ChatMessage newMessage;

    switch (type) {
      case 'chat_broadcast':
        // Safely handle the nickname, which is now the full federated address.
        final fullAddress = payload['nickname'] as String?;
        if (fullAddress == null || fullAddress.isEmpty) {
            print("Received chat_broadcast with no nickname. Skipping.");
            return;
        }
        final text = payload['text'] as String;
        newMessage = _ChatMessage(fullAddress, text, isMe: fullAddress == widget.currentUserAddress);
        break;
      case 'system_broadcast':
        final text = payload['text'] as String;
        newMessage = _ChatMessage("System", text, isMe: false, isSystem: true);
        break;
      default:
        return;
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
    Future.delayed(const Duration(milliseconds: 50), () {
      if (_scrollController.hasClients) {
        _scrollController.animateTo(
          _scrollController.position.maxScrollExtent,
          duration: const Duration(milliseconds: 250),
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
    return Padding(
      padding: const EdgeInsets.symmetric(horizontal: 32, vertical: 18),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          const Text(
            "Global Network Chat",
            style: TextStyle(
              fontSize: 18,
              color: roseWhite,
              fontWeight: FontWeight.w600,
            ),
          ),
          const SizedBox(height: 18),
          Expanded(
            child: Container(
              decoration: BoxDecoration(
                color: roseGray.withOpacity(0.7),
                borderRadius: BorderRadius.circular(18),
                border: Border.all(
                  color: rosePurple.withOpacity(0.15),
                  width: 2,
                ),
              ),
              child: ListView.builder(
                controller: _scrollController,
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
                          color: roseWhite.withOpacity(0.6),
                          fontStyle: FontStyle.italic,
                        ),
                      ),
                    );
                  }

                  final isMe = msg.isMe;
                  final displayName = msg.fullAddress.split('@').elementAt(1);
                  final avatarChar = (isMe ? widget.nickname : displayName).substring(0, 1).toUpperCase();
                  final avatarColor = isMe ? roseGreen : rosePink;

                  final avatar = CircleAvatar(
                      radius: 16,
                      backgroundColor: avatarColor,
                      child: Text(
                        avatarChar,
                        style: const TextStyle(
                          color: roseWhite, fontWeight: FontWeight.bold, fontSize: 16,
                        ),
                      ),
                    );

                  final bubble = Flexible(
                    child: Container(
                      padding: const EdgeInsets.symmetric(horizontal: 16, vertical: 10),
                      decoration: BoxDecoration(
                        color: isMe ? rosePink.withOpacity(0.7) : rosePurple.withOpacity(0.2),
                        borderRadius: BorderRadius.circular(14),
                      ),
                      child: Column(
                        crossAxisAlignment: CrossAxisAlignment.start,
                        children: [
                          if (!isMe)
                            Text(
                              msg.fullAddress,
                              style: const TextStyle(
                                color: rosePink,
                                fontWeight: FontWeight.bold,
                                fontSize: 13,
                              ),
                            ),
                          Text(
                            msg.text,
                            style: const TextStyle(
                              color: roseWhite,
                              fontSize: 15,
                            ),
                          ),
                        ],
                      ),
                    ),
                  );

                  return Container(
                    margin: const EdgeInsets.symmetric(vertical: 6, horizontal: 12),
                    child: Row(
                      mainAxisAlignment: isMe ? MainAxisAlignment.end : MainAxisAlignment.start,
                      crossAxisAlignment: CrossAxisAlignment.start,
                      children: isMe
                        ? [ bubble, const SizedBox(width: 8), avatar ]
                        : [ avatar, const SizedBox(width: 8), bubble ],
                    ),
                  );
                },
              ),
            ),
          ),
          const SizedBox(height: 12),
          Row(
            children: [
              Expanded(
                child: TextField(
                  controller: _chatController,
                  decoration: InputDecoration(
                    hintText: "Type a message...",
                    hintStyle: TextStyle(color: roseWhite.withOpacity(0.4)),
                    filled: true,
                    fillColor: roseGray.withOpacity(0.8),
                    border: OutlineInputBorder(
                      borderRadius: BorderRadius.circular(12),
                      borderSide: BorderSide.none,
                    ),
                    contentPadding: const EdgeInsets.symmetric(vertical: 0, horizontal: 16),
                  ),
                  style: const TextStyle(color: roseWhite, fontSize: 15),
                  onSubmitted: (_) => _sendMessage(),
                ),
              ),
              const SizedBox(width: 14),
              ElevatedButton.icon(
                icon: const Icon(Icons.send),
                label: const Text("Send"),
                style: ElevatedButton.styleFrom(
                  backgroundColor: rosePink,
                  foregroundColor: roseWhite,
                  padding: const EdgeInsets.symmetric(horizontal: 18, vertical: 12),
                  shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(10)),
                  elevation: 0,
                ),
                onPressed: _sendMessage,
              ),
            ],
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