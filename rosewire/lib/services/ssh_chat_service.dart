// CLIENT/services/ssh_chat_service.dart
import 'dart:async';
import 'dart:convert';
import 'dart:io';
import 'package:http/http.dart' as http;
import 'package:dartssh2/dartssh2.dart';
import 'ssh_file_service.dart';
import '../models/search_result.dart';

class SshChatService {
  // Add the client's current version
  static const String clientVersion = "1.0.0";

  // Host is now nullable, it will be set before connect() is called.
  String? _host;
  int _port;

  SSHClient? _client;
  SSHSession? _chatSession;
  String? _nickname;
  bool _disposed = false;

  SshFileService? _fileService;

  final _searchResultController = StreamController<List<SearchResult>>.broadcast();
  final _transferController = StreamController<List<Transfer>>.broadcast();
  final _messageController = StreamController<Map<String, dynamic>>.broadcast();
  // Add a new controller for version status
  final _versionStatusController = StreamController<String>.broadcast();

  Stream<List<SearchResult>> get searchResults => _searchResultController.stream;
  Stream<List<Transfer>> get transfers => _transferController.stream;
  Stream<Map<String, dynamic>> get messages => _messageController.stream;
  // Expose the version status stream
  Stream<String> get versionStatus => _versionStatusController.stream;

  // Constructor no longer sets a default host
  SshChatService({int port = 2222}) : _port = port;

  Future<void> setLibraryPath(String path) async => await _fileService?.setLibraryPath(path);
  void shareFiles(List<Map<String, dynamic>> files) => _fileService?.shareFiles(files);
  void searchFiles(String query) => _fileService?.searchFiles(query);
  void fetchTopFiles() => _fileService?.fetchTopFiles();
  void downloadFile(String fileName, int size, String peer) => _fileService?.downloadFile(fileName, size, peer);
  List<Transfer> getCurrentTransfers() => _fileService?.getCurrentTransfers() ?? [];

  void setServer({required String host, int port = 2222}) {
      _host = host;
      _port = port;
  }

  String? get host => _host;
  int get port => _port;
  
  Future<void> _checkServerVersion(String host) async {
    try {
        final uri = Uri.parse('https://$host:8080/api/version');
        final response = await http.get(uri).timeout(const Duration(seconds: 5));

        if (response.statusCode == 200) {
            final data = json.decode(response.body);
            final serverVersion = data['version'] as String?;
            if (serverVersion != null) {
                if (serverVersion != clientVersion) {
                    _versionStatusController.add("Warning: Server is running version $serverVersion, but client expects $clientVersion. Some features may not work correctly.");
                } else {
                    _versionStatusController.add("Server version is up-to-date.");
                }
            }
        } else {
             _versionStatusController.add("Warning: Could not verify server version. It may be outdated.");
        }
    } catch (e) {
        _versionStatusController.add("Warning: Could not verify server version. It may be outdated.");
        print("Version check failed: $e");
    }
  }

  Future<void> connect({
      required String nickname,
      required String keyPath,
      required String host, // Host is now a required parameter for connect
  }) async {
      _nickname = nickname;
      _host = host; // Set the host for this connection
      if (_client?.isClosed == false) return;

      // --- Add this line ---
      unawaited(_checkServerVersion(host));

      if (_host == null) {
          final errorMessage = "[System] Connection failed: Hostname not set.";
          _messageController.add({
              'type': 'system_broadcast',
              'payload': {'text': errorMessage, 'isSystem': true}
          });
          print(errorMessage);
          return;
      }

      try {
          final privateKey = await File(keyPath).readAsString();
          final socket = await SSHSocket.connect(_host!, _port);

          _client = SSHClient(
              socket,
              username: nickname,
              identities: SSHKeyPair.fromPem(privateKey),
          );

          await _client!.authenticated;

          _fileService = SshFileService(
              client: _client!,
              sendCommand: _sendCommand,
              nickname: _nickname!,
              onSearchResults: _searchResultController.add,
              onTransfersUpdate: _transferController.add,
          );

          _chatSession = await _client!.execute('subsystem:chat');

          _chatSession!.stdout
              .cast<List<int>>()
              .transform(utf8.decoder)
              .transform(const LineSplitter())
              .listen(
                  _handleServerMessage,
                  onError: (error) => _messageController.add({
                      'type': 'system_broadcast',
                      'payload': {'text': "[System] Connection error: $error", 'isSystem': true}
                  }),
                  onDone: () => _messageController.add({
                      'type': 'system_broadcast',
                      'payload': {'text': "[System] Disconnected from chat.", 'isSystem': true}
                  }),
              );

          requestStats();
          fetchTopFiles();
      } catch (e) {
          _messageController.add({
              'type': 'system_broadcast',
              'payload': {'text': "[System] Failed to connect to $_host: $e", 'isSystem': true}
          });
          dispose();
      }
  }

  void _sendCommand(String type, [Map<String, dynamic>? payload]) {
      if (_chatSession != null && _client?.isClosed == false) {
          final message = json.encode({
              'type': type,
              'payload': payload ?? {},
          });
          _chatSession!.stdin.add(utf8.encode('$message\n'));
      }
  }

  void _handleServerMessage(String msg) {
      if (msg.trim().isEmpty) return;
      try {
          final decoded = json.decode(msg) as Map<String, dynamic>;
          final type = decoded['type'] as String?;

          switch (type) {
              case 'chat_broadcast':
              case 'system_broadcast':
              case 'network_stats':
                  if(!_disposed) _messageController.add(decoded);
                  break;

              case 'search_results':
              case 'transfer_start':
              case 'upload_request':
              case 'transfer_error':
                  _fileService?.handleFileMessage(decoded);
                  break;

              default:
                  if(!_disposed) {
                      _messageController.add({
                          'type': 'system_broadcast',
                          'payload': {'text': "Unknown message: $msg", 'isSystem': true}
                      });
                  }
          }
      } catch (e) {
           if(!_disposed) {
              _messageController.add({
                  'type': 'system_broadcast',
                  'payload': {'text': "Error parsing: $msg", 'isSystem': true}
              });
           }
      }
  }

  void sendMessage(String text) {
      _sendCommand('chat_message', {'text': text});
  }

  void requestStats() {
      _sendCommand('get_stats');
  }

  void dispose() {
      if (_disposed) return;
      _disposed = true;
      _fileService?.dispose();
      _chatSession?.close();
      _client?.close();
      _searchResultController.close();
      _transferController.close();
      _messageController.close();
      _versionStatusController.close();
  }
}