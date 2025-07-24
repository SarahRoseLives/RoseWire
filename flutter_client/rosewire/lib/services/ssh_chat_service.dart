import 'dart:async';
import 'dart:convert';
import 'dart:io';

import 'package:dartssh2/dartssh2.dart';
import 'package:flutter/foundation.dart';
import '../models/search_result.dart';

/// Manages the SSH connection and chat subsystem communication.
class SshChatService {
  final String _host = 'localhost';
  final int _port = 2222;

  SSHClient? _client;
  SSHSession? _chatSession;

  final _messageController = StreamController<String>.broadcast();
  Stream<String> get messages => _messageController.stream;

  final _searchResultController = StreamController<List<SearchResult>>.broadcast();
  Stream<List<SearchResult>> get searchResults => _searchResultController.stream;

  Future<void> connect({
    required String nickname,
    required String keyPath,
  }) async {
    if (_client?.isClosed == false) {
      debugPrint("SSH client already connected.");
      return;
    }

    try {
      final privateKey = await File(keyPath).readAsString();
      final socket = await SSHSocket.connect(_host, _port);

      _client = SSHClient(
        socket,
        username: nickname,
        identities: SSHKeyPair.fromPem(privateKey),
      );

      await _client!.authenticated;

      // --------------------- FINAL FIX START ---------------------
      // The correct way to request a subsystem is to pass a specially
      // formatted command to the `execute` method. This creates the
      // session and requests the subsystem in a single step.
      _chatSession = await _client!.execute('subsystem:chat');
      // ---------------------- FINAL FIX END ----------------------

      _chatSession!.stdout
          .transform(StreamTransformer.fromBind(utf8.decoder.bind))
          .transform(const LineSplitter())
          .listen(
        (serverMessage) {
          if (serverMessage.startsWith('[SEARCH] ')) {
            final payload = serverMessage.substring(9);
            _handleSearchResult(payload);
          } else {
            _messageController.add(serverMessage);
          }
        },
        onError: (error) {
          debugPrint('Chat stream error: $error');
          _messageController.add("[System] Connection error: $error");
        },
        onDone: () {
          debugPrint('Chat stream closed.');
          _messageController.add("[System] Disconnected from chat.");
        },
      );
    } catch (e) {
      debugPrint("SSH connection failed: $e");
      _messageController.add("[System] Failed to connect: $e");
      dispose();
    }
  }

  void _handleSearchResult(String payload) {
    if (payload.isEmpty) {
      _searchResultController.add([]);
      return;
    }
    final results = payload.split('|').map((part) {
      final info = part.split(':');
      if (info.length == 3) {
        return SearchResult(
          fileName: info[0],
          size: int.tryParse(info[1]) ?? 0,
          peer: info[2],
        );
      }
      return null;
    }).where((result) => result != null).cast<SearchResult>().toList();

    _searchResultController.add(results);
  }

  void sendMessage(String text) {
    if (_chatSession != null && _client?.isClosed == false) {
      _chatSession!.stdin.add(utf8.encode('$text\n'));
    } else {
      debugPrint("Cannot send message, chat session is not active.");
    }
  }

  void searchFiles(String query) {
    sendMessage("/search $query");
  }

  void dispose() {
    _chatSession?.close();
    _client?.close();
    _messageController.close();
    _searchResultController.close();
  }
}