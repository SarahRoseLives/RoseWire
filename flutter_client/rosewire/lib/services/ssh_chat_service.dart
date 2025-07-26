import 'dart:async';
import 'dart:convert';
import 'dart:io';
import 'dart:math';
import 'package:path/path.dart' as p;
import 'package:dartssh2/dartssh2.dart';
import '../models/search_result.dart';

enum TransferStatus { pending, active, complete, failed }

class Transfer {
  final String transferID;
  final String fileName;
  final String fromUser;
  final String toUser;
  final int size;
  TransferStatus status;
  double progress;
  String speed;
  String error;
  DateTime startedAt;
  DateTime? completedAt;
  int _bytesTransferred = 0;

  Transfer({
    required this.transferID,
    required this.fileName,
    required this.fromUser,
    required this.toUser,
    required this.size,
    required this.status,
    this.progress = 0.0,
    this.speed = "",
    this.error = "",
    required this.startedAt,
    this.completedAt,
  });
}

class SshChatService {
  String _host;
  int _port;

  SSHClient? _client;
  SSHSession? _chatSession;
  String? _nickname;

  final _messageController = StreamController<Map<String, dynamic>>.broadcast();
  Stream<Map<String, dynamic>> get messages => _messageController.stream;

  final _searchResultController = StreamController<List<SearchResult>>.broadcast();
  Stream<List<SearchResult>> get searchResults => _searchResultController.stream;

  final _transferController = StreamController<List<Transfer>>.broadcast();
  Stream<List<Transfer>> get transfers => _transferController.stream;

  final Map<String, Transfer> _activeTransfers = {};
  final Map<String, IOSink> _fileSinks = {};

  String? _libraryPath;

  SshChatService({String host = 'sarahsforge.dev', int port = 2222})
      : _host = host,
        _port = port;

  // Allow updating the host/port
  void setServer({required String host, int port = 2222}) {
    _host = host;
    _port = port;
  }

  String get host => _host;
  int get port => _port;

  Future<void> setLibraryPath(String path) async {
    _libraryPath = path;
  }

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
      final payload = decoded['payload'];

