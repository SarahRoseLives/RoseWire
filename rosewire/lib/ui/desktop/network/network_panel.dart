// CLIENT/ui/desktop/network/network_panel.dart
import 'dart:async';
import 'package:flutter/material.dart';
import '../../../services/ssh_chat_service.dart';
import '../../../theme_manager.dart'; // Corrected import

class NetworkPanel extends StatefulWidget {
  final SshChatService chatService;
  const NetworkPanel({super.key, required this.chatService});

  @override
  State<NetworkPanel> createState() => _NetworkPanelState();
}

class _NetworkPanelState extends State<NetworkPanel> {
  Map<String, dynamic>? _stats;
  bool _loading = true;
  StreamSubscription? _statsSub;

  @override
  void initState() {
    super.initState();
    _statsSub = widget.chatService.messages.listen((msg) {
      if (msg['type'] == 'network_stats') {
        if (mounted) {
          setState(() {
            _stats = msg['payload'] as Map<String, dynamic>;
            _loading = false;
          });
        }
      }
    });
  }

  @override
  void dispose() {
    _statsSub?.cancel();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final stats = _stats;
    final users = stats?['users'] as List<dynamic>? ?? [];
    final relayServers = stats?['relayServers'] ?? 1;
    final totalUsers = stats?['totalUsers'] ?? users.length;
    final totalTransfers = stats?['totalTransfers'] ?? 0;
    final activeTransfers = stats?['activeTransfers'] ?? 0;

    return Padding(
      padding: const EdgeInsets.symmetric(horizontal: 32, vertical: 18),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Text(
            "Network Stats",
            style: TextStyle(
              fontSize: 20,
              color: theme.colorScheme.onBackground,
              fontWeight: FontWeight.bold,
            ),
          ),
          SizedBox(height: 18),
          Card(
            color: theme.colorScheme.surface.withOpacity(0.5),
            shape: RoundedRectangleBorder(
              borderRadius: BorderRadius.circular(16),
              side: BorderSide(color: theme.colorScheme.primary.withOpacity(0.25), width: 1.2),
            ),
            elevation: 4,
            child: Padding(
              padding: const EdgeInsets.symmetric(vertical: 18, horizontal: 24),
              child: Row(
                mainAxisAlignment: MainAxisAlignment.spaceAround,
                children: [
                  _StatItem(
                    icon: Icons.people_alt,
                    label: "Users Online",
                    value: "$totalUsers",
                    color: theme.colorScheme.primary,
                  ),
                  _StatItem(
                    icon: Icons.cloud_sync,
                    label: "Relay Servers",
                    value: "$relayServers",
                    color: Colors.purpleAccent,
                  ),
                  _StatItem(
                    icon: Icons.swap_vertical_circle,
                    label: "Active Transfers",
                    value: "$activeTransfers",
                    color: statusGreen,
                  ),
                  _StatItem(
                    icon: Icons.library_music,
                    label: "Total Transfers",
                    value: "$totalTransfers",
                    color: theme.colorScheme.onSurface,
                  ),
                ],
              ),
            ),
          ),
          SizedBox(height: 28),
          Text(
            "Users on the Network",
            style: TextStyle(
              fontSize: 16,
              color: theme.colorScheme.onBackground,
              fontWeight: FontWeight.w600,
            ),
          ),
          SizedBox(height: 12),
          Expanded(
            child: _loading
                ? Center(child: CircularProgressIndicator())
                : ListView.builder(
                    itemCount: users.length,
                    itemBuilder: (context, idx) {
                      final user = users[idx] as Map<String, dynamic>;
                      final statusColor =
                          user["status"] == "Online" ? statusGreen : theme.colorScheme.onSurface.withOpacity(0.6);
                      final nickname = user["nickname"].toString();

                      String nameForAvatar = nickname;
                      if (nickname.startsWith('@')) {
                          final parts = nickname.substring(1).split('@');
                          if (parts.isNotEmpty && parts[0].isNotEmpty) {
                              nameForAvatar = parts[0];
                          }
                      }
                      final avatarChar = nameForAvatar.isNotEmpty ? nameForAvatar.substring(0, 1).toUpperCase() : '?';

                      return Card(
                        color: theme.colorScheme.surface.withOpacity(0.4),
                        elevation: 2,
                        margin: EdgeInsets.symmetric(vertical: 5),
                        shape: RoundedRectangleBorder(
                          borderRadius: BorderRadius.circular(12),
                        ),
                        child: ListTile(
                          leading: CircleAvatar(
                            backgroundColor: theme.colorScheme.primary,
                            child: Text(
                              avatarChar,
                              style: TextStyle(color: theme.colorScheme.onPrimary, fontWeight: FontWeight.bold),
                            ),
                          ),
                          title: Text(
                            nickname,
                            style: TextStyle(color: theme.colorScheme.onSurface, fontWeight: FontWeight.bold),
                          ),
                          trailing: Text(
                            user["status"].toString(),
                            style: TextStyle(
                              color: statusColor,
                              fontWeight: FontWeight.bold,
                            ),
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

class _StatItem extends StatelessWidget {
  final IconData icon;
  final String label;
  final String value;
  final Color color;
  const _StatItem({
    required this.icon,
    required this.label,
    required this.value,
    required this.color,
  });

  @override
  Widget build(BuildContext context) {
    return Column(
      children: [
        Icon(icon, color: color, size: 32),
        SizedBox(height: 6),
        Text(
          value,
          style: TextStyle(
            color: color,
            fontWeight: FontWeight.bold,
            fontSize: 18,
          ),
        ),
        SizedBox(height: 4),
        Text(
          label,
          style: TextStyle(
            color: Theme.of(context).colorScheme.onSurface.withOpacity(0.7),
            fontSize: 13,
          ),
        ),
      ],
    );
  }
}