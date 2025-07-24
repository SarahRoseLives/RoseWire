import 'package:flutter/material.dart';
import '../rosewire_desktop.dart';

// This is a mockup panel showing network stats.
// Later, it can be replaced with live data from the network.

class NetworkPanel extends StatelessWidget {
  const NetworkPanel({super.key});

  @override
  Widget build(BuildContext context) {
    // Example mock data
    final List<Map<String, dynamic>> users = [
      {"nickname": "musicfan01", "status": "Online"},
      {"nickname": "audioenthusiast", "status": "Online"},
      {"nickname": "sshshare", "status": "Idle"},
      {"nickname": "rosewirebot", "status": "Online"},
      {"nickname": "SarahRoseLives", "status": "Online"},
    ];

    final int relayServers = 5;
    final int totalUsers = users.length;
    final int totalTransfers = 7;
    final int activeTransfers = 2;

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
                  _StatItem(
                    icon: Icons.swap_vertical_circle,
                    label: "Active Transfers",
                    value: "$activeTransfers",
                    color: roseGreen,
                  ),
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
            child: ListView.builder(
              itemCount: users.length,
              itemBuilder: (context, idx) {
                final user = users[idx];
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
                        user["nickname"].substring(0, 1).toUpperCase(),
                        style: TextStyle(color: roseWhite, fontWeight: FontWeight.bold),
                      ),
                    ),
                    title: Text(
                      user["nickname"],
                      style: TextStyle(color: roseWhite, fontWeight: FontWeight.bold),
                    ),
                    trailing: Text(
                      user["status"],
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