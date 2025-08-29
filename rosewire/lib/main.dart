import 'package:flutter/material.dart';
import 'dart:io' show Platform;

// Desktop-specific UI components
import 'ui/desktop/login_panel.dart';
import 'ui/desktop/rosewire_desktop.dart';

// Mobile-specific UI root
import 'ui/app/rosewire_app.dart';

// Import the new ThemeManager
import 'theme_manager.dart';

void main() async {
  // Ensure widgets are initialized before loading the theme
  WidgetsFlutterBinding.ensureInitialized();
  await themeManager.loadTheme();

  // Check if the platform is mobile (Android or iOS)
  // and run the corresponding app version.
  if (Platform.isAndroid || Platform.isIOS) {
    runApp(const RoseWireMobileWrapper());
  } else {
    runApp(const RoseWireDesktopWrapper());
  }
}

/// A wrapper for the Mobile UI that listens to theme changes.
class RoseWireMobileWrapper extends StatelessWidget {
  const RoseWireMobileWrapper({super.key});

  @override
  Widget build(BuildContext context) {
    return AnimatedBuilder(
      animation: themeManager,
      builder: (context, child) {
        return MaterialApp(
          title: 'RoseWire',
          debugShowCheckedModeBanner: false,
          theme: themeManager.themeData,
          home: const RoseWireAppMobile(), // The root widget for mobile
        );
      },
    );
  }
}

/// A wrapper for the Desktop UI that listens to theme changes.
class RoseWireDesktopWrapper extends StatefulWidget {
  const RoseWireDesktopWrapper({super.key});
  @override
  State<RoseWireDesktopWrapper> createState() => _RoseWireDesktopWrapperState();
}

class _RoseWireDesktopWrapperState extends State<RoseWireDesktopWrapper> {
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
    return AnimatedBuilder(
      animation: themeManager,
      builder: (context, child) {
        return MaterialApp(
          title: 'RoseWire',
          debugShowCheckedModeBanner: false,
          theme: themeManager.themeData,
          home: _loggedIn
              ? RoseWireDesktop(nickname: _nickname!, keyPath: _keyPath!)
              : LoginPanel(onLogin: _onLogin),
        );
      },
    );
  }
}