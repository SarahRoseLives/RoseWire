import 'package:flutter/material.dart';
import '../../../services/ssh_chat_service.dart';

class AboutPanel extends StatelessWidget {
  const AboutPanel({super.key});

  @override
  Widget build(BuildContext context) {
    return Center(
      child: Column(
        mainAxisAlignment: MainAxisAlignment.center,
        children: [
          Text(
            "RoseWire Modern Desktop",
            textAlign: TextAlign.center,
            style: TextStyle(
                color: Theme.of(context).colorScheme.onBackground,
                fontSize: 24,
                fontWeight: FontWeight.bold),
          ),
          const SizedBox(height: 12),
          Text(
            "Version: ${SshChatService.clientVersion}",
            textAlign: TextAlign.center,
            style: TextStyle(
              color: Theme.of(context).colorScheme.onBackground.withOpacity(0.7),
              fontSize: 16,
            ),
          ),
          const SizedBox(height: 24),
          Text(
            "Inspired by the classics, built for the future.",
            textAlign: TextAlign.center,
            style: TextStyle(
                color: Theme.of(context).colorScheme.onBackground.withOpacity(0.6),
                fontSize: 18,
                fontStyle: FontStyle.italic),
          ),
        ],
      ),
    );
  }
}