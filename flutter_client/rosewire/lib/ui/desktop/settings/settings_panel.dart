import 'package:flutter/material.dart';
import '../rosewire_desktop.dart';

class SettingsPanel extends StatelessWidget {
  const SettingsPanel({super.key});

  @override
  Widget build(BuildContext context) {
    return Center(
      child: Text(
        "Settings will be here soon!",
        style: TextStyle(color: roseWhite, fontSize: 20),
      ),
    );
  }
}