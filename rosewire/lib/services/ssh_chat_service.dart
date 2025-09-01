// CLIENT/services/ssh_chat_service.dart
import 'dart:async';
import 'dart:convert';
import 'dart:io';
import 'dart:math';
import 'package:http/http.dart' as http;
import 'package:dartssh2/dartssh2.dart';
import 'package:connectivity_plus/connectivity_plus.dart';
import 'ssh_file_service.dart';
import '../models/search_result.dart';

enum ConnectionStatus { connected, disconnected, reconnecting }

class SshChatService {
  static const String clientVersion = "1.0.5";

  String? _host;
  int _port;

  SSHClient? _client;
  SSHSession? _chatSession;
  String? _nickname;
  bool _disposed = false;
  bool _isReconnecting = false;
  Timer? _healthCheckTimer;

  StreamSubscription<List<ConnectivityResult>>? _connectivitySubscription;

  String? _lastNickname;
  String? _lastKeyPath;
  String? _lastHost;
  String? _libraryPath; // <-- FIX: Added to persist across reconnections

  SshFileService? _fileService;

  final _searchResultController =
      StreamController<List<SearchResult>>.broadcast();
  final _transferController = StreamController<List<Transfer>>.broadcast();
  final _messageController = StreamController<Map<String, dynamic>>.broadcast();
  final _versionStatusController = StreamController<String>.broadcast();
  final _connectionStatusController =
      StreamController<ConnectionStatus>.broadcast();
  final _identityController = StreamController<String>.broadcast();

  Stream<List<SearchResult>> get searchResults => _searchResultController.stream;
  Stream<List<Transfer>> get transfers => _transferController.stream;
  Stream<Map<String, dynamic>> get messages => _messageController.stream;
  Stream<String> get versionStatus => _versionStatusController.stream;
  Stream<ConnectionStatus> get connectionStatus =>
      _connectionStatusController.stream;
  Stream<String> get identity => _identityController.stream;

  SshChatService({int port = 2222}) : _port = port;

  // FIX: Modified to store path locally in the service
  Future<void> setLibraryPath(String path) async {
    _libraryPath = path;
    await _fileService?.setLibraryPath(path);
  }

  void shareFiles(List<Map<String, dynamic>> files) =>
      _fileService?.shareFiles(files);
  void searchFiles(String query) => _fileService?.searchFiles(query);
  void fetchTopFiles() => _fileService?.fetchTopFiles();
  void downloadFile(String fileName, int size, String peer) =>
      _fileService?.downloadFile(fileName, size, peer);
  List<Transfer> getCurrentTransfers() =>
      _fileService?.getCurrentTransfers() ?? [];

  void setServer({required String host, int port = 2222}) {
    _host = host;
    _port = port;
  }

  String? get host => _host;
  int get port => _port;

  Future<void> _checkServerVersion(String host) async {
    try {
      final uri = Uri.parse('https://$host:8080/api/version');
      final response =
          await http.get(uri).timeout(const Duration(seconds: 5));
      if (response.statusCode == 200) {
        final data = json.decode(response.body);
        final serverVersion = data['version'] as String?;
        if (serverVersion != null) {
          if (serverVersion != clientVersion) {
            _versionStatusController.add(
                "Warning: Server is running version $serverVersion, but client expects $clientVersion.");
          } else {
            _versionStatusController.add("Server version is up-to-date.");
          }
        }
      } else {
        _versionStatusController.add("Warning: Could not verify server version.");
      }
    } catch (e) {
      _versionStatusController.add("Warning: Could not verify server version.");
      print("Version check failed: $e");
    }
  }

  void _handleDisconnection() {
    if (_disposed || _isReconnecting) return;
    _isReconnecting = true;

    _healthCheckTimer?.cancel();
    _client?.close();

    _connectionStatusController.add(ConnectionStatus.disconnected);

    unawaited(_startReconnectionLoop());
  }

  Future<void> _startReconnectionLoop() async {
    print("Starting reconnection process...");
    int attempt = 0;
    while (!_disposed && _client?.isClosed != false) {
      final delay = Duration(seconds: min(5 * (attempt + 1), 30));
      _connectionStatusController.add(ConnectionStatus.reconnecting);
      print("Reconnection attempt ${attempt + 1} in ${delay.inSeconds} seconds...");
      await Future.delayed(delay);

      if (_disposed) break;

      final connectivityResult = await Connectivity().checkConnectivity();
      if (connectivityResult.contains(ConnectivityResult.none)) {
        print("Skipping reconnect attempt, no network connection.");
        attempt++;
        continue; // Skip this attempt and wait for the next cycle
      }

      try {
        await connect(
          nickname: _lastNickname!,
          keyPath: _lastKeyPath!,
          host: _lastHost!,
          isReconnect: true,
        );
      } catch (e) {
        print("Reconnect attempt ${attempt + 1} failed: $e");
      }
      attempt++;
    }
    _isReconnecting = false;
    print("Exited reconnection loop.");
  }

