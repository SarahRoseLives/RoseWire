import 'package:flutter/material.dart';
import 'dart:io' show Platform;

// Desktop-specific UI components
import 'ui/desktop/login_panel.dart';
import 'ui/desktop/rosewire_desktop.dart';

// Mobile-specific UI root
import 'ui/app/rosewire_app.dart';

void main() {
  // Check if the platform is mobile (Android or iOS)
  // and run the corresponding app version.
  if (Platform.isAndroid || Platform.isIOS) {
    runApp(const RoseWireMobileApp());
  } else {
    runApp(const RoseWireDesktopApp());
  }
}

/// A wrapper for the Mobile UI.
/// This provides the MaterialApp needed for the mobile screens.
class RoseWireMobileApp extends StatelessWidget {
  const RoseWireMobileApp({super.key});

  @override
  Widget build(BuildContext context) {
    return MaterialApp(
      title: 'RoseWire',
      debugShowCheckedModeBanner: false,
      theme: ThemeData(
        brightness: Brightness.dark,
        useMaterial3: true,
      ),
      home: const RoseWireAppMobile(), // The root widget for mobile
    );
  }
}

/// The original app class, renamed to specify it's for Desktop.
/// Its logic remains the same.
class RoseWireDesktopApp extends StatefulWidget {
  const RoseWireDesktopApp({super.key});
  @override
  State<RoseWireDesktopApp> createState() => _RoseWireDesktopAppState();
}

class _RoseWireDesktopAppState extends State<RoseWireDesktopApp> {
  bool _loggedIn = false;
  String? _nickname;
  String? _keyPath; // Path to selected private key

  void _onLogin(String nickname, String keyPath) {
    setState(() {
      _loggedIn = true;
      _nickname = nickname;
      _keyPath = keyPath;
    });
  }

  @override
  Widget build(BuildContext context) {
    return MaterialApp(
      title: 'RoseWire',
      debugShowCheckedModeBanner: false,
      theme: ThemeData(
        brightness: Brightness.dark,
        useMaterial3: true,
      ),
      home: _loggedIn
          ? RoseWireDesktop(nickname: _nickname!, keyPath: _keyPath!)
          : LoginPanel(onLogin: _onLogin),
    );
  }
}