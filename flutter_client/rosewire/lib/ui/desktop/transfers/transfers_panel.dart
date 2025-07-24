import 'package:flutter/material.dart';
import '../rosewire_desktop.dart';

class TransfersPanel extends StatelessWidget {
  const TransfersPanel({super.key});

  final List<Map<String, dynamic>> transfers = const [
    {
      "title": "RosePetal.flac",
      "progress": 0.73,
      "speed": "1.2 MB/s",
      "status": "Downloading",
      "user": "audioenthusiast"
    },
    {
      "title": "PinkNoise.wav",
      "progress": 1.0,
      "speed": "Done",
      "status": "Complete",
      "user": "sshshare"
    },
  ];

  @override
  Widget build(BuildContext context) {
    return Padding(
      padding: const EdgeInsets.symmetric(horizontal: 32, vertical: 18),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Text(
            "Active Transfers",
            style: TextStyle(
              fontSize: 18,
              color: roseWhite,
              fontWeight: FontWeight.w600,
            ),
          ),
          SizedBox(height: 18),
          Expanded(
            child: ListView.builder(
              itemCount: transfers.length,
              itemBuilder: (context, idx) {
                final item = transfers[idx];
                return Card(
                  elevation: 3,
                  margin: EdgeInsets.symmetric(vertical: 8),
                  color: roseGray.withOpacity(0.85),
                  shape: RoundedRectangleBorder(
                    borderRadius: BorderRadius.circular(16),
                    side: BorderSide(
                      color: roseGreen.withOpacity(0.2),
                      width: 1.2,
                    ),
                  ),
                  child: ListTile(
                    leading: Icon(
                      item["progress"] == 1.0 ? Icons.check_circle : Icons.downloading,
                      color: item["progress"] == 1.0 ? roseGreen : rosePink,
                    ),
                    title: Text(item["title"] ?? "", style: TextStyle(color: roseWhite, fontWeight: FontWeight.bold)),
                    subtitle: Text("${item["status"] ?? ""} • ${item["speed"] ?? ""}", style: TextStyle(color: roseWhite.withOpacity(0.7))),
                    trailing: SizedBox(
                      width: 120,
                      child: Column(
                        mainAxisAlignment: MainAxisAlignment.center,
                        children: [
                          LinearProgressIndicator(
                            value: item["progress"] as double,
                            color: roseGreen,
                            backgroundColor: rosePink.withOpacity(0.2),
                            minHeight: 8,
                            borderRadius: BorderRadius.circular(8),
                          ),
                          SizedBox(height: 6),
                          Text(item["user"] ?? "", style: TextStyle(color: rosePink, fontWeight: FontWeight.bold)),
                        ],
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