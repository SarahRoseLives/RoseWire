// CLIENT/ui/desktop/library/library_panel.dart
import 'dart:async';
import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:file_picker/file_picker.dart';
import 'dart:io';
import 'dart:convert';
import 'package:path_provider/path_provider.dart';
import 'package:url_launcher/url_launcher.dart';
import '../../../theme_manager.dart'; // Corrected import

// Helper function to get an icon based on the file extension.
IconData _getIconForFile(String fileName) {
  final extension = fileName.contains('.') ? fileName.split('.').last.toLowerCase() : '';
  switch (extension) {
    // Audio
    case 'mp3':
    case 'wav':
    case 'aac':
    case 'flac':
    case 'ogg':
    case 'm4a':
      return Icons.music_note;
    // Video
    case 'mp4':
    case 'mov':
    case 'avi':
    case 'mkv':
    case 'webm':
      return Icons.movie;
    // Image
    case 'jpg':
    case 'jpeg':
    case 'png':
    case 'gif':
    case 'bmp':
    case 'webp':
      return Icons.image;
    // Archive
    case 'zip':
    case 'rar':
    case '7z':
    case 'tar':
    case 'gz':
      return Icons.archive;
    // Document
    case 'pdf':
      return Icons.picture_as_pdf;
    case 'doc':
    case 'docx':
      return Icons.description; // Generic document icon
    case 'xls':
    case 'xlsx':
      return Icons.grid_on; // Spreadsheet icon
    case 'ppt':
    case 'pptx':
      return Icons.slideshow; // Presentation icon
    // Code/Text
    case 'txt':
    case 'md':
    case 'log':
      return Icons.article;
    case 'json':
    case 'xml':
    case 'html':
    case 'css':
    case 'js':
    case 'dart':
    case 'py':
    case 'java':
    case 'c':
    case 'cpp':
    case 'sh':
      return Icons.code;
    // Default
    default:
      return Icons.insert_drive_file;
  }
}


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
  final ScrollController _scrollController = ScrollController();
  Timer? _refreshTimer;

  final String _configFilename = "rosewire_library.json";

  @override
  void initState() {
    super.initState();
    _restoreLibrary();
    _refreshTimer = Timer.periodic(const Duration(minutes: 1), (timer) {
      if (_downloadsPath != null && mounted) {
        _loadFilesFromFolder(_downloadsPath!, persist: false, isBackgroundRefresh: true);
      }
    });
  }

  @override
  void dispose() {
    _refreshTimer?.cancel();
    _scrollController.dispose();
    super.dispose();
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

  Future<void> _openFolder() async {
    if (_downloadsPath == null) return;
    final uri = Uri.directory(_downloadsPath!);
    if (!await launchUrl(uri)) {
      if (mounted) {
        setState(() {
          _error = 'Could not open folder: $_downloadsPath';
        });
      }
    }
  }

  Future<void> _selectFolder() async {
    setState(() {
      _error = null; // Clear previous errors
    });

    String? selectedPath;

    try {
      selectedPath = await FilePicker.platform.getDirectoryPath(
        dialogTitle: 'Please select your RoseWire library folder',
      );
    } catch (e) {
      print('FilePicker (portal) failed: $e. Trying fallbacks...');
      selectedPath = null;
    }

    if (selectedPath == null) {
      try {
        final result = await Process.run('zenity', ['--file-selection', '--directory']);
        if (result.exitCode == 0) {
          selectedPath = (result.stdout as String).trim();
        }
      } catch (e) {
        print('Zenity not found or failed: $e');
        selectedPath = null;
      }
    }

    if (selectedPath == null) {
      try {
        final result = await Process.run('kdialog', ['--getexistingdirectory']);
        if (result.exitCode == 0) {
          selectedPath = (result.stdout as String).trim();
        }
      } catch (e) {
        print('Kdialog not found or failed: $e');
        selectedPath = null;
      }
    }

    if (selectedPath != null && selectedPath.isNotEmpty) {
      setState(() {
        _downloadsPath = selectedPath;
        _loading = true;
      });
      await _loadFilesFromFolder(selectedPath, persist: true);
    } else {
      setState(() {
        _error = 'Could not open folder picker.\nPlease ensure "zenity" or "kdialog" is installed on your system.';
      });
    }
  }

  Future<void> _loadFilesFromFolder(String folderPath, {bool persist = true, bool isBackgroundRefresh = false}) async {
    if (!isBackgroundRefresh) {
      if (mounted) {
        setState(() {
          _loading = true;
          _error = null;
        });
      }
    }

    try {
      final dir = Directory(folderPath);
      if (await dir.exists()) {
        final files = await dir
            .list()
            .where((f) => f is File)
            .toList();

        final currentFilePaths = _files.map((f) => f.path).toSet();
        final newFilePaths = files.map((f) => f.path).toSet();

        if (currentFilePaths.length == newFilePaths.length && currentFilePaths.containsAll(newFilePaths)) {
          if (!isBackgroundRefresh && mounted) {
            setState(() => _loading = false);
          }
          return; // No changes detected
        }


        if (mounted) {
          setState(() {
            _files = files;
            if (!isBackgroundRefresh) _loading = false;
          });
        }

        if (persist) {
          final configFile = await _libraryConfigFile();
          final config = {
            "folderPath": folderPath,
            "files": files.map((f) => (f as File).path).toList(),
          };
          await configFile.writeAsString(jsonEncode(config));
        }

        widget.onLibraryChanged(
          folderPath,
          files.cast<File>(),
        );
      } else {
        if (mounted) {
          setState(() {
            _files = [];
            _error = "Selected directory does not exist.";
            if (!isBackgroundRefresh) _loading = false;
          });
        }
      }
    } catch (e) {
      if (mounted) {
        setState(() {
          _files = [];
          _error = "Failed to load files: $e";
          if (!isBackgroundRefresh) _loading = false;
        });
      }
    }
  }

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    return Padding(
      padding: const EdgeInsets.symmetric(horizontal: 32, vertical: 18),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Row(
            children: [
              Flexible(
                child: Text(
                  "My Library (${_downloadsPath ?? "Choose Folder"})",
                  style: TextStyle(
                    fontSize: 18,
                    color: theme.colorScheme.onBackground,
                    fontWeight: FontWeight.w600,
                  ),
                  overflow: TextOverflow.ellipsis,
                ),
              ),
              const Spacer(),
              if (_downloadsPath != null)
                ElevatedButton.icon(
                  icon: const Icon(Icons.launch),
                  label: const Text("Open"),
                  onPressed: _openFolder,
                  style: ElevatedButton.styleFrom(
                    padding: const EdgeInsets.symmetric(horizontal: 18, vertical: 10),
                    shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(8)),
                  ),
                ),
              const SizedBox(width: 12),
              ElevatedButton.icon(
                icon: const Icon(Icons.folder_open),
                label: const Text("Select Folder"),
                onPressed: _selectFolder,
                style: ElevatedButton.styleFrom(
                  padding: const EdgeInsets.symmetric(horizontal: 18, vertical: 10),
                  shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(8)),
                ),
              ),
            ],
          ),
          const SizedBox(height: 18),
          Expanded(
            child: _downloadsPath == null && !_initialized
                ? const Center(child: CircularProgressIndicator())
                : _downloadsPath == null
                    ? Center(
                        child: Text(
                          "Please select your downloads folder.",
                          style: TextStyle(color: theme.colorScheme.onSurface),
                        ),
                      )
                    : _loading
                        ? const Center(child: CircularProgressIndicator())
                        : (_error != null)
                            ? Center(
                                child: Text(
                                  _error!,
                                  style: TextStyle(color: theme.colorScheme.error, fontSize: 16),
                                  textAlign: TextAlign.center,
                                ),
                              )
                            : _files.isEmpty
                                ? Center(
                                    child: Text(
                                      "No files found in selected folder.",
                                      style: TextStyle(color: theme.colorScheme.onSurface),
                                    ),
                                  )
                                : Scrollbar(
                                    thumbVisibility: true,
                                    controller: _scrollController,
                                    child: ListView.builder(
                                      controller: _scrollController,
                                      itemCount: _files.length,
                                      itemBuilder: (context, idx) {
                                        final file = _files[idx] as File;
                                        final name = file.path.split(Platform.pathSeparator).last;
                                        final size = file.lengthSync();
                                        return Card(
                                          elevation: 2,
                                          margin: const EdgeInsets.symmetric(vertical: 8),
                                          color: theme.colorScheme.surface.withOpacity(0.5),
                                          shape: RoundedRectangleBorder(
                                            borderRadius: BorderRadius.circular(16),
                                            side: BorderSide(
                                              color: theme.colorScheme.surface,
                                              width: 1.2,
                                            ),
                                          ),
                                          child: ListTile(
                                            leading: Icon(_getIconForFile(name), color: theme.colorScheme.primary),
                                            title: Text(name, style: TextStyle(color: theme.colorScheme.onSurface, fontWeight: FontWeight.bold)),
                                            subtitle: Text(
                                              "${(size / (1024 * 1024)).toStringAsFixed(2)} MB",
                                              style: TextStyle(color: theme.colorScheme.onSurface.withOpacity(0.7)),
                                            ),
                                          ),
                                        );
                                      },
                                    ),
                                  ),
          ),
        ],
      ),
    );
  }
}