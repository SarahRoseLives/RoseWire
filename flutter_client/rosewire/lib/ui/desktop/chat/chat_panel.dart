import 'dart:async';
import 'package:flutter/material.dart';
import '../../../services/ssh_chat_service.dart'; // <-- Import service
import '../rosewire_desktop.dart';

class ChatPanel extends StatefulWidget {
  final SshChatService chatService; // <-- Receive the service
  final String nickname; // <-- Receive the user's nickname

  const ChatPanel({
    super.key,
    required this.chatService,
    required this.nickname,
  });

  @override
  State<ChatPanel> createState() => _ChatPanelState();
}

class _ChatPanelState extends State<ChatPanel> {
  final TextEditingController _chatController = TextEditingController();
  final ScrollController _scrollController = ScrollController();
  final List<_ChatMessage> _messages = []; // <-- Start with an empty list
  StreamSubscription? _messageSubscription;

  @override
  void initState() {
    super.initState();
    // Subscribe to the message stream from the service
    _messageSubscription = widget.chatService.messages.listen(_onMessageReceived);
  }

  void _onMessageReceived(String rawMessage) {
    // A simple regex to parse messages from the server.
    // Format 1 (user message): [15:04] someuser: Hello world
    // Format 2 (system message): [15:04] anotheruser joined the chat.
    final userMsgRegex = RegExp(r'^\[.+\]\s(.+?):\s(.*)$');
    final systemMsgRegex = RegExp(r'^\[.+\]\s(?!.+:)(.*)$');

    final userMatch = userMsgRegex.firstMatch(rawMessage);
    final systemMatch = systemMsgRegex.firstMatch(rawMessage);

    late final _ChatMessage newMessage;

    if (userMatch != null) {
      final nickname = userMatch.group(1)!;
      final text = userMatch.group(2)!;
      newMessage = _ChatMessage(nickname, text, isMe: nickname == widget.nickname);
    } else if (systemMatch != null) {
      final text = systemMatch.group(1)!;
      newMessage = _ChatMessage("System", text, isMe: false, isSystem: true);
    } else {
      // Fallback for un-parsable messages
      newMessage = _ChatMessage("System", rawMessage, isMe: false, isSystem: true);
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

    // Send the message via the service
    widget.chatService.sendMessage(text);

    // The UI will update when the server echoes the message back.
    // This ensures the view is always synced with the server state.
    _chatController.clear();
  }

  void _scrollToBottom() {
    // A short delay ensures the list has been rebuilt before we scroll.
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
    // Clean up the subscription to avoid memory leaks
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
          Text(
            "Global Network Chat",
            style: TextStyle(
              fontSize: 18,
              color: roseWhite,
              fontWeight: FontWeight.w600,
            ),
          ),
          SizedBox(height: 18),
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

                  // Custom widget for system messages (joins/leaves)
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
                  return Container(
                    margin: EdgeInsets.symmetric(vertical: 6, horizontal: 12),
                    alignment: isMe ? Alignment.centerRight : Alignment.centerLeft,
                    child: Row(
                      mainAxisAlignment: isMe ? MainAxisAlignment.end : MainAxisAlignment.start,
                      crossAxisAlignment: CrossAxisAlignment.start,
                      children: [
                        if (!isMe)
                          CircleAvatar(
                            radius: 16,
                            backgroundColor: rosePink,
                            child: Text(
                              msg.nickname.substring(0, 1).toUpperCase(),
                              style: TextStyle(
                                color: roseWhite, fontWeight: FontWeight.bold, fontSize: 16,
                              ),
                            ),
                          ),
                        if (!isMe) SizedBox(width: 8),
                        Flexible(
                          child: Container(
                            padding: EdgeInsets.symmetric(horizontal: 16, vertical: 10),
                            decoration: BoxDecoration(
                              color: isMe ? rosePink.withOpacity(0.7) : rosePurple.withOpacity(0.2),
                              borderRadius: BorderRadius.circular(14),
                            ),
                            child: Column(
                              crossAxisAlignment: CrossAxisAlignment.start,
                              children: [
                                if (!isMe)
                                  Text(
                                    msg.nickname,
                                    style: TextStyle(
                                      color: rosePink,
                                      fontWeight: FontWeight.bold,
                                      fontSize: 13,
                                    ),
                                  ),
                                Text(
                                  msg.text,
                                  style: TextStyle(
                                    color: roseWhite,
                                    fontSize: 15,
                                  ),
                                ),
                              ],
                            ),
                          ),
                        ),
                        if (isMe) SizedBox(width: 8),
                        if (isMe)
                          CircleAvatar(
                            radius: 16,
                            backgroundColor: roseGreen,
                            child: Text(
                              msg.nickname.substring(0, 1).toUpperCase(),
                              style: TextStyle(
                                color: roseWhite, fontWeight: FontWeight.bold, fontSize: 16,
                              ),
                            ),
                          ),
                      ],
                    ),
                  );
                },
              ),
            ),
          ),
          SizedBox(height: 12),
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
                    contentPadding: EdgeInsets.symmetric(vertical: 0, horizontal: 16),
                  ),
                  style: TextStyle(color: roseWhite, fontSize: 15),
                  onSubmitted: (_) => _sendMessage(),
                ),
              ),
              SizedBox(width: 14),
              ElevatedButton.icon(
                icon: Icon(Icons.send),
                label: Text("Send"),
                style: ElevatedButton.styleFrom(
                  backgroundColor: rosePink,
                  foregroundColor: roseWhite,
                  padding: EdgeInsets.symmetric(horizontal: 18, vertical: 12),
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

// Update the _ChatMessage class to handle system messages
class _ChatMessage {
  final String nickname;
  final String text;
  final bool isMe;
  final bool isSystem;

  _ChatMessage(this.nickname, this.text, {required this.isMe, this.isSystem = false});
}