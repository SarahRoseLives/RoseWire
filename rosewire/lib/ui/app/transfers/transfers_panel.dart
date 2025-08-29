import 'dart:async';
import 'package:flutter/material.dart';
import '../../../services/ssh_file_service.dart';
import '../../../services/ssh_chat_service.dart';

class TransfersPanelMobile extends StatefulWidget {
  final SshChatService chatService;
  const TransfersPanelMobile({super.key, required this.chatService});

  @override
  State<TransfersPanelMobile> createState() => _TransfersPanelMobileState();
}

class _TransfersPanelMobileState extends State<TransfersPanelMobile> {
  StreamSubscription? _transferSubscription;
  List<Transfer> _transfers = [];

  @override
  void initState() {
    super.initState();

    // Get the current list of transfers when the panel first loads.
    // This ensures that if transfers were started on another screen,
    // they appear immediately when the user navigates here.
    _transfers = widget.chatService.getCurrentTransfers();

    // Subscribe to all future updates to the transfer list.
    _transferSubscription = widget.chatService.transfers.listen((updatedTransfers) {
      if (mounted) {
        // The service sends the complete, updated list every time.
        // We sort it to show the newest transfers at the top.
        updatedTransfers.sort((a, b) => b.startedAt.compareTo(a.startedAt));
        setState(() {
          _transfers = updatedTransfers;
        });
      }
    });
  }

  @override
  void dispose() {
    _transferSubscription?.cancel();
    super.dispose();
  }

  // Helper methods to determine UI elements based on transfer status.

  String _statusText(TransferStatus status) {
    switch (status) {
      case TransferStatus.pending:
        return "Pending...";
      case TransferStatus.active:
        return "Downloading...";
      case TransferStatus.complete:
        return "Complete";
      case TransferStatus.failed:
        return "Failed";
    }
  }

  IconData _statusIcon(TransferStatus status) {
    switch (status) {
      case TransferStatus.pending:
        return Icons.hourglass_top_rounded;
      case TransferStatus.active:
        return Icons.downloading_rounded;
      case TransferStatus.complete:
        return Icons.check_circle_rounded;
      case TransferStatus.failed:
        return Icons.error_rounded;
    }
  }

  Color _statusColor(TransferStatus status) {
    switch (status) {
      case TransferStatus.pending:
        return Colors.white.withOpacity(0.8);
      case TransferStatus.active:
        return Colors.pinkAccent;
      case TransferStatus.complete:
        return Colors.greenAccent;
      case TransferStatus.failed:
        return Colors.redAccent;
    }
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      backgroundColor: Colors.grey[900],
      body: _transfers.isEmpty
          ? Center(
              child: Text(
                "No active or recent transfers.",
                style: TextStyle(color: Colors.white.withOpacity(0.7), fontSize: 16),
              ),
            )
          : ListView.builder(
              padding: const EdgeInsets.all(8.0),
              itemCount: _transfers.length,
              itemBuilder: (context, idx) {
                final item = _transfers[idx];
                final color = _statusColor(item.status);

                return Card(
                  elevation: 3,
                  margin: const EdgeInsets.symmetric(vertical: 6, horizontal: 8),
                  color: Colors.grey[850],
                  shape: RoundedRectangleBorder(
                    borderRadius: BorderRadius.circular(12),
                    side: BorderSide(color: color.withOpacity(0.2), width: 1),
                  ),
                  child: Padding(
                    padding: const EdgeInsets.symmetric(vertical: 12.0, horizontal: 8.0),
                    child: Column(
                      crossAxisAlignment: CrossAxisAlignment.start,
                      children: [
                        ListTile(
                          leading: Icon(_statusIcon(item.status), color: color, size: 36),
                          title: Text(
                            item.fileName,
                            style: const TextStyle(color: Colors.white, fontWeight: FontWeight.bold, fontSize: 16),
                          ),
                          subtitle: Text(
                            "${_statusText(item.status)} from ${item.fromUser}",
                            style: TextStyle(color: Colors.white.withOpacity(0.7)),
                          ),
                        ),
                        if (item.status == TransferStatus.active || item.status == TransferStatus.complete)
                          Padding(
                            padding: const EdgeInsets.fromLTRB(16, 4, 16, 8),
                            child: Row(
                              children: [
                                Expanded(
                                  child: LinearProgressIndicator(
                                    value: item.progress,
                                    color: color,
                                    backgroundColor: color.withOpacity(0.2),
                                    minHeight: 6,
                                    borderRadius: BorderRadius.circular(6),
                                  ),
                                ),
                                const SizedBox(width: 12),
                                Text(
                                  item.status == TransferStatus.active && item.speed.isNotEmpty
                                      ? item.speed
                                      : "${(item.progress * 100).toStringAsFixed(0)}%",
                                  style: TextStyle(color: color, fontWeight: FontWeight.bold),
                                ),
                              ],
                            ),
                          ),
                        if (item.status == TransferStatus.failed && item.error.isNotEmpty)
                          Padding(
                            padding: const EdgeInsets.fromLTRB(16, 0, 16, 8),
                            child: Text(
                              "Error: ${item.error}",
                              style: TextStyle(color: Colors.redAccent.withOpacity(0.9), fontSize: 12),
                            ),
                          ),
                      ],
                    ),
                  ),
                );
              },
            ),
    );
  }
}