import 'dart:async';
import 'dart:io';
import 'dart:math';
import 'dart:typed_data';
import 'package:path/path.dart' as p;
import 'package:dartssh2/dartssh2.dart';
import 'package:path_provider/path_provider.dart';
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

class SshFileService {
    final SSHClient _client;
    final void Function(String, [Map<String, dynamic>?]) _sendCommand;
    final String _nickname;
    final void Function(List<SearchResult>) _onSearchResults;
    final void Function(List<Transfer>) _onTransfersUpdate;

    static const int _numStreams = 50;
    String? _libraryPath;
    bool _disposed = false;

    final Map<String, Transfer> _activeTransfers = {};

    SshFileService({
        required SSHClient client,
        required void Function(String, [Map<String, dynamic>?]) sendCommand,
        required String nickname,
        required void Function(List<SearchResult>) onSearchResults,
        required void Function(List<Transfer>) onTransfersUpdate,
    })  : _client = client,
          _sendCommand = sendCommand,
          _nickname = nickname,
          _onSearchResults = onSearchResults,
          _onTransfersUpdate = onTransfersUpdate;

    Future<void> setLibraryPath(String path) async {
        _libraryPath = path;
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

    void downloadFile(String fileName, int size, String peer) {
        if (_libraryPath == null) {
            print("[System] Cannot download: Library path not set.");
            _onTransfersUpdate(_activeTransfers.values.toList());
            return;
        }
        _sendCommand('get_file', {'fileName': fileName, 'peer': peer});
    }

    List<Transfer> getCurrentTransfers() => _activeTransfers.values.toList();

    void handleFileMessage(Map<String, dynamic> msg) {
        final type = msg['type'] as String?;
        final payload = msg['payload'];

        switch (type) {
            case 'search_results':
                _handleSearchResult(payload as Map<String, dynamic>);
                break;
            case 'transfer_start':
                _handleTransferStart(payload as Map<String, dynamic>);
                break;
            case 'upload_request':
                _handleUploadRequest(payload as Map<String, dynamic>);
                break;
            case 'transfer_error':
                _handleTransferError(payload as Map<String, dynamic>);
                break;
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
        if (!_disposed) _onSearchResults(results);
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
            final file = File(filePath);
            if (!await file.exists()) {
                _sendCommand('upload_error', {'transferID': transferID, 'message': "File not found locally"});
                return;
            }

            final fileSize = await file.length();
            final partSize = (fileSize / _numStreams).ceil();
            final uploadFutures = <Future>[];

            for (int i = 0; i < _numStreams; i++) {
                final startByte = i * partSize;
                final endByte = min((i + 1) * partSize, fileSize);
                if (startByte >= endByte) continue;

                uploadFutures.add(() async {
                    SSHSession? dataSession;
                    try {
                        final subsystem = 'subsystem:data-transfer:$transferID:$i';
                        dataSession = await _client.execute(subsystem);
                        final fileStream = file.openRead(startByte, endByte);
                        await dataSession.stdin.addStream(fileStream.map((chunk) => Uint8List.fromList(chunk)));
                    } finally {
                        await dataSession?.stdin.close();
                        await dataSession?.done;
                    }
                }());
            }

            await Future.wait(uploadFutures);
            _sendCommand('upload_done', {'transferID': transferID});
        } catch (e) {
            _sendCommand('upload_error', {'transferID': transferID, 'message': "Fatal error reading file: $e"});
        }
    }

    void _handleTransferStart(Map<String, dynamic> payload) async {
        final transferID = payload['transferID'] as String;
        final filename = payload['fileName'] as String;
        final size = payload['size'] as int;
        final fromUser = payload['fromUser'] as String;

        final newTransfer = Transfer(
            transferID: transferID,
            fileName: filename,
            size: size,
            fromUser: fromUser,
            toUser: _nickname,
            status: TransferStatus.active,
            startedAt: DateTime.now(),
        );
        _activeTransfers[transferID] = newTransfer;
        if (!_disposed) _onTransfersUpdate(_activeTransfers.values.toList());

        RandomAccessFile? finalFileSink;
        List<File> partFiles = [];

        try {
            if (_libraryPath == null) throw Exception("Library path is not set.");

            final tempDir = await getTemporaryDirectory();
            final downloadFutures = <Future>[];
            int totalBytesDownloaded = 0;
            var lastUpdateTime = DateTime.now();

            for (int i = 0; i < _numStreams; i++) {
                final partPath = p.join(tempDir.path, '$transferID.part$i');
                partFiles.add(File(partPath));

                downloadFutures.add(() async {
                    SSHSession? dataSession;
                    IOSink? sink;
                    try {
                        final subsystem = 'subsystem:data-transfer:$transferID:$i';
                        dataSession = await _client.execute(subsystem);
                        sink = partFiles[i].openWrite();

                        await for (final chunk in dataSession.stdout) {
                            sink.add(chunk);
                            totalBytesDownloaded += chunk.length;
                            if (newTransfer.size > 0) {
                                newTransfer.progress = totalBytesDownloaded / newTransfer.size;
                            }

                            // --- START FIX ---
                            // Yield to the event loop after every single chunk is processed.
                            // This is more aggressive and ensures the UI thread can always
                            // process paint and touch events, preventing lock-ups.
                            await Future.delayed(Duration.zero);
                            // --- END FIX ---

                            final now = DateTime.now();
                            if (now.difference(lastUpdateTime).inMilliseconds > 100) {
                                if (!_disposed) _onTransfersUpdate(_activeTransfers.values.toList());
                                lastUpdateTime = now;
                            }
                        }
                    } finally {
                        await sink?.close();
                        await dataSession?.done;
                    }
                }());
            }

            await Future.wait(downloadFutures);

            final finalPath = p.join(_libraryPath!, filename);
            finalFileSink = await File(finalPath).open(mode: FileMode.write);
            for (final partFile in partFiles) {
                if (await partFile.exists()) {
                    await finalFileSink.writeFrom(await partFile.readAsBytes());
                    await partFile.delete();
                }
            }

            newTransfer.status = TransferStatus.complete;
            newTransfer.progress = 1.0;
            newTransfer.completedAt = DateTime.now();

        } catch (e) {
            newTransfer.status = TransferStatus.failed;
            newTransfer.error = "Download failed: $e";
            print("!!! DOWNLOAD FAILED: $e");
        } finally {
            await finalFileSink?.close();
            for (final partFile in partFiles) {
                if (await partFile.exists()) {
                    try {
                      await partFile.delete();
                    } catch (e) {
                      print("Error deleting temp file: $e");
                    }
                }
            }
            if (!_disposed) _onTransfersUpdate(_activeTransfers.values.toList());
        }
    }

    void _handleTransferError(Map<String, dynamic> payload) async {
        final transferID = payload['transferID'] as String?;
        final errorMsg = payload['message'] as String? ?? "Transfer failed by peer.";

        if (transferID == null) return;
        final transfer = _activeTransfers[transferID];
        if (transfer == null) return;

        transfer.status = TransferStatus.failed;
        transfer.error = errorMsg;
        if (!_disposed) _onTransfersUpdate(_activeTransfers.values.toList());
    }

    void dispose() {
        _disposed = true;
    }
}
