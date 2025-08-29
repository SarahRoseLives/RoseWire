// CLIENT/ui/app/network/network_panel.dart
import 'dart:async';
import 'package:flutter/material.dart';
import '../../../services/ssh_chat_service.dart';
import '../../../theme_manager.dart'; // Corrected import

class NetworkPanelMobile extends StatefulWidget {
  final SshChatService chatService;
  const NetworkPanelMobile({super.key, required this.chatService});

  @override
  State<NetworkPanelMobile> createState() => _NetworkPanelMobileState();
}

class _NetworkPanelMobileState extends State<NetworkPanelMobile> {
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

    widget.chatService.requestStats();
  }

  @override
  void dispose() {
    _statsSub?.cancel();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    if (_loading) {
      return const Center(child: CircularProgressIndicator());
    }

    final stats = _stats;
    final users = stats?['users'] as List<dynamic>? ?? [];
    final relayServers = stats?['relayServers'] ?? 1;
    final totalUsers = stats?['totalUsers'] ?? users.length;
    final totalTransfers = stats?['totalTransfers'] ?? 0;
    final activeTransfers = stats?['activeTransfers'] ?? 0;

    return Scaffold(
      backgroundColor: Theme.of(context).scaffoldBackgroundColor,
      body: RefreshIndicator(
        onRefresh: () async {
          widget.chatService.requestStats();
        },
        child: ListView(
          padding: const EdgeInsets.all(12.0),
          children: [
            _buildStatsGrid(
              totalUsers: totalUsers,
              relayServers: relayServers,
              activeTransfers: activeTransfers,
              totalTransfers: totalTransfers,
            ),
            const SizedBox(height: 24),
            Text(
              "Users on the Network",
              style: TextStyle(
                fontSize: 18,
                color: Theme.of(context).colorScheme.onBackground.withOpacity(0.9),
                fontWeight: FontWeight.bold,
              ),
            ),
            const SizedBox(height: 8),
            if (users.isEmpty)
              const Center(
                  child: Padding(
                padding: EdgeInsets.all(32.0),
                child: Text("No user data available.",
                    style: TextStyle(color: Colors.white70)),
              ))
            else
              ...users.map((user) => _buildUserCard(user)).toList(),
          ],
        ),
      ),
    );
  }

  Widget _buildStatsGrid({
    required int totalUsers,
    required int relayServers,
    required int activeTransfers,
    required int totalTransfers,
  }) {
    final theme = Theme.of(context);
    return GridView.count(
      crossAxisCount: 2,
      shrinkWrap: true,
      physics: const NeverScrollableScrollPhysics(),
      crossAxisSpacing: 12,
      mainAxisSpacing: 12,
      childAspectRatio: 1.5,
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
          color: theme.colorScheme.secondary,
        ),
        _StatItem(
          icon: Icons.library_music,
          label: "Total Transfers",
          value: "$totalTransfers",
          color: theme.colorScheme.onSurface,
        ),
      ],
    );
  }

  Widget _buildUserCard(Map<String, dynamic> user) {
    final theme = Theme.of(context);
    final statusColor = user["status"] == "Online"
        ? statusGreen
        : theme.colorScheme.onSurface.withOpacity(0.6);
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
      color: theme.colorScheme.surface,
      elevation: 2,
      margin: const EdgeInsets.symmetric(vertical: 5),
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
          overflow: TextOverflow.ellipsis,
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
    final theme = Theme.of(context);
    return Card(
      color: theme.colorScheme.surface,
      elevation: 4,
      shape: RoundedRectangleBorder(
        borderRadius: BorderRadius.circular(16),
        side: BorderSide(color: color.withOpacity(0.3), width: 1),
      ),
      child: Column(
        mainAxisAlignment: MainAxisAlignment.center,
        children: [
          Icon(icon, color: color, size: 28),
          const SizedBox(height: 6),
          Text(
            value,
            style: TextStyle(
              color: color,
              fontWeight: FontWeight.bold,
              fontSize: 20,
            ),
          ),
          const SizedBox(height: 4),
          Text(
            label,
            textAlign: TextAlign.center,
            style: TextStyle(
              color: theme.colorScheme.onSurface.withOpacity(0.7),
              fontSize: 13,
            ),
          ),
        ],
      ),
    );
  }
}