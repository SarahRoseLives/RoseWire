import 'package:flutter/material.dart';
import 'package:shared_preferences/shared_preferences.dart';
import '../../../theme_manager.dart';

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
    final server =
        prefs.getString('rosewire_server') ?? 'rosewire.rosevines.network';
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
        _notice =
            "Instance updated. Please restart the app to connect to the new instance.";
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
          constraints: const BoxConstraints(maxWidth: 550),
          child: Column(
            mainAxisAlignment: MainAxisAlignment.center,
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              const Text(
                "Settings",
                style: TextStyle(
                    color: Colors.white,
                    fontSize: 24,
                    fontWeight: FontWeight.bold),
              ),
              const SizedBox(height: 32),
              _buildInstanceSettings(context),
              const Divider(height: 48, color: Colors.white24),
              _buildThemeSettings(context),
            ],
          ),
        ),
      ),
    );
  }

  Widget _buildInstanceSettings(BuildContext context) {
    final theme = Theme.of(context);
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Text(
          "Home Instance",
          style: TextStyle(
              color: theme.colorScheme.primary,
              fontSize: 16,
              fontWeight: FontWeight.w600),
        ),
        const SizedBox(height: 8),
        Text(
          "This is the server your client connects to. Your identity will be tied to this instance (e.g., @user@instance.com).",
          style: TextStyle(
              color: theme.colorScheme.onBackground.withOpacity(0.7),
              fontSize: 14),
        ),
        const SizedBox(height: 12),
        TextField(
          controller: _serverController,
          decoration: InputDecoration(
            hintText: "e.g., rosewire.rosevines.network",
            hintStyle: TextStyle(color: Colors.white.withOpacity(0.5)),
            filled: true,
            fillColor: Colors.black.withOpacity(0.2),
            border: OutlineInputBorder(
              borderRadius: BorderRadius.circular(10),
              borderSide: BorderSide.none,
            ),
            contentPadding:
                const EdgeInsets.symmetric(vertical: 12, horizontal: 16),
          ),
          style: const TextStyle(color: Colors.white, fontSize: 15),
        ),
        const SizedBox(height: 20),
        Align(
          alignment: Alignment.centerRight,
          child: ElevatedButton.icon(
            icon: const Icon(Icons.save),
            label: const Text("Save Instance"),
            style: ElevatedButton.styleFrom(
              padding: const EdgeInsets.symmetric(horizontal: 24, vertical: 14),
              shape: RoundedRectangleBorder(
                  borderRadius: BorderRadius.circular(10)),
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
              style: const TextStyle(
                  color: Colors.greenAccent, fontSize: 14),
            ),
          ),
        ],
      ],
    );
  }

  Widget _buildThemeSettings(BuildContext context) {
    final theme = Theme.of(context);
    final currentAccentColor = theme.primaryColor;

    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Text(
          "Accent Color",
          style: TextStyle(
              color: theme.colorScheme.primary,
              fontSize: 16,
              fontWeight: FontWeight.w600),
        ),
        const SizedBox(height: 8),
        Text(
          "Personalize the look of the app's key elements.",
          style: TextStyle(
              color: theme.colorScheme.onBackground.withOpacity(0.7),
              fontSize: 14),
        ),
        const SizedBox(height: 20),
        Wrap(
          spacing: 12.0,
          runSpacing: 12.0,
          children: ThemeManager.availableColors.map((color) {
            bool isSelected = color == currentAccentColor;
            return Tooltip(
              message: color.toString(),
              child: GestureDetector(
                onTap: () {
                  themeManager.setTheme(color);
                },
                child: Container(
                  width: 40,
                  height: 40,
                  decoration: BoxDecoration(
                    color: color,
                    shape: BoxShape.circle,
                    border: isSelected
                        ? Border.all(color: Colors.white70, width: 3)
                        : Border.all(color: Colors.white24, width: 1),
                  ),
                  child: isSelected
                      ? Icon(
                          Icons.check,
                          // --- CHANGE: Use white icon unless background is white ---
                          color: color == Colors.white ? Colors.black : Colors.white,
                        )
                      : null,
                ),
              ),
            );
          }).toList(),
        ),
      ],
    );
  }
}