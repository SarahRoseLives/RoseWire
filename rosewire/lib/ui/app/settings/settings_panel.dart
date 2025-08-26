import 'package:flutter/material.dart';
import 'package:shared_preferences/shared_preferences.dart';

class SettingsPanelMobile extends StatefulWidget {
  const SettingsPanelMobile({super.key});

  @override
  State<SettingsPanelMobile> createState() => _SettingsPanelMobileState();
}

class _SettingsPanelMobileState extends State<SettingsPanelMobile> {
  final _serverController = TextEditingController();
  String _currentServer = 'rosewire.rosevines.network';
  String? _notice;
  bool _isLoading = true;

  @override
  void initState() {
    super.initState();
    _loadServerPreference();
  }

  Future<void> _loadServerPreference() async {
    final prefs = await SharedPreferences.getInstance();
    final server = prefs.getString('rosewire_server') ?? 'rosewire.rosevines.network';
    if (mounted) {
      setState(() {
        _currentServer = server;
        _serverController.text = server;
        _isLoading = false;
      });
    }
  }

  Future<void> _saveServerPreference() async {
    final server = _serverController.text.trim();
    if (server.isEmpty) return;

    final prefs = await SharedPreferences.getInstance();
    await prefs.setString('rosewire_server', server);

    if (mounted) {
      setState(() {
        _currentServer = server;
        _notice = "Server updated. Please restart the app to connect to the new instance.";
      });
    }
  }

  @override
  void dispose() {
    _serverController.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    if (_isLoading) {
      return const Center(child: CircularProgressIndicator());
    }
    return Scaffold(
      backgroundColor: Colors.grey[900],
      body: SingleChildScrollView(
        padding: const EdgeInsets.all(16.0),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            const Text(
              "Settings",
              style: TextStyle(fontSize: 24, fontWeight: FontWeight.bold, color: Colors.white),
            ),
            const SizedBox(height: 24),
            Text(
              "Home Instance",
              style: TextStyle(color: Colors.pinkAccent, fontSize: 16, fontWeight: FontWeight.bold),
            ),
            const SizedBox(height: 8),
            Text(
              "This is the server your client connects to. Your identity will be tied to this instance (e.g., @user@instance.com).",
              style: TextStyle(color: Colors.white.withOpacity(0.7), fontSize: 14),
            ),
            const SizedBox(height: 12),
            TextField(
              controller: _serverController,
              decoration: InputDecoration(
                hintText: "e.g., rosewire.rosevines.network",
                hintStyle: TextStyle(color: Colors.white.withOpacity(0.5)),
                filled: true,
                fillColor: Colors.grey[800],
                border: OutlineInputBorder(
                  borderRadius: BorderRadius.circular(12),
                  borderSide: BorderSide.none,
                ),
              ),
              style: const TextStyle(color: Colors.white),
            ),
            const SizedBox(height: 16),
            SizedBox(
              width: double.infinity,
              child: ElevatedButton.icon(
                icon: const Icon(Icons.save),
                label: const Text("Save Instance"),
                onPressed: _saveServerPreference,
                style: ElevatedButton.styleFrom(
                  backgroundColor: Colors.pinkAccent,
                  foregroundColor: Colors.white,
                  padding: const EdgeInsets.symmetric(vertical: 12),
                  shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(12)),
                ),
              ),
            ),
            if (_notice != null) ...[
              const SizedBox(height: 24),
              Center(
                child: Text(
                  _notice!,
                  textAlign: TextAlign.center,
                  style: const TextStyle(color: Colors.greenAccent, fontSize: 14),
                ),
              ),
            ],
          ],
        ),
      ),
    );
  }
}