import 'dart:async';
import 'package:flutter/material.dart';
import '../../../services/ssh_chat_service.dart';
import '../rosewire_desktop.dart';

// Use the same colors as the rest of your UI
const rosePink = Color(0xFFEA4C89);
const rosePurple = Color(0xFF6C3483);
const roseWhite = Colors.white;
const roseGray = Color(0xFF22232A);
const roseGreen = Color(0xFF26C281);

class TransfersPanel extends StatefulWidget {
  final SshChatService chatService;
  const TransfersPanel({super.key, required this.chatService});

  @override
  State<TransfersPanel> createState() => _TransfersPanelState();
}

class _TransfersPanelState extends State<TransfersPanel> {
  StreamSubscription? _transferSubscription;
  List<Transfer> _transfers = [];

  @override
  void initState() {
    super.initState();

    // Get the current list of transfers upon panel initialization.
    // This prevents an empty panel if transfers were initiated before this
    // widget was built and subscribed to the stream.
    _transfers = widget.chatService.getCurrentTransfers();

    // Then, subscribe to all future updates to the transfer list.
    _transferSubscription = widget.chatService.transfers.listen((updatedTransfers) {
      if (mounted) {
        setState(() {
          // The service now sends back the complete list, so just replace it.
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
        return roseWhite.withOpacity(0.8);
      case TransferStatus.active:
        return rosePink;
      case TransferStatus.complete:
        return roseGreen;
      case TransferStatus.failed:
        return Colors.redAccent;
    }
  }

  @override
  Widget build(BuildContext context) {
    return Padding(
      padding: const EdgeInsets.symmetric(horizontal: 32, vertical: 18),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          const Text(
            "File Transfers",
            style: TextStyle(
              fontSize: 18,
              color: roseWhite,
              fontWeight: FontWeight.w600,
            ),
          ),
          const SizedBox(height: 18),
          Expanded(
            child: _transfers.isEmpty
                ? const Center(child: Text("No active or recent transfers.", style: TextStyle(color: roseWhite)))
                : ListView.builder(
                    itemCount: _transfers.length,
                    itemBuilder: (context, idx) {
                      final item = _transfers[idx];
                      final color = _statusColor(item.status);

                      return Card(
                        elevation: 3,
                        margin: const EdgeInsets.symmetric(vertical: 8),
                        color: roseGray.withOpacity(0.85),
                        shape: RoundedRectangleBorder(
                          borderRadius: BorderRadius.circular(16),
                          side: BorderSide(
                            color: color.withOpacity(0.2),
                            width: 1.2,
                          ),
                        ),
                        child: Padding(
                          padding: const EdgeInsets.symmetric(vertical: 8.0),
                          child: ListTile(
                            leading: Icon(_statusIcon(item.status), color: color, size: 32),
                            title: Text(item.fileName, style: const TextStyle(color: roseWhite, fontWeight: FontWeight.bold)),
                            subtitle: Column(
                              crossAxisAlignment: CrossAxisAlignment.start,
                              children: [
                                const SizedBox(height: 4),
                                Text(
                                  "${_statusText(item.status)} from ${item.fromUser}",
                                  style: TextStyle(color: roseWhite.withOpacity(0.7)),
                                ),
                                if (item.status == TransferStatus.failed && item.error.isNotEmpty)
                                  Padding(
                                    padding: const EdgeInsets.only(top: 4.0),
                                    child: Text(
                                      item.error,
                                      style: TextStyle(color: Colors.redAccent.withOpacity(0.9), fontSize: 12),
                                    ),
                                  ),
                              ],
                            ),
                            trailing: (item.status == TransferStatus.active || item.status == TransferStatus.complete)
                              ? SizedBox(
                                  width: 120,
                                  child: Column(
                                    mainAxisAlignment: MainAxisAlignment.center,
                                    crossAxisAlignment: CrossAxisAlignment.end,
                                    children: [
                                      Text(
                                        "${(item.progress * 100).toStringAsFixed(0)}%",
                                        style: TextStyle(color: color, fontWeight: FontWeight.bold),
                                      ),
                                      const SizedBox(height: 6),
                                      LinearProgressIndicator(
                                        value: item.progress,
                                        color: color,
                                        backgroundColor: color.withOpacity(0.2),
                                        minHeight: 6,
                                        borderRadius: BorderRadius.circular(6),
                                      ),
                                    ],
                                  ),
                                )
                              : null,
                          ),
                        ),
                      );
                    },
                  ),
          ),
        ],
      ),
    );
  }
}