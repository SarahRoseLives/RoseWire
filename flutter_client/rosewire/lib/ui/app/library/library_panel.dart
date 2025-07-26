import 'package:flutter/material.dart';

class LibraryPanelMobile extends StatelessWidget {
  final String nickname;
  final void Function(String folderPath, List<dynamic> files) onLibraryChanged;

  const LibraryPanelMobile({super.key, required this.nickname, required this.onLibraryChanged});

  @override
  Widget build(BuildContext context) {
    return Center(child: Text("LibraryPanel (Android UI)"));
  }
}