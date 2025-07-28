import 'dart:async';
import 'dart:convert';
import 'dart:io';
import 'package:dartssh2/dartssh2.dart';
import 'ssh_file_service.dart';
import '../models/search_result.dart';

class SshChatService {
    String _host;
    int _port;

    SSHClient? _client;
    SSHSession? _chatSession;
    String? _nickname;
    bool _disposed = false;

    SshFileService? _fileService;

    final _messageController = StreamController<Map<String, dynamic>>.broadcast();
    Stream<Map<String, dynamic>> get messages => _messageController.stream;

    SshChatService({String host = 'sarahsforge.dev', int port = 2222})
        : _host = host,
          _port = port;

    // Proxy properties and methods from SshFileService
    Stream<List<SearchResult>> get searchResults => _fileService?.searchResults ?? Stream.value([]);
    Stream<List<Transfer>> get transfers => _fileService?.transfers ?? Stream.value([]);

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

    String get host => _host;
    int get port => _port;

    Future<void> connect({
        required String nickname,
        required String keyPath,
    }) async {
        _nickname = nickname;
        if (_client?.isClosed == false) return;

        try {
            final privateKey = await File(keyPath).readAsString();
            final socket = await SSHSocket.connect(_host, _port);

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
        } catch (e) {
            _messageController.add({
                'type': 'system_broadcast',
                'payload': {'text': "[System] Failed to connect: $e", 'isSystem': true}
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
        _messageController.close();
    }
}