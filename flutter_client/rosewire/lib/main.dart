import 'package:flutter/material.dart';
import 'ui/desktop/login_panel.dart';
import 'ui/desktop/rosewire_desktop.dart';

void main() {
  runApp(const RoseWireApp());
}

class RoseWireApp extends StatefulWidget {
  const RoseWireApp({super.key});
  @override
  State<RoseWireApp> createState() => _RoseWireAppState();
}

class _RoseWireAppState extends State<RoseWireApp> {
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