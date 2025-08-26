import 'package:flutter/material.dart';
import '../rosewire_desktop.dart';

class AboutPanel extends StatelessWidget {
  const AboutPanel({super.key});

  @override
  Widget build(BuildContext context) {
    return Center(
      child: Text(
        "RoseWire Modern Desktop\n\nInspired by the classics, built for the future.",
        textAlign: TextAlign.center,
        style: TextStyle(color: roseWhite, fontSize: 20),
      ),
    );
  }
}