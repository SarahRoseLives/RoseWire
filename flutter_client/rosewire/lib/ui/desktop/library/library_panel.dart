import 'package:flutter/material.dart';
import '../rosewire_desktop.dart';
import 'dart:io';
import 'dart:convert';
import 'package:path_provider/path_provider.dart';

class LibraryPanel extends StatefulWidget {
  final String nickname;
  final void Function(String folderPath, List<File> files) onLibraryChanged;

  const LibraryPanel({
    super.key,
    required this.nickname,
    required this.onLibraryChanged,
  });

  @override
  State<LibraryPanel> createState() => _LibraryPanelState();
}

class _LibraryPanelState extends State<LibraryPanel> {
  List<FileSystemEntity> _files = [];
  bool _loading = false;
  String? _error;
  String? _downloadsPath;
  bool _initialized = false;

  final String _configFilename = "rosewire_library.json";

  @override
  void initState() {
    super.initState();
    _restoreLibrary();
  }

  Future<File> _libraryConfigFile() async {
    final dir = await getApplicationSupportDirectory();
    return File('${dir.path}/${widget.nickname}_$_configFilename');
  }

  Future<void> _restoreLibrary() async {
    try {
      final configFile = await _libraryConfigFile();
      if (await configFile.exists()) {
        final config = jsonDecode(await configFile.readAsString());
        final folderPath = config["folderPath"] as String?;
        if (folderPath != null && folderPath.isNotEmpty) {
          setState(() {
            _downloadsPath = folderPath;
            _loading = true;
            _error = null;
          });
          await _loadFilesFromFolder(folderPath, persist: false);
        }
      }
    } catch (e) {
      // Ignore errors, let user select
    }
    setState(() {
      _initialized = true;
    });
  }

  Future<void> _selectFolder() async {
    String? selectedPath = await _showFolderPicker();
    if (selectedPath != null && selectedPath.isNotEmpty) {
      setState(() {
        _downloadsPath = selectedPath;
        _loading = true;
        _error = null;
      });
      await _loadFilesFromFolder(selectedPath, persist: true);
    }
  }

  Future<void> _loadFilesFromFolder(String folderPath, {bool persist = true}) async {
    try {
      final dir = Directory(folderPath);
      if (await dir.exists()) {
        final files = await dir
            .list()
            .where((f) => f is File)
            .toList();

        setState(() {
          _files = files;
          _loading = false;
        });

        // Save the chosen library folder and file names
        if (persist) {
          final configFile = await _libraryConfigFile();
          final config = {
            "folderPath": folderPath,
            "files": files.map((f) => (f as File).path).toList(),
          };
          await configFile.writeAsString(jsonEncode(config));
        }

        // Notify parent (desktop) so it can trigger sharing
        widget.onLibraryChanged(
          folderPath,
          files.cast<File>(),
        );
      } else {
        setState(() {
          _files = [];
          _loading = false;
          _error = "Selected directory does not exist.";
        });
      }
    } catch (e) {
      setState(() {
        _files = [];
        _loading = false;
        _error = "Failed to load files: $e";
      });
    }
  }

  Future<String?> _showFolderPicker() async {
    // See previous implementation
    String? result = await showDialog<String>(
      context: context,
      builder: (context) {
        final controller = TextEditingController(text: _downloadsPath ?? "");
        return AlertDialog(
          title: Text("Select Downloads Folder"),
          content: TextField(
            controller: controller,
            decoration: InputDecoration(
              labelText: "Folder Path",
              hintText: "/home/user/Downloads",
            ),
          ),
          actions: [
            TextButton(
              onPressed: () => Navigator.of(context).pop(null),
              child: Text("Cancel"),
            ),
            ElevatedButton(
              onPressed: () => Navigator.of(context).pop(controller.text.trim()),
              child: Text("Select"),
            ),
          ],
        );
      },
    );
    return result;
  }

  @override
  Widget build(BuildContext context) {
    return Padding(
      padding: const EdgeInsets.symmetric(horizontal: 32, vertical: 18),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Row(
            children: [
              Text(
                "My Library (${_downloadsPath ?? "Choose Folder"})",
                style: TextStyle(
                  fontSize: 18,
                  color: roseWhite,
                  fontWeight: FontWeight.w600,
                ),
              ),
              SizedBox(width: 16),
              ElevatedButton.icon(
                icon: Icon(Icons.folder_open),
                label: Text("Select Folder"),
                onPressed: _selectFolder,
                style: ElevatedButton.styleFrom(
                  backgroundColor: rosePink,
                  foregroundColor: roseWhite,
                  padding: EdgeInsets.symmetric(horizontal: 18, vertical: 10),
                  shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(8)),
                ),
              ),
            ],
          ),
          SizedBox(height: 18),
          Expanded(
            child: _downloadsPath == null && !_initialized
                ? Center(
                    child: CircularProgressIndicator(),
                  )
                : _downloadsPath == null
                    ? Center(
                        child: Text(
                          "Please select your downloads folder.",
                          style: TextStyle(color: roseWhite),
                        ),
                      )
                    : _loading
                        ? Center(child: CircularProgressIndicator())
                        : (_error != null)
                            ? Center(
                                child: Text(
                                  _error!,
                                  style: TextStyle(color: roseWhite),
                                ),
                              )
                            : _files.isEmpty
                                ? Center(
                                    child: Text(
                                      "No files found in selected folder.",
                                      style: TextStyle(color: roseWhite),
                                    ),
                                  )
                                : ListView.builder(
                                    itemCount: _files.length,
                                    itemBuilder: (context, idx) {
                                      final file = _files[idx] as File;
                                      final name = file.path.split(Platform.pathSeparator).last;
                                      final size = file.lengthSync();
                                      return Card(
                                        elevation: 2,
                                        margin: EdgeInsets.symmetric(vertical: 8),
                                        color: roseGray.withOpacity(0.85),
                                        shape: RoundedRectangleBorder(
                                          borderRadius: BorderRadius.circular(16),
                                          side: BorderSide(
                                            color: rosePurple.withOpacity(0.2),
                                            width: 1.2,
                                          ),
                                        ),
                                        child: ListTile(
                                          leading: Icon(Icons.insert_drive_file, color: rosePink),
                                          title: Text(name, style: TextStyle(color: roseWhite, fontWeight: FontWeight.bold)),
                                          subtitle: Text(
                                            "${(size / (1024 * 1024)).toStringAsFixed(2)} MB",
                                            style: TextStyle(color: roseWhite.withOpacity(0.7)),
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