  void _healthCheck() {
    if (_client == null || _client!.isClosed || _isReconnecting || _disposed) {
      if (!_isReconnecting && _client?.isClosed != false) {
        _handleDisconnection();
      }
      return;
    }
    try {
      requestStats();
    } catch (e) {
      print("Health check failed, triggering disconnection: $e");
      _handleDisconnection();
    }
  }

  Future<void> connect({
    required String nickname,
    required String keyPath,
    required String host,
    bool isReconnect = false,
  }) async {
    _nickname = nickname;
    _host = host;

    if (!isReconnect) {
      _lastNickname = nickname;
      _lastKeyPath = keyPath;
      _lastHost = host;
      unawaited(_checkServerVersion(host));

      _connectivitySubscription?.cancel();
      _connectivitySubscription =
          Connectivity().onConnectivityChanged.listen((List<ConnectivityResult> result) {
        if (result.contains(ConnectivityResult.none) &&
            !result.any((r) => r != ConnectivityResult.none)) {
          print("Connectivity changed to offline. Triggering disconnection.");
          _handleDisconnection();
        }
      });
    }

    if (_host == null) throw Exception("Hostname not set.");

    try {
      final privateKey = await File(keyPath).readAsString();
      final socket = await SSHSocket.connect(_host!, _port);

      _client = SSHClient(
        socket,
        username: nickname,
        identities: SSHKeyPair.fromPem(privateKey),
        keepAliveInterval: const Duration(seconds: 30),
      );

      await _client!.authenticated;
      _connectionStatusController.add(ConnectionStatus.connected);

      _fileService = SshFileService(
        client: _client!,
        sendCommand: _sendCommand,
        nickname: _nickname!,
        onSearchResults: _searchResultController.add,
        onTransfersUpdate: _transferController.add,
      );

      // FIX: Re-apply the library path on every new connection
      if (_libraryPath != null) {
        await _fileService!.setLibraryPath(_libraryPath!);
      }

      _chatSession = await _client!.execute('subsystem:chat');

      _chatSession!.stdout
          .cast<List<int>>()
          .transform(utf8.decoder)
          .transform(const LineSplitter())
          .listen(
            _handleServerMessage,
            onError: (error) {
              print("[System] Connection error: $error");
              _handleDisconnection();
            },
            onDone: () {
              print("[System] Disconnected from chat.");
              _handleDisconnection();
            },
          );

      requestStats();
      fetchTopFiles();

      _healthCheckTimer?.cancel();
      _healthCheckTimer =
          Timer.periodic(const Duration(seconds: 15), (_) => _healthCheck());
    } catch (e) {
      _connectionStatusController.add(ConnectionStatus.disconnected);
      if (isReconnect) {
        throw Exception("Reconnect failed: $e");
      } else {
        _messageController.add({
          'type': 'system_broadcast',
          'payload': {
            'text': "[System] Failed to connect to $_host: $e",
            'isSystem': true
          }
        });
      }
    }
  }

  void _sendCommand(String type, [Map<String, dynamic>? payload]) {
    if (_chatSession != null && _client?.isClosed == false) {
      final message = json.encode({'type': type, 'payload': payload ?? {}});
      _chatSession!.stdin.add(utf8.encode('$message\n'));
    }
  }

  void _handleServerMessage(String msg) {
    if (msg.trim().isEmpty) return;
    try {
      final decoded = json.decode(msg) as Map<String, dynamic>;
      final type = decoded['type'] as String?;
      final payload = decoded['payload'];
      if (_disposed) return;
      switch (type) {
        case 'welcome':
          final identity =
              (payload as Map<String, dynamic>)['identity'] as String?;
          if (identity != null) {
            _identityController.add(identity);
          }
          break;
        case 'chat_broadcast':
        case 'system_broadcast':
        case 'network_stats':
          _messageController.add(decoded);
          break;
        case 'search_results':
        case 'transfer_start':
        case 'upload_request':
        case 'transfer_error':
        case 'upload_done':
          _fileService?.handleFileMessage(decoded);
          break;
        default:
          _messageController.add({
            'type': 'system_broadcast',
            'payload': {'text': "Unknown message: $msg", 'isSystem': true}
          });
      }
    } catch (e) {
      if (!_disposed) {
        _messageController.add({
          'type': 'system_broadcast',
          'payload': {'text': "Error parsing: $msg", 'isSystem': true}
        });
      }
    }
  }

  void sendMessage(String text) => _sendCommand('chat_message', {'text': text});
  void requestStats() => _sendCommand('get_stats');

  void dispose() {
    if (_disposed) return;
    _disposed = true;
    _isReconnecting = false;
    _healthCheckTimer?.cancel();
    _connectivitySubscription?.cancel();
    _fileService?.dispose();
    _chatSession?.close();
    _client?.close();
    _searchResultController.close();
    _transferController.close();
    _messageController.close();
    _versionStatusController.close();
    _connectionStatusController.close();
    _identityController.close();
  }
}