import 'package:flutter/material.dart';

class LoginPanelMobile extends StatelessWidget {
  final void Function(String nickname, String keyPath) onLogin;
  const LoginPanelMobile({super.key, required this.onLogin});

  @override
  Widget build(BuildContext context) {
    // TODO: Implement Android-native login flow
    return Center(child: Text("LoginPanel (Android UI)"));
  }
}