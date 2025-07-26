import 'package:flutter/material.dart';
import '../rosewire_desktop.dart';
import '../../../services/ssh_chat_service.dart';

// Accept a SshChatService to update server
class SettingsPanel extends StatefulWidget {
  final SshChatService? chatService;
  const SettingsPanel({super.key, this.chatService});

  @override
  State<SettingsPanel> createState() => _SettingsPanelState();
}

class _SettingsPanelState extends State<SettingsPanel> {
  final _serverController = TextEditingController();
  String _currentServer = '';
  String? _notice;

  @override
  void initState() {
    super.initState();
    final host = widget.chatService?.host ?? 'sarahsforge.dev';
    setState(() {
      _currentServer = host;
      _serverController.text = host;
    });
  }

  void _saveServer() {
    final server = _serverController.text.trim();
    if (server.isEmpty) return;
    setState(() {
      _currentServer = server;
      _notice = "Server updated. Please restart to apply.";
    });
    widget.chatService?.setServer(host: server);
    // Optionally persist to disk for next session, left as TODO
  }

  @override
  Widget build(BuildContext context) {
    return Center(
      child: Padding(
        padding: const EdgeInsets.symmetric(horizontal: 32, vertical: 18),
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            Text(
              "Settings",
              style: TextStyle(color: roseWhite, fontSize: 24, fontWeight: FontWeight.bold),
            ),
            const SizedBox(height: 32),
            Align(
              alignment: Alignment.centerLeft,
              child: Text(
                "Network Server:",
                style: TextStyle(color: roseWhite, fontSize: 16, fontWeight: FontWeight.w600),
              ),
            ),
            const SizedBox(height: 8),
            TextField(
              controller: _serverController,
              decoration: InputDecoration(
                hintText: "sarahsforge.dev",
                hintStyle: TextStyle(color: roseWhite.withOpacity(0.5)),
                filled: true,
                fillColor: roseGray.withOpacity(0.85),
                border: OutlineInputBorder(
                  borderRadius: BorderRadius.circular(10),
                  borderSide: BorderSide.none,
                ),
                contentPadding: EdgeInsets.symmetric(vertical: 12, horizontal: 16),
              ),
              style: TextStyle(color: roseWhite, fontSize: 15),
            ),
            const SizedBox(height: 12),
            ElevatedButton.icon(
              icon: Icon(Icons.save),
              label: Text("Save Server"),
              style: ElevatedButton.styleFrom(
                backgroundColor: rosePink,
                foregroundColor: roseWhite,
                padding: EdgeInsets.symmetric(horizontal: 24, vertical: 14),
                shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(10)),
                elevation: 0,
              ),
              onPressed: _saveServer,
            ),
            if (_notice != null) ...[
              const SizedBox(height: 18),
              Text(
                _notice!,
                style: TextStyle(color: roseGreen, fontSize: 14),
              ),
            ],
            const SizedBox(height: 32),
            Text(
              "Settings will be here soon!",
              style: TextStyle(color: roseWhite, fontSize: 20),
            ),
          ],
        ),
      ),
    );
  }
}