import 'package:flutter/material.dart';
import 'package:shared_preferences/shared_preferences.dart';
import '../rosewire_desktop.dart';

class SettingsPanel extends StatefulWidget {
  const SettingsPanel({super.key});

  @override
  State<SettingsPanel> createState() => _SettingsPanelState();
}

class _SettingsPanelState extends State<SettingsPanel> {
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
    // Load the saved server, defaulting if it doesn't exist.
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
        _notice = "Instance updated. Please restart the app to connect to the new instance.";
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
    return Padding(
      padding: const EdgeInsets.symmetric(horizontal: 32, vertical: 18),
      child: Center(
        child: ConstrainedBox(
          constraints: const BoxConstraints(maxWidth: 500),
          child: Column(
            mainAxisAlignment: MainAxisAlignment.center,
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              const Text(
                "Settings",
                style: TextStyle(color: roseWhite, fontSize: 24, fontWeight: FontWeight.bold),
              ),
              const SizedBox(height: 32),
              const Text(
                "Home Instance",
                style: TextStyle(color: rosePink, fontSize: 16, fontWeight: FontWeight.w600),
              ),
              const SizedBox(height: 8),
               Text(
                "This is the server your client connects to. Your identity will be tied to this instance (e.g., @user@instance.com).",
                style: TextStyle(color: roseWhite.withOpacity(0.7), fontSize: 14),
              ),
              const SizedBox(height: 12),
              TextField(
                controller: _serverController,
                decoration: InputDecoration(
                  hintText: "e.g., rosewire.rosevines.network",
                  hintStyle: TextStyle(color: roseWhite.withOpacity(0.5)),
                  filled: true,
                  fillColor: roseGray.withOpacity(0.85),
                  border: OutlineInputBorder(
                    borderRadius: BorderRadius.circular(10),
                    borderSide: BorderSide.none,
                  ),
                  contentPadding: const EdgeInsets.symmetric(vertical: 12, horizontal: 16),
                ),
                style: const TextStyle(color: roseWhite, fontSize: 15),
              ),
              const SizedBox(height: 20),
              Align(
                alignment: Alignment.centerRight,
                child: ElevatedButton.icon(
                  icon: const Icon(Icons.save),
                  label: const Text("Save Instance"),
                  style: ElevatedButton.styleFrom(
                    backgroundColor: rosePink,
                    foregroundColor: roseWhite,
                    padding: const EdgeInsets.symmetric(horizontal: 24, vertical: 14),
                    shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(10)),
                    elevation: 0,
                  ),
                  onPressed: _saveServerPreference,
                ),
              ),
              if (_notice != null) ...[
                const SizedBox(height: 24),
                Center(
                  child: Text(
                    _notice!,
                    textAlign: TextAlign.center,
                    style: const TextStyle(color: roseGreen, fontSize: 14),
                  ),
                ),
              ],
            ],
          ),
        ),
      ),
    );
  }
}