      switch (type) {
        case 'chat_broadcast':
        case 'system_broadcast':
          _messageController.add(decoded);
          break;
        case 'search_results':
          _handleSearchResult(payload as Map<String, dynamic>);
          break;
        case 'transfer_start':
          _handleTransferStart(payload as Map<String, dynamic>);
          break;
        case 'upload_request':
          _handleUploadRequest(payload as Map<String, dynamic>);
          break;
        case 'upload_data':
          _handleDownloadData(payload as Map<String, dynamic>);
          break;
        case 'upload_done':
          _handleDownloadDone(payload as Map<String, dynamic>);
          break;
        case 'transfer_error':
          _handleDownloadError(payload as Map<String, dynamic>);
          break;
        case 'network_stats':
          _messageController.add(decoded);
          break;
        default:
          _messageController.add({
            'type': 'system_broadcast',
            'payload': {'text': msg, 'isSystem': true}
          });
      }
    } catch (e) {
      _messageController.add({
        'type': 'system_broadcast',
        'payload': {'text': msg, 'isSystem': true}
      });
    }
  }

  void _handleSearchResult(Map<String, dynamic> payload) {
    final resultsList = payload['results'] as List<dynamic>? ?? [];
    final results = resultsList.map((item) {
      final map = item as Map<String, dynamic>;
      return SearchResult(
        fileName: map['fileName'] as String,
        size: map['size'] as int,
        peer: map['peer'] as String,
      );
    }).toList();
    _searchResultController.add(results);
  }

  void _handleTransferStart(Map<String, dynamic> payload) {
    final newTransfer = Transfer(
      transferID: payload['transferID'] as String,
      fileName: payload['fileName'] as String,
      size: payload['size'] as int,
      fromUser: payload['fromUser'] as String,
      toUser: _nickname!,
      status: TransferStatus.pending,
      startedAt: DateTime.now(),
    );
    _activeTransfers[newTransfer.transferID] = newTransfer;
    _transferController.add(_activeTransfers.values.toList());
  }

  void _handleUploadRequest(Map<String, dynamic> payload) async {
    final transferID = payload['transferID'] as String;
    final filename = payload['fileName'] as String;

    try {
      if (_libraryPath == null) {
        _sendCommand('upload_error', {'transferID': transferID, 'message': "Library path not set"});
        return;
      }
      final filePath = p.join(_libraryPath!, filename);
      if (!await File(filePath).exists()) {
        _sendCommand('upload_error', {'transferID': transferID, 'message': "File not found on my end"});
        return;
      }

      final file = File(filePath);
      final fileSize = await file.length();
      const chunkSize = 32768; // 32KB

      final raf = await file.open();
      try {
        for (int offset = 0; offset < fileSize; offset += chunkSize) {
          final bytesToRead = min(chunkSize, fileSize - offset);
          await raf.setPosition(offset);
          final chunk = await raf.read(bytesToRead);

          final encoded = base64.encode(chunk);
          _sendCommand('upload_data', {
            'transferID': transferID,
            'data': encoded,
          });
          await Future.delayed(const Duration(milliseconds: 0));
        }
      } finally {
        await raf.close();
      }
      _sendCommand('upload_done', {'transferID': transferID});
    } catch (e) {
      _sendCommand('upload_error', {'transferID': transferID, 'message': "Fatal error reading file: $e"});
    }
  }

  void _handleDownloadData(Map<String, dynamic> payload) async {
    final transferID = payload['transferID'] as String;
    final data = payload['data'] as String;

    final transfer = _activeTransfers[transferID];
    if (transfer == null || _libraryPath == null) return;

    if (transfer.status == TransferStatus.pending) {
      // Create file immediately on first chunk
      final file = File(p.join(_libraryPath!, transfer.fileName));
      if (await file.exists()) await file.delete(); // Clear previous
      transfer.status = TransferStatus.active;
    }

    try {
      final sink = _fileSinks[transferID] ?? File(p.join(_libraryPath!, transfer.fileName)).openWrite();
      _fileSinks[transferID] = sink;

      final chunk = base64.decode(data);
      sink.add(chunk);

      transfer._bytesTransferred += chunk.length;
      if (transfer.size > 0) {
        transfer.progress = transfer._bytesTransferred / transfer.size;
      }
      _transferController.add(_activeTransfers.values.toList());
    } catch (e) {
      transfer.status = TransferStatus.failed;
      transfer.error = "Failed to write chunk: $e";
      await _fileSinks[transferID]?.close();
      _fileSinks.remove(transferID);
      _transferController.add(_activeTransfers.values.toList());
    }
  }

  void _handleDownloadDone(Map<String, dynamic> payload) async {
    final transferID = payload['transferID'] as String;
    await _fileSinks[transferID]?.close();
    _fileSinks.remove(transferID);

    final transfer = _activeTransfers[transferID];
    if (transfer == null) return;

    transfer.status = TransferStatus.complete;
    transfer.progress = 1.0;
    transfer.completedAt = DateTime.now();
    _transferController.add(_activeTransfers.values.toList());
  }

  void _handleDownloadError(Map<String, dynamic> payload) async {
    final transferID = payload['transferID'] as String;
    final errorMsg = payload['message'] as String? ?? "Transfer failed by peer.";

    await _fileSinks[transferID]?.close();
    _fileSinks.remove(transferID);

    final transfer = _activeTransfers[transferID];
    if (transfer == null) return;

    transfer.status = TransferStatus.failed;
    transfer.error = errorMsg;
    _transferController.add(_activeTransfers.values.toList());
  }

  void sendMessage(String text) {
    _sendCommand('chat_message', {'text': text});
  }

  void shareFiles(List<Map<String, dynamic>> files) {
    _sendCommand('share', {'files': files});
  }

  void searchFiles(String query) {
    _sendCommand('search', {'query': query.toLowerCase()});
  }

  void fetchTopFiles() {
    _sendCommand('top_files');
  }

  void requestStats() {
    _sendCommand('get_stats');
  }

  void downloadFile(String fileName, int size, String peer) {
    if (_libraryPath == null) {
      _messageController.add({
        'type': 'system_broadcast',
        'payload': {'text': "[System] Cannot download: Library path not set.", 'isSystem': true}
      });
      return;
    }
    _sendCommand('get_file', {'fileName': fileName, 'peer': peer});
  }

  List<Transfer> getCurrentTransfers() => _activeTransfers.values.toList();

  void dispose() {
    _fileSinks.values.forEach((sink) => sink.close());
    _fileSinks.clear();
    _chatSession?.close();
    _client?.close();
    _messageController.close();
    _searchResultController.close();
    _transferController.close();
  }
}