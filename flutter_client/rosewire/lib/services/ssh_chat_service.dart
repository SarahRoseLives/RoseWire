import 'dart:async';
import 'dart:convert';
import 'dart:io';
import 'dart:math';
import 'dart:typed_data'; // Required for Uint8List
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

class SshChatService {
	String _host;
	int _port;

	SSHClient? _client;
	SSHSession? _chatSession;
	String? _nickname;
	bool _disposed = false;

	// You can increase this again to 5 or more for faster parallel downloads
	static const int _numStreams = 50;
	static const int _uploadChunkSize = 32768;

	final _messageController = StreamController<Map<String, dynamic>>.broadcast();
	Stream<Map<String, dynamic>> get messages => _messageController.stream;

	final _searchResultController = StreamController<List<SearchResult>>.broadcast();
	Stream<List<SearchResult>> get searchResults => _searchResultController.stream;

	final _transferController = StreamController<List<Transfer>>.broadcast();
	Stream<List<Transfer>> get transfers => _transferController.stream;

	final Map<String, Transfer> _activeTransfers = {};

	String? _libraryPath;

	SshChatService({String host = 'sarahsforge.dev', int port = 2222})
		: _host = host,
			_port = port;

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
				case 'transfer_error':
					_handleTransferError(payload as Map<String, dynamic>);
					break;
				case 'network_stats':
					_messageController.add(decoded);
					break;
				default:
					_messageController.add({
						'type': 'system_broadcast',
						'payload': {'text': "Unknown message: $msg", 'isSystem': true}
					});
			}
		} catch (e) {
			_messageController.add({
				'type': 'system_broadcast',
				'payload': {'text': "Error parsing: $msg", 'isSystem': true}
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
						dataSession = await _client!.execute(subsystem);

						final fileStream = file.openRead(startByte, endByte);

						await dataSession.stdin.addStream(
							fileStream.map((chunk) => Uint8List.fromList(chunk))
						);

						await dataSession.stdin.close();
						await dataSession.done;
					} catch (e) {
						// Error handling
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
			toUser: _nickname!,
			status: TransferStatus.active,
			startedAt: DateTime.now(),
		);
		_activeTransfers[transferID] = newTransfer;
		_transferController.add(_activeTransfers.values.toList());

		try {
			if (_libraryPath == null) {
				throw Exception("Library path is not set.");
			}
			final tempDir = await getTemporaryDirectory();
			final downloadFutures = <Future>[];
			final partFiles = <String>[];

			// **FIX:** Shared counter for total bytes downloaded.
			int totalBytesDownloaded = 0;
			// **FIX:** Throttling for UI updates.
			var lastUpdateTime = DateTime.now();

			for (int i = 0; i < _numStreams; i++) {
				final partPath = p.join(tempDir.path, '$transferID.part$i');
				partFiles.add(partPath);

				downloadFutures.add(() async {
					SSHSession? dataSession;
					try {
						final subsystem = 'subsystem:data-transfer:$transferID:$i';
						dataSession = await _client!.execute(subsystem);
						final sink = File(partPath).openWrite();

						await for (final chunk in dataSession.stdout) {
							sink.add(chunk);

							// **FIX:** Update progress inside the loop.
							totalBytesDownloaded += chunk.length;
							if (newTransfer.size > 0) {
								newTransfer.progress = totalBytesDownloaded / newTransfer.size;
							}

							// **FIX:** Throttle UI updates to every 100ms.
							final now = DateTime.now();
							if (now.difference(lastUpdateTime).inMilliseconds > 100) {
								 if (!_disposed) _transferController.add(_activeTransfers.values.toList());
								 lastUpdateTime = now;
							}
						}
						await sink.close();
						await dataSession.done;

					} catch (e) {
						rethrow;
					}
				}());
			}

			await Future.wait(downloadFutures);

			final finalPath = p.join(_libraryPath!, filename);
			final finalFileSink = File(finalPath).openWrite();
			for (final partPath in partFiles) {
				final partFile = File(partPath);
				await finalFileSink.addStream(partFile.openRead());
				await partFile.delete();
			}
			await finalFileSink.close();

			// Final update to set status to 100% and complete.
			newTransfer.status = TransferStatus.complete;
			newTransfer.progress = 1.0;
			newTransfer.completedAt = DateTime.now();
			_transferController.add(_activeTransfers.values.toList());
		} catch (e) {
			newTransfer.status = TransferStatus.failed;
			newTransfer.error = "Download failed: $e";
			_transferController.add(_activeTransfers.values.toList());
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
		_disposed = true;
		_chatSession?.close();
		_client?.close();
		_messageController.close();
		_searchResultController.close();
		_transferController.close();
	}
}