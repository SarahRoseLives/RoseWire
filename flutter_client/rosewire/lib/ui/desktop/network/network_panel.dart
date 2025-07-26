import 'dart:async';
import 'package:flutter/material.dart';
import '../../../services/ssh_chat_service.dart';
import '../rosewire_desktop.dart';

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
    // Listen to all messages from the service
    _statsSub = widget.chatService.messages.listen((msg) {
      // We only care about network_stats messages
      if (msg['type'] == 'network_stats') {
        if (mounted) {
          setState(() {
            _stats = msg['payload'] as Map<String, dynamic>;
            _loading = false;
          });
        }
      }
    });
    // Request stats when the panel loads
    widget.chatService.requestStats();
    setState(() {
      _loading = true;
    });
  }

  @override
  void dispose() {
    _statsSub?.cancel();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    final stats = _stats;
    final users = stats?['users'] as List<dynamic>? ?? [];
    final relayServers = stats?['relayServers'] ?? 1;
    final totalUsers = stats?['totalUsers'] ?? users.length;
    // New fields
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
              color: roseWhite,
              fontWeight: FontWeight.bold,
            ),
          ),
          SizedBox(height: 18),
          Card(
            color: roseGray.withOpacity(0.85),
            shape: RoundedRectangleBorder(
              borderRadius: BorderRadius.circular(16),
              side: BorderSide(color: rosePink.withOpacity(0.25), width: 1.2),
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
                    color: rosePink,
                  ),
                  _StatItem(
                    icon: Icons.cloud_sync,
                    label: "Relay Servers",
                    value: "$relayServers",
                    color: rosePurple,
                  ),
                  // Updated to use new stats field
                  _StatItem(
                    icon: Icons.swap_vertical_circle,
                    label: "Active Transfers",
                    value: "$activeTransfers",
                    color: roseGreen,
                  ),
                  // Updated to use new stats field
                  _StatItem(
                    icon: Icons.library_music,
                    label: "Total Transfers",
                    value: "$totalTransfers",
                    color: roseWhite,
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
              color: roseWhite,
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
                          user["status"] == "Online" ? roseGreen : roseWhite.withOpacity(0.6);
                      return Card(
                        color: roseGray.withOpacity(0.8),
                        elevation: 2,
                        margin: EdgeInsets.symmetric(vertical: 5),
                        shape: RoundedRectangleBorder(
                          borderRadius: BorderRadius.circular(12),
                        ),
                        child: ListTile(
                          leading: CircleAvatar(
                            backgroundColor: rosePink,
                            child: Text(
                              user["nickname"].toString().substring(0, 1).toUpperCase(),
                              style: TextStyle(color: roseWhite, fontWeight: FontWeight.bold),
                            ),
                          ),
                          title: Text(
                            user["nickname"].toString(),
                            style: TextStyle(color: roseWhite, fontWeight: FontWeight.bold),
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
            color: roseWhite.withOpacity(0.7),
            fontSize: 13,
          ),
        ),
      ],
    );
  }
}